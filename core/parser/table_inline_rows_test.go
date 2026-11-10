// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

func parseInlineRowsTable(t *testing.T, dialect, body string) (*ast.TableElement, []diagnostics.Diagnostic) {
	t.Helper()
	p := New(util.NewNoop())
	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if dialect == "document" {
		doc, diags = p.ParseDocument("---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TABLE\n"+body, "d.doclang")
	} else {
		doc, diags = p.Parse("---\nmode: strict\ntitle: \"D\"\n---\n\nSLIDE content\n  title: \"T\"\n  TABLE\n"+body, "d.slidelang")
	}
	if doc == nil || len(doc.ContentBlocks) == 0 {
		t.Fatalf("no content: %v", diags)
	}
	for _, el := range doc.ContentBlocks[0].Elements {
		if tbl, ok := el.(*ast.TableElement); ok {
			return tbl, diags
		}
	}
	t.Fatalf("no table: %v", diags)
	return nil, nil
}

func ruleIDCount(diags []diagnostics.Diagnostic, id string) (n int, severity diagnostics.Severity) {
	for _, d := range diags {
		if d.RuleID == id {
			n++
			severity = d.Severity
		}
	}
	return n, severity
}

// Issue #488: `rows:` con un arreglo anidado en la misma línea se lee como
// filas, en documentos y en slides.
func TestTableInlineRows_NestedArrayOnTheRowsLine(t *testing.T) {
	cases := []struct {
		value string
		want  [][]string
	}{
		{`[["a","b"],["c","d"]]`, [][]string{{"a", "b"}, {"c", "d"}}},
		{`[["a","b"]]`, [][]string{{"a", "b"}}},
		{`[ ["a", "b"] , [c, d] ]`, [][]string{{"a", "b"}, {"c", "d"}}},
		{`[["Manual, dual", "1,000"], ["[borrador]", "x]y"]]`, [][]string{{"Manual, dual", "1,000"}, {"[borrador]", "x]y"}}},
		{`[["a", "b"]]  # comentario`, [][]string{{"a", "b"}}},
		{`[["C:\", "b"]]`, [][]string{{`C:\`, "b"}}},
	}
	for _, dialect := range []string{"document", "slides"} {
		for _, c := range cases {
			tbl, diags := parseInlineRowsTable(t, dialect, "    headers: [\"x\", \"y\"]\n    rows: "+c.value+"\n")
			if !reflect.DeepEqual(tbl.Rows, c.want) {
				t.Errorf("%s %s: rows = %q, want %q", dialect, c.value, tbl.Rows, c.want)
			}
			if n, _ := ruleIDCount(diags, "TABLE007"); n != 0 {
				t.Errorf("%s %s: unexpected TABLE007: %v", dialect, c.value, diags)
			}
		}
	}
}

// Un valor en línea que no se puede cortar en filas sin adivinar (comillas
// sueltas, texto entre filas o después) no da ninguna fila y avisa con
// TABLE007, que es warning también en documento strict. Nunca da un corte
// distinto del que el autor escribió.
func TestTableInlineRows_AmbiguousValueIsReportedNotGuessed(t *testing.T) {
	for _, value := range []string{
		`[["Monitor 27\"", "1"], ["Monitor 24\"", "2"], ["Teclado", "3"]]`,
		`[["a", "b"], ["c", "d"`,
		`[["a", "b"] ["c", "d"]]`,
		`[["a", "b"]] basura`,
		`[["a" "b"]]`,
		`[["a"],]`,
		`[[a]b, c]]`,
	} {
		for _, dialect := range []string{"document", "slides"} {
			tbl, diags := parseInlineRowsTable(t, dialect, "    headers: [\"x\", \"y\"]\n    rows: "+value+"\n")
			if len(tbl.Rows) != 0 {
				t.Errorf("%s %s: rows = %q, want none", dialect, value, tbl.Rows)
			}
			if n, sev := ruleIDCount(diags, "TABLE007"); n != 1 || sev != diagnostics.Warning {
				t.Errorf("%s %s: want one TABLE007 warning, got %v", dialect, value, diags)
			}
		}
	}
}

// Todo lo que no es un arreglo anidado en la línea de `rows:` sigue el
// camino de main: `rows: []` y `rows: # …` con filas debajo leen las filas
// de abajo, y un arreglo en línea más filas debajo lee las dos.
func TestTableInlineRows_OtherValuesKeepMainPath(t *testing.T) {
	below := "      [\"c\", \"d\"]\n"
	cases := []struct {
		rows string
		want [][]string
	}{
		{"    rows: []\n" + below, [][]string{{"c", "d"}}},
		{"    rows: # filas\n" + below, [][]string{{"c", "d"}}},
		{"    rows:\n" + below, [][]string{{"c", "d"}}},
		{"    rows: [\"a\", \"b\"]\n" + below, [][]string{{"c", "d"}}},
		{"    rows: [[\"a\", \"b\"]]\n" + below, [][]string{{"a", "b"}, {"c", "d"}}},
	}
	for _, c := range cases {
		tbl, diags := parseInlineRowsTable(t, "document", "    headers: [\"x\", \"y\"]\n"+c.rows)
		if !reflect.DeepEqual(tbl.Rows, c.want) {
			t.Errorf("%q: rows = %q, want %q", c.rows, tbl.Rows, c.want)
		}
		if n, _ := ruleIDCount(diags, "TABLE007"); n != 0 {
			t.Errorf("%q: unexpected TABLE007: %v", c.rows, diags)
		}
	}
}
