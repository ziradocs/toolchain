// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// La alineación de la fila delimitadora llena ColumnAlign, el documento pasa
// a 2.25.0 (table-align-v1) y fmt la conserva en los dos DSL. Una tabla con
// "---" a secas no cambia de versión.
func TestTableAlignRoundTrip(t *testing.T) {
	want := []string{"left", "center", "right", ""}
	for _, tc := range []struct {
		name, src string
		document  bool
	}{
		{"slidelang flex", "---\nmode: flex\n---\n# Deck\n\n## S\n\n| A | B | C | D |\n|:--|:-:|--:|---|\n| 1 | 2 | 3 | 4 |\n", false},
		{"doclang flex", "---\ntitle: D\n---\n# Doc\n\n| A | B | C | D |\n|:--|:-:|--:|---|\n| 1 | 2 | 3 | 4 |\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := mustParse(t, tc.src, tc.document)
			table := firstOfType[*ast.TableElement](t, doc)
			if !reflect.DeepEqual(table.ColumnAlign, want) {
				t.Fatalf("columnAlign = %#v", table.ColumnAlign)
			}
			if doc.SchemaVersion != ast.TableAlignSchemaVersion {
				t.Fatalf("version = %s", doc.SchemaVersion)
			}
			format := FormatStrict
			if tc.document {
				format = FormatDocument
			}
			out, err := format(doc)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "|:---|:---:|---:|---|") {
				t.Errorf("fmt dropped the alignment:\n%s", out)
			}
			again := firstOfType[*ast.TableElement](t, mustParse(t, out, tc.document))
			if !reflect.DeepEqual(again.ColumnAlign, want) {
				t.Fatalf("round trip = %#v\n%s", again.ColumnAlign, out)
			}
		})
	}
}

func TestTableWithoutAlignKeepsLegacyVersion(t *testing.T) {
	doc := mustParse(t, "---\ntitle: D\n---\n# Doc\n\n| A | B |\n|---|---|\n| 1 | 2 |\n", true)
	table := firstOfType[*ast.TableElement](t, doc)
	if table.ColumnAlign != nil || doc.SchemaVersion != ast.LegacySchemaVersion {
		t.Fatalf("columnAlign = %#v version = %s", table.ColumnAlign, doc.SchemaVersion)
	}
}

// Una alineación que la forma TABLE no puede escribir no se pierde en
// silencio: fmt la rechaza.
func TestTableAlignOutsidePipeFormIsRejected(t *testing.T) {
	doc := mustParse(t, "---\ntitle: D\n---\n# Doc\n\n| A | B |\n|:-:|---|\n| 1 | 2 |\n", true)
	firstOfType[*ast.TableElement](t, doc).Caption = "Con leyenda"
	_, err := FormatDocument(doc)
	var unsupported *UnsupportedElementError
	if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), "column alignment") || !strings.Contains(err.Error(), "a caption") {
		t.Fatalf("alignment with caption formatted: %v", err)
	}
}
