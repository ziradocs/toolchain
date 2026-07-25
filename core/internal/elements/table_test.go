// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func TestTableParser_CanParse(t *testing.T) {
	parser := &TableParser{}

	tests := []struct {
		name     string
		line     string
		mode     string
		expected bool
	}{
		{"strict TABLE keyword", "TABLE", "strict", true},
		// issue #20: TABLE debe reconocerse también en flex (doclang solo
		// parsea flex) para que la sintaxis "cells:" explícita de celdas
		// fusionadas sea autorable en doclang.
		{"flex TABLE keyword", "TABLE", "flex", true},
		{"markdown table row", "| A | B |", "flex", true},
		{"plain text", "just some text", "flex", false},
		// Regresión de issue #245: TableParser va antes de Quote/Checklist/
		// Points/Text en el registry (GetDefaultRegistry) — sin exigir "|"
		// inicial, cualquier línea con 2+ pipes le robaba el elemento a su
		// parser real.
		{"bullet with 2+ pipes is not a table (issue #245)", "- Compara pandas | numpy | scipy", "flex", false},
		{"quote with 2+ pipes is not a table", "> revenue | costs | margin", "flex", false},
		{"checklist with 2+ pipes is not a table", "- [ ] a | b | c", "flex", false},
		{"markdown table row with leading whitespace still matches", "  | A | B |", "flex", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parser.CanParse(tt.line, tt.mode); got != tt.expected {
				t.Errorf("CanParse(%q, %q) = %v, want %v", tt.line, tt.mode, got, tt.expected)
			}
		})
	}
}

func TestTableParser_ParseMarkdownTable(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Header A | Header B |",
			"|---|---|",
			"| val1 | val2 |",
			"| val3 | val4 |",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatal("Element is not TableElement")
	}

	wantHeaders := []string{"Header A", "Header B"}
	if len(table.Headers) != len(wantHeaders) {
		t.Fatalf("len(Headers) = %d, want %d", len(table.Headers), len(wantHeaders))
	}
	for i, h := range wantHeaders {
		if table.Headers[i] != h {
			t.Errorf("Headers[%d] = %q, want %q", i, table.Headers[i], h)
		}
	}

	if len(table.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(table.Rows))
	}
}

// TestTableParser_ParseMarkdownTable_AutoDerivesCells cubre issue #20: una
// tabla markdown simple (sin celdas fusionadas) debe poblar Cells derivándolo
// de Headers/Rows — la fila de headers marcada IsHeader+Scope="col", el
// cuerpo como celdas simples sin span — para que un rulepack A11Y pueda
// caminar la estructura de celdas sin importar qué sintaxis de autoría se usó.
func TestTableParser_ParseMarkdownTable_AutoDerivesCells(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Header A | Header B |",
			"|---|---|",
			"| val1 | val2 |",
		},
	}

	result := parser.Parse(ctx, 0)
	table := result.Element.(*ast.TableElement)

	if len(table.Cells) != 2 {
		t.Fatalf("len(Cells) = %d, want 2 (1 header row + 1 body row)", len(table.Cells))
	}
	headerRow := table.Cells[0]
	if len(headerRow) != 2 || !headerRow[0].IsHeader || headerRow[0].Scope != "col" || headerRow[0].Content != "Header A" {
		t.Errorf("Cells[0] = %+v, want header row with IsHeader=true, Scope=col", headerRow)
	}
	bodyRow := table.Cells[1]
	if len(bodyRow) != 2 || bodyRow[0].IsHeader || bodyRow[0].Content != "val1" {
		t.Errorf("Cells[1] = %+v, want plain body row", bodyRow)
	}
}

// TestTableParser_ExplicitCells_MergedHeader cubre issue #20: la sintaxis
// "cells:" explícita dentro de un bloque TABLE en modo flex (doclang) debe
// parsear colspan/scope/header, y Headers/Rows derivados deben quedar
// dimensionalmente consistentes (misma cantidad de columnas en headers y en
// cada fila) — precisamente la propiedad que evita el falso positivo
// TABLE003 (ver TestElementStructureRule_MergedCells_NoFalsePositive en
// core/linter).
func TestTableParser_ExplicitCells_MergedHeader(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"TABLE",
			"  cells:",
			"    - [{content: A, header: true, colspan: 2}, {content: B, header: true}]",
			"    - [{content: 1}, {content: 2}, {content: 3}]",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatal("Element is not TableElement")
	}

	if len(table.Cells) != 2 {
		t.Fatalf("len(Cells) = %d, want 2", len(table.Cells))
	}
	if got := table.Cells[0][0]; !got.IsHeader || got.ColSpan != 2 || got.Content != "A" {
		t.Errorf("Cells[0][0] = %+v, want {Content:A IsHeader:true ColSpan:2}", got)
	}

	wantHeaders := []string{"A", "A", "B"}
	if len(table.Headers) != len(wantHeaders) {
		t.Fatalf("len(Headers) = %d, want %d (derived from colspan=2 on A)", len(table.Headers), len(wantHeaders))
	}
	for i, h := range wantHeaders {
		if table.Headers[i] != h {
			t.Errorf("Headers[%d] = %q, want %q", i, table.Headers[i], h)
		}
	}

	if len(table.Rows) != 1 || len(table.Rows[0]) != len(table.Headers) {
		t.Fatalf("Rows = %v, want 1 row with %d columns (matching Headers width)", table.Rows, len(table.Headers))
	}
}

