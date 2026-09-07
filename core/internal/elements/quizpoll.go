// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// quizPollBody es la forma YAML del cuerpo de un `<<quiz>>`/`<<poll>>`.
//
// Deserializar con yaml.v3 en vez de leer las llaves a mano da gratis las dos
// formas de `options:` que el issue #198 fijó — la lista de guiones y el array
// inline `["a", "b"]` — y también las comillas, los escapes y los valores
// multilínea. Un lector línea a línea tendría que reimplementar todo eso.
//
// Answer es *int y no int para distinguir "no declarado" de "answer: 0" (la
// primera opción). El AST lo colapsa a -1 al construir el elemento; acá arriba
// la diferencia todavía importa.
type quizPollBody struct {
	Question    string   `yaml:"question"`
	Options     []string `yaml:"options"`
	Answer      *int     `yaml:"answer"`
	Explanation string   `yaml:"explanation"`
	Multiple    bool     `yaml:"multiple"`
}

// quizPollKeys son las llaves que cada tag reconoce. Todo lo demás se reporta
// (QUIZ005/POLL004) en vez de ignorarse en silencio: atrapa el `feedback:` que
// el website documentaba y un `answer:` escrito dentro de un poll, que es el
// error de copiar un quiz y cambiarle el tag.
var quizPollKeys = map[string]map[string]bool{
	"quiz": {"question": true, "options": true, "answer": true, "explanation": true},
	"poll": {"question": true, "options": true, "multiple": true},
}

// QuizParser parsea `<<quiz>> … <<end>>` (issue #198).
type QuizParser struct{}

func NewQuizParser() *QuizParser { return &QuizParser{} }

// CanParse exige la línea EXACTA. Los tags de quiz/poll no llevan atributos (a
// diferencia de `<<chart: bar>>` o `<<media …>>`), así que la igualdad resuelve
// de una vez el problema de frontera de palabra: `<<quizzes>>` no matchea, y no
// hace falta la maquinaria de matchesMediaTag.
func (p *QuizParser) CanParse(line string, mode string) bool {
	return strings.TrimSpace(line) == "<<quiz>>"
}

func (p *QuizParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	body, consumed, diags := parseQuizPollBlock(ctx, startIndex, "quiz")

	pos := diagnostics.NewPosition(startIndex+1, 1)
	quiz := ast.NewQuizElement(pos)
	quiz.Question = body.Question
	quiz.Options = body.Options
	quiz.Explanation = body.Explanation
	if body.Answer != nil {
		quiz.Answer = *body.Answer
	}
	if quiz.Options == nil {
		quiz.Options = make([]string, 0)
	}

	return &ParseResult{Element: quiz, ConsumedLines: consumed, Diagnostics: diags}
}

// PollParser parsea `<<poll>> … <<end>>` (issue #198).
type PollParser struct{}

func NewPollParser() *PollParser { return &PollParser{} }

func (p *PollParser) CanParse(line string, mode string) bool {
	return strings.TrimSpace(line) == "<<poll>>"
}

func (p *PollParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	body, consumed, diags := parseQuizPollBlock(ctx, startIndex, "poll")

	pos := diagnostics.NewPosition(startIndex+1, 1)
	poll := ast.NewPollElement(pos)
	poll.Question = body.Question
	poll.Options = body.Options
	poll.Multiple = body.Multiple
	if poll.Options == nil {
		poll.Options = make([]string, 0)
	}

	return &ParseResult{Element: poll, ConsumedLines: consumed, Diagnostics: diags}
}

