// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"math/rand"
	"runtime"
	"strings"
	"testing"
	"time"
)

// oldRewriteInlineHTML es rewriteInlineHTML de v2.49.2, que devolvía los
// tramos de código con un strings.Replace por tramo.
func oldRewriteInlineHTML(line string) string {
	var protected []string
	masked := codeSpanPattern.ReplaceAllStringFunc(line, func(match string) string {
		protected = append(protected, match)
		return codeSentinel(len(protected) - 1)
	})
	for _, rw := range inlineHTMLRewrites {
		masked = rw.re.ReplaceAllString(masked, rw.repl)
	}
	for i, original := range protected {
		replacement := original
		if m := cleanCodeContent.FindStringSubmatch(original); m != nil {
			replacement = "`" + m[1] + "`"
		}
		masked = strings.Replace(masked, codeSentinel(i), replacement, 1)
	}
	return masked
}

func TestRewriteInlineHTMLMatchesOld(t *testing.T) {
	alphabet := []string{"`", "a", " ", "<u>", "</u>", "<code>", "</code>", "<code><u>x</u></code>", "<kbd>", "</kbd>", "<sub>", "</sub>", "<mark>", "</mark>", "x", "1"}
	r := rand.New(rand.NewSource(8))
	for i := 0; i < 50000; i++ {
		var b strings.Builder
		for n := 1 + r.Intn(20); n > 0; n-- {
			b.WriteString(alphabet[r.Intn(len(alphabet))])
		}
		s := b.String()
		if got, want := rewriteInlineHTML(s), oldRewriteInlineHTML(s); got != want {
			t.Fatalf("%q: %q, antes %q", s, got, want)
		}
	}
}

// Con muchos tramos de código en una línea es lineal (antes, O(tramos ·
// largo)).
func TestRewriteInlineHTMLScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	best := func(s string) time.Duration {
		d := time.Duration(1 << 62)
		for i := 0; i < 5; i++ {
			runtime.GC()
			st := time.Now()
			rewriteInlineHTML(s)
			if e := time.Since(st); e < d {
				d = e
			}
		}
		return d
	}
	for name, gen := range map[string]func(int) string{
		"code spans":  func(n int) string { return strings.Repeat("`c` ", n) },
		"tags <code>": func(n int) string { return strings.Repeat("<code>c</code> <u>x</u> ", n) },
	} {
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
