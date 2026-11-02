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
// (SubListMarker para la sublista, Marker para cada item) y, si falta (un AST
// leído de JSON), la sublista toma el tipo de su padre, igual que slidelang.
// Un cambio de marcador entre hermanos abre otra lista, como en Markdown.
func renderLegacyPoints(items []ast.PointItem, listType string, start *int64, variables map[string]interface{}) string {
	var html strings.Builder
	for i := 0; i < len(items); {
		kind := listType
		if items[i].Marker != "" {
			kind = items[i].Marker
		}
		j := i + 1
		for j < len(items) && pointKind(items[j], listType) == kind {
			j++
		}
		var runStart *int64
		if i == 0 {
			runStart = start
		}
		html.WriteString(openList(kind, runStart, items[i].Ordinal, ""))
		for _, item := range items[i:j] {
			fmt.Fprintf(&html, "<li>%s", pointContentHTML(item.Content, variables))
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

// pointContentHTML es el contenido de un item. Un item de viñeta que empieza
// con "[ ] " o "[x] " es una tarea, como en GFM: una tarea debajo de un item
// común (`- a` / `  - [ ] b`) llega al AST como un PointItem con ese texto,
// porque no hay forma de mezclar items y tareas en una misma lista, y sale con
// su casilla en vez del texto "[ ] b".
func pointContentHTML(content string, variables map[string]interface{}) string {
	task, rest := taskPrefix(content)
	html := ProcessTextWithVariablesAndMarkdownSecure(rest, variables)
	switch task {
	case 1:
		return `<input type="checkbox" disabled> ` + html
	case 2:
		return `<input type="checkbox" checked disabled> ` + html
	}
	return html
}

// taskPrefix separa la marca de tarea de un texto: 0 si no es tarea, 1 sin
// marcar, 2 marcada.
func taskPrefix(text string) (int, string) {
	switch {
	case strings.HasPrefix(text, "[ ] "):
		return 1, strings.TrimSpace(text[4:])
	case strings.HasPrefix(text, "[x] "), strings.HasPrefix(text, "[X] "):
		return 2, strings.TrimSpace(text[4:])
	}
	return 0, text
}

func pointKind(item ast.PointItem, listType string) string {
	if item.Marker != "" {
		return item.Marker
	}
	return listType
}

// openList abre una lista. start es el inicio serializado (list-start-v1); si
// no hay, vale el número que el autor escribió en el primer item (ordinal,
// solo en memoria). Un inicio de 1 no se escribe: es el de HTML.
func openList(kind string, start *int64, ordinal int64, class string) string {
	attrs := ""
	if class != "" {
		attrs = ` class="` + class + `"`
	}
	if kind != "ordered" {
		return "<ul" + attrs + ">"
	}
	n := ordinal
	if start != nil {
		n = *start
	}
	if n > 1 {
		attrs += ` start="` + strconv.FormatInt(n, 10) + `"`
	}
	return "<ol" + attrs + ">"
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
	ordered  bool
	ordinal  int64
	task     int // 0 no es tarea, 1 sin marcar, 2 marcada
	text     string
	children []*bodyListItem
}

// parseBodyListItem reconoce un item de Markdown: "-", "*" o "+" y espacio, o
// de uno a nueve dígitos con "." o ")" y espacio. Es el criterio con el que el
// parser conserva la sangría de esas líneas (internal/elements/list_indent.go).
func parseBodyListItem(line string) (*bodyListItem, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	indent := 0
	for _, c := range line[:len(line)-len(trimmed)] {
		if c == '\t' {
			indent += 4
		} else {
			indent++
		}
	}
	trimmed = strings.TrimRight(trimmed, " \t")
	if isBodyThematicBreak(trimmed) {
		return nil, false
	}
	item := &bodyListItem{indent: indent}
	switch {
	case len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ':
		item.text = strings.TrimSpace(trimmed[2:])
	default:
		i := 0
		for i < len(trimmed) && i < 9 && trimmed[i] >= '0' && trimmed[i] <= '9' {
			i++
		}
		if i == 0 || i+1 >= len(trimmed) || (trimmed[i] != '.' && trimmed[i] != ')') || trimmed[i+1] != ' ' {
			return nil, false
		}
		item.ordered = true
		item.ordinal, _ = strconv.ParseInt(trimmed[:i], 10, 64)
		item.text = strings.TrimSpace(trimmed[i+2:])
	}
	if !item.ordered {
		item.task, item.text = taskPrefix(item.text)
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

// renderProseWithLists arma la prosa de un TextElement, una cita o un
// ":::bloque". Sin ningún item de lista produce exactamente lo de siempre
// (ProcessInlineMarkdownSecure). Con listas, cada tramo de prosa pasa por esa
// misma función y cada lista se arma como árbol por sangría, con su tipo, su
// inicio y las tareas como casilla. paragraphs envuelve cada tramo de prosa en
// <p>: una cita lo necesita porque el <p> que envolvía todo el contenido
// terminaba conteniendo un <ul>, que es HTML inválido.
//
// Recibe el texto con las variables ya sustituidas, sin escapar.
func renderProseWithLists(text string, paragraphs bool) string {
	lines := strings.Split(text, "\n")
	hasList := false
	for _, line := range lines {
		if _, ok := parseBodyListItem(line); ok {
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
		if _, ok := parseBodyListItem(lines[i]); !ok {
			prose = append(prose, lines[i])
			i++
			continue
		}
		flushProse()
		var roots []*bodyListItem
		var stack []*bodyListItem
		var last *bodyListItem
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
					if next < len(lines) {
						if _, ok := parseBodyListItem(lines[next]); ok {
							i = next
							continue
						}
					}
					break
				}
				// Una línea sangrada más que su item lo continúa.
				if last != nil && len(lines[i])-len(strings.TrimLeft(lines[i], " \t")) > last.indent {
					last.text += " " + trimmed
					i++
					continue
				}
				break
			}
			for len(stack) > 0 && stack[len(stack)-1].indent >= item.indent {
				stack = stack[:len(stack)-1]
			}
			if len(stack) == 0 {
				roots = append(roots, item)
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, item)
			}
			stack = append(stack, item)
			last = item
			i++
		}
		html.WriteString(renderBodyList(roots))
	}
	flushProse()
	return html.String()
}

// renderBodyList dibuja hermanos agrupados en listas: un cambio entre
// ordenada y de viñetas abre otra lista, como en Markdown.
func renderBodyList(items []*bodyListItem) string {
	var html strings.Builder
	for i := 0; i < len(items); {
		j := i + 1
		for j < len(items) && items[j].ordered == items[i].ordered {
			j++
		}
		kind := "unordered"
		if items[i].ordered {
			kind = "ordered"
		}
		class := ""
		if !items[i].ordered {
			class = "checklist"
			for _, it := range items[i:j] {
				if it.task == 0 {
					class = ""
					break
				}
			}
		}
		html.WriteString(openList(kind, nil, items[i].ordinal, class))
		for _, it := range items[i:j] {
			html.WriteString("<li>")
			switch it.task {
			case 1:
				html.WriteString(`<input type="checkbox" disabled> `)
			case 2:
				html.WriteString(`<input type="checkbox" checked disabled> `)
			}
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
