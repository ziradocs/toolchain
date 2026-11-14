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
// contenedor, y si una cerca quedaría sin cerrar la leen como las versiones
// anteriores (abría toda línea que empieza con ``` y cerraba en la primera
// ``` exacta), para que ningún documento que esas versiones leían cambie de
// significado sin aviso. Una línea con un backtick en el info string (```js`
// …) es texto, salvo que como texto el resto deje una cerca sin cerrar.
//
// La usan los parsers de elementos que deciden dónde empieza o termina una
// cerca: el de código y el de code-group, los de diagramas y datos en cerca
// (mermaid, plantuml, chart, map), los cortes de prosa, de listas, de citas y
// de captions de imagen, IsNewElement y el despacho del registry, y el
// recorrido de core/include que decide qué deja abierto un archivo incluido.
// Las cercas ```math siguen su propia regla (elements.closesMathFence). El
// normalizador, fmt, el renderer y el pre-pase de node-id todavía no la usan.
package fence

import "strings"

// Open devuelve cuántos backticks abren la cerca de trimmed (una línea ya
// recortada), o 0 si la línea no abre una: tres o más backticks y un info
// string sin backticks.
func Open(trimmed string) int {
	n := run(trimmed)
	if n < 3 || strings.Contains(trimmed[n:], "`") {
		return 0
	}
	return n
}

// Info es el info string de una línea que abre una cerca: lo que sigue a los
// backticks, recortado.
func Info(trimmed string) string {
	return strings.TrimSpace(trimmed[run(trimmed):])
}

// Closes dice si trimmed cierra una cerca abierta con open backticks: solo
// backticks, al menos tantos como la apertura.
func Closes(trimmed string, open int) bool {
	return open > 0 && len(trimmed) >= open && run(trimmed) == len(trimmed)
}

// Differs dice si trimmed es una línea que la regla de CommonMark y la de
// las versiones anteriores pueden leer distinto: una que empieza con ``` y no
// es ni una apertura de tres backticks con un info string sin backticks ni
// una ``` exacta (una cerca de cuatro o más, un cierre de cuatro o más, un
// backtick en el info string). Un texto sin ninguna de estas líneas se lee
// igual con las dos reglas.
func Differs(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "```") {
		return false
	}
	return run(trimmed) != 3 || strings.Contains(trimmed[3:], "`")
}

// run es cuántos backticks empiezan trimmed.
func run(trimmed string) int {
	n := 0
	for n < len(trimmed) && trimmed[n] == '`' {
		n++
	}
	return n
}

// Reading es cómo se leyó una cerca (ver AtWithin).
type Reading int

const (
	// CommonMark: la cerca cierra con la regla de CommonMark.
	CommonMark Reading = iota
	// ClosedByMain: la cerca abre con la regla de CommonMark pero no la
	// cierra ninguna línea de solo backticks con al menos tantos; cierra en
	// la primera línea ``` exacta, como en las versiones anteriores.
	ClosedByMain
	// Unclosed: no la cierra ninguna de las dos reglas; llega al final de su
	// contenedor.
	Unclosed
	// OpenedByMain: la línea lleva un backtick en el info string, así que
	// para CommonMark es texto, pero leída como texto el resto dejaría una
	// cerca sin cerrar; se lee como en las versiones anteriores, como una
	// cerca que cierra en la primera línea ``` exacta (o no cierra).
	OpenedByMain
	// Earlier: el documento entero se lee con la regla de las versiones
	// anteriores (Earlier), porque con la de CommonMark tiene un error que
	// esa regla no tiene.
	Earlier
)

// EarlierAt es la cerca de lines[i] con la regla de las versiones anteriores,
// sin contenedor: abre toda línea que empieza con ``` y cierra la primera
// línea después de ella para la que closes dice que sí (una ``` exacta para
// un bloque de código; ``` o ```` para una cerca de diagrama o de datos), o
// llega al final de lines. Ticks es 3: esas versiones leían el info string
// después de tres backticks.
func EarlierAt(lines []string, i int, closes func(trimmed string) bool) (Block, bool) {
	if i < 0 || i >= len(lines) || !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
		return Block{}, false
	}
	b := Block{Start: i, Ticks: 3, End: -1, Limit: len(lines), Reading: Earlier, MainEnd: -1}
	for j := i + 1; j < len(lines); j++ {
		if closes(strings.TrimSpace(lines[j])) {
			b.End, b.MainEnd = j, j
			break
		}
	}
	return b, true
}

