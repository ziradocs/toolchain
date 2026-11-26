// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Una fila de un bloque cells: sin spans con menos celdas que el encabezado
// se completa en Cells y avisa TABLE010, como las demás formas. Hasta ahora
// FlattenCellsToRows dejaba Rows rectangular, PadShortTableRows no veía
// ninguna fila corta, y Cells (lo que escribe el HTML) seguía con una celda
// menos: dos <th> sobre un solo <td>.

const shortCellsBody = "TABLE\n  cells:\n    - [{content: \"A\", header: true}, {content: \"B\", header: true}]\n    - [{content: \"p\"}]\n"

func TestShortCellsRowIsPaddedWithAWarning(t *testing.T) {
	for _, tc := range []struct {
		name     string
		src      string
		document bool
	}{
		{"doclang without frontmatter", "# Doc\n\n" + shortCellsBody, true},
		{"doclang strict", "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n" + indentLines(shortCellsBody, 2), true},
		{"slidelang flex", "---\nmode: flex\ntitle: \"T\"\n---\n\n# Deck\n\n## S\n\n" + shortCellsBody, false},
		{"slidelang strict", "---\nmode: strict\ntitle: \"T\"\n---\n\nSLIDE content\n  title: \"T\"\n" + indentLines(shortCellsBody, 2), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if errs := buildErrors(tc.src, tc.document); len(errs) > 0 {
				t.Fatalf("build errors: %v\n%s", errs, tc.src)
			}
			p := parser.New(util.NewNoop())
			var doc *ast.AST
			var diags []diagnostics.Diagnostic
			if tc.document {
				doc, diags = p.ParseDocument(tc.src, "t.doclang")
			} else {
				doc, diags = p.Parse(tc.src, "t.slidelang")
			}
			table := findTable(doc)
			if table == nil {
				t.Fatalf("no table\n%s", tc.src)
				return
			}
			if !reflect.DeepEqual(table.Rows, [][]string{{"p", ""}}) {
				t.Errorf("rows %q", table.Rows)
			}
			if len(table.Cells) != 2 || len(table.Cells[1]) != 2 || table.Cells[1][1] != (ast.TableCell{}) {
				t.Errorf("cells row not completed: %+v", table.Cells)
			}
			// cells: no lleva la posición de cada fila: el aviso va en la
			// línea del TABLE, como el de una fila corta por sus spans.
			var got []diagnostics.Diagnostic
			for _, d := range diags {
				if d.RuleID == "TABLE010" {
					got = append(got, d)
				}
			}
			lines := strings.Split(tc.src, "\n")
			if len(got) != 1 || !got[0].IsWarning() || got[0].Position != table.Position || strings.TrimSpace(lines[got[0].Position.Line-1]) != "TABLE" || !strings.Contains(got[0].Message, "has 1 cell where the header has 2") {
				t.Fatalf("want one TABLE010 warning on the TABLE line, got %v", got)
			}
			for _, d := range lintDiags(t, tc.src, tc.document) {
				if strings.HasPrefix(d.RuleID, "TABLE") {
					t.Errorf("the linter reports the table: %v", d)
				}
			}
		})
	}
}

// Una fila que cubre el ancho del encabezado con sus spans no es corta: no se
// toca ni se avisa. Una fila con MÁS celdas que el encabezado tampoco cambia.
func TestCellsRowsThatAreNotShortStayAsWritten(t *testing.T) {
	header3 := "    - [{content: \"A\", header: true}, {content: \"B\", header: true}, {content: \"C\", header: true}]\n"
	for _, tc := range []struct {
		name  string
		rows  string
		cells []int // celdas que debe tener cada fila de cuerpo de Cells
	}{
		{"colspan that fills the row", "    - [{content: \"p\", colspan: 3}]\n", []int{1}},
		{"colspan plus a cell", "    - [{content: \"p\", colspan: 2}, {content: \"q\"}]\n", []int{2}},
		{"rowspan that reaches the next row", "    - [{content: \"p\", rowspan: 2}, {content: \"q\"}, {content: \"r\"}]\n    - [{content: \"s\"}, {content: \"t\"}]\n", []int{3, 2}},
		{"rowspan and colspan", "    - [{content: \"p\", rowspan: 2}, {content: \"q\", colspan: 2}]\n    - [{content: \"s\", colspan: 2}]\n", []int{2, 1}},
		{"more cells than the header", "    - [{content: \"p\"}, {content: \"q\"}, {content: \"r\"}, {content: \"s\"}]\n", []int{4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "# Doc\n\nTABLE\n  cells:\n" + header3 + tc.rows
			doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "t.doclang")
			for _, d := range diags {
				if d.RuleID == "TABLE010" || d.IsError() {
					t.Fatalf("unexpected diagnostic: %v", d)
				}
			}
			table := findTable(doc)
			if table == nil || len(table.Cells) != len(tc.cells)+1 {
				t.Fatalf("table %+v", table)
			}
			for i, n := range tc.cells {
				if len(table.Cells[i+1]) != n {
					t.Errorf("cells row %d has %d cells, want %d: %+v", i, len(table.Cells[i+1]), n, table.Cells[i+1])
				}
			}
		})
	}
}

