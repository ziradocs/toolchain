// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// bestOf devuelve el menor de k tiempos de f, cada uno después de un GC: el
// mínimo es lo que tarda f cuando nada la interrumpe, que es lo que se
// compara entre tamaños.
func bestOf(k int, f func()) time.Duration {
	best := time.Duration(1 << 62)
	for i := 0; i < k; i++ {
		runtime.GC()
		start := time.Now()
		f()
		if d := time.Since(start); d < best {
			best = d
		}
	}
	return best
}

// TestInlineLinksScaleLinearly mide el pipeline completo (HTML y LINK001)
// sobre las líneas de linearInputs, con n y con 4n: lineal da ~4x y
// cuadrático ~16x, así que el umbral de 10x deja margen de sobra para el
// ruido de una máquina cargada sin dejar pasar un caso cuadrático. n se
// calibra para que la medida chica dure al menos unos milisegundos (la
// resolución del reloj no pesa), y una comparación que falla se repite antes
// de reportarla: una pausa ajena puede inflar una medida, no las tres. La
// prueba que no depende del tiempo es TestLinkFinderStepsAreLinear.
func TestInlineLinksScaleLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	fns := []struct {
		name string
		run  func(string)
	}{
		{"ProcessInlineMarkdownSecure", func(s string) { ProcessInlineMarkdownSecure(s) }},
		{"FindInlineLinks", func(s string) { FindInlineLinks(s) }},
	}
	for name, gen := range linearInputs {
		for _, fn := range fns {
			n := 250
			for n < 32000 && bestOf(3, func() { fn.run(gen(n)) }) < 3*time.Millisecond {
				n *= 2
			}
			small, large := gen(n), gen(4*n)
			var t1, t4 time.Duration
			ok := false
			for attempt := 0; attempt < 3 && !ok; attempt++ {
				t1 = bestOf(5, func() { fn.run(small) })
				t4 = bestOf(5, func() { fn.run(large) })
				ok = t4 < 10*t1
			}
			if !ok {
				t.Errorf("%s/%s: %v con n=%d y %v con 4n (más que lineal)", name, fn.name, t1, n, t4)
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
