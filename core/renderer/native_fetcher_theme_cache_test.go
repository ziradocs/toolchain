// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func themeCacheChart() *ast.ChartElement {
	return &ast.ChartElement{
		ChartType: "bar",
		Title:     "Ventas",
		Data: [][]interface{}{
			{"Q1", 10.0, 20.0},
			{"Q2", 30.0, 40.0},
		},
		Series: []string{"A", "B"},
	}
}

// TestNativeChartFetcher_ThemeChangeDoesNotServeStalePNG es la reproducción
// exacta de un hallazgo de code-review sobre este PR. NativeChartFetcher es
// el atajo SIN Chromium, y su clave de cache se calculaba solo con elem +
// dimensiones: ni los colores de tema ni la paleta categórica entraban. Como
// FetchAndSave devuelve el archivo ya existente vía os.Stat sin re-
// rasterizar, un segundo build con otro tema al mismo outputDir devolvía la
// misma ruta y conservaba el PNG del primero.
//
// A diferencia de chromium.ChartFetcher, acá chartConfig se ignora por
// completo, así que la paleta NO viajaba por esa vía tampoco — este camino
// nunca estuvo cubierto, ni siquiera para categoricalColors desde #224.
func TestNativeChartFetcher_ThemeChangeDoesNotServeStalePNG(t *testing.T) {
	outputDir := t.TempDir()
	elem := themeCacheChart()
	const w, h = 400, 300

	first := NewNativeChartFetcher()
	first.SetChartThemeColors(ChartThemeColors{Surface: "#ff0000"})
	redPath, err := first.FetchAndSave(context.Background(), elem, "", outputDir, w, h)
	if err != nil {
		t.Fatalf("primer FetchAndSave: %v", err)
	}
	redBytes, err := os.ReadFile(filepath.Join(outputDir, redPath))
	if err != nil {
		t.Fatalf("leyendo el PNG rojo: %v", err)
	}

	second := NewNativeChartFetcher()
	second.SetChartThemeColors(ChartThemeColors{Surface: "#0000ff"})
	bluePath, err := second.FetchAndSave(context.Background(), elem, "", outputDir, w, h)
	if err != nil {
		t.Fatalf("segundo FetchAndSave: %v", err)
	}

	if bluePath == redPath {
		t.Fatalf("dos temas distintos produjeron la misma ruta %q: el color no entró al hash", redPath)
	}
	blueBytes, err := os.ReadFile(filepath.Join(outputDir, bluePath))
	if err != nil {
		t.Fatalf("leyendo el PNG azul: %v", err)
	}
	if bytes.Equal(redBytes, blueBytes) {
		t.Error("el segundo build sirvió los bytes del primero pese a tener otro tema")
	}
}

// TestNativeChartFetcher_CategoricalChangeDoesNotServeStalePNG cubre el mismo
// agujero para la paleta categórica, que lo arrastraba desde #224 — este
// camino ignora chartConfig, así que el argumento de "la paleta ya viaja
// dentro del config" (cierto en chromium.cacheKeyInput) no aplica acá.
func TestNativeChartFetcher_CategoricalChangeDoesNotServeStalePNG(t *testing.T) {
	outputDir := t.TempDir()
	elem := themeCacheChart()
	const w, h = 400, 300

	first := NewNativeChartFetcher()
	first.SetCategoricalColors([]string{"#ff0000", "#00ff00"})
	pathA, err := first.FetchAndSave(context.Background(), elem, "", outputDir, w, h)
	if err != nil {
		t.Fatalf("primer FetchAndSave: %v", err)
	}

	second := NewNativeChartFetcher()
	second.SetCategoricalColors([]string{"#0000ff", "#ffff00"})
	pathB, err := second.FetchAndSave(context.Background(), elem, "", outputDir, w, h)
	if err != nil {
		t.Fatalf("segundo FetchAndSave: %v", err)
	}

	if pathA == pathB {
		t.Errorf("dos paletas distintas produjeron la misma ruta %q", pathA)
	}
}

// TestNativeChartFetcher_ZeroValueKeepsHistoricHash fija que un fetcher sin
// tema (TryAllChartsNative, el gate de doclang) siga produciendo la MISMA
// clave de siempre, para no invalidar los PNG ya cacheados en disco de builds
// anteriores.
func TestNativeChartFetcher_ZeroValueKeepsHistoricHash(t *testing.T) {
	elem := themeCacheChart()
	plain := NewNativeChartFetcher().chartHash(elem, 400, 300)

	// La forma histórica: elem serializado + dimensiones, sin sufijo alguno.
	data, _ := json.Marshal(elem)
	historic := nativeContentHash(string(data) + "|400x300")
	if plain != historic {
		t.Errorf("el hash sin tema cambió: got %q, want %q", plain, historic)
	}

	themed := NewNativeChartFetcher()
	themed.SetChartThemeColors(ChartThemeColors{Grid: "#111111"})
	if themed.chartHash(elem, 400, 300) == plain {
		t.Error("un tema con solo chart-grid no cambió el hash")
	}
}

// TestNativeChartFetcher_SeedMatchesLookupWithTheme fija la consistencia que
// el hash-como-método garantiza: con los colores seteados ANTES de Seed, la
// siembra del gate le pega al lookup de FetchInline y no se re-rasteriza.
func TestNativeChartFetcher_SeedMatchesLookupWithTheme(t *testing.T) {
	elem := themeCacheChart()
	f := NewNativeChartFetcher()
	f.SetCategoricalColors([]string{"#123456"})
	f.SetChartThemeColors(ChartThemeColors{Axis: "#654321"})

	sentinel := []byte("PNG-sembrado-por-el-gate")
	f.Seed(elem, 400, 300, sentinel)

	got, err := f.FetchInline(context.Background(), elem, "", 400, 300)
	if err != nil {
		t.Fatalf("FetchInline: %v", err)
	}
	if !bytes.Equal(got, sentinel) {
		t.Error("FetchInline no encontró la siembra: Seed y el lookup usaron claves distintas")
	}
}
