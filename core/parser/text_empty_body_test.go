// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// Un TEXT sin cuerpo no se traga a los elementos hermanos escritos a su misma
// sangría: el cuerpo de TEXT son las líneas con más sangría que él. Antes
// `TEXT` / `POINTS` / `- uno` salía como el párrafo "POINTS - uno".
func TestStrictText_EmptyBodyDoesNotSwallowSiblings(t *testing.T) {
	for name, tc := range map[string]struct {
		src      string
		document bool
	}{
		"document": {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n  POINTS\n    - uno\n  CHECKLIST\n    - [ ] dos\n", true},
		"slide":    {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n  POINTS\n    - uno\n  CHECKLIST\n    - [ ] dos\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			p := New(util.NewNoop())
			var doc *ast.AST
			if tc.document {
				doc, _ = p.ParseDocument(tc.src, "d.doclang")
			} else {
				doc, _ = p.Parse(tc.src, "s.slidelang")
			}
			var types []string
			for _, el := range doc.ContentBlocks[0].Elements {
				switch e := el.(type) {
				case *ast.TextElement:
					if e.Content != "" {
						t.Errorf("the empty TEXT took content: %q", e.Content)
					}
					types = append(types, "text")
				case *ast.PointsElement:
					types = append(types, "points")
				case *ast.ChecklistElement:
					types = append(types, "checklist")
				}
			}
			if len(types) < 2 || types[len(types)-2] != "points" || types[len(types)-1] != "checklist" {
				t.Errorf("want POINTS and CHECKLIST after the TEXT, got %v", types)
			}
		})
	}
}

// La prosa escrita a la misma sangría que TEXT sigue siendo su cuerpo, como
// siempre; solo una línea que otro elemento reconoce lo corta. En documento,
// slide y columna tipada.
func TestStrictText_SameIndentBodyStillWorks(t *testing.T) {
	cases := map[string]struct {
		src      string
		document bool
	}{
		"document":     {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n  hola mundo\n  sigue\n  POINTS\n    - uno\n", true},
		"slide":        {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n  hola mundo\n  sigue\n  POINTS\n    - uno\n", false},
		"typed column": {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  <<grid>>\n  <<column typed>>\n    TEXT\n    hola mundo\n    sigue\n    POINTS\n      - uno\n  <<end>>\n", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := New(util.NewNoop())
			var doc *ast.AST
			var diags []diagnostics.Diagnostic
			if tc.document {
				doc, diags = p.ParseDocument(tc.src, "d.doclang")
			} else {
				doc, diags = p.Parse(tc.src, "s.slidelang")
			}
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("unexpected error: %v", d)
				}
			}
			var text string
			points := false
			_ = ast.Walk(doc, func(n ast.Node) error {
				switch e := n.(type) {
				case *ast.TextElement:
					text = e.Content
				case *ast.PointsElement:
					points = true
				}
				return nil
			})
			if text != "hola mundo sigue" || !points {
				t.Errorf("text %q points %v", text, points)
			}
		})
	}
}

// Un TEXT sin contenido da TEXT001: error en un documento strict, warning en
// un slide.
func TestStrictText_EmptyTextIsReported(t *testing.T) {
	_, diags := New(util.NewNoop()).ParseDocument("---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n  POINTS\n    - uno\n", "d.doclang")
	if len(diags) != 1 || diags[0].RuleID != "TEXT001" || !diags[0].IsError() || diags[0].Position.Line != 7 {
		t.Errorf("document: want one TEXT001 error at line 7, got %v", diags)
	}
	_, diags = New(util.NewNoop()).Parse("---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n  POINTS\n    - uno\n", "s.slidelang")
	if len(diags) != 1 || diags[0].RuleID != "TEXT001" || diags[0].IsError() {
		t.Errorf("slide: want one TEXT001 warning, got %v", diags)
	}
}
