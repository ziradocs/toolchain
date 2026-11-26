// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer_test

import (
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/formatter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// Una segunda fila de encabezado de cells: con menos celdas que la primera se
// completa con celdas de encabezado: en el AST, en la fuente que escribe fmt
// y en el HTML (<th>, no <td>).
func TestShortHeaderCellsRowIsCompletedWithHeaderCells(t *testing.T) {
	src := "# Doc\n\nTABLE\n  cells:\n" +
		"    - [{content: \"A\", header: true}, {content: \"B\", header: true}, {content: \"C\", header: true}]\n" +
		"    - [{content: \"D\", header: true}, {content: \"E\", header: true}]\n" +
		"    - [{content: \"x\"}, {content: \"y\"}, {content: \"z\"}]\n"
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "t.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d)
		}
	}
	var table *ast.TableElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if tbl, ok := n.(*ast.TableElement); ok && table == nil {
			table = tbl
		}
		return nil
	})
	if table == nil {
		t.Fatal("no table")
		return
	}
	if len(table.Cells) != 3 || len(table.Cells[1]) != 3 || table.Cells[1][2] != (ast.TableCell{IsHeader: true}) {
		t.Fatalf("second header row not completed with a header cell: %+v", table.Cells)
	}
	if len(table.Cells[2]) != 3 || table.Cells[2][2].IsHeader {
		t.Fatalf("data row changed: %+v", table.Cells[2])
	}

	source, err := formatter.FormatDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, `- [{content: D, header: true}, {content: E, header: true}, {content: "", header: true}]`) {
		t.Fatalf("fmt does not write the completed header row:\n%s", source)
	}
	again, diags := parser.New(util.NewNoop()).ParseDocument(source, "t.doclang")
	for _, d := range diags {
		if d.RuleID == "TABLE010" || d.IsError() {
			t.Fatalf("fmt output reads back with %v:\n%s", d, source)
		}
	}

	html := renderer.GenerateDocumentHTML(again, renderer.DocumentHTMLOptions{}, renderer.NewDefaultRenderContext())
	tags := regexp.MustCompile(`<(th|td)\b[^>]*>([^<]*)</(?:th|td)>`).FindAllStringSubmatch(html, -1)
	var got []string
	for _, m := range tags {
		got = append(got, m[1]+":"+m[2])
	}
	want := "th:A th:B th:C th:D th:E th: td:x td:y td:z"
	if strings.Join(got, " ") != want {
		t.Fatalf("table cells in HTML = %q, want %q", strings.Join(got, " "), want)
	}
}
