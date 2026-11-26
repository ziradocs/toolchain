// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"sort"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// El normalizador de flex reescribe el texto antes de que el parser lo lea,
// y varias de sus reglas cambian el número de líneas: MermaidRule quita el
// ``` de cierre de cada bloque, TablesRule agrega el encabezado y el
// separador de la tabla que arma con una lista, MarkdownSlideStructureRule
// (solo slides) inserta líneas. El parser numera las líneas del texto que
// leyó, así que sin traducirlas cada posición del AST y cada diagnóstico
// después del primer cambio apuntaba a otra línea del archivo: con diez
// bloques mermaid antes, un TABLE003 caía diez líneas arriba de la fila que
// fallaba. El normalizador corre entero solo cuando el Detector puntúa el
// documento (un documento sin frontmatter ya suma por missing_frontmatter),
// por eso el mismo archivo con frontmatter daba la línea correcta.
//
// En vez de pedirle a cada regla que conserve las líneas (hoy ninguna lo
// garantiza, y la próxima tampoco lo haría), se compara el texto que leyó el
// parser con el de antes de normalizar y se traduce cada línea leída a la
// del fuente de la que viene.

// normalizedLineMap devuelve, para cada línea (1-based) de seen, la línea
// (1-based) de source de la que viene; el índice 0 no se usa. Las líneas
// iguales se emparejan como en un diff de paciencia: se fija el prefijo y el
// sufijo comunes, luego las líneas que aparecen una sola vez en cada lado, en
// el orden más largo que comparten, y se repite entre cada par de anclas. Un
// tramo sin ninguna línea única (líneas en blanco, la misma tabla dos veces,
// el mismo diagrama otra vez) se empareja con la subsecuencia común más
// larga (Myers), acotada por myersMaxCost. Lo que queda sin pareja (un tramo
// reescrito) se reparte en proporción sobre el tramo del fuente que
// reemplazó, así que su primera línea cae en la primera del tramo original:
// la apertura de un ```mermaid convertido a <<mermaid>> apunta a la línea
// del ```mermaid. Una línea insertada sin nada en el fuente que reemplazar
// cae en la línea del fuente que la sigue (la primera viñeta de la lista que
// TablesRule convirtió en tabla), o en la última si va al final.
//
// Límite: el mapa se reconstruye comparando los dos textos, no lo entrega
// el normalizador (sus reglas son string → string y ninguna dice qué líneas
// quitó o agregó). Dentro de un tramo reescrito, una línea nueva que no es
// igual a ninguna del fuente cae en proporción, así que un diagnóstico sobre
// una línea que el normalizador cambió apunta al tramo, no siempre a la
// línea exacta. Las líneas que el normalizador deja iguales (una fila de
// tabla, un fence de código) apuntan a la suya, salvo que el tramo sin
// anclas pase de myersMaxCost, donde todo el tramo se reparte en proporción.
//
// El costo es O(n log n) en el número de líneas para la parte de paciencia:
// cada nivel de la recursión recorre su tramo una vez, y la profundidad
// está acotada (maxLineMapDepth); pasado ese límite el tramo va a Myers o se
// reparte en proporción. Myers cuesta O((n+m)·d) en un tramo de n y m
// líneas con d diferencias, y se corta en myersMaxCost.
func normalizedLineMap(source string, seen []string) []int {
	src := strings.Split(source, "\n")
	// Una entrada más: una posición en la línea que sigue a la última (el
	// fin del archivo) sigue siendo la que sigue a la última del fuente.
	out := make([]int, len(seen)+2)
	m := lineMatcher{src: src, seen: seen, out: out}
	m.match(0, len(src), 0, len(seen), 0)
	out[len(seen)+1] = len(src) + 1
	return out
}

// maxLineMapDepth acota la recursión de lineMatcher.match. Un texto real
// la resuelve en pocos niveles; el límite está para que una entrada hecha
// a propósito no la vuelva cuadrática.
const maxLineMapDepth = 64

// myersMaxCost acota el trabajo de lcsPairs en un tramo: (n+m)·d pasos y
// d² enteros de memoria. Con 1<<22 un tramo de 4 000 líneas admite unas mil
// diferencias.
const myersMaxCost = 1 << 22

type lineMatcher struct {
	src, seen []string
	out       []int
}

