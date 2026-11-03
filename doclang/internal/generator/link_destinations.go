// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"regexp"
	"strings"
)

// Este archivo tiene su gemelo en slidelang/internal/generator: los dos CLIs
// solo comparten core, y esto tiene que compilar contra el core publicado.

// linkImageOpen y linkLinkOpen son las aperturas de una imagen y un enlace
// inline, las mismas que inlineImagePattern e inlineLinkPattern en
// core/renderer/sanitizer.go (no exportados).
var (
	linkImageOpen = regexp.MustCompile(`!\[([^\]]*)\]\(`)
	linkLinkOpen  = regexp.MustCompile(`\[([^\]]+)\]\(`)
)

// maskLinkDestinations devuelve s con lo que va entre el "(" y el ")" de
// cada imagen y enlace inline (destino y título) cambiado por relleno del
// mismo largo, para buscar ahí la negrita y la cursiva: en el HTML el destino
// y el título son literales, como en CommonMark. Los índices sirven tal cual
// sobre s.
//
// Como en el HTML, un code span (los que code reconoce, el mismo patrón que
// usa el generador) se resuelve antes: no abre enlaces ni imágenes, así que
// en "`f[0](` y *x* texto)" la cursiva sigue siendo cursiva. Las imágenes se
// buscan primero y se tapan enteras para buscar los enlaces: en
// [![a](i.png)](url) el destino del enlace es url.
//
// Lineal: cada destino se lee con linkDestinationEnd, que usa los cierres de
// paréntesis precalculados, como el linkFinder de core.
func maskLinkDestinations(s string, code *regexp.Regexp) string {
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
	if code != nil && strings.Contains(s, "`") {
		for _, loc := range code.FindAllStringIndex(s, -1) {
			fill(scan, loc[0], loc[1])
		}
	}
	find := func(text string, open *regexp.Regexp, found func(start, openEnd, end int)) {
		closes := balancedCloses(text)
		for search := 0; search < len(text); {
			loc := open.FindStringIndex(text[search:])
			if loc == nil {
				return
			}
			start, openEnd := search+loc[0], search+loc[1]
			n, ok := linkDestinationEnd(text, openEnd, closes)
			if !ok {
				search = start + 1
				continue
			}
			found(start, openEnd, openEnd+n)
			search = openEnd + n
		}
	}
	if strings.Contains(s, "![") {
		images := string(scan)
		find(images, linkImageOpen, func(start, openEnd, end int) {
			fill(masked, openEnd, end-1)
			fill(scan, start, end)
		})
	}
	find(string(scan), linkLinkOpen, func(_, openEnd, end int) {
		fill(masked, openEnd, end-1)
	})
	return string(masked)
}

// linkDestinationEnd lee el destino de un enlace que empieza en s[at] (justo
// después del "(") con la gramática de renderer.ScanLinkDestination y
// devuelve cuántos bytes consume, ")" incluido. Es la misma lectura (la
// prueba TestLinkDestinationEndMatchesCore la compara con la de core), pero
// la de compatibilidad, la de una ruta suelta con espacios, toma el ")" que
// balancea de closes en vez de recorrer la línea: con ScanLinkDestination,
// una línea de aperturas sin cierre propio y un ")" al final se recorría
// entera por cada apertura.
func linkDestinationEnd(s string, at int, closes []int) (int, bool) {
	t := s[at:]
	if n, ok := strictLinkDestinationEnd(t); ok {
		return n, true
	}
	i := skipBlanks(t, 0)
	if i < len(t) && t[i] == '<' {
		return 0, false
	}
	if at >= len(closes) || closes[at] < 0 {
		return 0, false
	}
	end := closes[at] - at
	if end <= 0 {
		return 0, false
	}
	return end + 1, true
}

// strictLinkDestinationEnd es la gramática estricta de CommonMark de
// core/renderer (scanCommonMarkLinkParts): destino entre <...> o suelto sin
// espacios con los paréntesis balanceados, título opcional y ")".
func strictLinkDestinationEnd(s string) (int, bool) {
	i := skipBlanks(s, 0)
	var destStart, destEnd int
	if i < len(s) && s[i] == '<' {
		j := i + 1
		for ; ; j++ {
			if j >= len(s) {
				return 0, false
			}
			c := s[j]
			if c == '\\' && j+1 < len(s) && isASCIIPunct(s[j+1]) {
				j++
				continue
			}
			if c == '\n' || c == '\r' || c == '<' {
				return 0, false
			}
			if c == '>' {
				break
			}
		}
		destStart, destEnd = i+1, j
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
				return 0, false
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
			return 0, false
		}
		destStart, destEnd = i, j
		i = j
	}
	if destStart == destEnd {
		if i == 0 || i >= len(s) || s[i] != ')' {
			return 0, false
		}
		return i + 1, true
	}
	k := skipBlanks(s, i)
	if k < len(s) && s[k] == ')' {
		return k + 1, true
	}
	if k == i || k >= len(s) {
		return 0, false
	}
	closer := s[k]
	switch closer {
	case '"', '\'':
	case '(':
		closer = ')'
	default:
		return 0, false
	}
	t := k + 1
	for ; ; t++ {
		if t >= len(s) {
			return 0, false
		}
		c := s[t]
		if c == '\\' && t+1 < len(s) && isASCIIPunct(s[t+1]) {
			t++
			continue
		}
		if c == '\n' || c == '\r' || (s[k] == '(' && c == '(') {
			return 0, false
		}
		if c == closer {
			break
		}
	}
	m := skipBlanks(s, t+1)
	if m >= len(s) || s[m] != ')' {
		return 0, false
	}
	return m + 1, true
}

// balancedCloses es el de core/renderer: para cada posición q de s, el
// índice del primer ")" de su línea que no cierra un "(" abierto desde q, o
// -1. Con P la suma de prefijos ("(" +1, ")" -1), es el primer j > q de la
// línea con P[j] < P[q], menos uno; una pila monótona lo da para todos los q
// en un solo recorrido por línea.
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

func skipBlanks(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

func isASCIIPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}
