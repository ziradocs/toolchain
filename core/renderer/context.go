// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"context"

	"go.ziradocs.com/core/v2/util"
)

// RenderContext mantiene el contexto de rendering compartido
// Permite configurar opciones como PlantUML mode sin modificar todas las firmas
type RenderContext struct {
	PlantUMLMode   string          // "browser", "offline-assets", "offline-inline"
	PlantUMLServer string          // Custom server URL
	PlantUMLFormat string          // "svg" or "png"
	MermaidMode    string          // "browser", "offline-assets", "offline-inline"
	ChartMode      string          // "browser", "offline-assets", "offline-inline"
	MapMode        string          // "browser", "offline-assets", "offline-inline"
	MathMode       string          // "browser", "offline-assets", "offline-inline" (issue #239-B)
	OutputDir      string          // Output directory for assets
	Fetcher        PlantUMLFetcher // PlantUML fetcher inicializado (nil-able, ver interfaces en fetchers.go)
	MermaidFetcher MermaidFetcher  // Mermaid fetcher inicializado
	ChartFetcher   ChartFetcher    // Chart fetcher inicializado
	MapFetcher     MapFetcher      // Map fetcher inicializado
	MathFetcher    MathFetcher     // Math fetcher inicializado
	// Logger recibe los warnings/debug best-effort de GenerateDocumentHTML
	// (nonce CSP fallido, variable de tema rechazada por
	// SanitizeCSSCustomProperty) — issue #134/G1c. Antes esos dos sitios
	// llamaban util.Debug/util.Warn (el logger global de conveniencia del
	// CLI, `defaultLogger`), lo que solo funcionaba si el caller había
	// llamado util.InitDefault (cierto para slidelang, nunca cierto
	// para doclang, que arma su propio *util.Logger sin cablear el
	// global) — doclang perdía en silencio exactamente los warnings de
	// seguridad que BA-11 quería visibles. Logger nunca es nil tras pasar
	// por resolveRenderContext/NewDefaultRenderContext.
	Logger util.Logger
	// Ctx gobierna la cancelación/timeout de las llamadas a los fetchers de
	// PlantUML/Mermaid/Chart/Map (issue #134/G1d) — un caller (p. ej. un
	// servidor que quiere abortar el render de un request cuyo cliente se
	// desconectó) lo cancela y esa cancelación se propaga hasta la operación
	// de red/Chromium en curso. Nunca es nil tras pasar por
	// resolveRenderContext/NewDefaultRenderContext.
	Ctx context.Context
}

// NewDefaultRenderContext retorna un RenderContext con todos los modos en
// "browser" (rendering client-side contra CDNs, sin fetchers de Chromium) —
// el mismo default que antes sintetizaba GetRenderContext() cuando el
// global no se había configurado. Los callers que no necesitan pre-render
// offline (JSON, previews, tests) pueden usar este valor explícitamente en
// vez de depender de un fallback implícito. Logger por defecto es un noop;
// un caller que quiera ver los warnings de GenerateDocumentHTML debe
// asignar su propio ctx.Logger antes de llamarla.
func NewDefaultRenderContext() *RenderContext {
	return &RenderContext{
		PlantUMLMode:   "browser",
		PlantUMLServer: "",
		PlantUMLFormat: "svg",
		MermaidMode:    "browser",
		ChartMode:      "browser",
		MapMode:        "browser",
		MathMode:       "browser",
		OutputDir:      "",
		Fetcher:        nil,
		MermaidFetcher: nil,
		ChartFetcher:   nil,
		MapFetcher:     nil,
		MathFetcher:    nil,
		Logger:         util.NewNoop(),
		Ctx:            context.Background(),
	}
}

// resolveRenderContext normaliza un *RenderContext posiblemente nil al
// default de arriba — mismo nil-safety que antes daba el fallback de
// GetRenderContext() cuando el global no estaba seteado, ahora explícito en
// cada punto de entrada en vez de implícito vía estado de paquete. También
// cubre un ctx no-nil pero con Logger sin asignar (p. ej. un caller que
// arma el struct literal a mano en vez de vía NewDefaultRenderContext).
func resolveRenderContext(ctx *RenderContext) *RenderContext {
	if ctx == nil {
		return NewDefaultRenderContext()
	}
	if ctx.Logger == nil {
		ctx.Logger = util.NewNoop()
	}
	if ctx.Ctx == nil {
		ctx.Ctx = context.Background()
	}
	return ctx
}

// IsOfflineRenderMode indica si un modo de rendering pre-renderiza en build time
// (necesita Chromium), a diferencia de "browser"/"" que renderiza client-side.
// Fuente única del predicado, compartida por ambos CLIs para no duplicar la
// comparación de literales en cada capa (issue #92).
func IsOfflineRenderMode(mode string) bool {
	return mode == "offline-assets" || mode == "offline-inline"
}
