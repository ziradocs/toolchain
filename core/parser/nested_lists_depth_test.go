// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

func firstOf[T any](t *testing.T, doc *ast.AST) T {
	t.Helper()
	var found T
	ok := false
	_ = ast.Walk(doc, func(n ast.Node) error {
		if v, is := n.(T); is && !ok {
			found, ok = v, true
		}
		return nil
	})
	if !ok {
		t.Fatalf("no %T in the AST", found)
	}
	return found
}

func depth(items []ast.PointItem) int {
	d := 0
	for _, it := range items {
		if c := depth(it.SubPoints); c > d {
			d = c
		}
	}
	if len(items) == 0 {
		return 0
	}
	return d + 1
}

// La profundidad y los tipos de una lista anidada llegan al AST por los dos
// caminos de documento y por el de slides, sin nested-list-types-v1, y el
// documento sigue siendo 2.14 sin capacidades: el JSON tiene la forma que ya
// tenía (subPoints recursivo, sin subListType), solo que más profunda.
func TestNestedListsWithoutOptInKeepDepthAndLegacyContract(t *testing.T) {
	flexDoc := "---\nmode: flex\n---\n# S\n\n1. uno\n   - dos\n     1. tres\n        - cuatro\n          1. cinco\n"
	strictDoc := "---\nmode: strict\n---\nSECTION \"S\"\n  level: 1\n  POINTS\n    1. uno\n       - dos\n         1. tres\n            - cuatro\n              1. cinco\n"
	slideDoc := "---\nmode: flex\n---\n# S\n\n1. uno\n   - dos\n     1. tres\n        - cuatro\n          1. cinco\n"
	parse := map[string]func() (*ast.AST, bool){
		"doclang flex": func() (*ast.AST, bool) {
			doc, diags := New(util.NewNoop()).ParseDocument(flexDoc, "")
			return doc, hasErrors(diags)
		},
		"doclang strict": func() (*ast.AST, bool) {
			doc, diags := New(util.NewNoop()).ParseDocument(strictDoc, "")
			return doc, hasErrors(diags)
		},
		"slidelang flex": func() (*ast.AST, bool) {
			doc, diags := New(util.NewNoop()).Parse(slideDoc, "")
			return doc, hasErrors(diags)
		},
	}
	for name, fn := range parse {
		t.Run(name, func(t *testing.T) {
			doc, bad := fn()
			if bad || doc == nil {
				t.Fatalf("parse failed")
			}
			pts := firstOf[*ast.PointsElement](t, doc)
			if got := depth(pts.Items); got != 5 {
				t.Fatalf("depth %d, want 5", got)
			}
			kinds := []string{}
			for items := pts.Items; len(items) > 0; items = items[0].SubPoints {
				if items[0].SubListMarker != "" {
					kinds = append(kinds, items[0].SubListMarker)
				}
			}
			if strings.Join(kinds, ",") != "unordered,ordered,unordered,ordered" {
				t.Fatalf("sublist kinds %v", kinds)
			}
			if caps := ast.UsedCapabilities(doc); len(caps) != 0 {
				t.Fatalf("legacy document reports capabilities %v", caps)
			}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "subListType") || strings.Contains(string(raw), `"capabilities"`) {
				t.Fatalf("legacy JSON gained typed-list fields: %s", raw)
			}
			back, err := ast.DecodeAST(raw)
			if err != nil {
				t.Fatalf("the legacy JSON no longer decodes: %v", err)
			}
			if got := depth(firstOf[*ast.PointsElement](t, back).Items); got != 5 {
				t.Fatalf("depth after a JSON round-trip %d, want 5", got)
			}
		})
	}
}

// Tareas a profundidad 3 y listas dentro de citas y bloques, en un documento
// flex completo (con el normalizador encendido, que es lo que corre doclang).
func TestNestedListsInsideTasksQuotesAndCallouts(t *testing.T) {
	src := "---\nmode: flex\n---\n# S\n\n- [x] t1\n  - [ ] t1.1\n    - [x] t1.1.1\n\n> Cita\n> - a\n>   - b\n\n:::info Nota\nTexto\n- a\n  1. b\n:::\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "")
	if hasErrors(diags) {
		t.Fatalf("diagnostics %+v", diags)
	}
	cl := firstOf[*ast.ChecklistElement](t, doc)
	if len(cl.Items) != 1 || len(cl.Items[0].SubItems) != 1 || len(cl.Items[0].SubItems[0].SubItems) != 1 {
		t.Fatalf("checklist depth lost: %+v", cl.Items)
	}
	if q := firstOf[*ast.QuoteElement](t, doc); q.Content != "Cita\n- a\n  - b" {
		t.Fatalf("quote content %q", q.Content)
	}
	if b := firstOf[*ast.SpecialBlockElement](t, doc); b.Content != "Texto\n- a\n  1. b" {
		t.Fatalf("callout content %q", b.Content)
	}
}

// Con nested-list-types-v1 y list-start-v1 el camino tipado no cambia: el tipo
// y el inicio van serializados.
func TestTypedNestedListsUnchanged(t *testing.T) {
	src := "---\nmode: flex\nast_capabilities: [nested-list-types-v1, list-start-v1]\n---\n# S\n\n3. tres\n4. cuatro\n   - x\n     2. y\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "")
	if hasErrors(diags) {
		t.Fatalf("diagnostics %+v", diags)
	}
	pts := firstOf[*ast.PointsElement](t, doc)
	if pts.Start == nil || *pts.Start != 3 {
		t.Fatalf("start %v", pts.Start)
	}
	x := pts.Items[1].SubPoints[0]
	if pts.Items[1].SubListType != "unordered" || x.SubListType != "ordered" || x.SubListStart == nil || *x.SubListStart != 2 {
		t.Fatalf("typed nesting changed: %+v", pts.Items[1])
	}
}

func hasErrors(diags []diagnostics.Diagnostic) bool {
	for _, d := range diags {
		if d.IsError() {
			return true
		}
	}
	return false
}
