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
// At y AtWithin deciden la cerca entera, con lookahead acotado a su
// contenedor (el documento, o el :::code-group o bloque ::: que la contiene,
// ContainerEnd): aplican CommonMark cuando da un resultado bien formado en el
// contenedor, y si una cerca quedaría sin cerrar la leen como las
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
	// su contenedor (Limit).
	End int
	// Limit es el final (exclusivo) del contenedor en el que se buscó el
	// cierre: len(lines) para el documento, o el cierre de la pestaña, el
	// code-group o el ::: que la contiene.
	Limit   int
	Reading Reading
	// MainEnd es donde la cerraba la regla de las versiones anteriores
	// (abre toda línea que empieza con ```, cierra la primera ``` exacta):
	// el índice de esa línea o -1. No se acota al contenedor: es lo que
	// hacían esas versiones. Si es distinto de End, el bloque cambió de
	// extensión y quien lo parsea lo avisa.
	MainEnd int
}

// At es AtWithin sobre todo lines: la cerca de un documento sin contenedor.
func At(lines []string, i int) (Block, bool) {
	return AtWithin(lines, i, len(lines))
}

// AtWithin dice si lines[i] abre una cerca de código y hasta dónde llega,
// buscando el cierre solo antes de limit: el final de su contenedor (el
// documento, o el cierre de la pestaña, el code-group o el ::: que la
// contiene, ver ContainerEnd). La regla de CommonMark (Open, Closes) aplica
// cuando da un resultado bien formado dentro del contenedor. Si una cerca de
// CommonMark no cierra antes de limit, cierra donde cerraba en las versiones
// anteriores si esa línea está antes de limit (ClosedByMain) o llega a limit
// (Unclosed); y una línea con un backtick en el info string, que para
// CommonMark es texto, se lee como cerca si como texto el resto del
// contenedor dejaría una cerca sin cerrar (OpenedByMain). Así un documento
// que las versiones anteriores leían no cambia de significado en silencio: el
// parser avisa en cada caso.
func AtWithin(lines []string, i, limit int) (Block, bool) {
	if limit > len(lines) {
		limit = len(lines)
	}
	if i < 0 || i >= limit {
		return Block{}, false
	}
	trimmed := strings.TrimSpace(lines[i])
	if !strings.HasPrefix(trimmed, "```") {
		return Block{}, false
	}
	mainEnd := exactClose(lines, i, len(lines))
	boundedMain := exactClose(lines, i, limit)
	if n := Open(trimmed); n > 0 {
		if end := closing(lines, i, n, limit); end >= 0 {
			return Block{Start: i, Ticks: n, End: end, Limit: limit, Reading: CommonMark, MainEnd: mainEnd}, true
		}
		if boundedMain >= 0 {
			return Block{Start: i, Ticks: n, End: boundedMain, Limit: limit, Reading: ClosedByMain, MainEnd: mainEnd}, true
		}
		return Block{Start: i, Ticks: n, End: -1, Limit: limit, Reading: Unclosed, MainEnd: mainEnd}, true
	}
	if !leavesUnclosed(lines, i+1, limit) {
		return Block{}, false
	}
	ticks := len(trimmed) - len(strings.TrimLeft(trimmed, "`"))
	return Block{Start: i, Ticks: ticks, End: boundedMain, Limit: limit, Reading: OpenedByMain, MainEnd: mainEnd}, true
}

// Starts dice si lines[i] abre una cerca (ver At).
func Starts(lines []string, i int) bool {
	_, ok := At(lines, i)
	return ok
}

// Consumed son las líneas que ocupa b: apertura, cuerpo y cierre, o hasta el
// final de su contenedor si no cierra.
func (b Block) Consumed() int {
	if b.End < 0 {
		return b.Limit - b.Start
	}
	return b.End - b.Start + 1
}