// Con una fila más larga que el encabezado en la misma tabla, Headers llega
// al ancho de esa fila (FlattenCellsToRows), pero la fila corta se completa
// hasta la fila de encabezado de Cells: el aviso dice ese ancho, el mismo
// contra el que se midió y se rellenó, en el parser y por DecodeAST.
func TestShortCellsRowNextToALongRowNamesTheHeaderWidth(t *testing.T) {
	src := "# Doc\n\nTABLE\n  cells:\n    - [{content: \"A\", header: true}, {content: \"B\", header: true}]\n    - [{content: \"x\"}, {content: \"y\"}, {content: \"z\"}]\n    - [{content: \"p\"}]\n"
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "t.doclang")
	table := findTable(doc)
	if table == nil {
		t.Fatal("no table")
		return
	}
	if len(table.Headers) != 3 || len(table.Cells[2]) != 2 || len(table.Cells[1]) != 3 {
		t.Fatalf("headers %q cells %+v", table.Headers, table.Cells)
	}
	const msg = "has 1 cell where the header has 2"
	var short []diagnostics.Diagnostic
	for _, d := range diags {
		if d.RuleID == "TABLE010" {
			short = append(short, d)
		}
	}
	if len(short) != 1 || !strings.Contains(short[0].Message, msg) {
		t.Fatalf("want one TABLE010 that says %q, got %v", msg, short)
	}

	tbl := ast.NewTableElement(diagnostics.NewPosition(3, 1))
	tbl.Cells = [][]ast.TableCell{{{Content: "A", IsHeader: true}, {Content: "B", IsHeader: true}}, {{Content: "x"}, {Content: "y"}, {Content: "z"}}, {{Content: "p"}}}
	tbl.Headers, tbl.Rows = ast.FlattenCellsToRows(tbl.Cells)
	in := ast.NewAST(diagnostics.NewPosition(1, 1))
	in.ContentBlocks = []ast.ContentBlock{{Elements: []ast.Element{tbl}}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ast.DecodeAST(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := findTable(decoded); len(got.Cells[2]) != 2 || got.DecodePaddedRowWidth(1) != 2 {
		t.Fatalf("cells %+v width %d", got.Cells, got.DecodePaddedRowWidth(1))
	}
	var lint []diagnostics.Diagnostic
	for _, d := range (&ElementStructureRule{}).Check(&decoded.ContentBlocks[0]) {
		if d.RuleID == "TABLE010" {
			lint = append(lint, d)
		}
	}
	if len(lint) != 1 || !strings.Contains(lint[0].Message, msg) {
		t.Fatalf("want one linter TABLE010 that says %q, got %v", msg, lint)
	}
	// Por una cadena de filtros el ancho viaja con la anotación.
	raw, err = json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	next, err := ast.DecodeAST(raw)
	if err != nil {
		t.Fatal(err)
	}
	ast.CarryDecodePaddedRows(decoded, next)
	if w := findTable(next).DecodePaddedRowWidth(1); w != 2 {
		t.Fatalf("carried width %d, want 2", w)
	}
}

// Un AST en JSON (la salida de un --filter) con una fila de Cells corta y
// Rows ya completa, como la deja FlattenCellsToRows: DecodeAST completa Cells
// y el linter da TABLE010.
func TestDecodeASTPadsShortCellsRow(t *testing.T) {
	tbl := ast.NewTableElement(diagnostics.NewPosition(3, 1))
	tbl.Cells = [][]ast.TableCell{{{Content: "A", IsHeader: true}, {Content: "B", IsHeader: true}}, {{Content: "p"}}}
	tbl.Headers, tbl.Rows = ast.FlattenCellsToRows(tbl.Cells)
	doc := ast.NewAST(diagnostics.NewPosition(1, 1))
	doc.ContentBlocks = []ast.ContentBlock{{Elements: []ast.Element{tbl}}}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ast.DecodeAST(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := findTable(decoded)
	if len(got.Cells[1]) != 2 || !reflect.DeepEqual(got.Rows, [][]string{{"p", ""}}) {
		t.Fatalf("DecodeAST did not complete the cells row: rows %q cells %+v", got.Rows, got.Cells)
	}
	if !reflect.DeepEqual(got.DecodePaddedRows(), map[int]int{0: 1}) {
		t.Fatalf("padded rows %v", got.DecodePaddedRows())
	}
	var short []diagnostics.Diagnostic
	for _, d := range (&ElementStructureRule{}).Check(&decoded.ContentBlocks[0]) {
		if strings.HasPrefix(d.RuleID, "TABLE") {
			short = append(short, d)
		}
	}
	if len(short) != 1 || short[0].RuleID != "TABLE010" || short[0].Position.Line != 3 {
		t.Fatalf("want one TABLE010 on line 3, got %v", short)
	}
}
