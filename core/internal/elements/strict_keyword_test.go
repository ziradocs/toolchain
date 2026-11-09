// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import "testing"

// Issue #489: la palabra clave tiene que ser una palabra completa.
func TestHasStrictKeyword(t *testing.T) {
	for _, tc := range []struct {
		line string
		want bool
	}{
		{"TEXT", true},
		{"TEXT hola", true},
		{"TEXT\thola", true},
		{"TEXT hola", true},
		{"TEXT:", true},
		{"TEXT-x", true},
		{"TEXTO citado", false},
		{"TEXTOS", false},
		{"TEXT9", false},
		{"TEXT_X", false},
		{"TEXTÓ", false},
		{"TEXTé", false},
		{"TEX", false},
		{"text", false},
		{"", false},
	} {
		if got := HasStrictKeyword(tc.line, "TEXT"); got != tc.want {
			t.Errorf("HasStrictKeyword(%q, TEXT) = %v, want %v", tc.line, got, tc.want)
		}
	}
	if !HasStrictKeyword("CODE{verbatim} go", "CODE") {
		t.Error(`HasStrictKeyword("CODE{verbatim} go", CODE) = false, want true`)
	}
}

// Una línea del cuerpo de una cita que empieza con las letras de una palabra
// clave no corta la cita.
func TestIsNewElement_StrictKeywordNeedsWordBoundary(t *testing.T) {
	for line, want := range map[string]bool{
		"TEXTO citado":  false,
		"MAPA norte":    false,
		"TEXT":          true,
		"MAP":           true,
		"SECTION \"B\"": true,
		"SECTIONS":      false,
	} {
		if got := IsNewElement(line, "strict"); got != want {
			t.Errorf("IsNewElement(%q) = %v, want %v", line, got, want)
		}
	}
}
