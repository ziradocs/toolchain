// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package fence es la regla de las cercas de código flex (```lenguaje …
// ```): cuándo una línea abre una y cuándo la cierra. Es la regla de main,
// en un solo lugar:
//
//   - abre una línea que empieza con tres backticks;
//   - cierra una línea que es exactamente tres backticks, sin importar
//     cuántos backticks tenía la apertura.
//
// La usa el parser de bloques de código (internal/elements.CodeParser).
package fence

import "strings"

// Open devuelve cuántos backticks abren la cerca de trimmed (una línea ya
// recortada), o 0 si la línea no abre una: abre toda línea que empieza con
// tres backticks.
func Open(trimmed string) int {
	if !strings.HasPrefix(trimmed, "```") {
		return 0
	}
	return len(trimmed) - len(strings.TrimLeft(trimmed, "`"))
}

// Closes dice si trimmed cierra una cerca abierta con open backticks (lo que
// devolvió Open): es exactamente tres backticks. El largo de la apertura no
// cuenta: una cerca de cuatro también cierra en ```.
func Closes(trimmed string, open int) bool {
	return open > 0 && trimmed == "```"
}