// parseQuizPollBlock delimita el cuerpo del bloque y lo deserializa. Compartido
// por los dos tags: la única diferencia entre quiz y poll es qué llaves acepta
// y a qué nodo del AST va el resultado.
//
// Devuelve SIEMPRE un cuerpo utilizable (vacío si el YAML estaba roto) y el
// conteo de líneas consumidas, para que el llamador emita el elemento igual y
// el bloque no se reprocese como prosa suelta — mismo criterio que CHART002.
func parseQuizPollBlock(ctx *ParseContext, startIndex int, tag string) (quizPollBody, int, []diagnostics.Diagnostic) {
	var body quizPollBody
	var diags []diagnostics.Diagnostic
	pos := diagnostics.NewPosition(startIndex+1, 1)
	source := tag + "-parser"

	rawLines, consumed, closedBy := readQuizPollBody(ctx.Lines, startIndex, tag)

	// Un bloque que no se cerró explícitamente se REPORTA. Sin esto, todo lo
	// que hace terminar el bloque antes de tiempo —una frontera dentro de un
	// escalar de bloque, un dedent inesperado, el fin del archivo— quedaba sin
	// señal cuando el cuerpo alcanzaba a ser válido, y el autor solo veía un
	// contenido raro más adelante. `<<end>>` está exento del failsafe de flex,
	// así que este es el único lugar desde donde se puede avisar.
	if closedBy != closedByCloser {
		diags = append(diags, diagnostics.NewWarning(
			fmt.Sprintf("El bloque %s no se cerró con '<<end>>'; termina donde empieza el contenido siguiente y puede quedar incompleto", tag),
			pos, source).WithRuleID(quizPollRuleID(tag, "unclosed")))
	}

	yamlText := dedentBlock(rawLines)
	if strings.TrimSpace(yamlText) == "" {
		return body, consumed, diags
	}

	if err := yaml.Unmarshal([]byte(yamlText), &body); err != nil {
		// El bloque se consumió igual (ver el doc de la función). El elemento
		// sale vacío y el linter lo reporta como pregunta ausente / sin
		// opciones, así que el autor recibe las dos señales.
		diags = append(diags, diagnostics.NewWarning(
			fmt.Sprintf("El cuerpo YAML del %s es inválido y fue ignorado: %v", tag, err),
			pos, source).WithRuleID(quizPollRuleID(tag, "invalidYAML")))
		return quizPollBody{}, consumed, diags
	}

	if unknown := unknownQuizPollKeys(yamlText, tag); len(unknown) > 0 {
		diags = append(diags, diagnostics.NewWarning(
			fmt.Sprintf("Llave(s) no reconocida(s) en el bloque %s (%s); se esperaba %s — ignorada(s)",
				tag, strings.Join(unknown, ", "), strings.Join(sortedQuizPollKeys(tag), "/")),
			pos, source).WithRuleID(quizPollRuleID(tag, "unknownKey")))
	}

	return body, consumed, diags
}

// readQuizPollBody devuelve las líneas crudas del cuerpo y cuántas líneas
// consume el bloque entero (incluyendo el tag de apertura y, si aparece, el
// cierre).
//
// Deliberadamente NO usa parseYAMLBlock de chart.go: esa función gatea cada
// línea por ShouldProcessLine, que devuelve false cuando la PRIMERA línea está
// en columna 0 (ver AutoDetectIndentation en common.go) — y los dos decks
// originales del issue #198 escriben el cuerpo justo así, sin sangrar. Con ese
// gate el bloque consumiría cero líneas y el cuerpo entero caería como prosa.
func readQuizPollBody(lines []string, startIndex int, tag string) ([]string, int, quizPollCloser) {
	closer := "<</" + tag + ">>"
	raw := make([]string, 0, 8)
	consumed := 1 // el tag de apertura
	bodyIndent := -1
	closedBy := closedByEOF

	for i := startIndex + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Los cerradores van ANTES de la comprobación de frontera: los dos
		// empiezan por "<<", así que IsEmbeddedBlockBoundary los daría por
		// frontera y los dejaría sin consumir. `<</quiz>>` sin consumir sería
		// además un FLEX001 por línea (isFlexFailsafeExempt solo perdona
		// `<<end>>`), así que se consumen acá y ese failsafe no se toca.
		//
		// El cierre se decide POR LÍNEA, así que una línea que sea exactamente
		// `<<end>>` cierra el bloque aunque el YAML la considerara parte de un
		// escalar de bloque (`question: |`). Es la misma ambigüedad que tiene
		// chart —y cualquier terminador orientado a líneas— y se acepta porque
		// la alternativa (decidir la extensión con el parser de YAML) haría que
		// un cuerpo mal formado se tragara el resto del documento. El caso no
		// es silencioso: el bloque queda sin `options`, y QUIZ001/QUIZ002 lo
		// reportan con Error, así que el build se detiene.
		if trimmed == "<<end>>" || trimmed == closer {
			consumed++
			closedBy = closedByCloser
			break
		}

		if trimmed == "" {
			consumed++
			raw = append(raw, "")
			continue
		}

		indent := CalculateIndentLevel(line)

		// Frontera real: la línea es del documento. Se corta SIN consumir.
		//
		// El guard de sangría NO es cosmético. Sin él, una línea de
		// continuación de un escalar de bloque YAML se tomaba por frontera:
		//
		//	explanation: |
		//	  Ver el ejemplo
		//	  # Nota importante      <- se leía como heading del documento
		//	  mas texto
		//
		// El bloque se cortaba ahí, el `# Nota importante` abría un slide
		// fantasma y `mas texto` caía adentro como prosa. Y era SILENCIOSO:
		// el quiz ya tenía `options` y `answer`, así que ningún diagnóstico
		// disparaba — la pérdida de contenido del issue #192, exacta.
		//
		// Una frontera de verdad nunca está más sangrada que el cuerpo:
		// IsStrictBlockBoundary ya exige columna 0 para SLIDE/SECTION, y un
		// `# `/`---` de nivel de documento tampoco va sangrado. Así que
		// cualquier línea más profunda que bodyIndent es contenido, no
		// frontera.
		if indent <= bodyIndent || bodyIndent == -1 {
			if IsEmbeddedBlockBoundary(line) {
				closedBy = closedByBoundary
				break
			}
		}

		// bodyIndent se fija con la primera línea de contenido y solo sirve
		// para detectar el dedent que cierra un bloque sin cerrador explícito
		// (el caso de strict, donde el cuerpo va sangrado bajo el tag). Con el
		// cuerpo en columna 0 queda en 0 y la comparación nunca dispara: el
		// bloque termina solo por cerrador o por frontera, que es lo correcto.
		if bodyIndent == -1 {
			bodyIndent = indent
		} else if indent < bodyIndent {
			closedBy = closedByDedent
			break
		}

		raw = append(raw, line)
		consumed++
	}

	return raw, consumed, closedBy
}

