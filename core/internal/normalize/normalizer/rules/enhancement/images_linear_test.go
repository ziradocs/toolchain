// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"math/rand"
	"runtime"
	"strings"
	"testing"
	"time"
)

// oldImagesApply es ImagesRule.Apply de v2.49.2: un strings.Replace sobre la
// línea entera por cada imagen.
func oldImagesApply(r *ImagesRule, content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		newLine := line
		for _, m := range r.findImageMatches(line) {
			originalPath := line[m.pathStart:m.pathEnd]
			normalizedPath := r.pathNormalizer.NormalizeImagePath(originalPath)
			if originalPath != normalizedPath {
				oldImageTag := line[m.start:m.end]
				newImageTag := strings.Replace(oldImageTag, originalPath, normalizedPath, 1)
				newLine = strings.Replace(newLine, oldImageTag, newImageTag, 1)
			}
		}
		lines[i] = newLine
	}
	return strings.Join(lines, "\n")
}

func TestImagesRuleMatchesOld(t *testing.T) {
	r := NewImagesRule()
	parts := []string{"![", "](", ")", "a", "b.png", "assets/x.png", "d/c.jpg", " ", "\n", "!", "[", "]", "("}
	rng := rand.New(rand.NewSource(11))
	for i := 0; i < 50000; i++ {
		var b strings.Builder
		for n := 1 + rng.Intn(20); n > 0; n-- {
			b.WriteString(parts[rng.Intn(len(parts))])
		}
		s := b.String()
		got, _ := r.Apply(s)
		if want := oldImagesApply(r, s); got != want {
			t.Fatalf("%q: %q, antes %q", s, got, want)
		}
	}
}

// Muchas imágenes en una línea: lineal (antes, cuadrático).
func TestImagesRuleScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	r := NewImagesRule()
	best := func(s string) time.Duration {
		d := time.Duration(1 << 62)
		for i := 0; i < 5; i++ {
			runtime.GC()
			st := time.Now()
			_, _ = r.Apply(s)
			if e := time.Since(st); e < d {
				d = e
			}
		}
		return d
	}
	gen := func(n int) string { return strings.Repeat("![a](b.png) ", n) }
	n := 1000
	for n < 1<<18 && best(gen(n)) < 3*time.Millisecond {
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
	t.Errorf("%v con n=%d y %v con 4n", t1, n, t4)
}
