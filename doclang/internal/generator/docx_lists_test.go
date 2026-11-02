// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// docxListPara es un párrafo de word/document.xml con su numeración.
type docxListPara struct {
	text  string
	numID int // 0 si el párrafo no es de lista
	ilvl  int
	ind   bool // trae w:ind propio (pisaría la sangría del nivel)
}

// docxListFormat es el formato y el inicio del nivel que usa un numId.
type docxListFormat struct {
	numFmt  string
	lvlText string
	start   int
}

var (
	docxParaRe  = regexp.MustCompile(`(?s)<w:p>.*?</w:p>`)
	docxTextRe  = regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)
	docxIlvlRe  = regexp.MustCompile(`<w:ilvl w:val="(\d+)">`)
	docxNumIDRe = regexp.MustCompile(`<w:numId w:val="(\d+)">`)
)

func docxZipPart(t *testing.T, path, name string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open docx: %v", err)
	}
	defer func() { _ = r.Close() }()
	for _, f := range r.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rc.Close() }()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	t.Fatalf("%s not found in docx", name)
	return ""
}

func docxParagraphs(documentXML string) []docxListPara {
	var out []docxListPara
	for _, p := range docxParaRe.FindAllString(documentXML, -1) {
		var text strings.Builder
		for _, m := range docxTextRe.FindAllStringSubmatch(p, -1) {
			text.WriteString(m[1])
		}
		para := docxListPara{text: text.String(), ind: strings.Contains(p, "<w:ind ")}
		if m := docxNumIDRe.FindStringSubmatch(p); m != nil {
			para.numID, _ = strconv.Atoi(m[1])
			lvl := docxIlvlRe.FindStringSubmatch(p)
			para.ilvl, _ = strconv.Atoi(lvl[1])
		}
		out = append(out, para)
	}
	return out
}

type numberingXML struct {
	Abstract []struct {
		ID     int `xml:"abstractNumId,attr"`
		Levels []struct {
			Ilvl  int `xml:"ilvl,attr"`
			Start struct {
				Val int `xml:"val,attr"`
			} `xml:"start"`
			NumFmt struct {
				Val string `xml:"val,attr"`
			} `xml:"numFmt"`
			LvlText struct {
				Val string `xml:"val,attr"`
			} `xml:"lvlText"`
		} `xml:"lvl"`
	} `xml:"abstractNum"`
	Nums []struct {
		ID       int `xml:"numId,attr"`
		Abstract struct {
			Val int `xml:"val,attr"`
		} `xml:"abstractNumId"`
	} `xml:"num"`
}

// docxListFormatOf da el formato del nivel ilvl de la lista numID.
func docxListFormatOf(t *testing.T, numbering string, numID, ilvl int) docxListFormat {
	t.Helper()
	var n numberingXML
	if err := xml.Unmarshal([]byte(numbering), &n); err != nil {
		t.Fatalf("numbering.xml: %v", err)
	}
	for _, num := range n.Nums {
		if num.ID != numID {
			continue
		}
		for _, abs := range n.Abstract {
			if abs.ID != num.Abstract.Val {
				continue
			}
			for _, lvl := range abs.Levels {
				if lvl.Ilvl == ilvl {
					return docxListFormat{numFmt: lvl.NumFmt.Val, lvlText: lvl.LvlText.Val, start: lvl.Start.Val}
				}
			}
		}
	}
	t.Fatalf("numId %d ilvl %d not defined in numbering.xml", numID, ilvl)
	return docxListFormat{}
}

func buildDOCXFile(t *testing.T, doc *ast.AST) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.docx")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return out
}

type wantListItem struct {
	text   string
	ilvl   int
	numFmt string // decimal, bullet o none
	start  int
}

