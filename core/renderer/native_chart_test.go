// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	_ "image/png"
	"testing"

	"github.com/go-analyze/charts"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func newTestChartElement(chartType string) *ast.ChartElement {
	elem := ast.NewChartElement(diagnostics.Position{Line: 1}, chartType)
	elem.Title = "Test Chart"
	elem.Series = []string{"Series A", "Series B"}
	elem.Data = [][]interface{}{
		{"Q1", 10, 20},
		{"Q2", 15, 25},
		{"Q3", 20, 5},
	}
	return elem
}

// decodePNGAndCheck aplica el golden laxo que prescribe el plan para
// rasterización nativa (issue #130): decode + dimensiones + ratio de
// píxeles no-blancos, NO comparación byte-exacta — la salida de
// go-analyze/charts es, por diseño, visualmente distinta a la de Chart.js,
// así que un diff contra un PNG de referencia no tiene sentido acá.
func decodePNGAndCheck(t *testing.T, data []byte, wantWidth, wantHeight int) {
	t.Helper()

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}
	if format != "png" {
		t.Errorf("format = %q, want \"png\"", format)
	}

	bounds := img.Bounds()
	if bounds.Dx() != wantWidth || bounds.Dy() != wantHeight {
		t.Errorf("dimensions = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), wantWidth, wantHeight)
	}

	nonWhite := 0
	total := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 3 { // muestreo, no cada pixel
		for x := bounds.Min.X; x < bounds.Max.X; x += 3 {
			total++
			r, g, b, _ := img.At(x, y).RGBA()
			if (color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xff}) != (color.RGBA{0xff, 0xff, 0xff, 0xff}) {
				nonWhite++
			}
		}
	}
	if total == 0 {
		t.Fatal("no pixels sampled")
	}
	ratio := float64(nonWhite) / float64(total)
	if ratio < 0.01 {
		t.Errorf("non-white pixel ratio = %.4f, want >= 0.01 (image looks blank)", ratio)
	}
}

func TestRenderChartNativePNG_SupportedTypes(t *testing.T) {
	for _, chartType := range []string{"bar", "line", "pie", "doughnut"} {
		t.Run(chartType, func(t *testing.T) {
			elem := newTestChartElement(chartType)

			data, ok, err := RenderChartNativePNG(elem, 640, 480)
			if !ok {
				t.Fatalf("ok = false, want true for supported type %q", chartType)
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			decodePNGAndCheck(t, data, 640, 480)
		})
	}
}

func TestRenderChartNativePNG_UnsupportedTypesFallBack(t *testing.T) {
	tests := []struct {
		name string
		elem *ast.ChartElement
	}{
		{"combo", newTestChartElement("combo")},
		{"scatter", newTestChartElement("scatter")},
		{"unknown", newTestChartElement("totally-made-up-type")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, ok, err := RenderChartNativePNG(tt.elem, 640, 480)
			if ok {
				t.Fatalf("ok = true, want false (should fall back to chromedp) for %q", tt.elem.ChartType)
			}
			if err != nil {
				t.Errorf("unexpected error on graceful fallback: %v", err)
			}
			if data != nil {
				t.Errorf("data = %v, want nil when falling back", data)
			}
		})
	}
}

