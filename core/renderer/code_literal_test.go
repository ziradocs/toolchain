// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Issue #487: con RenderContext.LiteralCode el código es literal, `{{nombre}}`
// incluido, como en el AST. La etiqueta de una pestaña de code-group no es
// código y sigue como estaba. Sin la opción (el valor cero, lo que usa
// slidelang) el código sustituye como siempre.
func TestRenderElementToHTML_LiteralCodeOption(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"title": "Demo"}

	code := ast.NewCodeElement(pos, "yaml", "a: {{title}}")
	group := ast.NewCodeGroupElement(pos)
	group.CodeBlocks = append(group.CodeBlocks, ast.CodeBlock{Language: "go", Label: "main.go", Content: "b: {{title}}"})

	for _, tc := range []struct {
		ctx       *RenderContext
		code, grp string
	}{
		{&RenderContext{LiteralCode: true}, "a: {{title}}", "b: {{title}}"},
		{nil, "a: Demo", "b: Demo"},
		{&RenderContext{}, "a: Demo", "b: Demo"},
	} {
		if out := RenderElementToHTML(code, vars, tc.ctx); !strings.Contains(out, tc.code) {
			t.Errorf("ctx %+v: code %q not in:\n%s", tc.ctx, tc.code, out)
		}
		if out := RenderElementToHTML(group, vars, tc.ctx); !strings.Contains(out, tc.grp) {
			t.Errorf("ctx %+v: code-group %q not in:\n%s", tc.ctx, tc.grp, out)
		}
	}
}

// El documento completo con la opción: el bloque de código sale literal y el
// texto de alrededor sigue sustituyendo.
func TestGenerateDocumentHTML_LiteralCodeOption(t *testing.T) {
	doc := ast.NewAST(diagnostics.NewPosition(1, 1))
	doc.FrontMatter = ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	doc.FrontMatter.Title = "Demo"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "title")
	block.Heading = "A"
	block.Elements = []ast.Element{
		ast.NewTextElement(diagnostics.NewPosition(3, 1), "Título: {{title}}"),
		ast.NewCodeElement(diagnostics.NewPosition(4, 1), "yaml", "a: {{title}}"),
	}
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, &RenderContext{LiteralCode: true})
	if !strings.Contains(out, "a: {{title}}") {
		t.Errorf("the code block must keep {{title}}")
	}
	if !strings.Contains(out, "Título: Demo") {
		t.Errorf("prose must still substitute {{title}}")
	}
	if out := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, nil); !strings.Contains(out, "a: Demo") {
		t.Errorf("without the option the code block substitutes, as before")
	}
}

// PopulateInlineHTMLWithOptions con LiteralCode deja el contentHTML del código
// literal; PopulateInlineHTML (sin opciones) sustituye, como siempre.
func TestPopulateInlineHTML_LiteralCodeOption(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"title": "Demo"}
	for _, tc := range []struct {
		literal bool
		want    string
	}{{true, "a: {{title}}"}, {false, "a: Demo"}} {
		code := ast.NewCodeElement(pos, "yaml", "a: {{title}}")
		PopulateInlineHTMLWithOptions(newTestDoc(code), vars, InlineHTMLOptions{LiteralCode: tc.literal})
		if code.ContentHTML != tc.want {
			t.Errorf("literal=%v: contentHTML %q, want %q", tc.literal, code.ContentHTML, tc.want)
		}
	}
}

// substituteOutsideCode deja intactos los code spans y las cercas de código de
// la prosa, y sustituye el resto.
func TestSubstituteOutsideCode(t *testing.T) {
	vars := map[string]interface{}{"x": "X"}
	for _, tc := range []struct{ in, want string }{
		{"a {{x}} `b {{x}}` c {{x}}", "a X `b {{x}}` c X"},
		{"``uno ` {{x}}`` {{x}}", "``uno ` {{x}}`` X"},
		{"sin cierre ` {{x}}", "sin cierre ` X"},
		{"texto {{x}}\n```yaml\na: {{x}}\n```\nluego {{x}}", "texto X\n```yaml\na: {{x}}\n```\nluego X"},
		// Una cerca de cuatro guarda un ``` como contenido y cierra en su
		// propio ```` (CommonMark, como la pinta un visor de Markdown).
		{"````md\n```\n{{x}}\n```\n````\n{{x}}", "````md\n```\n{{x}}\n```\n````\nX"},
		// Un backtick en el info string no abre cerca: es un code span y el
		// resto es prosa (hallazgo 3 del review de #497).
		{"```code``` inline {{x}}\nluego {{x}}", "```code``` inline X\nluego X"},
		{"a\n```js`\n{{x}}", "a\n```js`\nX"},
		{"x ` y\n`{{x}}`", "x ` y\n`{{x}}`"},
		{"```no es cerca {{x}}", "```no es cerca X"},
		{"```\n{{x}} sin cerrar", "```\n{{x}} sin cerrar"},
		{"no ```es{{x}}` cerca", "no ```esX` cerca"},
	} {
		if got := proseVariables(tc.in, withLiteralCode(vars)); got != tc.want {
			t.Errorf("literal %q:\n got %q\nwant %q", tc.in, got, tc.want)
		}
		if got, want := proseVariables(tc.in, vars), ProcessVariables(tc.in, vars); got != want {
			t.Errorf("without the option %q: got %q, want ProcessVariables %q", tc.in, got, want)
		}
	}
}