// exactClose es la primera línea después de i y antes de limit que es una ```
// exacta (la regla de las versiones anteriores), o -1.
func exactClose(lines []string, i, limit int) int {
	for j := i + 1; j < limit; j++ {
		if strings.TrimSpace(lines[j]) == "```" {
			return j
		}
	}
	return -1
}

// closing es la primera línea después de i y antes de limit que cierra con la
// regla de CommonMark una cerca de n backticks, o -1.
func closing(lines []string, i, n, limit int) int {
	for j := i + 1; j < limit; j++ {
		if Closes(strings.TrimSpace(lines[j]), n) {
			return j
		}
	}
	return -1
}

// leavesUnclosed dice si, leyendo lines desde from hasta limit solo con la
// regla de CommonMark, alguna cerca queda sin cerrar.
func leavesUnclosed(lines []string, from, limit int) bool {
	for j := from; j < limit; j++ {
		if n := Open(strings.TrimSpace(lines[j])); n > 0 {
			end := closing(lines, j, n, limit)
			if end < 0 {
				return true
			}
			j = end
		}
	}
	return false
}

// IsCodeGroupOpener reconoce la apertura de un :::code-group (o
// ::::code-group), como CodeGroupParser.CanParse.
func IsCodeGroupOpener(trimmed string) bool {
	return strings.HasPrefix(trimmed, ":::code-group") || strings.HasPrefix(trimmed, "::::code-group")
}

// IsBlockOpener reconoce la apertura de un bloque ::: con tipo (":::note",
// "::: grid"…) que no es un code-group: lo que no es un ":::" pelado.
func IsBlockOpener(trimmed string) bool {
	return strings.HasPrefix(trimmed, ":::") && trimmed != ":::" && !IsCodeGroupOpener(trimmed)
}

// ContainerEnd es dónde termina el contenedor que abre en lines[start] (un
// :::code-group o un bloque ::: con tipo), buscando antes de limit: el índice
// de su ":::" de cierre y true, o el de la línea que lo corta sin cerrarlo
// (stop recibe la línea cruda y puede ser nil) o limit, y false.
//
// El cierre es el primer ":::" pelado tal que todo lo que se abrió entre la
// apertura y él cierra antes de él: cada cerca, leída con AtWithin acotada a
// ese ":::", y, en un bloque con tipo (no en un code-group, que solo tiene
// pestañas), cada bloque ::: anidado. Así un ":::" dentro de una pestaña o de
// una cerca que cierra después es código, y una pestaña que solo cerraría con
// CommonMark en otro code-group posterior no se lo traga: el grupo termina en
// su ":::" y la pestaña cierra donde cerraba antes (ClosedByMain).
//
// El costo: una cerca de cuatro backticks que guarda una línea ``` exacta y,
// después, un ":::" pelado ya no puede guardar ese ":::": el contenedor
// termina ahí y la cerca cierra en la ``` (ClosedByMain, con su aviso), que es
// como la leían las versiones anteriores.
func ContainerEnd(lines []string, start, limit int, stop func(line string) bool) (int, bool) {
	return (&containers{lines: lines, ends: map[int]int{}}).end(start, limit, stop)
}

// containers resuelve finales de contenedores de unas mismas líneas. El final
// de un contenedor anidado no depende de dónde termina el de afuera: si cierra
// antes de ese límite, cierra en el mismo ":::" que sin límite. Por eso se
// calcula una vez por apertura, sin límite (ends), y así un documento con
// muchos bloques anidados sin cerrar no es exponencial.
type containers struct {
	lines []string
	ends  map[int]int // apertura -> su ":::" sin límite, o -1 si no cierra
}

func (c *containers) end(start, limit int, stop func(line string) bool) (int, bool) {
	if limit > len(c.lines) {
		limit = len(c.lines)
	}
	if stop == nil {
		e, ok := c.ends[start]
		if !ok {
			e = -1
			if end, closed := c.scan(start, len(c.lines), nil); closed {
				e = end
			}
			c.ends[start] = e
		}
		if e >= 0 && e < limit {
			return e, true
		}
		return limit, false
	}
	return c.scan(start, limit, stop)
}

