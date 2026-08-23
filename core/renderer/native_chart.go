// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import (
	"encoding/json"
	"fmt"

	"github.com/go-analyze/charts"

	"go.ziradocs.com/core/v2/ast"
)

// native_chart.go implementa la rasterización nativa de charts (issue #130):
// bar/line/pie/doughnut se dibujan directamente vía go-analyze/charts, sin
// pasar por Chromium+Chart.js. combo/scatter/cualquier ChartType no
// reconocido, y el modo IsJSONMode (config Chart.js arbitraria escrita a
// mano, no mapeable a un tipo fijo) siguen cayendo al pipeline chromedp
// existente (RenderChartToPNG) — degradación esperada, no un error.

// nativeChartSupportedTypes son los ChartType con mapeo directo a
// go-analyze/charts. No es exhaustivo respecto al vocabulario de Chart.js a
// propósito: un mapeo aproximado (p. ej. forzar "combo" a un tipo simple)
// dibujaría un chart visualmente distinto al que el autor pidió, así que el
// fallback determinístico a chromedp es la salida segura para lo no cubierto.
var nativeChartSupportedTypes = map[string]bool{
	"bar":      true,
	"line":     true,
	"pie":      true,
	"doughnut": true,
}

// SupportsNativeChartRendering indica si elem puede rasterizarse sin
// Chromium bajo el contrato ESTRICTO pre-#148: cualquier elem.Options no
// vacío descalifica, sin mirar sus hojas. Expuesto para que los callers
// (fetchers, tests) decidan si necesitan un ChromiumRenderer en absoluto
// antes de intentarlo.
//
// Este comportamiento se conserva sin cambios a propósito — no delega en
// classifyChartOptions — porque slidelang/internal/generator/offline.go ya
// lo consume por nombre, y CI corre workspace-integration (slidelang contra
// el core DEL ÁRBOL vía go.work) además de build-test (slidelang contra el
// core PUBLICADO): cambiar la conducta de este símbolo existente rompería
// uno de los dos gates sin importar el orden en que se mergee (ver el
// comentario de workspace-integration en .github/workflows/ci.yml). Usar
// SupportsNativeChartRenderingWithOptions para la clasificación hoja-por-
// hoja del issue #148 — slidelang migra a ese símbolo en el PR consumidor,
// después de bump-core.
func SupportsNativeChartRendering(elem *ast.ChartElement) bool {
	if elem.IsJSONMode || !nativeChartSupportedTypes[elem.ChartType] {
		return false
	}
	return len(elem.Options) == 0
}

// SupportsNativeChartRenderingWithOptions es SupportsNativeChartRendering
// más la clasificación hoja-por-hoja de elem.Options (issue #148): una clave
// conocida e ignorable (p. ej. "responsive") o traducible (p. ej.
// plugins.title.text) no descalifica; cualquier hoja fuera de ese set sí, y
// cae a chromedp+Chart.js en vez de descartarse en silencio (hallazgo de
// code-review sobre PR #163, que sigue vigente para lo no reconocido). Ver
// native_chart_options.go para classifyChartOptions y el set reconocido.
func SupportsNativeChartRenderingWithOptions(elem *ast.ChartElement) bool {
	if elem.IsJSONMode || !nativeChartSupportedTypes[elem.ChartType] {
		return false
	}
	return classifyChartOptions(elem.Options)
}

// ChartDimensions vive en chart_dimensions.go, sin build tag: html.go la
// llama y se compila también para wasm (playground).

// RenderChartNativePNG rasteriza elem a PNG vía go-analyze/charts con la
// paleta por defecto. Firma sin cambios a propósito (issue "una sola danza"
// de motor-temas-v2.md): slidelang/internal/generator/offline.go llama a
// este símbolo por nombre, y CI corre workspace-integration (slidelang
// contra el core DEL ÁRBOL vía go.work) — agregarle un parámetro acá
// rompería ese build entre el merge de este PR y el PR de slidelang que
// consuma el tema. Ver RenderChartNativePNGWithColors para el override.
func RenderChartNativePNG(elem *ast.ChartElement, width, height int) (data []byte, ok bool, err error) {
	return RenderChartNativePNGWithColors(elem, width, height, nil)
}

