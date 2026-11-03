// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/renderer"
)

// linkDestinationEnd lee lo mismo que renderer.ScanLinkDestination: sobre un
// corpus de casos de la gramática y sobre entradas aleatorias de su alfabeto.
func TestLinkDestinationEndMatchesCore(t *testing.T) {
	corpus := []string{
		"img/a.png)", `img/a.png "t")`, "img/a.png 't')", "img/a.png (t))", `img/a.png   "t"  ) y`,
		"<mi dir/a.png>)", `<mi dir/a.png> "t")`, "<a)b.png>)", "foto_(1).png)", `a.png "di \"hola\"")`,
		" a.png )", " )", ")", `a.png"t")`, "t.png bare)", "mi dir/a.png)", `a.png "t" x)`, "a (b c))",
		"a(b c))", "a.png (t(u)))", `<a.png>"t")`, "<a b", "<a<b>)", "<a b) c", "foto_(1.png", "b (c",
		"a.png \"t\nu\")", "fotos/a (1).png) fin", "c d.png 'e)f') fin", `x\)*y*) y`, "b (c [a](b (c )",
	}
	check := func(s string) {
		t.Helper()
		_, want, wantOK := renderer.ScanLinkDestination(s)
		got, ok := linkDestinationEnd(s, 0, balancedCloses(s))
		if ok != wantOK || (ok && got != want) {
			t.Errorf("linkDestinationEnd(%q) = %d %v; ScanLinkDestination = %d %v", s, got, ok, want, wantOK)
		}
	}
	for _, s := range corpus {
		check(s)
	}
	rng := rand.New(rand.NewSource(1))
	const alphabet = "ab ()<>\"'\\\t\n*"
	for i := 0; i < 20000; i++ {
		b := make([]byte, 1+rng.Intn(14))
		for j := range b {
			b[j] = alphabet[rng.Intn(len(alphabet))]
		}
		check(string(b))
	}
}

// maskLinkDestinations es lineal también con aperturas que no cierran solas
// y un ")" al final de la línea: antes cada apertura leía hasta ese ")".
// El tamaño crece hasta que la medida no es ruido, para que la comparación
// de n contra 2n nunca quede vacía.
func TestMaskLinkDestinationsScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	for name, gen := range map[string]func(n int) string{
		"aperturas y un cierre": func(n int) string { return strings.Repeat("[a](b (c ", n) + ")" },
		"imágenes y un cierre":  func(n int) string { return strings.Repeat("![a](b (c ", n) + ")" },
		"sin cierre":            func(n int) string { return strings.Repeat("[a](b (c ![d](e ", n) },
		"enlaces e imágenes":    func(n int) string { return strings.Repeat(`*a* ![i](x*y.png "t") [b](u*v) `, n) },
		"con espacios":          func(n int) string { return strings.Repeat("[a](mi página.html) ![b](mi foto.png) ", n) },
	} {
		assertLinear(t, name, func(s string) { maskLinkDestinations(s, docxCodeSpanRe) }, gen)
	}
}

// assertLinear mide f sobre gen(n) y gen(2n), con n creciendo hasta que
// gen(n) tarde al menos 20 ms, y falla si el doble tarda más de 3.5 veces.
func assertLinear(t *testing.T, name string, f func(string), gen func(int) string) {
	t.Helper()
	best := func(s string) time.Duration {
		b := time.Duration(1 << 62)
		for i := 0; i < 5; i++ {
			start := time.Now()
			f(s)
			if d := time.Since(start); d < b {
				b = d
			}
		}
		return b
	}
	n := 500
	t1 := best(gen(n))
	for t1 < 20*time.Millisecond && n < 1<<20 {
		n *= 2
		t1 = best(gen(n))
	}
	t2 := best(gen(2 * n))
	if t2 > 7*t1/2 {
		t.Errorf("%s: %v con n=%d y %v con 2n (más que lineal)", name, t1, n, t2)
	}
}
