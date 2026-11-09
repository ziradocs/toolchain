// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

func strictDocTable(t *testing.T, body string) (*ast.TableElement, []diagnostics.Diagnostic) {
	t.Helper()
	src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  TABLE\n    headers: [\"x\", \"y\"]\n" + body
	doc, diags := New(util.NewNoop()).ParseDocument(src, "t.doclang")
	for _, el := range doc.ContentBlocks[0].Elements {
		if table, ok := el.(*ast.TableElement); ok {
			return table, diags
		}
	}
	t.Fatalf("no table element; diagnostics %v", diags)
	return nil, nil
}

// Issue #488: `rows:` con el arreglo entero en una sola línea da las mismas
// filas (y las mismas celdas) que la forma de una fila por línea.
func TestTableInlineRows_SameRowsAsMultiLine(t *testing.T) {
	for _, tc := range []struct {
		name, inline, multi string
	}{
		{"two rows", `    rows: [["a","b"],["c","d"]]` + "\n", "    rows: [\n      [\"a\",\"b\"],\n      [\"c\",\"d\"]\n    ]\n"},
		{"one row", `    rows: [["a","b"]]` + "\n", "    rows: [\n      [\"a\",\"b\"]\n    ]\n"},
		{"spacing", `    rows: [ ["a", "b"] , ["c", "d"] ]` + "\n", "    rows: [\n      [\"a\", \"b\"],\n      [\"c\", \"d\"]\n    ]\n"},
		{"commas and brackets inside quotes", `    rows: [["a, b","[x]"],["c\","d"]]` + "\n", "    rows: [\n      [\"a, b\",\"[x]\"],\n      [\"c\\\",\"d\"]\n    ]\n"},
		{"unquoted cells", `    rows: [[a, b], [c, d]]` + "\n", "    rows: [\n      [a, b],\n      [c, d]\n    ]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inline, diags := strictDocTable(t, tc.inline)
			if len(diags) != 0 {
				t.Fatalf("inline: want no diagnostics, got %v", diags)
			}
			multi, diags := strictDocTable(t, tc.multi)
			if len(diags) != 0 {
				t.Fatalf("multi-line: want no diagnostics, got %v", diags)
			}
			if len(inline.Rows) == 0 {
				t.Fatal("inline rows were dropped")
			}
			if !reflect.DeepEqual(inline.Rows, multi.Rows) || !reflect.DeepEqual(inline.Cells, multi.Cells) || !reflect.DeepEqual(inline.Headers, multi.Headers) {
				t.Errorf("inline and multi-line differ:\ninline rows %q\nmulti rows  %q", inline.Rows, multi.Rows)
			}
			for _, pos := range inline.RowPositions {
				if pos.Line != 9 {
					t.Errorf("an inline row is positioned at line %d, want the rows: line 9", pos.Line)
				}
			}
		})
	}
}

func TestTableInlineRows_TheIssueRepro(t *testing.T) {
	table, diags := strictDocTable(t, `    rows: [["a","b"],["c","d"]]`+"\n")
	if len(diags) != 0 {
		t.Fatalf("want no diagnostics, got %v", diags)
	}
	if want := [][]string{{"a", "b"}, {"c", "d"}}; !reflect.DeepEqual(table.Rows, want) {
		t.Errorf("rows %q, want %q", table.Rows, want)
	}
}

// `rows: []` sigue siendo una tabla sin filas declarada a propósito: no hay
// nada que no se haya podido leer, así que no hay TABLE007 (el linter sigue
// dando TABLE002).
func TestTableInlineRows_EmptyArrayIsNoRows(t *testing.T) {
	table, diags := strictDocTable(t, "    rows: []\n")
	if len(diags) != 0 || len(table.Rows) != 0 {
		t.Errorf("want zero rows and no diagnostics, got %q %v", table.Rows, diags)
	}
}

// Un valor en línea que no se puede leer es un error en strict, con su línea
// y el código TABLE007, en vez de una tabla vacía con solo el TABLE002.
func TestTableInlineRows_MalformedIsAnErrorInStrict(t *testing.T) {
	for name, body := range map[string]string{
		"unclosed":           `    rows: [["a","b"],["c","d"]` + "\n",
		"open row":           `    rows: [["a","b"],["c"` + "\n",
		"not an array":       "    rows: a, b\n",
		"bare cells":         `    rows: ["a","b"]` + "\n",
		"missing comma":      `    rows: [["a","b"] ["c","d"]]` + "\n",
		"trailing comma":     `    rows: [["a","b"],]` + "\n",
		"unclosed quote":     `    rows: [["a,"b"]]` + "\n",
		"inline plus a line": `    rows: [["a","b"]]` + "\n" + `      ["c","d"]` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			table, diags := strictDocTable(t, body)
			if len(diags) != 1 || !diags[0].IsError() || diags[0].RuleID != "TABLE007" || diags[0].Position.Line != 9 {
				t.Fatalf("want one TABLE007 error at line 9, got %v", diags)
			}
			if !strings.Contains(diags[0].Message, "rows:") {
				t.Errorf("the message should name rows:, got %q", diags[0].Message)
			}
			if len(table.Rows) != 0 {
				t.Errorf("a malformed value must not produce partial rows, got %q", table.Rows)
			}
		})
	}
}

// Los dos dialectos comparten el parser de TABLE: en un slide strict también
// se leen las filas en línea, y fuera de strict el valor ilegible es un warning.
func TestTableInlineRows_OtherDialects(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TABLE\n    headers: [\"x\", \"y\"]\n    rows: [[\"a\",\"b\"],[\"c\",\"d\"]]\n"
	doc, diags := New(util.NewNoop()).Parse(src, "t.slidelang")
	if len(diags) != 0 {
		t.Fatalf("slide: want no diagnostics, got %v", diags)
	}
	table := doc.ContentBlocks[0].Elements[0].(*ast.TableElement)
	if want := [][]string{{"a", "b"}, {"c", "d"}}; !reflect.DeepEqual(table.Rows, want) {
		t.Errorf("slide rows %q, want %q", table.Rows, want)
	}

	src = "# A\n\nTABLE\n  headers: [\"x\", \"y\"]\n  rows: [[\"a\",\"b\"],[\"c\",\"d\"]]\n"
	doc, diags = New(util.NewNoop()).ParseDocument(src, "t.doclang")
	if len(diags) != 0 {
		t.Fatalf("flex: want no diagnostics, got %v", diags)
	}
	var flexTable *ast.TableElement
	for _, el := range doc.ContentBlocks[0].Elements {
		if tb, ok := el.(*ast.TableElement); ok {
			flexTable = tb
		}
	}
	if flexTable == nil || !reflect.DeepEqual(flexTable.Rows, [][]string{{"a", "b"}, {"c", "d"}}) {
		t.Fatalf("flex rows: got %#v", flexTable)
	}

	src = "# A\n\nTABLE\n  headers: [\"x\", \"y\"]\n  rows: [[\"a\",\"b\"],\n"
	_, diags = New(util.NewNoop()).ParseDocument(src, "t.doclang")
	if len(diags) != 1 || diags[0].IsError() || diags[0].RuleID != "TABLE007" || diags[0].Position.Line != 5 {
		t.Errorf("flex: want one TABLE007 warning at line 5, got %v", diags)
	}
}
