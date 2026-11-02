// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// deepLegacyPoints arma a mano una lista sin tipar de depth niveles, para que
// la prueba valga igual contra el core publicado.
func deepLegacyPoints(depth int) *ast.PointsElement {
	pos := diagnostics.NewPosition(3, 1)
	var build func(d int) ast.PointItem
	build = func(d int) ast.PointItem {
		item := ast.NewPointItem(pos, fmt.Sprintf("L%d", d))
		if d < depth {
			item.SubPoints = append(item.SubPoints, build(d+1))
		}
		return *item
	}
	points := ast.NewPointsElement(pos)
	points.Items = append(points.Items, build(1))
	return points
}

// Una lista sin tipar más profunda que los nueve niveles de Word se dibuja en
// el último nivel, como el PPTX, en vez de cortar el documento; la tipada
// sigue fallando (TestDOCXNestedListsTooDeep).
func TestDOCXUntypedListDeeperThanNineClamps(t *testing.T) {
	doc := newTestAST()
	cl := ast.NewChecklistElement(diagnostics.NewPosition(1, 1))
	var task func(d int) ast.ChecklistItem
	task = func(d int) ast.ChecklistItem {
		item := ast.NewChecklistItem(diagnostics.NewPosition(1, 1), fmt.Sprintf("T%d", d), false)
		if d < 11 {
			item.SubItems = append(item.SubItems, task(d+1))
		}
		return *item
	}
	cl.Items = append(cl.Items, task(1))
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, deepLegacyPoints(11), cl)
	want := make([]wantListItem, 0, 22)
	for i := 1; i <= 11; i++ {
		want = append(want, wantListItem{fmt.Sprintf("L%d", i), min(i-1, 8), "bullet", 0})
	}
	for i := 1; i <= 11; i++ {
		want = append(want, wantListItem{fmt.Sprintf("T%d", i), min(i-1, 8), "none", 0})
	}
	checkListItems(t, buildDOCXFile(t, doc), want)
}

// Una lista dentro de una cita o de un bloque sale como lista nativa de Word,
// con sus niveles, y sin los espacios de la sangría delante.
func TestDOCXQuoteAndCalloutListsAreNativeLists(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	quote := ast.NewQuoteElement(pos, "Cita\n- qa\n  - qb\n\n    3. qc\nDespués")
	block := ast.NewSpecialBlockElement(pos, "info", "Texto\n- ca\n  1. cb")
	block.Title = "Nota"
	doc := newTestAST()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, quote, block)
	path := buildDOCXFile(t, doc)
	checkListItems(t, path, []wantListItem{
		{"qa", 0, "bullet", 0},
		{"qb", 1, "bullet", 0},
		{"qc", 2, "decimal", 3},
		{"ca", 0, "bullet", 0},
		{"cb", 1, "decimal", 1},
	})
	documentXML := docxZipPart(t, path, "word/document.xml")
	for _, text := range []string{"Cita", "Después", "Texto"} {
		if !strings.Contains(documentXML, text) {
			t.Errorf("prose %q missing", text)
		}
	}
	if strings.Contains(documentXML, "- qa") || strings.Contains(documentXML, "  - qb") {
		t.Error("quote list written as text lines")
	}
}

// Sin list-start-v1, una lista empieza en el número que escribió el autor
// (PointItem.WrittenOrdinal, en memoria). Con un core que todavía no lo
// tiene no hay de dónde leerlo.
func TestDOCXLegacyListStartsAtTheWrittenNumber(t *testing.T) {
	if _, ok := any(&ast.PointItem{}).(interface{ WrittenOrdinal() (int64, bool) }); !ok {
		t.Skip("this core does not record written ordinals")
	}
	path := buildDOCXFile(t, parseInline(t, "# L\n\n3. tres\n4. cuatro\n\n   7. siete\n\nOtra lista:\n\n0. cero\n"))
	checkListItems(t, path, []wantListItem{
		{"tres", 0, "decimal", 3},
		{"cuatro", 0, "decimal", 0},
		{"siete", 1, "decimal", 7},
		{"cero", 0, "decimal", 0},
	})
}

