// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"time"
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

// pptxMaskLinkDestinations recorre el texto una vez: con muchos enlaces e imágenes en un
// párrafo, el doble de texto tarda el doble, no el cuádruple.
func TestPptxMaskLinkDestinationsScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	gen := map[string]func(n int) string{
		"enlaces e imágenes": func(n int) string { return strings.Repeat(`*a* ![i](x*y.png "t") [b](u*v) `, n) },
		"con espacios":       func(n int) string { return strings.Repeat("[a](mi página.html) ![b](mi foto.png) ", n) },
		"sin cierre":         func(n int) string { return strings.Repeat("[a](b (c ![d](e ", n) },
	}
	best := func(s string) time.Duration {
		b := time.Duration(1 << 62)
		for i := 0; i < 5; i++ {
			start := time.Now()
			pptxMaskLinkDestinations(s)
			if d := time.Since(start); d < b {
				b = d
			}
		}
		return b
	}
	const n = 4000
	for name, g := range gen {
		t1, t2 := best(g(n)), best(g(2*n))
		if t1 > 2*time.Millisecond && t2 > 7*t1/2 {
			t.Errorf("%s: %v con n=%d y %v con 2n (más que lineal)", name, t1, n, t2)
		}
	}
}
