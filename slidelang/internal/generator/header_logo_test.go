// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// astWithHeaderLogo arma un deck de un slide cuyo header trae el logo que se
// le pase.
func astWithHeaderLogo(t *testing.T, logo *ast.LogoConfig) string {
	t.Helper()
	pos := diagnostics.NewPosition(1, 1)

	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = []ast.Element{ast.NewTextElement(pos, "Contenido.")}

	fm := &ast.FrontMatterNode{
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{Enabled: true, Logo: logo},
		},
	}
	doc := &ast.AST{FrontMatter: fm, ContentBlocks: []ast.ContentBlock{*block}}

	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	return html
}

// Un override de header por layout que solo fija la altura deja el logo sin
// `source`, y el template emitía `<img src="">`: error de html-validate
// (`attribute-allowed-values`) y, en un navegador, una petición de red al
// propio documento. Pasaba de verdad en
// examples/18_specialized_layouts/18.4_headers_footers_advanced_flex.slidelang.
func TestRenderHTMLPreview_HeaderLogoWithoutSourceEmitsNoImg(t *testing.T) {
	html := astWithHeaderLogo(t, &ast.LogoConfig{Height: "50px"})

	if strings.Contains(html, `src=""`) {
		t.Error(`el header emitió un <img src=""> para un logo sin source`)
	}
	// Se busca el <div>, no la clase a secas: el CSS embebido en la página
	// también menciona slidelang-header-logo.
	if strings.Contains(html, `<div class="slidelang-header-logo`) {
		t.Error("se emitió el contenedor del logo aunque no hay imagen que mostrar")
	}
}

// Y el caso normal sigue emitiendo el logo.
func TestRenderHTMLPreview_HeaderLogoWithSourceIsEmitted(t *testing.T) {
	html := astWithHeaderLogo(t, &ast.LogoConfig{Source: "assets/logo.png", Height: "50px"})

	if !strings.Contains(html, `<div class="slidelang-header-logo`) {
		t.Error("falta el contenedor del logo")
	}
	if !strings.Contains(html, `src="assets/logo.png"`) {
		t.Error("falta el src del logo")
	}
}
