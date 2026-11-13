// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "testing"

// enumerateInline llama f con cada cadena no vacía de hasta max piezas de
// alphabet.
func enumerateInline(alphabet []string, max int, f func(string)) {
	var rec func(prefix string, n int)
	rec = func(prefix string, n int) {
		if prefix != "" {
			f(prefix)
		}
		if n == max {
			return
		}
		for _, a := range alphabet {
			rec(prefix+a, n+1)
		}
	}
	rec("", 0)
}

// Ninguna entrada produce etiquetas de formato cruzadas: el HTML de una línea
// y el del cuerpo están siempre bien anidados. Enumera todas las cadenas de
// hasta 11 piezas de {*, a, espacio} (la familia de `***X* Y**`, que esta
// regla rompió y se arregló) y de hasta 6 de {*, =, ~, a, espacio, \}.
func TestInlineFormats_OutputIsAlwaysWellNested(t *testing.T) {
	if testing.Short() {
		t.Skip("enumeración larga")
	}
	bad := 0
	check := func(s string) {
		for _, html := range []string{ProcessInlineMarkdownSecureLine(s), ProcessTextWithVariablesAndMarkdownSecure(s, nil)} {
			if !htmlTagsWellNested(html) {
				bad++
				if bad <= 20 {
					t.Errorf("%q: badly nested %q", s, html)
				}
				return
			}
		}
	}
	enumerateInline([]string{"*", "a", " "}, 11, check)
	enumerateInline([]string{"*", "=", "~", "a", " ", `\`}, 6, check)
	if bad > 0 {
		t.Errorf("%d inputs produce badly nested HTML", bad)
	}
}
