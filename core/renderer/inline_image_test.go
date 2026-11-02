// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// inlineImageCases son una imagen inline en un párrafo, en un ítem de lista y
// en una celda: los tres caminos de markdown inline de un documento.
func inlineImageCases(src string) map[string]ast.Element {
	pos := diagnostics.NewPosition(1, 1)
	points := ast.NewPointsElement(pos)
	points.Items = append(points.Items, ast.PointItem{Content: "ítem con ![logo](" + src + ") al final"})
	table := ast.NewTableElement(pos)
	table.Headers = []string{"A", "B"}
	table.Rows = [][]string{{"![logo](" + src + ")", "x"}}
	return map[string]ast.Element{
		"párrafo": ast.NewTextElement(pos, "Texto con ![logo]("+src+") en medio."),
		"lista":   points,
		"celda":   table,
	}
}

func offlineInlineCtx(t *testing.T) (*RenderContext, string) {
	t.Helper()
	root := t.TempDir()
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = root
	return ctx, root
}

// TestInlineImages_OfflineInline: en offline-inline (la ruta del PDF) una
// imagen inline local se inlinea como data: URI, como una de bloque. Antes
// quedaba el src relativo y el PDF mostraba el ícono de imagen rota aunque
// el archivo existiera.
func TestInlineImages_OfflineInline(t *testing.T) {
	ctx, root := offlineInlineCtx(t)
	if err := os.WriteFile(filepath.Join(root, "logo.png"), []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, el := range inlineImageCases("logo.png") {
		html := RenderElementToHTML(el, nil, ctx)
		if !strings.Contains(html, `<img src="data:image/png;base64,`+base64Encode([]byte("PNGDATA"))+`" alt="logo">`) {
			t.Errorf("%s: inline image not inlined: %s", name, html)
		}
		// Idempotente: volver a pasar el HTML no cambia nada.
		if again := resolveInlineImages(html, ctx); again != html {
			t.Errorf("%s: resolveInlineImages is not idempotent", name)
		}
	}
}

// TestInlineImages_MissingOrBlocked: una imagen inline que falta o escapa de
// la raíz deja el marcador inline con el alt, en el idioma del documento.
func TestInlineImages_MissingOrBlocked(t *testing.T) {
	ctx, _ := offlineInlineCtx(t)
	for _, src := range []string{"falta.png", "../../fuera.png"} {
		for name, el := range inlineImageCases(src) {
			html := RenderElementToHTML(el, nil, ctx)
			if strings.Contains(html, "<img") || !strings.Contains(html, `<span class="image-unavailable-inline" role="img" aria-label="logo">Imagen no disponible: logo</span>`) {
				t.Errorf("%s %s: want the inline marker, got %s", name, src, html)
			}
		}
	}
	ctx.Lang = "en"
	if html := RenderElementToHTML(inlineImageCases("falta.png")["párrafo"], nil, ctx); !strings.Contains(html, "Image not available: logo") {
		t.Errorf("English marker: %s", html)
	}
}

// TestInlineImages_BrowserMode: en la vista previa se conserva el src
// relativo (lo resuelve el visor), también fuera de la raíz; sin AssetRoot
// no se toca nada, y una imagen remota permitida tampoco.
func TestInlineImages_BrowserMode(t *testing.T) {
	ctx := NewDefaultRenderContext()
	ctx.AssetRoot = t.TempDir()
	if html := RenderElementToHTML(inlineImageCases("falta.png")["párrafo"], nil, ctx); !strings.Contains(html, `<img src="falta.png" alt="logo">`) {
		t.Errorf("missing file in browser mode must keep src: %s", html)
	}
	if html := RenderElementToHTML(inlineImageCases("../images/logo.png")["lista"], nil, ctx); !strings.Contains(html, `<img src="../images/logo.png" alt="logo">`) {
		t.Errorf("a path outside AssetRoot in browser mode must keep its src: %s", html)
	}
	offline, _ := offlineInlineCtx(t)
	offline.SetAllowRemoteImages(true)
	if html := RenderElementToHTML(inlineImageCases("https://x.test/a.png")["celda"], nil, offline); !strings.Contains(html, `<img src="https://x.test/a.png" alt="logo">`) {
		t.Errorf("an allowed remote image must be untouched: %s", html)
	}
	if html := RenderElementToHTML(inlineImageCases("falta.png")["párrafo"], nil, nil); !strings.Contains(html, `<img src="falta.png" alt="logo">`) {
		t.Errorf("nil ctx must be untouched: %s", html)
	}
}

// TestRemoteImages_BlockedInBuildOutputs: en una salida de build (el PDF, el
// HTML offline) una imagen http(s) del documento, de bloque o inline, no se
// pide: sale el marcador con un aviso, y la CSP de la página solo admite
// imágenes 'self' y data:. Con AllowRemoteImages vuelve el comportamiento de
// siempre, y el HTML de modo browser no cambia.
func TestRemoteImages_BlockedInBuildOutputs(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	for _, mode := range []string{"offline-inline", "offline-assets"} {
		spy := &spyLogger{}
		ctx := NewDefaultRenderContext()
		ctx.ImageMode = mode
		ctx.AssetRoot = t.TempDir()
		ctx.Logger = spy
		for _, src := range []string{"https://x.test/a.png", "http://198.51.100.7/a.png", "//x.test/a.png"} {
			if html := RenderElementToHTML(ast.NewImageElement(pos, src, "Logo"), nil, ctx); strings.Contains(html, "<img") || !strings.Contains(html, "image-unavailable") {
				t.Errorf("%s block %s: %s", mode, src, html)
			}
			for name, el := range inlineImageCases(src) {
				if html := RenderElementToHTML(el, nil, ctx); strings.Contains(html, "<img") || !strings.Contains(html, "image-unavailable-inline") {
					t.Errorf("%s %s %s: %s", mode, name, src, html)
				}
			}
		}
		if len(spy.warnings) == 0 || !strings.Contains(spy.warnings[0], "--allow-remote-images") {
			t.Errorf("%s: want an actionable warning, got %v", mode, spy.warnings)
		}
	}

	browser := NewDefaultRenderContext()
	if html := RenderElementToHTML(ast.NewImageElement(pos, "https://x.test/a.png", "Logo"), nil, browser); html != `<img src="https://x.test/a.png" alt="Logo">` {
		t.Errorf("browser mode must keep remote images: %s", html)
	}

	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{{Elements: []ast.Element{ast.NewTextElement(pos, "hola")}}}}
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	if html := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, ctx); !strings.Contains(html, "img-src 'self' data:;") || strings.Contains(html, "img-src 'self' *") {
		t.Error("build-time output must use the offline CSP")
	}
	ctx.SetAllowRemoteImages(true)
	if html := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, ctx); !strings.Contains(html, "img-src 'self' * data:;") {
		t.Error("with remote images allowed the default CSP comes back")
	}
	if html := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, NewDefaultRenderContext()); !strings.Contains(html, "img-src 'self' * data:;") {
		t.Error("browser mode keeps the default CSP")
	}
}

