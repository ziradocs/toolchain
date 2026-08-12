// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
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

	// variables alimenta la sustitución {{var}} de las zonas de texto del
	// header/footer de Chromium (buildPDFHeaderTemplate/buildPDFFooterTemplate) —
	// el mismo BuildVariables que usa GenerateDocumentHTML internamente.
	variables := doc.FrontMatter.BuildVariables()
	if variables == nil {
		variables = make(map[string]interface{})
	}

	outputDir := filepath.Dir(outputFile)

	// ShowHeaders/ShowFooters/HeaderFooter se dejan en su default (false/
	// nil) a propósito: el chrome de un PDF lo pone Chromium vía
	// pdfOpts.HeaderTemplate/FooterTemplate más abajo, no los
	// <div class="page-header">/<div class="page-footer"> que
	// generatePageViewBody hornearía dentro del HTML impreso — pedirle
	// ambos a la vez producía un header/footer duplicado (issue #117).
	renderOpts := renderer.DocumentHTMLOptions{
		Title:             title,
		TOC:               opts.TOC,
		TOCDepth:          opts.TOCDepth,
		Numbering:         opts.Numbering,
		PageBreaks:        opts.PageBreaks,
		Theme:             opts.Theme,
		ThemeVariables:    opts.ThemeVariables,
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
	pdfOpts := resolvePDFOptions(opts.Page, p.logger)

	// header/footer resueltos desde el front matter (issue #117): su propio
	// Enabled gana sobre el gate de tema, igual que en HTML (ver
	// headerFooterConfigured en core/renderer/document_html.go) — sin
	// config, cae al fallback legado gateado por ShowHeaders/ShowFooters.
	// A diferencia de HTML, Chromium's print header/footer es GLOBAL para
	// todo el PDF (no hay paginación real todavía cuando se arma el
	// template), así que layout_defaults nunca aplica acá — limitación de
	// la plataforma, no un recorte de este PR.
	var headerCfg *ast.HeaderConfig
	var footerCfg *ast.FooterConfig
	if opts.HeaderFooter != nil {
		headerCfg = opts.HeaderFooter.Header
		footerCfg = opts.HeaderFooter.Footer
	}

	showHeader := opts.ShowHeaders
	if headerCfg != nil {
		showHeader = headerCfg.Enabled
	}
	showFooter := opts.ShowFooters
	if footerCfg != nil {
		showFooter = footerCfg.Enabled
	}

	pdfOpts.DisplayHeaderFooter = showHeader || showFooter
	if pdfOpts.DisplayHeaderFooter {
		// Los dos lados SIEMPRE se asignan de forma explícita, nunca se
		// dejan en "" — verificado empíricamente (Edge/Chromium 151, no
		// solo por lectura de la doc de CDP): con DisplayHeaderFooter=true,
		// un HeaderTemplate/FooterTemplate vacío NO produce un lado en
		// blanco, produce el template default de Chromium (fecha + título
		// a la izquierda/derecha) — exactamente el "chrome fantasma" que
		// este fix existe para evitar. pdfBlankChromeTemplate es HTML no
		// vacío que no imprime nada visible, así que activar solo el
		// footer (o un header/footer habilitado pero sin texto/zonas) no
		// hace que Chromium rellene el otro lado con su propio default.
		pdfOpts.HeaderTemplate = pdfBlankChromeTemplate
		if showHeader {
			pdfOpts.HeaderTemplate = buildPDFHeaderTemplate(headerCfg, title, variables)
		}
		pdfOpts.FooterTemplate = pdfBlankChromeTemplate
		if showFooter {
			pdfOpts.FooterTemplate = buildPDFFooterTemplate(footerCfg, variables)
		}
	}

	if err := chromiumRenderer.RenderHTMLToPDF(context.Background(), htmlContent, outputFile, pdfOpts); err != nil {
		return fmt.Errorf("PDF rendering failed: %w", err)
	}

	p.logger.Info("PDF", "✅ PDF document generated successfully: %s", outputFile)
	return nil
}

// pdfChromeStyle es el estilo del <div> raíz de un header/footer de
// Chromium: tres zonas repartidas con flex (mismo criterio visual que las
// tres zonas de HTML/page-view, ver core/renderer/document_html.go).
const pdfChromeStyle = `font-size:10px; width:100%; padding:0 10mm; box-sizing:border-box; display:flex; justify-content:space-between;`

// pdfBlankChromeTemplate es el HeaderTemplate/FooterTemplate que se usa
// cuando ese lado no debe imprimir nada VISIBLE, pero DisplayHeaderFooter
// sigue en true porque el otro lado sí tiene contenido. Verificado
// empíricamente (Edge/Chromium 151, no solo por lectura de la doc de CDP):
// con DisplayHeaderFooter=true, un HeaderTemplate/FooterTemplate vacío
// ("") NO produce un lado en blanco — produce el template DEFAULT de
// Chromium (fecha + título a la izquierda/derecha), el mismo "chrome
// fantasma" que este fix existe para evitar. Cualquier HTML no vacío que
// no imprima nada visible sirve; un <span> vacío es el más barato.
const pdfBlankChromeTemplate = `<span></span>`

