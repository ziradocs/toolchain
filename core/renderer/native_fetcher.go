// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.ziradocs.com/core/v2/ast"
)

// native_fetcher.go implementa ChartFetcher (fetchers.go) SIN Chromium
// (issue #164) — para un caller que ya PROBÓ que cada chart del documento
// rasteriza nativo con éxito (ver tryBuildNativeContext en
// slidelang/internal/generator/offline.go), evitando instanciar
// chromium.ChromiumRenderer para nada.
//
// A diferencia de chromium.ChartFetcher (que prefiere el camino nativo
// internamente pero cae a Chromium+Chart.js cuando el render nativo falla),
// este fetcher NO tiene fallback: el gate ya probó el render nativo antes
// de elegirlo, y de hecho le PASA los bytes ya rasterizados vía Seed, así
// que FetchInline/FetchAndSave normalmente ni siquiera vuelven a
// rasterizar — una sola pasada por chart en todo el build, no dos.
//
// Solo charts, deliberadamente: los mapas NO usan un fetcher nativo sin
// fallback. El render nativo de un mapa puede fallar en runtime (tiles
// inalcanzables, timeout, panic de go-staticmaps — ver
// renderer/native_map.go) por causas que no se pueden probar barato de
// antemano como sí se puede con un chart (puro-Go, en memoria,
// determinístico); sin Chromium eso sería un <div class="map-error">
// permanente en vez del fallback que chromium.MapFetcher ofrece. Por eso el
// gate rutea cualquier deck con un mapa al camino Chromium, donde
// chromium.MapFetcher igual intenta go-staticmaps primero (el mapa se
// rasteriza nativo en el happy path) pero con Chromium como red de
// seguridad. El costo es solo el arranque de Chromium para decks con mapas,
// no la calidad del render.

var _ ChartFetcher = (*NativeChartFetcher)(nil)

// nativeContentHash es el análogo de chromium.GenerateContentHash — no se
// reusa esa función directamente porque renderer/chromium importa este
// paquete (renderer), y renderer no puede importar de vuelta a
// renderer/chromium sin un ciclo.
func nativeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash[:8])
}

// chartHash es la clave de cache/seed de un chart: los mismos elem +
// dimensiones + colores de tema dan la misma clave, así que un Seed del gate
// coincide con el lookup de FetchInline/FetchAndSave. Incluye width/height
// porque afectan los bytes del PNG (mismo motivo que
// chromium/chart_fetcher.go's cacheKeyInput).
//
// Los colores TIENEN que estar acá, y este camino es más frágil que el de
// Chromium, no menos (hallazgo de code-review sobre este mismo PR): allá la
// paleta categórica viaja serializada dentro de chartConfig, que sí entra al
// hash, así que se cubría sola. Acá chartConfig se IGNORA por completo (ver
// FetchAndSave), de modo que ningún color llegaba nunca a la clave. Sumado a
// que FetchAndSave sirve el archivo ya existente vía os.Stat sin re-
// rasterizar, dos builds al mismo outputDir con temas distintos devolvían la
// misma ruta y conservaban el PNG del primero. Aplica igual a
// categoricalColors, que arrastraba el problema desde #224.
//
// Es un método, no una función suelta, justamente para que Seed, FetchInline
// y FetchAndSave no puedan desincronizarse: los tres derivan la clave de los
// MISMOS campos del fetcher.
func (f *NativeChartFetcher) chartHash(elem *ast.ChartElement, width, height int) string {
	categoricalColors, themeColors := f.colors()
	data, _ := json.Marshal(elem)
	key := fmt.Sprintf("%s|%dx%d", data, width, height)
	if len(categoricalColors) > 0 || !themeColors.IsZero() {
		// Solo se agrega cuando hay tema, para que un fetcher sin colores
		// (TryAllChartsNative, el gate de doclang) produzca el MISMO hash de
		// siempre y no invalide los PNG ya cacheados en disco.
		//
		// Se serializa como JSON en vez de concatenar con un separador
		// porque un color CSS puede traer comas —"rgb(1,2,3)"— y cualquier
		// join ingenuo volvería ambiguas dos paletas distintas.
		colors, _ := json.Marshal(struct {
			Categorical []string         `json:"c,omitempty"`
			Theme       ChartThemeColors `json:"t"`
		}{categoricalColors, themeColors})
		key += "|" + string(colors)
	}
	return nativeContentHash(key)
}

