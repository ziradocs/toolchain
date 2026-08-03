// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// astWithOrderMarkers builds a two-section AST with a heterogeneous mix of
// element types (Text/Quote/Code) in the first section and a second section
// after it — used by the order-preservation regression tests below. Mixing
// types guards against a bug that groups output by element type (a
// same-type run wouldn't expose that); the second section guards against a
// bug that reorders AST.ContentBlocks itself, which a single-section fixture
// can't exercise (code review of #62's documentation follow-up, PR #88).
func astWithOrderMarkers() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	front := ast.NewFrontMatterNode(pos)
	front.Title = "Order Fixture"
	doc.FrontMatter = front

	block1 := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block1.Elements = append(block1.Elements,
		ast.NewTextElement(pos, "order-marker-alpha"),
		ast.NewQuoteElement(pos, "order-marker-bravo"),
		ast.NewCodeElement(pos, "text", "order-marker-charlie"),
	)
	block2 := ast.NewContentBlock(diagnostics.NewPosition(3, 1), "content")
	block2.Elements = append(block2.Elements,
		ast.NewTextElement(pos, "order-marker-delta"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block1, *block2)
	return doc
}

// orderMarkers lists, in expected order, the markers astWithOrderMarkers
// scatters across its elements and ContentBlocks.
var orderMarkers = []string{
	"order-marker-alpha", "order-marker-bravo", "order-marker-charlie", "order-marker-delta",
}

func assertMarkersInOrder(t *testing.T, haystack string) {
	t.Helper()
	positions := make([]int, len(orderMarkers))
	for i, marker := range orderMarkers {
		positions[i] = strings.Index(haystack, marker)
		if positions[i] == -1 {
			t.Fatalf("expected marker %q in output, got positions %v", marker, positions)
		}
	}
	for i := 1; i < len(positions); i++ {
		if positions[i-1] >= positions[i] {
			t.Errorf("elements/sections were not emitted in AST order: %v at positions %v", orderMarkers, positions)
		}
	}
}

// TestDOCXGenerator_PreservesElementOrder is the doclang half of issue #62's
// "(a)" resolution: no renderer permutes ast.ContentBlock.Elements. docx.go
// appends paragraphs sequentially into the body flow (renderSection), so
// this guards against a future change (e.g. a reordering optimization)
// silently breaking that.
func TestDOCXGenerator_PreservesElementOrder(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithOrderMarkers()

	output := filepath.Join(t.TempDir(), "order.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	assertMarkersInOrder(t, docxDocumentXML(t, output))
}

// TestMarkdownGenerator_PreservesElementOrder is the Markdown half of the
// same guard.
func TestMarkdownGenerator_PreservesElementOrder(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)
	doc := astWithOrderMarkers()

	output := filepath.Join(t.TempDir(), "order.md")
	if err := gen.Generate(doc, output, GeneratorOptions{}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read generated markdown: %v", err)
	}
	assertMarkersInOrder(t, string(content))
}
