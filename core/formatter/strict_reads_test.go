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

// El texto de la línea de un TEXT o un QUOTE strict es su primera línea de
// contenido. fmt lo escribe en el cuerpo, que es la forma de siempre: se
// vuelve a leer igual y fmt de esa salida no la cambia.
func TestFormatStrict_KeywordLineTextRoundTrips(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"text", "  TEXT hola\n    mundo\n", "  TEXT\n    hola mundo\n"},
		{"text alone", "  TEXT: hola\n", "  TEXT\n    hola\n"},
		{"quote", "  QUOTE una cita\n    que sigue\n    AUTHOR: Ana\n", "  QUOTE\n    una cita\n    que sigue\n    AUTHOR: Ana\n"},
		{"quote alone", "  QUOTE: una cita\n", "  QUOTE\n    una cita\n"},
		{"quote with a glued keyword", "  QUOTE TEXTO citado\n", "  QUOTE\n    TEXTO citado\n"},
	}
	for _, tc := range cases {
		for _, document := range []bool{false, true} {
			name := tc.name + "/slide"
			src := "---\nmode: strict\ntitle: \"T\"\n---\n\nSLIDE content\n  title: \"S\"\n" + tc.body
			format := FormatStrict
			parse := func(s string) (*ast.AST, bool) {
				doc, diags := parser.New(util.NewNoop()).Parse(s, "r.slidelang")
				return doc, len(diags) == 0
			}
			if document {
				name = tc.name + "/document"
				src = "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n" + tc.body
				format = FormatDocumentStrict
				parse = func(s string) (*ast.AST, bool) {
					doc, diags := parser.New(util.NewNoop()).ParseDocument(s, "r.doclang")
					return doc, len(diags) == 0
				}
			}
			t.Run(name, func(t *testing.T) {
				doc, ok := parse(src)
				if !ok {
					t.Fatalf("source has diagnostics")
				}
				out, err := format(doc)
				if err != nil {
					t.Fatalf("fmt: %v", err)
				}
				if !strings.Contains(out, tc.want) {
					t.Errorf("fmt output does not contain %q:\n%s", tc.want, out)
				}
				again, ok := parse(out)
				if !ok {
					t.Fatalf("fmt output has diagnostics:\n%s", out)
				}
				if a, b := stripForComparison(astToTree(t, doc)), stripForComparison(astToTree(t, again)); !reflect.DeepEqual(a, b) {
					t.Errorf("fmt output reads differently: %v", diffASTs(a, b))
				}
				if out2, err := format(again); err != nil || out2 != out {
					t.Errorf("fmt is not idempotent: %v\n%s\n---\n%s", err, out, out2)
				}
			})
		}
	}
}
