// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import (
	"context"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestNativeChartFetcher_SeedAvoidsReRender prueba el fix de eficiencia de
// la review de PR #259 (issue #164): tras Seed, FetchInline devuelve los
// bytes sembrados SIN volver a rasterizar. Se siembra un centinela que NO es
// un PNG real — si FetchInline lo devuelve tal cual, es prueba directa de
// que consultó el seed en vez de re-llamar a RenderChartNativePNG (que
// habría producido bytes de PNG reales, distintos al centinela).
func TestNativeChartFetcher_SeedAvoidsReRender(t *testing.T) {
	elem := newTestChartElement("bar")
	width, height := ChartDimensions(elem)

	f := NewNativeChartFetcher()
	sentinel := []byte("SEEDED-NOT-A-REAL-PNG")
	f.Seed(elem, width, height, sentinel)

	got, err := f.FetchInline(context.Background(), elem, "", width, height)
	if err != nil {
		t.Fatalf("FetchInline: %v", err)
	}
	if string(got) != string(sentinel) {
		t.Errorf("FetchInline returned %q, want the seeded bytes %q — the seed cache was not consulted (chart re-rasterized)", got, sentinel)
	}
}

// TestNativeChartFetcher_FetchInlineWithoutSeedRenders confirma el otro
// lado: sin seed, FetchInline rasteriza de verdad y devuelve un PNG.
func TestNativeChartFetcher_FetchInlineWithoutSeedRenders(t *testing.T) {
	elem := newTestChartElement("bar")
	width, height := ChartDimensions(elem)

	f := NewNativeChartFetcher()
	got, err := f.FetchInline(context.Background(), elem, "", width, height)
	if err != nil {
		t.Fatalf("FetchInline: %v", err)
	}
	if len(got) < 8 || string(got[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("expected a real PNG (signature) from an unseeded FetchInline, got %d bytes", len(got))
	}
}

// TestNativeChartFetcher_UnsupportedChartErrors confirma que el fetcher es
// un error duro (no un fallback silencioso) cuando se le pide un chart que
// no soporta — el gate (tryBuildNativeContext) debe excluir esos antes de
// llegar acá, así que alcanzarlo es un bug del caller que debe fallar
// ruidosamente, no producir un placeholder.
func TestNativeChartFetcher_UnsupportedChartErrors(t *testing.T) {
	elem := ast.NewChartElement(diagnostics.Position{Line: 1}, "scatter") // sin renderer nativo
	elem.Data = [][]interface{}{{1.0, 2.0}}

	f := NewNativeChartFetcher()
	_, err := f.FetchInline(context.Background(), elem, "", 800, 600)
	if err == nil {
		t.Fatal("expected an error for an unsupported chart type, got nil — a fallback-less native fetcher must fail loudly, not silently")
	}
}