// RenderChartNativePNGWithColors es RenderChartNativePNG con un override de
// paleta categórica. Devuelve ok=false (sin error) cuando elem.ChartType no
// tiene mapeo nativo, está en IsJSONMode, o elem.Options trae alguna hoja no
// reconocida por classifyChartOptions — el caller debe caer a
// ChromiumRenderer.RenderChartToPNG en esos casos. Un error (con ok=true)
// indica que SÍ se intentó el camino nativo pero falló (p. ej. datos
// vacíos/no numéricos/filas de largo irregular).
//
// categoricalColors es el mismo override que RenderContext.ChartCategoricalColors
// (motor-temas-v2.md §2.2): nil/vacío reproduce la paleta por defecto de
// go-analyze/charts, sin cambio de comportamiento. Existe porque este
// rasterizador NUNCA pasa por GenerateChartConfigWithMode — trabaja
// directamente sobre elem, así que el chart-cat-* de un tema no le llega
// por ningún otro camino. Sin esto, cualquier caller que prefiera este
// rasterizador (chromium.ChartFetcher.renderFunc lo intenta primero para
// bar/line/pie/doughnut, issue #130 — que es la mayoría de los charts
// reales) ignoraría el tema en silencio incluso con
// RenderContext.ChartCategoricalColors seteado.
func RenderChartNativePNGWithColors(elem *ast.ChartElement, width, height int, categoricalColors []string) (data []byte, ok bool, err error) {
	if !SupportsNativeChartRenderingWithOptions(elem) {
		return nil, false, nil
	}
	ov := extractNativeChartOverrides(elem.Options)

	p := charts.NewPainter(charts.PainterOptions{
		OutputFormat: charts.ChartOutputPNG,
		Width:        width,
		Height:       height,
	})

	switch elem.ChartType {
	case "bar", "line":
		// chartSeriesValues solo se llama para bar/line: pie/doughnut usan
		// chartSingleSeriesValues, que solo lee row[1] de cada fila — llamar
		// a chartSeriesValues incondicionalmente (como antes) recorría TODAS
		// las columnas de cada fila y fallaba con datos de pie/doughnut que
		// traen columnas extra que esa rama nunca usa (hallazgo de
		// code-review sobre PR #163).
		values, categoryLabels, seriesErr := chartSeriesValues(elem)
		if seriesErr != nil {
			return nil, true, seriesErr
		}
		theme := nativeChartTheme(categoricalColors, len(values))
		names := resolveSeriesNames(elem.Series, len(values))
		if elem.ChartType == "bar" {
			opt := charts.NewBarChartOptionWithData(values)
			opt.Theme = theme
			applyTitleOverrides(&opt.Title, elem.Title, ov)
			opt.CategoryAxis.Labels = categoryLabels
			opt.Legend.SeriesNames = names
			applyLegendOverrides(&opt.Legend, ov)
			if len(opt.ValueAxis) > 0 {
				applyYAxisOverrides(&opt.ValueAxis[0], ov)
			}
			err = p.BarChart(opt)
		} else {
			opt := charts.NewLineChartOptionWithData(values)
			opt.Theme = theme
			applyTitleOverrides(&opt.Title, elem.Title, ov)
			opt.XAxis.Labels = categoryLabels
			opt.Legend.SeriesNames = names
			applyLegendOverrides(&opt.Legend, ov)
			if len(opt.YAxis) > 0 {
				applyYAxisOverrides(&opt.YAxis[0], ov)
			}
			err = p.LineChart(opt)
		}
	case "pie":
		pieValues, pieLabels, pieErr := chartSingleSeriesValues(elem)
		if pieErr != nil {
			return nil, true, pieErr
		}
		opt := charts.NewPieChartOptionWithData(pieValues)
		opt.Theme = nativeChartTheme(categoricalColors, len(pieValues))
		applyTitleOverrides(&opt.Title, elem.Title, ov)
		opt.Legend.SeriesNames = pieLabels
		applyLegendOverrides(&opt.Legend, ov)
		err = p.PieChart(opt)
	case "doughnut":
		doughnutValues, doughnutLabels, dErr := chartSingleSeriesValues(elem)
		if dErr != nil {
			return nil, true, dErr
		}
		opt := charts.NewDoughnutChartOptionWithData(doughnutValues)
		opt.Theme = nativeChartTheme(categoricalColors, len(doughnutValues))
		applyTitleOverrides(&opt.Title, elem.Title, ov)
		opt.Legend.SeriesNames = doughnutLabels
		applyLegendOverrides(&opt.Legend, ov)
		err = p.DoughnutChart(opt)
	default:
		// No debería llegar acá: nativeChartSupportedTypes ya lo filtró.
		return nil, false, nil
	}
	if err != nil {
		return nil, true, fmt.Errorf("native chart render failed: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, true, fmt.Errorf("native chart encode failed: %w", err)
	}
	return buf, true, nil
}

// nativeChartTheme construye el ColorPalette de go-analyze/charts que
// aplica categoricalColors a la paleta por defecto — nil/vacío devuelve nil,
// que asignado a opt.Theme es idéntico a no tocarlo (el zero value de la
// interfaz), así que un caller sin tema reproduce el render de antes byte
// por byte. charts.ParseColor acepta directamente los strings hex que trae
// un theme.json (p. ej. "#3498db").
//
// count es cuántas series/segmentos necesita colorear ESTE chart.
// go-analyze/charts@v0.6.0's getSeriesColor no repite por módulo puro una
// vez que el índice pedido alcanza el largo de la paleta: reusa el color
// de index%colorCount pero le ajusta saturación/luminosidad según
// index/colorCount (ver adjustSeriesColor en su fuente), así que devuelve
// una variante distinta, no el mismo color — al revés del contrato
// chart-cat-* de motor-temas-v2.md §2.2 (un set ORDENADO que el motor debe
// repetir exacto por módulo, igual que chartCategoricalPalette en html.go
// ya hace para el camino Chart.js), y hacía que el mismo chart se viera
// distinto entre el backend nativo y Chart.js para cualquier serie más
// allá del largo de la paleta (hallazgo de code-review sobre PR #224). Se
// expande categoricalColors a count entradas por módulo ANTES de
// entregárselo a WithSeriesColors, para que ningún índice que
// go-analyze/charts vaya a pedir quede nunca por debajo de colorCount y
// esa rama de ajuste no se dispare nunca.
func nativeChartTheme(categoricalColors []string, count int) charts.ColorPalette {
	if len(categoricalColors) == 0 {
		return nil
	}
	if count < len(categoricalColors) {
		count = len(categoricalColors)
	}
	colors := make([]charts.Color, count)
	for i := range colors {
		colors[i] = charts.ParseColor(categoricalColors[i%len(categoricalColors)])
	}
	return charts.GetDefaultTheme().WithSeriesColors(colors)
}

// chartSeriesValues transpone elem.Data (una fila por categoría: [label, v1,
// v2, ...], la misma convención que usa GenerateChartConfigWithMode) a la
// orientación que go-analyze/charts espera (una fila por serie:
// values[serie][categoría]). Devuelve error si no hay filas, la primera fila
// no tiene columnas de datos, alguna fila tiene un largo distinto a la
// primera, o algún valor no es numérico — en vez de silenciarlo a 0 (que
// dibujaría un chart engañoso) o zero-rellenar filas cortas: eso hacía que
// datos irregulares se vieran DISTINTO acá que en el pipeline chromedp/
// Chart.js existente (GenerateChartConfigWithMode compacta en vez de
// zero-rellenar), un mismo chart renderizando dos resultados distintos según
// qué backend le tocara (hallazgo de code-review sobre PR #163). Caer a
// chromedp para datos irregulares preserva el comportamiento ya establecido
// en vez de introducir un tercero.
func chartSeriesValues(elem *ast.ChartElement) (values [][]float64, categoryLabels []string, err error) {
	if len(elem.Data) == 0 || len(elem.Data[0]) < 2 {
		return nil, nil, fmt.Errorf("chart has no series data")
	}

	numSeries := len(elem.Data[0]) - 1 // -1 por la columna de label
	values = make([][]float64, numSeries)
	for i := range values {
		values[i] = make([]float64, len(elem.Data))
	}
	categoryLabels = make([]string, len(elem.Data))

	for rowIdx, row := range elem.Data {
		if len(row) != len(elem.Data[0]) {
			return nil, nil, fmt.Errorf("chart data row %d has %d columns, want %d (irregular row length)", rowIdx, len(row), len(elem.Data[0]))
		}
		categoryLabels[rowIdx] = fmt.Sprintf("%v", row[0])

		for seriesIdx := 0; seriesIdx < numSeries; seriesIdx++ {
			v, numErr := toFloat64(row[seriesIdx+1])
			if numErr != nil {
				return nil, nil, fmt.Errorf("chart data row %d, series %d: %w", rowIdx, seriesIdx, numErr)
			}
			values[seriesIdx][rowIdx] = v
		}
	}

	return values, categoryLabels, nil
}

// chartSingleSeriesValues extrae el valor+label de cada fila para
// pie/doughnut (un solo dataset con múltiples valores, misma convención que
// GenerateChartConfigWithMode's rama pie/doughnut: row[0]=label,
// row[1]=valor).
func chartSingleSeriesValues(elem *ast.ChartElement) (values []float64, labels []string, err error) {
	if len(elem.Data) == 0 {
		return nil, nil, fmt.Errorf("chart has no data")
	}

	values = make([]float64, 0, len(elem.Data))
	labels = make([]string, 0, len(elem.Data))
	for rowIdx, row := range elem.Data {
		if len(row) < 2 {
			return nil, nil, fmt.Errorf("chart data row %d missing value column", rowIdx)
		}
		v, numErr := toFloat64(row[1])
		if numErr != nil {
			return nil, nil, fmt.Errorf("chart data row %d: %w", rowIdx, numErr)
		}
		values = append(values, v)
		labels = append(labels, fmt.Sprintf("%v", row[0]))
	}

	return values, labels, nil
}

// toFloat64 coerciona un valor de ast.ChartElement.Data (poblado desde YAML/
// JSON, así que llega como float64/int/json.Number según el parser) a
// float64. Cualquier otro tipo (string no numérico, nil, bool) es un error —
// mejor caer a chromedp que dibujar un chart con ceros silenciosos.
func toFloat64(v interface{}) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, fmt.Errorf("invalid json.Number %q: %w", n, err)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("non-numeric value %v (%T)", v, v)
	}
}
