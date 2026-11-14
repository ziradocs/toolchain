// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Regla del dialecto strict: toda línea no vacía que un elemento toma termina
// en su contenido, en una propiedad que el elemento de verdad lee, o en un
// aviso anclado en esa línea que cite su texto y diga por qué no se leyó.
// Estos IDs son los avisos de las líneas que un elemento consumía sin leer.
// Son warnings y no errores: los documentos que hoy compilan siguen
// compilando, y el AST no cambia; lo único nuevo es que el texto perdido se
// nombra. El mismo criterio que CHECKLIST001 y TABLE007.
const (
	// KeywordLineRestRuleID: texto en la misma línea que la palabra clave de un
	// elemento (`TEXT hola`, `POINTS: uno`, `TEXTO citado`, un tercer argumento
	// de IMAGE) o de un SLIDE (`SLIDE content extra`) que nadie lee.
	KeywordLineRestRuleID = "STRICT004"
	// PointsLineRuleID: una línea del cuerpo de un POINTS strict que no es un
	// item.
	PointsLineRuleID = "POINTS001"
	// ImageLineRuleID: una línea del cuerpo de un IMAGE strict que no es una
	// propiedad que IMAGE lea.
	ImageLineRuleID = "IMG005"
	// TableLineRuleID: una línea del cuerpo de un TABLE que no es una fila ni
	// una propiedad que TABLE lea, o una fila Markdown strict sin "|" al
	// final.
	TableLineRuleID = "TABLE008"
	// ChartLineRuleID: una línea del cuerpo de un <<chart>> que el chart
	// consume sin leer.
	ChartLineRuleID = "CHART007"
	// MetricLineRuleID, QuizLineRuleID y PollLineRuleID: una línea de una
	// llave que el bloque no conoce. El aviso por bloque (METRIC003, QUIZ005,
	// POLL004) sigue igual; este nombra cada línea.
	MetricLineRuleID = "METRIC006"
	QuizLineRuleID   = "QUIZ007"
	PollLineRuleID   = "POLL007"
	// QuoteLineRuleID: un AUTHOR: o SOURCE: repetido en un QUOTE strict, cuyo
	// valor anterior se pierde.
	QuoteLineRuleID = "QUOTE001"
	// MathLineRuleID: un caption: o label: repetido en un <<math>>, cuyo valor
	// anterior se pierde. Mismo criterio que QUOTE001; en TABLE, IMAGE y
	// <<chart>> la propiedad repetida es TABLE008, IMG005 y CHART007.
	MathLineRuleID = "MATH001"
	// CodeGroupLineRuleID: una línea fuera de las cercas de un :::code-group
	// después de la primera, que ya es un error.
	CodeGroupLineRuleID = "CODEGROUP003"
	// DirectiveTextRuleID: texto de una directiva que no llega a ningún
	// parámetro (`@transition fade`, `@` sola), y un parámetro repetido que
	// otro posterior reemplaza (`type="a" type="b"`).
	DirectiveTextRuleID = "DIRECTIVE004"
	// ContainerPropertyRuleID: una propiedad de un SLIDE (title:, heading:,
	// subtitle:, kicker:, logo:, una opción del layout) o de un SECTION
	// (level:, id:), en documento o dentro de un SLIDE, que otra posterior
	// reemplaza.
	ContainerPropertyRuleID = "STRICT005"
	// MapLineRuleID: una propiedad de un <<map>> (type:, zoom:, center:,
	// title:…) o de un marcador (lng:, label:, value:…) que otra posterior
	// reemplaza.
	MapLineRuleID = "MAP003"
)

// ignoredLineWarning arma el aviso de una línea que el elemento consumió sin
// leer: la posición es la de esa línea y el mensaje cita su texto.
func ignoredLineWarning(ctx *ParseContext, i int, ruleID, source, format string, args ...any) diagnostics.Diagnostic {
	return diagnostics.NewWarning(fmt.Sprintf(format, args...), ctx.Position(i), source).WithRuleID(ruleID)
}

