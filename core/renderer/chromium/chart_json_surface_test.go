// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
)

// TestSurfaceFor_JSONModeGetsNoSurface cubre la SEGUNDA puerta del hallazgo
// de code-review sobre la invariante de modo JSON.
//
// El gate nativo (SupportsNativeChartRenderingWithOptions) ya rechaza esos
// charts, y había un test que lo comprobaba — pero renderFunc cae a Chromium
// justo después, y ahí seguía pasando la superficie del tema. O sea que la
// config literal del autor terminaba dibujada sobre un fondo temátizado,
// rompiendo por la puerta de atrás la invariante que
// RenderContext.ChartCategoricalColors documenta. Cubrir solo una de las dos
// puertas fue lo que dejó pasar el bug.
func TestSurfaceFor_JSONModeGetsNoSurface(t *testing.T) {
	f := &ChartFetcher{}
	f.SetChartThemeColors(renderer.ChartThemeColors{Surface: "#101010"})

	jsonChart := &ast.ChartElement{
		ChartType:  "bar",
		IsJSONMode: true,
		RawJSON:    json.RawMessage(`{"type":"bar","data":{}}`),
	}
	if got := f.surfaceFor(jsonChart); got != "" {
		t.Errorf("un chart en modo JSON no debe recibir chart-surface, got %q", got)
	}

	plain := &ast.ChartElement{ChartType: "bar"}
	if got := f.surfaceFor(plain); got != "#101010" {
		t.Errorf("un chart normal sí debe recibirla, got %q", got)
	}

	// renderFunc es nil-safe con elem (lo documenta FetchAndSave), así que
	// surfaceFor también tiene que serlo.
	if got := f.surfaceFor(nil); got != "#101010" {
		t.Errorf("elem nil debe usar la superficie, got %q", got)
	}
}

// TestSurfaceFor_NoThemeIsEmpty confirma que sin tema la decisión sigue
// siendo "" — el valor que hace que buildChartHTML use el blanco histórico.
func TestSurfaceFor_NoThemeIsEmpty(t *testing.T) {
	f := &ChartFetcher{}
	if got := f.surfaceFor(&ast.ChartElement{ChartType: "bar"}); got != "" {
		t.Errorf("sin tema surfaceFor debe devolver \"\", got %q", got)
	}
}
