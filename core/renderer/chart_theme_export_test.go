// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import (
	"context"
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

// --- Hallazgos de la segunda ronda de revisión sobre este PR ---

// capturingChartFetcher intercepta el chartConfig que renderChartElement le
// entrega al camino offline. Es el ÚNICO punto donde se puede observar lo
// que de verdad se manda a rasterizar: en modo offline el config no aparece
// en el HTML devuelto (ahí va un <img> al PNG ya generado), así que un test
// que mire el HTML no prueba nada de esto.
type capturingChartFetcher struct{ config string }

func (f *capturingChartFetcher) FetchAndSave(ctx context.Context, elem *ast.ChartElement, chartConfig string, outputDir string, width, height int) (string, error) {
	f.config = chartConfig
	return "charts/fake.png", nil
}

func (f *capturingChartFetcher) FetchInline(ctx context.Context, elem *ast.ChartElement, chartConfig string, width, height int) ([]byte, error) {
	f.config = chartConfig
	return []byte("fake"), nil
}

func (f *capturingChartFetcher) GetImageFormat() string { return "png" }

// TestRenderChartElement_ThemeReachesTheRealCallSite es el P1 más grave que
// encontró la revisión: los tokens existían en la API, tenían tests, y jamás
// llegaban a un chart real porque renderChartElement seguía llamando a
// GenerateChartConfigWithMode, que delega con el zero value.
//
// Este test afirma sobre el ÚNICO call site con un RenderContext en scope,
// no sobre la función que yo había agregado — que es exactamente la
// diferencia que dejó pasar el bug: mis tests anteriores ejercitaban una
// entrada que en producción nadie llamaba.
func TestRenderChartElement_ThemeReachesTheRealCallSite(t *testing.T) {
	fetcher := &capturingChartFetcher{}
	ctx := NewDefaultRenderContext()
	ctx.ChartMode = "offline-assets"
	ctx.ChartFetcher = fetcher
	ctx.OutputDir = t.TempDir()
	ctx.ChartThemeColors = ChartThemeColors{Grid: "#123456", Axis: "#654321", Label: "#abcdef"}

	renderChartElement(exportThemeChart(), nil, ctx)

	if fetcher.config == "" {
		t.Fatal("el fetcher nunca recibió un config")
	}
	for _, want := range []string{"#123456", "#654321", "#abcdef"} {
		if !strings.Contains(fetcher.config, want) {
			t.Errorf("el token %s no llegó al config que se manda a rasterizar:\n%s", want, fetcher.config)
		}
	}
}

// TestApplyChartThemeColors_AuthorAlwaysWins fija la semántica que habilita
// correr después del merge: el tema es un DEFAULT. Si el autor puso el color
// en su bloque options:, gana el autor — igual que los guards `=== undefined`
// de charts.js.
func TestApplyChartThemeColors_AuthorAlwaysWins(t *testing.T) {
	elem := exportThemeChart()
	elem.Options = map[string]interface{}{
		"scales": map[string]interface{}{
			"y": map[string]interface{}{
				"grid": map[string]interface{}{"color": "#author"},
			},
		},
	}
	cfg := decodeConfig(t, GenerateChartConfigWithTheme(elem, true, nil, ChartThemeColors{Grid: "#theme"}))

	if got := nestedColor(t, scaleOf(t, cfg, "y"), "grid"); got != "#author" {
		t.Errorf("scales.y.grid.color = %v, want #author (el tema no debe pisar al autor)", got)
	}
	if got := nestedColor(t, scaleOf(t, cfg, "x"), "grid"); got != "#theme" {
		t.Errorf("scales.x.grid.color = %v, want #theme (donde el autor no puso nada)", got)
	}
}

