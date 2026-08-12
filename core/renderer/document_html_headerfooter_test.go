// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// Estos tests cubren issue #117: doclang parseaba `header:`/`footer:`/
// `layout_defaults:` a ast.HeaderFooterConfig pero generatePageViewBody
// nunca lo leía — un documento con header/footer válido en el front matter
// parseaba limpio y no dibujaba nada. Antes de este fix no existía ningún
// test de render para esta superficie en todo el repo.

func simpleDoc(blocks ...ast.ContentBlock) *ast.AST {
	return &ast.AST{ContentBlocks: blocks}
}

// TestGeneratePageViewBody_NilHeaderFooter_Unchanged verifica que opts.
// HeaderFooter == nil produce exactamente el output previo a #117: título +
// "Página N" hardcodeados, gateados solo por ShowHeaders/ShowFooters.
func TestGeneratePageViewBody_NilHeaderFooter_Unchanged(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		Title:       "Reporte",
		ShowHeaders: true,
		ShowFooters: true,
	}, nil)

	if !strings.Contains(html, `<span>Reporte</span>`) {
		t.Errorf("expected legacy hardcoded title span, got:\n%s", html)
	}
	if !strings.Contains(html, `<span>Página 1</span>`) {
		t.Errorf("expected legacy hardcoded page number span, got:\n%s", html)
	}
	if strings.Contains(html, `class="page-header-left"`) || strings.Contains(html, `class="page-footer-left"`) {
		t.Errorf("nil HeaderFooter must not emit any new zone markup, got:\n%s", html)
	}
}

// TestGeneratePageViewBody_FrontMatterOverridesThemeGate cubre la decisión
// 1 del plan de #117: un header.enabled:true en front matter se dibuja
// aunque el tema no sea page-view (ShowHeaders/ShowFooters false, el gate
// que antes gobernaba en solitario).
func TestGeneratePageViewBody_FrontMatterOverridesThemeGate(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		ShowHeaders: false,
		ShowFooters: false,
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{
				Enabled: true,
				Text:    &ast.HeaderFooterText{Center: "Confidencial"},
			},
		},
	}, nil)

	if !strings.Contains(html, `class="page-header-center"`) {
		t.Errorf("expected header to render from front matter despite ShowHeaders=false, got:\n%s", html)
	}
	if !strings.Contains(html, "Confidencial") {
		t.Errorf("expected header text, got:\n%s", html)
	}
	if !strings.Contains(html, "page-view-mode") {
		t.Errorf("expected body to get page-view-mode class from front matter config, got:\n%s", html)
	}
}

// TestGeneratePageViewBody_ExplicitDisableSuppressesEvenUnderThemeGate
// cubre el otro lado de la decisión 1: enabled:false explícito suprime el
// header aunque el tema sí sea page-view (ShowHeaders true).
func TestGeneratePageViewBody_ExplicitDisableSuppressesEvenUnderThemeGate(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		Title:       "Reporte",
		ShowHeaders: true,
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{Enabled: false},
		},
	}, nil)

	if strings.Contains(html, `<div class="page-header"`) {
		t.Errorf("explicit enabled:false must suppress the header entirely, got:\n%s", html)
	}
}

// TestGeneratePageViewBody_ZonesEscapeHTML verifica que las tres zonas de
// texto pasan por EscapeHTML (este archivo arma HTML con fmt.Fprintf, no
// html/template, que escaparía esto solo — decisión 5 del plan de #117).
func TestGeneratePageViewBody_ZonesEscapeHTML(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{
				Enabled: true,
				Text: &ast.HeaderFooterText{
					Left:   `<script>alert(1)</script>`,
					Center: "Centro",
					Right:  "Der",
				},
			},
		},
	}, nil)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("expected header text to be HTML-escaped, got unescaped script tag:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("expected escaped script tag, got:\n%s", html)
	}
}

