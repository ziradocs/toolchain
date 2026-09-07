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
	PlantUMLMode   string // "browser", "offline-assets", "offline-inline"
	PlantUMLServer string // Custom server URL
	PlantUMLFormat string // "svg" or "png"
	MermaidMode    string // "browser", "offline-assets", "offline-inline"
	ChartMode      string // "browser", "offline-assets", "offline-inline"
	MapMode        string // "browser", "offline-assets", "offline-inline"
	MathMode       string // "browser", "offline-assets", "offline-inline" (issue #239-B)
	// ImageMode (issue #167) gobierna renderImageElement de forma más
	// angosta que los *Mode de arriba: solo "offline-inline" cambia algo
	// (inlinea fuentes locales como data: URI); "browser" y
	// "offline-assets" se comportan igual — offline-assets NO copia la
	// imagen a un assets/ propio en esta primera pasada, queda para un
	// follow-up si hace falta. El motivo del modo "offline-inline" es
	// estructural, no una preferencia de formato: el pipeline de PDF
	// inyecta el HTML final en about:blank vía Page.SetDocumentContent
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5), que no tiene base URL contra
	// la cual una <img src="ruta/relativa"> pueda resolver — sin inlinear,
	// la imagen queda rota en CUALQUIER PDF, sin importar que el archivo
	// exista.
	ImageMode string
	// AssetRoot confina las fuentes de imagen LOCALES elegibles para
	// inlinear bajo ImageMode == "offline-inline" (vía
	// util.ResolveConfinedPath — mismo mecanismo de confinamiento AL-4 que
	// ya usan DOCX/PPTX). Vacío deshabilita el inlineo por completo: sin
	// una raíz de confinamiento explícita, una fuente local se deja tal
	// cual (mismo comportamiento roto de siempre) en vez de leer del
	// filesystem sin límite alguno.
	AssetRoot string
	// ChartCategoricalColors overrides the built-in categorical palette used
	// to color chart series/segments in the offline/PDF render path —
	// nil/empty (every caller today) reproduces the existing hardcoded
	// defaults exactly, byte for byte. Reaches TWO independent renderers,
	// not one: GenerateChartConfigWithMode (Chart.js JSON, combo/scatter and
	// the browser path — NOT JSON-mode, see below) AND
	// RenderChartNativePNGWithColors via chromium.ChartFetcher's
	// categoricalColors field (go-analyze/charts, which is what actually
	// renders bar/line/pie/doughnut in offline/PDF — issue #130 makes it
	// the preferred path for those types, so it is NOT a minor branch).
	// A chart in JSON mode (elem.IsJSONMode/RawJSON — the author writes the
	// Chart.js config by hand) reaches NEITHER: renderChartElement
	// re-serializes RawJSON verbatim and skips GenerateChartConfigWithMode
	// entirely, and RenderChartNativePNGWithColors's own
	// SupportsNativeChartRenderingWithOptions gate rejects IsJSONMode
	// upfront — this is intentional (an author's literal config should not
	// be silently overridden by a theme), not a gap to close. Motor-temas-v2.md
	// §2.2's chart-cat-* contract: this
	// is an ORDERED set indexed by modulo, the same way the hardcoded
	// defaults already are — it exists so a caller that HAS resolved a
	// theme (slidelang, from --theme) can hand over its chart-cat-1..8
	// tokens as literal colors (RenderContext has no notion of "theme"
	// itself; it only carries already-resolved values). Never generate a
	// new shade for a series past the set's length — wrap via modulo like
	// the code that consumes this already does, don't invent one.
	ChartCategoricalColors []string
	// ChartThemeColors son los tokens chart-* NO categóricos de
	// motor-temas-v2.md §2.2 (surface/grid/axis/label) para el camino
	// offline/PDF. Zero value (todo caller de hoy) reproduce el render
	// anterior byte por byte, igual que ChartCategoricalColors.
	//
	// Va SEPARADO de ChartCategoricalColors, no fusionado en un struct
	// único, porque ese campo ya es consumido por slidelang: cambiarle el
	// tipo rompería workspace-integration (slidelang contra el core del
	// árbol) entre el merge de este PR y el consumidor.
	//
	// Ver ChartThemeColors para POR QUÉ chart-tooltip-bg y chart-seq-1..5
	// no están acá: no es plumbing faltante, es que no hay nada que puedan
	// pintar en un PNG estático. Siguen siendo browser-only.
	//
	// Igual que ChartCategoricalColors, NO alcanza a un chart en modo JSON:
	// la config literal del autor no se sobreescribe con el tema.
	ChartThemeColors ChartThemeColors
	// DiagramThemeColors son los tokens diagram-* de §2.2 para el camino
	// offline/PDF de Mermaid. Zero value (todo caller de hoy) reproduce el
	// render anterior byte por byte.
	//
	// Solo alcanza al camino Chromium: Mermaid vía --diagram-backend kroki se
	// renderiza en un servidor REMOTO, donde no hay mermaid.initialize() que
	// interceptar. Tematizar ese backend exige inyectar una directiva
	// %%{init:...}%% en el fuente antes del POST — otro mecanismo, y con la
	// misma incógnita que PlantUML (no controlamos la versión de Mermaid que
	// corre esa instancia). Queda como hueco DECLARADO, no silencioso.
	DiagramThemeColors DiagramThemeColors
	OutputDir          string          // Output directory for assets
	Fetcher            PlantUMLFetcher // PlantUML fetcher inicializado (nil-able, ver interfaces en fetchers.go)
	MermaidFetcher     MermaidFetcher  // Mermaid fetcher inicializado
	ChartFetcher       ChartFetcher    // Chart fetcher inicializado
	MapFetcher         MapFetcher      // Map fetcher inicializado
	MathFetcher        MathFetcher     // Math fetcher inicializado
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
		PlantUMLMode:           "browser",
		PlantUMLServer:         "",
		PlantUMLFormat:         "svg",
		MermaidMode:            "browser",
		ChartMode:              "browser",
		MapMode:                "browser",
		MathMode:               "browser",
		ImageMode:              "browser",
		AssetRoot:              "",
		ChartCategoricalColors: nil,
		ChartThemeColors:       ChartThemeColors{},
		DiagramThemeColors:     DiagramThemeColors{},
		OutputDir:              "",
		Fetcher:                nil,
		MermaidFetcher:         nil,
		ChartFetcher:           nil,
		MapFetcher:             nil,
		MathFetcher:            nil,
		Logger:                 util.NewNoop(),
		Ctx:                    context.Background(),
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