// El Markdown escribía solo el primer nivel de POINTS y CHECKLIST y la primera
// línea de una cita con "> ". Ahora sale todo, y un lector CommonMark arma el
// mismo árbol.
func TestMarkdownLegacyListsQuotesAndCallouts(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	points := ast.NewPointsElement(pos)
	points.ListType = "ordered"
	a := ast.NewPointItem(pos, "a")
	a.SubListMarker = "unordered"
	b := ast.NewPointItem(pos, "b")
	b.Marker = "unordered"
	b.SubListMarker = "ordered"
	c := ast.NewPointItem(pos, "c")
	c.Marker = "ordered"
	b.SubPoints = append(b.SubPoints, *c)
	a.SubPoints = append(a.SubPoints, *b)
	points.Items = append(points.Items, *a, *ast.NewPointItem(pos, "d"))
	m := NewMarkdownGenerator(newTestLogger())
	md := m.renderElement(points)
	if md != "1. a\n   - b\n     1. c\n2. d\n" {
		t.Fatalf("points:\n%s", md)
	}
	if got := commonMarkListShape(t, md); got != "ordered(1[unordered(1[ordered(1[])])],2[])" {
		t.Fatalf("CommonMark tree %s", got)
	}

	cl := ast.NewChecklistElement(pos)
	t1 := ast.NewChecklistItem(pos, "t1", true)
	t2 := ast.NewChecklistItem(pos, "t2", false)
	t2.SubItems = append(t2.SubItems, *ast.NewChecklistItem(pos, "t3", true))
	t1.SubItems = append(t1.SubItems, *t2)
	cl.Items = append(cl.Items, *t1)
	if got := m.renderElement(cl); got != "- [x] t1\n  - [ ] t2\n    - [x] t3\n" {
		t.Fatalf("checklist:\n%s", got)
	}

	quote := m.renderElement(ast.NewQuoteElement(pos, "Cita\n- a\n  - b\n\nOtra"))
	if quote != "> Cita\n> - a\n>   - b\n>\n> Otra\n" {
		t.Fatalf("quote:\n%s", quote)
	}
	block := ast.NewSpecialBlockElement(pos, "info", "uno\n- a\n  - b")
	block.Title = "T"
	if got := m.renderElement(block); got != "> **INFO: T**\n> uno\n> - a\n>   - b\n" {
		t.Fatalf("callout:\n%s", got)
	}
}

// El Markdown conserva el número inicial y el delimitador que escribió el
// autor, que en CommonMark separa "1)" de "1." en dos listas.
func TestMarkdownLegacyListKeepsStartAndDelimiter(t *testing.T) {
	if _, ok := any(&ast.PointItem{}).(interface{ WrittenDelimiter() string }); !ok {
		t.Skip("this core does not record delimiters")
	}
	doc := parseInline(t, "# L\n\n3) tres\n4) cuatro\n\n   7. siete\n")
	m := NewMarkdownGenerator(newTestLogger())
	var md strings.Builder
	for _, el := range doc.ContentBlocks[0].Elements {
		md.WriteString(m.renderElement(el))
	}
	if !strings.Contains(md.String(), "3) tres\n4) cuatro\n\n   7. siete\n") {
		t.Fatalf("markdown:\n%s", md.String())
	}
}

// Un "1)" del autor se numera "%1)" en Word, no "%1.".
func TestDOCXParenDelimiter(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := newTestAST()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, ast.NewQuoteElement(pos, "Cita\n1) uno\n2) dos"))
	path := buildDOCXFile(t, doc)
	got := checkListItemsAny(t, path, "uno")
	if f := docxListFormatOf(t, docxZipPart(t, path, "word/numbering.xml"), got.numID, got.ilvl); f.lvlText != "%1)" {
		t.Fatalf("lvlText %q", f.lvlText)
	}
}

func checkListItemsAny(t *testing.T, path, text string) docxListPara {
	t.Helper()
	for _, p := range docxParagraphs(docxZipPart(t, path, "word/document.xml")) {
		if strings.HasSuffix(p.text, text) && p.numID != 0 {
			return p
		}
	}
	t.Fatalf("%q is not a list paragraph", text)
	return docxListPara{}
}

// La prosa que sigue a una lista dentro de una cita va separada por una línea
// ">" en blanco: sin ella CommonMark la une al último item.
func TestMarkdownQuoteProseAfterList(t *testing.T) {
	m := NewMarkdownGenerator(newTestLogger())
	got := m.renderElement(ast.NewQuoteElement(diagnostics.NewPosition(1, 1), "Cita\n- a\nDespués"))
	if got != "> Cita\n> - a\n>\n> Después\n" {
		t.Fatalf("quote:\n%s", got)
	}
}
