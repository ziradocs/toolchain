// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// headingElementData corre la conversión sobre un deck de un solo slide con
// un solo elemento y devuelve el ElementData resultante.
func headingElementData(t *testing.T, el ast.Element, fm *ast.FrontMatterNode) ElementData {
	t.Helper()
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = []ast.Element{el}
	doc := &ast.AST{FrontMatter: fm, ContentBlocks: []ast.ContentBlock{*block}}

	got := PrepareTemplateDataWithRenderMode(doc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	if len(got.ContentBlocks) != 1 || len(got.ContentBlocks[0].Elements) != 1 {
		t.Fatalf("se esperaba 1 slide con 1 elemento, se obtuvo %d/%d",
			len(got.ContentBlocks), len(got.ContentBlocks[0].Elements))
	}
	return got.ContentBlocks[0].Elements[0]
}

// Issue #194: un encabezado de subsección llega como TextElement con HTML
// crudo. Si el converter lo tratara como texto normal, el template lo
// envolvería en <p> y markdownInline lo escaparía — la diapositiva
// mostraría "<h3 id=...>" literal.
func TestPrepareTemplateData_SubsectionHeadingKeepsHTML(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	el := ast.NewRawHTMLTextElement(pos, `<h3 id="foo">Foo</h3>`)
	el.Level = 3

	got := headingElementData(t, el, nil)

	if got.HeadingLevel != 3 {
		t.Errorf("HeadingLevel = %d, se esperaba 3", got.HeadingLevel)
	}
	if string(got.HeadingHTML) != `<h3 id="foo">Foo</h3>` {
		t.Errorf("HeadingHTML = %q", got.HeadingHTML)
	}
	if got.Content != "" {
		t.Errorf("Content debería quedar vacío para un heading, es %q", got.Content)
	}
}

// Un TextElement normal no debe tocar los campos de heading: es lo que hace
// que el template siga eligiendo la rama del <p>.
func TestPrepareTemplateData_PlainTextIsNotAHeading(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	got := headingElementData(t, ast.NewTextElement(pos, "### esto es texto"), nil)

	if got.HeadingHTML != "" || got.HeadingLevel != 0 {
		t.Errorf("un TextElement normal llenó los campos de heading: %q / %d",
			got.HeadingHTML, got.HeadingLevel)
	}
	if got.Content != "### esto es texto" {
		t.Errorf("Content = %q", got.Content)
	}
}

// El HTML del heading se interpola sin escapar, así que el valor de una
// {{variable}} sustituida DENTRO de él tiene que escaparse — si no, una
// variable con markup se convierte en un vector de inyección.
func TestPrepareTemplateData_SubsectionHeadingEscapesVariableValues(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	el := ast.NewRawHTMLTextElement(pos, `<h3 id="x">{{peligro}}</h3>`)
	el.Level = 3
	fm := &ast.FrontMatterNode{
		Variables: map[string]interface{}{"peligro": `<img src=x onerror=alert(1)>`},
	}

	got := headingElementData(t, el, fm)

	html := string(got.HeadingHTML)
	// Se busca la tag sin escapar, no la subcadena "onerror=": esa sigue
	// presente DENTRO del texto escapado, que es justamente lo correcto.
	if strings.Contains(html, "<img") {
		t.Errorf("el valor de la variable entró sin escapar: %q", html)
	}
	if !strings.Contains(html, "&lt;img") {
		t.Errorf("se esperaba el valor escapado en %q", html)
	}
	if !strings.HasPrefix(html, `<h3 id="x">`) {
		t.Errorf("el <h3> que lo envuelve se escapó también: %q", html)
	}
}

// El heading de una diapositiva tiene que salir igual que un párrafo con la
// misma entrada y las mismas variables: el párrafo sustituye en Content y el
// template le aplica markdownInline (ProcessInlineMarkdownSecureMultiline).
// Antes el heading sustituía sobre el <hN> ya armado por el parser, así que el
// valor de una variable usada como destino entraba al href sin pasar por el
// filtro de esquemas, y el Markdown de un valor salía literal.
func TestPrepareTemplateData_SubsectionHeadingMatchesParagraphText(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	for _, tc := range []struct {
		name  string
		input string
		vars  map[string]interface{}
	}{
		{"destino con esquema bloqueado", "[a]({{v}})", map[string]interface{}{"v": "javascript:alert(1)"}},
		{"destino permitido", "Ver [doc]({{u}})", map[string]interface{}{"u": "https://x.com/p?a=1&b=2"}},
		{"cursiva en el valor", "Hola {{n}}", map[string]interface{}{"n": "*Ana*"}},
		{"enlace en el valor", "Hola {{n}}", map[string]interface{}{"n": "[x](https://x.com)"}},
		{"HTML en el valor", "Hola {{n}}", map[string]interface{}{"n": "<b>x</b>"}},
		{"variable dentro del valor", "Hola {{n}}", map[string]interface{}{"n": "{{w}}", "w": "no"}},
		// Sin "{{" literal en la fuente: el placeholder solo aparece al
		// decodificar el destino, ya dentro del href emitido.
		{"placeholder con entidades en el destino", "[c](&#123;&#123;v}})", map[string]interface{}{"v": "javascript:alert(1)"}},
		{"placeholder con entidades en el src", "![c](&#123;&#123;v}})", map[string]interface{}{"v": "javascript:alert(1)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fm := &ast.FrontMatterNode{Variables: tc.vars}
			paragraph := headingElementData(t, ast.NewTextElement(pos, tc.input), fm)
			want := `<h3 id="h">` + renderer.ProcessInlineMarkdownSecureMultiline(paragraph.Content) + `</h3>`

			legacy := renderer.LegacyHeadingElement(ast.NewHeadingElement(pos, 3, tc.input, "h"))
			if got := string(headingElementData(t, legacy, fm).HeadingHTML); got != want {
				t.Errorf("HeadingHTML del legado\n got %q\nwant %q", got, want)
			}
			typed := ast.NewHeadingElement(pos, 3, tc.input, "h")
			if got := string(headingElementData(t, typed, fm).HeadingHTML); got != want {
				t.Errorf("HeadingHTML del tipado\n got %q\nwant %q", got, want)
			}
		})
	}
}