// match empareja src[a0:a1] con seen[b0:b1] y escribe out[b+1] para cada b
// del tramo.
func (m *lineMatcher) match(a0, a1, b0, b1, depth int) {
	for a0 < a1 && b0 < b1 && m.src[a0] == m.seen[b0] {
		m.out[b0+1] = a0 + 1
		a0++
		b0++
	}
	for a0 < a1 && b0 < b1 && m.src[a1-1] == m.seen[b1-1] {
		m.out[b1] = a1
		a1--
		b1--
	}
	if b0 >= b1 {
		return
	}
	if a0 >= a1 {
		m.spread(a0, a1, b0, b1)
		return
	}
	var anchors []lineAnchor
	if depth < maxLineMapDepth {
		anchors = m.uniqueAnchors(a0, a1, b0, b1)
	}
	if len(anchors) == 0 {
		m.matchCommon(a0, a1, b0, b1)
		return
	}
	pa, pb := a0, b0
	for _, an := range anchors {
		m.match(pa, an.a, pb, an.b, depth+1)
		m.out[an.b+1] = an.a + 1
		pa, pb = an.a+1, an.b+1
	}
	m.match(pa, a1, pb, b1, depth+1)
}

// matchCommon empareja un tramo sin anclas únicas con la subsecuencia común
// más larga y reparte en proporción lo que queda entre pares. Si el tramo es
// demasiado caro para lcsPairs, lo reparte entero.
func (m *lineMatcher) matchCommon(a0, a1, b0, b1 int) {
	pairs, ok := m.lcsPairs(a0, a1, b0, b1)
	if !ok {
		m.spread(a0, a1, b0, b1)
		return
	}
	pa, pb := a0, b0
	for _, p := range pairs {
		if p.b > pb {
			m.spread(pa, p.a, pb, p.b)
		}
		m.out[p.b+1] = p.a + 1
		pa, pb = p.a+1, p.b+1
	}
	if pb < b1 {
		m.spread(pa, a1, pb, b1)
	}
}

// lcsPairs devuelve los pares (a, b) de una subsecuencia común más larga de
// src[a0:a1] y seen[b0:b1], en orden (Myers, "An O(ND) Difference Algorithm
// and Its Variations", 1986). ok es false si las diferencias pasan de lo que
// permite myersMaxCost.
func (m *lineMatcher) lcsPairs(a0, a1, b0, b1 int) (pairs []lineAnchor, ok bool) {
	n, k := a1-a0, b1-b0
	maxD := n + k
	if lim := myersMaxCost / (n + k + 1); lim < maxD {
		maxD = lim
	}
	for maxD*maxD > myersMaxCost {
		maxD /= 2
	}
	off := maxD + 1
	v := make([]int, 2*off+1)
	// trace[d] son los v[kk] de kk = -d..d después del paso d.
	var trace [][]int
	at := func(row []int, d, kk int) int { return row[kk+d] }
	for d := 0; d <= maxD; d++ {
		for kk := -d; kk <= d; kk += 2 {
			var x int
			if kk == -d || (kk != d && v[off+kk-1] < v[off+kk+1]) {
				x = v[off+kk+1]
			} else {
				x = v[off+kk-1] + 1
			}
			y := x - kk
			for x < n && y < k && m.src[a0+x] == m.seen[b0+y] {
				x++
				y++
			}
			v[off+kk] = x
			if x >= n && y >= k {
				trace = append(trace, append([]int(nil), v[off-d:off+d+1]...))
				return m.lcsBacktrack(trace, at, a0, b0, n, k), true
			}
		}
		trace = append(trace, append([]int(nil), v[off-d:off+d+1]...))
	}
	return nil, false
}

// lcsBacktrack recorre trace de atrás para adelante y junta las diagonales
// (líneas iguales) del camino.
func (m *lineMatcher) lcsBacktrack(trace [][]int, at func([]int, int, int) int, a0, b0, x, y int) []lineAnchor {
	var rev []lineAnchor
	for d := len(trace) - 1; d > 0; d-- {
		prev := trace[d-1]
		kk := x - y
		var pk int
		if kk == -d || (kk != d && at(prev, d-1, kk-1) < at(prev, d-1, kk+1)) {
			pk = kk + 1
		} else {
			pk = kk - 1
		}
		px := at(prev, d-1, pk)
		py := px - pk
		sx := px + 1 // el paso fue a la derecha: una línea del fuente sin pareja
		if pk == kk+1 {
			sx = px // el paso fue hacia abajo: una línea de seen sin pareja
		}
		for x > sx && y > sx-kk {
			x--
			y--
			rev = append(rev, lineAnchor{a: a0 + x, b: b0 + y})
		}
		x, y = px, py
	}
	for x > 0 && y > 0 {
		x--
		y--
		rev = append(rev, lineAnchor{a: a0 + x, b: b0 + y})
	}
	out := make([]lineAnchor, len(rev))
	for i, p := range rev {
		out[len(rev)-1-i] = p
	}
	return out
}