// buildPDFHeaderTemplate arma el HeaderTemplate que Chromium usa para
// imprimir el header de cada página del PDF. header == nil (nunca hubo
// opts.HeaderFooter.Header) cae al fallback legado: título centrado, sin
// zonas — el output previo a issue #117 (que antes concatenaba el título
// SIN escapar; ver EscapeHTML acá). Nunca retorna "" — solo se llama
// cuando el header SÍ está habilitado (ver Generate), así que un
// resultado vacío de pdfChromeZonesHTML (sin texto ni logo) cae a
// pdfBlankChromeTemplate en vez de dejar que Chromium rellene con su
// propio default.
func buildPDFHeaderTemplate(header *ast.HeaderConfig, docTitle string, variables map[string]interface{}) string {
	if header == nil {
		return fmt.Sprintf(`<div style="font-size:10px; text-align:center; width:100%%;">%s</div>`, renderer.EscapeHTML(docTitle))
	}
	if html := pdfChromeZonesHTML(header.Text, nil, variables); html != "" {
		return html
	}
	return pdfBlankChromeTemplate
}

// buildPDFFooterTemplate es el análogo de buildPDFHeaderTemplate para el
// footer, sumando la numeración de página vía las clases especiales que
// Chromium reconoce (pageNumber/totalPages) — ver pdfPageNumberHTML.
func buildPDFFooterTemplate(footer *ast.FooterConfig, variables map[string]interface{}) string {
	if footer == nil {
		return `<div style="font-size:10px; text-align:center; width:100%;"><span class="pageNumber"></span> / <span class="totalPages"></span></div>`
	}
	if html := pdfChromeZonesHTML(footer.Text, footer.PageNumbers, variables); html != "" {
		return html
	}
	return pdfBlankChromeTemplate
}

// pdfChromeZonesHTML arma las tres zonas (left/center/right) de un header o
// footer de Chromium. pageNumbers, si no es nil y está Enabled, se agrega a
// la zona que indique su Position (default "right"). "" (sin <div> alguno)
// si las tres zonas terminan vacías — los callers (buildPDFHeaderTemplate/
// buildPDFFooterTemplate) son responsables de no dejar pasar ese "" hasta
// PDFOptions cuando DisplayHeaderFooter es true (ver pdfBlankChromeTemplate).
//
// StartFrom/ExcludeTitleSlides/ExcludeClosingSlides de PageNumbersConfig NO
// se evalúan acá: Chromium arma este template UNA vez para todo el PDF,
// antes de que exista ninguna noción de "qué bloque abre esta página" — a
// diferencia de HTML/page-view (ver resolvePageNumberDisplay en
// core/renderer/document_html.go), no hay forma de variar el header/footer
// por página desde este mecanismo. Limitación de la plataforma (Chromium
// print API), documentada en el alcance de doclang, no un recorte de este
// PR.
func pdfChromeZonesHTML(text *ast.HeaderFooterText, pageNumbers *ast.PageNumbersConfig, variables map[string]interface{}) string {
	left, center, right := "", "", ""
	if text != nil {
		left = renderer.EscapeHTML(renderer.ProcessVariables(text.Left, variables))
		center = renderer.EscapeHTML(renderer.ProcessVariables(text.Center, variables))
		right = renderer.EscapeHTML(renderer.ProcessVariables(text.Right, variables))
	}

	if pageNumbers != nil && pageNumbers.Enabled {
		numHTML := pdfPageNumberHTML(pageNumbers)
		switch pageNumbers.Position {
		case "left":
			left = joinNonEmpty(left, numHTML)
		case "center":
			center = joinNonEmpty(center, numHTML)
		default:
			right = joinNonEmpty(right, numHTML)
		}
	}

	if left == "" && center == "" && right == "" {
		return ""
	}
	return fmt.Sprintf(`<div style="%s"><span>%s</span><span>%s</span><span>%s</span></div>`, pdfChromeStyle, left, center, right)
}

// pdfPageNumberHTML traduce PageNumbersConfig.Format a HTML que Chromium
// puede imprimir: "{{current}}"/"{{total}}" se reemplazan por las clases
// especiales que Chromium reconoce y rellena en tiempo de impresión
// (pageNumber/totalPages) — NO por ProcessVariables, que resuelve
// variables del documento, no las de Chromium. El resto del formato
// (separadores como " / ") se escapa antes del reemplazo: como ninguno de
// los dos tokens contiene caracteres que EscapeHTML toque, escapar primero
// y reemplazar después no los rompe, y sí cierra la puerta a un Format con
// HTML arbitrario en el front matter.
func pdfPageNumberHTML(config *ast.PageNumbersConfig) string {
	format := config.Format
	if format == "" {
		format = "{{current}} / {{total}}"
	}
	html := renderer.EscapeHTML(format)
	html = strings.ReplaceAll(html, "{{current}}", `<span class="pageNumber"></span>`)
	html = strings.ReplaceAll(html, "{{total}}", `<span class="totalPages"></span>`)
	switch config.Style {
	case "bold":
		html = "<b>" + html + "</b>"
	case "caption":
		html = "<i>" + html + "</i>"
	}
	return html
}

// joinNonEmpty concatena dos fragmentos de HTML con un espacio separador,
// sin dejar un espacio colgante cuando alguno está vacío.
func joinNonEmpty(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " " + b
}