// La marca no se pone desde afuera: un frontmatter no puede producir la clave
// (no es UTF-8 válido) y el valor tiene que ser del tipo no exportado.
func TestLiteralCodeMarkerCannotBeForged(t *testing.T) {
	vars := map[string]interface{}{literalCodeKey: true, "x": "X"}
	if isLiteralCode(vars) {
		t.Fatal("a plain value under the key must not turn the option on")
	}
	if got := proseVariables("`{{x}}`", vars); got != "`X`" {
		t.Errorf("got %q", got)
	}
}

// Paridad del HTML con el contentHTML del AST para prosa con código, en los dos
// modos: lo que sale en el HTML es lo que get_ast dice.
func TestLiteralCode_ProseParity(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"x": "X"}
	build := func() []ast.Element {
		quote := ast.NewQuoteElement(pos, "cita {{x}} `{{x}}`")
		table := ast.NewTableElement(pos)
		table.Headers = []string{"`{{x}}` {{x}}"}
		table.Rows = [][]string{{"`{{x}}`"}}
		table.Cells = ast.DeriveCellsFromFlat(table.Headers, table.Rows)
		special := ast.NewSpecialBlockElement(pos, "info", "aviso {{x}}\n```sh\necho {{x}}\n```")
		nestedSpecial := ast.NewSpecialBlockElement(pos, "info", "uno {{x}}\n```sh\necho {{x}}\n```")
		nestedSpecial.Elements = []ast.Element{
			ast.NewTextElement(pos, "uno {{x}}"),
			ast.NewCodeElement(pos, "sh", "echo {{x}}"),
		}
		return []ast.Element{
			ast.NewTextElement(pos, "Usa `{{x}}` con {{x}}"),
			ast.NewTextElement(pos, "antes {{x}}\n```yaml\na: {{x}}\n```"),
			quote, table, special, nestedSpecial,
		}
	}
	html := func(e ast.Element) []string {
		switch v := e.(type) {
		case *ast.TextElement:
			return []string{v.ContentHTML}
		case *ast.QuoteElement:
			return []string{v.ContentHTML}
		case *ast.TableElement:
			return append(append([]string{}, v.HeadersHTML...), v.RowsHTML[0]...)
		case *ast.SpecialBlockElement:
			return []string{v.ContentHTML}
		}
		return nil
	}
	for _, literal := range []bool{false, true} {
		for i, el := range build() {
			PopulateInlineHTMLWithOptions(newTestDoc(el), vars, InlineHTMLOptions{LiteralCode: literal})
			full := RenderElementToHTML(el, vars, &RenderContext{LiteralCode: literal})
			for _, fragment := range html(el) {
				if literal && !strings.Contains(fragment, "{{x}}") {
					t.Errorf("element %d literal: the code kept no {{x}}: %q", i, fragment)
				}
				if !literal && strings.Contains(fragment, "{{x}}") {
					t.Errorf("element %d default: {{x}} must be substituted: %q", i, fragment)
				}
				if !strings.Contains(fragment, "X") && !strings.HasPrefix(fragment, "<code>") {
					t.Errorf("element %d literal=%v: prose outside code must substitute: %q", i, literal, fragment)
				}
				if _, isSpecial := el.(*ast.SpecialBlockElement); isSpecial {
					continue // el HTML de un ::: con Elements renderiza los Elements, no Content
				}
				if !strings.Contains(full, fragment) {
					t.Errorf("element %d literal=%v: contentHTML %q not in HTML:\n%s", i, literal, fragment, full)
				}
			}
			if literal {
				if strings.Contains(full, "echo X") || strings.Contains(full, "a: X") || strings.Contains(full, "<code>X</code>") {
					t.Errorf("element %d: code was substituted in the HTML:\n%s", i, full)
				}
			}
		}
	}
}

