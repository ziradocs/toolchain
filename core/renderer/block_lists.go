// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// Listas de un PointsElement, de un ChecklistElement y de la prosa de una
// cita o un ":::bloque", con la semántica de Markdown: se anidan por sangría a
// cualquier profundidad, cada lista es <ol> o <ul> según su marcador, una
// lista ordenada empieza en el número que escribió el autor, y la sublista vive
// DENTRO del <li> de su item (<li>…<ul>…</ul></li>).

// renderLegacyPoints dibuja una lista sin nested-list-types-v1. El AST no
// serializa el tipo de cada sublista; el parser lo deja en memoria
// (SubListMarker para la sublista, Marker y Delimiter para cada item) y, si
// falta (un AST leído de JSON), la sublista toma el tipo de su padre, igual que
// slidelang. Como en Markdown, un cambio de marcador entre hermanos, o de
// delimitador entre numerados ("1." a "1)"), abre otra lista.
func renderLegacyPoints(items []ast.PointItem, listType string, start *int64, variables map[string]interface{}) string {
	var html strings.Builder
	for i := 0; i < len(items); {
		kind := pointKind(items[i], listType)
		j := i + 1
		for j < len(items) && pointKind(items[j], listType) == kind && items[j].Delimiter == items[i].Delimiter {
			j++
		}
		runStart := items[i].Ordinal
		if i == 0 && start != nil {
			runStart = start
		}
		html.WriteString(openList(kind, runStart))
		for _, item := range items[i:j] {
			fmt.Fprintf(&html, "<li>%s", ProcessTextWithVariablesAndMarkdownSecure(item.Content, variables))
			if len(item.SubPoints) > 0 {
				child := item.SubListMarker
				if child == "" {
					child = kind
				}
				html.WriteString(renderLegacyPoints(item.SubPoints, child, nil, variables))
			}
			html.WriteString("</li>")
		}
		html.WriteString(closeList(kind))
		i = j
	}
	return html.String()
}

func pointKind(item ast.PointItem, listType string) string {
	if item.Marker != "" {
		return item.Marker
	}
	return listType
}

// openList abre una lista. start es el inicio serializado (list-start-v1) o,
// si no hay, el número que el autor escribió en el primer item (Ordinal, solo
// en memoria); nil numera desde 1. Un inicio de 1 no se escribe: es el de HTML.
// "0." empieza en 0, como en CommonMark.
func openList(kind string, start *int64) string {
	if kind != "ordered" {
		return "<ul>"
	}
	if start != nil && *start != 1 {
		return `<ol start="` + strconv.FormatInt(*start, 10) + `">`
	}
	return "<ol>"
}

func closeList(kind string) string {
	if kind == "ordered" {
		return "</ol>"
	}
	return "</ul>"
}

// renderChecklistItems dibuja una lista de tareas y sus sub-tareas a cualquier
// profundidad. La sublista lleva también la clase checklist para que la hoja
// de estilos le quite la viñeta, además de checklist-sub para su sangría.
func renderChecklistItems(items []ast.ChecklistItem, class string, variables map[string]interface{}) string {
	var html strings.Builder
	fmt.Fprintf(&html, `<ul class="%s">`, class)
	for _, item := range items {
		checked := ""
		if item.Checked {
			checked = "checked"
		}
		fmt.Fprintf(&html, `<li><input type="checkbox" %s disabled> %s`, checked, ProcessTextWithVariablesAndMarkdownSecure(item.Content, variables))
		if len(item.SubItems) > 0 {
			html.WriteString(renderChecklistItems(item.SubItems, "checklist checklist-sub", variables))
		}
		html.WriteString("</li>")
	}
	html.WriteString("</ul>")
	return html.String()
}

// bodyListItem es un item de lista reconocido en la prosa de una cita o un
// bloque.
type bodyListItem struct {
	indent   int
	content  int // columna de contenido: la sangría más el marcador y su espacio
	ordered  bool
	delim    byte // '.' o ')' en un item numerado
	ordinal  int64
	text     string
	children []*bodyListItem
}

