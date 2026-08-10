// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// errFakeFetch simula el fallo de un fetcher (p. ej. el "context canceled"
// del issue #114 antes de que el fix de core/renderer/chromium impidiera que
// un segundo Render* sobre el mismo *ChromiumRenderer se envenenara).
var errFakeFetch = errors.New("simulated fetch failure")

// Antes de este fix (issue #114), un fetcher que fallaba en modo offline
// terminaba en un <div class="*-error"> dentro del HTML/PDF entregado, sin
// que nada se reportara por consola: el build salía con exit 0 y "0
// warnings" mientras horneaba el error en el output. Estos tests verifican
// que cada rama de error de html.go ahora reporta a través de ctx.Logger
// (el mismo mecanismo que ya usa generateThemeVariables, issue #134/G1c),
// además de seguir devolviendo el div de error (el HTML no cambia).

type failingMermaidFetcher struct{}

func (failingMermaidFetcher) FetchAndSave(ctx context.Context, mermaidCode, outputDir string) (string, error) {
	return "", errFakeFetch
}
func (failingMermaidFetcher) FetchInline(ctx context.Context, mermaidCode string) (string, error) {
	return "", errFakeFetch
}

func TestRenderMermaidOfflineInline_ReportsFetcherErrorToLogger(t *testing.T) {
	spy := &spyLogger{}
	ctx := NewDefaultRenderContext()
	ctx.Logger = spy
	ctx.MermaidFetcher = failingMermaidFetcher{}

	html := renderMermaidOfflineInline("graph TD\nA-->B", ctx)

	if !strings.Contains(html, "mermaid-error") {
		t.Fatalf("expected the error div to still be returned, got: %s", html)
	}
	if len(spy.warnings) != 1 {
		t.Fatalf("expected exactly 1 warning reported to ctx.Logger, got %d: %v", len(spy.warnings), spy.warnings)
	}
}

func TestRenderMermaidOfflineAssets_ReportsFetcherErrorToLogger(t *testing.T) {
	spy := &spyLogger{}
	ctx := NewDefaultRenderContext()
	ctx.Logger = spy
	ctx.MermaidFetcher = failingMermaidFetcher{}
	ctx.OutputDir = "out"

	html := renderMermaidOfflineAssets("graph TD\nA-->B", ctx)

	if !strings.Contains(html, "mermaid-error") {
		t.Fatalf("expected the error div to still be returned, got: %s", html)
	}
	if len(spy.warnings) != 1 {
		t.Fatalf("expected exactly 1 warning reported to ctx.Logger, got %d: %v", len(spy.warnings), spy.warnings)
	}
}

type failingMathFetcher struct{}

func (failingMathFetcher) FetchAndSave(ctx context.Context, latex, outputDir string) (string, error) {
	return "", errFakeFetch
}
func (failingMathFetcher) FetchInline(ctx context.Context, latex string) (string, error) {
	return "", errFakeFetch
}

func TestRenderMathOfflineInline_ReportsFetcherErrorToLogger(t *testing.T) {
	spy := &spyLogger{}
	ctx := NewDefaultRenderContext()
	ctx.Logger = spy
	ctx.MathFetcher = failingMathFetcher{}

	html := renderMathOfflineInline("E = mc^2", ctx)

	if !strings.Contains(html, "math-error") {
		t.Fatalf("expected the error div to still be returned, got: %s", html)
	}
	if len(spy.warnings) != 1 {
		t.Fatalf("expected exactly 1 warning reported to ctx.Logger, got %d: %v", len(spy.warnings), spy.warnings)
	}
}

type failingChartFetcher struct{}

func (failingChartFetcher) FetchAndSave(ctx context.Context, elem *ast.ChartElement, chartConfig, outputDir string, width, height int) (string, error) {
	return "", errFakeFetch
}
func (failingChartFetcher) FetchInline(ctx context.Context, elem *ast.ChartElement, chartConfig string, width, height int) ([]byte, error) {
	return nil, errFakeFetch
}
func (failingChartFetcher) GetImageFormat() string { return "png" }

func TestRenderChartOfflineInline_ReportsFetcherErrorToLogger(t *testing.T) {
	spy := &spyLogger{}
	ctx := NewDefaultRenderContext()
	ctx.Logger = spy
	ctx.ChartFetcher = failingChartFetcher{}

	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewChartElement(pos, "radar")

	html := renderChartOfflineInline(elem, `{"type":"radar"}`, 400, 300, ctx)

	if !strings.Contains(html, "chart-error") {
		t.Fatalf("expected the error div to still be returned, got: %s", html)
	}
	if len(spy.warnings) != 1 {
		t.Fatalf("expected exactly 1 warning reported to ctx.Logger, got %d: %v", len(spy.warnings), spy.warnings)
	}
}

type failingMapFetcher struct{}

func (failingMapFetcher) FetchAndSave(ctx context.Context, config MapConfig, outputDir string, width, height int) (string, error) {
	return "", errFakeFetch
}
func (failingMapFetcher) FetchInline(ctx context.Context, config MapConfig, width, height int) ([]byte, error) {
	return nil, errFakeFetch
}
func (failingMapFetcher) GetImageFormat() string { return "png" }

func TestRenderMapOfflineInline_ReportsFetcherErrorToLogger(t *testing.T) {
	spy := &spyLogger{}
	ctx := NewDefaultRenderContext()
	ctx.Logger = spy
	ctx.MapFetcher = failingMapFetcher{}

	html := renderMapOfflineInline(MapConfig{CenterLat: 1, CenterLng: 2, Zoom: 5}, 400, 300, ctx)

	if !strings.Contains(html, "map-error") {
		t.Fatalf("expected the error div to still be returned, got: %s", html)
	}
	if len(spy.warnings) != 1 {
		t.Fatalf("expected exactly 1 warning reported to ctx.Logger, got %d: %v", len(spy.warnings), spy.warnings)
	}
}

type failingPlantUMLFetcher struct{}

func (failingPlantUMLFetcher) FetchDiagramToAssets(ctx context.Context, content string) (string, error) {
	return "", errFakeFetch
}
func (failingPlantUMLFetcher) FetchDiagramInline(ctx context.Context, content string) (string, error) {
	return "", errFakeFetch
}

func TestRenderPlantUMLOfflineInline_ReportsFetcherErrorToLogger(t *testing.T) {
	spy := &spyLogger{}
	ctx := NewDefaultRenderContext()
	ctx.Logger = spy
	ctx.Fetcher = failingPlantUMLFetcher{}

	html := renderPlantUMLOfflineInline("Bob -> Alice", ctx)

	if !strings.Contains(html, "plantuml-error") {
		t.Fatalf("expected the error div to still be returned, got: %s", html)
	}
	if len(spy.warnings) != 1 {
		t.Fatalf("expected exactly 1 warning reported to ctx.Logger, got %d: %v", len(spy.warnings), spy.warnings)
	}
}
