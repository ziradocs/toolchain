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
// código (TestRenderCodeGroupLabelMatchesLabelHTML). Sin la opción (el valor cero, lo que usa
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
		if got, _ := proseVariables(tc.in, withLiteralCode(vars)); got != tc.want {
			t.Errorf("literal %q:\n got %q\nwant %q", tc.in, got, tc.want)
		}
		if got, _ := proseVariables(tc.in, vars); got != ProcessVariables(tc.in, vars) {
			want := ProcessVariables(tc.in, vars)
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
	if got, _ := proseVariables("`{{x}}`", vars); got != "`X`" {
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

// Con la opción, los campos de texto plano (el título de un :::, el caption y
// el alt de una imagen, el autor de una cita…) respetan sus code spans igual
// en el HTML y en su *HTML del AST: los dos caminos sustituyen lo mismo.
func TestLiteralCode_PlainFieldsMatchTheAST(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"x": "X"}
	field := "T {{x}} `{{x}}`"
	block := &ast.SpecialBlockElement{BlockType: "note", Title: field, Content: "c"}
	image := ast.NewImageElement(pos, "a.png", field)
	image.Caption = field
	quote := ast.NewQuoteElement(pos, "q")
	quote.Author = field
	for _, el := range []ast.Element{block, image, quote} {
		out := RenderElementToHTML(el, vars, &RenderContext{LiteralCode: true})
		if !strings.Contains(out, "T X `{{x}}`") || strings.Contains(out, "`X`") {
			t.Errorf("%T: the span must stay literal and the rest substituted: %s", el, out)
		}
	}
	PopulateInlineHTMLWithOptions(newTestDoc(block), vars, InlineHTMLOptions{LiteralCode: true})
	PopulateInlineHTMLWithOptions(newTestDoc(image), vars, InlineHTMLOptions{LiteralCode: true})
	PopulateInlineHTMLWithOptions(newTestDoc(quote), vars, InlineHTMLOptions{LiteralCode: true})
	for name, got := range map[string]string{"block title": block.TitleHTML, "alt": image.AltHTML, "caption": image.CaptionHTML, "author": quote.AuthorHTML} {
		if !strings.Contains(got, "{{x}}") || !strings.Contains(got, "T X") {
			t.Errorf("%s: %q", name, got)
		}
	}
}

// Hallazgo 6 del review de #497: con la opción, un valor sustituido en prosa no
// cambia los code spans de la fuente. Sus backticks no emparejan con los de la
// fuente y salen tal cual; un salto de línea en el valor tampoco cambia nada.
func TestLiteralCode_ValuesDoNotChangeCodeSpans(t *testing.T) {
	vars := map[string]interface{}{"tick": "`", "x": "X", "nl": "a\nb"}
	for _, src := range []string{"Pre {{tick}} mid `{{x}}` post {{x}}", "Pre {{nl}} mid `{{x}}` post {{x}}"} {
		for name, out := range map[string]string{
			"text":  RenderElementToHTML(ast.NewTextElement(diagnostics.Position{}, src), vars, &RenderContext{LiteralCode: true}),
			"quote": RenderElementToHTML(ast.NewQuoteElement(diagnostics.Position{}, src), vars, &RenderContext{LiteralCode: true}),
		} {
			if !strings.Contains(out, "<code>{{x}}</code> post X") {
				t.Errorf("%s %q: %s", name, src, out)
			}
		}
	}
	out := RenderElementToHTML(ast.NewTextElement(diagnostics.Position{}, "Pre {{tick}} mid"), vars, &RenderContext{LiteralCode: true})
	if !strings.Contains(out, "Pre ` mid") {
		t.Errorf("the value's backtick must come out as a backtick: %s", out)
	}
}

// P1 del review de 31b68e7: con la opción apagada, un valor que ya trae los
// caracteres de uso privado U+E000/U+E001 llega al LangRun igual que en main.
func TestPopulateLangRuns_OffKeepsPrivateUseCharacters(t *testing.T) {
	vars := map[string]interface{}{"zz": "abc"}
	el := ast.NewTextElement(diagnostics.Position{}, "[es {{zz}}]{lang=fr}")
	PopulateLangRuns(newTestDoc(el), vars)
	if len(el.LangRuns) != 1 || el.LangRuns[0].Text != "es abc" {
		t.Errorf("LangRuns %#v", el.LangRuns)
	}
}

// Con la opción, las marcas de los valores no se confunden con lo que ya
// traen el texto o los valores, el salto de línea vuelve como <br> en el
// texto y como espacio dentro de una etiqueta (un href), y el Text de un
// LangRun recibe el backtick y el salto de línea como texto.
func TestLiteralCode_ValueMarks(t *testing.T) {
	ctx := &RenderContext{LiteralCode: true}
	render := func(src string, vars map[string]interface{}) string {
		return RenderElementToHTML(ast.NewTextElement(diagnostics.Position{}, src), vars, ctx)
	}
	if out := render("x {{v}} y", map[string]interface{}{"v": "a\nb"}); !strings.Contains(out, "x a<br>b y") {
		t.Errorf("a line break in a value must come back as <br>: %s", out)
	}
	if out := render("pre  {{v}} `{{x}}`", map[string]interface{}{"v": "a`b", "x": "X"}); !strings.Contains(out, "pre  a`b <code>{{x}}</code>") {
		t.Errorf("a private-use character in the source must stay as it is: %q", out)
	}
	if out := render("{{v}} `{{x}}`", map[string]interface{}{"v": "p`q", "x": "X"}); !strings.Contains(out, "p`q <code>{{x}}</code>") {
		t.Errorf("private-use characters in a value must stay as they are: %q", out)
	}
	out := render("[link]({{v}})", map[string]interface{}{"v": "https://example.com/a\nb"})
	if !strings.Contains(out, `<a href="https://example.com/a b">link</a>`) {
		t.Errorf("inside a tag a line break comes back as a space, not <br>: %s", out)
	}
	// Un navegador quita los saltos de línea de una URL: devolverlo como salto
	// de línea armaría un "javascript:" que el filtro de esquemas no vio.
	if out := render("[x]({{v}})", map[string]interface{}{"v": "java\nscript:alert(1)"}); strings.Contains(out, "javascript:") || strings.Contains(out, "java\nscript") {
		t.Errorf("a line break in a URL must not come back as one: %s", out)
	}
	el := ast.NewTextElement(diagnostics.Position{}, "[es {{v}}]{lang=fr}")
	PopulateLangRunsWithOptions(newTestDoc(el), map[string]interface{}{"v": "a`b\nc"}, InlineHTMLOptions{LiteralCode: true})
	if len(el.LangRuns) != 1 || el.LangRuns[0].Text != "es a`b\nc" {
		t.Errorf("LangRuns %#v", el.LangRuns)
	}
}

// Los campos de texto plano que faltaban en la prueba anterior: el valor de
// una métrica, la etiqueta de un marcador de mapa y la de una pestaña de
// code-group respetan sus code spans con la opción.
func TestLiteralCode_MoreFields(t *testing.T) {
	vars := map[string]interface{}{"x": "X"}
	field := "T {{x}} `{{x}}`"
	metric := ast.NewMetricElement(diagnostics.Position{})
	metric.Label, metric.Value = "m", field
	m := ast.NewMapElement(diagnostics.Position{}, "world")
	m.Markers = []ast.MapMarker{{Lat: 1, Lng: 2, Label: field}}
	for _, el := range []ast.Element{metric, m} {
		out := RenderElementToHTML(el, vars, &RenderContext{LiteralCode: true})
		if !strings.Contains(out, "T X `{{x}}`") {
			t.Errorf("%T: %s", el, out)
		}
	}
	PopulateInlineHTMLWithOptions(newTestDoc(metric), vars, InlineHTMLOptions{LiteralCode: true})
	group := ast.NewCodeGroupElement(diagnostics.Position{})
	group.CodeBlocks = []ast.CodeBlock{{Language: "js", Label: field, Content: "x"}}
	PopulateInlineHTMLWithOptions(newTestDoc(group), vars, InlineHTMLOptions{LiteralCode: true})
	if metric.ValueHTML != "T X `{{x}}`" || group.CodeBlocks[0].LabelHTML != "T X `{{x}}`" {
		t.Errorf("ValueHTML %q LabelHTML %q", metric.ValueHTML, group.CodeBlocks[0].LabelHTML)
	}
}

// Los LangRuns de una cita o un ::: emparejan los code spans igual que su
// HTML: con la opción, sobre el item ya juntado con su continuación; sin ella,
// como en main, línea por línea.
func TestPopulateLangRuns_ListItemsLikeTheHTML(t *testing.T) {
	src := "- a `x\n  [y]{lang=fr} z` w"
	for _, literal := range []bool{true, false} {
		for _, el := range []ast.Element{ast.NewQuoteElement(diagnostics.Position{}, src), &ast.SpecialBlockElement{BlockType: "note", Content: src}} {
			PopulateLangRunsWithOptions(newTestDoc(el), map[string]interface{}{"q": "Q"}, InlineHTMLOptions{LiteralCode: literal})
			var runs []ast.LangRun
			switch e := el.(type) {
			case *ast.QuoteElement:
				runs = e.LangRuns
			case *ast.SpecialBlockElement:
				runs = e.LangRuns
			}
			html := RenderElementToHTML(el, map[string]interface{}{"q": "Q"}, &RenderContext{LiteralCode: literal})
			if literal && (len(runs) != 0 || strings.Contains(html, `lang="fr"`)) {
				t.Errorf("%T literal: the lang span is inside a code span: runs %v, html %s", el, runs, html)
			}
			if !literal && len(runs) != 1 {
				t.Errorf("%T off: runs %v, want main's one run", el, runs)
			}
		}
	}
}

// El LangRun de un encabezado recibe el backtick y el salto de línea del
// valor como texto, igual que los demás caminos.
func TestPopulateLangRuns_HeadingValueMarks(t *testing.T) {
	el := &ast.HeadingElement{Level: 2, Text: "[a {{v}}]{lang=fr}", Anchor: "h"}
	PopulateLangRunsWithOptions(newTestDoc(el), map[string]interface{}{"v": "b`c\nd"}, InlineHTMLOptions{LiteralCode: true})
	if len(el.LangRuns) != 1 || el.LangRuns[0].Text != "a b`c\nd" {
		t.Errorf("LangRuns %#v", el.LangRuns)
	}
}

// Con la opción, un CRLF o un CR suelto en un valor es un salto de línea: no
// queda un \r crudo en el HTML.
func TestLiteralCode_CRLFValues(t *testing.T) {
	for _, v := range []string{"a\r\nb", "a\rb"} {
		out := RenderElementToHTML(ast.NewTextElement(diagnostics.Position{}, "x {{v}} y"), map[string]interface{}{"v": v}, &RenderContext{LiteralCode: true})
		if strings.Contains(out, "\r") || !strings.Contains(out, "x a<br>b y") {
			t.Errorf("%q: %q", v, out)
		}
	}
}

// La etiqueta de una pestaña de code-group (```bash [etiqueta]) no es código:
// es un campo de una línea, como una leyenda, con las {{variables}}
// sustituidas fuera de sus code spans y sin Markdown. El botón de la pestaña
// en el HTML dice lo mismo que su labelHTML, con la opción y sin ella.
func TestRenderCodeGroupLabelMatchesLabelHTML(t *testing.T) {
	vars := map[string]interface{}{"v": "VALOR", "x": "<i>"}
	button := regexp.MustCompile(`<button[^>]*>([^<]*)</button>`)
	for _, tc := range []struct{ label, literal, off string }{
		{"tab {{v}}", "tab VALOR", "tab VALOR"},
		{"otra `{{v}}`", "otra `{{v}}`", "otra `VALOR`"},
		{"{{x}} **b**", "&lt;i&gt; **b**", "&lt;i&gt; **b**"},
		{"{{falta}}", "{{falta}}", "{{falta}}"},
	} {
		for _, literal := range []bool{true, false} {
			want := tc.off
			if literal {
				want = tc.literal
			}
			group := ast.NewCodeGroupElement(diagnostics.Position{})
			group.CodeBlocks = []ast.CodeBlock{{Language: "bash", Label: tc.label, Content: "echo {{v}}"}}
			out := RenderElementToHTML(group, vars, &RenderContext{LiteralCode: literal})
			m := button.FindStringSubmatch(out)
			if m == nil {
				t.Fatalf("no tab button in:\n%s", out)
			}
			PopulateInlineHTMLWithOptions(newTestDoc(group), vars, InlineHTMLOptions{LiteralCode: literal})
			if m[1] != want || group.CodeBlocks[0].LabelHTML != want {
				t.Errorf("%q literal=%v: HTML %q, labelHTML %q, want %q", tc.label, literal, m[1], group.CodeBlocks[0].LabelHTML, want)
			}
		}
	}
}