// withLineRange extiende el diagnóstico hasta la línea end (índice en
// ctx.Lines), para un aviso que cubre un tramo: el valor de una llave que no
// se lee, un cuerpo YAML que se ignoró entero.
func withLineRange(ctx *ParseContext, d diagnostics.Diagnostic, end int) diagnostics.Diagnostic {
	if pos := ctx.Position(end); pos.Line > d.Position.Line {
		d.EndPosition = &pos
	}
	return d
}

// repeatedKeys lleva, por propiedad, el tramo de líneas (índices en
// ctx.Lines) de la última vez que un elemento la leyó. Una propiedad repetida
// pisa a la anterior, como siempre; la anterior se avisa porque su texto ya
// no llega al AST (el criterio de QUOTE001). Solo se registra lo que de
// verdad se leyó: un valor que el elemento descarta (con su propio aviso) no
// reemplaza a nadie.
type repeatedKeys struct {
	ctx                     *ParseContext
	ruleID, source, element string
	last                    map[string][2]int
	diags                   []diagnostics.Diagnostic
}

// newRepeatedKeys arma el registro de un elemento; element es su nombre en el
// mensaje ("TABLE", "<<math>> block").
func newRepeatedKeys(ctx *ParseContext, ruleID, source, element string) *repeatedKeys {
	return &repeatedKeys{ctx: ctx, ruleID: ruleID, source: source, element: element, last: map[string][2]int{}}
}

// read registra que la propiedad key, escrita de la línea start a la end, se
// leyó. Si ya se había leído, avisa la anterior.
func (r *repeatedKeys) read(key string, start, end int) {
	for end > start && strings.TrimSpace(r.ctx.Lines[end]) == "" {
		end--
	}
	if prev, ok := r.last[key]; ok {
		earlier := strings.TrimSpace(r.ctx.Lines[prev[0]])
		later := strings.TrimSpace(r.ctx.Lines[start])
		d := ignoredLineWarning(r.ctx, prev[0], r.ruleID, r.source,
			"This %s: line of the %s was ignored: %q. The later %q replaces it; only the last %s: is read.",
			key, r.element, earlier, later, key)
		if prev[1] > prev[0] {
			d = withLineRange(r.ctx, d, prev[1])
			d.Message = fmt.Sprintf("This %s: line of the %s and the lines of its value below it were ignored: %q. The later %q replaces it; only the last %s: is read.",
				key, r.element, earlier, later, key)
		}
		r.diags = append(r.diags, d)
	}
	r.last[key] = [2]int{start, end}
}

// extend lleva hasta end el tramo de la lectura de key que empezó en la línea
// start: el valor sigue en las líneas de abajo (un arreglo de varias líneas).
// Si esa línea no fue la última lectura de key, no hace nada.
func (r *repeatedKeys) extend(key string, start, end int) {
	if prev, ok := r.last[key]; ok && prev[0] == start && end > prev[1] {
		r.last[key] = [2]int{start, end}
	}
}

// keywordLineRest es lo que sigue a la palabra clave kw en la cabecera de un
// elemento strict (trimmed, ya sin sangría), y glued dice si ese resto empieza
// pegado a la palabra clave ("TEXTO" se despacha como TEXT por prefijo). Un
// resto vacío, o solo `{verbatim}` para CODE, no es texto perdido.
func keywordLineRest(trimmed, kw string) (rest string, glued bool) {
	after, ok := strings.CutPrefix(trimmed, kw)
	if !ok {
		return "", false
	}
	// `TEXT: hola` es `TEXT hola` con dos puntos: lo perdido es "hola", y un
	// ":" solo (`POINTS:`, `TEXT:`) no es texto perdido.
	if colon, ok := strings.CutPrefix(after, ":"); ok {
		return strings.TrimSpace(colon), false
	}
	if after == "" {
		return "", false
	}
	return strings.TrimSpace(after), after[0] != ' ' && after[0] != '\t'
}

