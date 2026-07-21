// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import "context"

// MermaidFetcher maneja la obtención y almacenamiento de diagramas Mermaid renderizados
type MermaidFetcher struct {
	*BaseFetcher
	renderer *ChromiumRenderer
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
	// Generar hash del contenido
	hash := GenerateContentHash(mermaidCode)

	// Función de renderizado
	renderFunc := func() ([]byte, error) {
		svgContent, err := f.renderer.RenderMermaidToSVG(ctx, mermaidCode)
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
	// Generar hash del contenido
	hash := GenerateContentHash(mermaidCode)

	// Función de renderizado
	renderFunc := func() ([]byte, error) {
		svgContent, err := f.renderer.RenderMermaidToSVG(ctx, mermaidCode)
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
