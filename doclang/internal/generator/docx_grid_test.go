// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// docxDocumentXML unzips a .docx and returns word/document.xml as a string.
func docxDocumentXML(t *testing.T, docxPath string) string {
	t.Helper()
	r, err := zip.OpenReader(docxPath)
	if err != nil {
		t.Fatalf("failed to open generated docx: %v", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open word/document.xml in docx: %v", err)
		}
		defer func() { _ = rc.Close() }()
		buf, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("failed to read word/document.xml: %v", err)
		}
		return string(buf)
	}
	t.Fatal("word/document.xml not found in generated docx")
	return ""
}

// astWithGrid reuses the shared minimal-AST fixture and appends a
// GridElement whose columns have multi-line Content (matching what
// core/elements/grid.go's parseColumn actually produces: Content
// is populated, Elements never is).
func astWithGrid() *ast.AST {
	doc := newTestAST()

	grid := ast.NewGridElement(diagnostics.NewPosition(3, 1))
	colA := ast.NewColumnElement(diagnostics.NewPosition(3, 1), "### Column Heading\nContenido columna A")
	colB := ast.NewColumnElement(diagnostics.NewPosition(3, 1), "Contenido columna B")
	grid.Columns = append(grid.Columns, *colA, *colB)

	block := doc.ContentBlocks[0]
	block.Elements = append(block.Elements, grid)
	doc.ContentBlocks[0] = block
	return doc
}

// TestDOCXGenerator_RenderGrid_RendersColumnContent cubre issue #56: antes,
// renderGrid iteraba column.Elements (siempre vacío, ver
// core/elements/grid.go parseColumn) y las columnas de un grid
// exportado a DOCX no contenían ningún texto — solo los separadores
// "Columna N". El fix lee column.Content en su lugar.
func TestDOCXGenerator_RenderGrid_RendersColumnContent(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithGrid()

	output := filepath.Join(t.TempDir(), "grid.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)

	for _, expected := range []string{"Contenido columna A", "Contenido columna B", "Column Heading"} {
		if !strings.Contains(xml, expected) {
			t.Errorf("generated DOCX document.xml missing %q", expected)
		}
	}
}
