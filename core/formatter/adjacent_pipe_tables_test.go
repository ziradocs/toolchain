// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

func tableCount(doc *ast.AST) int {
	n := 0
	_ = ast.Walk(doc, func(node ast.Node) error {
		if _, ok := node.(*ast.TableElement); ok {
			n++
		}
		return nil
	})
	return n
}

// Dos tablas Markdown seguidas se escriben con una línea en blanco entre ellas:
// sin ella, el parser strict las releía como una sola, con la segunda como
// filas de la primera.
func TestFormatStrict_AdjacentPipeTablesStaySeparate(t *testing.T) {
	for name, tc := range map[string]struct {
		src      string
		document bool
	}{
		"document": {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  | a | b |\n  |---|---|\n  | 1 | 2 |\n\n  | c | d |\n  |---|---|\n  | 3 | 4 |\n", true},
		"slide":    {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  | a | b |\n  |---|---|\n  | 1 | 2 |\n\n  | c | d |\n  |---|---|\n  | 3 | 4 |\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			p := parser.New(util.NewNoop())
			parse := func(src string) *ast.AST {
				if tc.document {
					d, _ := p.ParseDocument(src, "d.doclang")
					return d
				}
				d, _ := p.Parse(src, "s.slidelang")
				return d
			}
			doc := parse(tc.src)
			if tableCount(doc) != 2 {
				t.Fatalf("fixture: want 2 tables, got %d", tableCount(doc))
			}
			var out string
			var err error
			if tc.document {
				out, err = FormatDocumentStrict(doc)
			} else {
				out, err = FormatStrict(doc)
			}
			if err != nil {
				t.Fatalf("format: %v", err)
			}
			if n := tableCount(parse(out)); n != 2 {
				t.Errorf("after fmt: %d tables, want 2:\n%s", n, out)
			}
		})
	}
}

// Un CODE cuyo cuerpo termina en una línea "|" seguido de una tabla pipe no
// recibe una línea en blanco (el CODE la absorbería como contenido): fmt dos
// veces da lo mismo que una, y el AST no cambia.
func TestFormatStrict_CodeEndingInPipeThenPipeTableIsIdempotent(t *testing.T) {
	for name, tc := range map[string]struct {
		src      string
		document bool
	}{
		"document": {"---\nmode: strict\ntitle: \"x\"\n---\n\nSECTION \"A\"\n\n  CODE markdown\n    | a | b |\n    |---|---|\n    | 1 | 2 |\n  | h1 | h2 |\n  |----|----|\n  | v1 | v2 |\n", true},
		"slide":    {"---\nmode: strict\ntitle: \"x\"\n---\nSLIDE content\n  title: \"A\"\n  CODE markdown\n    | a | b |\n    | 1 | 2 |\n  | h1 | h2 |\n  |----|----|\n  | v1 | v2 |\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			p := parser.New(util.NewNoop())
			parse := func(src string) *ast.AST {
				if tc.document {
					d, _ := p.ParseDocument(src, "d.doclang")
					return d
				}
				d, _ := p.Parse(src, "s.slidelang")
				return d
			}
			format := func(d *ast.AST) string {
				var out string
				var err error
				if tc.document {
					out, err = FormatDocumentStrict(d)
				} else {
					out, err = FormatStrict(d)
				}
				if err != nil {
					t.Fatalf("format: %v", err)
				}
				return out
			}
			doc := parse(tc.src)
			once := format(doc)
			twice := format(parse(once))
			if once != twice {
				t.Errorf("fmt is not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
			}
			if a, b := codeOf(doc), codeOf(parse(once)); a != b {
				t.Errorf("the CODE content changed: %q -> %q", a, b)
			}
		})
	}
}

func codeOf(doc *ast.AST) string {
	var c string
	_ = ast.Walk(doc, func(n ast.Node) error {
		if e, ok := n.(*ast.CodeElement); ok {
			c = e.Content
		}
		return nil
	})
	return c
}
