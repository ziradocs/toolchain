// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func astWithOneElement(el ast.Element) *ast.AST {
	pos := diagnostics.Position{Line: 1, Column: 1}
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, el)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

func TestHasElementHelpers_OnlyMatchTheirOwnType(t *testing.T) {
	mermaid := astWithOneElement(ast.NewMermaidElement(diagnostics.Position{Line: 1}, "flowchart", "graph TD; A-->B"))
	chart := astWithOneElement(ast.NewChartElement(diagnostics.Position{Line: 1}, "bar"))
	mapDoc := astWithOneElement(&ast.MapElement{})
	math := astWithOneElement(ast.NewMathElement(diagnostics.Position{Line: 1}, "E = mc^2"))
	text := astWithOneElement(ast.NewTextElement(diagnostics.Position{Line: 1}, "just text"))

	cases := []struct {
		name string
		fn   func(*ast.AST) bool
		yes  *ast.AST
		no   []*ast.AST
	}{
		{"Mermaid", HasMermaidElements, mermaid, []*ast.AST{chart, mapDoc, math, text}},
		{"Chart", HasChartElements, chart, []*ast.AST{mermaid, mapDoc, math, text}},
		{"Map", HasMapElements, mapDoc, []*ast.AST{mermaid, chart, math, text}},
		{"Math", HasMathElements, math, []*ast.AST{mermaid, chart, mapDoc, text}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.fn(tc.yes) {
				t.Errorf("expected Has%sElements to be true for a document containing one", tc.name)
			}
			for _, doc := range tc.no {
				if tc.fn(doc) {
					t.Errorf("expected Has%sElements to be false for a document without one", tc.name)
				}
			}
		})
	}
}

func TestHasElementHelpers_NilASTIsFalse(t *testing.T) {
	if HasMermaidElements(nil) || HasChartElements(nil) || HasMapElements(nil) || HasMathElements(nil) {
		t.Error("expected all Has*Elements helpers to return false for a nil AST")
	}
}
