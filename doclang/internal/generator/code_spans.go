// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

// markdownCodeSpans devuelve los code spans de s, cada uno como [inicio,
// fin] del span completo, con el emparejamiento de CommonMark que usa el HTML
// (renderer.findCodeSpans, que vive en core/internal/codespan y no está
// exportado): el delimitador es una corrida de N backticks y solo otra
// corrida de exactamente N la cierra; una corrida sin cierre de su largo son
// backticks literales; un backtick precedido por una cantidad impar de
// barras invertidas no abre (la apertura empieza en el siguiente de la
// corrida), y los cierres no miran escapes.
//
// Antes el DOCX usaba el regex "`([^`]+)`", que empareja backticks sueltos de
// izquierda a derecha: con un backtick escapado o un code span de dos
// backticks antes, el emparejamiento se corría uno y el código del DOCX no
// coincidía con el del HTML. Los code spans se emparejan una vez sobre todo el
// texto, como en el HTML, y no desde la posición del walker.
//
// Cada corrida se indexa por largo y el cierre se busca con una búsqueda
// binaria, así que es lineal salvo un logaritmo.
func markdownCodeSpans(s string) [][]int {
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
		byLen[ends[k]-starts[k]] = append(byLen[ends[k]-starts[k]], k)
	}
	var spans [][]int
	for k := 0; k < len(starts); k++ {
		p := starts[k]
		if oddBackslashesBefore(s, p) {
			p++
		}
		if p >= ends[k] {
			continue
		}
		idx := byLen[ends[k]-p]
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
			continue
		}
		q := idx[lo]
		spans = append(spans, []int{p, ends[q]})
		k = q
	}
	return spans
}

// oddBackslashesBefore dice si antes de s[i] hay una cantidad impar de
// barras invertidas seguidas.
func oddBackslashesBefore(s string, i int) bool {
	n := 0
	for k := i - 1; k >= 0 && s[k] == '\\'; k-- {
		n++
	}
	return n%2 == 1
}

// codeSpanContentBounds devuelve dónde empieza y termina el contenido del
// code span s[start:end]: sin las corridas de backticks y, como en CommonMark
// (y en el HTML), sin un espacio a cada lado si el contenido empieza y
// termina con espacio y no es solo espacios.
func codeSpanContentBounds(s string, start, end int) (int, int) {
	a := start
	for a < end && s[a] == '`' {
		a++
	}
	b := end
	for b > a && s[b-1] == '`' {
		b--
	}
	if b-a >= 2 && s[a] == ' ' && s[b-1] == ' ' {
		for k := a; k < b; k++ {
			if s[k] != ' ' {
				return a + 1, b - 1
			}
		}
	}
	return a, b
}