// keywordLineRef es el patrón de un \ref{label}, el mismo que resuelve
// core/xref (refPattern).
var keywordLineRef = regexp.MustCompile(`\\ref\{([^}]+)\}`)

// keywordLineText decide qué pasa con el texto de la línea de un TEXT o un
// QUOTE strict (trimmed es la línea ya sin sangría, kw la palabra clave). Si
// el elemento lo lee como la primera línea de su contenido, devuelve ese
// texto y fix vacío. Si no lo lee, devuelve "" y, en fix, la indicación del
// STRICT004 que lo reporta. Sin texto, devuelve "" y fix vacío.
//
// No se lee:
//   - lo pegado a la palabra clave (`TEXTO citado`): el despacho por prefijo
//     la toma por TEXT, pero no es un TEXT bien escrito;
//   - un \ref: el texto de esta línea antes no llegaba al AST, y un \ref
//     sin resolver hace fallar el build, así que leerlo haría fallar
//     documentos que compilaban;
//   - un `<!-- node-id: … -->` o un `@include …`: en su propia línea son
//     directivas que se quitan antes de parsear, así que fmt no podría
//     escribirlos en el cuerpo y que se volvieran a leer igual;
//   - en QUOTE, lo que el cuerpo de la cita no leería como texto suyo
//     (quoteHeaderText).
//
// Es la única regla: la usan TextParser, QuoteParser y
// OpensElementAfterEmptyText.
func keywordLineText(trimmed, kw string) (text, fix string) {
	rest, glued := keywordLineRest(trimmed, kw)
	switch {
	case rest == "":
		return "", ""
	case glued:
		return "", fmt.Sprintf("%s reads the text on its line only after a space or a colon (%s hola): write it that way, or on the indented lines below it", kw, kw)
	case keywordLineRef.MatchString(rest):
		return "", fmt.Sprintf("A \\ref on the %s line is not read: write the text on the indented lines below it", kw)
	case strings.HasPrefix(rest, "<!-- node-id:") || rest == "@include" || strings.HasPrefix(rest, "@include "):
		return "", fmt.Sprintf("On a line of its own this text is a node-id or @include directive, not text, so %s does not read it on its line either", kw)
	case kw == "QUOTE" && !quoteHeaderText(rest):
		return "", "The text on the QUOTE line is read as the first line of the quote only when the quote body would read it as quote text, not as AUTHOR:, SOURCE:, the end of the quote or another element: write it on its own indented line below"
	}
	return rest, ""
}

// keywordLineTextWarning es el STRICT004 del texto de la línea de un TEXT o
// un QUOTE que el elemento no lee (ver keywordLineText).
func keywordLineTextWarning(ctx *ParseContext, i int, kw, source, fix string) diagnostics.Diagnostic {
	rest, glued := keywordLineRest(strings.TrimSpace(ctx.Lines[i]), kw)
	return keywordRestDiagnostic(ctx, i, kw, source, rest, glued, fix)
}

// keywordLineRestWarning es el STRICT004 de un elemento cuya cabecera no lleva
// argumentos (POINTS, CHECKLIST, TABLE): todo lo que sigue a la palabra clave
// en esa línea se descarta. Devuelve nil si no hay resto.
func keywordLineRestWarning(ctx *ParseContext, i int, kw, source string) *diagnostics.Diagnostic {
	rest, glued := keywordLineRest(strings.TrimSpace(ctx.Lines[i]), kw)
	if rest == "" {
		return nil
	}
	d := keywordRestDiagnostic(ctx, i, kw, source, rest, glued,
		fmt.Sprintf("%s takes nothing on its own line: write the content on the indented lines below it", kw))
	return &d
}

