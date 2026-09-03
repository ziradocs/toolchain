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
			name: "heading con formato inline pierde las tags",
			el:   ast.NewRawHTMLTextElement(pos, `<h3 id="x"><strong>Foo</strong> bar</h3>`),
			want: "### Foo bar",
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
