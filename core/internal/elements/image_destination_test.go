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
		source, _, fit, _, _ := p.parseMarkdownImage(tc.line)
		if source != tc.source || fit != tc.fit {
			t.Errorf("parseMarkdownImage(%q) = source %q fit %q, want %q %q", tc.line, source, fit, tc.source, tc.fit)
		}
	}
}
