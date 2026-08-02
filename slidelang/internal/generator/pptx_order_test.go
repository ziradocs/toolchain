// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// TestGeneratePPTX_PreservesElementOrder is the slidelang/PPTX half of
// issue #62's "(a)" resolution: no renderer permutes
// ast.ContentBlock.Elements. pptxAddElement (pptx.go) iterates
// block.Elements with a monotonically increasing cursorY, so shape-tree
// order should already match AST order — this guards that claim against
// regression.
func TestGeneratePPTX_PreservesElementOrder(t *testing.T) {
	dir := t.TempDir()

	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FilePath = "order.slidelang"

	block := ast.NewContentBlock(pos(), "content")
	block.Title = "Order"
	block.Elements = append(block.Elements,
		ast.NewTextElement(pos(), "order-marker-alpha"),
		ast.NewTextElement(pos(), "order-marker-bravo"),
		ast.NewTextElement(pos(), "order-marker-charlie"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{AssetRoot: dir}); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	outputPath := filepath.Join(dir, "order.pptx")
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}

	slideXML := zipEntryContent(t, outputPath, "ppt/slides/slide1.xml")

	iAlpha := strings.Index(slideXML, "order-marker-alpha")
	iBravo := strings.Index(slideXML, "order-marker-bravo")
	iCharlie := strings.Index(slideXML, "order-marker-charlie")

	if iAlpha == -1 || iBravo == -1 || iCharlie == -1 {
		t.Fatalf("expected all three markers in slide1.xml, got positions alpha=%d bravo=%d charlie=%d", iAlpha, iBravo, iCharlie)
	}
	if iAlpha >= iBravo || iBravo >= iCharlie {
		t.Errorf("shapes were not emitted in AST order: alpha@%d bravo@%d charlie@%d", iAlpha, iBravo, iCharlie)
	}
}
