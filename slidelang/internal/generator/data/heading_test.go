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
