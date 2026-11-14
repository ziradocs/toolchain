// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"math/rand"
	"strconv"
	"testing"
)

// naiveAnchors es HeadingAnchors antes de la pista next: prueba cada sufijo
// desde 2.
type naiveAnchors struct{ used map[string]int }

func (a *naiveAnchors) unique(text string) string {
	base := DeriveAnchor(text)
	prefixed := "heading"
	if base != "" {
		prefixed = "heading-" + base
	}
	candidate := prefixed
	for n := 2; a.used[candidate] > 0; n++ {
		candidate = prefixed + "-" + strconv.Itoa(n)
	}
	a.used[candidate]++
	return candidate
}

// La pista next no cambia ningún anchor: la misma secuencia de encabezados y
// de id declarados (Reserve) da lo mismo que la búsqueda desde 2, también
// cuando un id declarado ocupa un sufijo por delante de la pista.
func TestHeadingAnchorsMatchesNaive(t *testing.T) {
	texts := []string{"Intro", "intro", "Intro 2", "", "Fin", "Intro-2"}
	for seed := int64(1); seed <= 200; seed++ {
		rng := rand.New(rand.NewSource(seed))
		var got HeadingAnchors
		want := naiveAnchors{used: map[string]int{}}
		for step := 0; step < 60; step++ {
			if rng.Intn(4) == 0 {
				id := "heading-intro-" + strconv.Itoa(2+rng.Intn(12))
				got.Reserve(id)
				want.used[id]++
				continue
			}
			text := texts[rng.Intn(len(texts))]
			derived := got.Derive(text)
			g, w := got.Unique(text), want.unique(text)
			if g != w || derived != g {
				t.Fatalf("seed %d step %d: Unique(%q) = %q (Derive %q), naive %q", seed, step, text, g, derived, w)
			}
		}
	}
}
