// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func TestMathParser_CanParse(t *testing.T) {
	parser := &MathParser{}

	tests := []struct {
		name     string
		line     string
		mode     string
		expected bool
	}{
		{"strict <<math>>", "<<math>>", "strict", true},
		{"flex <<math>>", "<<math>>", "flex", true},
		{"flex $$", "$$", "flex", true},
		{"flex $$ inline start", "$$E=mc^2$$", "flex", true},
		{"strict $$ not recognized", "$$", "strict", false},
		{"plain text", "just some text", "flex", false},
		{"$$ not at line start", "the value is $$x^2$$ here", "flex", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parser.CanParse(tt.line, tt.mode); got != tt.expected {
				t.Errorf("CanParse(%q, %q) = %v, want %v", tt.line, tt.mode, got, tt.expected)
			}
		})
	}
}

func TestMathParser_ParseStrictBlock(t *testing.T) {
	parser := &MathParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"<<math>>",
			"  E = mc^2",
			"<<end>>",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	math, ok := result.Element.(*ast.MathElement)
	if !ok {
		t.Fatalf("Element type = %T, want *ast.MathElement", result.Element)
	}
	if math.Content != "E = mc^2" {
		t.Errorf("Content = %q, want %q", math.Content, "E = mc^2")
	}
	if math.Label != "" {
		t.Errorf("Label = %q, want empty (no label: line given)", math.Label)
	}
	if result.ConsumedLines != 3 {
		t.Errorf("ConsumedLines = %d, want 3", result.ConsumedLines)
	}
}

func TestMathParser_ParseStrictBlockWithLabel(t *testing.T) {
	parser := &MathParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"<<math>>",
			"  E = mc^2",
			`  label: "eq:einstein"`,
			"<<end>>",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	math := result.Element.(*ast.MathElement)
	if math.Content != "E = mc^2" {
		t.Errorf("Content = %q, want %q", math.Content, "E = mc^2")
	}
	if math.Label != "eq:einstein" {
		t.Errorf("Label = %q, want %q", math.Label, "eq:einstein")
	}
}

func TestMathParser_ParseMultilineDollarBlock(t *testing.T) {
	parser := &MathParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"$$",
			"x^2 + y^2 = z^2",
			"$$",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	math := result.Element.(*ast.MathElement)
	if math.Content != "x^2 + y^2 = z^2" {
		t.Errorf("Content = %q, want %q", math.Content, "x^2 + y^2 = z^2")
	}
	if math.Label != "" {
		t.Errorf("Label = %q, want empty (dollar form no lleva metadata)", math.Label)
	}
	if result.ConsumedLines != 3 {
		t.Errorf("ConsumedLines = %d, want 3", result.ConsumedLines)
	}
}

func TestMathParser_ParseSingleLineDollarBlock(t *testing.T) {
	parser := &MathParser{}
	ctx := &ParseContext{
		Mode:  "flex",
		Lines: []string{"$$E = mc^2$$"},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	math := result.Element.(*ast.MathElement)
	if math.Content != "E = mc^2" {
		t.Errorf("Content = %q, want %q", math.Content, "E = mc^2")
	}
	if result.ConsumedLines != 1 {
		t.Errorf("ConsumedLines = %d, want 1 (cierre en la misma línea)", result.ConsumedLines)
	}
}

// TestMathParser_ParseUnindentedContent bloquea la regresión encontrada vía
// smoke-test E2E: contenido de doclang (modo flex) va a columna 0, sin
// indentación relativa a <<math>> — a diferencia del contexto SLIDE
// indentado de slidelang strict. Antes del fix, la detección de indentación
// heredada de mermaid.go cerraba el bloque en la primera línea sin indentar,
// dejando Content vacío.
func TestMathParser_ParseUnindentedContent(t *testing.T) {
	parser := &MathParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"<<math>>",
			"E = mc^2",
			`label: "eq:einstein"`,
			"<<end>>",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	math, ok := result.Element.(*ast.MathElement)
	if !ok {
		t.Fatalf("Element type = %T, want *ast.MathElement", result.Element)
	}
	if math.Content != "E = mc^2" {
		t.Errorf("Content = %q, want %q (contenido sin indentar debe consumirse igual)", math.Content, "E = mc^2")
	}
	if math.Label != "eq:einstein" {
		t.Errorf("Label = %q, want %q", math.Label, "eq:einstein")
	}
	if result.ConsumedLines != 4 {
		t.Errorf("ConsumedLines = %d, want 4", result.ConsumedLines)
	}
}

func TestMathParser_StopsAtSlideBoundary(t *testing.T) {
	parser := &MathParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"<<math>>",
			"  E = mc^2",
			"---",
			"SLIDE content",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	math := result.Element.(*ast.MathElement)
	if math.Content != "E = mc^2" {
		t.Errorf("Content = %q, want %q (no debe cruzar el separador de slide)", math.Content, "E = mc^2")
	}
}
