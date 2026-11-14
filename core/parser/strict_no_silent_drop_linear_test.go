// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/util"
)

// Los avisos de líneas ignoradas se arman dentro de los bucles de cada
// elemento: un elemento con n líneas sueltas tiene que costar O(n), no O(n²).
// Mismo método que los demás ...ScalesLinearly: el mejor de cinco tiempos con
// n y con 4n, y 4n no puede costar diez veces más.
func TestStrictNoSilentDrop_IgnoredLinesScaleLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	bodies := map[string]func(n int) string{
		"POINTS": func(n int) string {
			return "  POINTS\n    - uno\n" + strings.Repeat("    suelta\n", n)
		},
		"IMAGE": func(n int) string {
			var b strings.Builder
			b.WriteString("  IMAGE \"a.png\" \"b\"\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&b, "    k%d: v\n", i)
			}
			return b.String()
		},
		"TABLE": func(n int) string {
			return "  TABLE\n    | a | b |\n" + strings.Repeat("    nota\n", n)
		},
		"CHART": func(n int) string {
			return "  <<chart: bar>>\n    data: [[\"a\", 1]]\n    series: [\"s\"]\n" + strings.Repeat("      anidada\n", n)
		},
		// Las llaves desconocidas de un bloque YAML se cuentan con una llave
		// cuyo valor crece, no con n llaves: yaml.v3 compara cada llave de un
		// mapeo con todas las demás para rechazar duplicados (O(k²) en la
		// biblioteca, fuera de este cambio), y eso taparía lo que se mide.
		"METRIC": func(n int) string {
			return "  <<metric>>\n    label: L\n    notas:\n" + strings.Repeat("      - x\n", n) + "  <<end>>\n"
		},
		"QUIZ": func(n int) string {
			return "  <<quiz>>\n    question: Q\n    options: [a, b]\n    answer: 0\n    notas:\n" + strings.Repeat("      - x\n", n) + "  <<end>>\n"
		},
		"CODEGROUP": func(n int) string {
			return "  :::code-group\n  ```js [a]\n  x()\n  ```\n" + strings.Repeat("  suelta\n", n) + "  :::\n"
		},
		"QUOTE": func(n int) string {
			return "  QUOTE\n    cita\n" + strings.Repeat("    AUTHOR: A\n", n)
		},
		"CHECKLIST": func(n int) string {
			return "  CHECKLIST\n    [ ] uno\n" + strings.Repeat("    [ ]\n", n)
		},
		// Esta y las dos de abajo eran cuadráticas en origin/main (ver el PR):
		// la continuación de un ítem copiaba el texto entero por línea, y k
		// encabezados iguales de un deck probaban los k sufijos anteriores
		// cada uno.
		"CHECKLIST continuation": func(n int) string {
			return "  CHECKLIST\n    [ ] uno\n" + strings.Repeat("    sigue el texto\n", n)
		},
	}
	// La continuación de un item flex (parseMarkdownList) y los encabezados
	// de un deck van por su propio dialecto.
	t.Run("flex list continuation", func(t *testing.T) {
		assertLinear(t, func(n int) string {
			return "---\nmode: flex\n---\n# A\n\n- uno\n" + strings.Repeat("  sigue el texto\n", n)
		}, false)
	})
	t.Run("repeated slide headings", func(t *testing.T) {
		assertLinear(t, func(n int) string {
			return noDropDocHeader + "SLIDE content\n  title: \"T\"\n" + strings.Repeat("  SECTION \"Intro\"\n", n)
		}, true)
	})
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			assertLinear(t, func(n int) string { return noDropDocHeader + "SECTION \"S\"\n" + body(n) }, false)
		})
	}
}

// assertLinear falla si parsear gen(4n) cuesta diez veces o más que gen(n).
func assertLinear(t *testing.T, gen func(n int) string, slide bool) {
	t.Helper()
	best := func(src string) time.Duration {
		d := time.Duration(1 << 62)
		for i := 0; i < 5; i++ {
			runtime.GC()
			st := time.Now()
			if slide {
				_, _ = New(util.NewNoop()).Parse(src, "big.slidelang")
			} else {
				_, _ = New(util.NewNoop()).ParseDocument(src, "big.doclang")
			}
			if e := time.Since(st); e < d {
				d = e
			}
		}
		return d
	}
	// Una base de 10 ms, no de 3: con todos los paquetes de `go test ./...`
	// corriendo a la vez, una medición de pocos milisegundos es puro ruido.
	n := 500
	for n < 1<<17 && best(gen(n)) < 10*time.Millisecond {
		n *= 2
	}
	small, large := gen(n), gen(4*n)
	var t1, t4 time.Duration
	for a := 0; a < 3; a++ {
		t1, t4 = best(small), best(large)
		if t4 < 10*t1 {
			return
		}
	}
	t.Errorf("%v with n=%d and %v with 4n", t1, n, t4)
}