// TestRemoteLogoAndMedia_BlockedInBuildOutputs: el logo del header y el
// src/poster de un <<video>> siguen la misma regla que las imágenes: en una
// salida de build, uno remoto no se pide sin AllowRemoteImages.
func TestRemoteLogoAndMedia_BlockedInBuildOutputs(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	ctx, root := offlineInlineCtx(t)
	spy := &spyLogger{}
	ctx.Logger = spy

	if got := renderHeaderLogo(&ast.LogoConfig{Source: "http://198.51.100.7/logo.png", Alt: "L"}, nil, ctx); got != "" {
		t.Errorf("remote logo in a build output: %s", got)
	}
	if err := os.WriteFile(filepath.Join(root, "logo.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := renderHeaderLogo(&ast.LogoConfig{Source: "logo.png", Alt: "L"}, nil, ctx); !strings.Contains(got, `src="data:image/png;base64,`) {
		t.Errorf("local logo should be inlined for the PDF: %s", got)
	}
	if got := renderHeaderLogo(&ast.LogoConfig{Source: "falta.png"}, nil, ctx); got != "" {
		t.Errorf("missing logo: %s", got)
	}

	video := ast.NewMediaElement(pos, "video", "http://198.51.100.7/v.mp4")
	if got := renderMediaElement(video, nil, ctx); strings.Contains(got, "198.51.100.7") || !strings.Contains(got, `class="media-error"`) {
		t.Errorf("remote video in a build output: %s", got)
	}
	if err := os.WriteFile(filepath.Join(root, "v.mp4"), []byte("MP4"), 0o644); err != nil {
		t.Fatal(err)
	}
	local := ast.NewMediaElement(pos, "video", "v.mp4")
	local.Poster = "https://198.51.100.7/poster.png"
	if got := renderMediaElement(local, nil, ctx); strings.Contains(got, "poster=") || !strings.Contains(got, "<video") {
		t.Errorf("remote poster in a build output must be dropped: %s", got)
	}
	if len(spy.warnings) < 3 {
		t.Errorf("want warnings for each blocked URL, got %v", spy.warnings)
	}

	ctx.SetAllowRemoteImages(true)
	if got := renderHeaderLogo(&ast.LogoConfig{Source: "http://198.51.100.7/logo.png"}, nil, ctx); !strings.Contains(got, "198.51.100.7") {
		t.Errorf("allowed remote logo: %s", got)
	}
	browser := NewDefaultRenderContext()
	if got := renderMediaElement(video, nil, browser); !strings.Contains(got, `src="http://198.51.100.7/v.mp4"`) {
		t.Errorf("browser mode keeps remote media: %s", got)
	}
}

// TestInlineImageCSS: las imágenes dentro de un párrafo, ítem o celda van en
// la línea; las de bloque siguen centradas.
func TestInlineImageCSS(t *testing.T) {
	css := generateDocumentStyles(DocumentHTMLOptions{}, nil)
	if !strings.Contains(css, "p img, li img, td img, th img, blockquote img, dd img, dt img {\n            display: inline;") {
		t.Error("no inline rule for images inside text")
	}
	if !strings.Contains(css, "img {\n            max-width: 100%;\n            height: auto;\n            display: block;") {
		t.Error("block images lost their rule")
	}
}
