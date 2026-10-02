// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestDetectRequiredElementsFromAST_DescendsIntoTypedGridColumn (issue #373):
// un chart dentro de una columna tipada (<<column typed>>/::: column typed)
// tiene que pedir el mismo módulo CSS "charts" que si estuviera a nivel de
// slide (mismo motivo que los issues #166/#173, que agregaron el case de
// chart/mermaid a este switch porque sin el módulo el <img>/<canvas> offline
// sale sin max-width/max-height). classifyElementCSSModule desciende a
// GridElement.Columns[*].Elements para esto.
func TestDetectRequiredElementsFromAST_DescendsIntoTypedGridColumn(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	chart := ast.NewChartElement(pos, "bar")

	col := ast.NewColumnElement(pos, "")
	col.Elements = []ast.Element{chart}

	grid := ast.NewGridElement(pos)
	grid.Columns = append(grid.Columns, *col)

	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, grid)
	astNode := ast.NewAST(pos)
	astNode.ContentBlocks = append(astNode.ContentBlocks, *block)

	g := &Generator{}
	required := g.detectRequiredElementsFromAST(astNode)

	found := false
	for _, e := range required {
		if e == "charts" {
			found = true
		}
	}
	if !found {
		t.Fatalf("detectRequiredElementsFromAST no pidió \"charts\" para un chart dentro de una columna tipada, obtenidos: %v", required)
	}
}
