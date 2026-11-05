// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "regexp"

// Este archivo guarda, como oráculo de las pruebas, el buscador de enlaces
// tal como estaba en v2.49.2 (antes de hacerlo lineal): las aperturas con el
// regex desde cada candidato y el destino con scanLinkParts sobre el resto
// de la vista. Es cuadrático con aperturas que no cierran, pero su salida es
// la referencia: el buscador nuevo tiene que devolver exactamente lo mismo.

// oracleLinkFinder busca enlaces (o imágenes, según open) en un texto completo.
// La vista desescapada y sus offsets se arman una sola vez por texto, y los
// cierres de paréntesis de la lectura de compatibilidad se precalculan (en
// O(n)) la primera vez que hacen falta: así recorrer todos los enlaces de
// una línea es lineal, no cuadrático.
type oracleLinkFinder struct {
	text    string
	open    *regexp.Regexp
	escaped bool
	view    string
	offsets []int // vista -> texto (solo escaped)
	toView  []int // texto -> vista, -1 dentro de una entidad (solo escaped)
	closes  []int // por índice de la vista: el ")" que balancea desde ahí, o -1
}

func newOracleLinkFinder(text string, open *regexp.Regexp, escaped bool) *oracleLinkFinder {
	f := &oracleLinkFinder{text: text, open: open, escaped: escaped, view: text}
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
func (f *oracleLinkFinder) closeFrom(v int) int {
	if f.closes == nil {
		f.closes = balancedCloses(f.view)
	}
	if v >= len(f.closes) || f.closes[v] < 0 {
		return -1
	}
	return f.closes[v] - v
}

// find busca, desde el byte from de text, el primer enlace con destino
// válido y devuelve sus índices absolutos con la forma de
// regexp.FindStringSubmatchIndex: [inicio, fin, inicio etiqueta, fin
// etiqueta, inicio destino, fin destino, inicio título, fin título]. El
// destino va sin <...> ni título; sin título, sus índices son -1. spaced
// marca la lectura de compatibilidad. Una apertura sin destino válido se
// salta y la búsqueda sigue desde el byte siguiente.
func (f *oracleLinkFinder) find(from int) (loc []int, spaced bool) {
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
