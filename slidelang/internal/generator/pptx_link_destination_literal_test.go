// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"reflect"
	"testing"
)

// El PPTX deja los enlaces y las imágenes dentro de un texto como su
// Markdown, pero el destino y el título son literales, como en el HTML: un
// "*" ahí adentro no es cursiva. Antes `![a](img/a*b*.png)` salía como
// "![a](img/a", "b" en cursiva y ".png)", y un "*" del destino cerraba una
// cursiva abierta antes del enlace.
func TestPptxSplitInline_DestinationsAreLiteral(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []pptxInlineSegment
	}{
		{"imagen", "Texto ![a](img/a*b*.png) fin", []pptxInlineSegment{
			{text: "Texto ![a](img/a*b*.png) fin"},
		}},
		{"título de imagen", `Gato ![g](img/g.png "un *gran* gato") fin`, []pptxInlineSegment{
			{text: `Gato ![g](img/g.png "un *gran* gato") fin`},
		}},
		{"enlace con título y cursiva después", `[x](https://e.com/a*b*c "t *y* z") y *c*`, []pptxInlineSegment{
			{text: `[x](https://e.com/a*b*c "t *y* z") y `}, {text: "c", italic: true},
		}},
		{"énfasis que abre antes del enlace", "*a ![i](x*y.png) b*", []pptxInlineSegment{
			{text: "a ![i](x*y.png) b", italic: true},
		}},
		{"negrita sin cierre fuera del destino", "**t [x](u**v)", []pptxInlineSegment{
			{text: "**t [x](u**v)"},
		}},
		{"la etiqueta sí lleva formato", "[**n**](a*b.html)", []pptxInlineSegment{
			{text: "["}, {text: "n", bold: true}, {text: "](a*b.html)"},
		}},
		{"code span no abre un enlace", "L4 `f[0](` y *importante* texto) fin", []pptxInlineSegment{
			{text: "L4 "}, {text: "f[0](", code: true}, {text: " y "}, {text: "importante", italic: true}, {text: " texto) fin"},
		}},
		{"guion bajo literal", `[y](https://e.com/a_b_c "t _y_ z") _z_`, []pptxInlineSegment{
			{text: `[y](https://e.com/a_b_c "t _y_ z") _z_`},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pptxSplitInline(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("pptxSplitInline(%q)\n got %+v\nwant %+v", tt.in, got, tt.want)
			}
		})
	}
}
