// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import (
	"bytes"
	"math"
	"image"
	"image/png"
	"testing"

	"github.com/go-analyze/charts"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestApplyYAxisOverrides_ZeroBaseline: con beginAtZero (el default de
// Chart.js para bar) el cero entra al rango del eje: mínimo 0 con datos no
// negativos, máximo 0 con datos no positivos, nada si ya lo cruzan. Un
// beginAtZero: false explícito y el default de line no tocan el eje, y los
// puntos ausentes no cuentan.
func TestApplyYAxisOverrides_ZeroBaseline(t *testing.T) {
	null := charts.GetNullValue()
	cases := []struct {
		name             string
		begin            *bool
		defaultBegin     bool
		values           [][]float64
		wantMin, wantMax *float64
	}{
		{"bar positivos", nil, true, [][]float64{{80, 85, 90}}, charts.Ptr(0.0), nil},
		{"bar con cero", nil, true, [][]float64{{0, 5}}, charts.Ptr(0.0), nil},
		{"bar negativos", nil, true, [][]float64{{-80, -85}, {-90, -1}}, charts.Ptr(-90.0), charts.Ptr(0.0)},
		{"bar cruzan el cero", nil, true, [][]float64{{-5, 10}}, nil, nil},
		{"bar beginAtZero false", charts.Ptr(false), true, [][]float64{{80, 90}}, nil, nil},
		{"bar con nulos", nil, true, [][]float64{{null, 80, 90}}, charts.Ptr(0.0), nil},
		{"bar sin valores", nil, true, [][]float64{{null}}, nil, nil},
		{"line default", nil, false, [][]float64{{80, 90}}, nil, nil},
		{"line beginAtZero true", charts.Ptr(true), false, [][]float64{{80, 90}}, charts.Ptr(0.0), nil},
		{"line beginAtZero true negativos", charts.Ptr(true), false, [][]float64{{-80, -90}}, charts.Ptr(-90.0), charts.Ptr(0.0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var axis charts.ValueAxisOption
			applyYAxisOverrides(&axis, nativeChartOverrides{yBeginAtZero: c.begin}, c.values, c.defaultBegin)
			if !samePtrValue(axis.Min, c.wantMin) || !samePtrValue(axis.Max, c.wantMax) {
				t.Errorf("Min=%v Max=%v, want Min=%v Max=%v", deref(axis.Min), deref(axis.Max), deref(c.wantMin), deref(c.wantMax))
			}
		})
	}
}

// TestNiceNegativeAxis: con todos los valores negativos el eje va de 0 a un
// mínimo redondo con un intervalo 1, 2 o 5 por una potencia de diez, como
// Chart.js. Antes salían etiquetas como -11.11 o -1.44.
func TestNiceNegativeAxis(t *testing.T) {
	cases := []struct {
		lo      float64
		wantMin float64
		labels  int
	}{
		{-90, -90, 10},
		{-12.5, -14, 8},
		{-0.37, -0.4, 9},
		{-1234, -1400, 8},
	}
	for _, c := range cases {
		gotMin, labels := niceNegativeAxis(c.lo)
		if math.Abs(gotMin-c.wantMin) > 1e-9 || labels != c.labels {
			t.Errorf("niceNegativeAxis(%v) = %v, %d; want %v, %d", c.lo, gotMin, labels, c.wantMin, c.labels)
		}
		step := -gotMin / float64(labels-1)
		if mant := step / math.Pow(10, math.Floor(math.Log10(step))); math.Abs(mant-1) > 1e-9 && math.Abs(mant-2) > 1e-9 && math.Abs(mant-5) > 1e-9 {
			t.Errorf("niceNegativeAxis(%v): step %v is not 1, 2 or 5 times a power of ten", c.lo, step)
		}
	}
	var axis charts.ValueAxisOption
	applyYAxisOverrides(&axis, nativeChartOverrides{}, [][]float64{{-3, -7, -12.5}}, true)
	if axis.LabelCount != 8 {
		t.Errorf("LabelCount = %d, want 8 (0 to -14 by 2)", axis.LabelCount)
	}
}

// TestExtractNativeChartOverrides_BeginAtZeroFalse: un beginAtZero: false
// explícito llega como tal (antes se confundía con "no escrito").
func TestExtractNativeChartOverrides_BeginAtZeroFalse(t *testing.T) {
	ov := extractNativeChartOverrides(map[string]interface{}{
		"scales": map[string]interface{}{"y": map[string]interface{}{"beginAtZero": false}},
	})
	if ov.yBeginAtZero == nil || *ov.yBeginAtZero {
		t.Fatalf("yBeginAtZero = %v, want explicit false", ov.yBeginAtZero)
	}
	if ov := extractNativeChartOverrides(map[string]interface{}{}); ov.yBeginAtZero != nil {
		t.Fatalf("yBeginAtZero = %v, want nil when absent", *ov.yBeginAtZero)
	}
}

// TestNativeChart_ExplicitMinLeavesNativePath: un mínimo explícito del autor
// (scales.y.min) no llega al camino nativo, así que el default de cero nunca
// lo pisa: el chart va a Chart.js, que sí lo respeta.
func TestNativeChart_ExplicitMinLeavesNativePath(t *testing.T) {
	elem := newTestChartElement("bar")
	elem.Options = map[string]interface{}{
		"scales": map[string]interface{}{"y": map[string]interface{}{"min": 50}},
	}
	if SupportsNativeChartRenderingWithOptions(elem) {
		t.Fatal("scales.y.min must disqualify the native renderer")
	}
}

// TestRenderChartNativePNG_BarsStartAtZero mide las barras en el PNG: con
// valores 80, 85 y 90 y el eje desde cero, la barra más baja mide cerca de
// 80/90 de la más alta. Con el eje recortado a un mínimo cercano a los datos
// (el defecto) medía menos de la mitad.
func TestRenderChartNativePNG_BarsStartAtZero(t *testing.T) {
	elem := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	elem.Data = [][]interface{}{{"A", 80}, {"B", 85}, {"C", 90}}
	data, ok, err := RenderChartNativePNGWithColors(elem, 600, 400, []string{"#ff0000"})
	if !ok || err != nil {
		t.Fatalf("native render: ok=%v err=%v", ok, err)
	}
	heights := barHeights(t, data)
	if len(heights) != 3 {
		t.Fatalf("found %d bars, want 3 (heights %v)", len(heights), heights)
	}
	ratio := float64(heights[0]) / float64(heights[2])
	if ratio < 0.85 || ratio > 0.93 {
		t.Fatalf("shortest/tallest bar = %.2f (heights %v), want ~%.2f (axis from zero)", ratio, heights, 80.0/90.0)
	}
}

// barHeights devuelve la altura en píxeles de cada barra roja, de izquierda
// a derecha: la mayor cantidad de píxeles rojos de una columna dentro de
// cada tramo contiguo de columnas con rojo.
func barHeights(t *testing.T, data []byte) []int {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	// La altura de una columna es su tramo vertical continuo de rojo más
	// largo: el cuadro rojo de la leyenda queda en otro tramo y no se suma.
	column := func(x int) int {
		best, run := 0, 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if isRed(img, x, y) {
				run++
				best = max(best, run)
			} else {
				run = 0
			}
		}
		return best
	}
	var heights []int
	inBar, best := false, 0
	for x := b.Min.X; x < b.Max.X; x++ {
		// Una columna con pocos rojos es antialiasing o la leyenda, no barra.
		if h := column(x); h > 20 {
			inBar, best = true, max(best, h)
			continue
		}
		if inBar {
			heights = append(heights, best)
			inBar, best = false, 0
		}
	}
	if inBar {
		heights = append(heights, best)
	}
	return heights
}

func isRed(img image.Image, x, y int) bool {
	r, g, bl, _ := img.At(x, y).RGBA()
	return r>>8 > 200 && g>>8 < 60 && bl>>8 < 60
}

func samePtrValue(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