// TestGeneratePageViewBody_LayoutDefaultsOverrideByBlockType cubre la
// cascada global → layout_defaults[blockType] con reemplazo total (no
// merge, paridad con slidelang): el bloque "title" usa su propio override
// en vez de heredar el header global.
func TestGeneratePageViewBody_LayoutDefaultsOverrideByBlockType(t *testing.T) {
	doc := simpleDoc(
		ast.ContentBlock{BlockType: "title", Heading: "Portada"},
		ast.ContentBlock{BlockType: "content", Title: "Sección 1"},
	)

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		PageBreaks: true,
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{
				Enabled: true,
				Text:    &ast.HeaderFooterText{Center: "Global"},
			},
			LayoutDefaults: map[string]*ast.LayoutHeaderFooterConfig{
				"title": {
					Header: &ast.HeaderConfig{
						Enabled: true,
						Text:    &ast.HeaderFooterText{Center: "Solo Portada"},
					},
				},
			},
		},
	}, nil)

	pages := strings.Split(html, `<div class="document-page">`)
	if len(pages) < 3 {
		t.Fatalf("expected 2 document-page divs (one per block), got %d segments:\n%s", len(pages), html)
	}
	if !strings.Contains(pages[1], "Solo Portada") {
		t.Errorf("expected the title page to use its layout_defaults override, got:\n%s", pages[1])
	}
	if strings.Contains(pages[1], ">Global<") {
		t.Errorf("layout_defaults override must replace the global header, not merge with it, got:\n%s", pages[1])
	}
	if !strings.Contains(pages[2], ">Global<") {
		t.Errorf("expected the content page to fall back to the global header, got:\n%s", pages[2])
	}
}

// TestGeneratePageViewBody_PageNumbers cubre el formato con
// {{current}}/{{total}}, StartFrom y ExcludeTitleSlides.
func TestGeneratePageViewBody_PageNumbers(t *testing.T) {
	doc := simpleDoc(
		ast.ContentBlock{BlockType: "title", Heading: "Portada"},
		ast.ContentBlock{BlockType: "content", Title: "Uno"},
		ast.ContentBlock{BlockType: "content", Title: "Dos"},
	)

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		PageBreaks: true,
		HeaderFooter: &ast.HeaderFooterConfig{
			Footer: &ast.FooterConfig{
				Enabled: true,
				PageNumbers: &ast.PageNumbersConfig{
					Enabled:            true,
					Format:             "{{current}} / {{total}}",
					ExcludeTitleSlides: true,
				},
			},
		},
	}, nil)

	pages := strings.Split(html, `<div class="document-page">`)
	if len(pages) < 4 {
		t.Fatalf("expected 3 document-page divs, got %d segments:\n%s", len(pages), html)
	}
	// La portada (blockType "title") no debe llevar numeración.
	if strings.Contains(pages[1], "page-numbers") {
		t.Errorf("expected the title page to have no page number (ExcludeTitleSlides), got:\n%s", pages[1])
	}
	if !strings.Contains(pages[2], `>2 / 3</span>`) {
		t.Errorf("expected page 2 of 3, got:\n%s", pages[2])
	}
	if !strings.Contains(pages[3], `>3 / 3</span>`) {
		t.Errorf("expected page 3 of 3, got:\n%s", pages[3])
	}
}

// TestGeneratePageViewBody_LogoBlocksDangerousScheme verifica que el src
// del logo pasa por SanitizeURL (mismo mecanismo que renderImageElement).
func TestGeneratePageViewBody_LogoBlocksDangerousScheme(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{
				Enabled: true,
				Logo:    &ast.LogoConfig{Source: `javascript:alert(1)`, Alt: "Logo"},
			},
		},
	}, nil)

	if strings.Contains(html, "javascript:") {
		t.Fatalf("expected a javascript: logo source to be blocked, got:\n%s", html)
	}
	if strings.Contains(html, `<img class="page-header-logo`) {
		t.Errorf("expected no logo <img> when the logo source is unsafe, got:\n%s", html)
	}
}

// TestGeneratePageViewBody_BorderRendersOnConfiguredEdges verifica que el
// borde solo se emite en los bordes que Position declara ("top", "bottom"
// o "both"), y que el valor de Style se escapa antes de interpolarse en el
// atributo style.
func TestGeneratePageViewBody_BorderRendersOnConfiguredEdges(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{
				Enabled: true,
				Text:    &ast.HeaderFooterText{Left: "x"},
				Border: &ast.BorderConfig{
					Enabled:  true,
					Position: "bottom",
					Color:    "#000",
				},
			},
		},
	}, nil)

	if strings.Contains(html, `class="page-header-border page-header-border-top"`) {
		t.Errorf("expected no top border when Position is \"bottom\", got:\n%s", html)
	}
	if !strings.Contains(html, `class="page-header-border page-header-border-bottom"`) {
		t.Errorf("expected a bottom border, got:\n%s", html)
	}
}

