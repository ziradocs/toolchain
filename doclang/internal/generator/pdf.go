// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"context"
	"fmt"
	"path/filepath"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/core/renderer/chromium"
	"go.ziradocs.com/core/util"
)

// PDFGenerator genera documentos PDF usando Chromium
type PDFGenerator struct {
	logger util.Logger
}

// NewPDFGenerator crea un nuevo generador PDF
func NewPDFGenerator(log util.Logger) *PDFGenerator {
	return &PDFGenerator{
		logger: log,
	}
}

// Generate genera un documento PDF
// loggerAdapter adapta util.Logger a renderer.ChromiumLogger
type loggerAdapter struct {
	logger util.Logger
}

func (l *loggerAdapter) Info(tag, format string, args ...interface{}) {
	l.logger.Info(tag, format, args...)
}

func (l *loggerAdapter) Warn(tag, format string, args ...interface{}) {
	// util.Logger.Warn solo tiene (message, args...)
	message := fmt.Sprintf("[%s] %s", tag, format)
	l.logger.Warn(message, args...)
}

func (l *loggerAdapter) Error(tag, format string, args ...interface{}) {
	// util.Logger.Error solo tiene (message, args...)
	message := fmt.Sprintf("[%s] %s", tag, format)
	l.logger.Error(message, args...)
}

func (p *PDFGenerator) Generate(doc *ast.AST, outputFile string, opts GeneratorOptions) error {
	p.logger.Info("PDF", "Building PDF document...")

	// 1. Generar HTML primero (con estilos optimizados para impresión)
	p.logger.Info("PDF", "Generating HTML for PDF...")

	title := ""
	if doc.FrontMatter != nil && doc.FrontMatter.Title != "" {
		title = doc.FrontMatter.Title
	}

	outputDir := filepath.Dir(outputFile)

	renderOpts := renderer.DocumentHTMLOptions{
		Title:             title,
		TOC:               opts.TOC,
		TOCDepth:          opts.TOCDepth,
		Numbering:         opts.Numbering,
		PageBreaks:        opts.PageBreaks,
		Theme:             opts.Theme,
		ThemeVariables:    opts.ThemeVariables,
		ShowHeaders:       opts.ShowHeaders,
		ShowFooters:       opts.ShowFooters,
		InteractiveViewer: false, // No viewer en PDF
		EmbedAssets:       true,
		// Image format options (for charts and maps in offline modes)
		ImageFormat: opts.ImageFormat, // 🆕 "png" o "webp"
		WebPQuality: opts.WebPQuality, // 🆕 Calidad WebP (1-100)
	}

	// ctx: PlantUML es el único que puede ir offline acá (mermaid/chart/map
	// nunca se pasan a ChartMode/MermaidMode/MapMode, quedan "browser" dentro
	// del HTML que después se imprime a PDF) — mismo comportamiento que antes
	// del split (issue #134/G1b), solo explícito ahora. PlantUML no depende
	// de Chromium, así que cr=nil es válido acá (el ChromiumRenderer real se
	// crea más abajo, solo para el paso HTML->PDF).
	ctx := chromium.NewRenderContext(nil, chromium.RenderContextOptions{
		PlantUMLMode:   opts.PlantUMLMode,
		PlantUMLServer: opts.PlantUMLServer,
		PlantUMLFormat: opts.PlantUMLFormat,
		OutputDir:      outputDir,
		Logger:         p.logger,
	})

	htmlContent := renderer.GenerateDocumentHTML(doc, renderOpts, ctx)

	// 2. Crear ChromiumRenderer
	p.logger.Info("PDF", "Initializing Chromium renderer...")

	// Adaptar logger
	adapter := &loggerAdapter{logger: p.logger}

	chromiumRenderer, err := chromium.NewChromiumRenderer(
		context.Background(),
		opts.ChromiumPath,
		opts.InstallChromium,
		adapter,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize chromium: %w", err)
	}
	defer chromiumRenderer.Close()

	// 3. Convertir HTML a PDF
	pdfOpts := chromium.DefaultPDFOptions()
	pdfOpts.DisplayHeaderFooter = opts.ShowHeaders || opts.ShowFooters

	if opts.ShowHeaders && opts.ShowFooters {
		// Templates simples para header/footer
		pdfOpts.HeaderTemplate = `<div style="font-size:10px; text-align:center; width:100%;">` + title + `</div>`
		pdfOpts.FooterTemplate = `<div style="font-size:10px; text-align:center; width:100%;"><span class="pageNumber"></span> / <span class="totalPages"></span></div>`
	}

	if err := chromiumRenderer.RenderHTMLToPDF(context.Background(), htmlContent, outputFile, pdfOpts); err != nil {
		return fmt.Errorf("PDF rendering failed: %w", err)
	}

	p.logger.Info("PDF", "✅ PDF document generated successfully: %s", outputFile)
	return nil
}
