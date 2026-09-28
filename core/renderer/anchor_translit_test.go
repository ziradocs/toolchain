// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"
	"unicode"
)

// Los anchors transliteran en vez de borrar letras con diacrítico.
func TestDeriveAnchorTransliterates(t *testing.T) {
	for in, want := range map[string]string{
		"Acompañar":            "acompanar",
		"Configuración rápida": "configuracion-rapida",
		"Übersicht":            "ubersicht",
		"Français ça va":       "francais-ca-va",
		"Straße":               "strasse",
		"Ærø":                  "aero",
		"Emoji 🚀 only":         "emoji--only",
		"日本語 title":            "-title",
	} {
		got := DeriveAnchor(in)
		if got != want {
			t.Errorf("DeriveAnchor(%q) = %q, want %q", in, got, want)
		}
		if again := DeriveAnchor(got); again != got {
			t.Errorf("DeriveAnchor is not idempotent on %q: %q", got, again)
		}
	}
}

// Ninguna letra latina se pierde: cada letra del texto (con o sin tilde,
// diéresis o eñe) aporta al menos una letra al anchor, en el mismo orden.
func TestDeriveAnchorNeverDropsALetter(t *testing.T) {
	for _, text := range []string{
		"Acompañar a la niña",
		"Pingüino güero",
		"Canción árbol índice óptimo último",
		"Él está aquí",
		"ÑANDÚ Ñoño",
		"Crème brûlée à la façon",
	} {
		want := 0
		for _, r := range transliterateAnchor(strings.ToLower(text)) {
			if r >= 'a' && r <= 'z' {
				want++
			}
		}
		letters := 0
		for _, r := range strings.ToLower(text) {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		got := 0
		for _, r := range DeriveAnchor(text) {
			if r >= 'a' && r <= 'z' {
				got++
			}
		}
		if got != letters || want != letters {
			t.Errorf("%q: %d letters in, %d in anchor %q", text, letters, got, DeriveAnchor(text))
		}
	}
}