// parseBodyListItem reconoce un item de Markdown: "-", "*" o "+" y espacio, o
// de uno a nueve dígitos con "." o ")" y espacio. Es el criterio con el que el
// parser conserva la sangría de esas líneas (internal/elements/list_indent.go).
// Una tarea ("- [ ] x") es un item de viñeta con ese texto, igual que un item
// de POINTS: DOCX, PPTX y Markdown la escriben así.
func parseBodyListItem(line string) (*bodyListItem, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	indent := indentColumns(line)
	trimmed = strings.TrimRight(trimmed, " \t")
	if isBodyThematicBreak(trimmed) {
		return nil, false
	}
	item := &bodyListItem{indent: indent}
	switch {
	case len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ':
		item.text = strings.TrimSpace(trimmed[2:])
		item.content = indent + 2
	default:
		i := 0
		for i < len(trimmed) && i < 9 && trimmed[i] >= '0' && trimmed[i] <= '9' {
			i++
		}
		if i == 0 || i+1 >= len(trimmed) || (trimmed[i] != '.' && trimmed[i] != ')') || trimmed[i+1] != ' ' {
			return nil, false
		}
		item.ordered = true
		item.delim = trimmed[i]
		item.ordinal, _ = strconv.ParseInt(trimmed[:i], 10, 64)
		item.text = strings.TrimSpace(trimmed[i+2:])
		item.content = indent + i + 2
	}
	return item, true
}

// isBodyThematicBreak: "* * *", "***", "___" no son viñetas (mismo criterio que
// el parser, internal/elements/text.go).
func isBodyThematicBreak(s string) bool {
	if len(s) < 3 || (s[0] != '*' && s[0] != '_') {
		return false
	}
	count := 0
	for _, c := range s {
		switch {
		case byte(c) == s[0]:
			count++
		case c == ' ' || c == '\t':
		default:
			return false
		}
	}
	return count >= 3
}

// startsBodyList dice si la línea i abre una lista. Como en CommonMark, una
// lista numerada solo interrumpe un párrafo si empieza en 1: "fundada en\n
// 1998. Creció" es una oración partida por el ancho de línea.
func startsBodyList(lines []string, i int, inParagraph bool) bool {
	item, ok := parseBodyListItem(lines[i])
	if !ok {
		return false
	}
	return !inParagraph || !item.ordered || item.ordinal == 1
}

