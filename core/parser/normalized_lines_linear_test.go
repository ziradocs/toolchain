// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

// normalizedLineMap no es cuadrático: ni con muchos tramos reescritos entre
// líneas distintas, ni con un archivo de líneas repetidas que no dan anclas.
func TestNormalizedLineMapScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	// Cada bloque: una línea única, un fence de tres líneas que en seen
	// pierde el cierre y cambia la sangría, y una línea en blanco.
	distinct := func(n int) (string, []string) {
		var src, seen []string
		for i := 0; i < n; i++ {
			src = append(src, fmt.Sprintf("párrafo %d", i), "```mermaid", "graph TD", "```", "")
			seen = append(seen, fmt.Sprintf("párrafo %d", i), "<<mermaid>>", "  graph TD", "")
		}
		return strings.Join(src, "\n"), seen
	}
	repeated := func(n int) (string, []string) {
		src := strings.Repeat("---\n\n", n)
		seen := strings.Split(strings.Repeat("***\n\n", n), "\n")
		return src, seen
	}
	for name, gen := range map[string]func(int) (string, []string){"distinct": distinct, "repeated": repeated} {
		t.Run(name, func(t *testing.T) {
			best := func(n int) time.Duration {
				src, seen := gen(n)
				d := time.Duration(1 << 62)
				for i := 0; i < 5; i++ {
					runtime.GC()
					st := time.Now()
					_ = normalizedLineMap(src, seen)
					if e := time.Since(st); e < d {
						d = e
					}
				}
				return d
			}
			n := 500
			for n < 1<<18 && best(n) < 3*time.Millisecond {
				n *= 2
			}
			var t1, t4 time.Duration
			for a := 0; a < 3; a++ {
				t1, t4 = best(n), best(4*n)
				if t4 < 10*t1 {
					return
				}
			}
			t.Errorf("%v con n=%d y %v con 4n", t1, n, t4)
		})
	}
}
