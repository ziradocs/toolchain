// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
)

const cacheKeyTestConfig = `{"type":"bar","data":{"datasets":[{"data":[1,2]}]}}`

func cacheKeyTestElem() *ast.ChartElement {
	return &ast.ChartElement{ChartType: "bar", Title: "T"}
}

// TestCacheKeyInput_ThemeColorsChangeTheKey es la razón por la que este PR
// tuvo que tocar el cache key y no solo el rasterizador. FetchAndSave sirve
// el archivo ya existente en disco sin invocar renderFunc (BaseFetcher hace
// un os.Stat y retorna temprano), así que si dos temas distintos producen el
// mismo hash, el segundo build sirve el PNG del primero — con los colores
// equivocados y sin ninguna señal de error.
//
// A diferencia de la paleta categórica, estos tokens NO aparecen en
// chartConfig por ningún lado (no son opciones de Chart.js que
// GenerateChartConfigWithMode serialice), así que el hash no los ve por
// ninguna otra vía.
func TestCacheKeyInput_ThemeColorsChangeTheKey(t *testing.T) {
	base := cacheKeyInput(cacheKeyTestElem(), cacheKeyTestConfig, 800, 600, renderer.ChartThemeColors{})

	for name, tc := range map[string]renderer.ChartThemeColors{
		"surface": {Surface: "#111111"},
		"grid":    {Grid: "#111111"},
		"axis":    {Axis: "#111111"},
		"label":   {Label: "#111111"},
		"todos":   {Surface: "#111111", Grid: "#222222", Axis: "#333333", Label: "#444444"},
	} {
		got := cacheKeyInput(cacheKeyTestElem(), cacheKeyTestConfig, 800, 600, tc)
		if got == base {
			t.Errorf("%s: el cache key no cambió con el tema puesto (%q) — dos temas servirían el mismo PNG cacheado", name, got)
		}
	}
}

// TestCacheKeyInput_DistinctThemesDistinctKeys cubre el caso que de verdad
// muerde: no "con tema vs sin tema", sino dos temas DISTINTOS entre sí, que
// es lo que pasa cuando alguien rebuildea el mismo deck con otro --theme
// hacia el mismo outputDir.
func TestCacheKeyInput_DistinctThemesDistinctKeys(t *testing.T) {
	a := cacheKeyInput(cacheKeyTestElem(), cacheKeyTestConfig, 800, 600, renderer.ChartThemeColors{Grid: "#111111"})
	b := cacheKeyInput(cacheKeyTestElem(), cacheKeyTestConfig, 800, 600, renderer.ChartThemeColors{Grid: "#222222"})
	if a == b {
		t.Error("dos temas distintos produjeron el mismo cache key")
	}
}

// TestCacheKeyInput_FieldsAreNotInterchangeable evita el bug clásico de
// concatenar campos sin separador: si el key fuera Surface+Grid pegados,
// {Surface:"#11", Grid:"#2222"} y {Surface:"#1122", Grid:"#22"} colisionarían.
func TestCacheKeyInput_FieldsAreNotInterchangeable(t *testing.T) {
	a := cacheKeyInput(cacheKeyTestElem(), cacheKeyTestConfig, 800, 600,
		renderer.ChartThemeColors{Surface: "#11", Grid: "#2222"})
	b := cacheKeyInput(cacheKeyTestElem(), cacheKeyTestConfig, 800, 600,
		renderer.ChartThemeColors{Surface: "#1122", Grid: "#22"})
	if a == b {
		t.Error("dos combinaciones distintas de campos colisionaron — falta separador entre ellos")
	}
}

// TestCacheKeyInput_ZeroValueKeepsTheHistoricKey fija el otro lado del
// contrato: un caller sin tema (todos los de hoy, y doclang siempre) tiene
// que producir EXACTAMENTE el key de antes, para no invalidar los assets ya
// cacheados en disco de builds anteriores.
func TestCacheKeyInput_ZeroValueKeepsTheHistoricKey(t *testing.T) {
	got := cacheKeyInput(cacheKeyTestElem(), cacheKeyTestConfig, 800, 600, renderer.ChartThemeColors{})
	want := cacheKeyTestConfig + "|T|800x600"
	if got != want {
		t.Errorf("cache key sin tema = %q, want %q — el zero value cambió el hash histórico", got, want)
	}
}
