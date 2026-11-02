// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// Sin nested-list-types-v1, core ya no aplana el tercer nivel de una lista ni
// de una lista de tareas en el segundo: el AST llega con la profundidad del
// autor (subPoints/subItems recursivos, sin subListType). La plantilla y el
// PPTX tienen que dibujarla entera en vez de quedarse con dos niveles. El AST
// se arma a mano para que la prueba valga igual contra el core publicado.
func legacyDeepPoints() *ast.PointsElement {
	points := ast.NewPointsElement(pos())
	a := ast.NewPointItem(pos(), "LevelA")
	a.SubListMarker = "ordered"
	b := ast.NewPointItem(pos(), "LevelB")
	b.SubListMarker = "unordered"
	c := ast.NewPointItem(pos(), "LevelC")
	c.SubPoints = append(c.SubPoints, *ast.NewPointItem(pos(), "LevelD"))
	b.SubPoints = append(b.SubPoints, *c)
	a.SubPoints = append(a.SubPoints, *b)
	points.Items = append(points.Items, *a)
	return points
}

func legacyDeepChecklist() *ast.ChecklistElement {
	cl := ast.NewChecklistElement(pos())
	t1 := ast.NewChecklistItem(pos(), "TaskOne", true)
	t2 := ast.NewChecklistItem(pos(), "TaskTwo", false)
	t2.SubItems = append(t2.SubItems, *ast.NewChecklistItem(pos(), "TaskThree", true))
	t1.SubItems = append(t1.SubItems, *t2)
	cl.Items = append(cl.Items, *t1)
	return cl
}

func TestLegacyNestedListsKeepEveryLevelInSlideHTMLAndPPTX(t *testing.T) {
	points, cl := legacyDeepPoints(), legacyDeepChecklist()
	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	block := ast.NewContentBlock(pos(), "content")
	block.Title = "List"
	block.Elements = append(block.Elements, points, cl)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatal(err)
	}
	squash := strings.Join(strings.Fields(html), "")
	if !strings.Contains(squash, "<li>LevelA<ol><li>LevelB<ul><li>LevelC<ul><li>LevelD</li></ul></li></ul></li></ol></li>") {
		t.Fatalf("slide HTML lost a level or a type: %s", html)
	}
	two, three := strings.Index(html, "TaskTwo"), strings.Index(html, "TaskThree")
	if two < 0 || three < two || !strings.Contains(html[two:three], "checklist-subitems") {
		t.Fatalf("slide HTML lost the third task level: %s", html)
	}

	xml := buildPPTXWithElements(t, "legacy-list", points, cl)
	for _, tc := range []struct{ text, level string }{
		{"LevelA", `lvl="0"`}, {"LevelB", `lvl="1"`}, {"LevelC", `lvl="2"`}, {"LevelD", `lvl="3"`},
		{"TaskOne", `lvl="0"`}, {"TaskTwo", `lvl="1"`}, {"TaskThree", `lvl="2"`},
	} {
		at := strings.Index(xml, tc.text)
		if at < 0 {
			t.Fatalf("PPTX lost %s", tc.text)
		}
		start := strings.LastIndex(xml[:at], "<a:p>")
		if start < 0 || !strings.Contains(xml[start:at], tc.level) {
			t.Fatalf("PPTX lost the level of %s", tc.text)
		}
	}
}

// Una lista sin list-start-v1 empieza en PPTX en el número que escribió el
// autor (PointItem.WrittenOrdinal, solo con un core que lo registra).
func TestLegacyListStartInPPTX(t *testing.T) {
	if _, ok := any(&ast.PointItem{}).(interface{ WrittenOrdinal() (int64, bool) }); !ok {
		t.Skip("this core does not record written ordinals")
	}
	doc, _ := parser.New(util.NewNoop()).Parse("---\nmode: flex\n---\n# S\n\n3. tres\n4. cuatro\n", "s.slidelang")
	var points *ast.PointsElement
	for _, el := range doc.ContentBlocks[0].Elements {
		if p, ok := el.(*ast.PointsElement); ok {
			points = p
		}
	}
	if points == nil {
		t.Fatal("no list")
	}
	xml := buildPPTXWithElements(t, "start-list", points)
	if strings.Count(xml, `startAt="3"`) != 2 {
		t.Fatalf("PPTX list does not start at 3")
	}
}

// Un item anidado dentro de una cita llega con su sangría en Content; el PPTX
// no la escribe delante de la línea.
func TestPPTXQuoteTrimsNestedListIndent(t *testing.T) {
	xml := buildPPTXWithElements(t, "quote-list", ast.NewQuoteElement(pos(), "Cita\n- a\n  - b"))
	if strings.Contains(xml, "  - b") {
		t.Fatalf("quote kept the list indentation")
	}
	if !strings.Contains(xml, "- b") {
		t.Fatalf("quote lost a line")
	}
}
