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
// ast.ContentBlock.Elements or ast.AST.ContentBlocks. pptxAddElement
// (pptx.go) iterates block.Elements with a monotonically increasing
// cursorY, so shape-tree order should already match AST order within a
// slide — this guards that claim against regression. It mixes the element
// types pptxAddElement actually dispatches in v0 (Text/Points/Table —
// Image is skipped here, its alt text isn't guaranteed to land verbatim in
// the shape tree) to catch a hypothetical group-by-type bug, and adds a
// second ContentBlock (slide2.xml) to guard ContentBlocks order too, which
// a single-slide fixture can't exercise (code review of #62's
// documentation follow-up, PR #88).
func TestGeneratePPTX_PreservesElementOrder(t *testing.T) {
	dir := t.TempDir()

	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FilePath = "order.slidelang"

	points := ast.NewPointsElement(pos())
	points.Items = append(points.Items, *ast.NewPointItem(pos(), "order-marker-bravo"))

	table := ast.NewTableElement(pos())
	table.Headers = []string{"order-marker-charlie"}

	block1 := ast.NewContentBlock(pos(), "content")
	block1.Title = "Order"
	block1.Elements = append(block1.Elements,
		ast.NewTextElement(pos(), "order-marker-alpha"),
		points,
		table,
	)
	block2 := ast.NewContentBlock(pos(), "content")
	block2.Title = "Order 2"
	block2.Elements = append(block2.Elements,
		ast.NewTextElement(pos(), "order-marker-delta"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block1, *block2)

	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{AssetRoot: dir}); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	outputPath := filepath.Join(dir, "order.pptx")
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}

	slide1XML := zipEntryContent(t, outputPath, "ppt/slides/slide1.xml")

	iAlpha := strings.Index(slide1XML, "order-marker-alpha")
	iBravo := strings.Index(slide1XML, "order-marker-bravo")
	iCharlie := strings.Index(slide1XML, "order-marker-charlie")

	if iAlpha == -1 || iBravo == -1 || iCharlie == -1 {
		t.Fatalf("expected all three markers in slide1.xml, got positions alpha=%d bravo=%d charlie=%d", iAlpha, iBravo, iCharlie)
	}
	if iAlpha >= iBravo || iBravo >= iCharlie {
		t.Errorf("shapes were not emitted in AST order (possible grouping-by-type bug): alpha@%d bravo@%d charlie@%d", iAlpha, iBravo, iCharlie)
	}

	slide2XML := zipEntryContent(t, outputPath, "ppt/slides/slide2.xml")
	if !strings.Contains(slide2XML, "order-marker-delta") {
		t.Fatalf("expected second ContentBlock's marker in slide2.xml (ContentBlocks order not preserved)")
	}
	if strings.Contains(slide1XML, "order-marker-delta") || strings.Contains(slide2XML, "order-marker-alpha") {
		t.Errorf("markers leaked across slides: block1/block2 content not cleanly separated")
	}
}