func (c *containers) scan(start, limit int, stop func(line string) bool) (int, bool) {
	nested := !IsCodeGroupOpener(strings.TrimSpace(c.lines[start]))
	for l := start + 1; l < limit; l++ {
		trimmed := strings.TrimSpace(c.lines[l])
		if stop != nil && stop(c.lines[l]) && c.wellFormed(start+1, l, nested) {
			return l, false
		}
		if trimmed == ":::" && c.wellFormed(start+1, l, nested) {
			return l, true
		}
	}
	return limit, false
}

// wellFormed dice si todo lo que se abre en lines[from:to] cierra antes de to:
// las cercas (AtWithin acotada a to) y, si nested, los bloques ::: anidados.
func (c *containers) wellFormed(from, to int, nested bool) bool {
	for k := from; k < to; k++ {
		trimmed := strings.TrimSpace(c.lines[k])
		if b, ok := AtWithin(c.lines, k, to); ok {
			if b.End < 0 {
				return false
			}
			k = b.End
			continue
		}
		if nested && (IsBlockOpener(trimmed) || IsCodeGroupOpener(trimmed)) {
			end, closed := c.end(k, to, nil)
			if !closed {
				return false
			}
			k = end
		}
	}
	return true
}

// End es el índice de la línea que cierra la cerca que abre en lines[i] (como
// la lee At sobre todo lines), len(lines) si no cierra, o -1 si lines[i] no
// abre una cerca.
func End(lines []string, i int) int {
	return EndWithin(lines, i, len(lines))
}

// EndWithin es End con el cierre buscado antes de limit (AtWithin): limit si
// no cierra.
func EndWithin(lines []string, i, limit int) int {
	b, ok := AtWithin(lines, i, limit)
	if !ok {
		return -1
	}
	if b.End < 0 {
		return b.Limit
	}
	return b.End
}

// Walk recorre lines desde from como las lee el parser y devuelve, para cada
// línea que abre una cerca, su Block (nil en las demás): cada cerca acotada a
// su contenedor (un :::code-group o un bloque ::: con tipo, ContainerEnd), y
// las líneas de adentro de una cerca no se miran. Es el recorrido de quien
// necesita saltar el código sin parsear: el normalizador, fmt y los avisos de
// cercas.
func Walk(lines []string, from int) []*Block {
	blocks := make([]*Block, len(lines))
	walkWithin(lines, from, len(lines), blocks)
	return blocks
}

func walkWithin(lines []string, from, limit int, blocks []*Block) {
	for i := from; i < limit; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if b, ok := AtWithin(lines, i, limit); ok {
			blocks[i] = &b
			if b.End < 0 {
				return
			}
			i = b.End
			continue
		}
		if IsCodeGroupOpener(trimmed) || IsBlockOpener(trimmed) {
			end, _ := ContainerEnd(lines, i, limit, nil)
			walkWithin(lines, i+1, end, blocks)
			i = end
		}
	}
}

// Ends es Walk reducido al cierre de cada cerca: el índice de la línea que la
// cierra, el final de su contenedor si no cierra, o -1 si la línea no abre una
// cerca.
func Ends(lines []string, from int) []int {
	ends := make([]int, len(lines))
	for i, b := range Walk(lines, from) {
		switch {
		case b == nil:
			ends[i] = -1
		case b.End < 0:
			ends[i] = b.Limit
		default:
			ends[i] = b.End
		}
	}
	return ends
}

// Lines marca las líneas de lines, leídas desde from, que caen en una cerca
// de código (apertura y cierre incluidos), con el recorrido de Walk.
func Lines(lines []string, from int) []bool {
	marks := make([]bool, len(lines))
	for i, b := range Walk(lines, from) {
		if b == nil {
			continue
		}
		end := b.End
		if end < 0 {
			end = b.Limit - 1
		}
		for j := i; j <= end; j++ {
			marks[j] = true
		}
	}
	return marks
}