// Encabezados con un code span: con la opción, el código queda literal en
// todos los caminos (nodo tipado, forma legada, RenderElementToHTML).
func TestLiteralCode_Headings(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"n": "N"}
	input := "Usa `{{n}}` y {{n}}"
	want := "Usa <code>{{n}}</code> y N"

	typed := ast.NewHeadingElement(pos, 2, input, "h")
	PopulateInlineHTMLWithOptions(newTestDoc(typed), vars, InlineHTMLOptions{LiteralCode: true})
	if !strings.Contains(typed.TextHTML, want) {
		t.Errorf("typed TextHTML %q, want %q", typed.TextHTML, want)
	}
	if got := RenderElementToHTML(ast.NewHeadingElement(pos, 2, input, "h"), vars, &RenderContext{LiteralCode: true}); !strings.Contains(got, want) {
		t.Errorf("RenderElementToHTML %q, want %q", got, want)
	}
	legacy := LegacyHeadingElement(ast.NewHeadingElement(pos, 2, input, "h"))
	if got := RenderElementToHTML(legacy, vars, &RenderContext{LiteralCode: true}); !strings.Contains(got, want) {
		t.Errorf("legacy heading %q, want %q", got, want)
	}
	if got := RenderElementToHTML(ast.NewHeadingElement(pos, 2, input, "h"), vars, nil); !strings.Contains(got, "<code>N</code>") {
		t.Errorf("without the option the code span substitutes, as before: %q", got)
	}
}

// Un encabezado legado sin HeadingSource (llega así por un --filter) se
// sustituye sobre su HTML: con la opción, el <code> queda literal.
func TestLiteralCode_LegacyHeadingWithoutSource(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"n": "N"}
	el := ast.NewTextElement(pos, `<h2 id="h">Usa <code>{{n}}</code> y {{n}}</h2>`)
	el.IsRawHTML = true
	el.Level = 2
	if got := RenderElementToHTML(el, vars, &RenderContext{LiteralCode: true}); !strings.Contains(got, "<code>{{n}}</code> y N") {
		t.Errorf("literal: %q", got)
	}
	if got := RenderElementToHTML(el, vars, nil); !strings.Contains(got, "<code>N</code> y N") {
		t.Errorf("default: %q", got)
	}
}

// El TOC del documento arma sus títulos con HeadingContentHTML desde las
// variables de GenerateDocumentHTML, no desde RenderElementToHTML: falla si
// GenerateDocumentHTML deja de marcar las variables con la opción.
func TestGenerateDocumentHTML_LiteralCodeReachesTheTOC(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "Demo"
	block := ast.NewContentBlock(pos, "title")
	block.Heading = "A"
	heading := ast.NewHeadingElement(pos, 2, "Usa `{{title}}`", "usa")
	block.Elements = []ast.Element{LegacyHeadingElement(heading), ast.NewTextElement(pos, "texto")}
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out := GenerateDocumentHTML(doc, DocumentHTMLOptions{TOC: true, TOCDepth: 3}, &RenderContext{LiteralCode: true})
	if strings.Contains(out, "<code>Demo</code>") {
		t.Errorf("a code span was substituted (TOC or heading):\n%s", out)
	}
	if strings.Count(out, "<code>{{title}}</code>") < 2 {
		t.Errorf("want the literal code span in the heading and in the TOC:\n%s", out)
	}
}

// PopulateLangRunsWithOptions con LiteralCode deriva los LangRuns del mismo
// texto que el contentHTML: un valor sustituido dentro de código ya no puede
// abrir un span de idioma.
func TestPopulateLangRuns_LiteralCodeOption(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"v": "a` [mal]{lang=de} `b"}
	for _, tc := range []struct {
		literal bool
		langs   []string
	}{{true, []string{"fr"}}, {false, []string{"de", "fr"}}} {
		el := ast.NewTextElement(pos, "`{{v}}` [hola]{lang=fr}")
		PopulateLangRunsWithOptions(newTestDoc(el), vars, InlineHTMLOptions{LiteralCode: tc.literal})
		var got []string
		for _, r := range el.LangRuns {
			got = append(got, r.Lang)
		}
		if strings.Join(got, ",") != strings.Join(tc.langs, ",") {
			t.Errorf("literal=%v: langs %v, want %v", tc.literal, got, tc.langs)
		}
	}
}

// SetLiteralCode es la forma por interfaz de prender la opción, para un CLI
// que compila contra un core publicado sin el campo.
func TestRenderContext_SetLiteralCode(t *testing.T) {
	ctx := &RenderContext{}
	var setter interface{ SetLiteralCode(bool) } = ctx
	setter.SetLiteralCode(true)
	code := ast.NewCodeElement(diagnostics.NewPosition(1, 1), "yaml", "a: {{x}}")
	if out := RenderElementToHTML(code, map[string]interface{}{"x": "X"}, ctx); !strings.Contains(out, "a: {{x}}") {
		t.Errorf("SetLiteralCode(true) must keep the code literal: %s", out)
	}
}

