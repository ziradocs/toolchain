// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import "testing"

func TestSanitizeBookmarkID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "space becomes underscore", input: "my chart", expected: "my_chart"},
		{name: "path separators stripped", input: "a/b\\c", expected: "abc"},
		{name: "path traversal stripped", input: "../../etc/passwd", expected: "etcpasswd"},
		{name: "dots commas colons stripped", input: "v1.2,3:4;5(6)", expected: "v123456"},
		{name: "already clean value unchanged", input: "bar-chart_2024", expected: "bar-chart_2024"},
		{name: "absolute path stripped to bare token", input: "/etc/hostname", expected: "etchostname"},
		// Transliteración de acentos (#112, #116): antes, el strip
		// whitelist-only borraba la vocal acentuada entera en vez de
		// dejar su equivalente ASCII, produciendo IDs irreconocibles
		// (p.ej. "Sección" → "Seccin", no "Seccion"). Ahora se
		// transliteran primero (NFD + drop de marcas Mn) para que el
		// resultado sea el ASCII intuitivo del título original.
		{name: "accented o transliterated - Sección", input: "Sección", expected: "Seccion"},
		{name: "accented o transliterated - Publicación", input: "Publicación", expected: "Publicacion"},
		{name: "ñ transliterated to n", input: "Año Nuevo", expected: "Ano_Nuevo"},
		{name: "ü transliterated to u", input: "Pingüino", expected: "Pinguino"},
		{name: "uppercase accents transliterated", input: "ÁÉÍÓÚÑÜ", expected: "AEIOUNU"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeBookmarkID(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeBookmarkID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
			for _, sep := range []rune{'/', '\\'} {
				for _, r := range result {
					if r == sep {
						t.Errorf("sanitizeBookmarkID(%q) = %q still contains path separator %q", tt.input, result, sep)
					}
				}
			}
		})
	}
}

// TestSanitizeBookmarkID_AccentCollisions documenta el comportamiento de
// colisión post-fix (#112, #116). Dos headings que difieren solo por
// contenido no acentuado siguen produciendo IDs distintos y legibles (antes
// del fix, borrar la vocal acentuada podía producir IDs irreconocibles que
// coincidían por accidente con los de otro heading no relacionado). El fix
// NO elimina toda posibilidad de colisión — un documento que tenga tanto la
// versión acentuada como la no acentuada de la MISMA palabra ("Sección" y
// "Seccion") seguirá colisionando, pero de forma predecible por contenido,
// no por un accidente del orden de strip de caracteres.
func TestSanitizeBookmarkID_AccentCollisions(t *testing.T) {
	t.Run("distinct headings that only differ by accent placement stay distinguishable from other content", func(t *testing.T) {
		got := sanitizeBookmarkID("Sección Especial")
		other := sanitizeBookmarkID("Sección General")
		if got == other {
			t.Errorf("expected distinct bookmark IDs for distinct headings, got %q for both", got)
		}
		if got != "Seccion_Especial" {
			t.Errorf("sanitizeBookmarkID(%q) = %q, want %q (transliterated, not mangled)", "Sección Especial", got, "Seccion_Especial")
		}
	})

	t.Run("accented and unaccented spelling of the same word still collide by design", func(t *testing.T) {
		accented := sanitizeBookmarkID("Múltiples secciones")
		unaccented := sanitizeBookmarkID("Multiples secciones")
		if accented != unaccented {
			t.Errorf("expected accented/unaccented spellings of the same word to collide (documented limitation), got %q vs %q", accented, unaccented)
		}
	})
}
