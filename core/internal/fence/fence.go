// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package fence es la regla única de las cercas de código flex (```lenguaje
// … ```): cuándo una línea abre una y hasta dónde llega.
//
// Open y Closes son la regla de CommonMark con backticks, línea por línea:
//
//   - abre una línea de tres o más backticks seguidos de un info string sin
//     backticks (```go, ````md, ``` solo);
//   - cierra una línea de solo backticks, al menos tantos como la apertura.
//
// At decide la cerca entera, con lookahead: aplica CommonMark cuando da un
// resultado bien formado, y si una cerca quedaría sin cerrar la lee como las
// versiones anteriores (abría toda línea que empieza con ``` y cerraba en la
// primera ``` exacta), para que ningún documento que esas versiones leían
// cambie de significado sin aviso. Una línea con un backtick en el info
// string (```js` …) es prosa, salvo que como prosa el resto deje una cerca sin
// cerrar.
//
// La usan todos los que deciden dónde empieza o termina una cerca: el parser
// de código y el de code-group, los de diagramas y datos en cerca (mermaid,
// plantuml, chart, map), las reglas que cortan prosa o listas ante una cerca,
// el escáner de regiones literales del pre-pase de node-id, el fragmento de
// contenido, el normalizador y fmt. Las cercas ```math siguen su propia regla
// (elements.closesMathFence), que es la misma con backticks y además acepta
// tildes; sin cierre pasan a CodeParser. La prosa que pinta el renderer usa
// solo la regla de CommonMark (renderer.fenceLines y startsOtherBodyBlock):
// ahí no hay un parser de bloques que pueda cambiar de lectura.
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

// Reading es cómo se leyó una cerca (ver At).
type Reading int

const (
	// CommonMark: la cerca cierra con la regla de CommonMark.
	CommonMark Reading = iota
	// ClosedByMain: la cerca abre con la regla de CommonMark pero no la
	// cierra ninguna línea de solo backticks con al menos tantos; cierra en
	// la primera línea ``` exacta, como en las versiones anteriores.
	ClosedByMain
	// Unclosed: no la cierra ninguna de las dos reglas; llega al final.
	Unclosed
	// OpenedByMain: la línea lleva un backtick en el info string, así que
	// para CommonMark es texto, pero leída como texto el resto dejaría una
	// cerca sin cerrar; se lee como en las versiones anteriores, como una
	// cerca que cierra en la primera línea ``` exacta (o no cierra).
	OpenedByMain
)

// Block es una cerca de código de lines que abre en la línea Start.
type Block struct {
	Start int
	// Ticks son los backticks de la apertura.
	Ticks int
	// End es el índice de la línea que la cierra, o -1 si llega al final de
	// lines.
	End     int
	Reading Reading
	// MainEnd es donde la cerraba la regla de las versiones anteriores
	// (abre toda línea que empieza con ```, cierra la primera ``` exacta):
	// el índice de esa línea o -1. Si es distinto de End, el bloque cambió de
	// extensión y quien lo parsea lo avisa.
	MainEnd int
}

// At dice si lines[i] abre una cerca de código y hasta dónde llega, con
// lookahead sobre el resto de lines. Los parsers pasan sus ParseContext.Lines,
// que en la práctica son el resto del documento: ni una pestaña de
// code-group ni el cuerpo de un ::: acotan la búsqueda de cierre. La regla de CommonMark (Open, Closes) aplica cuando
// da un resultado bien formado. Si una cerca de CommonMark no tiene cierre,
// cierra donde cerraba en las versiones anteriores (ClosedByMain, Unclosed); y
// una línea con un backtick en el info string, que para CommonMark es texto,
// se lee como cerca si como texto el resto dejaría una cerca sin cerrar
// (OpenedByMain). Así un documento que las versiones anteriores leían no
// cambia de significado en silencio: el parser avisa en cada caso.
func At(lines []string, i int) (Block, bool) {
	if i < 0 || i >= len(lines) {
		return Block{}, false
	}
	trimmed := strings.TrimSpace(lines[i])
	if !strings.HasPrefix(trimmed, "```") {
		return Block{}, false
	}
	mainEnd := -1
	for j := i + 1; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "```" {
			mainEnd = j
			break
		}
	}
	if n := Open(trimmed); n > 0 {
		if end := closing(lines, i, n); end >= 0 {
			return Block{Start: i, Ticks: n, End: end, Reading: CommonMark, MainEnd: mainEnd}, true
		}
		if mainEnd >= 0 {
			return Block{Start: i, Ticks: n, End: mainEnd, Reading: ClosedByMain, MainEnd: mainEnd}, true
		}
		return Block{Start: i, Ticks: n, End: -1, Reading: Unclosed, MainEnd: -1}, true
	}
	if !leavesUnclosed(lines, i+1) {
		return Block{}, false
	}
	ticks := len(trimmed) - len(strings.TrimLeft(trimmed, "`"))
	return Block{Start: i, Ticks: ticks, End: mainEnd, Reading: OpenedByMain, MainEnd: mainEnd}, true
}

// Starts dice si lines[i] abre una cerca (ver At).
func Starts(lines []string, i int) bool {
	_, ok := At(lines, i)
	return ok
}

// Consumed son las líneas que ocupa b: apertura, cuerpo y cierre, o hasta el
// final de lines si no cierra.
func (b Block) Consumed(lines []string) int {
	if b.End < 0 {
		return len(lines) - b.Start
	}
	return b.End - b.Start + 1
}

// closing es la primera línea después de i que cierra con la regla de
// CommonMark una cerca de n backticks, o -1.
func closing(lines []string, i, n int) int {
	for j := i + 1; j < len(lines); j++ {
		if Closes(strings.TrimSpace(lines[j]), n) {
			return j
		}
	}
	return -1
}

// leavesUnclosed dice si, leyendo lines desde from solo con la regla de
// CommonMark, alguna cerca queda sin cerrar.
func leavesUnclosed(lines []string, from int) bool {
	for j := from; j < len(lines); j++ {
		if n := Open(strings.TrimSpace(lines[j])); n > 0 {
			end := closing(lines, j, n)
			if end < 0 {
				return true
			}
			j = end
		}
	}
	return false
}

// End es el índice de la línea que cierra la cerca que abre en lines[i] (como
// la lee At), len(lines) si no cierra, o -1 si lines[i] no abre una cerca. Es
// la forma cómoda de At para quien recorre líneas y solo necesita saltar el
// cuerpo de una cerca.
func End(lines []string, i int) int {
	b, ok := At(lines, i)
	if !ok {
		return -1
	}
	if b.End < 0 {
		return len(lines)
	}
	return b.End
}

// Lines marca las líneas de lines, leídas desde from, que caen en una cerca
// de código (apertura y cierre incluidos), con la lectura de At.
func Lines(lines []string, from int) []bool {
	marks := make([]bool, len(lines))
	for i := from; i < len(lines); i++ {
		end := End(lines, i)
		if end < 0 {
			continue
		}
		for j := i; j <= end && j < len(lines); j++ {
			marks[j] = true
		}
		i = end
	}
	return marks
}
