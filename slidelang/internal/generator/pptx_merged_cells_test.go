// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// zipEntryContent returns the content of a single named entry inside path
// (a .pptx/.docx/any zip package) as a string.
func zipEntryContent(t *testing.T, path, entryName string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("output is not a valid zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.Name != entryName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open %s: %v", entryName, err)
		}
		defer func() { _ = rc.Close() }()
		buf, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("failed to read %s: %v", entryName, err)
		}
		return string(buf)
	}
	t.Fatalf("%s not found in package", entryName)
	return ""
}

// astWithMergedTable builds a minimal one-slide AST with a TableElement
// whose Cells (issue #20) declare a header cell spanning two columns —
// e.Headers/e.Rows are left at their DeriveCellsFromFlat-consistent values
// so a bug that accidentally used the flat path instead of Cells wouldn't
// silently look correct.
func astWithMergedTable() *ast.AST {
	pos := diagnostics.Position{Line: 1, Column: 1}
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)

	table := ast.NewTableElement(pos)
	table.Headers = []string{"Wide Header", ""}
	table.Rows = [][]string{{"A1", "B1"}}
	table.Cells = [][]ast.TableCell{
		{
			{Content: "Wide Header", IsHeader: true, Scope: "col", ColSpan: 2},
		},
		{
			{Content: "A1"},
			{Content: "B1"},
		},
	}

	block := ast.NewContentBlock(pos, "content")
	block.Title = "Merged Table Slide"
	block.Elements = append(block.Elements, table)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	return doc
}

// TestGeneratePPTX_MergedTableCells_UsesMergeCells covers issue #20: a
// TableElement whose Cells declare a real colspan must render via
// Table.MergeCells in the PPTX output, not silently collapse to the flat
// Headers/Rows path (which has no way to express a merge at all).
func TestGeneratePPTX_MergedTableCells_UsesMergeCells(t *testing.T) {
	dir := t.TempDir()
	doc := astWithMergedTable()
	doc.FilePath = "merged.slidelang"

	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{}); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	outputPath := filepath.Join(dir, "merged.pptx")
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}

	names := zipEntryNames(t, outputPath)
	var slideXML string
	for _, n := range names {
		if strings.HasPrefix(n, "ppt/slides/slide") && strings.HasSuffix(n, ".xml") {
			slideXML = zipEntryContent(t, outputPath, n)
			break
		}
	}
	if slideXML == "" {
		t.Fatalf("no slide XML found in package, entries: %v", names)
	}

	if !strings.Contains(slideXML, "gridSpan") {
		t.Errorf("expected a gridSpan attribute from the colspan=2 merge, slide XML:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, "Wide Header") {
		t.Errorf("expected the anchor cell's text to survive the merge, slide XML:\n%s", slideXML)
	}
}

// TestPptxTableGridWidth_SumsColSpanOfFirstRow covers the grid-width helper
// in isolation: AddTable needs the true column count, which can exceed
// len(row) once any cell declares ColSpan > 1.
func TestPptxTableGridWidth_SumsColSpanOfFirstRow(t *testing.T) {
	cells := [][]ast.TableCell{
		{{Content: "a", ColSpan: 2}, {Content: "b"}},
	}
	if got := pptxTableGridWidth(cells); got != 3 {
		t.Errorf("pptxTableGridWidth() = %d, want 3", got)
	}
}

// TestPptxTableUsesCellStructure_SimpleTableIsFalse covers the gate that
// decides plain-vs-merged rendering: a table whose Cells is exactly what
// DeriveCellsFromFlat would produce (header row IsHeader+scope=col, plain
// body rows) must NOT be treated as "has real structure" — otherwise every
// simple table in every existing deck would switch rendering paths.
func TestPptxTableUsesCellStructure_SimpleTableIsFalse(t *testing.T) {
	simple := ast.DeriveCellsFromFlat([]string{"H1", "H2"}, [][]string{{"a", "b"}})
	elem := &ast.TableElement{Cells: simple}
	if pptxTableUsesCellStructure(elem) {
		t.Error("a DeriveCellsFromFlat-shaped table must not be treated as having real cell structure")
	}
}

func TestPptxTableUsesCellStructure_MergedTableIsTrue(t *testing.T) {
	elem := &ast.TableElement{Cells: astWithMergedTable().ContentBlocks[0].Elements[0].(*ast.TableElement).Cells}
	if !pptxTableUsesCellStructure(elem) {
		t.Error("a table with a real colspan must be treated as having real cell structure")
	}
}
