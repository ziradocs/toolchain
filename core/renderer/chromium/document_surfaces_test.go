// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
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

:::warning
Warning with a [link](https://example.com/warning).
:::

:::danger
Danger with a [link](https://example.com/danger).
:::

:::success
Success with a [link](https://example.com/success).
:::

:::tip
Tip with a [link](https://example.com/tip).
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
    warningLink: cs('.alert-warning a[href]').color,
    dangerLink: cs('.alert-danger a[href], .alert-error a[href]').color,
    successLink: cs('.alert-success a[href]').color,
    tipLink: cs('.alert-tip a[href]').color,
    quoteLink: cs('blockquote a[href]').color,
  };
})()`

func writeSurfacesHTML(t *testing.T, vars map[string]string) string {
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
	return path
}

func renderSurfaces(t *testing.T, ctx context.Context, vars map[string]string) map[string]string {
	t.Helper()
	path := writeSurfacesHTML(t, vars)
	var got map[string]string
	if err := chromedp.Do(ctx,
		chromedp.Navigate("file://"+path),
		store(&got, chromedp.Evaluate[map[string]string](surfaceProbe)),
	); err != nil {
		t.Fatalf("chromedp: %v", err)
	}
	return got
}

// pressedLinkColor presiona el botón del mouse sobre el vínculo (sin
// soltarlo encima, para no navegar) y lee su color mientras está :active.
func pressedLinkColor(t *testing.T, ctx context.Context, selector string) string {
	t.Helper()
	type point struct{ X, Y float64 }
	var pt point
	var color string
	mouse := func(ctx context.Context, t *chromedp.Target, p input.DispatchMouseEventParams) error {
		_, err := cdp.Call(ctx, t, input.DispatchMouseEvent, p)
		return err
	}
	err := chromedp.Do(ctx,
		store(&pt, chromedp.Evaluate[point](fmt.Sprintf(`(() => { const a = document.querySelector(%q); a.scrollIntoView({block: "center"}); const r = a.getBoundingClientRect(); return {X: r.left + r.width / 2, Y: r.top + r.height / 2}; })()`, selector))),
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			return mouse(ctx, t, input.DispatchMouseEventParams{Type: chromedp.MousePressed, X: pt.X, Y: pt.Y, Button: input.MouseButtonLeft, ClickCount: 1})
		}),
		store(&color, chromedp.Evaluate[string](fmt.Sprintf(`getComputedStyle(document.querySelector(%q)).color`, selector))),
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			if err := mouse(ctx, t, input.DispatchMouseEventParams{Type: chromedp.MouseMoved, X: 1, Y: 1}); err != nil {
				return err
			}
			return mouse(ctx, t, input.DispatchMouseEventParams{Type: chromedp.MouseReleased, X: 1, Y: 1, Button: input.MouseButtonLeft, ClickCount: 1})
		}),
	)
	if err != nil {
		t.Fatalf("chromedp: %v", err)
	}
	return color
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
			"warningLink": "rgb(0, 0, 238)",
			"dangerLink":  "rgb(0, 0, 238)",
			"successLink": "rgb(0, 0, 238)",
			"tipLink":     "rgb(0, 0, 238)",
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
			"linkColor":   "rgb(125, 211, 252)",
			"infoLink":    "rgb(0, 0, 238)",
			"warningLink": "rgb(0, 0, 238)",
			"dangerLink":  "rgb(0, 0, 238)",
			"successLink": "rgb(0, 0, 238)",
			"tipLink":     "rgb(0, 0, 238)",
			"quoteLink":   "rgb(0, 0, 238)",
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s = %q, want %q", k, got[k], v)
			}
		}
	})

	// Un vínculo presionado se pinta con ActiveText (rojo), como lo hacía la
	// hoja del navegador antes de que existieran las reglas de vínculo, en el
	// cuerpo y dentro de una cita o una alerta, con o sin color del tema.
	t.Run("vínculo presionado", func(t *testing.T) {
		for _, vars := range []map[string]string{nil, {
			"--doclang-body-link-color":         "#7dd3fc",
			"--doclang-blockquote-link-color":   "#ccbbaa",
			"--doclang-callout-info-link-color": "#99aabb",
		}} {
			path := writeSurfacesHTML(t, vars)
			if err := chromedp.Do(ctx, chromedp.Navigate("file://"+path)); err != nil {
				t.Fatal(err)
			}
			for _, sel := range []string{"p a[href]", "blockquote a[href]", ".alert-info a[href]", ".alert-tip a[href]"} {
				if got := pressedLinkColor(t, ctx, sel); got != "rgb(255, 0, 0)" {
					t.Errorf("vars=%v %s presionado = %q, want rgb(255, 0, 0)", vars != nil, sel, got)
				}
			}
		}
	})
}

const inkDoc = `---
title: "Inks"
---

# Inks

Page [p]{.danger} [Ctrl]{.kbd}.

| Head [h](https://example.com/th) [x]{.danger} | B |
|---|---|
| [t]{.danger} | odd |
| [s]{.danger} | even |

:::warning
Warning [w]{.danger} and **[b]{.danger}**.

| In alert |
|---|
| [n]{.danger} |
:::
`

// inkProbe lee el color calculado de cada tinta según la superficie donde va.
const inkProbe = `(() => {
  const c = (sel) => getComputedStyle(document.querySelector(sel)).color;
  return {
    page: c('p > .slidelang-text-danger'),
    th: c('thead th .slidelang-text-danger'),
    thLink: c('thead th a[href]'),
    td: c('table tbody tr:nth-child(1) .slidelang-text-danger'),
    stripe: c('table tbody tr:nth-child(2) .slidelang-text-danger'),
    alert: c('.alert-warning p > .slidelang-text-danger, .alert-warning .alert-content > .slidelang-text-danger'),
    alertBold: c('.alert-warning strong > .slidelang-text-danger'),
    alertTable: c('.alert-warning table .slidelang-text-danger'),
  };
})()`

func renderInks(t *testing.T, ctx context.Context, vars map[string]string) map[string]string {
	t.Helper()
	doc, _ := parser.New(util.NewNoop()).ParseDocument(inkDoc, "inks.doclang")
	if doc == nil {
		t.Fatal("ParseDocument devolvió nil")
	}
	html := renderer.GenerateDocumentHTML(doc, renderer.DocumentHTMLOptions{
		Title: "Inks", ThemeVariables: vars,
		PlantUMLMode: "offline-inline", MermaidMode: "offline-inline", ChartMode: "offline-inline",
		MapMode: "offline-inline", MathMode: "offline-inline",
	}, renderer.NewDefaultRenderContext())
	path := filepath.Join(t.TempDir(), "inks.html")
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := chromedp.Do(ctx,
		chromedp.Navigate("file://"+path),
		store(&got, chromedp.Evaluate[map[string]string](inkProbe)),
	); err != nil {
		t.Fatalf("chromedp: %v", err)
	}
	return got
}

// TestDocumentInks_ComputedStyle: sin variables cada tinta es la del
// documento en todas las superficies (el render de siempre); con la variable
// de una superficie, la tinta toma ese valor ahí y en lo que va dentro, y la
// superficie más interna gana (una tabla dentro de una alerta usa la de la
// tabla). El vínculo del encabezado de la tabla sigue al del cuerpo hasta
// que el tema le da el suyo.
func TestDocumentInks_ComputedStyle(t *testing.T) {
	path := requireTestChromiumPath(t)
	mgr := newChromiumManager(path, false, noopChromiumLogger{}, DefaultChromiumBrand)
	ctx, cancel, err := mgr.CreateContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, 60*time.Second)
	defer cancelTimeout()

	check := func(t *testing.T, got, want map[string]string) {
		t.Helper()
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s = %q, want %q", k, got[k], v)
			}
		}
	}
	const literal = "rgb(220, 38, 38)" // #dc2626

	t.Run("sin variables, la tinta de siempre", func(t *testing.T) {
		got := renderInks(t, ctx, nil)
		check(t, got, map[string]string{
			"page": literal, "th": literal, "td": literal, "stripe": literal,
			"alert": literal, "alertBold": literal, "alertTable": literal,
			"thLink": "rgb(0, 0, 238)",
		})
	})

	t.Run("tinta del documento y vínculo del cuerpo", func(t *testing.T) {
		got := renderInks(t, ctx, map[string]string{
			"--doclang-danger-color":    "#fca5a5",
			"--doclang-body-link-color": "#7dd3fc",
		})
		pink := "rgb(252, 165, 165)"
		check(t, got, map[string]string{
			"page": pink, "th": pink, "td": pink, "stripe": pink,
			"alert": pink, "alertBold": pink, "alertTable": pink,
			"thLink": "rgb(125, 211, 252)",
		})
	})

	t.Run("tinta por superficie", func(t *testing.T) {
		got := renderInks(t, ctx, map[string]string{
			"--doclang-danger-color":                 "#fca5a5",
			"--doclang-table-danger-color":           "#111111",
			"--doclang-table-stripe-danger-color":    "#222222",
			"--doclang-table-header-danger-color":    "#333333",
			"--doclang-callout-warning-danger-color": "#444444",
			"--doclang-body-link-color":              "#7dd3fc",
			"--doclang-table-header-link-color":      "#555555",
		})
		check(t, got, map[string]string{
			"page":       "rgb(252, 165, 165)",
			"td":         "rgb(17, 17, 17)",
			"stripe":     "rgb(34, 34, 34)",
			"th":         "rgb(51, 51, 51)",
			"alert":      "rgb(68, 68, 68)",
			"alertBold":  "rgb(68, 68, 68)", // un <strong> no reinicia la tinta
			"alertTable": "rgb(17, 17, 17)", // la tabla, más interna, gana
			"thLink":     "rgb(85, 85, 85)",
		})
	})
}
