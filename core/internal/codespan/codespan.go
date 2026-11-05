// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package codespan empareja los delimitadores de los code spans de
// CommonMark en tiempo casi lineal. Lo comparten renderer.findCodeSpans,
// elements.codeSpanRanges y formatter.codeSpanRunRanges, que antes tenían
// cada uno una copia del mismo bucle: por cada corrida de backticks que no
// cerraba se recorría el resto del texto buscando otra del mismo largo, y
// una línea con muchas aperturas sin cierre (una barra invertida seguida de
// dos backticks, repetido) era cuadrática.
package codespan

// Span es un code span: la corrida de apertura [OpenStart, OpenEnd) y la de
// cierre [CloseStart, CloseEnd). El contenido es [OpenEnd, CloseStart).
type Span struct {
	OpenStart, OpenEnd   int
	CloseStart, CloseEnd int
}

// Find devuelve los code spans de s de izquierda a derecha: el delimitador es
// una corrida de N backticks y solo otra corrida de exactamente N la cierra;
// una corrida sin cierre de su largo son backticks literales y la búsqueda
// sigue después de ella. escaped(i) dice si el backtick en i está escapado
// (precedido por una corrida impar de barras invertidas): un backtick
// escapado no abre, y la apertura empieza en el siguiente de la corrida. Los
// cierres no miran escapes, como en CommonMark.
//
// Es el mismo resultado que el bucle anterior, que buscaba el cierre
// recorriendo el texto; aquí cada corrida se indexa por largo y el cierre se
// busca con una búsqueda binaria.
func Find[T byte | rune](s []T, escaped func(i int) bool) []Span {
	var starts, ends []int
	for i := 0; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == '`' {
			j++
		}
		starts, ends = append(starts, i), append(ends, j)
		i = j
	}
	if len(starts) < 2 {
		return nil
	}
	byLen := map[int][]int{}
	for k := range starts {
		l := ends[k] - starts[k]
		byLen[l] = append(byLen[l], k)
	}
	var spans []Span
	for k := 0; k < len(starts); k++ {
		p := starts[k]
		if escaped(p) {
			// Solo el primero de la corrida puede estar escapado: los
			// demás los precede un backtick.
			p++
		}
		if p >= ends[k] {
			continue
		}
		q, ok := next(byLen[ends[k]-p], k)
		if !ok {
			continue
		}
		spans = append(spans, Span{OpenStart: p, OpenEnd: ends[k], CloseStart: starts[q], CloseEnd: ends[q]})
		k = q
	}
	return spans
}

// next devuelve el primer índice de corrida mayor que k en idx (ordenado).
func next(idx []int, k int) (int, bool) {
	lo, hi := 0, len(idx)
	for lo < hi {
		m := (lo + hi) / 2
		if idx[m] <= k {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo == len(idx) {
		return 0, false
	}
	return idx[lo], true
}