// checkListItems exige que cada ítem aparezca, en orden, como párrafo de
// lista con su nivel y formato, sin sangría directa. sameList agrupa los
// textos que deben compartir numId (hermanos de una misma lista) y
// newList los que deben estar en listas distintas.
func checkListItems(t *testing.T, path string, want []wantListItem) map[string]docxListPara {
	t.Helper()
	paras := docxParagraphs(docxZipPart(t, path, "word/document.xml"))
	numbering := docxZipPart(t, path, "word/numbering.xml")
	byText := map[string]docxListPara{}
	next := 0
	for _, w := range want {
		found := false
		for ; next < len(paras); next++ {
			if strings.HasSuffix(paras[next].text, w.text) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("item %q missing (or out of order) in document.xml", w.text)
		}
		p := paras[next]
		next++
		if p.numID == 0 {
			t.Errorf("item %q is not a Word list paragraph", w.text)
			continue
		}
		if p.ilvl != w.ilvl {
			t.Errorf("item %q: ilvl = %d, want %d", w.text, p.ilvl, w.ilvl)
		}
		if p.ind {
			t.Errorf("item %q: direct w:ind overrides the level indent", w.text)
		}
		f := docxListFormatOf(t, numbering, p.numID, p.ilvl)
		if f.numFmt != w.numFmt {
			t.Errorf("item %q: numFmt = %q, want %q", w.text, f.numFmt, w.numFmt)
		}
		if w.numFmt == "decimal" && f.lvlText != fmt.Sprintf("%%%d.", p.ilvl+1) {
			t.Errorf("item %q: lvlText = %q, want the counter of its own level", w.text, f.lvlText)
		}
		if w.start != 0 && f.start != w.start {
			t.Errorf("item %q: start = %d, want %d", w.text, f.start, w.start)
		}
		byText[w.text] = p
	}
	return byText
}

func sameList(t *testing.T, got map[string]docxListPara, texts ...string) {
	t.Helper()
	for _, text := range texts[1:] {
		if got[text].numID != got[texts[0]].numID {
			t.Errorf("%q and %q should be siblings in one list (numId %d vs %d)", texts[0], text, got[texts[0]].numID, got[text].numID)
		}
	}
}

func distinctLists(t *testing.T, got map[string]docxListPara, texts ...string) {
	t.Helper()
	seen := map[int]string{}
	for _, text := range texts {
		if prev, ok := seen[got[text].numID]; ok {
			t.Errorf("%q and %q share numId %d; each list must restart on its own", prev, text, got[text].numID)
		}
		seen[got[text].numID] = text
	}
}

// El defecto: los SubPoints de una lista y los SubItems de un checklist no
// llegaban al DOCX. Fuente Markdown flex, de punta a punta por el parser.
func TestDOCXNestedListsFlexMarkdown(t *testing.T) {
	src := "# Listas\n\n" +
		"- uno\n  - dos\n  - dos-b\n- tres\n\n" +
		"Texto entre listas.\n\n" +
		"1. a\n   - b-viñeta\n2. c\n   1. d-numerado\n   2. e-numerado\n3. f\n\n" +
		"- [ ] tarea\n  - [x] subtarea\n"
	path := buildDOCXFile(t, parseInline(t, src))
	got := checkListItems(t, path, []wantListItem{
		{"uno", 0, "bullet", 0},
		{"dos", 1, "bullet", 0},
		{"dos-b", 1, "bullet", 0},
		{"tres", 0, "bullet", 0},
		{"a", 0, "decimal", 1},
		{"b-viñeta", 1, "bullet", 0},
		{"c", 0, "decimal", 0},
		{"d-numerado", 1, "decimal", 1},
		{"e-numerado", 1, "decimal", 0},
		{"f", 0, "decimal", 0},
		{"☐ tarea", 0, "none", 0},
		{"☑ subtarea", 1, "none", 0},
	})
	sameList(t, got, "uno", "tres")
	sameList(t, got, "dos", "dos-b")
	sameList(t, got, "a", "c", "f")
	sameList(t, got, "d-numerado", "e-numerado")
	distinctLists(t, got, "uno", "dos", "a", "b-viñeta", "d-numerado", "☐ tarea", "☑ subtarea")
}

