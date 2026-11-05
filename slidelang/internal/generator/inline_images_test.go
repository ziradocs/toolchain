// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// onePixelPNG es un PNG de 1x1.
const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// Una imagen inline (en un párrafo, un ítem o una celda) se incrusta en
// offline-inline como la de bloque. Antes quedaba el src relativo y el PDF,
// que se imprime desde about:blank, mostraba el ícono de imagen rota.
func TestRenderHTMLPreview_InlineImagesAreInlinedOffline(t *testing.T) {
	dir := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if err := os.WriteFile(filepath.Join(dir, "a.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	pos := diagnostics.NewPosition(1, 1)
	text := ast.NewTextElement(pos, "Texto ![x](a.png) y ![falta](no.png) fin.")
	points := ast.NewPointsElement(pos)
	points.Items = []ast.PointItem{{Content: `Ítem ![y](a.png "t")`}}
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Imágenes"
	block.Elements = []ast.Element{text, points}
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	render := func(mode string) string {
		ctx := renderer.NewDefaultRenderContext()
		ctx.ImageMode, ctx.AssetRoot = mode, dir
		html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, ctx)
		if err != nil {
			t.Fatalf("RenderHTMLPreview: %v", err)
		}
		return html
	}

	html := render("offline-inline")
	uri := "data:image/png;base64," + onePixelPNG
	if !strings.Contains(html, `<img src="`+uri+`" alt="x">`) {
		t.Errorf("la imagen inline del párrafo no se incrustó:\n%s", html)
	}
	if !strings.Contains(html, `<img src="`+uri+`" alt="y" title="t">`) {
		t.Errorf("la imagen inline del ítem no se incrustó con su título:\n%s", html)
	}
	if !strings.Contains(html, `<img src="no.png" alt="falta">`) {
		t.Errorf("una imagen que falta debe quedar como estaba:\n%s", html)
	}

	if html := render("browser"); !strings.Contains(html, `<img src="a.png" alt="x">`) {
		t.Errorf("fuera de offline-inline el src relativo no cambia:\n%s", html)
	}
}

// Una ruta que sale de AssetRoot no se lee.
func TestInlineLocalInlineImages_Confined(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "afuera.png")
	png, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if err := os.WriteFile(outside, png, 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(outside) }()
	ctx := renderer.NewDefaultRenderContext()
	ctx.ImageMode, ctx.AssetRoot = "offline-inline", root
	tag := `<img src="../afuera.png" alt="a">`
	if got := inlineLocalInlineImages(tag, ctx); got != tag {
		t.Errorf("una ruta fuera de la raíz se incrustó: %s", got)
	}
}