// keywordRestDiagnostic es el texto común de STRICT004.
func keywordRestDiagnostic(ctx *ParseContext, i int, kw, source, rest string, glued bool, fix string) diagnostics.Diagnostic {
	if glued {
		return ignoredLineWarning(ctx, i, KeywordLineRestRuleID, source,
			"This line is read as the keyword %s, and the rest of the line, glued to it, was ignored: %q. %s.",
			kw, rest, fix)
	}
	return ignoredLineWarning(ctx, i, KeywordLineRestRuleID, source,
		"The text after %s on this line was ignored: %q. %s.", kw, rest, fix)
}

// lastBodyLine es el índice, en ctx.Lines, de la última línea no vacía de un
// cuerpo cuyas líneas raw empiezan en ctx.Lines[first]. -1 si no hay ninguna.
func lastBodyLine(raw []string, first int) int {
	for k := len(raw) - 1; k >= 0; k-- {
		if strings.TrimSpace(raw[k]) != "" {
			return first + k
		}
	}
	return -1
}

// unknownYAMLKeyLines devuelve un aviso por llave de nivel superior que el
// bloque no lee, anclado en la línea de la llave y extendido hasta la última
// línea de su valor. yamlText es el cuerpo ya dedentado y su línea 1 es
// ctx.Lines[first]: dedentBlock no agrega ni quita líneas. known dice qué
// llaves lee el bloque; tag es el nombre del bloque para el mensaje.
//
// Una sola pasada de yaml.Node: las líneas de cada llave salen de la línea de
// la llave siguiente, sin volver a recorrer el cuerpo por cada una. (Decodificar
// a un yaml.Node no pasa por la comparación de llaves duplicadas de yaml.v3,
// que es cuadrática en el número de llaves.)
func unknownYAMLKeyLines(ctx *ParseContext, yamlText string, first int, known map[string]bool, tag, ruleID, source string) []diagnostics.Diagnostic {
	root, lines := yamlMappingRoot(yamlText)
	if root == nil {
		return nil
	}
	return mappingKeyLines(ctx, root, lines, first, len(lines)-1, known, tag, ruleID, source)
}

// duplicateYAMLKeyLines devuelve un aviso por llave de nivel superior que el
// cuerpo YAML de un bloque repite, en la línea de cada aparición anterior y
// citando la siguiente por su texto. yaml.v3 rechaza el cuerpo entero por una
// llave repetida (el aviso de YAML inválido del bloque cubre todo el cuerpo):
// ningún valor gana, y este aviso nombra la línea que lo causó. yamlText y
// first como en unknownYAMLKeyLines.
func duplicateYAMLKeyLines(ctx *ParseContext, yamlText string, first int, tag, ruleID, source string) []diagnostics.Diagnostic {
	root, lines := yamlMappingRoot(yamlText)
	if root == nil {
		return nil
	}
	seen := map[string]*yaml.Node{}
	var out []diagnostics.Diagnostic
	for n := 0; n+1 < len(root.Content); n += 2 {
		key := root.Content[n]
		prev, ok := seen[key.Value]
		seen[key.Value] = key
		if !ok || prev.Line < 1 || prev.Line > len(lines) || key.Line > len(lines) {
			continue
		}
		earlier := strings.TrimSpace(lines[prev.Line-1])
		if prev.Line == key.Line {
			out = append(out, ignoredLineWarning(ctx, first+prev.Line-1, ruleID, source,
				"This line of the %s block has the key %q twice: %q. A key can appear only once, so the whole body of the block was ignored.",
				tag, key.Value, earlier))
			continue
		}
		out = append(out, ignoredLineWarning(ctx, first+prev.Line-1, ruleID, source,
			"This %s: line of the %s block is written again below as %q: %q. A key can appear only once, so the whole body of the block was ignored.",
			key.Value, tag, strings.TrimSpace(lines[key.Line-1]), earlier))
	}
	return out
}