// La misma lista en DocLang strict.
func TestDOCXNestedListsStrict(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSECTION \"S\"\n\n" +
		"  POINTS\n    - uno\n      - dos\n    - tres\n\n" +
		"  POINTS\n    1. a\n      1. b\n    2. c\n\n" +
		"  CHECKLIST\n    - [ ] t1\n      - [x] t2\n"
	path := buildDOCXFile(t, parseDocLang(t, src))
	got := checkListItems(t, path, []wantListItem{
		{"uno", 0, "bullet", 0},
		{"dos", 1, "bullet", 0},
		{"tres", 0, "bullet", 0},
		{"a", 0, "decimal", 1},
		{"b", 1, "decimal", 1},
		{"c", 0, "decimal", 0},
		{"☐ t1", 0, "none", 0},
		{"☑ t2", 1, "none", 0},
	})
	sameList(t, got, "a", "c")
	distinctLists(t, got, "uno", "a", "b")
}

// nested-list-types-v1: antes el DOCX lo rechazaba; ahora cada nivel sale con
// su tipo, y la sublista numerada de ParentB empieza en 1 en su propia lista.
func TestDOCXNestedListTypesNativeLevels(t *testing.T) {
	doc := newTestAST()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, typedDocumentPoints())
	ast.SetTableContract(doc)
	path := buildDOCXFile(t, doc)
	got := checkListItems(t, path, []wantListItem{
		{"ParentA", 0, "decimal", 1},
		{"ChildA", 1, "bullet", 0},
		{"GrandchildA", 2, "decimal", 1},
		{"ChildB", 1, "bullet", 0},
		{"ParentB", 0, "decimal", 0},
		{"ChildC", 1, "decimal", 1},
	})
	sameList(t, got, "ParentA", "ParentB")
	sameList(t, got, "ChildA", "ChildB")
	distinctLists(t, got, "ParentA", "ChildA", "GrandchildA", "ChildC")
}

// Un AST leído de JSON no trae SubListMarker: la sublista toma el tipo de la
// lista padre. El inicio del ordered (list-start-v1) se respeta en ambos
// niveles; antes el número se escribía como texto desde 1.
func TestDOCXNestedListsFromJSONAndStart(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	points := ast.NewPointsElement(pos)
	points.ListType = "ordered"
	five, seven := int64(5), int64(7)
	points.Start = &five
	parent := ast.NewPointItem(pos, "Quinto")
	parent.SubListStart = &seven
	parent.SubPoints = append(parent.SubPoints, *ast.NewPointItem(pos, "Séptimo"))
	points.Items = append(points.Items, *parent, *ast.NewPointItem(pos, "Sexto"))
	doc := newTestAST()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, points)
	checkListItems(t, buildDOCXFile(t, doc), []wantListItem{
		{"Quinto", 0, "decimal", 5},
		{"Séptimo", 1, "decimal", 7},
		{"Sexto", 0, "decimal", 0},
	})
}

