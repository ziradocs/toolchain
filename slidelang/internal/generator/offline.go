// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package generator

import (
	"context"
	"fmt"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/core/renderer/chromium"
)

// offline.go conecta slidelang al pipeline de rendering offline que ya vive en
// core/renderer (el mismo que usa doclang), en vez de reimplementarlo
// (issue #92). En modos offline, mermaid/chart/map se pre-renderizan con Chromium
// a assets/ (offline-assets) o inline como SVG/data-URI (offline-inline); en modo
// browser slidelang mantiene su rendering client-side contra CDNs.

// slidelangChromiumBrand aísla los directorios de instalación/cache de
// Chromium y el texto de ayuda bajo la marca de slidelang (en vez de heredar
// los de doclang: ~/.doclang, mensajes "doclang build ..."). Se inyecta por
// instancia vía chromium.NewChromiumRendererWithBrand (issue #92) — NO
// mutando estado global, a diferencia del diseño anterior (dos `var` a nivel
// de paquete en core), señalado en code review de PR #122 como un antipatrón
// sin thread-safety.
var slidelangChromiumBrand = chromium.ChromiumBrand{
	Name: "slidelang",
	InstallHint: `Examples:
  slidelang build deck.slidelang --format html --render-mode=offline-assets --install-chromium
  slidelang build deck.slidelang --format html --render-mode=offline-assets --chromium-path=/usr/bin/chromium
  slidelang build deck.slidelang --format html --render-mode=offline-inline --chromium-path="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"`,
}

// hasInteractiveElements indica si el AST contiene algún mermaid/chart/map — los
// únicos elementos que el pipeline offline pre-renderiza. Un deck sin ellos no
// necesita Chromium aunque se pida un modo offline.
func hasInteractiveElements(astNode *ast.AST) bool {
	if astNode == nil {
		return false
	}
	for _, block := range astNode.ContentBlocks {
		for _, el := range block.Elements {
			switch el.(type) {
			case *ast.MermaidElement, *ast.ChartElement, *ast.MapElement:
				return true
			}
		}
	}
	return false
}

// SetupOfflineRenderContext arma el *renderer.RenderContext que el caller debe
// pasar explícitamente (issue #134/G1a) a GenerateWithOptions para que
// renderer.RenderElementToHTML pre-renderice mermaid/chart/map en modos
// offline. Devuelve ese contexto y una función de cleanup (siempre no-nil)
// que cierra Chromium; en modo browser, o si el deck no tiene elementos
// interactivos, el contexto es NewDefaultRenderContext() y el cleanup es un
// no-op. El contexto SOLO es nil cuando err también es no-nil (fallo al
// inicializar Chromium) — todo caller debe chequear err antes de usar el
// contexto retornado, igual que ya hace runBuild (build.go).
//
// Está pensado para llamarse UNA vez envolviendo todo el build (antes del loop de
// formatos en runBuild), de modo que un fallo de inicialización de Chromium ocurra
// ANTES de escribir cualquier formato al disco — sin dejar salida parcial. slidelang
// no renderiza PlantUML, así que ese modo queda en "browser".
func (g *Generator) SetupOfflineRenderContext(astNode *ast.AST, outputDir string, opts GeneratorOptions) (*renderer.RenderContext, func(), error) {
	noop := func() {}
	if !opts.IsOffline() {
		return renderer.NewDefaultRenderContext(), noop, nil
	}
	// Un deck de solo texto no necesita Chromium aunque se pida offline; no
	// forzamos su instalación/arranque para nada (issue #92).
	if !hasInteractiveElements(astNode) {
		return renderer.NewDefaultRenderContext(), noop, nil
	}

	// issue #164: un deck cuyos charts son TODOS nativo-capaces (y que no
	// trae mermaid/math/mapas) tampoco necesita Chromium — se rasterizan con
	// go-analyze/charts en memoria. Deliberadamente NO se toca
	// hasInteractiveElements (compartido con pdf.go, que siempre tiene
	// Chromium ya instanciado sin importar esto — ver pdf.go:42) ni
	// buildInteractiveRenderContext: esta es una rama nueva y aparte.
	if ctx, ok := g.tryBuildNativeContext(astNode, outputDir, opts); ok {
		g.logger.Info("HTML", "✅ Offline rendering habilitado sin Chromium (todos los elementos son nativo-capaces)")
		return ctx, noop, nil
	}

	// Marca de slidelang inyectada por instancia (issue #92) — sin tocar estado
	// global de core; doclang no se ve afectado ni necesita restaurar nada.
	logAdapter := &renderer.ChromiumLoggerAdapter{Logger: g.logger}
	chromiumR, err := chromium.NewChromiumRendererWithBrand(context.Background(), opts.ChromiumPath, opts.InstallChromium, logAdapter, slidelangChromiumBrand)
	if err != nil {
		return nil, noop, fmt.Errorf("failed to initialize Chromium for offline rendering: %w", err)
	}

	ctx := buildInteractiveRenderContext(chromiumR, outputDir, opts)

	g.logger.Info("HTML", "✅ Offline rendering habilitado (render-mode: %s, image-format: %s)", opts.RenderMode, resolveImageFormat(opts.ImageFormat))

	return ctx, func() {
		chromiumR.Close()
	}, nil
}