// duplicateJSONKeyLines devuelve un aviso por llave que un objeto del cuerpo
// JSON de un bloque repite, en la línea de la aparición anterior y citando la
// siguiente por su texto. El cuerpo va de ctx.Lines[first] a ctx.Lines[last]
// (el cierre del bloque, si está, no estorba: el lector se detiene al terminar
// el valor). json.Valid acepta las llaves repetidas y quien lee el JSON se
// queda con la última, así que la anterior no llega a la salida.
func duplicateJSONKeyLines(ctx *ParseContext, first, last int, tag, ruleID, source string) []diagnostics.Diagnostic {
	if last >= len(ctx.Lines) {
		last = len(ctx.Lines) - 1
	}
	if first > last {
		return nil
	}
	text := strings.Join(ctx.Lines[first:last+1], "\n")
	type frame struct {
		object, expectKey bool
		keys              map[string]int
	}
	var stack []*frame
	valueDone := func() {
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].expectKey = true
		}
	}
	var out []diagnostics.Diagnostic
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				stack = append(stack, &frame{object: true, expectKey: true, keys: map[string]int{}})
			case '[':
				stack = append(stack, &frame{})
			default:
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				valueDone()
				if len(stack) == 0 {
					return out
				}
			}
			continue
		case string:
			if n := len(stack); n > 0 && stack[n-1].object && stack[n-1].expectKey {
				top := stack[n-1]
				top.expectKey = false
				line := first + strings.Count(text[:dec.InputOffset()], "\n")
				if prev, ok := top.keys[t]; ok {
					out = append(out, repeatedJSONKey(ctx, prev, line, t, tag, ruleID, source))
				}
				top.keys[t] = line
				continue
			}
		}
		valueDone()
	}
	return out
}

// repeatedJSONKey es el aviso de la llave key del JSON en la línea earlier
// que la de la línea later reemplaza.
func repeatedJSONKey(ctx *ParseContext, earlier, later int, key, tag, ruleID, source string) diagnostics.Diagnostic {
	if earlier == later {
		return ignoredLineWarning(ctx, earlier, ruleID, source,
			"An object on this line of the %s's JSON has the key %q twice: %q. Only the last one is read.",
			tag, key, strings.TrimSpace(ctx.Lines[earlier]))
	}
	return ignoredLineWarning(ctx, earlier, ruleID, source,
		"The %q key on this line of the %s's JSON was ignored: %q. The later %q replaces it; only the last one is read.",
		key, tag, strings.TrimSpace(ctx.Lines[earlier]), strings.TrimSpace(ctx.Lines[later]))
}

// yamlMappingRoot decodifica yamlText a un yaml.Node y devuelve su mapeo de
// nivel superior (nil si no lo es) y sus líneas.
func yamlMappingRoot(yamlText string) (*yaml.Node, []string) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlText), &doc); err != nil || len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil
	}
	return root, strings.Split(yamlText, "\n")
}

// mappingKeyLines es unknownYAMLKeyLines para un mapeo cualquiera del
// documento: limit es el índice (en lines) de la última línea que puede
// pertenecer al valor de su última llave.
func mappingKeyLines(ctx *ParseContext, m *yaml.Node, lines []string, first, limit int, known map[string]bool, tag, ruleID, source string) []diagnostics.Diagnostic {
	knownNames := make([]string, 0, len(known))
	for k := range known {
		knownNames = append(knownNames, k)
	}
	sort.Strings(knownNames)
	reads := strings.Join(knownNames, ", ")
	// keysOnLine cuenta las llaves por línea, en una pasada: una llave que
	// comparte su línea con otra está en un mapeo de una sola línea.
	keysOnLine := make(map[int]int, len(m.Content)/2)
	for n := 0; n+1 < len(m.Content); n += 2 {
		keysOnLine[m.Content[n].Line]++
	}
	var out []diagnostics.Diagnostic
	for n := 0; n+1 < len(m.Content); n += 2 {
		key := m.Content[n]
		if known[key.Value] {
			continue
		}
		start := key.Line - 1
		end := limit
		if n+2 < len(m.Content) {
			end = m.Content[n+2].Line - 2
		}
		for end > start && strings.TrimSpace(lines[end]) == "" {
			end--
		}
		if start < 0 || start >= len(lines) {
			continue
		}
		if keysOnLine[key.Line] > 1 {
			// La llave comparte la línea con otras que sí se leen (un mapeo
			// en una sola línea, `{question: …, zeta: 1}`): lo ignorado es la
			// llave, no la línea.
			out = append(out, ignoredLineWarning(ctx, first+start, ruleID, source,
				"The %s block has no %q key, so that key and its value were ignored; the rest of this line is read: %q. The block reads %s.",
				tag, key.Value, strings.TrimSpace(lines[start]), reads))
			continue
		}
		d := ignoredLineWarning(ctx, first+start, ruleID, source,
			"The %s block has no %q key, so this line was ignored: %q. The block reads %s.",
			tag, key.Value, strings.TrimSpace(lines[start]), reads)
		if end > start {
			d = withLineRange(ctx, d, first+end)
			d.Message = fmt.Sprintf("The %s block has no %q key, so this line and the lines of its value below it were ignored: %q. The block reads %s.",
				tag, key.Value, strings.TrimSpace(lines[start]), reads)
		}
		out = append(out, d)
	}
	return out
}