// Word admite 9 niveles; uno más falla en vez de aplanarse en silencio.
func TestDOCXNestedListsTooDeep(t *testing.T) {
	pos := diagnostics.NewPosition(3, 1)
	var build func(depth int) ast.PointItem
	build = func(depth int) ast.PointItem {
		item := ast.NewPointItem(pos, fmt.Sprintf("L%d", depth))
		if depth < 10 {
			item.SubListType = "unordered"
			item.SubPoints = append(item.SubPoints, build(depth+1))
		}
		return *item
	}
	points := ast.NewPointsElement(pos)
	points.Items = append(points.Items, build(1))
	doc := newTestAST()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, points)
	deep := filepath.Join(t.TempDir(), "deep.docx")
	err := New(newTestLogger()).Generate(doc, deep, GeneratorOptions{Format: "docx"})
	if err == nil || !strings.Contains(err.Error(), "9 niveles") {
		t.Fatalf("want a depth error, got %v", err)
	}
	if _, statErr := os.Stat(deep); !os.IsNotExist(statErr) {
		t.Fatalf("DOCX wrote output before failing on depth: %v", statErr)
	}

	// Nueve niveles sí entran.
	points.Items[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints = nil
	points.Items[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints[0].SubPoints[0].SubListType = ""
	want := make([]wantListItem, 0, 9)
	for i := 1; i <= 9; i++ {
		want = append(want, wantListItem{fmt.Sprintf("L%d", i), i - 1, "bullet", 0})
	}
	checkListItems(t, buildDOCXFile(t, doc), want)
}

// Un bloque especial con contenido anidado tipado (Elements) se escribe desde
// Elements: la tabla sale como tabla de Word y la prosa una sola vez. Antes
// se escribía Content línea por línea y la tabla salía como texto con barras.
//
// Una lista dentro del bloque no llega a Elements: core la deja como prosa en
// Content y ya sin la sangría de sus subniveles, así que acá solo se exige que
// ningún ítem se pierda.
func TestDOCXCalloutNestedElements(t *testing.T) {
	src := "# Doc\n\n:::info Nota\nTexto previo.\n\n| A | B |\n|---|---|\n| celda-1 | celda-2 |\n\n- fuera\n  - dentro\n:::\n"
	doc := parseInline(t, src)
	block, ok := doc.ContentBlocks[0].Elements[0].(*ast.SpecialBlockElement)
	if !ok || len(block.Elements) == 0 {
		t.Fatalf("fixture did not produce a callout with Elements: %#v", doc.ContentBlocks[0].Elements)
	}
	documentXML := docxZipPart(t, buildDOCXFile(t, doc), "word/document.xml")
	if !strings.Contains(documentXML, "<w:tbl>") {
		t.Error("callout table not written as a Word table")
	}
	if strings.Contains(documentXML, "| celda-1") {
		t.Error("callout table written as raw text")
	}
	for _, want := range []string{"Nota", "celda-1", "celda-2", "fuera", "dentro"} {
		if !strings.Contains(documentXML, want) {
			t.Errorf("callout text %q missing", want)
		}
	}
	if n := strings.Count(documentXML, "Texto previo."); n != 1 {
		t.Errorf("callout prose written %d times, want 1", n)
	}
}

// Autor y fuente de una cita no se escribían.
func TestDOCXQuoteAuthorAndSource(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	quote := ast.NewQuoteElement(pos, "La cita.")
	quote.Author = "Ada_Lovelace"
	quote.Source = "Notas *G*"
	doc := newTestAST()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, quote)
	documentXML := docxZipPart(t, buildDOCXFile(t, doc), "word/document.xml")
	for _, want := range []string{"La cita.", "— Ada_Lovelace", "Notas *G*"} {
		if !strings.Contains(documentXML, want) {
			t.Errorf("quote text %q missing from DOCX", want)
		}
	}
}

// Un mermaid o una ecuación dentro de un bloque especial o una columna tipada
// también pide Chromium; antes solo se miraba el primer nivel de la sección.
func TestDOCXNeedsChromiumForNestedRichElements(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewSpecialBlockElement(pos, "info", "")
	block.Elements = []ast.Element{ast.NewMathElement(pos, "x^2")}
	doc := newTestAST()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, block)
	g := NewDOCXGenerator(newTestLogger(), "")
	if !g.needsChromiumRendering(doc) {
		t.Error("math inside a callout does not request Chromium")
	}
	if g.canRenderWithoutChromium(doc) {
		t.Error("math inside a callout claims a native render")
	}
}

