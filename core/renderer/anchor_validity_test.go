// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "testing"

// Un anchor que empieza por dígito o guion no es usable: html-validate lo
// rechaza (`valid-id`) y, sobre todo, no es direccionable por CSS —
// `#1-primer-paso` no es un selector válido.
func TestDeriveAnchor_AlwaysStartsWithLetter(t *testing.T) {
	for _, tc := range []struct {
		text string
		want string
	}{
		{"1. Primer paso", "h-1-primer-paso"},
		{"2024 en cifras", "h-2024-en-cifras"},
		{"-guion inicial", "h--guion-inicial"},
		{"_guion bajo", "h-_guion-bajo"},
		{"Título normal", "ttulo-normal"},
		{"already-fine", "already-fine"},
	} {
		if got := DeriveAnchor(tc.text); got != tc.want {
			t.Errorf("DeriveAnchor(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

// Idempotencia: la salida vuelve a pasar por SanitizeAnchor en varias rutas
// (el `explicitID` de buildHeadingElement, entre otras), así que no puede
// moverse en la segunda pasada.
func TestSanitizeAnchor_Idempotent(t *testing.T) {
	for _, s := range []string{
		"h-1-primer-paso", "already-fine", "details-2", "h", "h--guion-inicial",
	} {
		if got := SanitizeAnchor(s); got != s {
			t.Errorf("SanitizeAnchor(%q) = %q — no es idempotente", s, got)
		}
	}
}

// SanitizeAnchor deja pasar el vacío a propósito: es la señal que
// DocumentStrictParser usa para reportar un `id:` declarado sin caracteres
// utilizables. El fallback solo corresponde en la derivación, donde no hay
// id que corregir.
func TestSanitizeAnchor_KeepsEmptySoDeclaredIDsCanBeReported(t *testing.T) {
	if got := SanitizeAnchor("🎉"); got != "" {
		t.Errorf("SanitizeAnchor(%q) = %q, se esperaba vacío", "🎉", got)
	}
	if AnchorFallback == "" {
		t.Error("AnchorFallback no puede ser vacío: es lo que reemplaza al anchor vacío")
	}
}