// closedByX indica CÓMO terminó el bloque. Solo el cerrador explícito es un
// final limpio; los otros tres significan que el autor no cerró el bloque, y
// eso se reporta en vez de deducirse en silencio.
type quizPollCloser int

const (
	closedByEOF quizPollCloser = iota
	closedByCloser
	closedByBoundary
	closedByDedent
)

// dedentBlock quita la sangría común de las líneas para que yaml.v3 las acepte:
// un documento YAML no puede arrancar con indentación. Los tabs iniciales se
// expanden a dos espacios porque YAML los rechaza como indentación.
func dedentBlock(lines []string) string {
	minIndent := -1
	expanded := make([]string, 0, len(lines))
	for _, line := range lines {
		line = expandLeadingTabs(line)
		expanded = append(expanded, line)
		if strings.TrimSpace(line) == "" {
			continue
		}
		if n := countLeadingSpaces(line); minIndent == -1 || n < minIndent {
			minIndent = n
		}
	}
	if minIndent <= 0 {
		return strings.Join(expanded, "\n")
	}

	out := make([]string, 0, len(expanded))
	for _, line := range expanded {
		if len(line) >= minIndent {
			line = line[minIndent:]
		} else {
			line = strings.TrimLeft(line, " ")
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func expandLeadingTabs(line string) string {
	i := 0
	for i < len(line) && line[i] == '\t' {
		i++
	}
	if i == 0 {
		return line
	}
	return strings.Repeat("  ", i) + line[i:]
}

// unknownQuizPollKeys reporta las llaves de nivel superior que el tag no
// reconoce. yaml.v3 ignora en silencio cualquier campo sin destino en el
// struct, así que hace falta una segunda pasada a un mapa para verlas — mismo
// patrón que CHART005.
func unknownQuizPollKeys(yamlText, tag string) []string {
	var generic map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlText), &generic); err != nil {
		return nil
	}
	known := quizPollKeys[tag]
	var unknown []string
	for key := range generic {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	return unknown
}

func sortedQuizPollKeys(tag string) []string {
	keys := make([]string, 0, len(quizPollKeys[tag]))
	for key := range quizPollKeys[tag] {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// quizPollRuleID mapea (tag, motivo) al ID público del diagnóstico. Los IDs se
// declaran acá y no en el sitio de emisión para que la lista de
// core/linter/policy.go (que permite habilitarlos/deshabilitarlos por ID) tenga
// una sola fuente que leer.
func quizPollRuleID(tag, reason string) string {
	ids := map[string]map[string]string{
		"quiz": {"invalidYAML": "QUIZ004", "unknownKey": "QUIZ005", "unclosed": "QUIZ006"},
		"poll": {"invalidYAML": "POLL003", "unknownKey": "POLL004", "unclosed": "POLL005"},
	}
	if id, ok := ids[tag][reason]; ok {
		return id
	}
	// Un mapa exhaustivo en vez de un switch con default: el default anterior
	// devolvía POLL004 para CUALQUIER combinación no contemplada, así que un
	// motivo nuevo del lado de quiz habría salido etiquetado como regla de
	// poll. Acá una combinación desconocida es visiblemente eso.
	return "QUIZPOLL_UNKNOWN"
}
