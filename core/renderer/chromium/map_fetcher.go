// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"encoding/json"
	"fmt"

	"go.ziradocs.com/core/v2/renderer"
)

// MapFetcher maneja la obtención y almacenamiento de mapas renderizados
type MapFetcher struct {
	*BaseFetcher
	renderer *ChromiumRenderer
}

// NewMapFetcher crea un nuevo fetcher con Chromium renderer
func NewMapFetcher(renderer *ChromiumRenderer, logger FetcherLogger) *MapFetcher {
	return &MapFetcher{
		BaseFetcher: NewBaseFetcher(renderer, logger, "maps", "MAP"),
		renderer:    renderer,
	}
}

// renderFunc arma la función de renderizado compartida por FetchAndSave/
// FetchInline: issue #130 — intenta primero go-staticmaps (sin Chromium) vía
// renderer.RenderMapNativePNG cuando el formato pedido es PNG (go-staticmaps no
// produce WebP); si falla, cae al pipeline chromedp existente — nunca un
// error duro por preferir lo nativo.
func (f *MapFetcher) renderFunc(ctx context.Context, config renderer.MapConfig, width, height int) func() ([]byte, error) {
	return func() ([]byte, error) {
		if f.GetImageFormat() != "webp" {
			data, err := renderer.RenderMapNativePNG(ctx, config, width, height)
			if err == nil {
				return data, nil
			}
			f.logger.Warn(f.logTag, "native map render failed, falling back to Chromium: %v", err)
		}
		if f.GetImageFormat() == "webp" {
			return f.renderer.RenderMapToWebP(ctx, config, width, height, f.webpQuality)
		}
		return f.renderer.RenderMapToPNG(ctx, config, width, height)
	}
}

// FetchAndSave renderiza un mapa y lo guarda (PNG o WebP según configuración)
// Retorna la ruta relativa al archivo guardado
func (f *MapFetcher) FetchAndSave(ctx context.Context, config renderer.MapConfig, outputDir string, width, height int) (string, error) {
	hash := generateMapHash(config, width, height)
	return f.BaseFetcher.FetchAndSave(hash, outputDir, f.renderFunc(ctx, config, width, height))
}

// FetchInline renderiza un mapa y retorna imagen (PNG o WebP) para embedding
func (f *MapFetcher) FetchInline(ctx context.Context, config renderer.MapConfig, width, height int) ([]byte, error) {
	hash := generateMapHash(config, width, height)
	return f.BaseFetcher.FetchInline(hash, f.renderFunc(ctx, config, width, height))
}

// generateMapHash genera un hash único para la configuración del mapa.
// width/height se agregan explícitamente a lo hasheado (no solo config, que
// ya cubre center/zoom/mapType/markers/heatmap): son parte de lo que afecta
// los bytes del PNG en AMBOS backends (chromedp toma un screenshot de esas
// dimensiones exactas; el nativo las pasa a ctx.SetSize), y antes no
// formaban parte del hash — dos mapas con el mismo renderer.MapConfig pero distinto
// width/height colisionaban en el mismo nombre de archivo de cache y
// FetchAndSave servía el PNG del tamaño equivocado vía el os.Stat() temprano
// sin siquiera invocar renderFunc (mismo patrón de hallazgo de security
// review que cacheKeyInput en chart_fetcher.go, PR #163 — acá era
// pre-existente incluso antes de agregar el render nativo, porque el
// screenshot de chromedp ya dependía de width/height).
func generateMapHash(config renderer.MapConfig, width, height int) string {
	jsonData, _ := json.Marshal(config)
	return GenerateContentHash(fmt.Sprintf("%s|%dx%d", jsonData, width, height))
}

// ClearCache limpia el cache interno
func (f *MapFetcher) ClearCache() {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()
	f.cache = make(map[string]string)
}
