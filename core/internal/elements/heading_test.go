// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func TestHeadingParser_CanParse(t *testing.T) {
	tests := []struct {
		name string
		line string
		mode string
		want bool
	}{
		{"level 3 flex", "### Título", "flex", true},
		{"level 6 flex", "###### Título", "flex", true},
		{"level 2 no matchea (piso es 3)", "## Título", "flex", false},
		{"level 1 no matchea", "# Título", "flex", false},
		{"level 7 no matchea", "####### Título", "flex", false},
		{"strict no matchea nunca", "### Título", "strict", false},
		{"sin espacio no es heading", "###Título", "flex", false},
		{"solo hashes sin texto no es heading", "###", "flex", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &HeadingParser{}
			if got := p.CanParse(tt.line, tt.mode); got != tt.want {
				t.Errorf("CanParse(%q, %q) = %v, want %v", tt.line, tt.mode, got, tt.want)
			}
		})
	}
}

// TestHeadingParser_Parse_NilHeadingAnchor_FallsBackToPlainDerive cubre el
// caso de un caller (DocumentFlexParser) que NO fija ctx.HeadingAnchor: sin
// dedup por diseño (C31 — ningún dialecto de documento deduplica anchors
// hoy, ni siquiera a nivel top; agregarlo es una decisión de producto
// aparte). El resultado tiene que ser IDÉNTICO a como DocumentFlexParser ya
// deriva sus propios anchors de nivel top hoy: elements.DeriveAnchor plano.
func TestHeadingParser_Parse_NilHeadingAnchor_FallsBackToPlainDerive(t *testing.T) {
	p := &HeadingParser{}
	ctx := &ParseContext{Mode: "flex", Lines: []string{"### Mi Titulo"}}

	result := p.Parse(ctx, 0)
	el, ok := result.Element.(*ast.TextElement)
	if !ok || !el.IsRawHTML {
		t.Fatalf("Element no es un heading RawHTML: %T", result.Element)
	}
	if !strings.Contains(el.Content, `id="mi-titulo"`) {
		t.Errorf("Content = %q, quería id=\"mi-titulo\" (DeriveAnchor plano, sin prefijo \"heading-\")", el.Content)
	}
}

// TestHeadingParser_Parse_UsesHeadingAnchorWhenSet confirma que, cuando el
// caller SÍ fija ctx.HeadingAnchor (FlexParser), el resultado pasa por él —
// no por DeriveAnchor directo.
func TestHeadingParser_Parse_UsesHeadingAnchorWhenSet(t *testing.T) {
	p := &HeadingParser{}
	ctx := &ParseContext{
		Mode:  "flex",
		Lines: []string{"### Mi Titulo"},
		HeadingAnchor: func(text string) string {
			return "custom-" + text
		},
	}

	result := p.Parse(ctx, 0)
	el, ok := result.Element.(*ast.TextElement)
	if !ok || !el.IsRawHTML {
		t.Fatalf("Element no es un heading RawHTML: %T", result.Element)
	}
	if !strings.Contains(el.Content, `id="custom-mi-titulo"`) {
		t.Errorf("Content = %q, quería el id derivado de ctx.HeadingAnchor", el.Content)
	}
	if result.ConsumedLines != 1 {
		t.Errorf("ConsumedLines = %d, want 1", result.ConsumedLines)
	}
}
