// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package fence es la regla única de las cercas de código flex (```lenguaje
// … ```): cuándo una línea abre una y cuándo la cierra. Es la de CommonMark,
// con backticks:
//
//   - abre una línea de tres o más backticks seguidos de un info string sin
//     backticks (```go, ````md, ``` solo);
//   - cierra una línea de solo backticks, al menos tantos como la apertura.
//
// Una línea que empieza con ``` pero lleva un backtick en el info string
// (```js` …) no abre: es prosa. Así una cerca de cuatro backticks puede
// guardar un ``` como contenido.
//
// La usan todos los que deciden dónde empieza o termina una cerca: el parser
// de código y el de code-group, los de diagramas y datos en cerca (mermaid,
// plantuml, chart, map), las reglas que cortan prosa o listas ante una cerca,
// el seguimiento de cercas de DocumentFlexParser, el escáner de regiones
// literales (pre-pase de node-id e @include), el fragmento de contenido, el
// normalizador y fmt. Las cercas ```math siguen su propia regla
// (elements.closesMathFence), que es la misma con backticks y además acepta
// tildes.
package fence

import "strings"

// Open devuelve cuántos backticks abren la cerca de trimmed (una línea ya
// recortada), o 0 si la línea no abre una.
func Open(trimmed string) int {
	n := 0
	for n < len(trimmed) && trimmed[n] == '`' {
		n++
	}
	if n < 3 || strings.Contains(trimmed[n:], "`") {
		return 0
	}
	return n
}

// Info es el info string de una línea que abre una cerca: lo que sigue a los
// backticks, recortado.
func Info(trimmed string) string {
	return strings.TrimSpace(strings.TrimLeft(trimmed, "`"))
}

// Closes dice si trimmed cierra una cerca abierta con open backticks: solo
// backticks, al menos tantos como la apertura.
func Closes(trimmed string, open int) bool {
	return open > 0 && len(trimmed) >= open && strings.Trim(trimmed, "`") == ""
}

// For devuelve la cerca con la que escribir un bloque de código cuyo
// contenido es content: tres backticks, o uno más que la línea de solo
// backticks más larga del contenido, que de otro modo cerraría el bloque al
// releerlo.
func For(content string) string {
	n := 3
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if t != "" && strings.Trim(t, "`") == "" && len(t) >= n {
			n = len(t) + 1
		}
	}
	return strings.Repeat("`", n)
}