func TestRenderChartNativePNG_JSONModeFallsBack(t *testing.T) {
	elem := newTestChartElement("bar")
	elem.IsJSONMode = true
	elem.RawJSON = []byte(`{"type":"bar","data":{}}`)

	_, ok, err := RenderChartNativePNG(elem, 640, 480)
	if ok {
		t.Fatal("ok = true, want false: IsJSONMode must always fall back to chromedp regardless of ChartType")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRenderChartNativePNG_NoDataFallsBackWithError(t *testing.T) {
	elem := ast.NewChartElement(diagnostics.Position{Line: 1}, "bar")
	// Sin Data: es un tipo soportado pero sin datos que graficar.

	_, ok, err := RenderChartNativePNG(elem, 640, 480)
	if !ok {
		t.Fatal("ok = false, want true: chart type is supported, the failure is data-shape related")
	}
	if err == nil {
		t.Error("expected an error for a supported chart type with no data")
	}
}

// TestRenderChartNativePNG_JaggedRowsFallBack cubre un hallazgo de
// code-review sobre PR #163: antes, filas de largo irregular se
// zero-rellenaban en el índice correcto (más preciso que Chart.js, que
// compacta y desalinea la serie contra las categorías) - eso hacía que el
// mismo chart se viera DISTINTO según qué backend le tocara. Ahora datos
// irregulares caen a chromedp (igual que cualquier otro dato inválido para
// el camino nativo), preservando un único comportamiento.
func TestRenderChartNativePNG_JaggedRowsFallBack(t *testing.T) {
	elem := newTestChartElement("bar")
	elem.Data = [][]interface{}{
		{"Q1", 10, 20},
		{"Q2", 15}, // le falta la columna de la serie B
		{"Q3", 20, 5},
	}

	_, ok, err := RenderChartNativePNG(elem, 640, 480)
	if !ok {
		t.Fatal("ok = false, want true: bar is a supported type, the failure is data-shape related")
	}
	if err == nil {
		t.Error("expected an error for irregular row lengths, got nil (silent divergence from the chromedp path)")
	}
}

// TestRenderChartNativePNG_PieDoughnutIgnoreExtraColumns cubre un hallazgo
// de code-review: pie/doughnut solo leen row[0] (label) y row[1] (valor) —
// antes, chartSeriesValues se llamaba incondicionalmente ANTES del switch y
// fallaba con datos de pie/doughnut que traen columnas extra (p. ej. una
// columna de notas) que esa rama nunca usa.
func TestRenderChartNativePNG_PieDoughnutIgnoreExtraColumns(t *testing.T) {
	for _, chartType := range []string{"pie", "doughnut"} {
		t.Run(chartType, func(t *testing.T) {
			elem := newTestChartElement(chartType)
			elem.Data = [][]interface{}{
				{"A", 10, "nota irrelevante"},
				{"B", 20, "otra nota"},
			}

			data, ok, err := RenderChartNativePNG(elem, 640, 480)
			if !ok {
				t.Fatalf("ok = false, want true for supported type %q", chartType)
			}
			if err != nil {
				t.Fatalf("unexpected error with extra unused columns: %v", err)
			}
			decodePNGAndCheck(t, data, 640, 480)
		})
	}
}

// TestRenderChartNativePNG_OptionsFallBack cubre un hallazgo de code-review
// (originalmente sobre PR #163, sigue vigente tras el clasificador por
// hoja del issue #148 — ver native_chart_options.go): una hoja de
// elem.Options fuera del set reconocido (acá, un eje secundario scales.y1,
// que go-analyze/charts no puede reproducir) debe respetarse cayendo a
// chromedp en vez de descartarse en silencio. No toda clave de Options
// descalifica ya (ver TestClassifyChartOptions), pero esta sigue haciéndolo.
func TestRenderChartNativePNG_OptionsFallBack(t *testing.T) {
	elem := newTestChartElement("bar")
	elem.Options = map[string]interface{}{
		"scales": map[string]interface{}{
			"y1": map[string]interface{}{"type": "linear", "position": "right"},
		},
	}

	if SupportsNativeChartRenderingWithOptions(elem) {
		t.Fatal("SupportsNativeChartRenderingWithOptions() = true, want false for an unrecognized leaf")
	}

	_, ok, err := RenderChartNativePNG(elem, 640, 480)
	if ok {
		t.Fatal("ok = true, want false: custom Chart.js options must fall back to chromedp")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestChartSeriesValues_DataOrientation cubre un hallazgo de code-review:
// decodePNGAndCheck (dimensiones + ratio no-blanco) no puede detectar un bug
// de transposición o de mapeo label/valor - la imagen sigue siendo un PNG
// del tamaño correcto y no-vacío aunque los datos estén mal orientados. Este
// test verifica directamente la función pura de transformación de datos,
// sin pasar por rasterización, así que SÍ detecta ese tipo de bug.
func TestChartSeriesValues_DataOrientation(t *testing.T) {
	elem := newTestChartElement("bar")
	elem.Data = [][]interface{}{
		{"Q1", 10, 100},
		{"Q2", 20, 200},
		{"Q3", 30, 300},
	}

	values, categoryLabels, err := chartSeriesValues(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantCategories := []string{"Q1", "Q2", "Q3"}
	if len(categoryLabels) != len(wantCategories) {
		t.Fatalf("categoryLabels = %v, want %v", categoryLabels, wantCategories)
	}
	for i, want := range wantCategories {
		if categoryLabels[i] != want {
			t.Errorf("categoryLabels[%d] = %q, want %q", i, categoryLabels[i], want)
		}
	}

	// Orientación esperada: values[serie][categoría], NO values[categoría][serie].
	wantValues := [][]float64{
		{10, 20, 30},    // serie 0 (columna 1 de cada fila)
		{100, 200, 300}, // serie 1 (columna 2 de cada fila)
	}
	if len(values) != len(wantValues) {
		t.Fatalf("values has %d series, want %d", len(values), len(wantValues))
	}
	for seriesIdx, wantSeries := range wantValues {
		if len(values[seriesIdx]) != len(wantSeries) {
			t.Fatalf("values[%d] has %d samples, want %d", seriesIdx, len(values[seriesIdx]), len(wantSeries))
		}
		for sampleIdx, want := range wantSeries {
			if got := values[seriesIdx][sampleIdx]; got != want {
				t.Errorf("values[%d][%d] = %v, want %v (series/category transposition bug)", seriesIdx, sampleIdx, got, want)
			}
		}
	}
}

// TestChartSingleSeriesValues_LabelValueCorrespondence verifica que cada
// label quede pareado con el valor de SU MISMA fila (row[0]/row[1]), no con
// el de otra — el bug que un test solo-de-imagen no detectaría.
func TestChartSingleSeriesValues_LabelValueCorrespondence(t *testing.T) {
	elem := newTestChartElement("pie")
	elem.Data = [][]interface{}{
		{"Search Engine", 1048},
		{"Direct", 735},
		{"Email", 580},
	}

	values, labels, err := chartSingleSeriesValues(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantLabels := []string{"Search Engine", "Direct", "Email"}
	wantValues := []float64{1048, 735, 580}

	if len(labels) != len(wantLabels) || len(values) != len(wantValues) {
		t.Fatalf("labels=%v values=%v, want labels=%v values=%v", labels, values, wantLabels, wantValues)
	}
	for i := range wantLabels {
		if labels[i] != wantLabels[i] {
			t.Errorf("labels[%d] = %q, want %q", i, labels[i], wantLabels[i])
		}
		if values[i] != wantValues[i] {
			t.Errorf("values[%d] = %v, want %v (label/value pairing bug)", i, values[i], wantValues[i])
		}
	}
}

func TestToFloat64_JSONNumber(t *testing.T) {
	v, err := toFloat64(json.Number("42.5"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 42.5 {
		t.Errorf("toFloat64(json.Number(\"42.5\")) = %v, want 42.5", v)
	}

	if _, err := toFloat64(json.Number("not-a-number")); err == nil {
		t.Error("expected an error for an invalid json.Number")
	}
}

func TestToFloat64_NonNumericError(t *testing.T) {
	if _, err := toFloat64("not a number"); err == nil {
		t.Error("expected an error for a non-numeric string")
	}
	if _, err := toFloat64(nil); err == nil {
		t.Error("expected an error for nil")
	}
	if _, err := toFloat64(true); err == nil {
		t.Error("expected an error for a bool")
	}
}

func TestSupportsNativeChartRendering(t *testing.T) {
	tests := []struct {
		name string
		elem *ast.ChartElement
		want bool
	}{
		{"bar", newTestChartElement("bar"), true},
		{"line", newTestChartElement("line"), true},
		{"pie", newTestChartElement("pie"), true},
		{"doughnut", newTestChartElement("doughnut"), true},
		{"combo", newTestChartElement("combo"), false},
		{"scatter", newTestChartElement("scatter"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportsNativeChartRendering(tt.elem); got != tt.want {
				t.Errorf("SupportsNativeChartRendering(%q) = %v, want %v", tt.elem.ChartType, got, tt.want)
			}
		})
	}

	t.Run("json_mode_always_false", func(t *testing.T) {
		elem := newTestChartElement("bar")
		elem.IsJSONMode = true
		if SupportsNativeChartRendering(elem) {
			t.Error("SupportsNativeChartRendering() = true, want false for IsJSONMode")
		}
	})

	// Regresión de code-review (PR #159): SupportsNativeChartRendering debe
	// conservar el contrato ESTRICTO pre-#148 (cualquier Options no vacío
	// descalifica) sin importar qué tan inocua sea la clave — incluso una
	// que classifyChartOptions sí aprobaría. slidelang/offline.go consume
	// este símbolo por nombre, y CI corre tanto build-test (slidelang
	// contra el core PUBLICADO) como workspace-integration (slidelang
	// contra el core del árbol vía go.work); si este gate empezara a
	// delegar en classifyChartOptions, un mismo test de slidelang pasaría
	// en un job y fallaría en el otro según qué core resuelva. La
	// clasificación hoja-por-hoja vive en
	// SupportsNativeChartRenderingWithOptions — ver el test siguiente.
	t.Run("options_present_is_always_false_even_when_every_leaf_is_ignorable", func(t *testing.T) {
		elem := newTestChartElement("bar")
		elem.Options = map[string]interface{}{"responsive": true}
		if SupportsNativeChartRendering(elem) {
			t.Error("SupportsNativeChartRendering() = true, want false: this gate does not classify leaves")
		}
	})
}

// TestSupportsNativeChartRenderingWithOptions_ApprovesIgnorableOnlyOptions
// es el contrapunto directo del subtest anterior: la clasificación
// hoja-por-hoja del issue #148 vive acá, no en SupportsNativeChartRendering.
func TestSupportsNativeChartRenderingWithOptions_ApprovesIgnorableOnlyOptions(t *testing.T) {
	elem := newTestChartElement("bar")
	elem.Options = map[string]interface{}{"responsive": true}
	if !SupportsNativeChartRenderingWithOptions(elem) {
		t.Error("SupportsNativeChartRenderingWithOptions() = false, want true for an ignorable-only options set")
	}
}

// TestNativeChartTheme_EmptyOverrideIsNilPalette es la mitad de no-regresión
// del override de motor-temas-v2.md §2.2 para el rasterizador nativo: nil
// debe producir un ColorPalette nil (idéntico al zero value de opt.Theme
// antes de que este campo existiera), no un ColorPalette con la paleta por
// defecto envuelta — go-analyze/charts distingue "sin Theme seteado" (usa
// PainterOptions.Theme) de "Theme == GetDefaultTheme()" solo por identidad
// de puntero en algunos casos internos, así que envolver innecesariamente
// no es equivalente incluso si el resultado visual sería el mismo hoy.
func TestNativeChartTheme_EmptyOverrideIsNilPalette(t *testing.T) {
	if got := nativeChartTheme(nil, 3); got != nil {
		t.Errorf("nativeChartTheme(nil, 3) = %v, want nil", got)
	}
	if got := nativeChartTheme([]string{}, 3); got != nil {
		t.Errorf("nativeChartTheme([]string{}, 3) = %v, want nil", got)
	}
}

// TestNativeChartTheme_RepeatsExactlyPastPaletteLength is the code-review
// finding on PR #224: go-analyze/charts@v0.6.0's own getSeriesColor does
// NOT wrap a short palette via pure modulo once the requested index
// passes the palette's length — it returns colors[index%colorCount]
// adjusted for saturation/lightness by index/colorCount (adjustSeriesColor
// in its source), a different-looking shade, not an exact repeat. That
// breaks the chart-cat-* contract (motor-temas-v2.md §2.2: an ORDERED set
// the engine must repeat EXACTLY via modulo, matching
// chartCategoricalPalette's colors[i%len(colors)] in html.go) — a chart
// with more series than palette entries would render differently on the
// native backend than on Chart.js. count must force every index the chart
// will actually request to land inside the (pre-expanded) palette, so
// go-analyze/charts' own adjustment branch never fires.
func TestNativeChartTheme_RepeatsExactlyPastPaletteLength(t *testing.T) {
	colors := []string{"#ff0000", "#00ff00"}
	// count=5: indices 2,3,4 all fall past len(colors)=2 and must repeat
	// the same two colors exactly, not an HSL-adjusted variant.
	palette := nativeChartTheme(colors, 5)
	if palette == nil {
		t.Fatal("nativeChartTheme with a non-empty override returned nil")
	}
	want := []charts.Color{
		charts.ParseColor(colors[0]),
		charts.ParseColor(colors[1]),
		charts.ParseColor(colors[0]),
		charts.ParseColor(colors[1]),
		charts.ParseColor(colors[0]),
	}
	for i, w := range want {
		if got := palette.GetSeriesColor(i); got != w {
			t.Errorf("GetSeriesColor(%d) = %v, want exact repeat %v (no HSL adjustment)", i, got, w)
		}
	}
}

// TestRenderChartNativePNGWithColors_CategoricalColorsOverrideChangesOutput
// es el hallazgo bloqueante de la revisión de PR2 (motor-temas-v2.md §2.2):
// antes de este cambio, RenderChartNativePNG no tenía forma de recibir el
// tema — chromium.ChartFetcher.renderFunc prefiere este camino para
// bar/line/pie/doughnut (issue #130), así que sin este parámetro
// RenderContext.ChartCategoricalColors era letra muerta para la mayoría de
// los charts reales en el camino offline/PDF, con GenerateChartConfigWithMode
// sirviendo solo a combo/scatter/JSON-mode. No hay golden de píxeles exacto
// (ver decodePNGAndCheck) así que la prueba es la más fuerte que se puede
// hacer sin acoplarse a la rasterización interna de go-analyze/charts: el
// PNG con override debe ser BYTE-DISTINTO del PNG sin override, para cada
// tipo que este rasterizador soporta. Corre contra RenderChartNativePNGWithColors,
// no RenderChartNativePNG — ese último mantiene su firma de 3 args a
// propósito (ver su doc comment) y siempre pasa nil.
func TestRenderChartNativePNGWithColors_CategoricalColorsOverrideChangesOutput(t *testing.T) {
	override := []string{"#ff0000", "#00ff00"}

	for _, chartType := range []string{"bar", "line", "pie", "doughnut"} {
		t.Run(chartType, func(t *testing.T) {
			elem := newTestChartElement(chartType)

			defaultData, ok, err := RenderChartNativePNGWithColors(elem, 640, 480, nil)
			if !ok || err != nil {
				t.Fatalf("RenderChartNativePNGWithColors(nil) ok=%v err=%v", ok, err)
			}
			overriddenData, ok, err := RenderChartNativePNGWithColors(elem, 640, 480, override)
			if !ok || err != nil {
				t.Fatalf("RenderChartNativePNGWithColors(override) ok=%v err=%v", ok, err)
			}

			if bytes.Equal(defaultData, overriddenData) {
				t.Error("override produced byte-identical PNG to the default palette — ChartCategoricalColors is not reaching the native rasterizer")
			}
		})
	}
}