// yamlDiscardedTail busca, en un bloque YAML ya decodificado a root, las
// líneas que yaml.v3 dejó afuera sin error: cuando la colección de nivel
// superior empieza más sangrada que lo que la sigue ("    -" y debajo
// "  - [...]", o un "---" a la sangría de la llave), el lector termina el
// documento antes de esas líneas y las descarta. lines son las líneas del
// bloque tal como se decodificaron (sin dedentar). Devuelve los índices (en
// lines) de la primera y la última línea no vacía descartadas, o ok=false si
// no hay ninguna.
//
// Se decide desde el nodo, no desde la sangría del texto: el fin del documento
// es la última línea que cubre el último nodo (ver yamlNodeEnd), así que una
// fila de flujo o un escalar entre comillas partidos en varias líneas, con
// cualquier sangría, quedan dentro. Además, lo descartado está siempre menos
// sangrado que la colección de nivel superior (si no, yaml.v3 lo leería o daría
// error), y se exige también: un fin mal calculado no puede producir un aviso
// sobre una línea que sí se leyó.
func yamlDiscardedTail(lines []string, root *yaml.Node) (start, end int, ok bool) {
	if root == nil || root.Line < 1 {
		return 0, 0, false
	}
	rootIndent := root.Column - 1
	last := yamlNodeEnd(lines, root)
	start = -1
	for k := last + 1; k < len(lines); k++ {
		trimmed := strings.TrimSpace(lines[k])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if start < 0 {
			if CalculateIndentLevel(lines[k]) >= rootIndent {
				continue
			}
			start = k
		}
		end = k
	}
	return start, end, start >= 0
}

