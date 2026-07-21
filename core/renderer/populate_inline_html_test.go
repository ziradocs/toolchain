// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// newTestDoc envuelve un único elemento en un AST mínimo con un ContentBlock,
// para poder ejercitar PopulateInlineHTML (que opera sobre *ast.AST) igual
// que lo hace generateJSON.
func newTestDoc(elem ast.Element) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, elem)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

const xssPayload = `<img src=x onerror=alert(1)>`

// TestPopulateInlineHTML_MatchesRenderElementToHTML es la guarda anti-drift
// descrita en el plan de #64: para cada tipo de elemento de prosa, el campo
// *HTML poblado por PopulateInlineHTML debe aparecer literalmente dentro del
// HTML que produce RenderElementToHTML para el MISMO elemento y variables —
// si alguien cambia el mapeo campo→función sanitizer en un solo lado, esta
// prueba lo detecta.
func TestPopulateInlineHTML_MatchesRenderElementToHTML(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"name": xssPayload}

	cases := []struct {
		name string
		elem ast.Element
		get  func(ast.Element) []string // campos *HTML a verificar
	}{
		{
			name: "text",
			elem: ast.NewTextElement(pos, "**bold** {{name}}"),
			get: func(e ast.Element) []string {
				return []string{e.(*ast.TextElement).ContentHTML}
			},
		},
		{
			name: "code",
			elem: ast.NewCodeElement(pos, "go", "fmt.Println({{name}})"),
			get: func(e ast.Element) []string {
				return []string{e.(*ast.CodeElement).ContentHTML}
			},
		},
		{
			name: "quote",
			elem: func() ast.Element {
				q := ast.NewQuoteElement(pos, "**quoted** {{name}}")
				q.Author = "{{name}}"
				q.Source = "{{name}}"
				return q
			}(),
			get: func(e ast.Element) []string {
				q := e.(*ast.QuoteElement)
				return []string{q.ContentHTML, q.AuthorHTML, q.SourceHTML}
			},
		},
		{
			name: "table",
			elem: func() ast.Element {
				tbl := ast.NewTableElement(pos)
				tbl.Headers = []string{"**H1** {{name}}"}
				tbl.Rows = [][]string{{"cell {{name}}"}}
				tbl.Caption = "cap {{name}}"
				return tbl
			}(),
			get: func(e ast.Element) []string {
				tbl := e.(*ast.TableElement)
				return append(append([]string{}, tbl.HeadersHTML...), append(tbl.RowsHTML[0], tbl.CaptionHTML)...)
			},
		},
		{
			// TitleHTML se excluye deliberadamente de esta comparación: a
			// diferencia de RenderElementToHTML (usado solo por doclang),
			// slidelang SÍ aplica markdown al título de un callout (ver
			// TestPopulateInlineHTML_SpecialBlockTitleAppliesMarkdown), así
			// que TitleHTML puede divergir intencionalmente del fragmento de
			// RenderElementToHTML cuando el título trae sintaxis markdown.
			name: "special_block",
			elem: func() ast.Element {
				sb := ast.NewSpecialBlockElement(pos, "warning", "**warn** {{name}}")
				sb.Title = "title {{name}}"
				return sb
			}(),
			get: func(e ast.Element) []string {
				sb := e.(*ast.SpecialBlockElement)
				return []string{sb.ContentHTML}
			},
		},
		{
			name: "code_group",
			elem: func() ast.Element {
				cg := ast.NewCodeGroupElement(pos)
				cg.CodeBlocks = append(cg.CodeBlocks, ast.CodeBlock{Language: "go", Label: "main.go", Content: "fmt.Println({{name}})"})
				return cg
			}(),
			get: func(e ast.Element) []string {
				// LabelHTML se excluye: RenderElementToHTML (renderCodeGroupElement,
				// usado solo por doclang) NO sustituye {{variables}} en Label,
				// a diferencia del pipeline real de slidelang (converter.go
				// ConvertCodeBlocksWithVariables), que sí lo hace — ver
				// TestPopulateInlineHTML_CodeGroupLabelSubstitutesVariables.
				return []string{e.(*ast.CodeGroupElement).CodeBlocks[0].ContentHTML}
			},
		},
		{
			name: "mermaid",
			elem: func() ast.Element {
				m := ast.NewMermaidElement(pos, "flowchart", "flowchart TD\nA-->B")
				m.Title = "title {{name}}"
				return m
			}(),
			get: func(e ast.Element) []string {
				return []string{e.(*ast.MermaidElement).TitleHTML}
			},
		},
		{
			name: "plantuml",
			elem: func() ast.Element {
				p := ast.NewPlantUMLElement(pos, "sequence", "Alice -> Bob")
				p.Title = "title {{name}}"
				return p
			}(),
			get: func(e ast.Element) []string {
				return []string{e.(*ast.PlantUMLElement).TitleHTML}
			},
		},
		{
			name: "chart",
			elem: func() ast.Element {
				c := ast.NewChartElement(pos, "bar")
				c.Title = "title {{name}}"
				return c
			}(),
			get: func(e ast.Element) []string {
				return []string{e.(*ast.ChartElement).TitleHTML}
			},
		},
		{
			name: "map",
			elem: func() ast.Element {
				m := ast.NewMapElement(pos, "world")
				m.Title = "title {{name}}"
				return m
			}(),
			get: func(e ast.Element) []string {
				return []string{e.(*ast.MapElement).TitleHTML}
			},
		},
		{
			name: "grid",
			elem: func() ast.Element {
				g := ast.NewGridElement(pos)
				g.Content = "grid {{name}}"
				col := ast.NewColumnElement(pos, "col {{name}}")
				g.Columns = append(g.Columns, *col)
				return g
			}(),
			get: func(e ast.Element) []string {
				g := e.(*ast.GridElement)
				return []string{g.ContentHTML, g.Columns[0].ContentHTML}
			},
		},
		{
			name: "image",
			elem: func() ast.Element {
				img := ast.NewImageElement(pos, "photo.png", "alt {{name}}")
				img.Caption = "caption {{name}}"
				return img
			}(),
			get: func(e ast.Element) []string {
				img := e.(*ast.ImageElement)
				return []string{img.AltHTML, img.CaptionHTML}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := newTestDoc(tc.elem)
			PopulateInlineHTML(doc, vars)

			full := RenderElementToHTML(tc.elem, vars, nil)
			for _, fragment := range tc.get(tc.elem) {
				if fragment == "" {
					t.Fatalf("expected non-empty *HTML field")
				}
				if !strings.Contains(full, fragment) {
					t.Errorf("populated fragment %q not found in RenderElementToHTML output:\n%s", fragment, full)
				}
			}
			// El payload XSS nunca debe sobrevivir sin escapar en ningún *HTML.
			for _, fragment := range tc.get(tc.elem) {
				if strings.Contains(fragment, xssPayload) {
					t.Errorf("XSS payload leaked unescaped into *HTML field: %q", fragment)
				}
			}
		})
	}
}