// NativeChartFetcher implementa ChartFetcher rasterizando vía
// RenderChartNativePNG — nunca instancia Chromium. Solo válido para charts
// que el caller ya probó nativo-renderizables (ver el doc del paquete y
// tryBuildNativeContext).
type NativeChartFetcher struct {
	mu        sync.Mutex
	pathCache map[string]string // hash -> ruta relativa del PNG ya guardado (dedup de FetchAndSave)
	seedCache map[string][]byte // hash -> PNG ya rasterizado por el gate (evita el 2º render, issue #164)
	// categoricalColors/themeColors son los overrides de tema de
	// motor-temas-v2.md §2.2. Entran a chartHash (ver por qué es
	// obligatorio ahí) y al render de fallback. Zero value = sin tema, que
	// es el caso de TryAllChartsNative.
	categoricalColors []string
	themeColors       ChartThemeColors
}

// NewNativeChartFetcher crea un fetcher de charts sin Chromium.
func NewNativeChartFetcher() *NativeChartFetcher {
	return &NativeChartFetcher{
		pathCache: make(map[string]string),
		seedCache: make(map[string][]byte),
	}
}

// SetCategoricalColors y SetChartThemeColors setean los overrides de tema
// (motor-temas-v2.md §2.2) para el render de fallback y, sobre todo, para la
// clave de cache. Setters en vez de parámetros de NewNativeChartFetcher
// porque esa firma la llama slidelang por nombre y está congelada mientras
// CI corra workspace-integration — mismo patrón que
// chromium.ChartFetcher.SetCategoricalColors.
//
// ORDEN IMPORTANTE: hay que llamarlos ANTES de Seed. La clave de siembra se
// deriva de estos campos, así que sembrar primero y setear después dejaría
// los bytes sembrados bajo una clave que FetchInline/FetchAndSave ya no
// buscarían — no daría un render incorrecto (el fallback rasteriza con los
// colores correctos), pero desperdiciaría la siembra y rasterizaría dos
// veces.
func (f *NativeChartFetcher) SetCategoricalColors(categoricalColors []string) {
	f.mu.Lock()
	f.categoricalColors = categoricalColors
	f.mu.Unlock()
}

// colors lee los overrides bajo el mutex y los devuelve por valor, para que
// chartHash y render no toquen los campos sin sincronizar — el fetcher se
// comparte entre goroutines (el pathCache/seedCache ya lo asumen) y el
// -race de CI marcaría la lectura suelta. Devuelve ANTES de que el caller
// vuelva a tomar el lock: sync.Mutex no es reentrante.
func (f *NativeChartFetcher) colors() ([]string, ChartThemeColors) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.categoricalColors, f.themeColors
}

// SetChartThemeColors — ver SetCategoricalColors, incluido el orden respecto
// de Seed.
func (f *NativeChartFetcher) SetChartThemeColors(themeColors ChartThemeColors) {
	f.mu.Lock()
	f.themeColors = themeColors
	f.mu.Unlock()
}

// Seed registra los bytes PNG que el gate ya rasterizó para elem (issue
// #164): así el render de salida (FetchInline/FetchAndSave) los reusa en vez
// de volver a llamar a RenderChartNativePNG — una sola rasterización por
// chart en todo el build. La clave es exactamente la misma (chartHash) que
// usan FetchInline/FetchAndSave, así que un elem con las mismas dimensiones
// y los mismos colores da un hit.
func (f *NativeChartFetcher) Seed(elem *ast.ChartElement, width, height int, pngData []byte) {
	hash := f.chartHash(elem, width, height)
	f.mu.Lock()
	f.seedCache[hash] = pngData
	f.mu.Unlock()
}

