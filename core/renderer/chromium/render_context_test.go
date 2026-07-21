// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import "testing"

// TestNewRenderContext_BrowserModeFetchersAreTrueNil guarda contra el
// hallazgo de security-review sobre PR #176: RenderContext.{Fetcher,
// MermaidFetcher,ChartFetcher,MapFetcher} son interfaces (renderer/
// fetchers.go), no punteros concretos. Si NewRenderContext declarara sus
// locals como *XxxFetcher (puntero concreto) y los dejara sin asignar en
// modo "browser", asignar ese nil concreto al campo interfaz produciría una
// interfaz NO nil (type=*XxxFetcher, value=nil) — los `if ctx.XxxFetcher ==
// nil` de html.go dejarían de detectar el caso "sin fetcher" y en cambio
// panicarían al invocar un método sobre un receiver nil. Este test verifica
// el nil verdadero (interfaz nil, no solo valor nil) con cr=nil y todos los
// modos en "browser" (o vacío, que normaliza a "browser").
func TestNewRenderContext_BrowserModeFetchersAreTrueNil(t *testing.T) {
	ctx := NewRenderContext(nil, RenderContextOptions{})

	if ctx.Fetcher != nil {
		t.Errorf("expected ctx.Fetcher to be a true nil interface in browser mode, got %#v", ctx.Fetcher)
	}
	if ctx.MermaidFetcher != nil {
		t.Errorf("expected ctx.MermaidFetcher to be a true nil interface in browser mode, got %#v", ctx.MermaidFetcher)
	}
	if ctx.ChartFetcher != nil {
		t.Errorf("expected ctx.ChartFetcher to be a true nil interface in browser mode, got %#v", ctx.ChartFetcher)
	}
	if ctx.MapFetcher != nil {
		t.Errorf("expected ctx.MapFetcher to be a true nil interface in browser mode, got %#v", ctx.MapFetcher)
	}
}

// TestNewRenderContext_NilChromiumRendererKeepsMermaidChartMapNil cubre el
// mismo hallazgo con cr=nil pero modos offline pedidos explícitamente: sin
// un *ChromiumRenderer, mermaid/chart/map no pueden construirse (dependen de
// Chromium) y deben seguir siendo nil verdadero, no un nil envuelto en
// interfaz no-nil.
func TestNewRenderContext_NilChromiumRendererKeepsMermaidChartMapNil(t *testing.T) {
	ctx := NewRenderContext(nil, RenderContextOptions{
		MermaidMode: "offline-assets",
		ChartMode:   "offline-inline",
		MapMode:     "offline-assets",
	})

	if ctx.MermaidFetcher != nil {
		t.Errorf("expected ctx.MermaidFetcher to be a true nil interface with cr=nil, got %#v", ctx.MermaidFetcher)
	}
	if ctx.ChartFetcher != nil {
		t.Errorf("expected ctx.ChartFetcher to be a true nil interface with cr=nil, got %#v", ctx.ChartFetcher)
	}
	if ctx.MapFetcher != nil {
		t.Errorf("expected ctx.MapFetcher to be a true nil interface with cr=nil, got %#v", ctx.MapFetcher)
	}
}
