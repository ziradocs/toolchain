// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// directiveWarnSpy es un util.Logger mínimo (vía util.NoopLogger embebido)
// que solo registra Warn(), para assertar el CONTENIDO del mensaje — mismo
// patrón que core/renderer's spyLogger.
type directiveWarnSpy struct {
	util.NoopLogger
	warnings []string
}

func (s *directiveWarnSpy) Warn(format string, args ...interface{}) {
	s.warnings = append(s.warnings, fmt.Sprintf(format, args...))
}

func (s *directiveWarnSpy) sawSubstring(substr string) bool {
	for _, w := range s.warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

// TestRenderHTMLPreview_FloatLeftDirective_AppliesToNextImage es el test
// discriminante de issue #72: antes de este fix, @float-left colgaba su
// clase slidelang-float-left del <div> VACÍO de la propia directiva —
// flotar un div vacío no hace nada. El CSS real
// (slidelang-element.slidelang-image.slidelang-float-left, ver
// css/assets/css/elements/images.css) exige las tres clases en el MISMO
// elemento que la imagen. Este test verifica el mecanismo general (issue
// #72, alcance completo): una directiva modificadora adjunta su clase al
// elemento SIGUIENTE, no se dibuja a sí misma.
func TestRenderHTMLPreview_FloatLeftDirective_AppliesToNextImage(t *testing.T) {
	pos := pos()
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements,
		ast.NewDirectiveNode(pos, "float-left"),
		ast.NewImageElement(pos, "photo.png", "a photo"),
	)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	html, err := g.RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	// No debe quedar ningún <div> de directiva suelto: la directiva se
	// consumió por completo, no se dibujó a sí misma.
	if strings.Contains(html, `data-element-type="directive"`) {
		t.Errorf("the float-left directive rendered its own element instead of being consumed as a modifier:\n%s", html)
	}

	// El <div> de la imagen debe llevar las tres clases que el selector CSS
	// exige, todas en el MISMO elemento.
	imageDiv := regexp.MustCompile(`<div class="[^"]*"\s*\n?\s*id="slidelang-element-image-[^"]*"`).FindString(html)
	if imageDiv == "" {
		t.Fatalf("could not find the image element's div, html:\n%s", html)
	}
	for _, want := range []string{"slidelang-element", "slidelang-image", "slidelang-float-left"} {
		if !strings.Contains(imageDiv, want) {
			t.Errorf("image div is missing class %q (all three must be on the SAME element for the CSS selector to match), got: %s", want, imageDiv)
		}
	}
}

// TestRenderHTMLPreview_ImageWithoutDirective_HasNoFloatClass es el
// contrapunto: una imagen sin ninguna directiva precedente no debe llevar
// slidelang-float-left "de sobra" — el mecanismo no debe filtrar clases
// entre elementos que no la pidieron.
func TestRenderHTMLPreview_ImageWithoutDirective_HasNoFloatClass(t *testing.T) {
	pos := pos()
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements, ast.NewImageElement(pos, "photo.png", "a photo"))
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	html, err := g.RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	// El check tiene que mirar SOLO la class del div de la imagen, no el
	// HTML completo: la hoja de estilos embebida (images.css) siempre
	// contiene el selector .slidelang-float-left como regla CSS, exista o
	// no un elemento que la use — strings.Contains sobre el html entero
	// daría un falso positivo.
	imageDiv := regexp.MustCompile(`<div class="[^"]*"\s*\n?\s*id="slidelang-element-image-[^"]*"`).FindString(html)
	if imageDiv == "" {
		t.Fatalf("could not find the image element's div, html:\n%s", html)
	}
	if strings.Contains(imageDiv, "slidelang-float-left") {
		t.Errorf("an image with no preceding directive must not carry slidelang-float-left, got: %s", imageDiv)
	}
}

// TestRenderHTMLPreview_TrailingModifierDirective_WarnsAndDiscards cubre
// el caso borde: una directiva modificadora al FINAL del slide no tiene a
// quién atarse. Se descarta, pero no en silencio.
func TestRenderHTMLPreview_TrailingModifierDirective_WarnsAndDiscards(t *testing.T) {
	pos := pos()
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements,
		ast.NewTextElement(pos, "some text"),
		ast.NewDirectiveNode(pos, "float-left"),
	)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	spy := &directiveWarnSpy{}
	g := New(spy)
	html, err := g.RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	if strings.Contains(html, `data-element-type="directive"`) {
		t.Errorf("a trailing modifier directive must not render its own element either, got:\n%s", html)
	}
	if !spy.sawSubstring("float-left") {
		t.Errorf("expected a warning naming the discarded directive class, got: %v", spy.warnings)
	}
}

// TestRenderHTMLPreview_MultipleModifiers_AllApplyToSameElement confirma
// que varias directivas modificadoras consecutivas se acumulan y aplican
// todas al mismo elemento siguiente.
func TestRenderHTMLPreview_MultipleModifiers_AllApplyToSameElement(t *testing.T) {
	pos := pos()
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements,
		ast.NewDirectiveNode(pos, "float-left"),
		ast.NewDirectiveNode(pos, "large"),
		ast.NewImageElement(pos, "photo.png", "a photo"),
	)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	html, err := g.RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	imageDiv := regexp.MustCompile(`<div class="[^"]*"\s*\n?\s*id="slidelang-element-image-[^"]*"`).FindString(html)
	if imageDiv == "" {
		t.Fatalf("could not find the image element's div, html:\n%s", html)
	}
	for _, want := range []string{"slidelang-float-left", "slidelang-text-large"} {
		if !strings.Contains(imageDiv, want) {
			t.Errorf("image div is missing class %q from an accumulated modifier, got: %s", want, imageDiv)
		}
	}
}

// TestRenderHTMLPreview_TimerDirective_StillRendersItsOwnElement confirma
// que timer/transition NO se convirtieron en modificadores — siguen
// dibujando su propio widget, sin cambios.
func TestRenderHTMLPreview_TimerDirective_StillRendersItsOwnElement(t *testing.T) {
	pos := pos()
	directive := ast.NewDirectiveNode(pos, "timer")
	directive.Parameters["duration"] = "30"
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements, directive)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	html, err := g.RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	if !strings.Contains(html, "slidelang-slide-timer") {
		t.Errorf("expected the timer directive to still render its own widget, got:\n%s", html)
	}
}