// Cada campo de texto de los elementos que el DOCX escribe sin Chromium lleva
// un centinela distinto, y cada centinela tiene que aparecer en el
// document.xml. Es la red contra texto que un renderer deja caer en silencio
// (como los SubPoints o el autor de una cita).
func TestDOCXWritesEveryTextField(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	var els []ast.Element
	add := func(e ast.Element) { els = append(els, e) }

	add(ast.NewHeadingElement(pos, 2, "S-heading", ""))
	add(ast.NewTextElement(pos, "S-text"))

	points := ast.NewPointsElement(pos)
	p1 := ast.NewPointItem(pos, "S-point")
	p1.SubPoints = append(p1.SubPoints, *ast.NewPointItem(pos, "S-subpoint"))
	points.Items = append(points.Items, *p1)
	add(points)

	check := ast.NewChecklistElement(pos)
	c1 := ast.NewChecklistItem(pos, "S-task", false)
	c1.SubItems = append(c1.SubItems, *ast.NewChecklistItem(pos, "S-subtask", true))
	check.Items = append(check.Items, *c1)
	add(check)

	code := ast.NewCodeElement(pos, "go", "S-code")
	code.Filename = "S-filename"
	add(code)

	table := ast.NewTableElement(pos)
	table.Headers = []string{"S-header"}
	table.Rows = [][]string{{"S-cell"}}
	table.Rows = [][]string{{"S-cell", "S-extracell"}}
	table.Caption = "S-tcaption"
	add(table)

	bare := ast.NewTableElement(pos)
	bare.Rows = [][]string{{"S-noheader"}}
	add(bare)

	media := ast.NewMediaElement(pos, "video", "https://example.com/v.mp4")
	media.Caption = "S-mediacap"
	add(media)

	geo := ast.NewMapElement(pos, "static")
	geo.Title = "S-maptitle"
	add(geo)

	img := ast.NewImageElement(pos, "missing.png", "S-alt")
	img.Caption = "S-imgcap"
	add(img)

	quote := ast.NewQuoteElement(pos, "S-quote")
	quote.Author = "S-author"
	quote.Source = "S-source"
	add(quote)

	metric := ast.NewMetricElement(pos)
	metric.Label, metric.Value, metric.Delta, metric.Caption = "S-mlabel", "S-mvalue", "S-mdelta", "S-mcaption"
	add(metric)

	quiz := ast.NewQuizElement(pos)
	quiz.Question, quiz.Options, quiz.Explanation = "S-quiz", []string{"S-qopt1", "S-qopt2"}, "S-qexpl"
	add(quiz)

	poll := ast.NewPollElement(pos)
	poll.Question, poll.Options = "S-poll", []string{"S-popt"}
	add(poll)

	block := ast.NewSpecialBlockElement(pos, "warning", "S-callout")
	block.Title = "S-ctitle"
	add(block)

	group := ast.NewCodeGroupElement(pos)
	group.CodeBlocks = []ast.CodeBlock{{Language: "sh", Label: "S-glabel", Content: "S-gcode"}}
	add(group)

	grid := ast.NewGridElement(pos)
	col := ast.NewColumnElement(pos, "S-rawcol")
	typed := ast.NewColumnElement(pos, "")
	typed.Elements = []ast.Element{ast.NewTextElement(pos, "S-typedcol")}
	grid.Columns = []ast.ColumnElement{*col, *typed}
	add(grid)

	doc := newTestAST()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, els...)
	documentXML := docxZipPart(t, buildDOCXFile(t, doc), "word/document.xml")
	for _, s := range []string{
		"S-heading", "S-text", "S-point", "S-subpoint", "S-task", "S-subtask",
		"S-code", "S-filename", "S-header", "S-cell", "S-extracell", "S-tcaption", "S-noheader",
		"S-mediacap", "S-alt", "S-imgcap", "S-maptitle",
		"S-quote", "S-author", "S-source",
		"S-mlabel", "S-mvalue", "S-mdelta", "S-mcaption",
		"S-quiz", "S-qopt1", "S-qopt2", "S-qexpl", "S-poll", "S-popt",
		"S-callout", "S-ctitle", "S-glabel", "S-gcode", "S-rawcol", "S-typedcol",
	} {
		if !strings.Contains(documentXML, s) {
			t.Errorf("%s missing from document.xml", s)
		}
	}
}
