// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"encoding/json"

	"go.ziradocs.com/core/v2/renderer"
)

// MermaidFetcher maneja la obtención y almacenamiento de diagramas Mermaid renderizados
type MermaidFetcher struct {
	*BaseFetcher
	renderer *ChromiumRenderer
	// themeColors son los tokens diagram-* (renderer.DiagramThemeColors).
	// Entran al render Y a la clave de cache: ver cacheKey.
	themeColors renderer.DiagramThemeColors
}

// SetDiagramThemeColors setea los tokens diagram-* que este fetcher reenvía a
// RenderMermaidToSVGWithTheme. Setter en vez de parámetro de
// NewMermaidFetcher porque esa firma está congelada mientras la llamen
// slidelang/doclang por nombre — mismo patrón que
// ChartFetcher.SetCategoricalColors. Zero value reproduce el render de
// siempre.
func (f *MermaidFetcher) SetDiagramThemeColors(themeColors renderer.DiagramThemeColors) {
	f.themeColors = themeColors
}

// cacheKey deriva la clave de cache de UN solo lugar, para que FetchAndSave y
// FetchInline no puedan desincronizarse.
//
// El tema TIENE que entrar acá. Antes se hasheaba solo mermaidCode, y
// BaseFetcher.FetchAndSave devuelve el archivo ya existente en disco (os.Stat)
// sin volver a rasterizar: dos builds al mismo outputDir con temas distintos
// habrían servido el SVG del primero. Es exactamente el mismo modo de falla
// que una revisión de #250 encontró en los dos fetchers de charts, así que
// acá se cubre desde el principio en vez de esperar a que lo encuentren.
//
// Solo se agrega cuando hay tema, para que un fetcher sin colores produzca el
// MISMO hash de siempre y no invalide los diagramas ya cacheados.
func (f *MermaidFetcher) cacheKey(mermaidCode string) string {
	if f.themeColors.IsZero() {
		return GenerateContentHash(mermaidCode)
	}
	// JSON y no concatenación: un color CSS puede traer comas y separadores
	// (rgb(1,2,3)), y un join ingenuo volvería ambiguos dos temas distintos.
	theme, _ := json.Marshal(f.themeColors)
	return GenerateContentHash(mermaidCode + "|" + string(theme))
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
