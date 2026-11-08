// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

const surfacesDoc = `---
title: "Surfaces"
---

# Surfaces

Text with a [link](https://example.com/) and [Ctrl]{.kbd}.

| A | B |
|---|---|
| 1 | odd |
| 2 | even |

> Quoted with a [link](https://example.com/quote).

:::info
Info alert with a [link](https://example.com/info).
:::

::: grid

::: column
Left column.
:::

::: column
Right column.
:::

:::
`

// surfaceProbe lee el estilo calculado de cada elemento tematizable.
const surfaceProbe = `(() => {
  const cs = (sel) => getComputedStyle(document.querySelector(sel));
  return {
    bodyColor: cs('body').color,
    tableBg: cs('table').backgroundColor,
    tableColor: cs('table td').color,
    kbdBg: cs('.slidelang-kbd').backgroundColor,
    kbdColor: cs('.slidelang-kbd').color,
    kbdBorder: cs('.slidelang-kbd').borderTopColor,
    gridBg: cs('.grid-column').backgroundColor,
    gridBorder: cs('.grid-column').borderTopColor,
    gridColor: cs('.grid-column').color,
    linkColor: cs('p a[href]').color,
    quoteBg: cs('blockquote').backgroundColor,
    quoteColor: cs('blockquote').color,
    quoteBorder: cs('blockquote').borderLeftColor,
    infoBg: cs('.alert-info').backgroundColor,
    infoColor: cs('.alert-info').color,
    infoLink: cs('.alert-info a[href]').color,
    quoteLink: cs('blockquote a[href]').color,
  };
})()`

func renderSurfaces(t *testing.T, ctx context.Context, vars map[string]string) map[string]string {
	t.Helper()
	doc, _ := parser.New(util.NewNoop()).ParseDocument(surfacesDoc, "surfaces.doclang")
	if doc == nil {
		t.Fatal("ParseDocument devolvió nil")
	}
	html := renderer.GenerateDocumentHTML(doc, renderer.DocumentHTMLOptions{
		Title:          "Surfaces",
		ThemeVariables: vars,
		// Modos offline sin elementos que los usen: así el HTML no lleva
		// ningún <script> de CDN y la página carga sin red.
		PlantUMLMode: "offline-inline",
		MermaidMode:  "offline-inline",
		ChartMode:    "offline-inline",
		MapMode:      "offline-inline",
		MathMode:     "offline-inline",
	}, renderer.NewDefaultRenderContext())

	path := filepath.Join(t.TempDir(), "surfaces.html")
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := chromedp.Run(ctx,
		chromedp.Navigate("file://"+path),
		chromedp.Evaluate(surfaceProbe, &got),
	); err != nil {
		t.Fatalf("chromedp: %v", err)
	}
	return got
}

// TestDocumentSurfaces_ComputedStyle comprueba en un Chromium de verdad las
// dos mitades del contrato de las variables de superficie: sin variables el
// estilo calculado es el de los literales de siempre (el render de los temas
// embebidos no cambia, porque ninguno las define), y con variables cada
// elemento toma la suya.
func TestDocumentSurfaces_ComputedStyle(t *testing.T) {
	path := requireTestChromiumPath(t)
	mgr := newChromiumManager(path, false, noopChromiumLogger{}, DefaultChromiumBrand)
	ctx, cancel, err := mgr.CreateContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, 60*time.Second)
	defer cancelTimeout()

	t.Run("sin variables, los literales de antes", func(t *testing.T) {
		got := renderSurfaces(t, ctx, nil)
		want := map[string]string{
			"tableBg":     "rgb(255, 255, 255)",
			"kbdBg":       "rgb(247, 247, 247)",
			"kbdBorder":   "rgb(204, 204, 204)",
			"gridBg":      "rgb(248, 250, 252)",
			"gridBorder":  "rgb(226, 232, 240)",
			"linkColor":   "rgb(0, 0, 238)", // el azul del navegador
			"quoteBg":     "rgb(247, 250, 252)",
			"quoteColor":  "rgb(74, 85, 104)",
			"quoteBorder": "rgb(52, 152, 219)",
			"infoBg":      "rgb(235, 248, 255)",
			"infoColor":   "rgb(44, 82, 130)",
			"infoLink":    "rgb(0, 0, 238)",
			"quoteLink":   "rgb(0, 0, 238)",
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s = %q, want %q", k, got[k], v)
			}
		}
		// Los textos que se heredaban se siguen heredando.
		for _, k := range []string{"tableColor", "kbdColor", "gridColor"} {
			if got[k] != got["bodyColor"] {
				t.Errorf("%s = %q, want el color del cuerpo %q", k, got[k], got["bodyColor"])
			}
		}
	})

	t.Run("tema oscuro, cada elemento toma su variable", func(t *testing.T) {
		got := renderSurfaces(t, ctx, map[string]string{
			"--doclang-page-bg":                 "#0f172a",
			"--doclang-text-color":              "#e2e8f0",
			"--doclang-table-bg":                "#111111",
			"--doclang-kbd-bg":                  "#222222",
			"--doclang-kbd-border":              "#333333",
			"--doclang-panel-bg":                "#444444",
			"--doclang-panel-border":            "#555555",
			"--doclang-body-link-color":         "#7dd3fc",
			"--doclang-blockquote-bg":           "#666666",
			"--doclang-blockquote-color":        "#eeeeee",
			"--doclang-blockquote-border":       "#38bdf8",
			"--doclang-callout-info-bg":         "#777777",
			"--doclang-callout-info-color":      "#dddddd",
			"--doclang-callout-info-border":     "#888888",
			"--doclang-callout-info-link-color": "#99aabb",
			"--doclang-blockquote-link-color":   "#ccbbaa",
		})
		want := map[string]string{
			"bodyColor":   "rgb(226, 232, 240)",
			"tableBg":     "rgb(17, 17, 17)",
			"tableColor":  "rgb(226, 232, 240)",
			"kbdBg":       "rgb(34, 34, 34)",
			"kbdBorder":   "rgb(51, 51, 51)",
			"kbdColor":    "rgb(226, 232, 240)",
			"gridBg":      "rgb(68, 68, 68)",
			"gridBorder":  "rgb(85, 85, 85)",
			"gridColor":   "rgb(226, 232, 240)",
			"linkColor":   "rgb(125, 211, 252)",
			"quoteBg":     "rgb(102, 102, 102)",
			"quoteColor":  "rgb(238, 238, 238)",
			"quoteBorder": "rgb(56, 189, 248)",
			"infoBg":      "rgb(119, 119, 119)",
			"infoColor":   "rgb(221, 221, 221)",
			"infoLink":    "rgb(153, 170, 187)",
			"quoteLink":   "rgb(204, 187, 170)",
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s = %q, want %q", k, got[k], v)
			}
		}
	})

	// Un tema que solo cambia el color de vínculo del cuerpo: la cita y la
	// alerta conservan su fondo claro literal, así que sus vínculos siguen con
	// el azul del navegador en vez del color claro del tema.
	t.Run("vínculo del tema, cita y alerta literales", func(t *testing.T) {
		got := renderSurfaces(t, ctx, map[string]string{
			"--doclang-body-link-color": "#7dd3fc",
		})
		want := map[string]string{
			"linkColor": "rgb(125, 211, 252)",
			"infoLink":  "rgb(0, 0, 238)",
			"quoteLink": "rgb(0, 0, 238)",
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s = %q, want %q", k, got[k], v)
			}
		}
	})
}
