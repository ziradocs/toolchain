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
// camino de main: `rows: []`, `rows: # …`, `rows:` solo y un arreglo plano,
// con filas debajo, leen las filas de abajo.
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

// Un arreglo anidado en la línea de `rows:` más filas debajo: se leen las
// dos, primero las de la línea.
func TestTableInlineRows_InlineAndRowsBelowReadBoth(t *testing.T) {
	tbl, diags := parseInlineRowsTable(t, "document", "    headers: [\"x\", \"y\"]\n    rows: [[\"a\", \"b\"]]\n      [\"c\", \"d\"]\n")
	if want := [][]string{{"a", "b"}, {"c", "d"}}; !reflect.DeepEqual(tbl.Rows, want) {
		t.Errorf("rows = %q, want %q", tbl.Rows, want)
	}
	if n, _ := ruleIDCount(diags, "TABLE007"); n != 0 {
		t.Errorf("unexpected TABLE007: %v", diags)
	}
}

// Celdas entre comillas simples (el estilo de un repr de Python) no se leen:
// en strict la comilla simple es contenido y los apóstrofos quedarían en la
// celda. Sale TABLE007 pidiendo comillas dobles. Un apóstrofo dentro de una
// celda sí se lee.
func TestTableInlineRows_SingleQuotesAskForDoubleQuotes(t *testing.T) {
	for _, value := range []string{`[['a', 'b'], ['c', 'd']]`, `[["a", 'b']]`, `[['a, b', 'c']]`} {
		tbl, diags := parseInlineRowsTable(t, "document", "    headers: [\"x\", \"y\"]\n    rows: "+value+"\n")
		if len(tbl.Rows) != 0 {
			t.Errorf("%s: rows = %q, want none", value, tbl.Rows)
		}
		found := false
		for _, d := range diags {
			if d.RuleID == "TABLE007" && d.IsWarning() && strings.Contains(d.Message, "double quotes") && strings.Contains(d.Message, "single quotes") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want a TABLE007 warning about single quotes, got %v", value, diags)
		}
	}
	for value, want := range map[string][][]string{
		`[["O'Brien", "x"]]`: {{"O'Brien", "x"}},
		`[[O'Brien, x]]`:     {{"O'Brien", "x"}},
	} {
		tbl, _ := parseInlineRowsTable(t, "document", "    headers: [\"x\", \"y\"]\n    rows: "+value+"\n")
		if !reflect.DeepEqual(tbl.Rows, want) {
			t.Errorf("%s: rows = %q, want %q", value, tbl.Rows, want)
		}
	}
}

// En flex, un bloque TABLE con `rows:` en línea se lee igual.
func TestTableInlineRows_Flex(t *testing.T) {
	src := "---\nmode: flex\ntitle: \"D\"\n---\n\n# A\n\nTABLE\n  headers: [\"x\", \"y\"]\n  rows: [[\"a\",\"b\"],[\"c\",\"d\"]]\n"
	for _, parse := range []func(string, string) (*ast.AST, []diagnostics.Diagnostic){New(util.NewNoop()).ParseDocument, New(util.NewNoop()).Parse} {
		doc, diags := parse(src, "d")
		var rows [][]string
		for _, b := range doc.ContentBlocks {
			for _, el := range b.Elements {
				if tbl, ok := el.(*ast.TableElement); ok {
					rows = tbl.Rows
				}
			}
		}
		if want := [][]string{{"a", "b"}, {"c", "d"}}; !reflect.DeepEqual(rows, want) {
			t.Errorf("flex: rows = %q, want %q (%v)", rows, want, diags)
		}
	}
}
