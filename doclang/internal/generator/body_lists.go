// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strconv"
	"strings"
)

// Listas dentro de la prosa de una cita o un ":::bloque".
//
// core guarda esas listas en el Content del elemento, un string, con la
// sangría de cada item anidado. Este archivo las vuelve a armar con las mismas
// reglas que el HTML de core (renderProseWithLists en core/renderer): un item
// es hijo del de arriba si su marcador empieza en la columna de contenido de
// ese item; un cambio entre numerada y de viñetas, o de delimitador, abre otra
// lista; una lista numerada solo interrumpe un párrafo si empieza en 1. DOCX y
// Markdown las escriben como listas en vez de como líneas sueltas con los
// espacios de la sangría delante.

// bodyItem es un item de lista de la prosa de una cita o un bloque.
type bodyItem struct {
	indent   int
	content  int
	ordered  bool
	delim    byte
	ordinal  int64
	text     string
	children []*bodyItem
}

// bodyPart es un tramo del cuerpo: prosa (lines) o una lista (items).
type bodyPart struct {
	lines []string
	items []*bodyItem
}

func parseBodyItem(line string) (*bodyItem, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	indent := bodyIndent(line)
	trimmed = strings.TrimRight(trimmed, " \t")
	if isBodyRule(trimmed) {
		return nil, false
	}
	item := &bodyItem{indent: indent}
	if len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ' {
		item.text = strings.TrimSpace(trimmed[2:])
		item.content = indent + 2
		return item, true
	}
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
	return item, true
}

// isBodyRule: "* * *", "***", "___" son una regla, no una viñeta.
func isBodyRule(s string) bool {
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

// splitBody parte el cuerpo de una cita o un bloque en tramos de prosa y
// listas. Sin listas devuelve un solo tramo de prosa con todas las líneas.
func splitBody(content string) []bodyPart {
	lines := strings.Split(content, "\n")
	var parts []bodyPart
	var prose []string
	flush := func() {
		if len(prose) > 0 {
			parts = append(parts, bodyPart{lines: prose})
			prose = nil
		}
	}
	for i := 0; i < len(lines); {
		first, ok := parseBodyItem(lines[i])
		inParagraph := len(prose) > 0 && strings.TrimSpace(prose[len(prose)-1]) != ""
		if !ok || inParagraph && first.ordered && first.ordinal != 1 {
			prose = append(prose, lines[i])
			i++
			continue
		}
		flush()
		var roots, stack []*bodyItem
		var last *bodyItem
		blank := false
		for i < len(lines) {
			item, ok := parseBodyItem(lines[i])
			if !ok {
				trimmed := strings.TrimSpace(lines[i])
				if trimmed == "" {
					next := i + 1
					for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
						next++
					}
					if next < len(lines) {
						if _, ok := parseBodyItem(lines[next]); ok {
							i = next
							blank = true
							continue
						}
					}
					break
				}
				if last != nil && bodyIndent(lines[i]) > last.indent && !startsBodyBlock(trimmed) {
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
		parts = append(parts, bodyPart{items: roots})
	}
	flush()
	return parts
}

// bodyRuns agrupa hermanos en listas: un cambio de tipo o de delimitador abre
// otra.
func bodyRuns(items []*bodyItem) [][]*bodyItem {
	var runs [][]*bodyItem
	for i := 0; i < len(items); {
		j := i + 1
		for j < len(items) && items[j].ordered == items[i].ordered && items[j].delim == items[i].delim {
			j++
		}
		runs = append(runs, items[i:j])
		i = j
	}
	return runs
}

// bodyIndent es la sangría en columnas, con tabuladores a paradas de 4 como
// CommonMark.
func bodyIndent(line string) int {
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

// startsBodyBlock: una línea que abre otro bloque (una cerca, "$$", "---",
// un encabezado...) no es texto de un item.
func startsBodyBlock(trimmed string) bool {
	if isBodyRule(trimmed) || strings.Trim(trimmed, "-") == "" {
		return true
	}
	for _, prefix := range []string{"<<", "$$", "@", ":::", ">", "|", "#", "```", "~~~", "<", "![", `\[`} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