// spread reparte seen[b0:b1] sobre src[a0:a1] en proporción. Con src vacío
// (una inserción pura) todo cae en la línea del fuente que sigue al tramo, o
// en la última del fuente si el tramo está al final.
func (m *lineMatcher) spread(a0, a1, b0, b1 int) {
	n, k := a1-a0, b1-b0
	for j := 0; j < k; j++ {
		var line int
		switch {
		case n > 0:
			line = a0 + j*n/k + 1
		case a0 < len(m.src):
			line = a0 + 1
		default:
			line = len(m.src)
		}
		if line < 1 {
			line = 1
		}
		m.out[b0+j+1] = line
	}
}

type lineAnchor struct{ a, b int }

// uniqueAnchors devuelve los pares (a, b) de líneas que aparecen una sola
// vez en src[a0:a1] y una sola vez en seen[b0:b1], reducidos a la
// subsecuencia creciente más larga en los dos índices.
func (m *lineMatcher) uniqueAnchors(a0, a1, b0, b1 int) []lineAnchor {
	type count struct{ a, b, na, nb int }
	counts := make(map[string]*count, b1-b0)
	for b := b0; b < b1; b++ {
		c := counts[m.seen[b]]
		if c == nil {
			c = &count{}
			counts[m.seen[b]] = c
		}
		c.nb++
		c.b = b
	}
	for a := a0; a < a1; a++ {
		if c := counts[m.src[a]]; c != nil {
			c.na++
			c.a = a
		}
	}
	var pairs []lineAnchor
	for b := b0; b < b1; b++ {
		if c := counts[m.seen[b]]; c.na == 1 && c.nb == 1 {
			pairs = append(pairs, lineAnchor{a: c.a, b: b})
		}
	}
	return longestIncreasingAnchors(pairs)
}

// longestIncreasingAnchors toma pares ordenados por b y devuelve la
// subsecuencia más larga creciente también en a (patience sorting).
func longestIncreasingAnchors(pairs []lineAnchor) []lineAnchor {
	if len(pairs) == 0 {
		return nil
	}
	tails := []int{}                // índice en pairs del menor final de cada largo
	prev := make([]int, len(pairs)) // predecesor de cada par en su cadena
	for i, p := range pairs {
		k := sort.Search(len(tails), func(j int) bool { return pairs[tails[j]].a >= p.a })
		if k > 0 {
			prev[i] = tails[k-1]
		} else {
			prev[i] = -1
		}
		if k == len(tails) {
			tails = append(tails, i)
		} else {
			tails[k] = i
		}
	}
	out := make([]lineAnchor, len(tails))
	for i, k := tails[len(tails)-1], len(tails)-1; k >= 0; i, k = prev[i], k-1 {
		out[k] = pairs[i]
	}
	return out
}

// seenLines arma el texto que leyó un parser de cuerpo: las primeras offset
// líneas de source (el frontmatter, que el normalizador no toca) seguidas
// del cuerpo que recibió.
func seenLines(source string, offset int, body []string) []string {
	src := strings.Split(source, "\n")
	if offset > len(src) {
		offset = len(src)
	}
	out := make([]string, 0, offset+len(body))
	out = append(out, src[:offset]...)
	return append(out, body...)
}

// restoreNormalizedPositions lleva las posiciones de doc y de diags, que
// cuentan las líneas de seen, a las de source. No hace nada si el parser
// leyó source tal cual.
func restoreNormalizedPositions(doc *ast.AST, diags []diagnostics.Diagnostic, source string, seen []string) {
	if strings.Join(seen, "\n") == source {
		return
	}
	lineMap := normalizedLineMap(source, seen)
	restoreNodePositions(doc, lineMap)
	restoreDiagnosticPositions(diags, lineMap)
}