// TestTableParser_StrictMode_CaptionPopulated cubre issue #9: TableElement.Caption
// existía en el struct pero el parser strict-mode nunca lo poblaba desde la línea
// "caption:" (a diferencia de image.go, que ya soporta ese patrón).
func TestTableParser_StrictMode_CaptionPopulated(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"TABLE",
			`  headers: ["Q1", "Q2"]`,
			`  caption: "Ventas trimestrales"`,
			"  rows:",
			"      [100, 200]",
			"      [150, 250]",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatal("Element is not TableElement")
	}

	if table.Caption != "Ventas trimestrales" {
		t.Errorf("Caption = %q, want %q", table.Caption, "Ventas trimestrales")
	}
	if len(table.Headers) != 2 {
		t.Errorf("len(Headers) = %d, want 2", len(table.Headers))
	}
	if len(table.Rows) != 2 {
		t.Errorf("len(Rows) = %d, want 2", len(table.Rows))
	}
}

// TestTableParser_StrictMode_CaptionAfterRows cubre el mismo caso que
// TestTableParser_StrictMode_CaptionPopulated pero con "caption:" DESPUÉS del
// bloque "rows:", para verificar que el manejo de índice (i--/continue) del
// sub-loop de rows no rompe el procesamiento de la línea de caption siguiente.
func TestTableParser_StrictMode_CaptionAfterRows(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"TABLE",
			`  headers: ["Q1", "Q2"]`,
			"  rows:",
			"      [100, 200]",
			"      [150, 250]",
			`  caption: "Ventas trimestrales"`,
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	table := result.Element.(*ast.TableElement)

	if table.Caption != "Ventas trimestrales" {
		t.Errorf("Caption = %q, want %q (caption after rows must still be parsed)", table.Caption, "Ventas trimestrales")
	}
	if len(table.Rows) != 2 {
		t.Errorf("len(Rows) = %d, want 2", len(table.Rows))
	}
}

// TestTableParser_StrictMode_NoCaption_StaysEmpty es la contraparte de
// regresión: una tabla strict-mode sin línea "caption:" debe dejar
// TableElement.Caption como "" (sin inventar ni heredar un valor por defecto).
func TestTableParser_StrictMode_NoCaption_StaysEmpty(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"TABLE",
			`  headers: ["Q1", "Q2"]`,
			"  rows:",
			"      [100, 200]",
		},
	}

	result := parser.Parse(ctx, 0)
	table := result.Element.(*ast.TableElement)

	if table.Caption != "" {
		t.Errorf("Caption = %q, want empty when no caption: line is present", table.Caption)
	}
}

// TestTableParser_MarkdownMode_NeverSetsCaption cubre issue #9: el soporte de
// caption se agregó solo al parser YAML (strict mode); parseMarkdownTable no
// tiene sintaxis de caption. Esta prueba fija esa asimetría documentada para
// detectar si una futura implementación agrega caption al modo markdown sin
// que sea intencional.
func TestTableParser_MarkdownMode_NeverSetsCaption(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Header A | Header B |",
			"|---|---|",
			"| val1 | val2 |",
		},
	}

	result := parser.Parse(ctx, 0)
	table := result.Element.(*ast.TableElement)

	if table.Caption != "" {
		t.Errorf("Caption = %q, want empty: markdown-mode tables have no caption syntax", table.Caption)
	}
}

// TestTableParser_ParseMarkdownTable_SerializesAsEmptyArrays cubre issue #8:
// headers/rows deben serializar como [] (no JSON null) incluso cuando quedan
// vacíos, round-tripping por json.Marshal (no solo inspeccionando el slice Go).
func TestTableParser_ParseMarkdownTable_SerializesAsEmptyArrays(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Header A | Header B |",
			"|---|---|",
			"| val1 | val2 |",
		},
	}

	result := parser.Parse(ctx, 0)
	table := result.Element.(*ast.TableElement)

	data, err := json.Marshal(table)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if decoded["headers"] == nil {
		t.Errorf("serialized headers is null, want a non-null array: %s", data)
	}
	if decoded["rows"] == nil {
		t.Errorf("serialized rows is null, want a non-null array: %s", data)
	}
}
