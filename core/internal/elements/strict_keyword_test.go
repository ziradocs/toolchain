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

// EndsStrictBody: una línea más sangrada que la cabecera corta el cuerpo
// solo con la palabra clave completa; a la sangría de la cabecera, o menos,
// corta por prefijo como siempre (posición de elemento). IsNewElement sigue
// por prefijo.
func TestEndsStrictBody(t *testing.T) {
	for _, tc := range []struct {
		line string
		want bool
	}{
		{"    TEXTO citado", false},
		{"    MAPA norte", false},
		{"    CODEX python", false},
		{"    TEXT hola", true},
		{"    MAP", true},
		{"    SECTION \"B\"", true},
		{"    SECTIONS", false},
		{"    @include x", true},
		{"  TEXTO", true},
		{"  CODEX python", true},
		{"TEXTOS", true},
		{"  texto", false},
	} {
		if got := EndsStrictBody(tc.line, 2); got != tc.want {
			t.Errorf("EndsStrictBody(%q, 2) = %v, want %v", tc.line, got, tc.want)
		}
	}
	if !IsNewElement("TEXTO citado", "strict") {
		t.Error("IsNewElement keeps main's prefix rule")
	}
}
