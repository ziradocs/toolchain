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
// empieza justo DESPUÉS del "(" de apertura; el destino termina en el
// primer ")" que no cierra un "(" abierto dentro del propio destino, igual
// que en CommonMark. Así https://es.wikipedia.org/wiki/Foo_(bar) y
// javascript:alert(1) se leen completos, en vez de cortarse en el primer ")"
// (que dejaba un ")" suelto en el texto y, en el primer caso, un href
// truncado).
//
// Devuelve el destino sin los paréntesis externos y cuántos bytes de s
// consumió, incluido el ")" de cierre. ok es false cuando no hay tal cierre
// en la misma línea (paréntesis desbalanceados o un salto de línea antes) o
// cuando el destino queda vacío; en ambos casos la sintaxis no es un enlace
// y el texto se deja literal, como en CommonMark.
//
// Exportado para que los generadores que leen el Markdown inline por su
// cuenta (el DOCX de doclang) reconozcan los mismos destinos que el
// renderer HTML en vez de mantener su propia copia del patrón.
func ScanLinkDestination(s string) (dest string, consumed int, ok bool) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n', '\r':
			return "", 0, false
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
				continue
			}
			if i == 0 {
				return "", 0, false
			}
			return s[:i], i + 1, true
		}
	}
	return "", 0, false
}

// findInlineLink busca en text el primer enlace (o imagen, según open) con
// destino balanceado y devuelve sus índices con la misma forma que
// regexp.FindStringSubmatchIndex: [inicio, fin, inicio etiqueta, fin
// etiqueta, inicio destino, fin destino]. Una apertura sin destino válido se
// salta y la búsqueda sigue desde el byte siguiente, así que un enlace bien
// formado más adelante en la misma línea se sigue reconociendo.
func findInlineLink(text string, open *regexp.Regexp) []int {
	search := 0
	for search < len(text) {
		loc := open.FindStringSubmatchIndex(text[search:])
		if loc == nil {
			return nil
		}
		start := search + loc[0]
		openEnd := search + loc[1]
		if dest, n, ok := ScanLinkDestination(text[openEnd:]); ok {
			return []int{start, openEnd + n, search + loc[2], search + loc[3], openEnd, openEnd + len(dest)}
		}
		search = start + 1
	}
	return nil
}

// FindInlineLinkIndex devuelve los índices del primer enlace [texto](destino)
// de s, con destino balanceado, en la forma de regexp.FindStringSubmatchIndex
// (grupo 1 la etiqueta, grupo 2 el destino), o nil si no hay ninguno.
// Exportado para los generadores que recorren el Markdown inline con sus
// propios patrones (el DOCX de doclang), de modo que reconozcan los mismos
// enlaces que el renderer HTML sin mantener una copia del regex.
func FindInlineLinkIndex(s string) []int {
	return findInlineLink(s, inlineLinkPattern)
}

// replaceInlineLinks reemplaza cada ocurrencia de open + destino
// balanceado en text por lo que devuelva replace(match, etiqueta, destino).
func replaceInlineLinks(text string, open *regexp.Regexp, replace func(match, label, dest string) string) string {
	var b strings.Builder
	copied := 0
	for copied < len(text) {
		loc := findInlineLink(text[copied:], open)
		if loc == nil {
			break
		}
		b.WriteString(text[copied : copied+loc[0]])
		b.WriteString(replace(text[copied+loc[0]:copied+loc[1]], text[copied+loc[2]:copied+loc[3]], text[copied+loc[4]:copied+loc[5]]))
		copied += loc[1]
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
	// Text es la etiqueta del enlace o el alt de la imagen ya procesados
	// (escapados y con los formatos inline aplicados).
	Text string
	// Destination es el destino tal como lo escribió el autor, sin los
	// paréntesis externos y sin decodificar referencias de carácter.
	Destination string
	// Dropped es true cuando el renderer descartó el destino (esquema no
	// permitido o URL inválida) y emitió solo el texto o el alt.
	Dropped bool
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