// Block es una cerca de código de lines que abre en la línea Start.
type Block struct {
	Start int
	// Ticks son los backticks de la apertura.
	Ticks int
	// End es el índice de la línea que la cierra, o -1 si llega al final de
	// su contenedor (Limit).
	End int
	// Limit es el final (exclusivo) del contenedor en el que se buscó el
	// cierre: len(lines) para el documento, o el ":::" del code-group o del
	// bloque que la contiene.
	Limit   int
	Reading Reading
	// MainEnd es donde la cerraba la regla de las versiones anteriores
	// (cierra la primera ``` exacta), buscando en el mismo contenedor: el
	// índice de esa línea o -1.
	MainEnd int
}

// Index lee las cercas de unas mismas líneas y recuerda lo que ya calculó: si
// el resto de un contenedor, leído desde una línea, deja una cerca sin cerrar
// con cada regla, y dónde termina cada contenedor. Sin él, cada pregunta desde
// una línea distinta vuelve a recorrer el resto, y un documento con muchas
// líneas que empiezan con ``` es cuadrático. Las funciones del paquete que
// reciben lines usan un Index nuevo en cada llamada.
type Index struct {
	lines   []string
	trimmed []string
	// cm y main: si leer desde una línea hasta el final con CommonMark o con
	// la regla anterior deja una cerca sin cerrar (0 sin calcular, 1 no, 2
	// sí). Solo hasta el final: un contenedor es corto y se recorre sin
	// memoria.
	cm, main []int8
	// ends: apertura de un contenedor -> su ":::" sin límite, o -1 si no
	// cierra (ver ContainerEnd).
	ends map[int]int
}

// NewIndex es el Index de lines. lines no debe cambiar mientras se use.
func NewIndex(lines []string) *Index {
	trimmed := make([]string, len(lines))
	for i, l := range lines {
		trimmed[i] = strings.TrimSpace(l)
	}
	return &Index{lines: lines, trimmed: trimmed, ends: map[int]int{}}
}

// Of dice si x es el Index de lines (las mismas líneas, no una copia).
func (x *Index) Of(lines []string) bool {
	return x != nil && len(x.lines) == len(lines) && (len(lines) == 0 || &x.lines[0] == &lines[0])
}

// At es AtWithin sobre todo lines: la cerca de un documento sin contenedor.
func At(lines []string, i int) (Block, bool) {
	return NewIndex(lines).AtWithin(i, len(lines))
}

// AtWithin es Index.AtWithin con un Index nuevo.
func AtWithin(lines []string, i, limit int) (Block, bool) {
	return NewIndex(lines).AtWithin(i, limit)
}

// AtWithin dice si la línea i abre una cerca de código y hasta dónde llega,
// buscando el cierre solo antes de limit: el final de su contenedor (el
// documento, o el ":::" del code-group o del bloque que la contiene, ver
// ContainerEnd). La regla de CommonMark (Open, Closes) aplica cuando da un
// resultado bien formado dentro del contenedor. Si una cerca de CommonMark no
// cierra antes de limit, cierra donde cerraba en las versiones anteriores si
// esa línea está antes de limit (ClosedByMain) o llega a limit (Unclosed); y
// una línea con un backtick en el info string, que para CommonMark es texto,
// se lee como cerca si como texto el resto del contenedor dejaría una cerca
// sin cerrar (OpenedByMain).
func (x *Index) AtWithin(i, limit int) (Block, bool) {
	if limit > len(x.lines) {
		limit = len(x.lines)
	}
	if i < 0 || i >= limit {
		return Block{}, false
	}
	trimmed := x.trimmed[i]
	if !strings.HasPrefix(trimmed, "```") {
		return Block{}, false
	}
	if n := Open(trimmed); n > 0 {
		mainEnd := x.exactClose(i, limit)
		if end := x.closing(i, n, limit); end >= 0 {
			return Block{Start: i, Ticks: n, End: end, Limit: limit, Reading: CommonMark, MainEnd: mainEnd}, true
		}
		if mainEnd >= 0 {
			return Block{Start: i, Ticks: n, End: mainEnd, Limit: limit, Reading: ClosedByMain, MainEnd: mainEnd}, true
		}
		return Block{Start: i, Ticks: n, End: -1, Limit: limit, Reading: Unclosed, MainEnd: -1}, true
	}
	if !x.leavesUnclosed(i+1, limit) {
		return Block{}, false
	}
	mainEnd := x.exactClose(i, limit)
	return Block{Start: i, Ticks: run(trimmed), End: mainEnd, Limit: limit, Reading: OpenedByMain, MainEnd: mainEnd}, true
}

