// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// notesRoundTripAST es el JSON de los elementos de cada bloque sin
// posiciones: lo que fmt tiene que conservar al reescribir el documento.
func notesRoundTripAST(t *testing.T, doc *ast.AST) string {
	t.Helper()
	raw, err := json.Marshal(doc.ContentBlocks)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	var strip func(any) any
	strip = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			delete(x, "position")
			delete(x, "endPosition")
			for k, e := range x {
				x[k] = strip(e)
			}
		case []any:
			for i, e := range x {
				x[i] = strip(e)
			}
		}
		return v
	}
	out, _ := json.Marshal(strip(v))
	return string(out)
}

func parseStrictNotes(t *testing.T, src string, document bool) *ast.AST {
	t.Helper()
	p := parser.New(util.NewNoop())
	var doc *ast.AST
	if document {
		doc, _ = p.ParseDocument(src, "t.doclang")
	} else {
		doc, _ = p.Parse(src, "t.slidelang")
	}
	if doc == nil {
		t.Fatalf("no AST\n%s", src)
	}
	return doc
}

func formatStrictNotes(t *testing.T, doc *ast.AST, document bool) string {
	t.Helper()
	var out string
	var err error
	if document {
		out, err = FormatDocumentStrict(doc)
	} else {
		out, err = FormatStrict(doc)
	}
	if err != nil {
		t.Fatalf("fmt: %v", err)
	}
	return out
}

// Un @notes seguido de elementos hermanos, en un SLIDE, en un SECTION y en una
// columna tipada: lo que escribe fmt se relee con el mismo AST, y fmt sobre su
// propia salida da lo mismo. En una columna tipada fmt no dejaba la línea en
// blanco después de las notas, y al releer el @notes se tragaba al TEXT.
func TestFormatStrict_NotesThenSiblingsRoundTrip(t *testing.T) {
	bodies := map[string]string{
		"blank line then TEXT":      "  @notes\n    uno\n    dos\n\n  TEXT\n    Hermano\n",
		"TEXT right after the body": "  @notes\n    uno\n  TEXT\n    Hermano\n  POINTS\n    - a\n",
		"several notes":             "  @notes\n    uno\n  TEXT\n    Uno\n  @notes\n    dos\n\n  CODE python\n    x = 1\n",
		"notes that kept a sibling": "  @notes\n    uno\n  :::note\n  Ojo\n",
		"typed column":              "  <<grid>>\n  <<column typed>>\n    @notes\n      uno\n\n    TEXT\n      Hermano\n    @notes\n      dos\n    POINTS\n      - a\n  <<column typed>>\n    TEXT\n      Otra\n  <<end>>\n",
		"notes last in the column":  "  <<grid>>\n  <<column typed>>\n    TEXT\n      Antes\n    @notes\n      uno\n  <<end>>\n",
	}
	for name, body := range bodies {
		for _, document := range []bool{false, true} {
			header := "SLIDE content\n  title: \"S\"\n"
			label := name + "/slidelang"
			if document {
				header = "SECTION \"S\"\n"
				label = name + "/doclang"
			}
			t.Run(label, func(t *testing.T) {
				src := "---\nmode: strict\ntitle: \"T\"\n---\n\n" + header + body
				doc := parseStrictNotes(t, src, document)
				out := formatStrictNotes(t, doc, document)
				reread := parseStrictNotes(t, out, document)
				if got, want := notesRoundTripAST(t, reread), notesRoundTripAST(t, doc); got != want {
					t.Errorf("fmt changed the AST\n--- source ---\n%s--- fmt ---\n%s--- before ---\n%s\n--- after ---\n%s", src, out, want, got)
				}
				if again := formatStrictNotes(t, reread, document); again != out {
					t.Errorf("fmt is not idempotent\n--- first ---\n%s--- second ---\n%s", out, again)
				}
				if name == "typed column" && !strings.Contains(out, "      uno\n\n    TEXT") {
					t.Errorf("no blank line after the notes in the column:\n%s", out)
				}
			})
		}
	}
}
