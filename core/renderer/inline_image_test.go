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
// relativo (lo resuelve el visor) salvo que escape de la raíz; sin AssetRoot
// no se toca nada, y una imagen remota nunca.
func TestInlineImages_BrowserMode(t *testing.T) {
	ctx := NewDefaultRenderContext()
	ctx.AssetRoot = t.TempDir()
	if html := RenderElementToHTML(inlineImageCases("falta.png")["párrafo"], nil, ctx); !strings.Contains(html, `<img src="falta.png" alt="logo">`) {
		t.Errorf("missing file in browser mode must keep src: %s", html)
	}
	if html := RenderElementToHTML(inlineImageCases("../../x.png")["lista"], nil, ctx); !strings.Contains(html, "image-unavailable-inline") {
		t.Errorf("escaping path in browser mode must be the marker: %s", html)
	}
	offline, _ := offlineInlineCtx(t)
	if html := RenderElementToHTML(inlineImageCases("https://x.test/a.png")["celda"], nil, offline); !strings.Contains(html, `<img src="https://x.test/a.png" alt="logo">`) {
		t.Errorf("remote image must be untouched: %s", html)
	}
	if html := RenderElementToHTML(inlineImageCases("falta.png")["párrafo"], nil, nil); !strings.Contains(html, `<img src="falta.png" alt="logo">`) {
		t.Errorf("nil ctx must be untouched: %s", html)
	}
}
