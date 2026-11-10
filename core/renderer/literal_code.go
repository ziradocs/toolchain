// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
)

// literalCodeMarker es el valor que marca un mapa de variables con la opción de
// código literal (RenderContext.LiteralCode, InlineHTMLOptions.LiteralCode).
// Es un tipo no exportado: fuera de este paquete nadie puede construirlo, así
// que la marca no se puede poner ni falsificar desde un frontmatter ni desde
// otro paquete.
type literalCodeMarker struct{}

// literalCodeKey es la clave de la marca. No es UTF-8 válido, así que ningún
// frontmatter (que el parser de YAML exige en UTF-8) produce esa clave, y el
// patrón de {{nombre}} tampoco la puede nombrar.
const literalCodeKey = "\xffliteral-code"

// withLiteralCode devuelve una copia de variables marcada con la opción de
// código literal. La opción viaja así con el mapa por todas las funciones de
// render y de población del AST, sin cambiar sus firmas: las que sustituyen
// prosa (proseVariables y sus variantes) y cuerpos de código (codeBody) la
// consultan. Un mapa nil queda nil: sin variables no se sustituye nada y todo
// ya es literal.
func withLiteralCode(variables map[string]interface{}) map[string]interface{} {
	if variables == nil || isLiteralCode(variables) {
		return variables
	}
	marked := make(map[string]interface{}, len(variables)+1)
	for k, v := range variables {
		marked[k] = v
	}
	marked[literalCodeKey] = literalCodeMarker{}
	return marked
}

// isLiteralCode dice si variables lleva la marca de withLiteralCode.
func isLiteralCode(variables map[string]interface{}) bool {
	_, ok := variables[literalCodeKey].(literalCodeMarker)
	return ok
}

// codeBody es el cuerpo de un bloque de código listo para escapar: literal con
// la opción de código literal (issue #487), con `{{nombre}}` sustituido si no.
func codeBody(content string, variables map[string]interface{}) string {
	if isLiteralCode(variables) {
		return content
	}
	return ProcessVariables(content, variables)
}

// proseLayout es cómo parte un camino de render un texto de prosa en las
// unidades que le pasa enteras al Markdown inline. Los code spans se emparejan
// dentro de cada unidad, así que la sustitución que respeta el código tiene
// que partir el texto igual que el camino que lo va a pintar.
type proseLayout int

const (
	// layoutLines: cada línea es una unidad (ProcessInlineMarkdownSecure,
	// ProcessInlineMarkdownSecureMultiline).
	layoutLines proseLayout = iota
	// layoutWhole: el texto entero es una unidad
	// (ProcessInlineMarkdownSecureLine: encabezados, títulos).
	layoutWhole
	// layoutLists: la prosa de una cita o un ::: (renderProseWithLists), donde
	// un item de lista junta en una unidad sus líneas de continuación.
	layoutLists
)

// proseVariables sustituye `{{nombre}}` en un texto de prosa que después pasa
// por ProcessInlineMarkdownSecure (línea por línea). Sin la opción de código
// literal es ProcessVariables. Con ella no toca lo que es código en la
// fuente: los code spans, emparejados en cada línea como los empareja el
// renderer, y, en un texto de varias líneas, las cercas de código escritas
// adentro (ver proseFenceOpen). En un texto de una sola línea no hay cerca.
func proseVariables(text string, variables map[string]interface{}) string {
	return proseVariablesIn(text, variables, layoutLines)
}

// proseLineVariables es proseVariables para un texto que se pinta entero como
// una sola unidad (ProcessInlineMarkdownSecureLine): un encabezado, un título.
func proseLineVariables(text string, variables map[string]interface{}) string {
	return proseVariablesIn(text, variables, layoutWhole)
}

// proseListVariables es proseVariables para la prosa que pasa por
// renderProseWithLists: los code spans de un item de lista se emparejan sobre
// el item ya juntado con sus líneas de continuación, como lo pinta el renderer.
func proseListVariables(text string, variables map[string]interface{}) string {
	return proseVariablesIn(text, variables, layoutLists)
}

func proseVariablesIn(text string, variables map[string]interface{}, layout proseLayout) string {
	if !isLiteralCode(variables) || !strings.Contains(text, "{{") {
		return ProcessVariables(text, variables)
	}
	return substituteOutside(text, literalCodeRanges(text, layout), func(s string) string {
		return ProcessVariables(s, variables)
	}, func(s string) string { return s })
}