// TestPopulateInlineHTML_PointsAndChecklistRecursion cubre points/checklist
// (con sub-items anidados), que no encajan en la tabla genérica de arriba
// porque viven dentro de un slice de Items, no como campos directos del
// elemento.
func TestPopulateInlineHTML_PointsAndChecklistRecursion(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"name": xssPayload}

	points := ast.NewPointsElement(pos)
	sub := ast.NewPointItem(pos, "sub **bold** {{name}}")
	item := ast.NewPointItem(pos, "main {{name}}")
	item.SubPoints = append(item.SubPoints, *sub)
	points.Items = append(points.Items, *item)

	checklist := ast.NewChecklistElement(pos)
	subCheck := ast.NewChecklistItem(pos, "subcheck {{name}}", false)
	check := ast.NewChecklistItem(pos, "check {{name}}", true)
	check.SubItems = append(check.SubItems, *subCheck)
	checklist.Items = append(checklist.Items, *check)

	pos2 := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos2)
	block := ast.NewContentBlock(pos2, "content")
	block.Elements = append(block.Elements, points, checklist)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	PopulateInlineHTML(doc, vars)

	gotPoints := doc.ContentBlocks[0].Elements[0].(*ast.PointsElement)
	if gotPoints.Items[0].ContentHTML == "" {
		t.Error("PointItem.ContentHTML not populated")
	}
	if gotPoints.Items[0].SubPoints[0].ContentHTML == "" {
		t.Error("PointItem.SubPoints[0].ContentHTML not populated (recursion)")
	}
	if strings.Contains(gotPoints.Items[0].SubPoints[0].ContentHTML, xssPayload) {
		t.Error("sub-point XSS payload leaked unescaped")
	}

	gotChecklist := doc.ContentBlocks[0].Elements[1].(*ast.ChecklistElement)
	if gotChecklist.Items[0].ContentHTML == "" {
		t.Error("ChecklistItem.ContentHTML not populated")
	}
	if gotChecklist.Items[0].SubItems[0].ContentHTML == "" {
		t.Error("ChecklistItem.SubItems[0].ContentHTML not populated (recursion)")
	}
	if strings.Contains(gotChecklist.Items[0].SubItems[0].ContentHTML, xssPayload) {
		t.Error("sub-checklist-item XSS payload leaked unescaped")
	}
}

