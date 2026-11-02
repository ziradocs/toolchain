// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// TestParseDocument_MathFence: un ```math de un Markdown simple es una
// ecuación, no un bloque de código con el LaTeX crudo (el formatter lo
// canoniza a <<math>>, ver formatter/math_fence_test.go).
func TestParseDocument_MathFence(t *testing.T) {
	src := "# Informe\n\n```math\n\\int_0^1 x^2 dx = \\frac{1}{3}\n```\n\n```latex\n\\LaTeX\n```\n"
	doc := parsePlainMarkdown(t, src)

	var maths []*ast.MathElement
	var codes []*ast.CodeElement
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			switch e := el.(type) {
			case *ast.MathElement:
				maths = append(maths, e)
			case *ast.CodeElement:
				codes = append(codes, e)
			}
		}
	}
	if len(maths) != 1 || maths[0].Content != `\int_0^1 x^2 dx = \frac{1}{3}` {
		t.Fatalf("want one math element with the fence body, got %+v", maths)
	}
	if len(codes) != 1 || codes[0].Language != "latex" {
		t.Fatalf("```latex must stay a code block, got %+v", codes)
	}

}

// TestParseDocument_UnclosedMathFence: un ```math sin cerrar no se come las
// secciones que siguen.
func TestParseDocument_UnclosedMathFence(t *testing.T) {
	doc := parsePlainMarkdown(t, "# Uno\n\n```math\nx^2\n\n# Dos\n\nTexto final.\n")
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if _, ok := el.(*ast.MathElement); ok {
				t.Fatalf("unclosed fence became an equation: %+v", el)
			}
		}
	}
	if !astContains(t, doc, "x^2") {
		t.Error("the fence body was lost")
	}
}

// TestParseDocument_TildeMathFence: ~~~math también es una ecuación.
func TestParseDocument_TildeMathFence(t *testing.T) {
	doc := parsePlainMarkdown(t, "# Uno\n\n~~~math\nx^2\n~~~\n\nTexto.\n")
	found := false
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if m, ok := el.(*ast.MathElement); ok && m.Content == "x^2" {
				found = true
			}
		}
	}
	if !found || !astContains(t, doc, "Texto.") {
		t.Errorf("~~~math was not parsed as an equation followed by the text")
	}
}
