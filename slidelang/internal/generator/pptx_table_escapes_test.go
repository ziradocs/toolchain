// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// Las celdas de una tabla en PPTX no pasan por el procesador inline, pero el
// HTML sí: `c \*c\*` se ve `c *c*` ahí. Los escapes se resuelven igual en el
// encabezado y en las celdas.
func TestGeneratePPTX_TableCellsResolveEscapes(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Headers = []string{`H \*h\*`}
	table.Rows = [][]string{{`c \*c\* y \=\= y C:\Users\_tmp`}}
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Tabla"
	block.Elements = []ast.Element{table}
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}, FilePath: "test.slidelang"}

	dir := t.TempDir()
	if err := New(util.NewNoop()).generatePPTX(doc, dir, GeneratorOptions{AssetRoot: dir}); err != nil {
		t.Fatal(err)
	}
	xml := zipEntryContent(t, filepath.Join(dir, "test.pptx"), "ppt/slides/slide1.xml")
	for _, want := range []string{">H *h*<", `>c *c* y == y C:\Users\_tmp<`} {
		if !strings.Contains(xml, want) {
			t.Errorf("missing %s in slide1.xml", want)
		}
	}
	if strings.Contains(xml, `\*`) {
		t.Errorf("an escape backslash reached the slide")
	}
}

// La misma regla en una tabla con celdas tipadas (Cells, con colspan).
func TestGeneratePPTX_TypedTableCellsResolveEscapes(t *testing.T) {
	doc := astWithMergedTable()
	table := doc.ContentBlocks[0].Elements[0].(*ast.TableElement)
	table.Cells[1][0].Content = `A \*1\*`
	doc.FilePath = "test.slidelang"
	dir := t.TempDir()
	if err := New(util.NewNoop()).generatePPTX(doc, dir, GeneratorOptions{AssetRoot: dir}); err != nil {
		t.Fatal(err)
	}
	xml := zipEntryContent(t, filepath.Join(dir, "test.pptx"), "ppt/slides/slide1.xml")
	if !strings.Contains(xml, ">A *1*<") || strings.Contains(xml, `\*`) {
		t.Errorf("typed cell kept its escape backslash")
	}
}
