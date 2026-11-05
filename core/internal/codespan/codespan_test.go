// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package codespan

import (
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// oracle es el bucle que tenían renderer.findCodeSpans,
// elements.codeSpanRanges y formatter.codeSpanRunRanges en v2.49.2.
func oracle(s []byte, escaped func(int) bool) []Span {
	var spans []Span
	n := len(s)
	i := 0
	for i < n {
		if s[i] != '`' || escaped(i) {
			i++
			continue
		}
		start := i
		for i < n && s[i] == '`' {
			i++
		}
		openLen := i - start
		for j := i; j < n; {
			if s[j] != '`' {
				j++
				continue
			}
			closeStart := j
			for j < n && s[j] == '`' {
				j++
			}
			if j-closeStart == openLen {
				spans = append(spans, Span{OpenStart: start, OpenEnd: start + openLen, CloseStart: closeStart, CloseEnd: j})
				i = j
				break
			}
		}
	}
	return spans
}

func oddBackslashes(s []byte) func(int) bool {
	return func(i int) bool {
		c := 0
		for k := i - 1; k >= 0 && s[k] == '\\'; k-- {
			c++
		}
		return c%2 == 1
	}
}

func TestFindMatchesOldLoop(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	alphabet := []byte("``` a\\")
	for i := 0; i < 50000; i++ {
		s := make([]byte, 1+r.Intn(30))
		for j := range s {
			s[j] = alphabet[r.Intn(len(alphabet))]
		}
		got, want := Find(s, oddBackslashes(s)), oracle(s, oddBackslashes(s))
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: %v, antes %v", s, got, want)
		}
	}
}

// Find escala como n log n: con 4n tarda ~4-5 veces lo de n (el bucle viejo,
// ~16 con aperturas que no cierran).
func TestFindScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	for name, gen := range map[string]func(n int) string{
		"barra y dos backticks": func(n int) string { return strings.Repeat("\\``a", n) },
		"backtick escapado":     func(n int) string { return strings.Repeat("](\\`", n) },
		"spans cortos":          func(n int) string { return strings.Repeat("`c` ", n) },
		"largos crecientes": func(n int) string {
			var b strings.Builder
			for i := 1; b.Len() < 4*n; i++ {
				b.WriteString(strings.Repeat("`", i%200+1))
				b.WriteString("a")
			}
			return b.String()
		},
	} {
		run := func(s string) { b := []byte(s); Find(b, oddBackslashes(b)) }
		best := func(s string) time.Duration {
			d := time.Duration(1 << 62)
			for i := 0; i < 5; i++ {
				runtime.GC()
				st := time.Now()
				run(s)
				if e := time.Since(st); e < d {
					d = e
				}
			}
			return d
		}
		n := 1000
		for n < 1<<18 && best(gen(n)) < 3*time.Millisecond {
			n *= 2
		}
		small, large := gen(n), gen(4*n)
		ok := false
		var t1, t4 time.Duration
		for a := 0; a < 3 && !ok; a++ {
			t1, t4 = best(small), best(large)
			ok = t4 < 10*t1
		}
		if !ok {
			t.Errorf("%s: %v con n=%d y %v con 4n", name, t1, n, t4)
		}
	}
}
