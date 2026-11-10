// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import "testing"

// Issue #489: la palabra clave tiene que ser una palabra completa para cortar
// un cuerpo.
func TestHasStrictKeyword(t *testing.T) {
	for _, tc := range []struct {
		line string
		want bool
	}{
		{"TEXT", true},
		{"TEXT hola", true},
		{"TEXT\thola", true},
		{"TEXT hola", true},
		{"TEXT: hola", true},
		{"TEXT{x}", true},
		{"TEXTO citado", false},
		{"TEXTOS", false},
		{"TEXT-heavy", false},
		{"TEXT.", false},
		{"text", false},
		{"", false},
	} {
		if got := HasStrictKeyword(tc.line, "TEXT"); got != tc.want {
			t.Errorf("HasStrictKeyword(%q, TEXT) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestIsNewElement_StrictKeywordBoundary(t *testing.T) {
	for line, want := range map[string]bool{
		"TEXTO citado":  false,
		"MAPA norte":    false,
		"CODEX":         false,
		"TEXT hola":     true,
		"MAP":           true,
		"SECTION \"B\"": true,
		"SECTIONS":      false,
	} {
		if got := IsNewElement(line, "strict"); got != want {
			t.Errorf("IsNewElement(%q) = %v, want %v", line, got, want)
		}
	}
}
