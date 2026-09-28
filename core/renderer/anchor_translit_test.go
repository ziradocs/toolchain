// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "testing"

// Los anchors transliteran en vez de borrar letras con diacrítico.
func TestDeriveAnchorTransliterates(t *testing.T) {
	for in, want := range map[string]string{
		"Acompañar":           "acompanar",
		"Configuración rápida": "configuracion-rapida",
		"Übersicht":           "ubersicht",
		"Français ça va":      "francais-ca-va",
		"Straße":              "strasse",
		"Ærø":                 "aero",
		"Emoji 🚀 only":        "emoji--only",
		"日本語 title":           "-title",
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