// yamlNodeEnd es el índice (en lines) de la última línea que ocupa root. yaml.v3
// solo da la posición donde empieza cada nodo, así que el fin se calcula sobre
// el texto a partir del último nodo hoja:
//
//   - si el último nodo cuelga de una colección de flujo, el fin es el cierre
//     (`]` o `}`) de la colección de flujo más externa;
//   - un escalar entre comillas termina en su comilla de cierre;
//   - uno de bloque (`|`, `>`) o uno plano de varias líneas sigue mientras las
//     líneas estén más sangradas que su llave o su guion.
//
// Si el texto no se deja leer así (un cierre que no aparece), devuelve la última
// línea: no hay cola que avisar.
func yamlNodeEnd(lines []string, root *yaml.Node) int {
	lastLine := len(lines) - 1
	// El camino hasta la última hoja, con el padre de cada nodo.
	path := []*yaml.Node{root}
	for n := root; len(n.Content) > 0; {
		n = n.Content[len(n.Content)-1]
		path = append(path, n)
	}
	for _, n := range path {
		if (n.Kind == yaml.SequenceNode || n.Kind == yaml.MappingNode) && n.Style&yaml.FlowStyle != 0 {
			if l, ok := flowCloseLine(lines, n.Line-1, n.Column-1); ok {
				return l
			}
			return lastLine
		}
	}
	leaf := path[len(path)-1]
	start := leaf.Line - 1
	if start < 0 || start > lastLine {
		return lastLine
	}
	switch {
	case leaf.Kind == yaml.ScalarNode && leaf.Style&yaml.DoubleQuotedStyle != 0:
		if l, ok := quoteCloseLine(lines, start, leaf.Column-1, '"'); ok {
			return l
		}
		return lastLine
	case leaf.Kind == yaml.ScalarNode && leaf.Style&yaml.SingleQuotedStyle != 0:
		if l, ok := quoteCloseLine(lines, start, leaf.Column-1, '\''); ok {
			return l
		}
		return lastLine
	}
	// Escalar plano, de bloque o un nodo vacío: las líneas que siguen y están
	// más sangradas que el dueño del valor (la llave o el guion) son suyas.
	owner := root
	if len(path) >= 2 {
		parent := path[len(path)-2]
		owner = parent
		if parent.Kind == yaml.MappingNode && len(parent.Content) >= 2 {
			owner = parent.Content[len(parent.Content)-2] // la llave
		}
	}
	ownerIndent := owner.Column - 1
	end := start
	for k := start + 1; k <= lastLine; k++ {
		trimmed := strings.TrimSpace(lines[k])
		if trimmed == "" {
			continue
		}
		if CalculateIndentLevel(lines[k]) <= ownerIndent {
			break
		}
		end = k
	}
	return end
}

// flowCloseLine busca, desde lines[line][col] (un `[` o `{`), la línea de su
// cierre, saltando comillas y comentarios.
func flowCloseLine(lines []string, line, col int) (int, bool) {
	depth := 0
	var quote byte
	for l := line; l < len(lines); l++ {
		s := lines[l]
		c := 0
		if l == line {
			c = col
		}
		for ; c < len(s); c++ {
			ch := s[c]
			switch {
			case quote == '"':
				switch ch {
				case '\\':
					c++
				case '"':
					quote = 0
				}
			case quote == '\'':
				if ch == '\'' {
					if c+1 < len(s) && s[c+1] == '\'' {
						c++
					} else {
						quote = 0
					}
				}
			case (ch == '"' || ch == '\'') && startsFlowScalar(s, c):
				quote = ch
			case ch == '#' && (c == 0 || s[c-1] == ' ' || s[c-1] == '\t'):
				c = len(s)
			case ch == '[' || ch == '{':
				depth++
			case ch == ']' || ch == '}':
				depth--
				if depth == 0 {
					return l, true
				}
			}
		}
	}
	return 0, false
}

// startsFlowScalar dice si la comilla en s[c] abre un escalar entre comillas
// dentro de una colección de flujo: solo si es el primer carácter del escalar,
// o sea, si antes de ella (saltando espacios) está el principio de la línea, `[`,
// `{`, `,` o `:`. Una comilla dentro de un escalar plano (`Don't`, `5" screen`,
// `rock 'n roll`) es parte de ese escalar.
func startsFlowScalar(s string, c int) bool {
	k := c - 1
	for k >= 0 && (s[k] == ' ' || s[k] == '\t') {
		k--
	}
	if k < 0 {
		return true
	}
	switch s[k] {
	case '[', '{', ',', ':':
		return true
	}
	return false
}

// quoteCloseLine busca, desde la comilla de apertura en lines[line][col], la
// línea de la comilla que cierra el escalar.
func quoteCloseLine(lines []string, line, col int, q byte) (int, bool) {
	for l := line; l < len(lines); l++ {
		s := lines[l]
		c := 0
		if l == line {
			c = col + 1
		}
		for ; c < len(s); c++ {
			switch {
			case q == '"' && s[c] == '\\':
				c++
			case s[c] == q && q == '\'' && c+1 < len(s) && s[c+1] == '\'':
				c++
			case s[c] == q:
				return l, true
			}
		}
	}
	return 0, false
}
