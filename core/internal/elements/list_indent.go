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
	indent := CalculateIndentLevel(line)
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
