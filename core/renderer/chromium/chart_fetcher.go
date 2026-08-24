// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"fmt"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
)

// ChartFetcher maneja la obtención y almacenamiento de gráficos renderizados
type ChartFetcher struct {
	*BaseFetcher
	renderer *ChromiumRenderer
	// categoricalColors es RenderContext.ChartCategoricalColors, cableado
	// hasta acá porque renderFunc prefiere el rasterizador nativo (abajo)
	// para bar/line/pie/doughnut — el camino chartConfig/Chart.js (que sí
	// ve ChartCategoricalColors vía GenerateChartConfigWithMode) nunca se
	// ejecuta para esos tipos salvo que el render nativo falle. Sin este
	// campo el tema le llegaría solo a combo/scatter, que es la minoría de
	// los charts reales (motor-temas-v2.md §2.2) — un chart en modo JSON no
	// recibe el tema por NINGÚN camino, a propósito: ver el comentario de
	// RenderContext.ChartCategoricalColors.
	categoricalColors []string
}

// NewChartFetcher crea un nuevo fetcher con Chromium renderer. Firma sin
// cambios a propósito (issue "una sola danza" de motor-temas-v2.md): este
// símbolo lo llama slidelang/internal/generator/offline.go directamente, y
// CI corre workspace-integration (slidelang contra el core DEL ÁRBOL vía
// go.work) — agregarle un parámetro acá rompería ese build entre el merge
// de este PR y el PR de slidelang que lo consuma. Usar SetCategoricalColors
// (mismo patrón que SetImageFormat) para setear el override después de
// construir.
func NewChartFetcher(renderer *ChromiumRenderer, logger FetcherLogger) *ChartFetcher {
	return &ChartFetcher{
		BaseFetcher: NewBaseFetcher(renderer, logger, "charts", "CHART"),
		renderer:    renderer,
	}
}

// SetCategoricalColors setea el override de RenderContext.ChartCategoricalColors
// que renderFunc reenvía a RenderChartNativePNGWithColors — ver el comentario
// del campo categoricalColors. nil dado (el zero value si nunca se llama)
// reproduce el render nativo por defecto.
func (f *ChartFetcher) SetCategoricalColors(categoricalColors []string) {
	f.categoricalColors = categoricalColors
}

// renderFunc arma la función de renderizado compartida por FetchAndSave/
// FetchInline: issue #130 — intenta primero go-analyze/charts (sin
// Chromium) vía renderer.RenderChartNativePNGWithColors cuando elem lo soporta
// (bar/line/pie/doughnut, no IsJSONMode) y el formato pedido es PNG
// (go-analyze/charts no produce WebP); si no aplica, o si el intento nativo
// falla, cae al pipeline chromedp existente — nunca un error duro por
// preferir lo nativo.
func (f *ChartFetcher) renderFunc(ctx context.Context, elem *ast.ChartElement, chartConfig string, width, height int) func() ([]byte, error) {
	return func() ([]byte, error) {
		if f.GetImageFormat() != "webp" && elem != nil {
			if data, ok, nativeErr := renderer.RenderChartNativePNGWithColors(elem, width, height, f.categoricalColors); ok {
				if nativeErr == nil {
					return data, nil
				}
				f.logger.Warn(f.logTag, "native chart render failed, falling back to Chromium: %v", nativeErr)
			}
		}
		if f.GetImageFormat() == "webp" {
			return f.renderer.RenderChartToWebP(ctx, chartConfig, width, height, f.webpQuality)
		}
		return f.renderer.RenderChartToPNG(ctx, chartConfig, width, height)
	}
}

// cacheKeyInput arma el string a hashear para el cache de disco de
// FetchAndSave (offline-assets): antes de esta PR, chartConfig por sí solo
// era suficiente porque era la ÚNICA entrada que afectaba los bytes del PNG
// (elem.Title nunca se usaba en GenerateChartConfigWithMode - solo se pinta
// aparte, en un <div class="chart-title"> fuera de la imagen, ver html.go).
// renderer.RenderChartNativePNG cambia eso: dibuja elem.Title DENTRO del PNG
// (opt.Title.Text), así que dos charts con el mismo chartConfig pero
// distinto Title ahora producen bytes distintos, y sin incluir Title acá,
// FetchAndSave serviría un PNG con el título viejo/de otro chart desde
// os.Stat(outputPath) sin siquiera invocar renderFunc (hallazgo de security
// review sobre PR #163). width/height también se agregan por la misma
// razón (ya eran parte de lo que afecta los bytes del PNG, en ambos
// caminos, y no estaban en el hash).
func cacheKeyInput(elem *ast.ChartElement, chartConfig string, width, height int) string {
	title := ""
	if elem != nil {
		title = elem.Title
	}
	return fmt.Sprintf("%s|%s|%dx%d", chartConfig, title, width, height)
}

// FetchAndSave renderiza un gráfico y lo guarda (PNG o WebP según configuración)
// Retorna la ruta relativa al archivo guardado. elem es el ChartElement
// original (nil-safe: si es nil, se salta el intento nativo) — se necesita
// además de chartConfig (el JSON Chart.js ya serializado) porque el camino
// nativo trabaja sobre los datos estructurados, no sobre Chart.js JSON.
func (f *ChartFetcher) FetchAndSave(ctx context.Context, elem *ast.ChartElement, chartConfig string, outputDir string, width, height int) (string, error) {
	hash := GenerateContentHash(cacheKeyInput(elem, chartConfig, width, height))
	return f.BaseFetcher.FetchAndSave(hash, outputDir, f.renderFunc(ctx, elem, chartConfig, width, height))
}

// FetchInline renderiza un gráfico y retorna imagen (PNG o WebP) para embedding
func (f *ChartFetcher) FetchInline(ctx context.Context, elem *ast.ChartElement, chartConfig string, width, height int) ([]byte, error) {
	hash := GenerateContentHash(cacheKeyInput(elem, chartConfig, width, height))
	return f.BaseFetcher.FetchInline(hash, f.renderFunc(ctx, elem, chartConfig, width, height))
}

// ClearCache limpia el cache interno
func (f *ChartFetcher) ClearCache() {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()
	f.cache = make(map[string]string)
}