// TestPopulateInlineHTML_TextRawHTML cubre el branch IsRawHTML de TextElement:
// el contenido existente (HTML de confianza) no debe re-escaparse, pero el
// valor sustituido de cada {{variable}} sí (ver ProcessVariablesEscapeValues,
// CR-2 del audit de seguridad).
func TestPopulateInlineHTML_TextRawHTML(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewRawHTMLTextElement(pos, `<h2 id="x">Config {{name}}</h2>`)
	vars := map[string]interface{}{"name": xssPayload}

	doc := newTestDoc(elem)
	PopulateInlineHTML(doc, vars)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).ContentHTML
	if !strings.HasPrefix(got, `<h2 id="x">Config `) {
		t.Errorf("raw HTML tags were altered: %q", got)
	}
	if strings.Contains(got, xssPayload) {
		t.Errorf("variable value was not escaped in raw-HTML text: %q", got)
	}
	if !strings.Contains(got, "&lt;img") {
		t.Errorf("expected escaped variable value, got: %q", got)
	}
}

// TestPopulateInlineHTML_InlineMarkdownFormats verifica los 6 formatos inline
// soportados por ProcessInlineMarkdownFormatsSecure, a través del campo
// ContentHTML de un TextElement.
func TestPopulateInlineHTML_InlineMarkdownFormats(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewTextElement(pos, "==mark== ~~del~~ **bold** *em* `code` [link](https://example.com)")

	doc := newTestDoc(elem)
	PopulateInlineHTML(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).ContentHTML
	for _, want := range []string{
		"<mark>mark</mark>",
		"<del>del</del>",
		"<strong>bold</strong>",
		"<em>em</em>",
		"<code>code</code>",
		`<a href="https://example.com">link</a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ContentHTML missing %q; got %q", want, got)
		}
	}
}

// TestPopulateInlineHTML_ContentBlockFields cubre title/heading/subtitle del
// ContentBlock, que no son elementos y por eso no pasan por
// populateElementHTML.
func TestPopulateInlineHTML_ContentBlockFields(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "title")
	block.Title = "T {{name}}"
	block.Heading = "H {{name}}"
	block.Subtitle = "S {{name}}"
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	vars := map[string]interface{}{"name": xssPayload}
	PopulateInlineHTML(doc, vars)

	got := doc.ContentBlocks[0]
	for _, field := range []string{got.TitleHTML, got.HeadingHTML, got.SubtitleHTML} {
		if field == "" {
			t.Fatalf("ContentBlock *HTML field not populated")
		}
		if strings.Contains(field, xssPayload) {
			t.Errorf("XSS payload leaked unescaped in ContentBlock field: %q", field)
		}
		if !strings.Contains(field, "&lt;img") {
			t.Errorf("expected escaped variable value, got: %q", field)
		}
	}
}

// TestPopulateInlineHTML_NilSafety cubre doc nil y variables nil: no debe
// paniquear, y con variables nil el texto se escapa igual (sin sustitución).
func TestPopulateInlineHTML_NilSafety(t *testing.T) {
	PopulateInlineHTML(nil, nil) // no debe paniquear

	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewTextElement(pos, "hello {{name}}")
	doc := newTestDoc(elem)

	PopulateInlineHTML(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).ContentHTML
	if !strings.Contains(got, "{{name}}") {
		t.Errorf("expected unresolved placeholder to survive when variables is nil, got: %q", got)
	}
}

// TestPopulateInlineHTML_Additive confirma que poblar los campos *HTML no
// modifica los campos crudos existentes (issue #64: contentHTML es aditivo,
// content sigue siendo la fuente de verdad estructural).
func TestPopulateInlineHTML_Additive(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewTextElement(pos, "**raw** markdown, unmodified")
	doc := newTestDoc(elem)

	PopulateInlineHTML(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement)
	if got.Content != "**raw** markdown, unmodified" {
		t.Errorf("Content field was mutated: %q", got.Content)
	}
	if got.ContentHTML == got.Content {
		t.Errorf("ContentHTML should differ from raw Content after markdown rendering")
	}
}

// TestPopulateInlineHTML_SpecialBlockTitleAppliesMarkdown cubre una
// divergencia deliberada de RenderElementToHTML: el template real de
// slidelang (internal/generator/template/base.go, `{{.Title | markdown}}`)
// SÍ aplica markdown al título de un callout, así que TitleHTML debe
// reflejar eso — a diferencia de otros campos "Title" de este archivo
// (SpecialBlockElement es el único vars-only-por-convención que en realidad
// no lo es).
func TestPopulateInlineHTML_SpecialBlockTitleAppliesMarkdown(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewSpecialBlockElement(pos, "warning", "content")
	elem.Title = "**bold title**"

	doc := newTestDoc(elem)
	PopulateInlineHTML(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.SpecialBlockElement).TitleHTML
	if !strings.Contains(got, "<strong>bold title</strong>") {
		t.Errorf("expected TitleHTML to apply markdown, got: %q", got)
	}
}

// TestPopulateInlineHTML_CodeGroupLabelSubstitutesVariables cubre una
// divergencia deliberada de RenderElementToHTML (renderCodeGroupElement, que
// NO sustituye {{variables}} en Label): el pipeline real de slidelang
// (data.ConvertCodeBlocksWithVariables) sí las sustituye antes de que el
// template renderice el label del tab, así que LabelHTML debe reflejar eso.
func TestPopulateInlineHTML_CodeGroupLabelSubstitutesVariables(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewCodeGroupElement(pos)
	elem.CodeBlocks = append(elem.CodeBlocks, ast.CodeBlock{
		Language: "go", Label: "{{filename}}", Content: "package {{pkg}}",
	})
	vars := map[string]interface{}{"filename": "main.go", "pkg": xssPayload}

	doc := newTestDoc(elem)
	PopulateInlineHTML(doc, vars)

	block := doc.ContentBlocks[0].Elements[0].(*ast.CodeGroupElement).CodeBlocks[0]
	if block.LabelHTML != "main.go" {
		t.Errorf("LabelHTML = %q, want variable substituted to %q", block.LabelHTML, "main.go")
	}
	if strings.Contains(block.ContentHTML, xssPayload) {
		t.Errorf("CodeGroup ContentHTML leaked unescaped XSS payload: %q", block.ContentHTML)
	}
	if !strings.Contains(block.ContentHTML, "&lt;img") {
		t.Errorf("expected escaped variable value in ContentHTML, got: %q", block.ContentHTML)
	}
}