// render devuelve los bytes del chart: los sembrados por el gate si existen
// (sin re-rasterizar), o una rasterización nativa fresca si no. Un chart no
// nativo-capaz es un error de programación del caller (el gate debió
// excluirlo), no un caso a degradar.
func (f *NativeChartFetcher) render(elem *ast.ChartElement, hash string, width, height int) ([]byte, error) {
	f.mu.Lock()
	if data, ok := f.seedCache[hash]; ok {
		f.mu.Unlock()
		return data, nil
	}
	f.mu.Unlock()

	// Fallback: casi nunca se ejecuta en un build temátizado — el gate que
	// construye NativeChartFetcher (tryBuildNativeContext en slidelang,
	// TryAllChartsNative en core) ya rasterizó y Seed()ó cada chart antes de
	// que el fetcher se use para servir nada; esto solo corre si esa siembra
	// falla o si las claves no coinciden.
	//
	// Aun así rasteriza CON los colores del fetcher, no con la paleta por
	// defecto: antes este camino perdía el tema en silencio, que es
	// exactamente el modo de falla que motor-temas-v2.md existe para
	// eliminar. Con el zero value es idéntico a RenderChartNativePNG.
	categoricalColors, themeColors := f.colors()
	data, ok, err := RenderChartNativePNGWithTheme(elem, width, height, categoricalColors, themeColors)
	if !ok {
		return nil, fmt.Errorf("chart type %q has no native renderer (caller must gate on SupportsNativeChartRendering before using NativeChartFetcher)", elem.ChartType)
	}
	return data, err
}

// FetchAndSave rasteriza elem (o reusa el seed) y lo guarda como PNG,
// devolviendo la ruta relativa. chartConfig (el JSON Chart.js ya
// serializado) se ignora deliberadamente: el camino nativo trabaja sobre
// elem, no sobre Chart.js JSON — mismo motivo por el que
// chromium.ChartFetcher.renderFunc tampoco lo usa en su rama nativa. El
// nombre/ruta (charts/chart_hash.png) espeja el de chromium.BaseFetcher
// para que el HTML de renderer/html.go (src="assets/%s") funcione sin
// importar qué fetcher generó el archivo.
func (f *NativeChartFetcher) FetchAndSave(ctx context.Context, elem *ast.ChartElement, chartConfig string, outputDir string, width, height int) (string, error) {
	hash := f.chartHash(elem, width, height)

	f.mu.Lock()
	if cached, ok := f.pathCache[hash]; ok {
		f.mu.Unlock()
		return cached, nil
	}
	f.mu.Unlock()

	assetsDir := filepath.Join(outputDir, "charts")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create charts directory: %w", err)
	}
	filename := fmt.Sprintf("chart_%s.png", hash)
	outputPath := filepath.Join(assetsDir, filename)
	relativePath := filepath.Join("charts", filename)

	if _, err := os.Stat(outputPath); err == nil {
		f.mu.Lock()
		f.pathCache[hash] = relativePath
		f.mu.Unlock()
		return relativePath, nil
	}

	data, err := f.render(elem, hash, width, height)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save chart: %w", err)
	}

	f.mu.Lock()
	f.pathCache[hash] = relativePath
	f.mu.Unlock()
	return relativePath, nil
}

// FetchInline rasteriza elem (o reusa el seed) y devuelve los bytes sin
// guardar archivo.
func (f *NativeChartFetcher) FetchInline(ctx context.Context, elem *ast.ChartElement, chartConfig string, width, height int) ([]byte, error) {
	return f.render(elem, f.chartHash(elem, width, height), width, height)
}

// GetImageFormat siempre es "png": go-analyze/charts no produce WebP (el
// gate excluye WebP de este camino, ver tryBuildNativeContext).
func (f *NativeChartFetcher) GetImageFormat() string { return "png" }