// TabWithin es Index.TabWithin con un Index nuevo.
func TabWithin(lines []string, i, limit int) (Block, bool) {
	return NewIndex(lines).TabWithin(i, limit)
}

// TabWithin es AtWithin para una pestaña de un :::code-group: ahí una línea
// que empieza con ``` siempre abre una pestaña, como en las versiones
// anteriores, porque una línea de texto no tiene lugar en el grupo. Una con un
// backtick en el info string se lee como OpenedByMain.
func (x *Index) TabWithin(i, limit int) (Block, bool) {
	if b, ok := x.AtWithin(i, limit); ok {
		return b, true
	}
	if limit > len(x.lines) {
		limit = len(x.lines)
	}
	if i < 0 || i >= limit || !strings.HasPrefix(x.trimmed[i], "```") {
		return Block{}, false
	}
	mainEnd := x.exactClose(i, limit)
	return Block{Start: i, Ticks: run(x.trimmed[i]), End: mainEnd, Limit: limit, Reading: OpenedByMain, MainEnd: mainEnd}, true
}

// Consumed son las líneas que ocupa b: apertura, cuerpo y cierre, o hasta el
// final de su contenedor si no cierra.
func (b Block) Consumed() int {
	if b.End < 0 {
		return b.Limit - b.Start
	}
	return b.End - b.Start + 1
}

// Body son las líneas del cuerpo de b en lines, sin la apertura ni el cierre.
func (b Block) Body(lines []string) []string {
	end := b.End
	if end < 0 {
		end = b.Limit
	}
	return lines[b.Start+1 : end]
}

// exactClose es la primera línea después de i y antes de limit que es una ```
// exacta (la regla de las versiones anteriores), o -1.
func (x *Index) exactClose(i, limit int) int {
	for j := i + 1; j < limit; j++ {
		if x.trimmed[j] == "```" {
			return j
		}
	}
	return -1
}

// closing es la primera línea después de i y antes de limit que cierra con la
// regla de CommonMark una cerca de n backticks, o -1.
func (x *Index) closing(i, n, limit int) int {
	for j := i + 1; j < limit; j++ {
		if Closes(x.trimmed[j], n) {
			return j
		}
	}
	return -1
}

// leavesUnclosed dice si, leyendo desde from hasta limit solo con la regla de
// CommonMark, alguna cerca queda sin cerrar.
func (x *Index) leavesUnclosed(from, limit int) bool {
	return x.unclosed(&x.cm, from, limit, func(j int) (bool, int) {
		if n := Open(x.trimmed[j]); n > 0 {
			return true, x.closing(j, n, limit)
		}
		return false, 0
	})
}

// MainLeavesUnclosed es Index.MainLeavesUnclosed con un Index nuevo.
func MainLeavesUnclosed(lines []string, from, limit int) bool {
	return NewIndex(lines).MainLeavesUnclosed(from, limit)
}

// MainLeavesUnclosed dice si, leyendo desde from hasta limit con la regla de
// las versiones anteriores (abre toda línea que empieza con ```, cierra la
// primera ``` exacta), alguna cerca queda sin cerrar. Si no queda ninguna,
// esas versiones leían ese tramo como un texto bien formado, y una lectura
// distinta es un cambio que se avisa.
func (x *Index) MainLeavesUnclosed(from, limit int) bool {
	if limit > len(x.lines) {
		limit = len(x.lines)
	}
	return x.unclosed(&x.main, from, limit, func(j int) (bool, int) {
		if strings.HasPrefix(x.trimmed[j], "```") {
			return true, x.exactClose(j, limit)
		}
		return false, 0
	})
}

