// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"

	"go.ziradocs.com/core/v2/renderer"
)

// MermaidFetcher maneja la obtención y almacenamiento de diagramas Mermaid renderizados
type MermaidFetcher struct {
	*BaseFetcher
	renderer    *ChromiumRenderer
	themeColors renderer.DiagramThemeColors
}

// SetDiagramThemeColors configures the resolved visual values used while
// rasterizing Mermaid. It also changes the cache key, avoiding stale SVGs
// when a caller rebuilds into the same assets directory with another theme.
func (f *MermaidFetcher) SetDiagramThemeColors(themeColors renderer.DiagramThemeColors) {
	f.themeColors = themeColors
}

func (f *MermaidFetcher) cacheKey(mermaidCode string) string {
	if f.themeColors.IsZero() {
		return GenerateContentHash(mermaidCode)
	}
	return GenerateContentHash(mermaidCode + "|" + f.themeColors.CacheFingerprint())
}

// NewMermaidFetcher crea un nuevo fetcher con Chromium renderer
func NewMermaidFetcher(renderer *ChromiumRenderer, logger FetcherLogger) *MermaidFetcher {
	fetcher := &MermaidFetcher{
		BaseFetcher: NewBaseFetcher(renderer, logger, "diagrams", "MERMAID"),
		renderer:    renderer,
	}
	// Mermaid siempre usa SVG
	fetcher.SetImageFormat("svg", 0)
	return fetcher
}

// FetchAndSave renderiza un diagrama Mermaid y lo guarda como SVG
// Retorna la ruta relativa al archivo guardado
func (f *MermaidFetcher) FetchAndSave(ctx context.Context, mermaidCode string, outputDir string) (string, error) {
	hash := f.cacheKey(mermaidCode)

	// Función de renderizado
	renderFunc := func() ([]byte, error) {
		svgContent, err := f.renderer.RenderMermaidToSVGWithTheme(ctx, mermaidCode, f.themeColors)
		if err != nil {
			return nil, err
		}
		return []byte(svgContent), nil
	}

	// Usar BaseFetcher para manejar cache y guardado
	return f.BaseFetcher.FetchAndSave(hash, outputDir, renderFunc)
}

// FetchInline renderiza un diagrama Mermaid y retorna el SVG como string
func (f *MermaidFetcher) FetchInline(ctx context.Context, mermaidCode string) (string, error) {
	hash := f.cacheKey(mermaidCode)

	// Función de renderizado
	renderFunc := func() ([]byte, error) {
		svgContent, err := f.renderer.RenderMermaidToSVGWithTheme(ctx, mermaidCode, f.themeColors)
		if err != nil {
			return nil, err
		}
		return []byte(svgContent), nil
	}

	// Usar BaseFetcher para manejar rendering inline
	data, err := f.BaseFetcher.FetchInline(hash, renderFunc)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ClearCache limpia el cache interno
func (f *MermaidFetcher) ClearCache() {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()
	f.cache = make(map[string]string)
}
