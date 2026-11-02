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

// linkFinder busca enlaces (o imágenes, según open) en un texto completo.
// La vista desescapada y sus offsets se arman una sola vez por texto, y los
// cierres de paréntesis de la lectura de compatibilidad se precalculan (en
// O(n)) la primera vez que hacen falta: así recorrer todos los enlaces de
// una línea es lineal, no cuadrático.
type linkFinder struct {
	text    string
	open    *regexp.Regexp
	escaped bool
	view    string
	offsets []int // vista -> texto (solo escaped)
	toView  []int // texto -> vista, -1 dentro de una entidad (solo escaped)
	closes  []int // por índice de la vista: el ")" que balancea desde ahí, o -1
}

func newLinkFinder(text string, open *regexp.Regexp, escaped bool) *linkFinder {
	f := &linkFinder{text: text, open: open, escaped: escaped, view: text}
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
	return f
}

// closeFrom devuelve, relativo a v, el ")" que balancea los paréntesis desde
// el índice v de la vista, o -1.
func (f *linkFinder) closeFrom(v int) int {
	if f.closes == nil {
		f.closes = balancedCloses(f.view)
	}
	if v >= len(f.closes) || f.closes[v] < 0 {
		return -1
	}
	return f.closes[v] - v
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
		m := f.open.FindStringSubmatchIndex(f.text[search:])
		if m == nil {
			return nil, false
		}
		start := search + m[0]
		openEnd := search + m[1]
		v := openEnd
		if f.escaped {
			v = f.toView[openEnd]
		}
		if v >= 0 {
			parts, ok := scanLinkParts(f.view[v:], func() int { return f.closeFrom(v) })
			if ok {
				abs := func(i int) int {
					if f.escaped {
						return f.offsets[v+i]
					}
					return v + i
				}
				out := []int{start, abs(parts.consumed), search + m[2], search + m[3], abs(parts.destStart), abs(parts.destEnd), -1, -1}
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

// FindInlineLinkIndex devuelve los índices del primer enlace [texto](destino)
// de s, Markdown crudo, en la forma de regexp.FindStringSubmatchIndex: grupo
// 1 la etiqueta, grupo 2 el destino (sin <...> ni título) y grupo 3 el
// título (-1, -1 si no lleva), o nil si no hay ninguno. Exportado para los
// generadores que recorren el Markdown inline con sus propios patrones (el
// DOCX de doclang), de modo que reconozcan los mismos enlaces que el
// renderer HTML sin mantener una copia del regex.
func FindInlineLinkIndex(s string) []int {
	loc, _ := newLinkFinder(s, inlineLinkPattern, false).find(0)
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
func replaceInlineLinks(text string, open *regexp.Regexp, replace func(m inlineLinkMatch) string) string {
	if !strings.Contains(text, "](") || (open == inlineImagePattern && !strings.Contains(text, "![")) {
		return text
	}
	f := newLinkFinder(text, open, true)
	var b strings.Builder
	copied := 0
	for copied < len(text) {
		loc, spaced := f.find(copied)
		if loc == nil {
			break
		}
		m := inlineLinkMatch{match: text[loc[0]:loc[1]], label: text[loc[2]:loc[3]], dest: text[loc[4]:loc[5]], spaced: spaced}
		m.attrs = open == inlineImagePattern && inlineAttrsPrefix.MatchString(text[loc[1]:])
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