// unclosed recorre desde from hasta limit con la regla open (si la línea j
// abre una cerca y dónde cierra, o -1) y dice si alguna queda sin cerrar.
// Leer desde una línea que no está dentro de una cerca da lo mismo que leer
// desde la siguiente de ese recorrido, así que, hasta el final de las líneas,
// memo guarda la respuesta de cada línea por la que pasó.
func (x *Index) unclosed(memo *[]int8, from, limit int, open func(j int) (bool, int)) bool {
	if limit != len(x.lines) {
		for j := from; j < limit; j++ {
			if opens, end := open(j); opens {
				if end < 0 {
					return true
				}
				j = end
			}
		}
		return false
	}
	if *memo == nil {
		*memo = make([]int8, len(x.lines))
	}
	m := *memo
	var path []int
	result := false
	for j := from; j < limit; j++ {
		if m[j] != 0 {
			result = m[j] == 2
			break
		}
		path = append(path, j)
		if opens, end := open(j); opens {
			if end < 0 {
				result = true
				break
			}
			j = end
		}
	}
	v := int8(1)
	if result {
		v = 2
	}
	for _, j := range path {
		m[j] = v
	}
	return result
}

// MainHolds dice si, leyendo con la regla de las versiones anteriores desde
// from (fuera de toda cerca), la línea l cae dentro de una cerca que esa regla
// cierra, sin ser su apertura. Es lo que distingue una cerca de cuatro
// backticks que esas versiones leían mal (su línea de cierre les abría otra
// cerca) de una cuya extensión cambia de verdad (su línea de cierre era
// contenido de una cerca de antes).
func (x *Index) MainHolds(from, l int) bool {
	for j := from; j < len(x.lines) && j <= l; j++ {
		if !strings.HasPrefix(x.trimmed[j], "```") {
			continue
		}
		if j == l {
			return false
		}
		c := x.exactClose(j, len(x.lines))
		if c < 0 {
			return false
		}
		if l <= c {
			return true
		}
		j = c
	}
	return false
}

