// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ScanLinkDestination recorre el destino de un enlace o imagen inline. s
// empieza justo DESPUÉS del "(" de apertura y la sintaxis es la de
// CommonMark: el destino va entre <...> (puede llevar espacios) o es una
// corrida sin espacios cuyos paréntesis internos están balanceados, así que
// https://es.wikipedia.org/wiki/Foo_(bar) y javascript:alert(1) se leen
// completos. Lo puede seguir, separado por espacios, un título entre
// comillas dobles, simples o paréntesis; después solo cabe el ")" de cierre.
//
// Compatibilidad: un destino sin <...> con espacios que no abren un título
// válido (`![a](mi foto.png)`, `[a](t.png otra)`) no es enlace en
// CommonMark, pero antes de esta gramática sí lo era: todo el texto hasta el
// ")" que balancea los paréntesis era la ruta. Se sigue leyendo así, para
// no perder en silencio imágenes y enlaces que ya funcionaban, y
// ParseLinkDestination lo marca (Spaced) para que el linter avise (LINK001)
// y sugiera <...> o %20.
//
// Devuelve el texto crudo entre los paréntesis externos (destino, <...> y
// título incluidos, como lo escribió el autor) y cuántos bytes de s consumió,
// incluido el ")" de cierre. ParseLinkDestination separa el destino del
// título. ok es false cuando la sintaxis no es un enlace y el texto se deja
// literal: sin ")" de cierre en la misma línea o con paréntesis
// desbalanceados. Un "()" vacío tampoco es enlace (regla anterior a esta
// gramática); "( )" sí, con destino vacío.
//
// Exportado para que los generadores que leen el Markdown inline por su
// cuenta (el DOCX de doclang) reconozcan los mismos destinos que el
// renderer HTML en vez de mantener su propia copia del patrón.
func ScanLinkDestination(s string) (dest string, consumed int, ok bool) {
	parts, ok := scanLinkParts(s, nil)
	if !ok {
		return "", 0, false
	}
	return s[:parts.consumed-1], parts.consumed, true
}

// LinkDestination es un destino de enlace o imagen ya separado en sus
// partes; ver ParseLinkDestination.
type LinkDestination struct {
	// Dest es el destino sin los <...> ni el título, tal como lo escribió
	// el autor (sin decodificar escapes ni referencias de carácter: es una
	// URL, y las rutas locales se decodifican al leer el archivo, ver
	// LocalPathFromURL).
	Dest string
	// Title es el título con sus escapes de barra invertida resueltos, o ""
	// si no lleva.
	Title string
	// Consumed son los bytes leídos, incluido el ")" de cierre.
	Consumed int
	// Spaced dice que el destino es una ruta suelta con espacios, leída con
	// la regla anterior a CommonMark (ver ScanLinkDestination): funciona,
	// pero otras herramientas no la leen como enlace. Se escribe <...> o %20.
	Spaced bool
}

// ParseLinkDestination es ScanLinkDestination con el destino y el título por
// separado.
func ParseLinkDestination(s string) (LinkDestination, bool) {
	parts, ok := scanLinkParts(s, nil)
	if !ok {
		return LinkDestination{}, false
	}
	d := LinkDestination{Dest: s[parts.destStart:parts.destEnd], Consumed: parts.consumed, Spaced: parts.spaced}
	if parts.titleStart >= 0 {
		d.Title = unescapeBackslashes(s[parts.titleStart:parts.titleEnd])
	}
	return d, true
}

// linkParts son los índices, dentro del texto que sigue al "(", del destino
// y del título de un enlace; titleStart es -1 cuando no hay título. spaced
// marca la lectura de compatibilidad (ver ScanLinkDestination).
type linkParts struct {
	destStart, destEnd   int
	titleStart, titleEnd int
	consumed             int
	spaced               bool
}

// scanLinkParts implementa la gramática de ScanLinkDestination sobre s.
// closeAt, si no es nil, devuelve el índice del ")" que balancea los
// paréntesis desde el inicio de s (o -1): lo precalcula linkFinder para que
// la lectura de compatibilidad no recorra la línea entera en cada intento.
func scanLinkParts(s string, closeAt func() int) (linkParts, bool) {
	p, ok := scanCommonMarkLinkParts(s)
	if ok {
		return p, true
	}
	// Lectura de compatibilidad: solo para un destino sin <...>.
	i := skipSpaceTab(s, 0)
	if i < len(s) && s[i] == '<' {
		return p, false
	}
	end := -1
	if closeAt != nil {
		end = closeAt()
	} else {
		end = balancedClose(s)
	}
	if end <= 0 {
		return p, false
	}
	start, stop := i, end
	for stop > start && (s[stop-1] == ' ' || s[stop-1] == '\t') {
		stop--
	}
	return linkParts{destStart: start, destEnd: stop, titleStart: -1, titleEnd: -1, consumed: end + 1, spaced: true}, true
}