// TestApplyChartThemeColors_TitleAndY1AreReachable cubre el P2 de las ramas
// muertas: aplicar el tema dentro de applyExportOptimizations era inútil para
// plugins.title y scales.y1, porque esos los aporta el autor y llegan recién
// en MergeChartOptions, después. Correr al final los vuelve alcanzables.
func TestApplyChartThemeColors_TitleAndY1AreReachable(t *testing.T) {
	elem := exportThemeChart()
	elem.Options = map[string]interface{}{
		"plugins": map[string]interface{}{
			"title": map[string]interface{}{"display": true, "text": "T"},
		},
		"scales": map[string]interface{}{
			"y1": map[string]interface{}{"position": "right"},
		},
	}
	cfg := decodeConfig(t, GenerateChartConfigWithTheme(elem, true, nil, ChartThemeColors{Axis: "#eeeeee", Label: "#dddddd"}))

	plugins := cfg["options"].(map[string]interface{})["plugins"].(map[string]interface{})
	title, ok := plugins["title"].(map[string]interface{})
	if !ok {
		t.Fatal("no hay plugins.title")
	}
	if title["color"] != "#dddddd" {
		t.Errorf("plugins.title.color = %v, want #dddddd", title["color"])
	}
	if got := nestedColor(t, scaleOf(t, cfg, "y1"), "ticks"); got != "#eeeeee" {
		t.Errorf("scales.y1.ticks.color = %v, want #eeeeee", got)
	}
}

// TestApplyChartThemeColors_RadialScale cubre el P2 de radar/polarArea: no
// dibujan sobre x/y sino sobre "r", que applyExportOptimizations no crea.
// Antes quedaban con x/y temátizados que no se ven y su escala real intacta.
// El navegador (#228) ya pinta r + angleLines + pointLabels; el export
// converge acá.
func TestApplyChartThemeColors_RadialScale(t *testing.T) {
	for _, chartType := range []string{"radar", "polarArea"} {
		elem := exportThemeChart()
		elem.ChartType = chartType
		cfg := decodeConfig(t, GenerateChartConfigWithTheme(elem, true, nil, ChartThemeColors{
			Grid: "#111111", Axis: "#222222",
		}))
		r := scaleOf(t, cfg, "r")

		if got := nestedColor(t, r, "grid"); got != "#111111" {
			t.Errorf("%s: scales.r.grid.color = %v, want #111111", chartType, got)
		}
		if got := nestedColor(t, r, "angleLines"); got != "#111111" {
			t.Errorf("%s: scales.r.angleLines.color = %v, want #111111", chartType, got)
		}
		if got := nestedColor(t, r, "pointLabels"); got != "#222222" {
			t.Errorf("%s: scales.r.pointLabels.color = %v, want #222222", chartType, got)
		}
	}
}

// TestApplyChartThemeColors_TreemapLabels cubre el último P2: en un treemap
// la leyenda va apagada y las escalas se borran, así que el único texto
// visible vive en dataset.labels.color — cuyo default en
// chartjs-chart-treemap@4.2.0 es "black". Sin esto quedaba negro sobre una
// superficie oscura.
func TestApplyChartThemeColors_TreemapLabels(t *testing.T) {
	elem := &ast.ChartElement{
		ChartType: "treemap",
		Data: [][]interface{}{
			{"A", 10.0},
			{"B", 20.0},
		},
	}
	cfg := decodeConfig(t, GenerateChartConfigWithTheme(elem, true, nil, ChartThemeColors{Label: "#f0f0f0"}))

	data := cfg["data"].(map[string]interface{})
	datasets := data["datasets"].([]interface{})
	if len(datasets) == 0 {
		t.Fatal("el treemap no generó datasets")
	}
	dataset := datasets[0].(map[string]interface{})
	labels, ok := dataset["labels"].(map[string]interface{})
	if !ok {
		t.Fatal("el dataset del treemap no trae bloque labels")
	}
	if labels["color"] != "#f0f0f0" {
		t.Errorf("dataset.labels.color = %v, want #f0f0f0", labels["color"])
	}
	// Y la config generada sigue encendiendo las etiquetas, que es lo que
	// vuelve visible este color.
	if labels["display"] != true {
		t.Errorf("dataset.labels.display = %v, want true", labels["display"])
	}
}
