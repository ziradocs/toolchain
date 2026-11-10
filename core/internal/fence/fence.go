// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package fence es la regla de las cercas de código flex (```lenguaje …
// ```): cuándo una línea abre una y cuándo la cierra. La comparten el parser
// de bloques de código (internal/elements.CodeParser) y la sustitución de
// variables que respeta el código (renderer.proseVariables), para que lo que
// el renderer trata como código dentro de la prosa sea lo mismo que el parser
// lee como código.
package fence

import "strings"

// Opens dice si trimmed (una línea ya recortada) abre una cerca de código:
// empieza con tres backticks.
func Opens(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```")
}

// Closes dice si trimmed cierra una cerca de código abierta: es exactamente
// tres backticks.
func Closes(trimmed string) bool {
	return trimmed == "```"
}
