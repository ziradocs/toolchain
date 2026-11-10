// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "testing"

// Una barra invertida antes de *, = o ~ hace literal ese carácter: no
// abre ni cierra formato y la barra no se ve. Dentro de un code span y en el
// destino de un enlace la barra se queda como está. `\\` es una barra
// escrita a mano: se deja tal cual y no escapa al carácter que la sigue.
func TestInlineFormats_BackslashEscapes(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"escaped italic", `\*escapado\*`, `*escapado*`},
		{"escaped bold", `\*\*no\*\*`, `**no**`},
		{"one escaped side", `\*a*`, `*a*`},
		{"underscore is not a delimiter", `snake\_case y C:\Users\_tmp`, `snake\_case y C:\Users\_tmp`},
		{"escaped highlight", `\=\=x\=\=`, `==x==`},
		{"escaped strike", `\~\~x\~\~`, `~~x~~`},
		{"escape next to real format", `\* nota y **real**`, `* nota y <strong>real</strong>`},
		{"double backslash is not an escape", `\\*x*`, `\\<em>x</em>`},
		{"other backslashes untouched", `C:\Users\temp y \n`, `C:\Users\temp y \n`},
		{"code span keeps backslash", "`\\*` y \\*", `<code>\*</code> y *`},
		{"link text", `[a\*b](https://e.com)`, `<a href="https://e.com">a*b</a>`},
		{"link destination untouched", `[a](https://e.com/x\*y)`, `<a href="https://e.com/x\*y">a</a>`},
		{"image alt", `![a\*b](x.png)`, `<img src="x.png" alt="a*b">`},
		{"html stays escaped", `\*<b>\*`, `*&lt;b&gt;*`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProcessInlineMarkdownSecureLine(tc.src); got != tc.want {
				t.Errorf("line: %q, want %q", got, tc.want)
			}
			if got := ProcessTextWithVariablesAndMarkdownSecure(tc.src, nil); got != tc.want {
				t.Errorf("body: %q, want %q", got, tc.want)
			}
		})
	}
}

// El título de un slide sigue la misma regla.
func TestBlockTitleHTML_BackslashEscapes(t *testing.T) {
	if got := BlockTitleHTML(`Usa \*args y \*\*kwargs`, nil); got != `Usa *args y **kwargs` {
		t.Errorf("BlockTitleHTML = %q", got)
	}
}
