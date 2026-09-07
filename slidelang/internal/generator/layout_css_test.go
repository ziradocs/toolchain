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

func astWithSlideTypes(types ...string) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := &ast.AST{}
	for _, t := range types {
		block := ast.NewContentBlock(pos, t)
		block.Title = "Slide " + t
		block.Elements = []ast.Element{ast.NewTextElement(pos, "Contenido.")}
		doc.ContentBlocks = append(doc.ContentBlocks, *block)
	}
	return doc
}

func TestDetectRequiredLayoutsFromAST(t *testing.T) {
	g := New(util.NewNoop())

	for _, tc := range []struct {
		name  string
		types []string
		want  []string
	}{
		{"solo los que tienen CSS", []string{"comparison", "content", "stats"}, []string{"comparison", "stats"}},
		{"sin repetir", []string{"stats", "stats", "stats"}, []string{"stats"}},
		{"ordenado", []string{"testimonial", "comparison", "hero"}, []string{"comparison", "hero", "testimonial"}},
		{"nombre inventado se filtra", []string{"comparision", "no_existe"}, []string{}},
		{"solo content", []string{"content", "title"}, []string{}},
		{"mayúsculas se normalizan", []string{"STATS"}, []string{"stats"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := g.detectRequiredLayoutsFromAST(astWithSlideTypes(tc.types...))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("= %v, se esperaba %v", got, tc.want)
			}
		})
	}

	if got := g.detectRequiredLayoutsFromAST(nil); got != nil {
		t.Errorf("con AST nil se esperaba nil, se obtuvo %v", got)
	}
}

// El punto entero del issue #254: que el CSS del layout llegue al bundle y que
// su selector matchee lo que el template emite.
func TestRenderHTMLPreview_LayoutCSSReachesTheDOM(t *testing.T) {
	doc := astWithSlideTypes("comparison")

	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	if !strings.Contains(html, `data-slide-type="comparison"`) {
		t.Fatal("el slide no lleva data-slide-type")
	}
	// Se afirma sobre el SELECTOR y no sobre el comentario de sección: la ruta
	// de preview minifica el CSS y los comentarios no sobreviven. El selector
	// sí, y es lo que de verdad importa — que exista una regla capaz de
	// matchear el `data-slide-type` que el template acaba de emitir, que es
	// exactamente lo que faltaba en #254.
	if !strings.Contains(html, `[data-slide-type="comparison"]`) {
		t.Error("el CSS emitido no tiene un selector que matchee el DOM — es el bug de #254")
	}
	if !strings.Contains(html, "grid-template-columns") {
		t.Error("el CSS del layout comparison no llegó al bundle")
	}
}

// Un deck sin layouts con CSS no debe arrastrar la sección.
func TestRenderHTMLPreview_ContentOnlyDeckHasNoLayoutCSS(t *testing.T) {
	html, err := New(util.NewNoop()).RenderHTMLPreview(
		astWithSlideTypes("content", "content"), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	for _, layout := range []string{"comparison", "stats", "hero", "testimonial"} {
		if strings.Contains(html, `[data-slide-type="`+layout+`"]`) {
			t.Errorf("un deck solo-content emitió el CSS de %q", layout)
		}
	}
}

// Tipar un slide no puede quitarle el vestido que ya tenía: la base y los temas
// externos hacen key en .slidelang-content-slide (issue #254).
func TestRenderHTMLPreview_TypedSlidesKeepContentChrome(t *testing.T) {
	html, err := New(util.NewNoop()).RenderHTMLPreview(
		astWithSlideTypes("stats", "title", "section"), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	if !strings.Contains(html, "slidelang-stats-slide slidelang-content-slide") {
		t.Error("un slide `stats` perdió el vestido de contenido")
	}
	// Los que tienen fondo propio no deben recibirlo: se lo pisaría.
	for _, own := range []string{"slidelang-title-slide slidelang-content-slide", "slidelang-section-slide slidelang-content-slide"} {
		if strings.Contains(html, own) {
			t.Errorf("un slide con vestido propio recibió el de contenido: %q", own)
		}
	}

	// Un slide `content` NO debe recibirla dos veces: ya la trae por la vía
	// normal (`slidelang-{{.Type}}-slide`), y duplicarla en el atributo es un
	// error de html-validate (no-dup-class). Lo pescó el gate del corpus, no
	// los tests unitarios.
	contentHTML, err := New(util.NewNoop()).RenderHTMLPreview(
		astWithSlideTypes("content"), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	if strings.Contains(contentHTML, "slidelang-content-slide slidelang-content-slide") {
		t.Error("un slide `content` recibió la clase de contenido duplicada")
	}
}

// Issue #255: la opción llega al DOM como `data-layout-*` y el CSS la honra
// sin JavaScript. Es el mismo tipo de acoplamiento que rompió quiz/poll —
// atributo emitido por un lado y selector escrito por otro—, así que se ata
// con un test.
func TestRenderHTMLPreview_LayoutOptionsReachTheDOMAndTheCSS(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	comparison := ast.NewContentBlock(pos, "comparison")
	comparison.Title = "Comparación"
	comparison.Elements = []ast.Element{ast.NewTextElement(pos, "A")}
	comparison.LayoutConfig = &ast.LayoutConfig{Columns: 2}

	hero := ast.NewContentBlock(pos, "hero")
	hero.Title = "Titular"
	hero.Elements = []ast.Element{ast.NewTextElement(pos, "B")}
	hero.LayoutConfig = &ast.LayoutConfig{Align: "left"}

	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*comparison, *hero}}
	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	for _, attr := range []string{`data-layout-columns="2"`, `data-layout-align="left"`} {
		if !strings.Contains(html, attr) {
			t.Errorf("el DOM no emite %s", attr)
		}
	}
	for _, selector := range []string{`[data-layout-columns="2"]`, `[data-layout-align="left"]`} {
		if !strings.Contains(html, selector) {
			t.Errorf("el CSS no tiene ninguna regla para %s — el atributo no haría nada", selector)
		}
	}
}

// Un slide sin opciones no debe arrastrar atributos vacíos.
func TestRenderHTMLPreview_NoLayoutOptionsEmitsNoAttributes(t *testing.T) {
	html, err := New(util.NewNoop()).RenderHTMLPreview(
		astWithSlideTypes("comparison"), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	// Se mira el <div> del slide, no el documento entero: el bundle SIEMPRE
	// trae los selectores `[data-layout-columns="N"]` del CSS, que no son
	// atributos emitidos.
	start := strings.Index(html, "<div class=\"slidelang-slide")
	if start == -1 {
		t.Fatal("no se encontró el div del slide")
	}
	div := html[start : start+strings.Index(html[start:], ">")]
	for _, attr := range []string{"data-layout-columns", "data-layout-align"} {
		if strings.Contains(div, attr) {
			t.Errorf("un slide sin opciones emitió %s en su div:\n%s", attr, div)
		}
	}
}
