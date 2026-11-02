// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"
)

// TestDOCXGenerator_TOC_SingleLocalizedTitle: un solo título, en el idioma
// del documento, y las entradas dentro del resultado del campo TOC (que
// sigue sucio para que Word lo reconstruya), no en párrafos sueltos que
// quedarían duplicados al actualizarlo.
func TestDOCXGenerator_TOC_SingleLocalizedTitle(t *testing.T) {
	for lang, want := range map[string]string{"": "Tabla de Contenidos", "es-MX": "Tabla de Contenidos", "en": "Table of Contents", "pt-BR": "Sumário", "xx": "Tabla de Contenidos"} {
		doc := astWithTwoSections()
		doc.FrontMatter.Lang = lang
		xml := buildDOCXXML(t, doc, GeneratorOptions{TOC: true})

		if n := strings.Count(xml, "<w:t>"+want+"</w:t>"); n != 1 {
			t.Errorf("lang %q: want title %q once, got %d", lang, want, n)
		}
		if want != "Table of Contents" && strings.Contains(xml, "Table of Contents") {
			t.Errorf("lang %q: English TOC label still present", lang)
		}
		sep := strings.Index(xml, `w:fldCharType="separate"`)
		end := strings.Index(xml, `w:fldCharType="end"`)
		entry := strings.Index(xml, "<w:t>Introduction</w:t>")
		if !(sep < entry && entry < end) {
			t.Errorf("lang %q: TOC entries are not the field result (sep %d, entry %d, end %d)", lang, sep, entry, end)
		}
		if !strings.Contains(xml, `w:dirty="true"`) {
			t.Errorf("lang %q: TOC field is not dirty, Word won't rebuild it", lang)
		}
	}
}
