// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "testing"

// El destino y el título de un enlace o una imagen son literales, como en
// CommonMark: el resaltado, el tachado, la negrita y la cursiva no corren
// adentro. Esas pasadas iban antes que las de imagen y enlace, así que
// `img/a*b*.png` llegaba al src como `img/a<em>b</em>.png` y el título
// "un *gran* gato" salía "un gran gato".
func TestInlineDestinationsAreLiteral(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`Texto ![a](img/a*b*.png) fin`, `Texto <img src="img/a*b*.png" alt="a"> fin`},
		{`![g](img/g.png "un *gran* gato")`, `<img src="img/g.png" alt="g" title="un *gran* gato">`},
		{`[x](https://e.com/a*b*c "t *y* z") y *c*`, `<a href="https://e.com/a*b*c" title="t *y* z">x</a> y <em>c</em>`},
		{`[x](https://e.com/a**b**c) y **n**`, `<a href="https://e.com/a**b**c">x</a> y <strong>n</strong>`},
		{`[x](https://e.com/?q==b==) ==m==`, `<a href="https://e.com/?q==b==">x</a> <mark>m</mark>`},
		{`[x](a~~b~~c.html "~~t~~")`, `<a href="a~~b~~c.html" title="~~t~~">x</a>`},
		{`![a](<mi *dir*/a.png> '*t*')`, `<img src="mi%20*dir*/a.png" alt="a" title="*t*">`},
		{`![a](a*b.png (c*d))`, `<img src="a*b.png" alt="a" title="c*d">`},
		// "_" no es énfasis en este renderer: queda literal en todos lados.
		{`![g](img/a_b_.png "un _gran_ gato") y _x_`, `<img src="img/a_b_.png" alt="g" title="un _gran_ gato"> y _x_`},
		{`[y](https://e.com/a_b_c "t _y_ z")`, `<a href="https://e.com/a_b_c" title="t _y_ z">y</a>`},
		// Un énfasis que abre antes del enlace no cierra con un "*" del
		// destino: cierra con el que sigue, o queda literal.
		{`*a ![i](img/a*b*.png) b*`, `<em>a <img src="img/a*b*.png" alt="i"> b</em>`},
		{`**t [x](u**v)`, `**t <a href="u**v">x</a>`},
		{`*t [x](u*v)`, `*t <a href="u*v">x</a>`},
		// La etiqueta sí lleva formato.
		{`[**n** y *c*](a*b.html)`, `<a href="a*b.html"><strong>n</strong> y <em>c</em></a>`},
		{`[![a](x*y.png "*t*")](https://e.com/*z* "*u*")`, `<a href="https://e.com/*z*" title="*u*"><img src="x*y.png" alt="a" title="*t*"></a>`},
		// Un code span dentro del destino sigue resolviéndose antes (y su
		// <code> se valida como parte del href, ver inlineDestinationText).
		{"[a](`*x*`)", `<a href="&lt;code&gt;*x*&lt;/code&gt;">a</a>`},
	} {
		if got := ProcessInlineMarkdownSecure(tc.in); got != tc.want {
			t.Errorf("ProcessInlineMarkdownSecure(%q)\n got %s\nwant %s", tc.in, got, tc.want)
		}
	}
}

// El linter ve el mismo destino que el HTML.
func TestFindInlineLinksDestinationIsLiteral(t *testing.T) {
	links := FindInlineLinks(`Texto ![a](img/a*b*.png) y [x](https://e.com/a*b*c "t *y* z")`)
	if len(links) != 2 || links[0].Destination != "img/a*b*.png" || links[1].Destination != "https://e.com/a*b*c" {
		t.Fatalf("FindInlineLinks = %+v", links)
	}
}
