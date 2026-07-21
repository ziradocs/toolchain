// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package serializer

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/renderer"
)

// TestSerializeToJSON_ProseCoverage cubre issue #9: --format json debe incluir el
// cuerpo de prosa completo de CADA tipo de elemento, no solo metadatos. Este test
// congela el contrato: si un campo deja de serializarse (tag json roto, campo
// renombrado, etc.), esta prueba debe fallar.
func TestSerializeToJSON_ProseCoverage(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	astNode := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")

	text := ast.NewTextElement(pos, "Cuerpo de texto de prosa")

	points := ast.NewPointsElement(pos)
	sub := ast.NewPointItem(pos, "Sub punto anidado")
	item := ast.NewPointItem(pos, "Punto principal")
	item.SubPoints = append(item.SubPoints, *sub)
	points.Items = append(points.Items, *item)

	quote := ast.NewQuoteElement(pos, "Una cita memorable")
	quote.Author = "Ada Lovelace"
	quote.Source = "Notas sobre la máquina analítica"

	table := ast.NewTableElement(pos)
	table.Headers = []string{"Q1", "Q2"}
	table.Rows = [][]string{{"100", "200"}}
	table.Caption = "Ventas trimestrales"

	code := ast.NewCodeElement(pos, "go", "func main() {}")

	codeGroup := ast.NewCodeGroupElement(pos)
	codeGroup.CodeBlocks = append(codeGroup.CodeBlocks, ast.CodeBlock{
		Language: "go", Label: "main.go", Content: "package main",
	})

	image := ast.NewImageElementWithContext(pos, "photo.png", "Texto alternativo", ast.ImageContextContent)
	image.Caption = "Pie de foto descriptivo"

	grid := ast.NewGridElement(pos)
	grid.Content = "Prosa suelta dentro del grid"
	col := ast.NewColumnElement(pos, "Contenido de la columna")
	grid.Columns = append(grid.Columns, *col)

	callout := ast.NewSpecialBlockElement(pos, "warning", "Este es un callout de advertencia")

	checklist := ast.NewChecklistElement(pos)
	checkItem := ast.NewChecklistItem(pos, "Tarea completada", true)
	checklist.Items = append(checklist.Items, *checkItem)

	mermaid := ast.NewMermaidElement(pos, "flowchart", "flowchart TD\nA --> B")

	chart := ast.NewChartElement(pos, "bar")
	chart.Series = []string{"Serie A"}
	chart.Data = [][]interface{}{{"Q1", 100.0}}

	mapElem := ast.NewMapElement(pos, "world")
	mapElem.Markers = append(mapElem.Markers, ast.MapMarker{Lat: 1.0, Lng: 2.0, Label: "Ciudad", Details: "Detalle del marcador"})

	block.Elements = append(block.Elements,
		text, points, quote, table, code, codeGroup, image, grid, callout, checklist, mermaid, chart, mapElem,
	)
	astNode.ContentBlocks = append(astNode.ContentBlocks, *block)

	// Issue #64: --format json expone HTML inline pre-renderizado junto al
	// contenido crudo, poblado por renderer.PopulateInlineHTML antes de
	// serializar (mismo paso que hace generateJSON). Se reproduce aquí para
	// que este test siga cubriendo el contrato real de generateJSON.
	renderer.PopulateInlineHTML(astNode, nil)

	s := New()
	data, err := s.SerializeToJSON(astNode)
	if err != nil {
		t.Fatalf("SerializeToJSON error: %v", err)
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("failed to decode serialized AST: %v", err)
	}

	elements := generic["contentBlocks"].([]interface{})[0].(map[string]interface{})["elements"].([]interface{})
	if len(elements) != 13 {
		t.Fatalf("expected 13 elements, got %d", len(elements))
	}

	byType := make(map[string]map[string]interface{})
	for _, e := range elements {
		m := e.(map[string]interface{})
		byType[m["type"].(string)] = m
	}

	assertField := func(elemType, field string, want interface{}) {
		t.Helper()
		elem, ok := byType[elemType]
		if !ok {
			t.Fatalf("element type %q missing from serialized output", elemType)
		}
		got, ok := elem[field]
		if !ok {
			t.Errorf("%s.%s missing from serialized output", elemType, field)
			return
		}
		if got != want {
			t.Errorf("%s.%s = %v, want %v", elemType, field, got, want)
		}
	}

	assertField("text", "content", "Cuerpo de texto de prosa")
	assertField("quote", "content", "Una cita memorable")
	assertField("quote", "author", "Ada Lovelace")
	assertField("quote", "source", "Notas sobre la máquina analítica")
	assertField("table", "caption", "Ventas trimestrales")
	assertField("code", "content", "func main() {}")
	assertField("image", "alt", "Texto alternativo")
	assertField("image", "caption", "Pie de foto descriptivo")
	assertField("grid", "content", "Prosa suelta dentro del grid")
	assertField("special_block", "content", "Este es un callout de advertencia")
	assertField("special_block", "blockType", "warning")
	assertField("mermaid", "content", "flowchart TD\nA --> B")

	// Issue #64: cada elemento de prosa expone un campo "*HTML" con el mismo
	// contenido ya renderizado a HTML inline (sin markdown para "Cuerpo de
	// texto de prosa", que no usa ningún formato inline, así que el HTML
	// coincide con el crudo).
	assertField("text", "contentHTML", "Cuerpo de texto de prosa")
	assertField("code", "contentHTML", "func main() {}")
	assertField("image", "altHTML", "Texto alternativo")
	assertField("image", "captionHTML", "Pie de foto descriptivo")
	assertField("grid", "contentHTML", "Prosa suelta dentro del grid")
	assertField("special_block", "contentHTML", "Este es un callout de advertencia")

	// Campos anidados (arrays/objetos) requieren asserts propios
	points_ := byType["points"]
	items := points_["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("points.items len = %d, want 1", len(items))
	}
	firstItem := items[0].(map[string]interface{})
	if firstItem["content"] != "Punto principal" {
		t.Errorf("points.items[0].content = %v, want %q", firstItem["content"], "Punto principal")
	}
	if firstItem["contentHTML"] != "Punto principal" {
		t.Errorf("points.items[0].contentHTML = %v, want %q", firstItem["contentHTML"], "Punto principal")
	}
	// PointItem sigue siendo "point_item" (issue #60 solo desambiguó
	// ChecklistItem, que antes lo compartía).
	if firstItem["type"] != "point_item" {
		t.Errorf(`points.items[0].type = %v, want "point_item"`, firstItem["type"])
	}
	subPoints := firstItem["subPoints"].([]interface{})
	if len(subPoints) != 1 || subPoints[0].(map[string]interface{})["content"] != "Sub punto anidado" {
		t.Errorf("points.items[0].subPoints missing/incorrect: %v", subPoints)
	}

	tableElem := byType["table"]
	headers := tableElem["headers"].([]interface{})
	if len(headers) != 2 || headers[0] != "Q1" {
		t.Errorf("table.headers incorrect: %v", headers)
	}

	codeGroupElem := byType["code_group"]
	codeBlocks := codeGroupElem["codeBlocks"].([]interface{})
	if len(codeBlocks) != 1 || codeBlocks[0].(map[string]interface{})["content"] != "package main" {
		t.Errorf("code_group.codeBlocks incorrect: %v", codeBlocks)
	}

	gridElem := byType["grid"]
	columns := gridElem["columns"].([]interface{})
	if len(columns) != 1 || columns[0].(map[string]interface{})["content"] != "Contenido de la columna" {
		t.Errorf("grid.columns incorrect: %v", columns)
	}

	checklistElem := byType["checklist"]
	checkItems := checklistElem["items"].([]interface{})
	if len(checkItems) != 1 {
		t.Fatalf("checklist.items len = %d, want 1", len(checkItems))
	}
	firstCheck := checkItems[0].(map[string]interface{})
	if firstCheck["content"] != "Tarea completada" || firstCheck["checked"] != true {
		t.Errorf("checklist.items[0] incorrect: %v", firstCheck)
	}
	if firstCheck["contentHTML"] != "Tarea completada" {
		t.Errorf("checklist.items[0].contentHTML = %v, want %q", firstCheck["contentHTML"], "Tarea completada")
	}
	// Issue #60: ChecklistItem ya no comparte el discriminador "point_item"
	// con PointItem (SchemaVersion 2.0.0).
	if firstCheck["type"] != "checklist_item" {
		t.Errorf(`checklist.items[0].type = %v, want "checklist_item"`, firstCheck["type"])
	}

	mapElemDecoded := byType["map"]
	markers := mapElemDecoded["markers"].([]interface{})
	if len(markers) != 1 || markers[0].(map[string]interface{})["details"] != "Detalle del marcador" {
		t.Errorf("map.markers incorrect: %v", markers)
	}
}
