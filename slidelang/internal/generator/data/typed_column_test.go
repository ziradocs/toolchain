// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// gridElementData corre la conversión sobre un deck de un solo slide con un
// solo GridElement y devuelve el ElementData resultante (Type == "grid").
func gridElementData(t *testing.T, grid *ast.GridElement) ElementData {
	t.Helper()
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = []ast.Element{grid}
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	got := PrepareTemplateDataWithRenderMode(doc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	if len(got.ContentBlocks) != 1 || len(got.ContentBlocks[0].Elements) != 1 {
		t.Fatalf("se esperaba 1 slide con 1 elemento, se obtuvo %d/%d",
			len(got.ContentBlocks), len(got.ContentBlocks[0].Elements))
	}
	return got.ContentBlocks[0].Elements[0]
}

// Issue #373: una columna cruda (solo Content, sin Elements) debe convertirse
// EXACTAMENTE igual que antes de este cambio — ColumnData.Content procesado
// por variables, ColumnData.Elements vacío.
func TestConvertColumns_RawColumnUnchanged(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	grid := ast.NewGridElement(pos)
	grid.Columns = append(grid.Columns, *ast.NewColumnElement(pos, "Left side."))
	grid.Columns = append(grid.Columns, *ast.NewColumnElement(pos, "Right side."))

	got := gridElementData(t, grid)

	if len(got.Columns) != 2 {
		t.Fatalf("se esperaban 2 columnas, se obtuvieron %d", len(got.Columns))
	}
	if got.Columns[0].Content != "Left side." {
		t.Errorf("Columns[0].Content = %q, se esperaba %q", got.Columns[0].Content, "Left side.")
	}
	if got.Columns[1].Content != "Right side." {
		t.Errorf("Columns[1].Content = %q, se esperaba %q", got.Columns[1].Content, "Right side.")
	}
	if len(got.Columns[0].Elements) != 0 || len(got.Columns[1].Elements) != 0 {
		t.Errorf("una columna cruda no debe traer Elements: %+v", got.Columns)
	}
}

// Una columna tipada (Content vacío, Elements poblado a mano, como haría el
// parser nuevo de core) debe convertir cada elemento anidado con el mismo
// pipeline que un elemento de slide: texto, puntos, imagen y chart, en orden.
func TestConvertColumns_TypedColumnConvertsNestedElements(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	grid := ast.NewGridElement(pos)

	text := ast.NewTextElement(pos, "Hola {{name}}.")
	points := ast.NewPointsElement(pos)
	points.Items = append(points.Items, *ast.NewPointItem(pos, "Uno"))
	image := ast.NewImageElement(pos, "foo.png", "alt")
	chart := ast.NewChartElement(pos, "bar")
	chart.Labels = []string{"Q1"}
	chart.Series = []string{"Serie"}

	col := ast.NewColumnElement(pos, "")
	col.Elements = []ast.Element{text, points, image, chart}
	grid.Columns = append(grid.Columns, *col)

	got := gridElementData(t, grid)

	if len(got.Columns) != 1 {
		t.Fatalf("se esperaba 1 columna, se obtuvieron %d", len(got.Columns))
	}
	column := got.Columns[0]
	if column.Content != "" {
		t.Errorf("Content de una columna tipada debe quedar vacío, es %q", column.Content)
	}
	if len(column.Elements) != 4 {
		t.Fatalf("se esperaban 4 elementos anidados, se obtuvieron %d", len(column.Elements))
	}

	if column.Elements[0].Type != "text" {
		t.Errorf("Elements[0].Type = %q, se esperaba \"text\"", column.Elements[0].Type)
	}
	if column.Elements[0].Content != "Hola {{name}}." {
		t.Errorf("Elements[0].Content = %q", column.Elements[0].Content)
	}
	if column.Elements[1].Type != "points" {
		t.Errorf("Elements[1].Type = %q, se esperaba \"points\"", column.Elements[1].Type)
	}
	if len(column.Elements[1].Items) != 1 || column.Elements[1].Items[0].Content != "Uno" {
		t.Errorf("Elements[1].Items = %+v", column.Elements[1].Items)
	}
	if column.Elements[2].Type != "image" {
		t.Errorf("Elements[2].Type = %q, se esperaba \"image\"", column.Elements[2].Type)
	}
	if column.Elements[2].Source != "foo.png" {
		t.Errorf("Elements[2].Source = %q", column.Elements[2].Source)
	}
	if column.Elements[3].Type != "chart" {
		t.Errorf("Elements[3].Type = %q, se esperaba \"chart\"", column.Elements[3].Type)
	}

	// El ID de cada elemento anidado tiene que ser único y derivado del ID
	// del grid (ver nestedElementID) — lo que generateChartsMetadata usa
	// para emparejar el <canvas id="..."> con su config Chart.js.
	seen := map[string]bool{}
	for _, el := range column.Elements {
		if el.ElementID == "" {
			t.Errorf("elemento %q sin ElementID", el.Type)
		}
		if seen[el.ElementID] {
			t.Errorf("ElementID %q repetido entre elementos anidados", el.ElementID)
		}
		seen[el.ElementID] = true
	}
}

// El chart anidado en una columna tipada debe aparecer en data.Charts con un
// ID que matchee el ElementID que el template usó para su <canvas>, no solo
// "existir" un canvas sin su configuración Chart.js (issue #373).
func TestGenerateChartsMetadata_DescendsIntoTypedColumn(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	grid := ast.NewGridElement(pos)
	chart := ast.NewChartElement(pos, "bar")
	chart.Labels = []string{"Q1"}
	chart.Series = []string{"Serie"}
	chart.Data = [][]interface{}{{10.0}}

	col := ast.NewColumnElement(pos, "")
	col.Elements = []ast.Element{chart}
	grid.Columns = append(grid.Columns, *col)

	block := ast.NewContentBlock(pos, "content")
	block.Elements = []ast.Element{grid}
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	data := PrepareTemplateDataWithRenderMode(doc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())

	gridData := data.ContentBlocks[0].Elements[0]
	if len(gridData.Columns) != 1 || len(gridData.Columns[0].Elements) != 1 {
		t.Fatalf("estructura inesperada: %+v", gridData)
	}
	chartElementID := gridData.Columns[0].Elements[0].ElementID
	wantCanvasID := "slidelang-element-chart-0-" + chartElementID

	if len(data.Charts) != 1 {
		t.Fatalf("se esperaba 1 chart en data.Charts, se obtuvieron %d", len(data.Charts))
	}
	if data.Charts[0].ID != wantCanvasID {
		t.Errorf("data.Charts[0].ID = %q, se esperaba %q (el mismo id= que el <canvas> del template)",
			data.Charts[0].ID, wantCanvasID)
	}
}
