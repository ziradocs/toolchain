// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"

	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/core/util"
)

// RenderContextOptions configura NewRenderContext — mismos nombres/semántica
// que los campos equivalentes de renderer.DocumentHTMLOptions (issue #134/G1b).
type RenderContextOptions struct {
	PlantUMLMode   string
	PlantUMLServer string
	PlantUMLFormat string
	MermaidMode    string
	ChartMode      string
	MapMode        string
	MathMode       string // issue #239-B
	OutputDir      string
	ImageFormat    string
	WebPQuality    int
	// Logger recibe los warnings/debug best-effort de GenerateDocumentHTML
	// (issue #134/G1c) — nil degrada a un noop vía renderer.RenderContext.Logger.
	Logger util.Logger
	// Ctx gobierna la cancelación/timeout de los fetchers de PlantUML/
	// Mermaid/Chart/Map (issue #134/G1d) — nil degrada a context.Background()
	// vía renderer.RenderContext.Ctx.
	Ctx context.Context
}

// NewRenderContext arma un *renderer.RenderContext con los fetchers de
// mermaid/chart/map/plantuml correspondientes a opts. cr puede ser nil: solo
// gatilla la construcción de los fetchers de mermaid/chart/map (que sí
// dependen de Chromium); PlantUML no lo necesita (fetcher HTTP puro, ver
// plantuml_fetcher.go) y se construye igual con cr==nil.
//
// Único punto de verdad para esta construcción (issue #134/G1b) — antes
// vivía duplicada dentro de renderer.GenerateDocumentHTML (pre-split), que
// no podía seguir construyendo fetchers de renderer/chromium sin crear un
// ciclo de import (chromium -> renderer -> chromium). Cada caller (ambos
// CLIs) arma su propio *renderer.RenderContext explícitamente y lo pasa a
// renderer.RenderElementToHTML/GenerateDocumentHTML (mismo patrón que G1a).
func NewRenderContext(cr *ChromiumRenderer, opts RenderContextOptions) *renderer.RenderContext {
	plantumlMode := opts.PlantUMLMode
	if plantumlMode == "" {
		plantumlMode = "browser"
	}
	plantumlFormat := opts.PlantUMLFormat
	if plantumlFormat == "" {
		plantumlFormat = "svg"
	}

	// Los locals de abajo se declaran con el tipo interfaz (renderer.XxxFetcher),
	// no con el puntero concreto (*XxxFetcher): asignar un *XxxFetcher(nil) a un
	// campo interfaz produce una interfaz NO nil (type=*XxxFetcher, value=nil),
	// así que los `if ctx.XxxFetcher == nil` de html.go dejarían de detectar el
	// caso "sin fetcher" y en cambio panicarían al invocar un método sobre un
	// receiver nil. Declarando el local ya como interfaz, dejarlo sin asignar
	// conserva el nil verdadero (hallazgo de security-review sobre PR #176).
	var fetcher renderer.PlantUMLFetcher
	if renderer.IsOfflineRenderMode(plantumlMode) {
		fetcher = NewPlantUMLFetcher(opts.PlantUMLServer, plantumlFormat, opts.OutputDir)
	}

	mermaidMode := opts.MermaidMode
	if mermaidMode == "" {
		mermaidMode = "browser"
	}
	var mermaidFetcher renderer.MermaidFetcher
	if cr != nil && renderer.IsOfflineRenderMode(mermaidMode) {
		mermaidFetcher = NewMermaidFetcher(cr, renderer.NoopFetcherLogger{})
	}

	imageFormat := opts.ImageFormat
	if imageFormat == "" {
		imageFormat = "png"
	}
	webpQuality := opts.WebPQuality
	if webpQuality == 0 {
		webpQuality = 85
	}

	chartMode := opts.ChartMode
	if chartMode == "" {
		chartMode = "browser"
	}
	var chartFetcher renderer.ChartFetcher
	if cr != nil && renderer.IsOfflineRenderMode(chartMode) {
		cf := NewChartFetcher(cr, renderer.NoopFetcherLogger{})
		cf.SetImageFormat(imageFormat, webpQuality)
		chartFetcher = cf
	}

	mapMode := opts.MapMode
	if mapMode == "" {
		mapMode = "browser"
	}
	var mapFetcher renderer.MapFetcher
	if cr != nil && renderer.IsOfflineRenderMode(mapMode) {
		mf := NewMapFetcher(cr, renderer.NoopFetcherLogger{})
		mf.SetImageFormat(imageFormat, webpQuality)
		mapFetcher = mf
	}

	mathMode := opts.MathMode
	if mathMode == "" {
		mathMode = "browser"
	}
	var mathFetcher renderer.MathFetcher
	if cr != nil && renderer.IsOfflineRenderMode(mathMode) {
		mathFetcher = NewMathFetcher(cr, renderer.NoopFetcherLogger{})
	}

	logger := opts.Logger
	if logger == nil {
		logger = util.NewNoop()
	}

	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	return &renderer.RenderContext{
		PlantUMLMode:   plantumlMode,
		PlantUMLServer: opts.PlantUMLServer,
		PlantUMLFormat: plantumlFormat,
		MermaidMode:    mermaidMode,
		ChartMode:      chartMode,
		MapMode:        mapMode,
		MathMode:       mathMode,
		OutputDir:      opts.OutputDir,
		Fetcher:        fetcher,
		MermaidFetcher: mermaidFetcher,
		ChartFetcher:   chartFetcher,
		MapFetcher:     mapFetcher,
		MathFetcher:    mathFetcher,
		Logger:         logger,
		Ctx:            ctx,
	}
}
