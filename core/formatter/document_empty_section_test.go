// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Una sección flex vacía que no es la primera sobrevive a fmt, en flex y en
// strict, con y sin node-id. Antes el parser la descartaba: `fmt` la borraba
// de la fuente y, con node-id, FormatDocument fallaba al reubicarlo
// (el AST formateado tenía menos nodos que el original).

type sectionShape struct {
	Title    string
	Elements int
	NodeID   string
}

func sectionShapes(doc *ast.AST) []sectionShape {
	var out []sectionShape
	for _, b := range doc.ContentBlocks {
		title := b.Title
		if title == "" {
			title = b.Heading
		}
		out = append(out, sectionShape{title, len(b.Elements), b.NodeID})
	}
	return out
}

func parseDocumentWithIDs(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("diagnóstico de error: %v\n--- fuente ---\n%s", d, src)
		}
	}
	return doc
}

func TestFormatDocument_EmptySectionRoundTrips(t *testing.T) {
	cases := map[string]struct {
		src  string
		want []sectionShape
	}{
		"en medio": {
			"# T\n\nx\n\n# A\n\n# B\n\nTexto.\n",
			[]sectionShape{{"T", 1, ""}, {"A", 0, ""}, {"B", 1, ""}},
		},
		"al final": {
			"# A\n\nTexto.\n\n# B\n",
			[]sectionShape{{"A", 1, ""}, {"B", 0, ""}},
		},
		"seguidas": {
			"# A\n# B\n# C\nt\n",
			[]sectionShape{{"A", 0, ""}, {"B", 0, ""}, {"C", 1, ""}},
		},
		"node-id en medio": {
			"<!-- node-id: SecT -->\n# T\n\nx\n\n<!-- node-id: SecA -->\n# A\n\n<!-- node-id: SecB -->\n# B\n\nTexto.\n",
			[]sectionShape{{"T", 1, "SecT"}, {"A", 0, "SecA"}, {"B", 1, "SecB"}},
		},
		"node-id al final": {
			"# T\n\nx\n\n<!-- node-id: SecA -->\n# A\n",
			[]sectionShape{{"T", 1, ""}, {"A", 0, "SecA"}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			doc := parseDocumentWithIDs(t, tc.src)
			if got := sectionShapes(doc); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parse: secciones %+v, se esperaba %+v", got, tc.want)
			}

			for dialect, format := range map[string]func(*ast.AST) (string, error){
				"flex":   FormatDocument,
				"strict": FormatDocumentStrict,
			} {
				out, err := format(doc)
				if err != nil {
					t.Fatalf("%s: %v", dialect, err)
				}
				reparsed := parseDocumentWithIDs(t, out)
				if got := sectionShapes(reparsed); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("%s: el reparse da %+v, se esperaba %+v\n--- salida ---\n%s", dialect, got, tc.want, out)
				}
				out2, err := format(reparsed)
				if err != nil || out2 != out {
					t.Fatalf("%s: no idempotente (err=%v)\n--- 1ª ---\n%s\n--- 2ª ---\n%s", dialect, err, out, out2)
				}
			}
		})
	}
}

// El formatter escribe una sección vacía no inicial también cuando el AST no
// viene del parser.
func TestFormatDocument_EmptySectionFromBuiltAST(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	title := ast.NewContentBlock(pos, "title")
	title.Heading = "T"
	title.Elements = []ast.Element{&ast.TextElement{BaseNode: ast.BaseNode{Type: ast.NodeTypeText}, Content: "x"}}
	empty := ast.NewContentBlock(pos, "content")
	empty.Title = "A"
	empty.NodeID = "SecA"
	doc := ast.NewAST(pos)
	doc.ContentBlocks = []ast.ContentBlock{*title, *empty}

	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	want := []sectionShape{{"T", 1, ""}, {"A", 0, "SecA"}}
	if got := sectionShapes(parseDocumentWithIDs(t, out)); !reflect.DeepEqual(got, want) {
		t.Fatalf("reparse: %+v, se esperaba %+v\n--- salida ---\n%s", got, want, out)
	}
}