// proseLineVariablesSecure es ProcessVariablesSecure (escapa el texto y los
// valores, sin Markdown) para un título de bloque, que se escribe en una sola
// unidad: con la opción de código literal no sustituye dentro de sus code
// spans, igual que un encabezado `##` (proseLineVariables).
func proseLineVariablesSecure(text string, variables map[string]interface{}) string {
	if !isLiteralCode(variables) || !strings.Contains(text, "{{") {
		return ProcessVariablesSecure(text, variables)
	}
	return substituteOutside(text, literalCodeRanges(text, layoutWhole), func(s string) string {
		return ProcessVariablesSecure(s, variables)
	}, EscapeHTML)
}

// substituteOutside aplica outside a los tramos de text fuera de ranges (rangos
// de bytes ordenados y sin solapes) e inside a los de adentro. Un `{{nombre}}`
// no tiene backticks ni saltos de línea, así que nunca cruza el borde de un
// code span ni de una línea de cerca: partir el texto no cambia qué se
// sustituye fuera de los rangos.
func substituteOutside(text string, ranges [][2]int, outside, inside func(string) string) string {
	var b strings.Builder
	last := 0
	for _, r := range ranges {
		if r[0] < last {
			r[0] = last
		}
		if r[1] <= r[0] {
			continue
		}
		b.WriteString(outside(text[last:r[0]]))
		b.WriteString(inside(text[r[0]:r[1]]))
		last = r[1]
	}
	b.WriteString(outside(text[last:]))
	return b.String()
}

// literalCodeRanges son los rangos de bytes de text que son código en la
// fuente para un camino con la segmentación layout: los code spans de cada
// unidad y, en un texto de varias líneas que no es layoutWhole, las líneas de
// las cercas de código. Salen ordenados y sin solapes.
func literalCodeRanges(text string, layout proseLayout) [][2]int {
	if layout == layoutWhole || !strings.Contains(text, "\n") {
		return spanRanges(text, 0)
	}
	lines := strings.Split(text, "\n")
	starts := make([]int, len(lines))
	for i, off := 0, 0; i < len(lines); i++ {
		starts[i] = off
		off += len(lines[i]) + 1
	}
	var ranges [][2]int
	inFence := fenceLines(lines)
	switch layout {
	case layoutLists:
		for _, unit := range proseUnits(lines) {
			ranges = append(ranges, unit.spanRanges(lines, starts)...)
		}
	default:
		for i, line := range lines {
			ranges = append(ranges, spanRanges(line, starts[i])...)
		}
	}
	for i, line := range lines {
		if inFence[i] {
			ranges = append(ranges, [2]int{starts[i], starts[i] + len(line)})
		}
	}
	return mergeRanges(ranges)
}

// spanRanges son los rangos completos (delimitadores incluidos) de los code
// spans de unit, desplazados offset bytes.
func spanRanges(unit string, offset int) [][2]int {
	var out [][2]int
	for _, sp := range findCodeSpans(unit) {
		out = append(out, [2]int{offset + sp.fullStart, offset + sp.fullEnd})
	}
	return out
}

// mergeRanges ordena y une rangos que se tocan o se solapan.
func mergeRanges(ranges [][2]int) [][2]int {
	if len(ranges) < 2 {
		return ranges
	}
	sorted := append([][2]int(nil), ranges...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j][0] < sorted[j-1][0]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	out := sorted[:1]
	for _, r := range sorted[1:] {
		if last := &out[len(out)-1]; r[0] <= last[1] {
			if r[1] > last[1] {
				last[1] = r[1]
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

// fenceLines marca las líneas de lines que caen en una cerca de código
// escrita dentro de la prosa, con su apertura y su cierre. La regla es la de
// CommonMark con backticks, que es como pinta la cerca un visor de Markdown:
// abre una línea de tres o más backticks con un info string sin backticks
// (```code``` inline es un code span, no una cerca), cierra una línea de solo
// backticks, al menos tantos como la apertura, y una cerca sin cerrar llega al
// final del texto. El HTML de la prosa no pinta la cerca como bloque; que sea
// literal lo decide la sintaxis de la fuente, como en el AST.
func fenceLines(lines []string) []bool {
	marks := make([]bool, len(lines))
	open := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if open > 0 {
			marks[i] = true
			if len(trimmed) >= open && strings.Trim(trimmed, "`") == "" {
				open = 0
			}
			continue
		}
		if n := proseFenceOpen(trimmed); n > 0 {
			marks[i] = true
			open = n
		}
	}
	return marks
}

// proseFenceOpen devuelve cuántos backticks abren una cerca en la línea
// recortada trimmed, o 0 si no abre una (ver fenceLines).
func proseFenceOpen(trimmed string) int {
	n := len(trimmed) - len(strings.TrimLeft(trimmed, "`"))
	if n < 3 || strings.Contains(trimmed[n:], "`") {
		return 0
	}
	return n
}
