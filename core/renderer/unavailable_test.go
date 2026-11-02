// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestOfflineCaptureFailures_NoInternalErrorInBody: cuando una captura falla
// (mapa sin red, chart, mermaid, math, plantuml) o no hay renderer offline, el
// cuerpo lleva un marcador con la clase de error de siempre y una nota en el
// idioma del documento; el error de Go va solo al aviso del log.
func TestOfflineCaptureFailures_NoInternalErrorInBody(t *testing.T) {
	newCtx := func(lang string) (*RenderContext, *spyLogger) {
		spy := &spyLogger{}
		ctx := NewDefaultRenderContext()
		ctx.Logger = spy
		ctx.Lang = lang
		ctx.OutputDir = "out"
		ctx.MermaidFetcher = failingMermaidFetcher{}
		ctx.MathFetcher = failingMathFetcher{}
		ctx.ChartFetcher = failingChartFetcher{}
		ctx.MapFetcher = failingMapFetcher{}
		ctx.Fetcher = failingPlantUMLFetcher{}
		return ctx, spy
	}
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	cases := []struct {
		name, class, es, en string
		render              func(*RenderContext) string
	}{
		{"map inline", "map-error", "Mapa no disponible", "Map not available", func(c *RenderContext) string {
			return renderMapOfflineInline(MapConfig{CenterLat: 1, CenterLng: 2, Zoom: 5}, 400, 300, c)
		}},
		{"map assets", "map-error", "Mapa no disponible", "Map not available", func(c *RenderContext) string {
			return renderMapOfflineAssets(MapConfig{CenterLat: 1, CenterLng: 2, Zoom: 5}, 400, 300, c)
		}},
		{"chart inline", "chart-error", "Gráfica no disponible", "Chart not available", func(c *RenderContext) string {
			return renderChartOfflineInline(chart, "{}", 400, 300, c)
		}},
		{"chart assets", "chart-error", "Gráfica no disponible", "Chart not available", func(c *RenderContext) string {
			return renderChartOfflineAssets(chart, "{}", 400, 300, c)
		}},
		{"mermaid inline", "mermaid-error", "Diagrama no disponible", "Diagram not available", func(c *RenderContext) string {
			return renderMermaidOfflineInline("graph TD\nA-->B", c)
		}},
		{"mermaid assets", "mermaid-error", "Diagrama no disponible", "Diagram not available", func(c *RenderContext) string {
			return renderMermaidOfflineAssets("graph TD\nA-->B", c)
		}},
		{"math inline", "math-error", "Ecuación no disponible", "Equation not available", func(c *RenderContext) string {
			return renderMathOfflineInline("x^2", c)
		}},
		{"math assets", "math-error", "Ecuación no disponible", "Equation not available", func(c *RenderContext) string {
			return renderMathOfflineAssets("x^2", c)
		}},
		{"plantuml inline", "plantuml-error", "Diagrama no disponible", "Diagram not available", func(c *RenderContext) string {
			return renderPlantUMLOfflineInline("Bob -> Alice", c)
		}},
		{"plantuml assets", "plantuml-error", "Diagrama no disponible", "Diagram not available", func(c *RenderContext) string {
			return renderPlantUMLOfflineAssets("Bob -> Alice", c)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for lang, note := range map[string]string{"": tc.es, "es-MX": tc.es, "en": tc.en} {
				ctx, spy := newCtx(lang)
				html := tc.render(ctx)
				if !strings.HasPrefix(html, `<div class="`+tc.class+`"`) {
					t.Errorf("lang %q: want the %s div, got %s", lang, tc.class, html)
				}
				if !strings.Contains(html, note) {
					t.Errorf("lang %q: want note %q, got %s", lang, note, html)
				}
				if strings.Contains(html, errFakeFetch.Error()) || strings.Contains(strings.ToLower(html), "fail") {
					t.Errorf("lang %q: the internal error leaked into the body: %s", lang, html)
				}
				if len(spy.warnings) != 1 || !strings.Contains(spy.warnings[0], errFakeFetch.Error()) {
					t.Errorf("lang %q: the detail must stay in exactly one warning, got %v", lang, spy.warnings)
				}
			}
		})
	}

	// Sin renderer offline configurado: el texto viejo nombraba flags que no
	// existen (--map-mode=browser); ahora es el mismo marcador y un aviso.
	ctx := NewDefaultRenderContext()
	spy := &spyLogger{}
	ctx.Logger = spy
	for _, html := range []string{
		renderMapOfflineInline(MapConfig{}, 1, 1, ctx),
		renderChartOfflineInline(chart, "{}", 1, 1, ctx),
		renderMermaidOfflineInline("graph TD", ctx),
		renderMathOfflineInline("x", ctx),
	} {
		if strings.Contains(html, "--") || strings.Contains(html, "fetcher") || !strings.Contains(html, "no disponible") {
			t.Errorf("unconfigured renderer: %s", html)
		}
	}
	if len(spy.warnings) != 4 {
		t.Errorf("want one warning per unconfigured renderer, got %v", spy.warnings)
	}
}