// balancedClose devuelve el índice del primer ")" de s que no cierra un "("
// abierto dentro de s, o -1 si antes hay un salto de línea o se acaba s.
func balancedClose(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n', '\r':
			return -1
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// scanCommonMarkLinkParts es la gramática estricta de CommonMark.
func scanCommonMarkLinkParts(s string) (linkParts, bool) {
	p := linkParts{titleStart: -1, titleEnd: -1}
	i := skipSpaceTab(s, 0)
	if i < len(s) && s[i] == '<' {
		j := i + 1
		for ; ; j++ {
			if j >= len(s) {
				return p, false
			}
			c := s[j]
			if c == '\\' && j+1 < len(s) && isASCIIPunct(s[j+1]) {
				j++
				continue
			}
			if c == '\n' || c == '\r' || c == '<' {
				return p, false
			}
			if c == '>' {
				break
			}
		}
		p.destStart, p.destEnd = i+1, j
		i = j + 1
	} else {
		depth := 0
		j := i
	loop:
		for ; j < len(s); j++ {
			c := s[j]
			switch {
			case c == '\\' && j+1 < len(s) && isASCIIPunct(s[j+1]):
				j++
			case c == ' ' || c == '\t':
				break loop
			case c < 0x20 || c == 0x7f:
				return p, false
			case c == '(':
				depth++
			case c == ')':
				if depth == 0 {
					break loop
				}
				depth--
			}
		}
		if j >= len(s) || depth != 0 {
			return p, false
		}
		p.destStart, p.destEnd = i, j
		i = j
	}
	if p.destStart == p.destEnd {
		// "()" no es enlace, como antes; "( )" sí, con destino vacío (como
		// en CommonMark), y el filtro de URLs lo descarta: queda la etiqueta.
		if i == 0 || i >= len(s) || s[i] != ')' {
			return p, false
		}
		p.consumed = i + 1
		return p, true
	}
	k := skipSpaceTab(s, i)
	if k < len(s) && s[k] == ')' {
		p.consumed = k + 1
		return p, true
	}
	if k == i || k >= len(s) {
		return p, false
	}
	closer := s[k]
	switch closer {
	case '"', '\'':
	case '(':
		closer = ')'
	default:
		return p, false
	}
	t := k + 1
	for ; ; t++ {
		if t >= len(s) {
			return p, false
		}
		c := s[t]
		if c == '\\' && t+1 < len(s) && isASCIIPunct(s[t+1]) {
			t++
			continue
		}
		if c == '\n' || c == '\r' || (s[k] == '(' && c == '(') {
			return p, false
		}
		if c == closer {
			break
		}
	}
	p.titleStart, p.titleEnd = k+1, t
	m := skipSpaceTab(s, t+1)
	if m >= len(s) || s[m] != ')' {
		return p, false
	}
	p.consumed = m + 1
	return p, true
}

func skipSpaceTab(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

func isASCIIPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}

// SpacedDestinationSuggestion es la forma que el aviso LINK001 sugiere para
// una ruta suelta con espacios (LinkDestination.Spaced): la ruta entre <...>
// y, si dest termina en un título bien formado de CommonMark ("…", '…' o
// (…), con sus escapes, separado por espacios), el título afuera:
// `mi dir/x.png "T"` sugiere `<mi dir/x.png> "T"`, no `<mi dir/x.png "T">`,
// que haría del título parte de la ruta. `a (1).png` no lleva título (después
// del ")" sigue texto) y sugiere `<a (1).png>`.
func SpacedDestinationSuggestion(dest string) string {
	if path, rest, ok := splitSpacedTitle(dest); ok {
		return "<" + path + ">" + rest
	}
	return "<" + dest + ">"
}

// splitSpacedTitle separa de p un título bien formado al final, precedido de
// espacios; ver SpacedDestinationSuggestion. Devuelve la ruta y el resto (los
// espacios y el título, tal cual).
func splitSpacedTitle(p string) (path, rest string, ok bool) {
	n := len(p)
	if n < 2 {
		return "", "", false
	}
	escaped := make([]bool, n)
	for i := 0; i < n; i++ {
		if p[i] == '\\' && i+1 < n && isASCIIPunct(p[i+1]) {
			escaped[i+1] = true
			i++
		}
	}
	if escaped[n-1] {
		return "", "", false
	}
	var open byte
	switch p[n-1] {
	case '"', '\'':
		open = p[n-1]
	case ')':
		open = '('
	default:
		return "", "", false
	}
	k := -1
	for i := n - 2; i >= 0; i-- {
		if escaped[i] {
			continue
		}
		if p[i] == '\n' || p[i] == '\r' {
			return "", "", false
		}
		if p[i] == open {
			k = i
			break
		}
		// Un título entre paréntesis no lleva "(" ni ")" sin escapar.
		if open == '(' && p[i] == ')' {
			return "", "", false
		}
	}
	if k <= 0 || (p[k-1] != ' ' && p[k-1] != '\t') {
		return "", "", false
	}
	path = strings.TrimRight(p[:k], " \t")
	if path == "" {
		return "", "", false
	}
	return path, p[len(path):], true
}

// unescapeBackslashes quita la barra invertida de cada escape de puntuación
// ASCII (\" -> "), como CommonMark hace en los títulos.
func unescapeBackslashes(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isASCIIPunct(s[i+1]) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// escapedLinkView traduce el texto que ve el pipeline inline (ya pasado por
// EscapeHTML, con centinelas "<zdcN/>"/"<zdeN/>" crudos) a la forma que lee
// scanLinkParts: cada &lt; &gt; &quot; &#39; &amp; vuelve a ser su carácter,
// y un "<", ">", "\"" o "'" crudo (solo pueden venir de un centinela o de un
// tag que insertó una pasada anterior) pasa a "a", para que no se lea como
// sintaxis de destino. offsets[k] es el índice en el texto original del byte
// k de la vista; offsets[len(view)] es len(text).
func escapedLinkView(text string) (view string, offsets []int) {
	var b strings.Builder
	b.Grow(len(text))
	offsets = make([]int, 0, len(text)+1)
	for i := 0; i < len(text); {
		c := text[i]
		if c == '&' {
			matched := false
			for _, e := range [...]struct {
				ent string
				ch  byte
			}{{"&lt;", '<'}, {"&gt;", '>'}, {"&quot;", '"'}, {"&#39;", '\''}, {"&amp;", '&'}} {
				if strings.HasPrefix(text[i:], e.ent) {
					b.WriteByte(e.ch)
					offsets = append(offsets, i)
					i += len(e.ent)
					matched = true
					break
				}
			}
			if matched {
				continue
			}
		}
		if c == '<' || c == '>' || c == '"' || c == '\'' {
			c = 'a'
		}
		b.WriteByte(c)
		offsets = append(offsets, i)
		i++
	}
	offsets = append(offsets, len(text))
	return b.String(), offsets
}

// linkFinder busca enlaces (o imágenes, con image) en un texto completo, en
// tiempo lineal en el largo del texto, también cuando las aperturas no
// cierran ("[a](b" mil veces en una línea). Es la misma búsqueda que hacía
// el regex de apertura (inlineLinkPattern o inlineImagePattern) desde cada
// candidato seguido de scanLinkParts sobre el resto de la vista, con el
// mismo resultado; lo que cambia es que nada se recorre dos veces:
//
//   - La vista desescapada y sus offsets se arman una sola vez por texto.
//   - La apertura se busca a mano y el "]" que cierra la etiqueta se
//     recuerda: con el regex, "[[[…](b" recorría la etiqueta entera desde
//     cada "[".
//   - Un destino suelto que no cierra deja guardada la región que recorrió
//     (bareRegion), con la profundidad de paréntesis de cada byte y el ")"
//     que balancea desde cada uno. Las aperturas siguientes que caen adentro
//     se resuelven con esa tabla en vez de recorrer la región otra vez.
//   - La lectura de compatibilidad (ruta suelta con espacios) guarda igual
//     su región (looseRegion).
//
// Todo se calcula a medida que hace falta, no al crear el buscador: un
// buscador que encuentra un enlace cerca del principio no paga el resto del
// texto. Las memorias viven en el buscador, así que solo sirven a quien
// recorre un texto con el mismo buscador; FindInlineLinkIndex crea uno por
// llamada (ver su comentario).
//
// Las memorias suponen que las búsquedas avanzan, que es como lo usan
// replaceInlineLinks y hideLinkDestinations (cada find empieza donde terminó
// el enlace anterior). Una búsqueda hacia atrás da el mismo resultado, solo
// que sin aprovecharlas.
type linkFinder struct {
	text    string
	image   bool
	escaped bool
	view    string
	offsets []int // vista -> texto (solo escaped)
	toView  []int // texto -> vista, -1 dentro de una entidad (solo escaped)

	// rbAt es el primer "]" de text desde rbFrom (len(text) si no hay).
	rbFrom, rbAt int
	rbOK         bool

	bare  bareRegion
	loose looseRegion
	attrs attrsScanner

	// steps cuenta los bytes que recorren los escaneos. Solo lo leen las
	// pruebas de escala, que así comparan trabajo y no tiempo.
	steps int
}

func newLinkFinder(text string, image, escaped bool) *linkFinder {
	f := &linkFinder{text: text, image: image, escaped: escaped, view: text}
	if escaped {
		f.view, f.offsets = escapedLinkView(text)
		f.toView = make([]int, len(text)+1)
		for i := range f.toView {
			f.toView[i] = -1
		}
		for k, o := range f.offsets {
			f.toView[o] = k
		}
	}
	f.attrs.text, f.attrs.quote = text, "&quot;"
	return f
}

// nextOpen busca desde search la primera apertura "[etiqueta](" (o
// "![alt](" con image): lo mismo que FindStringSubmatchIndex de
// inlineLinkPattern o inlineImagePattern sobre text[search:], en índices
// absolutos. La etiqueta de un enlace no puede ser vacía; la de una imagen
// sí. Ninguna de las dos lleva "]".
func (f *linkFinder) nextOpen(search int) (start, labelStart, labelEnd, openEnd int, ok bool) {
	t := f.text
	for b := search; b < len(t); {
		k := strings.IndexByte(t[b:], '[')
		f.steps += k + 1
		if k < 0 {
			break
		}
		b += k
		start, labelStart = b, b+1
		b++
		if f.image {
			if start == search || t[start-1] != '!' {
				continue
			}
			start--
		}
		e := f.nextRB(labelStart)
		if e >= len(t) {
			// Sin "]" de aquí en adelante, ninguna apertura posterior cierra.
			break
		}
		if !f.image && e == labelStart {
			continue
		}
		if e+1 < len(t) && t[e+1] == '(' {
			return start, labelStart, e, e + 2, true
		}
	}
	return 0, 0, 0, 0, false
}

// nextRB devuelve el índice del primer "]" de text desde x, o len(text).
func (f *linkFinder) nextRB(x int) int {
	if !f.rbOK || x < f.rbFrom || x > f.rbAt {
		k := strings.IndexByte(f.text[x:], ']')
		if k < 0 {
			f.rbAt = len(f.text)
			f.steps += len(f.text) - x
		} else {
			f.rbAt = x + k
			f.steps += k + 1
		}
		f.rbFrom, f.rbOK = x, true
	}
	return f.rbAt
}

// scanParts es scanLinkParts(f.view[v:], …) con las memorias del buscador.
// Los índices de linkParts son relativos a v, como los de scanLinkParts.
func (f *linkFinder) scanParts(v int) (linkParts, bool) {
	p, ok := f.strictParts(v)
	if ok {
		return p, true
	}
	// Lectura de compatibilidad: solo para un destino sin <...>.
	s := f.view
	i := skipSpaceTab(s, v)
	if i < len(s) && s[i] == '<' {
		return p, false
	}
	end := f.looseClose(v)
	if end <= 0 {
		return p, false
	}
	start, stop := i-v, end
	rel := s[v:]
	for stop > start && (rel[stop-1] == ' ' || rel[stop-1] == '\t') {
		stop--
	}
	return linkParts{destStart: start, destEnd: stop, titleStart: -1, titleEnd: -1, consumed: end + 1, spaced: true}, true
}

// strictParts es scanCommonMarkLinkParts(f.view[v:]). El destino suelto lo
// lee bareEnd; los demás tramos (<...>, el título, los espacios) no hace
// falta guardarlos: cada uno termina, a lo sumo, donde empieza el mismo tramo
// de la apertura siguiente que llega hasta ahí, así que entre todas las
// aperturas de una línea se recorren una vez.
func (f *linkFinder) strictParts(v int) (linkParts, bool) {
	s := f.view
	p := linkParts{titleStart: -1, titleEnd: -1}
	i := skipSpaceTab(s, v)
	f.steps += i - v
	var destStart, destEnd int
	if i < len(s) && s[i] == '<' {
		j := i + 1
		for ; ; j++ {
			if j >= len(s) {
				f.steps += j - i
				return p, false
			}
			c := s[j]
			if c == '\\' && j+1 < len(s) && isASCIIPunct(s[j+1]) {
				j++
				continue
			}
			if c == '\n' || c == '\r' || c == '<' {
				f.steps += j - i
				return p, false
			}
			if c == '>' {
				break
			}
		}
		f.steps += j - i
		destStart, destEnd = i+1, j
		i = j + 1
	} else {
		j, ok := f.bareEnd(i)
		if !ok {
			return p, false
		}
		destStart, destEnd = i, j
		i = j
	}
	p.destStart, p.destEnd = destStart-v, destEnd-v
	if destStart == destEnd {
		// "()" no es enlace, como antes; "( )" sí, con destino vacío.
		if i == v || i >= len(s) || s[i] != ')' {
			return p, false
		}
		p.consumed = i + 1 - v
		return p, true
	}
	k := skipSpaceTab(s, i)
	f.steps += k - i
	if k < len(s) && s[k] == ')' {
		p.consumed = k + 1 - v
		return p, true
	}
	if k == i || k >= len(s) {
		return p, false
	}
	closer := s[k]
	switch closer {
	case '"', '\'':
	case '(':
		closer = ')'
	default:
		return p, false
	}
	t := k + 1
	for ; ; t++ {
		if t >= len(s) {
			f.steps += t - k
			return p, false
		}
		c := s[t]
		if c == '\\' && t+1 < len(s) && isASCIIPunct(s[t+1]) {
			t++
			continue
		}
		if c == '\n' || c == '\r' || (s[k] == '(' && c == '(') {
			f.steps += t - k
			return p, false
		}
		if c == closer {
			break
		}
	}
	f.steps += t - k
	p.titleStart, p.titleEnd = k+1-v, t-v
	m := skipSpaceTab(s, t+1)
	f.steps += m - t
	if m >= len(s) || s[m] != ')' {
		return p, false
	}
	p.consumed = m + 1 - v
	return p, true
}

// bareRegion es un tramo [start, stop) de la vista que recorrió un destino
// suelto sin cerrar: termina en stop, que es un espacio o tab (ws), un
// carácter de control o el final del texto, sin ningún espacio ni control
// antes. depth[k] es la profundidad de paréntesis (sin contar los escapados)
// antes del byte start+k, y closeAt[k] el ")" que balancea desde start+k
// dentro del tramo, o -1.
//
// Una apertura posterior cuyo destino empieza adentro del tramo se detiene
// en el mismo stop o antes, en su closeAt. Los escapes de barra invertida se
// emparejan igual desde las dos: un destino empieza después de "(" o de un
// espacio, que no pueden ser el segundo byte de un escape, así que la
// lectura desde cualquiera de las dos empareja las mismas barras.
type bareRegion struct {
	start, stop int
	ws          bool
	depth       []int
	closeAt     []int
}

// bareEnd lee un destino suelto desde i (el bucle de scanCommonMarkLinkParts)
// y devuelve dónde termina: en un espacio o tab con los paréntesis
// balanceados, o en el ")" que no cierra un "(" abierto adentro.
func (f *linkFinder) bareEnd(i int) (int, bool) {
	r := &f.bare
	if r.depth != nil && i >= r.start && i < r.stop {
		k := i - r.start
		if c := r.closeAt[k]; c >= 0 {
			return c, true
		}
		if r.ws && r.depth[r.stop-r.start] == r.depth[k] {
			return r.stop, true
		}
		return 0, false
	}
	s := f.view
	depth := 0
	j := i
	ws := false
loop:
	for ; j < len(s); j++ {
		c := s[j]
		switch {
		case c == '\\' && j+1 < len(s) && isASCIIPunct(s[j+1]):
			j++
		case c == ' ' || c == '\t':
			ws = true
			break loop
		case c < 0x20 || c == 0x7f:
			break loop
		case c == '(':
			depth++
		case c == ')':
			if depth == 0 {
				f.steps += j - i + 1
				return j, true
			}
			depth--
		}
	}
	if j > len(s) {
		j = len(s)
	}
	f.steps += j - i
	if ws && depth == 0 {
		return j, true
	}
	f.recordBare(i, j, ws)
	return 0, false
}

// recordBare arma la bareRegion [i, stop).
func (f *linkFinder) recordBare(i, stop int, ws bool) {
	s := f.view
	r := &f.bare
	n := stop - i
	r.start, r.stop, r.ws = i, stop, ws
	r.depth = resized(r.depth, n+1)
	r.closeAt = resized(r.closeAt, n+1)
	d := 0
	for k := 0; k < n; k++ {
		r.depth[k] = d
		c := s[i+k]
		if c == '\\' && i+k+1 < len(s) && isASCIIPunct(s[i+k+1]) {
			// El byte escapado no cuenta, y no empieza ningún destino.
			r.depth[k+1] = d
			k++
			continue
		}
		switch c {
		case '(':
			d++
		case ')':
			d--
		}
	}
	r.depth[n] = d
	// closeAt[k]: el primer m > k con depth[m] < depth[k], menos uno, es
	// el ")" que balancea desde k. Pila monótona, como balancedCloses.
	stack := make([]int, 0, 16)
	for k := n; k >= 0; k-- {
		for len(stack) > 0 && r.depth[stack[len(stack)-1]] >= r.depth[k] {
			stack = stack[:len(stack)-1]
		}
		r.closeAt[k] = -1
		if len(stack) > 0 {
			r.closeAt[k] = i + stack[len(stack)-1] - 1
		}
		stack = append(stack, k)
	}
	f.steps += 2 * n
}

// looseRegion es un tramo [start, stop) de la vista que recorrió la lectura
// de compatibilidad sin encontrar el ")" que balancea antes de un salto de
// línea o del final: closes es balancedCloses del tramo (relativo a start).
type looseRegion struct {
	start, stop int
	closes      []int
}

// looseClose es balancedClose(f.view[v:]): el ")" que balancea desde v,
// relativo a v, o -1.
func (f *linkFinder) looseClose(v int) int {
	r := &f.loose
	if r.closes != nil && v >= r.start && v < r.stop {
		k := v - r.start
		if r.closes[k] < 0 {
			return -1
		}
		return r.closes[k] - k
	}
	s := f.view
	depth := 0
	j := v
loop:
	for ; j < len(s); j++ {
		switch s[j] {
		case '\n', '\r':
			break loop
		case '(':
			depth++
		case ')':
			if depth == 0 {
				f.steps += j - v + 1
				return j - v
			}
			depth--
		}
	}
	f.steps += 2 * (j - v)
	r.start, r.stop = v, j
	r.closes = balancedCloses(s[v:j])
	if r.closes == nil {
		r.closes = []int{}
	}
	return -1
}

// resized devuelve s con largo n y en cero, reusando su memoria si alcanza.
func resized[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	s = s[:n]
	clear(s)
	return s
}

// balancedCloses calcula balancedClose para cada posición de s a la vez:
// con P la suma de prefijos ("(" +1, ")" -1), el cierre desde q es el primer
// j > q de su línea con P[j] < P[q], menos uno. Una pila monótona lo da para
// todos los q en un solo recorrido por línea.
func balancedCloses(s string) []int {
	res := make([]int, len(s))
	for i := range res {
		res[i] = -1
	}
	var prefix, stack []int
	for a := 0; a <= len(s); {
		b := a
		for b < len(s) && s[b] != '\n' && s[b] != '\r' {
			b++
		}
		prefix = append(prefix[:0], 0)
		for i := a; i < b; i++ {
			d := 0
			switch s[i] {
			case '(':
				d = 1
			case ')':
				d = -1
			}
			prefix = append(prefix, prefix[len(prefix)-1]+d)
		}
		stack = stack[:0]
		for q := b - a; q >= 0; q-- {
			for len(stack) > 0 && prefix[stack[len(stack)-1]] >= prefix[q] {
				stack = stack[:len(stack)-1]
			}
			if q < b-a && len(stack) > 0 {
				res[a+q] = a + stack[len(stack)-1] - 1
			}
			stack = append(stack, q)
		}
		a = b + 1
	}
	return res
}

// find busca, desde el byte from de text, el primer enlace con destino
// válido y devuelve sus índices absolutos con la forma de
// regexp.FindStringSubmatchIndex: [inicio, fin, inicio etiqueta, fin
// etiqueta, inicio destino, fin destino, inicio título, fin título]. El
// destino va sin <...> ni título; sin título, sus índices son -1. spaced
// marca la lectura de compatibilidad. Una apertura sin destino válido se
// salta y la búsqueda sigue desde el byte siguiente.
func (f *linkFinder) find(from int) (loc []int, spaced bool) {
	search := from
	for search < len(f.text) {
		start, labelStart, labelEnd, openEnd, ok := f.nextOpen(search)
		if !ok {
			return nil, false
		}
		v := openEnd
		if f.escaped {
			v = f.toView[openEnd]
		}
		if v >= 0 {
			parts, ok := f.scanParts(v)
			if ok {
				abs := func(i int) int {
					if f.escaped {
						return f.offsets[v+i]
					}
					return v + i
				}
				out := []int{start, abs(parts.consumed), labelStart, labelEnd, abs(parts.destStart), abs(parts.destEnd), -1, -1}
				if parts.titleStart >= 0 {
					out[6], out[7] = abs(parts.titleStart), abs(parts.titleEnd)
				}
				return out, parts.spaced
			}
		}
		search = start + 1
	}
	return nil, false
}

// attrsScanner dice si en un byte de text empieza un bloque de atributos
// {clave=valor ...} completo, el de inlineAttrsBody: lo mismo que
// regexp.MustCompile(`^` + inlineAttrsBody).MatchString(text[p:]) (con
// quote "&quot;", texto escapado) o su variante con comillas crudas (quote
// `"`), en tiempo lineal sobre todas las consultas de un texto.
//
// Con el regex, cada imagen seguida de "{" recorría lo que quedaba de la
// línea (el valor sin comillas, [^\s}]+, se come todo lo que no sea espacio)
// y "![a](b){a=" repetido era cuadrático. Ningún tramo del bloque lleva "}"
// salvo el cierre, así que un bloque que empieza en p termina en el primer
// "}" después de p. Para todas las consultas que comparten ese "}" se
// resuelve una sola vez, de derecha a izquierda, qué sufijos del tramo son
// una lista de atributos válida (ver fill).
type attrsScanner struct {
	text  string
	quote string

	braceFrom, braceAt int
	braceOK            bool

	// Tramo resuelto: [regStart, regEnd), con regEnd el "}". ok[k]: el
	// sufijo text[regStart+k:regEnd] es (\s*clave=valor)*\s*. nonSpace[k]:
	// el primer byte que no es espacio desde regStart+k (relativo).
	regStart, regEnd int
	ok               []bool
	nonSpace         []int
	scratch          []int
	steps            int
}

// blockEnd devuelve dónde termina (después del "}") el bloque de atributos
// que empieza en p, si lo hay.
func (a *attrsScanner) blockEnd(p int) (int, bool) {
	t := a.text
	if p >= len(t) || t[p] != '{' {
		return 0, false
	}
	x := p + 1
	if !a.braceOK || x < a.braceFrom || x > a.braceAt {
		k := strings.IndexByte(t[x:], '}')
		if k < 0 {
			a.braceAt = len(t)
			a.steps += len(t) - x
		} else {
			a.braceAt = x + k
			a.steps += k + 1
		}
		a.braceFrom, a.braceOK = x, true
	}
	q := a.braceAt
	if q >= len(t) {
		return 0, false
	}
	if a.ok == nil || a.regEnd != q || x < a.regStart {
		a.fill(x, q)
	}
	k := x - a.regStart
	if !a.ok[k] || a.regStart+a.nonSpace[k] >= q {
		// Sin ningún atributo ("{ }") tampoco es un bloque: el regex pide
		// al menos uno.
		return 0, false
	}
	return q + 1, true
}

// fill resuelve el tramo [start, end) de derecha a izquierda. Un sufijo que
// empieza en un espacio vale lo que vale el sufijo siguiente; uno que empieza
// en una letra tiene que empezar con un atributo clave=valor seguido de un
// sufijo válido. La clave ([A-Za-z][\w-]*) termina en el primer byte que no
// es [\w-], que tiene que ser "="; el valor es el tramo entre comillas (quote,
// sin el primer byte de quote adentro) o cualquier corrida no vacía de bytes
// que no son espacio, así que basta saber si alguno de los sufijos que
// empiezan dentro de esa corrida, o justo al final, es válido (nextOK).
func (a *attrsScanner) fill(start, end int) {
	t := a.text
	n := end - start
	a.regStart, a.regEnd = start, end
	a.ok = resized(a.ok, n+1)
	a.nonSpace = resized(a.nonSpace, n+1)
	// scratch guarda, por tramos de n+1: nextOK, nextSpace, keyEnd,
	// nextQuote.
	a.scratch = resized(a.scratch, 4*(n+1))
	nextOK := a.scratch[0 : n+1]
	nextSpace := a.scratch[n+1 : 2*(n+1)]
	keyEnd := a.scratch[2*(n+1) : 3*(n+1)]
	nextQuote := a.scratch[3*(n+1):]
	q0 := a.quote[0]
	a.ok[n] = true
	nextOK[n], nextSpace[n], keyEnd[n], nextQuote[n], a.nonSpace[n] = n, n, n, n, n
	for k := n - 1; k >= 0; k-- {
		c := t[start+k]
		space := c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'
		if space {
			nextSpace[k] = k
			a.nonSpace[k] = a.nonSpace[k+1]
		} else {
			nextSpace[k] = nextSpace[k+1]
			a.nonSpace[k] = k
		}
		if c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' || c == '-' {
			keyEnd[k] = keyEnd[k+1]
		} else {
			keyEnd[k] = k
		}
		if c == q0 {
			nextQuote[k] = k
		} else {
			nextQuote[k] = nextQuote[k+1]
		}
		switch {
		case space:
			a.ok[k] = a.ok[k+1]
		case c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z':
			ke := keyEnd[k+1]
			if ke >= n || t[start+ke] != '=' {
				break
			}
			v := ke + 1
			if v+len(a.quote) <= n && t[start+v:start+v+len(a.quote)] == a.quote {
				x := nextQuote[v+len(a.quote)]
				if x+len(a.quote) <= n && t[start+x:start+x+len(a.quote)] == a.quote && a.ok[x+len(a.quote)] {
					a.ok[k] = true
					break
				}
			}
			if v < n && nextSpace[v] != v && nextOK[v+1] <= nextSpace[v] {
				a.ok[k] = true
			}
		}
		if a.ok[k] {
			nextOK[k] = k
		} else {
			nextOK[k] = nextOK[k+1]
		}
	}
	a.steps += 2 * n
}

// FindInlineLinkIndex devuelve los índices del primer enlace [texto](destino)
// de s, Markdown crudo, en la forma de regexp.FindStringSubmatchIndex: grupo
// 1 la etiqueta, grupo 2 el destino (sin <...> ni título) y grupo 3 el
// título (-1, -1 si no lleva), o nil si no hay ninguno. Exportado para los
// generadores que recorren el Markdown inline con sus propios patrones, de
// modo que reconozcan los mismos enlaces que el renderer HTML sin mantener
// una copia del regex.
//
// Cada llamada empieza de cero: una apertura sin cierre se lee hasta el final
// del texto. Llamarla en un bucle sobre lo que queda de una línea (un enlace
// por vuelta) es cuadrático en el peor caso; quien recorre todos los enlaces
// de un texto tiene que usar un solo buscador para todo el recorrido, como
// replaceInlineLinks (o, en doclang y slidelang, el linkScanner de
// link_destinations.go).
func FindInlineLinkIndex(s string) []int {
	loc, _ := newLinkFinder(s, false, false).find(0)
	return loc
}

// inlineLinkMatch es un enlace o imagen que encontró replaceInlineLinks, con
// el destino y el título en la forma escapada del texto (título "" sin
// título).
type inlineLinkMatch struct {
	match, label, dest, title string
	spaced                    bool
	// attrs: un bloque {clave=valor} pegado al ")" que la pasada de
	// imágenes descarta (ver inlineImageAttrsPattern).
	attrs bool
}

// replaceInlineLinks reemplaza cada ocurrencia de open + destino válido en
// text (ya pasado por EscapeHTML) por lo que devuelva replace.
func replaceInlineLinks(text string, image bool, replace func(m inlineLinkMatch) string) string {
	if !strings.Contains(text, "](") || (image && !strings.Contains(text, "![")) {
		return text
	}
	f := newLinkFinder(text, image, true)
	var b strings.Builder
	copied := 0
	for copied < len(text) {
		loc, spaced := f.find(copied)
		if loc == nil {
			break
		}
		m := inlineLinkMatch{match: text[loc[0]:loc[1]], label: text[loc[2]:loc[3]], dest: text[loc[4]:loc[5]], spaced: spaced}
		if image {
			_, m.attrs = f.attrs.blockEnd(loc[1])
		}
		if loc[6] >= 0 {
			m.title = text[loc[6]:loc[7]]
		}
		b.WriteString(text[copied:loc[0]])
		b.WriteString(replace(m))
		copied = loc[1]
	}
	if copied == 0 {
		return text
	}
	b.WriteString(text[copied:])
	return b.String()
}

// unescapeLinkDestination deshace el EscapeHTML de entrada sobre un destino
// tal como lo ve la pasada de enlace, devolviendo lo que escribió el autor.
// El orden (&amp; al final) es el inverso exacto de EscapeHTML; no se usa
// UnescapeHTML porque esa decodifica &amp; primero y convertiría un
// "&amp;lt;" del autor en "<".
func unescapeLinkDestination(escaped string) string {
	raw := strings.ReplaceAll(escaped, "&lt;", "<")
	raw = strings.ReplaceAll(raw, "&gt;", ">")
	raw = strings.ReplaceAll(raw, "&quot;", "\"")
	raw = strings.ReplaceAll(raw, "&#39;", "'")
	return strings.ReplaceAll(raw, "&amp;", "&")
}

// decodeLinkDestination lleva el destino tal como lo ve la pasada de enlace
// (texto ya escapado con EscapeHTML) a la URL que se valida y se emite.
func decodeLinkDestination(escaped string) string {
	return DecodeLinkDestination(unescapeLinkDestination(escaped))
}

// charRefPattern reconoce una referencia de carácter completa, terminada en
// ";" como exige CommonMark.
var charRefPattern = regexp.MustCompile(`&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{0,31});`)

// DecodeLinkDestination decodifica las referencias de carácter (&#106;,
// &colon;, &amp;...) de un destino de enlace o imagen inline, como hace
// CommonMark con los destinos. Importa para el filtro de esquemas: sin
// decodificar, `&#106;avascript:` pasaba como una URL relativa, y cualquier
// renderer CommonMark que reciba esa misma fuente la lee como javascript:.
//
// Solo se decodifican referencias completas terminadas en ";".
// html.UnescapeString no sirve tal cual: fuera de un atributo también
// decodifica entidades heredadas sin ";" y por prefijo, así que el query
// string "?id=1&section=intro" salía como "?id=1§ion=intro" (y "&param=",
// "&region=", "&copy=" igual). Por eso se aplica solo a lo que casa con
// charRefPattern, y se descarta su resultado cuando no es la referencia
// entera (p. ej. "&notit;", que por prefijo daría "¬it;").
//
// Es el paso que el renderer aplica antes de validar el esquema, expuesto
// para que el linter valide exactamente la misma URL.
func DecodeLinkDestination(raw string) string {
	if !strings.Contains(raw, "&") {
		return raw
	}
	return charRefPattern.ReplaceAllStringFunc(raw, func(ref string) string {
		decoded := html.UnescapeString(ref)
		if decoded == ref || utf8.RuneCountInString(decoded) > 2 {
			return ref
		}
		return decoded
	})
}

// URLProblem clasifica por qué ValidateURLScheme rechaza una URL.
type URLProblem int

const (
	// URLAllowed: la URL pasa el filtro y se emite como href/src.
	URLAllowed URLProblem = iota
	// URLSchemeNotAllowed: la URL se entiende, pero su esquema no está en
	// la allowlist (javascript:, data:, vbscript:, file:, o cualquier
	// esquema desconocido).
	URLSchemeNotAllowed
	// URLInvalid: la URL no se puede interpretar (net/url la rechaza).
	URLInvalid
)

// ClassifyURL aplica el mismo filtro que ValidateURLScheme y dice por qué
// rechaza, junto con el esquema en minúsculas cuando lo hay. ValidateURLScheme
// se implementa encima de esta función, así que las dos no pueden divergir.
func ClassifyURL(rawURL string) (problem URLProblem, scheme string) {
	rawURL = strings.TrimSpace(rawURL)
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return URLInvalid, ""
	}
	scheme = strings.ToLower(parsedURL.Scheme)
	switch scheme {
	case "http", "https", "mailto", "tel", "ftp", "":
		return URLAllowed, scheme
	default:
		return URLSchemeNotAllowed, scheme
	}
}

