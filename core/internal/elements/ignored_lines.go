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
// la llave siguiente, sin volver a recorrer el cuerpo por cada una.
func unknownYAMLKeyLines(ctx *ParseContext, yamlText string, first int, known map[string]bool, tag, ruleID, source string) []diagnostics.Diagnostic {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlText), &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	lines := strings.Split(yamlText, "\n")
	knownNames := make([]string, 0, len(known))
	for k := range known {
		knownNames = append(knownNames, k)
	}
	sort.Strings(knownNames)
	var out []diagnostics.Diagnostic
	for n := 0; n+1 < len(root.Content); n += 2 {
		key := root.Content[n]
		if known[key.Value] {
			continue
		}
		start := key.Line - 1
		end := len(lines) - 1
		if n+2 < len(root.Content) {
			end = root.Content[n+2].Line - 2
		}
		for end > start && strings.TrimSpace(lines[end]) == "" {
			end--
		}
		if start < 0 || start >= len(lines) {
			continue
		}
		d := ignoredLineWarning(ctx, first+start, ruleID, source,
			"The %s block has no %q key, so this line was ignored: %q. The block reads %s.",
			tag, key.Value, strings.TrimSpace(lines[start]), strings.Join(knownNames, ", "))
		if end > start {
			d = withLineRange(ctx, d, first+end)
			d.Message = fmt.Sprintf("The %s block has no %q key, so this line and its value, through line %d, were ignored: %q. The block reads %s.",
				tag, key.Value, d.EndPosition.Line, strings.TrimSpace(lines[start]), strings.Join(knownNames, ", "))
		}
		out = append(out, d)
	}
	return out
}
