// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func exportThemeChart() *ast.ChartElement {
	return &ast.ChartElement{
		ChartType: "bar",
		Data: [][]interface{}{
			{"Q1", 10.0, 20.0},
			{"Q2", 30.0, 40.0},
		},
		Series: []string{"A", "B"},
	}
}

func decodeConfig(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("config no es JSON válido: %v", err)
	}
	return cfg
}

func scaleOf(t *testing.T, cfg map[string]interface{}, axis string) map[string]interface{} {
	t.Helper()
	options, ok := cfg["options"].(map[string]interface{})
	if !ok {
		t.Fatal("el config no trae options")
	}
	scales, ok := options["scales"].(map[string]interface{})
	if !ok {
		t.Fatal("el config no trae options.scales")
	}
	scale, ok := scales[axis].(map[string]interface{})
	if !ok {
		t.Fatalf("el config no trae la escala %q", axis)
	}
	return scale
}

func nestedColor(t *testing.T, parent map[string]interface{}, key string) interface{} {
	t.Helper()
	child, ok := parent[key].(map[string]interface{})
	if !ok {
		t.Fatalf("no existe el bloque %q", key)
	}
	return child["color"]
}

// TestGenerateChartConfigWithTheme_ScaleColors fija el mapeo verificado
// contra chart.js@4.5.1 (el bundle que cdn_tags.go fija), que registra
// scale.grid.color, scale.ticks.color y scale.border.color como opciones
// ruteadas reales.
func TestGenerateChartConfigWithTheme_ScaleColors(t *testing.T) {
	raw := GenerateChartConfigWithTheme(exportThemeChart(), true, nil, ChartThemeColors{
		Grid:  "#111111",
		Axis:  "#222222",
		Label: "#333333",
	})
	cfg := decodeConfig(t, raw)

	for _, axis := range []string{"x", "y"} {
		scale := scaleOf(t, cfg, axis)
		if got := nestedColor(t, scale, "grid"); got != "#111111" {
			t.Errorf("scales.%s.grid.color = %v, want #111111", axis, got)
		}
		if got := nestedColor(t, scale, "ticks"); got != "#222222" {
			t.Errorf("scales.%s.ticks.color = %v, want #222222", axis, got)
		}
		// border.color es la mitad que le falta al camino de navegador de
		// #228; sin ella la línea del eje quedaría de distinto color que en
		// el rasterizador nativo, que la pinta vía WithX/YAxisColor.
		if got := nestedColor(t, scale, "border"); got != "#222222" {
			t.Errorf("scales.%s.border.color = %v, want #222222", axis, got)
		}
	}

	options := cfg["options"].(map[string]interface{})
	plugins := options["plugins"].(map[string]interface{})
	legend := plugins["legend"].(map[string]interface{})
	if got := nestedColor(t, legend, "labels"); got != "#333333" {
		t.Errorf("plugins.legend.labels.color = %v, want #333333", got)
	}
}

// TestGenerateChartConfigWithTheme_ZeroValueByteForByte es la garantía que
// todo este PR promete: un caller sin tema produce EXACTAMENTE el config de
// antes.
func TestGenerateChartConfigWithTheme_ZeroValueByteForByte(t *testing.T) {
	elem := exportThemeChart()
	for _, forExport := range []bool{true, false} {
		old := GenerateChartConfigWithMode(elem, forExport, nil)
		zero := GenerateChartConfigWithTheme(elem, forExport, nil, ChartThemeColors{})
		if old != zero {
			t.Errorf("forExport=%v: el zero value cambió el config\n old=%s\nzero=%s", forExport, old, zero)
		}
		if strings.Contains(zero, `"color"`) {
			t.Errorf("forExport=%v: sin tema no debería emitirse ningún \"color\" de escala: %s", forExport, zero)
		}
	}
}

// TestGenerateChartConfigWithTheme_BrowserModeIgnoresTheme fija que el tema
// solo aplica al camino de export: el de navegador ya lo cubre #228 desde el
// cliente, y emitirlo también server-side daría dos fuentes de verdad para el
// mismo pixel.
func TestGenerateChartConfigWithTheme_BrowserModeIgnoresTheme(t *testing.T) {
	elem := exportThemeChart()
	themed := GenerateChartConfigWithTheme(elem, false, nil, ChartThemeColors{Grid: "#111111", Axis: "#222222"})
	plain := GenerateChartConfigWithMode(elem, false, nil)
	if themed != plain {
		t.Errorf("el modo navegador no debe cambiar con el tema\n themed=%s\n plain=%s", themed, plain)
	}
}

// TestGenerateChartConfigWithTheme_PartialTokens comprueba que cada token es
// independiente: declarar solo uno no arrastra a los demás.
func TestGenerateChartConfigWithTheme_PartialTokens(t *testing.T) {
	raw := GenerateChartConfigWithTheme(exportThemeChart(), true, nil, ChartThemeColors{Grid: "#abcdef"})
	cfg := decodeConfig(t, raw)
	x := scaleOf(t, cfg, "x")

	if got := nestedColor(t, x, "grid"); got != "#abcdef" {
		t.Errorf("grid.color = %v, want #abcdef", got)
	}
	if ticks, ok := x["ticks"].(map[string]interface{}); ok {
		if _, has := ticks["color"]; has {
			t.Error("sin chart-axis no debería emitirse ticks.color")
		}
	}
	if _, has := x["border"]; has {
		t.Error("sin chart-axis no debería crearse el bloque border")
	}
}

// TestRenderChartNativePNGWithTheme_JSONModeRejected fija la invariante que
// context.go documenta con más énfasis —la config literal del autor nunca se
// sobreescribe con el tema— en el seam donde de verdad aplica.
//
// Deliberadamente NO se prueba llamando a GenerateChartConfigWithTheme con un
// elem en modo JSON: esa función NUNCA se invoca para esos charts
// (renderChartElement re-serializa RawJSON y se la salta por completo), así
// que un test así comprobaría un camino inexistente y daría una falsa
// sensación de cobertura. El gate real para el camino nativo es
// SupportsNativeChartRenderingWithOptions, y es el que se afirma acá.
func TestRenderChartNativePNGWithTheme_JSONModeRejected(t *testing.T) {
	elem := &ast.ChartElement{
		ChartType:  "bar",
		IsJSONMode: true,
		RawJSON:    json.RawMessage(`{"type":"bar","data":{"labels":["a"],"datasets":[{"data":[1]}]}}`),
	}

	_, ok, err := RenderChartNativePNGWithTheme(elem, 400, 300, []string{"#ff0000"}, ChartThemeColors{
		Surface: "#111111",
		Grid:    "#222222",
	})
	if ok {
		t.Error("un chart en modo JSON no debe entrar al rasterizador nativo ni con tema puesto")
	}
	if err != nil {
		t.Errorf("el rechazo debe ser ok=false sin error, got %v", err)
	}
}