// HasExactClose dice si alguna línea después de i es una ``` exacta: la
// regla de las versiones anteriores cerraba ahí la cerca de i.
func (x *Index) HasExactClose(i int) bool {
	return x.exactClose(i, len(x.lines)) >= 0
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

// ContainerEnd es Index.ContainerEnd con un Index nuevo.
func ContainerEnd(lines []string, start, limit int, stop func(line string) bool) (int, bool) {
	return NewIndex(lines).ContainerEnd(start, limit, stop)
}

// ContainerEnd es dónde termina el contenedor que abre en la línea start (un
// :::code-group o un bloque ::: con tipo), buscando antes de limit: el índice
// de su ":::" de cierre y true, o el de la línea que lo corta sin cerrarlo
// (stop recibe la línea cruda y puede ser nil) o limit, y false.
//
// El cierre es el primer ":::" pelado tal que todo lo que se abrió entre la
// apertura y él cierra antes de él: cada cerca, leída con AtWithin acotada a
// ese ":::" (TabWithin en un code-group), y, en un bloque con tipo (no en un
// code-group, que solo tiene pestañas), cada bloque ::: anidado. Así un ":::"
// dentro de una pestaña o de una cerca que cierra después es código, y una
// pestaña que solo cerraría con CommonMark en otro code-group posterior no se
// lo traga: el grupo termina en su ":::" y la pestaña cierra donde cerraba
// antes (ClosedByMain).
//
// El costo: una cerca de cuatro backticks que guarda una línea ``` exacta y,
// después, un ":::" pelado ya no puede guardar ese ":::": el contenedor
// termina ahí y la cerca cierra en la ``` (ClosedByMain, con su aviso), que es
// como la leían las versiones anteriores.
//
// El final de un contenedor anidado no depende de dónde termina el de afuera:
// si cierra antes de ese límite, cierra en el mismo ":::" que sin límite. Por
// eso se calcula una vez por apertura, sin límite, y así un documento con
// muchos bloques anidados sin cerrar no es exponencial.
func (x *Index) ContainerEnd(start, limit int, stop func(line string) bool) (int, bool) {
	if limit > len(x.lines) {
		limit = len(x.lines)
	}
	if stop == nil {
		e, ok := x.ends[start]
		if !ok {
			e = -1
			if end, closed := x.scan(start, len(x.lines), nil); closed {
				e = end
			}
			x.ends[start] = e
		}
		if e >= 0 && e < limit {
			return e, true
		}
		return limit, false
	}
	return x.scan(start, limit, stop)
}

func (x *Index) scan(start, limit int, stop func(line string) bool) (int, bool) {
	nested := !IsCodeGroupOpener(x.trimmed[start])
	for l := start + 1; l < limit; l++ {
		if stop != nil && stop(x.lines[l]) && x.wellFormed(start+1, l, nested) {
			return l, false
		}
		if x.trimmed[l] == ":::" && x.wellFormed(start+1, l, nested) {
			return l, true
		}
	}
	return limit, false
}

// wellFormed dice si todo lo que se abre en las líneas [from, to) cierra antes
// de to: las cercas (AtWithin acotada a to, TabWithin si no nested) y, si
// nested, los bloques ::: anidados.
func (x *Index) wellFormed(from, to int, nested bool) bool {
	at := x.AtWithin
	if !nested {
		at = x.TabWithin
	}
	for k := from; k < to; k++ {
		if b, ok := at(k, to); ok {
			if b.End < 0 {
				return false
			}
			k = b.End
			continue
		}
		if nested && (IsBlockOpener(x.trimmed[k]) || IsCodeGroupOpener(x.trimmed[k])) {
			end, closed := x.ContainerEnd(k, to, nil)
			if !closed {
				return false
			}
			k = end
		}
	}
	return true
}

// Walk es Index.Walk con un Index nuevo.
func Walk(lines []string, from int) []*Block {
	return NewIndex(lines).Walk(from)
}

// Walk recorre las líneas desde from como las lee el parser y devuelve, para
// cada línea que abre una cerca, su Block (nil en las demás): cada cerca
// acotada a su contenedor (un :::code-group o un bloque ::: con tipo,
// ContainerEnd), y las líneas de adentro de una cerca no se miran. Es el
// recorrido de quien necesita saltar el código sin parsear, como el aviso de
// las líneas que antes abrían una cerca y ahora son texto.
func (x *Index) Walk(from int) []*Block {
	blocks := make([]*Block, len(x.lines))
	x.walkWithin(from, len(x.lines), false, blocks)
	return blocks
}

// walkWithin recorre las líneas [from, limit); tabs dice que son las pestañas
// de un code-group (TabWithin).
func (x *Index) walkWithin(from, limit int, tabs bool, blocks []*Block) {
	at := x.AtWithin
	if tabs {
		at = x.TabWithin
	}
	for i := from; i < limit; i++ {
		if b, ok := at(i, limit); ok {
			blocks[i] = &b
			if b.End < 0 {
				return
			}
			i = b.End
			continue
		}
		// Un code-group solo tiene pestañas: un ":::" con tipo adentro es una
		// línea suelta, no un contenedor.
		if trimmed := x.trimmed[i]; !tabs && (IsCodeGroupOpener(trimmed) || IsBlockOpener(trimmed)) {
			end, _ := x.ContainerEnd(i, limit, nil)
			x.walkWithin(i+1, end, IsCodeGroupOpener(trimmed), blocks)
			i = end
		}
	}
}

// Lines es Index.Lines con un Index nuevo.
func Lines(lines []string, from int) []bool {
	return NewIndex(lines).Lines(from)
}

// Lines marca las líneas, leídas desde from, que caen en una cerca de código
// (apertura y cierre incluidos), con el recorrido de Walk.
func (x *Index) Lines(from int) []bool {
	marks := make([]bool, len(x.lines))
	for i, b := range x.Walk(from) {
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
