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
	"go.ziradocs.com/core/v2/util"
)

// buildMergedCellsAST arma un *ast.AST con una sola tabla cuyas Cells se
// pasan tal cual — usado para reproducir, contra el generador PPTX real, dos
// escenarios de falla que el code review de #41 encontró: (a) grid width
// derivado solo de la fila 0, (b) merges solapados abortando Save() para
// todo el deck.
func buildMergedCellsAST(name string, cells [][]ast.TableCell) *ast.AST {
	pos := diagnostics.Position{Line: 1, Column: 1}
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FilePath = name

	block := ast.NewContentBlock(pos, "content")
	block.Title = "Repro Slide"

	table := ast.NewTableElement(pos)
	table.Cells = cells
	block.Elements = append(block.Elements, table)

	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

// TestPptxAddTableCells_GridWidthMatchesWidestRow reproduce el hallazgo de
// code review "pptxTableGridWidth deriva el ancho de grilla solo de la fila
// 0": row0 declara colspan=2 (ancho 2), row1 declara 3 celdas propias (ancho
// 3) — antes del fix, cols=2 y la celda "ccc" se descartaba en silencio.
func TestPptxAddTableCells_GridWidthMatchesWidestRow(t *testing.T) {
	cells := [][]ast.TableCell{
		{{Content: "Grupo", IsHeader: true, Scope: "col", ColSpan: 2}},
		{{Content: "aaa"}, {Content: "bbb"}, {Content: "ccc"}},
	}
	doc := buildMergedCellsAST("wide-row.slidelang", cells)

	dir := t.TempDir()
	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{}); err != nil {
		t.Fatalf("generatePPTX: %v", err)
	}

	outputPath := filepath.Join(dir, "wide-row.pptx")
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

	if !strings.Contains(slideXML, "ccc") {
		t.Errorf("generated pptx is missing cell content \"ccc\" — grid width was computed too narrow and the trailing cell got dropped:\n%s", slideXML)
	}
}

// TestPptxAddTableCells_OverlappingSpansDoNotAbortSave reproduce el hallazgo
// "pptxAddTableCells nunca clampea la extensión de un merge contra
// occupied[][], así que spans solapados llegan a MergeCells y abortan
// Save() para todo el deck": row0 [A][B rowspan=2], row1 [C rowspan=2
// colspan=2] (se solapa con B), row2 [D].
func TestPptxAddTableCells_OverlappingSpansDoNotAbortSave(t *testing.T) {
	cells := [][]ast.TableCell{
		{{Content: "A"}, {Content: "B", RowSpan: 2}},
		{{Content: "C", RowSpan: 2, ColSpan: 2}},
		{{Content: "D"}},
	}
	doc := buildMergedCellsAST("overlap.slidelang", cells)

	dir := t.TempDir()
	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{}); err != nil {
		t.Fatalf("generatePPTX returned an error for overlapping Cells instead of degrading the merge: %v", err)
	}

	outputPath := filepath.Join(dir, "overlap.pptx")
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf(".pptx file was not written: %v", err)
	}
}
