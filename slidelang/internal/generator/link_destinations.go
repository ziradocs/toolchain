// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import "strings"

// Este archivo tiene su gemelo en doclang/internal/generator: los dos CLIs
// solo comparten core, y esto tiene que compilar contra el core publicado.
// El buscador (linkScanner) y los atributos (attrsScanner) son copias de los
// de core/renderer, que no están exportados.

// maskLinkDestinations devuelve s con lo que va entre el "(" y el ")" de
// cada imagen y enlace inline (destino y título) cambiado por relleno del
// mismo largo, para buscar ahí la negrita y la cursiva: en el HTML el destino
// y el título son literales, como en CommonMark. Los índices sirven tal cual
// sobre s.
//
// Como en el HTML, un code span (los que devuelve codeSpans, los mismos que
// reconoce el generador) se resuelve antes: no abre enlaces ni imágenes, así que
// en "`f[0](` y *x* texto)" la cursiva sigue siendo cursiva. Las imágenes se
// buscan primero y se tapan enteras para buscar los enlaces: en
// [![a](i.png)](url) el destino del enlace es url.
//
// Lineal: cada pasada usa un solo linkScanner.
func maskLinkDestinations(s string, codeSpans func(string) [][]int) string {
	if !strings.Contains(s, "](") {
		return s
	}
	masked := []byte(s)
	scan := []byte(s)
	fill := func(b []byte, from, to int) {
		for i := from; i < to; i++ {
			b[i] = 'x'
		}
	}
	if codeSpans != nil && strings.Contains(s, "`") {
		for _, loc := range codeSpans(s) {
			fill(scan, loc[0], loc[1])
		}
	}
	find := func(text string, image bool, found func(start, openEnd, end int)) {
		f := newLinkScanner(text, image)
		for from := 0; from < len(text); {
			loc, _ := f.find(from)
			if loc == nil {
				return
			}
			// loc[3] es el fin de la etiqueta: ahí van "](".
			found(loc[0], loc[3]+2, loc[1])
			from = loc[1]
		}
	}
	if strings.Contains(s, "![") {
		find(string(scan), true, func(start, openEnd, end int) {
			fill(masked, openEnd, end-1)
			fill(scan, start, end)
		})
	}
	find(string(scan), false, func(_, openEnd, end int) {
		fill(masked, openEnd, end-1)
	})
	return string(masked)
}

// linkParts son los índices, relativos al texto que sigue al "(", del
// destino y del título de un enlace; titleStart es -1 cuando no hay título.
// spaced marca la lectura de compatibilidad (una ruta suelta con espacios).
type linkParts struct {
	destStart, destEnd   int
	titleStart, titleEnd int
	consumed             int
	spaced               bool
}

// linkScanner busca enlaces (o imágenes, con image) en un texto Markdown
// crudo, en tiempo lineal en el largo del texto. Es el linkFinder de
// core/renderer (no exportado) sin la vista desescapada, que aquí no hace
// falta porque el texto llega sin escapar: lo mismo que el regex de apertura
// desde cada candidato seguido de renderer.ScanLinkDestination sobre el
// resto, con el mismo resultado que renderer.FindInlineLinkIndex (ver
// TestLinkScannerMatchesCore), pero sin recorrer nada dos veces:
//
//   - La apertura se busca a mano y el "]" que cierra la etiqueta se
//     recuerda: con el regex, "[[[…](b" recorría la etiqueta desde cada "[".
//   - Un destino suelto que no cierra deja guardada la región que recorrió
//     (bareRegion), y las aperturas siguientes que caen adentro se resuelven
//     con esa tabla. La lectura de compatibilidad (ruta suelta con espacios)
//     guarda igual la suya (looseRegion).
//
// Todo se calcula a medida que hace falta, y las memorias suponen que las
// búsquedas avanzan; una búsqueda hacia atrás da el mismo resultado sin
// aprovecharlas.
type linkScanner struct {
	text  string
	image bool
	view  string // el mismo text; el nombre es el de core

	// rbAt es el primer "]" de text desde rbFrom (len(text) si no hay).
	rbFrom, rbAt int
	rbOK         bool

	bare  bareRegion
	loose looseRegion

	// steps cuenta los bytes que recorren los escaneos (solo para las
	// pruebas de escala).
	steps int
}

func newLinkScanner(text string, image bool) *linkScanner {
	return &linkScanner{text: text, image: image, view: text}
}

