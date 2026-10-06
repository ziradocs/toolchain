// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

func firstTableOf(t *testing.T, doc *ast.AST) *ast.TableElement {
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

// Fuentes strict con la tabla escrita con pipes de borde dentro de un TABLE
// (https://github.com/ziradocs/toolchain/issues/379). Antes del arreglo el
// parser inflaba la tabla a cinco columnas y `fmt` escribía esas cinco.
const edgePipeSlideSrc = `---
mode: strict
title: "T"
---

SLIDE content
  title: "A"
  TABLE
    | Métrica | Q2 | Q3 |
    |---------|----|----|
    | Ingresos | 10 |  |
    | Costos || 5 |
`

const edgePipeDocSrc = `---
mode: strict
title: "T"
---

SECTION "S"

  TABLE
    | Métrica | Q2 | Q3 |
    |---------|----|----|
    | Ingresos | 10 |  |
    | Costos || 5 |
`

var (
	edgePipeWantHeaders = []string{"Métrica", "Q2", "Q3"}
	edgePipeWantRows    = [][]string{{"Ingresos", "10", ""}, {"Costos", "", "5"}}
)

func TestFormatStrict_TableBlockEdgePipes_KeepsColumnsAndIsIdempotent(t *testing.T) {
	doc, diags := parser.New(util.NewNoop()).Parse(edgePipeSlideSrc, "t.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d)
		}
	}
	table := firstTableOf(t, doc)
	if !reflect.DeepEqual(table.Headers, edgePipeWantHeaders) || !reflect.DeepEqual(table.Rows, edgePipeWantRows) {
		t.Fatalf("parsed Headers = %#v, Rows = %#v", table.Headers, table.Rows)
	}

	first, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if !strings.Contains(first, "| Métrica | Q2 | Q3 |\n") || !strings.Contains(first, "|---|---|---|\n") {
		t.Errorf("formatted table does not have three columns:\n%s", first)
	}
	if strings.Contains(first, "---------") {
		t.Errorf("the delimiter row survived as a data row:\n%s", first)
	}

	reparsed, diags := parser.New(util.NewNoop()).Parse(first, "t.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("reparse error: %v\n%s", d, first)
		}
	}
	reTable := firstTableOf(t, reparsed)
	if !reflect.DeepEqual(reTable.Headers, edgePipeWantHeaders) || !reflect.DeepEqual(reTable.Rows, edgePipeWantRows) {
		t.Errorf("reparsed Headers = %#v, Rows = %#v", reTable.Headers, reTable.Rows)
	}

	second, err := FormatStrict(reparsed)
	if err != nil {
		t.Fatalf("second FormatStrict: %v", err)
	}
	if first != second {
		t.Errorf("fmt is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// Una fila de datos de guiones ("n/a" como "| - | - |") no es la fila
// delimitadora: sólo lo es la que sigue inmediatamente al header. Tiene que
// sobrevivir al parseo del TABLE y al ciclo fmt -> reparse.
func TestFormatStrict_TableBlockDashDataRow_SurvivesRoundTrip(t *testing.T) {
	src := `---
mode: strict
title: "T"
---

SLIDE content
  title: "A"
  TABLE
    | a | b |
    |---|---|
    | 1 | 2 |
    | - | - |
`
	want := [][]string{{"1", "2"}, {"-", "-"}}

	doc, diags := parser.New(util.NewNoop()).Parse(src, "t.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d)
		}
	}
	if table := firstTableOf(t, doc); !reflect.DeepEqual(table.Rows, want) {
		t.Fatalf("parsed Rows = %#v, want %#v", table.Rows, want)
	}

	first, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	reparsed, diags := parser.New(util.NewNoop()).Parse(first, "t.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("reparse error: %v\n%s", d, first)
		}
	}
	if table := firstTableOf(t, reparsed); !reflect.DeepEqual(table.Rows, want) {
		t.Errorf("reparsed Rows = %#v, want %#v\n%s", table.Rows, want, first)
	}
	second, err := FormatStrict(reparsed)
	if err != nil {
		t.Fatalf("second FormatStrict: %v", err)
	}
	if first != second {
		t.Errorf("fmt is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestFormatDocumentStrict_TableBlockEdgePipes_KeepsColumnsAndIsIdempotent(t *testing.T) {
	doc, diags := parser.New(util.NewNoop()).ParseDocument(edgePipeDocSrc, "t.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d)
		}
	}
	table := firstTableOf(t, doc)
	if !reflect.DeepEqual(table.Headers, edgePipeWantHeaders) || !reflect.DeepEqual(table.Rows, edgePipeWantRows) {
		t.Fatalf("parsed Headers = %#v, Rows = %#v", table.Headers, table.Rows)
	}

	first, err := FormatDocumentStrict(doc)
	if err != nil {
		t.Fatalf("FormatDocumentStrict: %v", err)
	}
	if !strings.Contains(first, "| Métrica | Q2 | Q3 |\n") {
		t.Errorf("formatted table does not have three columns:\n%s", first)
	}

	reparsed, diags := parser.New(util.NewNoop()).ParseDocument(first, "t.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("reparse error: %v\n%s", d, first)
		}
	}
	reTable := firstTableOf(t, reparsed)
	if !reflect.DeepEqual(reTable.Headers, edgePipeWantHeaders) || !reflect.DeepEqual(reTable.Rows, edgePipeWantRows) {
		t.Errorf("reparsed Headers = %#v, Rows = %#v", reTable.Headers, reTable.Rows)
	}

	second, err := FormatDocumentStrict(reparsed)
	if err != nil {
		t.Fatalf("second FormatDocumentStrict: %v", err)
	}
	if first != second {
		t.Errorf("fmt is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
