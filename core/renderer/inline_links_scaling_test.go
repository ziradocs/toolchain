// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"
	"time"
)

// scalingInputs son líneas que crecen con n: muchos enlaces e imágenes
// válidos, aperturas que fallan (destino sin cierre), títulos y rutas con
// espacios. Recorrerlas tiene que ser lineal: antes cada candidato volvía a
// armar la vista desescapada del resto de la línea (cuadrático).
var scalingInputs = map[string]func(n int) string{
	"enlaces":      func(n int) string { return strings.Repeat("[a](https://x.com/p?a=1&b=2) ", n) },
	"imágenes":     func(n int) string { return strings.Repeat(`![a](<mi dir/a.png> "t") `, n) },
	"sin cierre":   func(n int) string { return strings.Repeat("[a](b&c ", n) },
	"con espacios": func(n int) string { return strings.Repeat("[a](mi página.html) ", n) },
	"parentesis":   func(n int) string { return strings.Repeat("[a](b (c ", n) },
	"énfasis":      func(n int) string { return strings.Repeat(`*a ![i](x*y.png "*t*") [b](u*v) `, n) },
}

func bestOf(f func()) time.Duration {
	best := time.Duration(1 << 62)
	for i := 0; i < 5; i++ {
		start := time.Now()
		f()
		if d := time.Since(start); d < best {
			best = d
		}
	}
	return best
}

func TestInlineLinksScaleLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	const n = 5000
	for name, gen := range scalingInputs {
		small, large := gen(n), gen(2*n)
		for _, fn := range []struct {
			name string
			run  func(string)
		}{
			{"ProcessInlineMarkdownSecure", func(s string) { ProcessInlineMarkdownSecure(s) }},
			{"FindInlineLinks", func(s string) { FindInlineLinks(s) }},
		} {
			t1 := bestOf(func() { fn.run(small) })
			t2 := bestOf(func() { fn.run(large) })
			// Lineal da ~2x y cuadrático ~4x; 3.5x deja margen para el ruido de CI.
			if t1 > 5*time.Millisecond && t2 > 7*t1/2 {
				t.Errorf("%s/%s: %v con n=%d y %v con 2n (más que lineal)", name, fn.name, t1, n, t2)
			}
			if t2 > 2*time.Second {
				t.Errorf("%s/%s: %v con 2n=%d", name, fn.name, t2, 2*n)
			}
		}
	}
}

func BenchmarkInlineLinksLine(b *testing.B) {
	line := strings.Repeat("[a](https://x.com/p?a=1&b=2) y ![i](<mi dir/a.png> \"t\") ", 2000)
	b.SetBytes(int64(len(line)))
	for i := 0; i < b.N; i++ {
		ProcessInlineMarkdownSecure(line)
	}
}
