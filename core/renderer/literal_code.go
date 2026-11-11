// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"regexp"
	"sort"
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

// proseVariables prepara un texto de prosa que después pasa por
// ProcessInlineMarkdownSecure (línea por línea): sustituye `{{nombre}}` y
// devuelve también las marcas que hay que deshacer en el HTML que salga de ese
// Markdown (valueMarks.html). Sin la opción de código literal es
// ProcessVariables y no hay marcas. Con ella no toca lo que es código en la
// fuente: los code spans, emparejados en cada línea como los empareja el
// renderer, y, en un texto de varias líneas, las cercas de código escritas
// adentro (ver fenceLines). En un texto de una sola línea no hay cerca. Y un
// valor sustituido no puede cambiar los code spans de la fuente: sus backticks
// y sus saltos de línea pasan al Markdown como dos caracteres de uso privado
// que ni el texto ni los valores traen, que no emparejan ni parten líneas ni
// items, y que valueMarks vuelve a poner. Sin la opción, un salto de línea en
// un valor sigue partiendo el texto como siempre.
func proseVariables(text string, variables map[string]interface{}) (string, valueMarks) {
	return proseMarkdownVariables(text, variables, layoutLines)
}

// proseLineMarkdownVariables es proseVariables para un texto que se pinta
// entero como una sola unidad de Markdown (ProcessInlineMarkdownSecureLine): un
// encabezado.
func proseLineMarkdownVariables(text string, variables map[string]interface{}) (string, valueMarks) {
	return proseMarkdownVariables(text, variables, layoutWhole)
}

// proseListVariables es proseVariables para la prosa que pasa por
// renderProseWithLists: los code spans de un item de lista se emparejan sobre
// el item ya juntado con sus líneas de continuación, como lo pinta el renderer.
func proseListVariables(text string, variables map[string]interface{}) (string, valueMarks) {
	return proseMarkdownVariables(text, variables, layoutLists)
}

// proseLineVariables sustituye `{{nombre}}` en un campo de texto plano de una
// línea (un título de sección, el alt o el caption de una imagen) que se
// escribe sin Markdown: con la opción de código literal, fuera de sus code
// spans. Sin la opción es ProcessVariables.
func proseLineVariables(text string, variables map[string]interface{}) string {
	if !isLiteralCode(variables) || !strings.Contains(text, "{{") {
		return ProcessVariables(text, variables)
	}
	return substituteOutside(text, literalCodeRanges(text, layoutWhole), func(s string) string {
		return ProcessVariables(s, variables)
	}, func(s string) string { return s })
}

// valueMarks son los dos caracteres con los que proseMarkdownVariables marcó
// los backticks (tick) y los saltos de línea (newline) de los valores
// sustituidos. Vacío si no marcó nada: entonces html y text no cambian nada.
type valueMarks struct {
	tick, newline string
}

// html deshace las marcas en el HTML que salió del Markdown: el backtick
// vuelve como backtick y el salto de línea como <br>, salvo dentro de una
// etiqueta (un href, un title…), donde un <br> no tiene sentido y un salto de
// línea en una URL lo quitaría el navegador: ahí vuelve como espacio.
func (m valueMarks) html(s string) string {
	if m.tick == "" || !strings.Contains(s, m.tick) && !strings.Contains(s, m.newline) {
		return s
	}
	var b strings.Builder
	restore := func(seg, newline string) {
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(seg, m.tick, "`"), m.newline, newline))
	}
	for {
		start := strings.IndexByte(s, '<')
		if start < 0 {
			restore(s, "<br>")
			return b.String()
		}
		end := htmlTagEnd(s, start)
		restore(s[:start], "<br>")
		restore(s[start:end], " ")
		s = s[end:]
	}
}

// text deshace las marcas en un texto que no es HTML (el de un LangRun): el
// salto de línea vuelve como salto de línea.
func (m valueMarks) text(s string) string {
	if m.tick == "" {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, m.tick, "`"), m.newline, "\n")
}

var proseVariablePattern = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

// privateUse son los rangos de caracteres de uso privado de Unicode: el del
// plano básico y los de los planos 15 y 16.
var privateUse = [][2]rune{{0xE000, 0xF8FF}, {0xF0000, 0xFFFFD}, {0x100000, 0x10FFFD}}

