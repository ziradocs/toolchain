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

// astWithOrderMarkers builds a minimal one-section AST with three
// TextElements whose content is only distinguishable by an order marker —
// used by the order-preservation regression tests below.
func astWithOrderMarkers() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	front := ast.NewFrontMatterNode(pos)
	front.Title = "Order Fixture"
	doc.FrontMatter = front

	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Elements = append(block.Elements,
		ast.NewTextElement(pos, "order-marker-alpha"),
		ast.NewTextElement(pos, "order-marker-bravo"),
		ast.NewTextElement(pos, "order-marker-charlie"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

func assertMarkersInOrder(t *testing.T, haystack string) {
	t.Helper()
	iAlpha := strings.Index(haystack, "order-marker-alpha")
	iBravo := strings.Index(haystack, "order-marker-bravo")
	iCharlie := strings.Index(haystack, "order-marker-charlie")

	if iAlpha == -1 || iBravo == -1 || iCharlie == -1 {
		t.Fatalf("expected all three markers in output, got positions alpha=%d bravo=%d charlie=%d", iAlpha, iBravo, iCharlie)
	}
	if iAlpha >= iBravo || iBravo >= iCharlie {
		t.Errorf("elements were not emitted in AST order: alpha@%d bravo@%d charlie@%d", iAlpha, iBravo, iCharlie)
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
