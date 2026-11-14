// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// Una fila con guiones en una celda ("| 2019 | --- |") es una fila de datos
// en los dos dialectos (elements.IsMarkdownSeparatorLine). fmt la escribe
// tal cual después de su propia fila separadora, así que al releerla sigue
// siendo la misma fila, y un segundo fmt da el mismo texto. Se prueba con y
// sin separador en la fuente, con la fila como primera línea de la tabla,
// con un separador solo de dos puntos y con una fila "|" sin celdas.
func TestTableDashCellRoundTrip(t *testing.T) {
	tables := map[string]struct {
		src     string
		headers []string
		rows    [][]string
	}{
		"after separator": {"| Año | Valor |\n| --- | --- |\n| 2019 | --- |\n| a --- b | c |\n", []string{"Año", "Valor"}, [][]string{{"2019", "---"}, {"a --- b", "c"}}},
		"no separator":    {"| Año | Valor |\n| 2019 | --- |\n| 2020 | 5 |\n", []string{"Año", "Valor"}, [][]string{{"2019", "---"}, {"2020", "5"}}},
		"first line":      {"| 2019 | --- |\n| --- | --- |\n| 2020 | 5 |\n", []string{"2019", "---"}, [][]string{{"2020", "5"}}},
		"colon separator": {"| A | B |\n|:--|--:|\n| :-- | x |\n", []string{"A", "B"}, [][]string{{":--", "x"}}},
		// Una fila "|" sola es una fila sin celdas, y fmt la escribe así.
		"bare pipe row": {"| A | B |\n| --- | --- |\n|\n| 1 | 2 |\n", []string{"A", "B"}, [][]string{{}, {"1", "2"}}},
	}
	indent := func(s string) string {
		var b strings.Builder
		for _, l := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
			b.WriteString("  " + l + "\n")
		}
		return b.String()
	}
	type dialect struct {
		name     string
		document bool
		wrap     func(string) string
		format   func(*ast.AST) (string, error)
	}
	dialects := []dialect{
		{"doclang flex", true, func(s string) string { return "---\ntitle: D\n---\n# Doc\n\n" + s }, FormatDocument},
		{"doclang flex :::note", true, func(s string) string { return "---\ntitle: D\n---\n# Doc\n\n:::note\n" + s + ":::\n" }, FormatDocument},
		{"doclang flex to strict", true, func(s string) string { return "---\ntitle: D\n---\n# Doc\n\n" + s }, FormatDocumentStrict},
		{"doclang strict", true, func(s string) string { return "---\nmode: strict\ntitle: D\n---\nSECTION \"S\"\n" + indent(s) }, FormatDocumentStrict},
		{"slidelang flex", false, func(s string) string { return "---\nmode: flex\n---\n# Deck\n\n## S\n\n" + s }, FormatStrict},
		{"slidelang strict", false, func(s string) string { return "---\nmode: strict\n---\nSLIDE content\n  title: \"T\"\n" + indent(s) }, FormatStrict},
	}
	for _, d := range dialects {
		for name, table := range tables {
			t.Run(d.name+"/"+name, func(t *testing.T) {
				doc := mustParse(t, d.wrap(table.src), d.document)
				before := firstOfType[*ast.TableElement](t, doc)
				if !reflect.DeepEqual(before.Headers, table.headers) || !reflect.DeepEqual(before.Rows, table.rows) {
					t.Fatalf("parse = %q %q, want %q %q", before.Headers, before.Rows, table.headers, table.rows)
				}
				out, err := d.format(doc)
				if err != nil {
					t.Fatal(err)
				}
				again := mustParse(t, out, d.document)
				after := firstOfType[*ast.TableElement](t, again)
				if !reflect.DeepEqual(before.Headers, after.Headers) || !reflect.DeepEqual(before.Rows, after.Rows) || !reflect.DeepEqual(before.ColumnAlign, after.ColumnAlign) {
					t.Fatalf("round trip changed the table:\nbefore %q %q %q\nafter  %q %q %q\n%s",
						before.Headers, before.Rows, before.ColumnAlign, after.Headers, after.Rows, after.ColumnAlign, out)
				}
				twice, err := d.format(again)
				if err != nil {
					t.Fatal(err)
				}
				if twice != out {
					t.Fatalf("fmt is not idempotent:\n%s\n---\n%s", out, twice)
				}
			})
		}
	}
}
