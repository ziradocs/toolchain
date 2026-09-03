// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Issue #194: PPTX recibe una línea de texto, no HTML. Un encabezado de
// subsección llega con el <hN> en Content, así que hay que des-renderizarlo
// — si no, la diapositiva mostraría el markup literal. Se reconstruye la
// línea Markdown, que es lo que este generador recibía antes de #194.
func TestPPTXTextContent(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	for _, tc := range []struct {
		name string
		el   *ast.TextElement
		want string
	}{
		{
			name: "texto normal pasa tal cual",
			el:   ast.NewTextElement(pos, "Un párrafo."),
			want: "Un párrafo.",
		},
		{
			name: "heading nivel 3",
			el:   ast.NewRawHTMLTextElement(pos, `<h3 id="foo">Foo</h3>`),
			want: "### Foo",
		},
		{
			name: "heading nivel 6",
			el:   ast.NewRawHTMLTextElement(pos, `<h6 id="x">Seis</h6>`),
			want: "###### Seis",
		},
		{
			name: "negrita vuelve a su Markdown",
			el:   ast.NewRawHTMLTextElement(pos, `<h3 id="x"><strong>Important</strong> result`+"</h3>"),
			want: "### **Important** result",
		},
		{
			name: "cursiva",
			el:   ast.NewRawHTMLTextElement(pos, `<h3 id="x">un <em>detalle</em></h3>`),
			want: "### un *detalle*",
		},
		{
			name: "código inline",
			el:   ast.NewRawHTMLTextElement(pos, `<h3 id="x">usa <code>go build</code></h3>`),
			want: "### usa `go build`",
		},
		{
			name: "tachado y resaltado",
			el:   ast.NewRawHTMLTextElement(pos, `<h3 id="x"><del>viejo</del> y <mark>nuevo</mark></h3>`),
			want: "### ~~viejo~~ y ==nuevo==",
		},
		{
			name: "enlace",
			el:   ast.NewRawHTMLTextElement(pos, `<h3 id="x">ver <a href="https://x.com">esto</a></h3>`),
			want: "### ver [esto](https://x.com)",
		},
		{
			name: "span de idioma vuelve a su sintaxis",
			el:   ast.NewRawHTMLTextElement(pos, `<h3 id="x">dice <span lang="fr">bonjour</span></h3>`),
			want: "### dice [bonjour]{lang=fr}",
		},
		{
			name: "una tag tecleada por el autor sigue escapada y no se reinterpreta",
			el:   ast.NewRawHTMLTextElement(pos, `<h3 id="x">usar &lt;strong&gt; a mano</h3>`),
			want: "### usar <strong> a mano",
		},
		{
			name: "entidades HTML se decodifican",
			el:   ast.NewRawHTMLTextElement(pos, `<h4 id="x">A &amp; B</h4>`),
			want: "#### A & B",
		},
		{
			name: "RawHTML que no es heading sale sin markup",
			el:   ast.NewRawHTMLTextElement(pos, `<div>Suelto</div>`),
			want: "Suelto",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pptxTextContent(tc.el); got != tc.want {
				t.Errorf("pptxTextContent() = %q, se esperaba %q", got, tc.want)
			}
		})
	}
}
