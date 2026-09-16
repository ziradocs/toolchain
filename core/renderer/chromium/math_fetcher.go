// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"

	"go.ziradocs.com/core/v2/renderer"
)

// MathFetcher maneja la obtención y almacenamiento de ecuaciones LaTeX
// renderizadas a SVG (issue #239-B) — mismo mecanismo que MermaidFetcher
// (mermaid_fetcher.go), del que es una copia estructural: MathJax-SVG
// produce un SVG autocontenido igual que Mermaid, así que el patrón de
// fetch/cache/guardado es idéntico, solo cambia el renderer subyacente
// (RenderMathToSVG en vez de RenderMermaidToSVG).
type MathFetcher struct {
	*BaseFetcher
	renderer    *ChromiumRenderer
	themeColors renderer.DiagramThemeColors
}

// SetDiagramThemeColors applies the shared equation/diagram palette to the
// MathJax page and keeps cached SVGs separate per theme.
func (f *MathFetcher) SetDiagramThemeColors(themeColors renderer.DiagramThemeColors) {
	f.themeColors = themeColors
}

func (f *MathFetcher) cacheKey(latex string) string {
	if f.themeColors.IsZero() {
		return GenerateContentHash(latex)
	}
	return GenerateContentHash(latex + "|" + f.themeColors.CacheFingerprint())
}

// NewMathFetcher crea un nuevo fetcher con Chromium renderer
func NewMathFetcher(renderer *ChromiumRenderer, logger FetcherLogger) *MathFetcher {
	fetcher := &MathFetcher{
		BaseFetcher: NewBaseFetcher(renderer, logger, "equations", "MATH"),
		renderer:    renderer,
	}
	// MathJax-SVG siempre produce SVG
	fetcher.SetImageFormat("svg", 0)
	return fetcher
}

// FetchAndSave renderiza una ecuación y la guarda como SVG. Retorna la ruta
// relativa al archivo guardado.
func (f *MathFetcher) FetchAndSave(ctx context.Context, latex string, outputDir string) (string, error) {
	hash := f.cacheKey(latex)

	renderFunc := func() ([]byte, error) {
		svgContent, err := f.renderer.RenderMathToSVGWithTheme(ctx, latex, f.themeColors)
		if err != nil {
			return nil, err
		}
		return []byte(svgContent), nil
	}

	return f.BaseFetcher.FetchAndSave(hash, outputDir, renderFunc)
}

// FetchInline renderiza una ecuación y retorna el SVG como string.
func (f *MathFetcher) FetchInline(ctx context.Context, latex string) (string, error) {
	hash := f.cacheKey(latex)

	renderFunc := func() ([]byte, error) {
		svgContent, err := f.renderer.RenderMathToSVGWithTheme(ctx, latex, f.themeColors)
		if err != nil {
			return nil, err
		}
		return []byte(svgContent), nil
	}

	data, err := f.BaseFetcher.FetchInline(hash, renderFunc)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ClearCache limpia el cache interno
func (f *MathFetcher) ClearCache() {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()
	f.cache = make(map[string]string)
}
