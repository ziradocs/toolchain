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
// el orden más largo que comparten, y se repite entre cada par de anclas.
// Lo que queda sin pareja (un tramo reescrito) se reparte en proporción
// sobre el tramo del fuente que reemplazó, así que su primera línea cae en
// la primera del tramo original: la apertura de un ```mermaid convertido a
// <<mermaid>> apunta a la línea del ```mermaid. Una línea insertada sin nada
// en el fuente que reemplazar cae en la línea del fuente que la sigue (la
// primera viñeta de la lista que TablesRule convirtió en tabla), o en la
// última si va al final.
//
// El costo es O(n log n) en el número de líneas: cada nivel de la
// recursión recorre su tramo una vez, y la profundidad está acotada
// (maxLineMapDepth); pasado ese límite el tramo se reparte en proporción.
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
	if a0 >= a1 || depth >= maxLineMapDepth {
		m.spread(a0, a1, b0, b1)
		return
	}
	anchors := m.uniqueAnchors(a0, a1, b0, b1)
	if len(anchors) == 0 {
		m.spread(a0, a1, b0, b1)
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
