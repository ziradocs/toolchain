// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import "testing"

// parseMarkdownImage cortaba la fuente en el primer ")": "foto_(1).png"
// llegaba como "foto_(1" y "javascript:alert(1)" como "javascript:alert(1".
// Ahora usa el mismo escáner balanceado que las imágenes inline.
func TestParseMarkdownImage_BalancedDestination(t *testing.T) {
	p := &ImageParser{}
	for _, tc := range []struct {
		line, source, fit string
	}{
		{"![Foto](img/foto_(1).png)", "img/foto_(1).png", ""},
		{"![Img](javascript:alert(1))", "javascript:alert(1)", ""},
		{"![Foto](img/foto_(1).png){fit=cover}", "img/foto_(1).png", "cover"},
		{"![Foto](img/foto_(1.png", "", ""},
	} {
		source, _, _, fit, _, _, _ := p.parseMarkdownImage(tc.line)
		if source != tc.source || fit != tc.fit {
			t.Errorf("parseMarkdownImage(%q) = source %q fit %q, want %q %q", tc.line, source, fit, tc.source, tc.fit)
		}
	}
}

// El destino de una imagen de bloque sigue CommonMark: el título va aparte,
// <...> se quita y una línea que no es exactamente una imagen (dos imágenes,
// texto después, una palabra suelta en el destino) no es imagen de bloque.
func TestParseMarkdownImage_TitleAndBrackets(t *testing.T) {
	p := &ImageParser{}
	for _, tc := range []struct {
		line, source, title, fit string
		ok                       bool
	}{
		{`![a](img/a.png "t")`, "img/a.png", "t", "", true},
		{`![a](img/a.png 't')`, "img/a.png", "t", "", true},
		{`![a](img/a.png (t))`, "img/a.png", "t", "", true},
		{`![a](<mi dir/a.png>)`, "mi dir/a.png", "", "", true},
		{`![a](<mi dir/a.png> "t"){fit=cover}`, "mi dir/a.png", "t", "cover", true},
		{`![a](mi%20dir/a.png)`, "mi%20dir/a.png", "", "", true},
		{`![a](t.png bare)`, "t.png bare", "", "", true},
		{`![a](a.png) ![b](b.png)`, "", "", "", false},
		{`![a](a.png) y texto`, "", "", "", false},
	} {
		source, _, title, fit, _, _, ok := p.parseMarkdownImage(tc.line)
		if source != tc.source || title != tc.title || fit != tc.fit || ok != tc.ok {
			t.Errorf("parseMarkdownImage(%q) = %q %q %q %v; want %q %q %q %v", tc.line, source, title, fit, ok, tc.source, tc.title, tc.fit, tc.ok)
		}
		if p.CanParse(tc.line, "flex") != tc.ok {
			t.Errorf("CanParse(%q) = %v", tc.line, !tc.ok)
		}
	}
}
