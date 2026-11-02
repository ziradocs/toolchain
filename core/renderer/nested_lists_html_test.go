// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer_test

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// renderFirst parsea src como documento y devuelve el HTML del primer
// elemento del tipo pedido.
func renderFirst[T ast.Element](t *testing.T, src string) string {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("diagnostic %+v", d)
		}
	}
	var found ast.Element
	_ = ast.Walk(doc, func(n ast.Node) error {
		if v, ok := n.(T); ok && found == nil {
			found = v
		}
		return nil
	})
	if found == nil {
		var zero T
		t.Fatalf("no %T in the AST", zero)
	}
	return renderer.RenderElementToHTML(found, nil, nil)
}

func flexDoc(body string) string { return "---\nmode: flex\n---\n# S\n\n" + body }

func TestNestedListsHTML(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			// Defecto 1: sin la capacidad una sublista ordenada bajo una
			// viñeta salía con viñetas.
			"ordered sublist under a bullet",
			flexDoc("- a\n- b\n  1. c\n  1. d\n"),
			"<ul><li>a</li><li>b<ol><li>c</li><li>d</li></ol></li></ul>",
		},
		{
			// Defecto 3: del tercer nivel para abajo todo quedaba en el
			// segundo.
			"five levels",
			flexDoc("1. uno\n   - dos\n     1. tres\n        - cuatro\n          1. cinco\n2. otro\n"),
			"<ol><li>uno<ul><li>dos<ol><li>tres<ul><li>cuatro<ol><li>cinco</li></ol></li></ul></li></ol></li></ul></li><li>otro</li></ol>",
		},
		{
			"ordered starts",
			flexDoc("3. tres\n4. cuatro\n   7. siete\n   8. ocho\n"),
			`<ol start="3"><li>tres</li><li>cuatro<ol start="7"><li>siete</li><li>ocho</li></ol></li></ol>`,
		},
		{
			// Como en Markdown, un cambio de marcador entre hermanos abre otra
			// lista dentro del mismo item.
			"marker change starts a new sublist",
			flexDoc("- a\n  - b\n  1. c\n  2. d\n"),
			"<ul><li>a<ul><li>b</li></ul><ol><li>c</li><li>d</li></ol></li></ul>",
		},
		{
			// Una tarea debajo de un item común sale con su casilla.
			"task under a plain bullet",
			flexDoc("- a\n  - [ ] b\n    - [x] c\n"),
			`<ul><li>a<ul><li><input type="checkbox" disabled> b<ul><li><input type="checkbox" checked disabled> c</li></ul></li></ul></li></ul>`,
		},
		{
			"strict depth",
			"---\nmode: strict\n---\nSECTION \"S\"\n  level: 1\n  POINTS\n    - a\n      1. b\n        - c\n",
			"<ul><li>a<ol><li>b<ul><li>c</li></ul></li></ol></li></ul>",
		},
		{
			"typed list with starts",
			"---\nmode: flex\nast_capabilities: [nested-list-types-v1, list-start-v1]\n---\n# S\n\n3. tres\n4. cuatro\n   2. dos\n",
			`<ol start="3"><li>tres</li><li>cuatro<ol start="2"><li>dos</li></ol></li></ol>`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := renderFirst[*ast.PointsElement](t, c.src); got != c.want {
				t.Fatalf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}

// Un AST leído de JSON no trae el tipo de las sublistas en memoria: la
// sublista toma el tipo de su padre, igual que slidelang.
func TestLegacySublistWithoutMarkerFollowsItsParent(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	pts := ast.NewPointsElement(pos)
	pts.ListType = "ordered"
	parent := ast.NewPointItem(pos, "a")
	parent.SubPoints = append(parent.SubPoints, *ast.NewPointItem(pos, "b"))
	pts.Items = append(pts.Items, *parent)
	if got := renderer.RenderElementToHTML(pts, nil, nil); got != "<ol><li>a<ol><li>b</li></ol></li></ol>" {
		t.Fatalf("got %s", got)
	}
}

// Defecto 4: las tareas se anidan a cualquier profundidad y cada sublista
// lleva la clase checklist, que es la que le quita la viñeta.
func TestChecklistHTMLAtDepthThree(t *testing.T) {
	got := renderFirst[*ast.ChecklistElement](t, flexDoc("- [x] t1\n  - [ ] t1.1\n    - [x] t1.1.1\n- [ ] t2\n"))
	want := `<ul class="checklist"><li><input type="checkbox" checked disabled> t1<ul class="checklist checklist-sub"><li><input type="checkbox"  disabled> t1.1<ul class="checklist checklist-sub"><li><input type="checkbox" checked disabled> t1.1.1</li></ul></li></ul></li><li><input type="checkbox"  disabled> t2</li></ul>`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

// Defecto 2: una lista dentro de una cita sale anidada y fuera de todo <p>.
func TestQuoteWithNestedListHTML(t *testing.T) {
	got := renderFirst[*ast.QuoteElement](t, flexDoc("> Antes\n> - a\n>   - b\n>     1. c\n> Después\n"))
	want := "<blockquote><p>Antes</p><ul><li>a<ul><li>b<ol><li>c</li></ol></li></ul></li></ul><p>Después</p></blockquote>"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if strings.Contains(got, "<p><ul") || strings.Contains(got, "<p><ol") {
		t.Fatalf("list inside a paragraph: %s", got)
	}
	// Sin listas, el HTML de siempre.
	plain := renderFirst[*ast.QuoteElement](t, flexDoc("> uno\n> dos\n"))
	if plain != "<blockquote><p>uno<br>dos</p></blockquote>" {
		t.Fatalf("plain quote changed: %s", plain)
	}
	strict := renderFirst[*ast.QuoteElement](t, "---\nmode: strict\n---\nSECTION \"S\"\n  level: 1\n  QUOTE\n    Cita\n    - a\n      - b\n")
	if strict != "<blockquote><p>Cita</p><ul><li>a<ul><li>b</li></ul></li></ul></blockquote>" {
		t.Fatalf("strict quote: %s", strict)
	}
}

func TestCalloutWithNestedListHTML(t *testing.T) {
	got := renderFirst[*ast.SpecialBlockElement](t, flexDoc(":::info Nota\nTexto\n- a\n  - b\n    3. c\n- [x] hecho\n:::\n"))
	if !strings.Contains(got, `<div class="alert-content">Texto<ul><li>a<ul><li>b<ol start="3"><li>c</li></ol></li></ul></li><li><input type="checkbox" checked disabled> hecho</li></ul></div>`) {
		t.Fatalf("got %s", got)
	}
}
