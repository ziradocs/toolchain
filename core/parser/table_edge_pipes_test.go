// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/util"
)

// firstTable devuelve la primera TableElement del AST, buscando en todos los
// bloques.
func firstTable(t *testing.T, doc *ast.AST) *ast.TableElement {
	t.Helper()
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if table, ok := el.(*ast.TableElement); ok {
				return table
			}
		}
	}
	t.Fatalf("no TableElement in the AST")
	return nil
}

// Los tres cuerpos de abajo describen la misma tabla. Con pipes de borde y
// fila delimitadora dentro de un TABLE (la forma del bug de
// https://github.com/ziradocs/toolchain/issues/379), con pipes de borde sin
// delimitadora, y sin pipes de borde. Los tres deben dar el mismo AST y
// ningún TABLE003.
var edgePipeTableBodies = map[string]string{
	"edge pipes with delimiter row": `  TABLE
    | Métrica | Q2 | Q3 |
    |---------|----|----|
    | Ingresos | 10 | 12 |
    | Costos | 4 | 5 |`,
	"edge pipes without delimiter row": `  TABLE
    | Métrica | Q2 | Q3 |
    | Ingresos | 10 | 12 |
    | Costos | 4 | 5 |`,
	"no edge pipes": `  TABLE
    Métrica | Q2 | Q3
    Ingresos | 10 | 12
    Costos | 4 | 5`,
	"bare markdown table": `  | Métrica | Q2 | Q3 |
  |---------|----|----|
  | Ingresos | 10 | 12 |
  | Costos | 4 | 5 |`,
}

func TestStrictParser_TableBlockEdgePipes_SameASTAsWithoutThem(t *testing.T) {
	wantHeaders := []string{"Métrica", "Q2", "Q3"}
	wantRows := [][]string{{"Ingresos", "10", "12"}, {"Costos", "4", "5"}}

	for name, body := range edgePipeTableBodies {
		t.Run(name, func(t *testing.T) {
			src := "SLIDE content\n  title: \"A\"\n" + body + "\n"
			doc, diags := NewStrictParser(src, util.NewNoop()).Parse()
			if n := countErrors(diags); n != 0 {
				t.Fatalf("got %d error diagnostics, want 0: %v", n, diags)
			}
			table := firstTable(t, doc)
			if !reflect.DeepEqual(table.Headers, wantHeaders) {
				t.Errorf("Headers = %#v, want %#v", table.Headers, wantHeaders)
			}
			if !reflect.DeepEqual(table.Rows, wantRows) {
				t.Errorf("Rows = %#v, want %#v", table.Rows, wantRows)
			}
			if len(table.Cells) != 3 || len(table.Cells[0]) != 3 {
				t.Errorf("Cells shape = %d rows, first row has %d cells, want 3 rows of 3", len(table.Cells), len(table.Cells[0]))
			}
			for _, d := range linter.New().Lint(doc) {
				if d.RuleID == "TABLE003" {
					t.Errorf("unexpected TABLE003 (%q)", d.Message)
				}
			}
		})
	}
}

func TestDocumentStrictParser_TableBlockEdgePipes_SameASTAsWithoutThem(t *testing.T) {
	wantHeaders := []string{"Métrica", "Q2", "Q3"}
	wantRows := [][]string{{"Ingresos", "10", "12"}, {"Costos", "4", "5"}}

	for name, body := range edgePipeTableBodies {
		t.Run(name, func(t *testing.T) {
			src := "SECTION \"S\"\n\n" + body + "\n"
			doc, diags := parseStrictDoc(t, src)
			assertNoErrors(t, diags)
			table := firstTable(t, doc)
			if !reflect.DeepEqual(table.Headers, wantHeaders) {
				t.Errorf("Headers = %#v, want %#v", table.Headers, wantHeaders)
			}
			if !reflect.DeepEqual(table.Rows, wantRows) {
				t.Errorf("Rows = %#v, want %#v", table.Rows, wantRows)
			}
		})
	}
}

// Una celda vacía legítima, en medio o pegada al borde, sobrevive: sólo los
// pipes de borde dejan de producir columnas.
func TestStrictParser_TableBlockEdgePipes_LegitimateEmptyCellsSurvive(t *testing.T) {
	src := `SLIDE content
  title: "A"
  TABLE
    | a | b | c |
    |---|---|---|
    | 1 || 3 |
    |   | 2 | 3 |
    | 1 | 2 |   |
`
	doc, diags := NewStrictParser(src, util.NewNoop()).Parse()
	if n := countErrors(diags); n != 0 {
		t.Fatalf("got %d error diagnostics, want 0: %v", n, diags)
	}
	table := firstTable(t, doc)
	wantRows := [][]string{{"1", "", "3"}, {"", "2", "3"}, {"1", "2", ""}}
	if !reflect.DeepEqual(table.Headers, []string{"a", "b", "c"}) {
		t.Errorf("Headers = %#v", table.Headers)
	}
	if !reflect.DeepEqual(table.Rows, wantRows) {
		t.Errorf("Rows = %#v, want %#v", table.Rows, wantRows)
	}
}