// Hallazgo 5 del review de #497: el título de una sección (`# Uno {{x}}
// `{{x}}“) respeta sus code spans como un encabezado `##`, en el h1, en el TOC
// (con el mismo anchor que el h1) y en el TitleHTML/HeadingHTML del AST.
func TestLiteralCode_SectionTitles(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"title": "X"}
	build := func() *ast.AST {
		doc := ast.NewAST(pos)
		doc.FrontMatter = ast.NewFrontMatterNode(pos)
		doc.FrontMatter.Title = "X"
		first := ast.NewContentBlock(pos, "title")
		first.Heading = "Portada {{title}} `{{title}}`"
		second := ast.NewContentBlock(pos, "content")
		second.Title = "Uno {{title}} `{{title}}`"
		second.Elements = []ast.Element{ast.NewTextElement(pos, "texto")}
		doc.ContentBlocks = []ast.ContentBlock{*first, *second}
		return doc
	}
	for _, literal := range []bool{true, false} {
		doc := build()
		out := GenerateDocumentHTML(doc, DocumentHTMLOptions{TOC: true}, &RenderContext{LiteralCode: literal})
		want := "Uno X `X`"
		if literal {
			want = "Uno X `{{title}}`"
		}
		h1 := regexp.MustCompile(`<h1 id="([^"]*)">` + regexp.QuoteMeta(EscapeHTML(want)) + `</h1>`).FindStringSubmatch(out)
		if h1 == nil {
			t.Fatalf("literal=%v: no <h1> with %q:\n%s", literal, want, out)
		}
		if !strings.Contains(out, `<a href="#`+h1[1]+`">`+EscapeHTML(want)+`</a>`) {
			t.Errorf("literal=%v: the TOC entry must link to the h1 id %q", literal, h1[1])
		}

		doc = build()
		PopulateInlineHTMLWithOptions(doc, vars, InlineHTMLOptions{LiteralCode: literal})
		wantTitle, wantHeading := "Uno X `X`", "Portada X `X`"
		if literal {
			wantTitle, wantHeading = "Uno X `{{title}}`", "Portada X `{{title}}`"
		}
		if doc.ContentBlocks[1].TitleHTML != wantTitle || doc.ContentBlocks[0].HeadingHTML != wantHeading {
			t.Errorf("literal=%v: TitleHTML %q HeadingHTML %q", literal, doc.ContentBlocks[1].TitleHTML, doc.ContentBlocks[0].HeadingHTML)
		}
	}
}

// Los LangRuns de un encabezado tipado se derivan con la misma sustitución
// que su HTML: con la opción, un valor dentro de un code span no abre un span
// de idioma.
func TestPopulateLangRuns_LiteralCodeHeading(t *testing.T) {
	vars := map[string]interface{}{"v": "a` [mal]{lang=de} `b"}
	for _, tc := range []struct {
		literal bool
		langs   []string
	}{{true, []string{"fr"}}, {false, []string{"de", "fr"}}} {
		el := &ast.HeadingElement{Level: 2, Text: "`{{v}}` [hola]{lang=fr}", Anchor: "h"}
		PopulateLangRunsWithOptions(newTestDoc(el), vars, InlineHTMLOptions{LiteralCode: tc.literal})
		var got []string
		for _, r := range el.LangRuns {
			got = append(got, r.Lang)
		}
		if strings.Join(got, ",") != strings.Join(tc.langs, ",") {
			t.Errorf("literal=%v: langs %v, want %v", tc.literal, got, tc.langs)
		}
	}
}

// Hallazgo 1 del review de #497: en una cita o un ::: el renderer junta un
// item con sus líneas de continuación y empareja los code spans sobre el item
// entero; la sustitución los empareja igual. Vale también para la prosa de un
// TEXT anidado en un ::: con elementos.
func TestLiteralCode_ListItemContinuation(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"x": "X"}
	src := "- corre `make {{x}}\n  --verbose` ahora {{x}}"
	for _, el := range []ast.Element{
		ast.NewQuoteElement(pos, src),
		&ast.SpecialBlockElement{BlockType: "note", Content: src},
		&ast.SpecialBlockElement{BlockType: "note", Content: src, Elements: []ast.Element{ast.NewTextElement(pos, src)}},
	} {
		out := RenderElementToHTML(el, vars, &RenderContext{LiteralCode: true})
		if !strings.Contains(out, "<code>make {{x}} --verbose</code> ahora X") {
			t.Errorf("%T: %s", el, out)
		}
	}
}
