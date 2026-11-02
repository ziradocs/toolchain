// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestRenderImageElement_UnavailableKeepsFigure: una imagen que falta en
// offline-inline (la ruta de PDF) no deja un <img> roto; el marcador lleva el
// alt, la leyenda numerada y el ancla de la figura, en el idioma del
// documento.
func TestRenderImageElement_UnavailableKeepsFigure(t *testing.T) {
	img := ast.NewImageElement(diagnostics.NewPosition(1, 1), "falta.png", `Plano <del> edificio`)
	img.Caption = "Planta baja"
	img.Label = "fig:plano"
	img.Number = 2
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = t.TempDir()

	got := renderImageElement(img, nil, ctx)
	for _, want := range []string{
		`<figure id="fig-plano">`,
		`<span class="unavailable-note">Imagen no disponible</span>`,
		`<span class="unavailable-title">Plano &lt;del&gt; edificio</span>`,
		`<figcaption>Figura 2: Planta baja</figcaption>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<img") {
		t.Errorf("an unavailable image must not leave an <img>: %s", got)
	}

	ctx.Lang = "en-US"
	if got := renderImageElement(img, nil, ctx); !strings.Contains(got, "Image not available") || !strings.Contains(got, "Figure 2:") {
		t.Errorf("English document should get the English note: %s", got)
	}
}

// TestRenderImageElement_BrowserModeKeepsRelativeSrc: en la vista previa del
// navegador el src relativo se conserva aunque el archivo no exista (lo
// resuelve el visor), pero una ruta que escapa de AssetRoot ya se sabe
// bloqueada y sale como marcador. Una ruta absoluta puede ser una URL
// relativa a la raíz del sitio: se conserva.
func TestRenderImageElement_BrowserModeKeepsRelativeSrc(t *testing.T) {
	ctx := NewDefaultRenderContext()
	ctx.AssetRoot = t.TempDir()
	pos := diagnostics.NewPosition(1, 1)

	if got := renderImageElement(ast.NewImageElement(pos, "img/falta.png", "Logo"), nil, ctx); got != `<img src="img/falta.png" alt="Logo">` {
		t.Errorf("missing file in browser mode: %s", got)
	}
	if got := renderImageElement(ast.NewImageElement(pos, "/static/logo.png", "Logo"), nil, ctx); got != `<img src="/static/logo.png" alt="Logo">` {
		t.Errorf("root-relative URL in browser mode: %s", got)
	}
	if got := renderImageElement(ast.NewImageElement(pos, "../../etc/x.png", "Logo"), nil, ctx); !strings.Contains(got, `class="image-unavailable"`) {
		t.Errorf("an escaping path in browser mode should be the marker: %s", got)
	}
	ctx.AssetRoot = ""
	if got := renderImageElement(ast.NewImageElement(pos, "../x.png", "Logo"), nil, ctx); got != `<img src="../x.png" alt="Logo">` {
		t.Errorf("without AssetRoot nothing is confined: %s", got)
	}
}

// TestRenderImageElement_UnsafeURLIsMarker: una URL peligrosa ya no imprime
// "Image URL blocked for security reasons" en inglés en el cuerpo.
func TestRenderImageElement_UnsafeURLIsMarker(t *testing.T) {
	got := renderImageElement(ast.NewImageElement(diagnostics.NewPosition(1, 1), "javascript:alert(1)", ""), nil, nil)
	if !strings.Contains(got, `class="image-unavailable"`) || strings.Contains(got, "security") || strings.Contains(got, "javascript") {
		t.Errorf("got %s", got)
	}
}
