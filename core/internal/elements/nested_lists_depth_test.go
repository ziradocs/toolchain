// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// shape describe un árbol de items como "contenido(tipo de la sublista)[hijos]"
// para comparar en una sola línea.
func pointShape(items []ast.PointItem) string {
	var parts []string
	for _, it := range items {
		s := it.Content
		if len(it.SubPoints) > 0 {
			s += "(" + it.SubListMarker + ")[" + pointShape(it.SubPoints) + "]"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

// Sin nested-list-types-v1 la lista conserva la profundidad que escribió el
// autor, de 1 a 5 niveles, en los dos dialectos: un item más sangrado que el
// anterior es su hijo. Antes, del tercer nivel para abajo todo quedaba como
// hermano del segundo.
func TestLegacyPointsKeepTheDepth(t *testing.T) {
	cases := []struct {
		name  string
		mode  string
		lines []string
		want  string
	}{
		{"flex 1 level", "flex", []string{"- a", "- b"}, "a b"},
		{"flex 2 levels", "flex", []string{"- a", "  - b"}, "a(unordered)[b]"},
		{"flex 3 levels", "flex", []string{"- a", "  - b", "    - c"}, "a(unordered)[b(unordered)[c]]"},
		{"flex 5 levels mixed", "flex", []string{"1. uno", "   - dos", "     1. tres", "        - cuatro", "          1. cinco", "2. otro"},
			"uno(unordered)[dos(ordered)[tres(unordered)[cuatro(ordered)[cinco]]]] otro"},
		{"flex back up a level", "flex", []string{"- a", "  - b", "    - c", "  - d", "- e"}, "a(unordered)[b(unordered)[c] d] e"},
		{"flex bullet with ordered child", "flex", []string{"- a", "- b", "  1. c", "  1. d"}, "a b(ordered)[c d]"},
		{"strict 1 level", "strict", []string{"POINTS", "  - a", "  - b"}, "a b"},
		{"strict 3 levels", "strict", []string{"POINTS", "  - a", "    1. b", "      - c"}, "a(ordered)[b(unordered)[c]]"},
		{"strict 5 levels", "strict", []string{"POINTS", "  1. uno", "    - dos", "      1. tres", "        - cuatro", "          1. cinco"},
			"uno(unordered)[dos(ordered)[tres(unordered)[cuatro(ordered)[cinco]]]]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pts := parsePoints(t, c.mode, c.lines...)
			if got := pointShape(pts.Items); got != c.want {
				t.Fatalf("shape %q, want %q", got, c.want)
			}
			raw, err := json.Marshal(pts)
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"subListType", "subListStart", `"start"`, "ordinal", "Ordinal", "Marker"} {
				if strings.Contains(string(raw), field) {
					t.Fatalf("without the opt-in the JSON must keep the legacy shape, found %s: %s", field, raw)
				}
			}
		})
	}
}

// Cada item recuerda en memoria el número que escribió el autor, a cualquier
// profundidad; el render lo usa como inicio de la lista.
func TestLegacyPointsRecordOrdinals(t *testing.T) {
	pts := parsePoints(t, "flex", "3. tres", "4. cuatro", "   7. siete", "   8. ocho", "      - x")
	if pts.Items[0].Ordinal != 3 || pts.Items[1].Ordinal != 4 {
		t.Fatalf("top ordinals %d %d", pts.Items[0].Ordinal, pts.Items[1].Ordinal)
	}
	sub := pts.Items[1].SubPoints
	if len(sub) != 2 || sub[0].Ordinal != 7 || sub[1].Ordinal != 8 {
		t.Fatalf("sub ordinals %+v", sub)
	}
	if sub[1].SubPoints[0].Ordinal != 0 {
		t.Fatalf("a bullet has no ordinal, got %d", sub[1].SubPoints[0].Ordinal)
	}
	strict := parsePoints(t, "strict", "POINTS", "  5. cinco", "    2. dos")
	if strict.Items[0].Ordinal != 5 || strict.Items[0].SubPoints[0].Ordinal != 2 {
		t.Fatalf("strict ordinals %+v", strict.Items)
	}
}

func checklistShape(items []ast.ChecklistItem) string {
	var parts []string
	for _, it := range items {
		s := it.Content
		if it.Checked {
			s += "*"
		}
		if len(it.SubItems) > 0 {
			s += "[" + checklistShape(it.SubItems) + "]"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

// Las tareas se anidan a cualquier profundidad; antes había dos niveles y una
// sub-sub-tarea salía como hermana de su padre.
func TestChecklistKeepsTheDepth(t *testing.T) {
	cases := []struct {
		name  string
		mode  string
		lines []string
		want  string
	}{
		{"flex depth 3", "flex", []string{"- [x] t1", "  - [ ] t1.1", "    - [x] t1.1.1", "  - [ ] t1.2", "- [ ] t2"}, "t1*[t1.1[t1.1.1*] t1.2] t2"},
		{"flex depth 5", "flex", []string{"- [ ] a", "  - [ ] b", "    - [ ] c", "      - [ ] d", "        - [x] e"}, "a[b[c[d[e*]]]]"},
		{"strict depth 3", "strict", []string{"CHECKLIST", "  [x] t1", "    [ ] t1.1", "      - [x] t1.1.1", "  [ ] t2"}, "t1*[t1.1[t1.1.1*]] t2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := &ParseContext{Mode: c.mode, Lines: c.lines, Logger: util.NewNoop()}
			res := (&ChecklistParser{}).Parse(ctx, 0)
			cl, ok := res.Element.(*ast.ChecklistElement)
			if !ok {
				t.Fatalf("element is %T", res.Element)
			}
			if got := checklistShape(cl.Items); got != c.want {
				t.Fatalf("shape %q, want %q", got, c.want)
			}
		})
	}
}

// Una línea de continuación de una tarea strict se suma a la última tarea
// abierta, la más profunda, no a la tarea base.
func TestStrictChecklistContinuationGoesToTheDeepestItem(t *testing.T) {
	ctx := &ParseContext{Mode: "strict", Lines: []string{"CHECKLIST", "  [ ] t1", "    [ ] t1.1", "      sigue"}, Logger: util.NewNoop()}
	cl := (&ChecklistParser{}).Parse(ctx, 0).Element.(*ast.ChecklistElement)
	if cl.Items[0].Content != "t1" || cl.Items[0].SubItems[0].Content != "t1.1 sigue" {
		t.Fatalf("continuation went to the wrong item: %+v", cl.Items)
	}
}

// Una lista dentro de una cita conserva en Content la sangría de sus items
// anidados; la prosa y una cita sin listas anidadas quedan como antes.
func TestQuoteKeepsNestedListIndent(t *testing.T) {
	cases := []struct {
		name  string
		mode  string
		lines []string
		want  string
	}{
		{"flex nested", "flex", []string{"> Antes", "> - a", ">   - b", ">     1. c"}, "Antes\n- a\n  - b\n    1. c"},
		{"flex prose unchanged", "flex", []string{">   texto con sangría", "> - a", "> - b"}, "texto con sangría\n- a\n- b"},
		{"flex indented prose first", "flex", []string{">   Antes", "> - a", ">   - b"}, "Antes\n- a\n  - b"},
		{"strict nested", "strict", []string{"QUOTE", "    Cita", "    - a", "      - b"}, "Cita\n- a\n  - b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := &ParseContext{Mode: c.mode, Lines: c.lines, Logger: util.NewNoop()}
			q, ok := (&QuoteParser{}).Parse(ctx, 0).Element.(*ast.QuoteElement)
			if !ok {
				t.Fatal("not a quote")
			}
			if q.Content != c.want {
				t.Fatalf("content %q, want %q", q.Content, c.want)
			}
		})
	}
}

// Lo mismo dentro de un ":::bloque", en flex y en strict (donde el cuerpo vive
// sangrado bajo el ":::").
func TestSpecialBlockKeepsNestedListIndent(t *testing.T) {
	cases := []struct {
		name  string
		mode  string
		lines []string
		want  string
	}{
		{"flex", "flex", []string{":::info Nota", "Texto", "- a", "  - b", "    1. c", ":::"}, "Texto\n- a\n  - b\n    1. c"},
		{"strict", "strict", []string{"  :::info Nota", "    Texto", "    - a", "      - b", "  :::"}, "Texto\n- a\n  - b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := &ParseContext{Mode: c.mode, Lines: c.lines, Logger: util.NewNoop()}
			b, ok := (&SpecialBlockParser{}).Parse(ctx, 0).Element.(*ast.SpecialBlockElement)
			if !ok {
				t.Fatal("not a special block")
			}
			if b.Content != c.want {
				t.Fatalf("content %q, want %q", b.Content, c.want)
			}
		})
	}
}