// tryBuildNativeContext intenta armar un *renderer.RenderContext que
// rasteriza todos los charts del documento SIN Chromium (issue #164),
// devolviendo (ctx, true) solo si TODO chart rasteriza nativo con ÉXITO
// —probado de verdad, no por elegibilidad de tipo— y el documento no trae
// mermaid/math (Chromium-only) ni mapas (ver abajo) ni se pidió webp. Los
// bytes ya probados se SIEMBRAN en el fetcher (chartFetcher.Seed), así que
// el render de salida los reusa en vez de volver a rasterizar — una sola
// pasada por chart en todo el build, no dos. Si algún chart no puede
// renderizarse nativo, o hay un elemento que requiere Chromium, devuelve
// (nil, false) y el caller cae al camino Chromium (que sí tiene fallback).
//
// Por qué probar el render y no solo el tipo:
// renderer.SupportsNativeChartRendering solo mira tipo/Options/JSONMode; un
// chart type-eligible con DATOS malformados (vacíos, no numéricos, filas
// irregulares — ver renderer/native_chart.go) igual falla el render real
// (RenderChartNativePNG: ok=true, err!=nil), y como este camino no
// instancia Chromium, terminaría en un <div class="chart-error"> sin el
// fallback a Chart.js que chromium.ChartFetcher SÍ ofrece — regresión real
// encontrada en code review (bot Codex sobre la primera versión de este PR).
//
// Por qué los mapas quedan FUERA del camino nativo: a diferencia de un chart
// (render nativo puro-Go, en memoria, determinístico, barato de probar), el
// render nativo de un mapa puede fallar en runtime por causas de red/tiles/
// timeout/panic (renderer/native_map.go) que no se pueden probar barato de
// antemano; sin Chromium eso sería un <div class="map-error"> permanente en
// vez del fallback que chromium.MapFetcher ofrece (segundo hallazgo de la
// misma review). Un deck con CUALQUIER mapa se rutea a Chromium, donde
// chromium.MapFetcher igual intenta go-staticmaps primero — el mapa se
// rasteriza nativo en el happy path, solo que con Chromium instanciado como
// red de seguridad. El costo es solo el arranque de Chromium para decks con
// mapas, no la calidad del render.
func (g *Generator) tryBuildNativeContext(astNode *ast.AST, outputDir string, opts GeneratorOptions) (*renderer.RenderContext, bool) {
	if resolveImageFormat(opts.ImageFormat) == "webp" {
		return nil, false
	}
	chartFetcher := renderer.NewNativeChartFetcher()
	if astNode != nil {
		for _, block := range astNode.ContentBlocks {
			for _, el := range block.Elements {
				switch e := el.(type) {
				case *ast.MermaidElement, *ast.MathElement, *ast.MapElement:
					return nil, false
				case *ast.ChartElement:
					if !renderer.SupportsNativeChartRendering(e) {
						return nil, false
					}
					width, height := renderer.ChartDimensions(e)
					data, ok, err := renderer.RenderChartNativePNG(e, width, height)
					if !ok || err != nil {
						return nil, false
					}
					chartFetcher.Seed(e, width, height, data)
				}
			}
		}
	}
	// Sin MapFetcher: este camino solo se alcanza para documentos sin mapas
	// (el case *ast.MapElement de arriba retorna false), así que ningún
	// render de mapa puede llegar acá.
	return &renderer.RenderContext{
		ChartMode:    opts.RenderMode,
		PlantUMLMode: "browser",
		OutputDir:    outputDir,
		ChartFetcher: chartFetcher,
	}, true
}

// resolveImageFormat aplica el default "png" cuando --image-format no se
// especificó (input vacío) — único punto de verdad reusado por
// buildInteractiveRenderContext y el log de arriba.
func resolveImageFormat(imageFormat string) string {
	if imageFormat == "" {
		return "png"
	}
	return imageFormat
}

// resolveWebPQuality aplica el default 85 cuando --webp-quality no se
// especificó (0, el zero-value de un flag int no tocado).
func resolveWebPQuality(webpQuality int) int {
	if webpQuality == 0 {
		return 85
	}
	return webpQuality
}

// buildInteractiveRenderContext arma un *renderer.RenderContext con
// fetchers de mermaid/chart/map sobre `chromiumR`, con renderMode/
// image-format/webp-quality tomados de `opts` — única fuente de verdad
// reusada por SetupOfflineRenderContext (modo --format html offline) y
// generatePDF (pdf.go, issue #128), que antes duplicaban byte-a-byte esta
// misma construcción (hallazgo de code-review sobre PR #160). slidelang no
// renderiza PlantUML, así que ese modo queda fijo en "browser".
func buildInteractiveRenderContext(chromiumR *chromium.ChromiumRenderer, outputDir string, opts GeneratorOptions) *renderer.RenderContext {
	imageFormat := resolveImageFormat(opts.ImageFormat)
	webpQuality := resolveWebPQuality(opts.WebPQuality)

	fetcherLog := renderer.NoopFetcherLogger{}
	mermaidFetcher := chromium.NewMermaidFetcher(chromiumR, fetcherLog)
	chartFetcher := chromium.NewChartFetcher(chromiumR, fetcherLog)
	chartFetcher.SetImageFormat(imageFormat, webpQuality)
	mapFetcher := chromium.NewMapFetcher(chromiumR, fetcherLog)
	mapFetcher.SetImageFormat(imageFormat, webpQuality)

	return &renderer.RenderContext{
		MermaidMode:    opts.RenderMode,
		ChartMode:      opts.RenderMode,
		MapMode:        opts.RenderMode,
		PlantUMLMode:   "browser",
		OutputDir:      outputDir,
		MermaidFetcher: mermaidFetcher,
		ChartFetcher:   chartFetcher,
		MapFetcher:     mapFetcher,
	}
}
