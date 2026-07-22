// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
)

// HTMLGenerator genera documentos HTML
type HTMLGenerator struct {
	logger util.Logger
}

// NewHTMLGenerator crea un nuevo generador HTML
func NewHTMLGenerator(log util.Logger) *HTMLGenerator {
	return &HTMLGenerator{
		logger: log,
	}
}

// Generate genera un documento HTML usando el renderer compartido de core
func (h *HTMLGenerator) Generate(doc *ast.AST, outputFile string, opts GeneratorOptions) error {
	h.logger.Info("HTML", "Building HTML document...")

	// Extraer título del frontmatter o usar opciones
	title := ""
	if doc.FrontMatter != nil && doc.FrontMatter.Title != "" {
		title = doc.FrontMatter.Title
	}

	// Determinar outputDir (directorio donde se guarda el HTML)
	outputDir := filepath.Dir(outputFile)

	// 🆕 Inicializar Chromium renderer si algún modo offline está habilitado
	var chromiumRenderer *chromium.ChromiumRenderer

	needsChromium := (opts.MermaidMode == "offline-assets" || opts.MermaidMode == "offline-inline") ||
		(opts.ChartMode == "offline-assets" || opts.ChartMode == "offline-inline") ||
		(opts.MapMode == "offline-assets" || opts.MapMode == "offline-inline") ||
		(opts.MathMode == "offline-assets" || opts.MathMode == "offline-inline")

	if needsChromium {
		h.logger.Info("HTML", "Initializing Chromium renderer for offline mode...")

		// Crear adaptador de logger
		logAdapter := &renderer.ChromiumLoggerAdapter{Logger: h.logger}

		// Inicializar ChromiumRenderer
		chromiumR, err := chromium.NewChromiumRenderer(context.Background(), opts.ChromiumPath, opts.InstallChromium, logAdapter)
		if err != nil {
			return fmt.Errorf("failed to initialize Chromium renderer: %w", err)
		}
		defer chromiumR.Close()

		chromiumRenderer = chromiumR

		h.logger.Info("HTML", "✅ Offline rendering enabled (Mermaid: %s, Charts: %s, Maps: %s)",
			opts.MermaidMode, opts.ChartMode, opts.MapMode)
	}

	// Configurar opciones de renderizado usando el renderer del core
	renderOpts := renderer.DocumentHTMLOptions{
		Title:             title,
		TOC:               opts.TOC,
		TOCDepth:          opts.TOCDepth,
		Numbering:         opts.Numbering,
		PageBreaks:        opts.PageBreaks,
		Theme:             opts.Theme,
		ThemeVariables:    opts.ThemeVariables,    // 🆕 Pasar variables del tema
		ShowHeaders:       opts.ShowHeaders,       // 🆕 Para page-view
		ShowFooters:       opts.ShowFooters,       // 🆕 Para page-view
		InteractiveViewer: opts.InteractiveViewer, // 🆕 Viewer interactivo
		EmbedAssets:       true,
		// PlantUML/Mermaid/Chart/Map modes: DocumentHTMLOptions.GenerateDocumentHTML
		// no las usa para construir fetchers (eso es ctx, abajo — issue #134/G1b),
		// pero generateDocumentScripts (document_html.go) sigue leyendo estos
		// mismos campos de opts, independientemente de ctx, para decidir si emitir
		// los <script>/<link> CDN de mermaid/chart.js/leaflet — dejarlos en "" acá
		// haría que un render offline-inline/offline-assets igual cargara los CDN.
		PlantUMLMode:   opts.PlantUMLMode,
		PlantUMLServer: opts.PlantUMLServer,
		PlantUMLFormat: opts.PlantUMLFormat,
		MermaidMode:    opts.MermaidMode,
		ChartMode:      opts.ChartMode,
		MapMode:        opts.MapMode,
		MathMode:       opts.MathMode,
		// Image format options (for charts and maps in offline modes)
		ImageFormat: opts.ImageFormat, // 🆕 "png" o "webp"
		WebPQuality: opts.WebPQuality, // 🆕 Calidad WebP (1-100)
	}

	// ctx controla el rendering offline de mermaid/chart/map/plantuml (issue
	// #92); se arma explícitamente acá (issue #134/G1b) en vez de que
	// GenerateDocumentHTML lo construya internamente a partir de
	// ChromiumRenderer — ver renderer/chromium/render_context.go.
	ctx := chromium.NewRenderContext(chromiumRenderer, chromium.RenderContextOptions{
		PlantUMLMode:   opts.PlantUMLMode,
		PlantUMLServer: opts.PlantUMLServer,
		PlantUMLFormat: opts.PlantUMLFormat,
		MermaidMode:    opts.MermaidMode,
		ChartMode:      opts.ChartMode,
		MapMode:        opts.MapMode,
		MathMode:       opts.MathMode,
		OutputDir:      outputDir,
		ImageFormat:    opts.ImageFormat,
		WebPQuality:    opts.WebPQuality,
		Logger:         h.logger,
	})

	// Generar HTML usando el renderer compartido
	html := renderer.GenerateDocumentHTML(doc, renderOpts, ctx)

	// Write to file
	if err := os.WriteFile(outputFile, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}

	h.logger.Info("HTML", "HTML document generated successfully: %s", outputFile)
	return nil
}
