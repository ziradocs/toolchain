// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package generator

import (
	"context"
	"fmt"
	"path/filepath"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/core/renderer/chromium"
)

// pdf.go implementa --format pdf (issue #128), reusando el mismo pipeline de
// Chromium que doclang/internal/generator/pdf.go: en vez de escribir un
// archivo temporal y navegar a file://, el HTML final se inyecta vía
// about:blank + Page.SetDocumentContent (docs/SECURITY_AUDIT_2026-07.md,
// AL-5) dentro de renderer.RenderHTMLToPDF.
//
// A diferencia de doclang (que nunca propaga --render-mode a su
// DocumentHTMLOptions, dejando mermaid/chart/map en modo browser dentro del
// PDF), acá se fuerza siempre offline-inline: RenderHTMLToPDF le da a la
// página inyectada una sola ventana fija de 500ms
// (chromium_renderer.go:271) para que termine de cargar - probado en vivo
// contra un deck con mermaid, ese margen NO alcanza para que el script CDN
// de mermaid.js cargue + inicialice + dibuje, y el diagrama queda en blanco
// en el PDF. offline-inline pre-renderiza mermaid/chart/map ANTES de armar
// el HTML (mismo pipeline que --render-mode=offline-inline en --format
// html), horneándolos como SVG/data-URI - determinístico, sin carrera contra
// un timeout fijo, y sin depender de CDNs en absoluto para el PDF (mejor que
// el modo browser incluso para decks sin diagramas: nada que la página
// impresa necesite ir a buscar por red). offline-assets no aplica (rutas
// relativas a un directorio assets/ que este pipeline nunca escribe:
// apuntarlo a un documento inyectado sobre about:blank, sin origen de
// archivo, las dejaría rotas en el PDF).
func (g *Generator) generatePDF(astNode *ast.AST, outputDir string, opts GeneratorOptions) error {
	g.logger.Info("PDF", "Building PDF presentation...")

	g.logger.Info("PDF", "Initializing Chromium renderer...")
	logAdapter := &renderer.ChromiumLoggerAdapter{Logger: g.logger}
	chromiumRenderer, err := chromium.NewChromiumRendererWithBrand(context.Background(), opts.ChromiumPath, opts.InstallChromium, logAdapter, slidelangChromiumBrand)
	if err != nil {
		return fmt.Errorf("failed to initialize chromium: %w", err)
	}
	defer chromiumRenderer.Close()

	// El HTML del PDF debe ser autocontenido: no se escriben reset.css/
	// presentation.css/presentation.js a outputDir para este formato, así que
	// EmbedAssets debe forzarse a true sin importar --embed-assets (que
	// controla la salida --format html, un formato independiente en el mismo
	// build).
	pdfOpts := opts
	pdfOpts.EmbedAssets = true
	pdfOpts.RenderMode = "offline-inline"

	// ctx es un valor local, no estado global (issue #134/G1a) — ya no hace
	// falta guardar/restaurar un contexto previo para no pisar la generación
	// HTML del mismo build (build.go's runBuild): cada formato arma y usa el
	// suyo propio, sin compartir nada mutable entre sí.
	ctx := renderer.NewDefaultRenderContext()
	if hasInteractiveElements(astNode) {
		ctx = buildInteractiveRenderContext(chromiumRenderer, outputDir, pdfOpts)
	}

	presentationConfig, err := g.preparePresentationConfig(astNode, outputDir, pdfOpts, ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare presentation config: %w", err)
	}

	htmlTemplate := presentationConfig.Builder.Build()

	finalHTML, err := g.renderHTML(presentationConfig, htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to render HTML for PDF: %w", err)
	}

	g.logger.Info("PDF", "Rendering PDF...")
	outputPath := filepath.Join(outputDir, resolveOutputFilename(astNode, "pdf"))
	if err := chromiumRenderer.RenderHTMLToPDF(context.Background(), finalHTML, outputPath, slidesPDFOptions()); err != nil {
		return fmt.Errorf("PDF rendering failed: %w", err)
	}

	g.logger.Info("PDF", "✅ PDF presentation generated successfully: %s", outputPath)
	return nil
}

// slidesPDFOptions retorna las opciones de PDF para una presentación de
// slides: una página por slide, en vez del flujo de texto continuo de un
// documento (por eso vive en slidelang, no en core — doclang
// nunca necesita este preset de página; CLAUDE.md: "Presentation-only
// features → slidelang/internal/", hallazgo de code-review sobre PR
// #160). PaperWidth/PaperHeight ya vienen en proporción 16:9
// (13.333×7.5in, el tamaño estándar de PowerPoint widescreen) — no se activa
// Landscape, porque ancho>alto ya describe una página apaisada; combinarlo
// con Landscape:true rotaría la página dos veces. Márgenes en cero porque
// cada slide ya trae su propio padding vía CSS y una página exacta al
// tamaño del slide es lo que produce "un slide = una página" al combinarse
// con `@media print { .slidelang-slide { page-break-after: always } }`
// (internal/generator/css/builder.go).
func slidesPDFOptions() chromium.PDFOptions {
	return chromium.PDFOptions{
		PaperWidth:  13.333,
		PaperHeight: 7.5,
		Landscape:   false,
		Scale:       1.0,
	}
}