// InlineLink es un enlace [texto](destino) o una imagen ![alt](destino)
// tal como lo procesó el renderer.
type InlineLink struct {
	Image bool
	// Text es la etiqueta del enlace ya procesada (escapada y con los
	// formatos inline aplicados) o, en una imagen, el alt en texto plano
	// escapado, el mismo que va al atributo.
	Text string
	// Destination es el destino tal como lo escribió el autor, sin los
	// paréntesis externos, sin <...> ni título, y sin decodificar referencias
	// de carácter. Un code
	// span dentro del destino aparece como <code>…</code>, que es lo que
	// el renderer valida.
	Destination string
	// Dropped es true cuando el renderer descartó el destino (esquema no
	// permitido o URL inválida) y emitió solo el texto o el alt.
	Dropped bool
	// DroppedAttributes es true cuando una imagen inline lleva pegado un
	// bloque {clave=valor} (p. ej. {width=30%}), que no tiene representación
	// inline y el renderer descarta.
	DroppedAttributes bool
	// Spaced es true cuando el destino es una ruta suelta con espacios, leída
	// con la regla de compatibilidad (ver ScanLinkDestination): funciona,
	// pero conviene escribirla entre <...> o con %20.
	Spaced bool
}

// FindInlineLinks devuelve los enlaces e imágenes inline que el renderer
// procesa en text, línea por línea, con la decisión que tomó sobre cada
// destino. No es una reimplementación: corre las mismas pasadas de
// ProcessInlineMarkdownFormatsSecure (code spans, spans de clase e idioma,
// imágenes antes que enlaces) y registra lo que hacen, así que una regla de
// lint que diga "este enlace no se va a emitir" no puede contradecir al HTML.
// Un enlace que el renderer deja literal (sin destino balanceado, o con la
// etiqueta cruzando un tag) no aparece.
func FindInlineLinks(text string) []InlineLink {
	var out []InlineLink
	record := func(l InlineLink) { out = append(out, l) }
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "](") {
			processInlineFormats(EscapeHTML(line), record)
		}
	}
	return out
}

// EscapeURLSpaces codifica los espacios y tabuladores internos de una URL
// (%20, %09) para que el src o href emitido sea una URL válida. Un destino
// <mi carpeta/a.png> llega al AST con el espacio literal; los %XX que ya
// traía no se tocan, así que no hay doble codificación.
func EscapeURLSpaces(u string) string {
	u = strings.TrimSpace(u)
	if !strings.ContainsAny(u, " \t") {
		return u
	}
	return strings.NewReplacer(" ", "%20", "\t", "%09").Replace(u)
}

// LocalPathFromURL devuelve la ruta de archivo de un destino local (sin
// esquema): los destinos son URLs, así que "mi%20carpeta/a.png" es el
// archivo "mi carpeta/a.png". Si la ruta no se puede decodificar (un "%" suelto, como en "100%.png") se
// devuelve tal cual.
func LocalPathFromURL(src string) string {
	if !strings.Contains(src, "%") {
		return src
	}
	if decoded, err := url.PathUnescape(src); err == nil {
		return decoded
	}
	return src
}
