// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import "strings"

// keepListIndent devuelve una línea del cuerpo de una cita o de un ":::bloque"
// tal como la guarda Content. Una línea de prosa sale sin sangría, como
// siempre. Una línea que es un item de lista conserva la sangría que tiene por
// encima de base (la sangría de la primera línea del cuerpo), porque es lo
// único que dice qué item cuelga de cuál: antes la línea salía recortada y
// "> - a\n>   - b" llegaba al AST como dos items del mismo nivel.
//
// Content sigue siendo un string y la forma del JSON no cambia; lo que cambia
// es que un item anidado llega con sus espacios delante. Una cita o un bloque
// sin listas anidadas guarda exactamente lo mismo que antes.
func keepListIndent(line string, base int) string {
	trimmed := strings.TrimSpace(line)
	indent := IndentColumns(line)
	if indent <= base || !IsBodyListItem(trimmed) {
		return trimmed
	}
	return strings.Repeat(" ", indent-base) + trimmed
}

// IsBodyListItem reconoce un item de lista de Markdown al inicio de una línea
// ya recortada: "-", "*" o "+" seguidos de espacio, o de uno a nueve dígitos
// seguidos de "." o ")" y espacio. Es el mismo criterio con el que el renderer
// arma listas dentro de una cita o un bloque (renderer/block_lists.go).
func IsBodyListItem(trimmed string) bool {
	if isThematicBreak(trimmed) {
		return false
	}
	if len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ' {
		return true
	}
	i := 0
	for i < len(trimmed) && i < 9 && trimmed[i] >= '0' && trimmed[i] <= '9' {
		i++
	}
	return i > 0 && i+1 < len(trimmed) && (trimmed[i] == '.' || trimmed[i] == ')') && trimmed[i+1] == ' '
}

// firstBodyIndent es la sangría de la primera línea con contenido de lines a
// partir de from, o 0 si no hay ninguna.
func firstBodyIndent(lines []string, from int, stop func(string) bool) int {
	for i := from; i < len(lines); i++ {
		if stop != nil && stop(lines[i]) {
			return 0
		}
		if strings.TrimSpace(lines[i]) != "" {
			return CalculateIndentLevel(lines[i])
		}
	}
	return 0
}

// IndentColumns es la sangría de una línea en columnas, con cada tabulador
// avanzando hasta la siguiente parada de 4, como en CommonMark: "\t- b" está en
// la columna 4 y " \t- b" también. Es la medida con la que se comparan las
// columnas de contenido de una lista.
func IndentColumns(line string) int {
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

// otherBlockParsers son los parsers del registry que abren un bloque propio:
// todos menos las listas y el párrafo.
var otherBlockParsers = func() []ElementParser {
	var out []ElementParser
	for _, p := range GetDefaultRegistry().parsers {
		switch p.(type) {
		case *PointsParser, *ChecklistParser, *TextParser:
			continue
		}
		out = append(out, p)
	}
	return out
}()

// StartsOtherBlock dice si una línea ya recortada abre un bloque que no es
// texto de un item de lista: lo que reclama otro parser del registry (chart,
// ecuación, código, tabla, cita, bloque, directiva, imagen...), un encabezado,
// una regla o separador ("---", "***"), una cerca, HTML o un "$$". Corta la
// continuación de un item, que es lo que hacía siempre la lista con cualquier
// línea que no fuera un item.
func StartsOtherBlock(trimmed string) bool {
	if trimmed == "" || isThematicBreak(trimmed) || strings.Trim(trimmed, "-") == "" {
		return true
	}
	for _, prefix := range []string{"<<", "$$", "@", ":::", ">", "|", "#", "```", "~~~", "<", "![", `\[`} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	for _, p := range otherBlockParsers {
		if p.CanParse(trimmed, "flex") {
			return true
		}
	}
	return false
}
