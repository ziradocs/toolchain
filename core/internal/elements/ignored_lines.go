// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
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
	// CodeGroupLineRuleID: una línea fuera de las cercas de un :::code-group
	// después de la primera, que ya es un error.
	CodeGroupLineRuleID = "CODEGROUP003"
	// DirectiveTextRuleID: texto de una directiva que no llega a ningún
	// parámetro (`@transition fade`, `@` sola).
	DirectiveTextRuleID = "DIRECTIVE004"
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

// keywordLineRestWarning es el STRICT004 de un elemento cuya cabecera no lleva
// argumentos (TEXT, POINTS, QUOTE, CHECKLIST, TABLE): todo lo que sigue a la
// palabra clave en esa línea se descarta. Devuelve nil si no hay resto.
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
				if ch == '\\' {
					c++
				} else if ch == '"' {
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
			case ch == '"' || ch == '\'':
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
