// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import "testing"

// El texto plano del encabezado es lo que el <h1> muestra, sin etiquetas: las
// entidades del HTML se deshacen una sola vez. Una entidad que el autor
// escribió como texto (`&lt;br&gt;`) se ve así en el <h1> y así tiene que
// quedar en data-slide-title y en el índice del JSON embebido.
func TestSlideHeaderLine_PlainTextDecodesOnce(t *testing.T) {
	cases := []struct{ src, want string }{
		{"Usa &lt;br&gt; y **más**", "Usa &lt;br&gt; y más"},
		{"Literal &amp;lt; y &amp;quot; y &amp;#39;", "Literal &amp;lt; y &amp;quot; y &amp;#39;"},
		{`A & B < C "D" 'E'`, `A & B < C "D" 'E'`},
		{"Comillas &quot;x&quot; y &#39;y&#39;", "Comillas &quot;x&quot; y &#39;y&#39;"},
	}
	for _, tc := range cases {
		if got, _ := slideHeaderLine(tc.src, nil); got != tc.want {
			t.Errorf("%q: plain %q, want %q", tc.src, got, tc.want)
		}
	}
}