// pickMarks elige dos caracteres de uso privado que no aparecen en used, para
// marcar sin confundirse con nada que ya traigan el texto o los valores.
// Recorre used una sola vez. Que no queden dos libres exigiría que el texto y
// sus valores trajeran más de 137 000 caracteres de uso privado distintos;
// entonces no se marca nada y los valores se sustituyen tal cual, como sin la
// opción.
func pickMarks(used string) valueMarks {
	present := map[rune]bool{}
	for _, r := range used {
		if r >= 0xE000 && r <= 0xF8FF || r >= 0xF0000 {
			present[r] = true
		}
	}
	var picked []string
	for _, rg := range privateUse {
		for r := rg[0]; r <= rg[1] && len(picked) < 2; r++ {
			if !present[r] {
				picked = append(picked, string(r))
			}
		}
	}
	if len(picked) < 2 {
		return valueMarks{}
	}
	return valueMarks{tick: picked[0], newline: picked[1]}
}

func proseMarkdownVariables(text string, variables map[string]interface{}, layout proseLayout) (string, valueMarks) {
	if !isLiteralCode(variables) || !strings.Contains(text, "{{") {
		return ProcessVariables(text, variables), valueMarks{}
	}
	// Los valores que se van a sustituir, para elegir marcas que no traigan.
	var used strings.Builder
	used.WriteString(text)
	marked := false
	for _, m := range proseVariablePattern.FindAllStringSubmatch(text, -1) {
		if value, ok := variables[m[1]]; ok {
			v := fmt.Sprintf("%v", value)
			used.WriteString(v)
			marked = marked || strings.ContainsAny(v, "`\n\r")
		}
	}
	var marks valueMarks
	if marked {
		marks = pickMarks(used.String())
	}
	outside := func(seg string) string {
		if marks.tick == "" {
			return ProcessVariables(seg, variables)
		}
		return proseVariablePattern.ReplaceAllStringFunc(seg, func(match string) string {
			if value, ok := variables[match[2:len(match)-2]]; ok {
				v := strings.ReplaceAll(fmt.Sprintf("%v", value), "`", marks.tick)
				// Un CRLF o un CR suelto también es un salto de línea.
				v = strings.ReplaceAll(strings.ReplaceAll(v, "\r\n", "\n"), "\r", "\n")
				return strings.ReplaceAll(v, "\n", marks.newline)
			}
			return match
		})
	}
	return substituteOutside(text, literalCodeRanges(text, layout), outside, func(s string) string { return s }), marks
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

// codeSpanRangesIn son los rangos de text que caen en un code span de las
// unidades de layout, sin las cercas: lo que el HTML pinta como <code>.
func codeSpanRangesIn(text string, layout proseLayout) [][2]int {
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
	if layout == layoutLists {
		for _, unit := range proseUnits(lines) {
			ranges = append(ranges, unit.spanRanges(lines, starts)...)
		}
	} else {
		for i, line := range lines {
			ranges = append(ranges, spanRanges(line, starts[i])...)
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
	sort.Slice(sorted, func(i, j int) bool { return sorted[i][0] < sorted[j][0] })
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
	return fenceLinesOf(lines, false)
}

// fenceLinesOf es fenceLines; con tildes reconoce además las cercas de tildes
// de CommonMark (tres o más "~", con cualquier info string; cierra una línea
// de solo tildes, al menos tantas como la apertura). Las usa
// renderProseWithLists para no partir un párrafo dentro de una cerca; el
// código literal sigue siendo solo el de backticks.
func fenceLinesOf(lines []string, tildes bool) []bool {
	marks := make([]bool, len(lines))
	open := 0
	fence := "`"
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if open > 0 {
			marks[i] = true
			if len(trimmed) >= open && strings.Trim(trimmed, fence) == "" {
				open = 0
			}
			continue
		}
		if n := proseFenceOpen(trimmed); n > 0 {
			marks[i] = true
			open, fence = n, "`"
		} else if n := tildeFenceOpen(trimmed); tildes && n > 0 {
			marks[i] = true
			open, fence = n, "~"
		}
	}
	return marks
}

// tildeFenceOpen devuelve cuántas tildes abren una cerca en la línea
// recortada trimmed, o 0 si no abre una (ver fenceLinesOf).
func tildeFenceOpen(trimmed string) int {
	n := len(trimmed) - len(strings.TrimLeft(trimmed, "~"))
	if n < 3 {
		return 0
	}
	return n
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
