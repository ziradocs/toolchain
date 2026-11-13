// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "testing"

// UnescapeHTML deshace EscapeHTML en una sola pasada: una entidad que el
// autor escribió como texto (`&lt;br&gt;`, escapada como `&amp;lt;br&amp;gt;`)
// vuelve a ser ese texto y no se decodifica una segunda vez.
func TestUnescapeHTML_InvertsEscapeHTML(t *testing.T) {
	for _, s := range []string{
		"Usa &lt;br&gt; y &amp;",
		`Comillas &quot;x&quot; y &#39;y&#39;`,
		`A & B < C > D "E" 'F'`,
		"&amp;amp; &amp;lt;",
		"&#8217; &nbsp; sin tocar",
	} {
		if got := UnescapeHTML(EscapeHTML(s)); got != s {
			t.Errorf("UnescapeHTML(EscapeHTML(%q)) = %q", s, got)
		}
	}
}