// TestGeneratePageViewBody_BorderStyleRejectsInjection verifica que
// border.Style pasa por sanitizeStyleValue (no por EscapeHTMLAttribute,
// que no bloquea ";") antes de interpolarse en el atributo
// style="border-bottom: ...". Un Style malicioso con ";" no debe poder
// abrir una segunda declaración CSS dentro del mismo atributo.
func TestGeneratePageViewBody_BorderStyleRejectsInjection(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{
				Enabled: true,
				Text:    &ast.HeaderFooterText{Left: "x"},
				Border: &ast.BorderConfig{
					Enabled:  true,
					Position: "bottom",
					Style:    "solid; background: url(http://evil/track)",
				},
			},
		},
	}, nil)

	if strings.Contains(html, "evil") {
		t.Fatalf("expected the malicious border.Style to be rejected outright, got:\n%s", html)
	}
	// Con el valor rechazado, debe caer al default "solid" (no quedar vacío).
	if !strings.Contains(html, "border-bottom: 1px solid #ccc;") {
		t.Errorf("expected the border to fall back to defaults when Style is rejected, got:\n%s", html)
	}
}

// TestGeneratePageViewBody_HeaderHeightPropagatesToContentPadding cubre el
// acoplamiento entre la chrome (.page-header) y .page-content: ambos deben
// leer el mismo valor de Height para que el padding-top del contenido no
// quede corto (y el texto se superponga con un header más alto que el
// default). openDocumentPageDiv resuelve esto emitiendo Height como
// custom property en el .document-page ancestro común.
func TestGeneratePageViewBody_HeaderHeightPropagatesToContentPadding(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{
				Enabled: true,
				Height:  "30mm",
				Text:    &ast.HeaderFooterText{Left: "x"},
			},
		},
	}, nil)

	if !strings.Contains(html, `<div class="document-page" style="--doclang-header-height: 30mm;">`) {
		t.Fatalf("expected --doclang-header-height custom property on .document-page, got:\n%s", html)
	}
	// El header ya no debe llevar height inline: lo hereda de la custom
	// property vía la regla `.page-header { height: var(--doclang-header-height, 15mm) }`.
	if strings.Contains(html, `<div class="page-header" style="height:`) {
		t.Errorf("expected no inline height on .page-header itself, got:\n%s", html)
	}
}

// TestGeneratePageViewBody_HeaderNumberingAdvancesEvenWithoutFooters
// pin-ea el comportamiento documentado en el docstring de
// generatePageViewBody: pageNum avanza en cada salto de página aunque
// ShowFooters sea false, a diferencia del código previo a issue #117
// (donde pageNum solo se incrementaba dentro del `if opts.ShowFooters`, y
// por lo tanto todo header repetía "Página 1" si los footers estaban
// apagados). Inalcanzable desde doclang hoy (su único caller iguala
// ambos flags), pero DocumentHTMLOptions es API pública de core.
func TestGeneratePageViewBody_HeaderNumberingAdvancesEvenWithoutFooters(t *testing.T) {
	doc := simpleDoc(
		ast.ContentBlock{BlockType: "title", Heading: "Uno"},
		ast.ContentBlock{BlockType: "content", Title: "Dos"},
		ast.ContentBlock{BlockType: "content", Title: "Tres"},
	)

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		Title:       "Reporte",
		ShowHeaders: true,
		ShowFooters: false,
		PageBreaks:  true,
	}, nil)

	for _, want := range []string{"Página 1", "Página 2", "Página 3"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected header numbering to advance across pages, missing %q in:\n%s", want, html)
		}
	}
}

// TestSanitizeStyleValue_RejectsBreakoutCharacters cubre la validación que
// protege el atributo style="..." armado a mano (decisión 5 del plan de
// #117): un valor de Height/Background con un carácter capaz de cerrar el
// atributo o inyectar una regla CSS se descarta en vez de interpolarse.
func TestSanitizeStyleValue_RejectsBreakoutCharacters(t *testing.T) {
	dangerous := []string{
		`60px" onmouseover="alert(1)`,
		`red; } </style><script>alert(1)</script>`,
		"red\nbackground: url(x)",
	}
	for _, v := range dangerous {
		if got := sanitizeStyleValue(v); got != "" {
			t.Errorf("sanitizeStyleValue(%q) = %q, want \"\"", v, got)
		}
	}
	if got := sanitizeStyleValue("60px"); got != "60px" {
		t.Errorf("sanitizeStyleValue(\"60px\") = %q, want \"60px\"", got)
	}
}
