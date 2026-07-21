// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import (
	"encoding/json"
	"fmt"

	"github.com/go-analyze/charts"

	"go.ziradocs.com/core/ast"
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
// Chromium. Expuesto para que los callers (fetchers, tests) decidan si
// necesitan un ChromiumRenderer en absoluto antes de intentarlo. elem.Options
// (config Chart.js arbitraria: ejes secundarios, posición de leyenda,
// estilos de plugins) no tiene equivalente en go-analyze/charts y
// RenderChartNativePNG nunca la lee — si el autor la configuró, se respeta
// cayendo a chromedp+Chart.js en vez de descartarla en silencio (hallazgo de
// code-review sobre PR #163).
func SupportsNativeChartRendering(elem *ast.ChartElement) bool {
	return !elem.IsJSONMode && len(elem.Options) == 0 && nativeChartSupportedTypes[elem.ChartType]
}

// ChartDimensions vive en chart_dimensions.go, sin build tag: html.go la
// llama y se compila también para wasm (playground).

// RenderChartNativePNG rasteriza elem a PNG vía go-analyze/charts. Devuelve
// ok=false (sin error) cuando elem.ChartType no tiene mapeo nativo, está en
// IsJSONMode, o trae elem.Options — el caller debe caer a
// ChromiumRenderer.RenderChartToPNG en esos casos. Un error (con ok=true)
// indica que SÍ se intentó el camino nativo pero falló (p. ej. datos vacíos/
// no numéricos/filas de largo irregular).
func RenderChartNativePNG(elem *ast.ChartElement, width, height int) (data []byte, ok bool, err error) {
	if !SupportsNativeChartRendering(elem) {
		return nil, false, nil
	}

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
		names := resolveSeriesNames(elem.Series, len(values))
		if elem.ChartType == "bar" {
			opt := charts.NewBarChartOptionWithData(values)
			opt.Title.Text = elem.Title
			opt.CategoryAxis.Labels = categoryLabels
			opt.Legend.SeriesNames = names
			err = p.BarChart(opt)
		} else {
			opt := charts.NewLineChartOptionWithData(values)
			opt.Title.Text = elem.Title
			opt.XAxis.Labels = categoryLabels
			opt.Legend.SeriesNames = names
			err = p.LineChart(opt)
		}
	case "pie":
		pieValues, pieLabels, pieErr := chartSingleSeriesValues(elem)
		if pieErr != nil {
			return nil, true, pieErr
		}
		opt := charts.NewPieChartOptionWithData(pieValues)
		opt.Title.Text = elem.Title
		opt.Legend.SeriesNames = pieLabels
		err = p.PieChart(opt)
	case "doughnut":
		doughnutValues, doughnutLabels, dErr := chartSingleSeriesValues(elem)
		if dErr != nil {
			return nil, true, dErr
		}
		opt := charts.NewDoughnutChartOptionWithData(doughnutValues)
		opt.Title.Text = elem.Title
		opt.Legend.SeriesNames = doughnutLabels
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