// nextOpen busca desde search la primera apertura "[etiqueta](" (o
// "![alt](" con image): lo mismo que FindStringSubmatchIndex de
// inlineLinkPattern o inlineImagePattern sobre text[search:], en índices
// absolutos. La etiqueta de un enlace no puede ser vacía; la de una imagen
// sí. Ninguna de las dos lleva "]".
func (f *linkScanner) nextOpen(search int) (start, labelStart, labelEnd, openEnd int, ok bool) {
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

// openAt dice si en i empieza una apertura "![alt](" (o "[etiqueta](" sin
// image) y devuelve dónde termina, después del "(": el regex de apertura
// anclado en text[i:].
func (f *linkScanner) openAt(i int) (openEnd int, ok bool) {
	t := f.text
	labelStart := i + 1
	if f.image {
		if i+1 >= len(t) || t[i] != '!' || t[i+1] != '[' {
			return 0, false
		}
		labelStart = i + 2
	} else if i >= len(t) || t[i] != '[' {
		return 0, false
	}
	e := f.nextRB(labelStart)
	if e >= len(t) || !f.image && e == labelStart || e+1 >= len(t) || t[e+1] != '(' {
		return 0, false
	}
	return e + 2, true
}

// destEnd es renderer.ScanLinkDestination(text[v:]) para un v justo después
// del "(" de una apertura: cuántos bytes consume el destino, ")" incluido.
// El destino crudo es text[v : v+n-1].
func (f *linkScanner) destEnd(v int) (n int, ok bool) {
	p, ok := f.scanParts(v)
	if !ok {
		return 0, false
	}
	return p.consumed, true
}

// nextRB devuelve el índice del primer "]" de text desde x, o len(text).
func (f *linkScanner) nextRB(x int) int {
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

// scanParts lee el destino que empieza en v (justo después del "(") con la
// gramática de renderer.ScanLinkDestination, con las memorias del buscador.
// Los índices de linkParts son relativos a v.
func (f *linkScanner) scanParts(v int) (linkParts, bool) {
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

// strictParts es la gramática estricta de CommonMark de core
// (scanCommonMarkLinkParts) desde v. El destino suelto lo
// lee bareEnd; los demás tramos (<...>, el título, los espacios) no hace
// falta guardarlos: cada uno termina, a lo sumo, donde empieza el mismo tramo
// de la apertura siguiente que llega hasta ahí, así que entre todas las
// aperturas de una línea se recorren una vez.
func (f *linkScanner) strictParts(v int) (linkParts, bool) {
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

// bareEnd lee un destino suelto desde i (el bucle de la gramática estricta)
// y devuelve dónde termina: en un espacio o tab con los paréntesis
// balanceados, o en el ")" que no cierra un "(" abierto adentro.
func (f *linkScanner) bareEnd(i int) (int, bool) {
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
func (f *linkScanner) recordBare(i, stop int, ws bool) {
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

// looseClose es el ")" que balancea desde v en la misma línea (la lectura de
// compatibilidad de core), relativo a v, o -1.
func (f *linkScanner) looseClose(v int) int {
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
// válido (lo que renderer.FindInlineLinkIndex devuelve sobre text[from:]) y
// devuelve sus índices absolutos con la forma de
// regexp.FindStringSubmatchIndex: [inicio, fin, inicio etiqueta, fin
// etiqueta, inicio destino, fin destino, inicio título, fin título]. El
// destino va sin <...> ni título; sin título, sus índices son -1. spaced
// marca la lectura de compatibilidad. Una apertura sin destino válido se
// salta y la búsqueda sigue desde el byte siguiente.
func (f *linkScanner) find(from int) (loc []int, spaced bool) {
	search := from
	for search < len(f.text) {
		start, labelStart, labelEnd, openEnd, ok := f.nextOpen(search)
		if !ok {
			return nil, false
		}
		v := openEnd
		if parts, ok := f.scanParts(v); ok {
			out := []int{start, v + parts.consumed, labelStart, labelEnd, v + parts.destStart, v + parts.destEnd, -1, -1}
			if parts.titleStart >= 0 {
				out[6], out[7] = v+parts.titleStart, v+parts.titleEnd
			}
			return out, parts.spaced
		}
		search = start + 1
	}
	return nil, false
}

// attrsScanner dice si en un byte de text empieza un bloque de atributos
// {clave=valor ...} completo: lo mismo que el regex anclado
// `^\{(?:\s*[A-Za-z][\w-]*=(?:"[^"}]*"|[^\s}]+))+\s*\}` sobre text[p:] (con
// quote `"`; con "&quot;" es la variante de core para el texto escapado),
// en tiempo lineal sobre todas las consultas de un texto.
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

func skipSpaceTab(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

func isASCIIPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}