// renderProseWithLists arma la prosa de una cita o un ":::bloque" (su Content
// o los tramos de prosa entre sus elementos). Un TextElement suelto no pasa
// por acá: a nivel de sección las listas ya las separa PointsParser. Sin
// ningún item de lista produce exactamente lo de siempre
// (ProcessInlineMarkdownSecure). Con listas, cada tramo de prosa pasa por esa
// misma función y cada lista se arma como árbol con la regla de CommonMark: un
// item es hijo del de arriba si su marcador empieza en la columna de contenido
// de ese item o más adentro. paragraphs envuelve cada tramo de prosa en <p>:
// una cita lo necesita porque el <p> que envolvía todo el contenido terminaba
// conteniendo un <ul>, que es HTML inválido.
//
// Recibe el texto con las variables ya sustituidas, sin escapar.
func renderProseWithLists(text string, paragraphs bool) string {
	lines := strings.Split(text, "\n")
	hasList := false
	for i := range lines {
		inParagraph := i > 0 && strings.TrimSpace(lines[i-1]) != "" && !isBodyListLine(lines[i-1])
		if startsBodyList(lines, i, inParagraph) {
			hasList = true
			break
		}
	}
	if !hasList {
		out := ProcessInlineMarkdownSecure(text)
		if paragraphs {
			return "<p>" + out + "</p>"
		}
		return out
	}

	var html strings.Builder
	var prose []string
	flushProse := func() {
		if len(prose) == 0 {
			return
		}
		out := ProcessInlineMarkdownSecure(strings.Join(prose, "\n"))
		prose = prose[:0]
		if out == "" {
			return
		}
		if paragraphs {
			out = "<p>" + out + "</p>"
		}
		html.WriteString(out)
	}

	for i := 0; i < len(lines); {
		inParagraph := len(prose) > 0 && strings.TrimSpace(prose[len(prose)-1]) != ""
		if !startsBodyList(lines, i, inParagraph) {
			prose = append(prose, lines[i])
			i++
			continue
		}
		flushProse()
		var roots []*bodyListItem
		var stack []*bodyListItem
		var last *bodyListItem
		blank := false
		for i < len(lines) {
			item, ok := parseBodyListItem(lines[i])
			if !ok {
				trimmed := strings.TrimSpace(lines[i])
				if trimmed == "" {
					// Una línea en blanco sigue dentro de la lista solo si lo
					// que viene después es otro item.
					next := i + 1
					for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
						next++
					}
					if next < len(lines) && isBodyListLine(lines[next]) {
						i = next
						blank = true
						continue
					}
					break
				}
				// Una línea sangrada más que su item lo continúa, salvo que
				// abra otro bloque (una cerca, "$$", "---"...): esa sigue como
				// prosa, como antes.
				if last != nil && indentColumns(lines[i]) > last.indent && !startsOtherBodyBlock(trimmed) {
					last.text += " " + trimmed
					i++
					continue
				}
				break
			}
			depth := len(stack)
			for depth > 0 && item.indent < stack[depth-1].content {
				depth--
			}
			siblings := roots
			if depth > 0 {
				siblings = stack[depth-1].children
			}
			// Como en CommonMark, una lista numerada nueva dentro del último
			// item solo interrumpe su texto si empieza en 1: "- a\n  1998.
			// el año" es texto de "a". Más afuera abre su lista igual.
			newList := len(siblings) == 0 || siblings[len(siblings)-1].ordered != item.ordered || siblings[len(siblings)-1].delim != item.delim
			if last != nil && !blank && depth == len(stack) && newList && item.ordered && item.ordinal != 1 {
				last.text += " " + strings.TrimSpace(lines[i])
				i++
				continue
			}
			stack = stack[:depth]
			if depth == 0 {
				roots = append(roots, item)
			} else {
				parent := stack[depth-1]
				parent.children = append(parent.children, item)
			}
			stack = append(stack, item)
			last = item
			blank = false
			i++
		}
		html.WriteString(renderBodyList(roots))
	}
	flushProse()
	return html.String()
}

func isBodyListLine(line string) bool {
	_, ok := parseBodyListItem(line)
	return ok
}

// renderBodyList dibuja hermanos agrupados en listas: un cambio entre
// ordenada y de viñetas, o de delimitador entre numerados, abre otra lista,
// como en Markdown.
func renderBodyList(items []*bodyListItem) string {
	var html strings.Builder
	for i := 0; i < len(items); {
		j := i + 1
		for j < len(items) && items[j].ordered == items[i].ordered && items[j].delim == items[i].delim {
			j++
		}
		kind := "unordered"
		var start *int64
		if items[i].ordered {
			kind = "ordered"
			start = &items[i].ordinal
		}
		html.WriteString(openList(kind, start))
		for _, it := range items[i:j] {
			html.WriteString("<li>")
			html.WriteString(ProcessInlineMarkdownFormatsSecure(EscapeHTML(it.text)))
			if len(it.children) > 0 {
				html.WriteString(renderBodyList(it.children))
			}
			html.WriteString("</li>")
		}
		html.WriteString(closeList(kind))
		i = j
	}
	return html.String()
}

// indentColumns es la sangría en columnas, con tabuladores a paradas de 4
// como CommonMark (el mismo cálculo que elements.IndentColumns, que el
// renderer no puede importar).
func indentColumns(line string) int {
	col := 0
	for _, c := range line {
		switch c {
		case ' ':
			col++
		case '\t':
			col += 4 - col%4
		default:
			return col
		}
	}
	return col
}

// startsOtherBodyBlock dice si una línea de la prosa de una cita o un bloque
// abre otro bloque y no puede ser texto de un item: el mismo criterio que
// elements.StartsOtherBlock, por prefijo.
func startsOtherBodyBlock(trimmed string) bool {
	if isBodyThematicBreak(trimmed) || strings.Trim(trimmed, "-") == "" {
		return true
	}
	for _, prefix := range []string{"<<", "$$", "@", ":::", ">", "|", "#", "```", "~~~", "<", "![", `\[`} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
