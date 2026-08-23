// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"fmt"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/themes/document"
)

// GeneratorOptions define las opciones de generación de documentos
type GeneratorOptions struct {
	Format            string            // html, pdf, docx, markdown
	Theme             string            // Theme name
	ThemeVariables    map[string]string // 🆕 Variables CSS del tema
	ShowHeaders       bool              // 🆕 Mostrar headers (page-view)
	ShowFooters       bool              // 🆕 Mostrar footers (page-view)
	InteractiveViewer bool              // 🆕 Viewer interactivo con sidebar, dark mode
	TOC               bool              // Table of contents
	TOCDepth          int               // TOC depth (1-6)
	Numbering         bool              // Section numbering
	PageBreaks        bool              // Page breaks between sections
	// Page is the parsed `page:` front matter namespace (size/margins,
	// author-facing strings verbatim — see core/util/length.go). nil means
	// the document has no opinion. Consumed only by the PDF generator today
	// (see resolvePDFOptions in page.go) — HTML/Markdown/DOCX don't set
	// page geometry (see the plan's "Fuera de alcance" section for why).
	Page *ast.PageConfig
	// HeaderFooter is the parsed `header:`/`footer:`/`layout_defaults:`
	// front matter namespace (issue #117). nil means the document has no
	// opinion, same convention as Page. Consumed by HTML (renderer.
	// DocumentHTMLOptions.HeaderFooter — core/v2.9.0+), PDF (its own
	// Chromium header/footer template, generated from this same config)
	// and DOCX (native Section.Header/Footer). Markdown has no notion of a
	// page, so it only passes the raw front matter through on round-trip.
	HeaderFooter *ast.HeaderFooterConfig
	// Watermark is the parsed `watermark:` front matter namespace (issue
	// #179). nil means the document has no opinion. Unlike HeaderFooter,
	// it applies uniformly with no per-blockType/layout cascade. Consumed
	// by HTML (renderer.DocumentHTMLOptions.Watermark), PDF (same HTML
	// renderer — a page-drawn overlay, not a Chromium print template, so
	// pdf.go needs no code of its own for it), and DOCX (rasterized via
	// renderer.RenderWatermarkPNG, since docxgo has no w:pict/VML API for
	// a native text watermark). Markdown round-trips it, same as
	// HeaderFooter.
	Watermark *ast.WatermarkConfig
	// PlantUML options
	PlantUMLMode   string // "browser" (default), "offline-assets", "offline-inline"
	PlantUMLServer string // Custom PlantUML server (default: https://www.plantuml.com/plantuml)
	PlantUMLFormat string // "svg" (default) or "png" for offline modes
	// Mermaid options
	MermaidMode string // "browser" (default), "offline-assets", "offline-inline"
	// DiagramBackend selecciona qué motor resuelve mermaid/plantuml en los
	// modos offline: "chromium" (default) o "kroki" — este último no
	// necesita Chromium en esta máquina (ver
	// core/renderer/chromium/kroki_fetcher.go). No afecta chart/map/math.
	DiagramBackend string
	// KrokiServer es la URL base de un servidor Kroki propio (vacío = el
	// público https://kroki.io). Solo aplica cuando DiagramBackend ==
	// "kroki".
	KrokiServer string
	// Chart.js options
	ChartMode string // "browser" (default), "offline-assets", "offline-inline"
	// Leaflet Maps options
	MapMode string // "browser" (default), "offline-assets", "offline-inline"
	// Math (LaTeX/MathJax) options (issue #239-B)
	MathMode string // "browser" (default), "offline-assets", "offline-inline"
	// ImageMode (issue #167): "offline-inline" inlinea fuentes de imagen
	// LOCALES como data: URI, confinadas a AssetRoot. --format pdf lo
	// fuerza incondicionalmente (ver pdf.go) porque su problema — el HTML
	// se inyecta en about:blank, sin base URL — existe pase lo que pase
	// diga este campo; para --format html sí sigue lo que declare
	// --render-mode, igual que ChartMode/MapMode/etc arriba.
	ImageMode string
	// Image format options (for charts and maps in offline modes)
	ImageFormat string // "png" (default) or "webp"
	WebPQuality int    // WebP quality: 1-100 (default: 85)
	// Chromium options (for PDF and offline rendering)
	ChromiumPath    string // Custom path to Chromium/Chrome/Edge
	InstallChromium bool   // Auto-install Chromium if not found
	// AssetRoot confina las fuentes de imagen locales del DOCX a este
	// directorio (ver docs/SECURITY_AUDIT_2026-07.md, AL-4); vacío desactiva
	// la confinación (no usado por los demás formatos).
	AssetRoot string
}

// Generator es el generador principal de documentos
type Generator struct {
	logger util.Logger
}

// New crea una nueva instancia del generador
func New(log util.Logger) *Generator {
	return &Generator{
		logger: log,
	}
}

// Generate genera un documento en el formato especificado
func (g *Generator) Generate(doc *ast.AST, outputFile string, opts GeneratorOptions) error {
	g.logger.Info("GENERATOR", "Generating %s document to %s", opts.Format, outputFile)

	switch opts.Format {
	case "html":
		return g.generateHTML(doc, outputFile, opts)
	case "pdf":
		return g.generatePDF(doc, outputFile, opts)
	case "docx":
		return g.generateDOCX(doc, outputFile, opts)
	case "markdown", "md":
		return g.generateMarkdown(doc, outputFile, opts)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Format)
	}
}

func (g *Generator) generateHTML(doc *ast.AST, outputFile string, opts GeneratorOptions) error {
	g.logger.Info("HTML", "Generating HTML document...")

	gen := NewHTMLGenerator(g.logger)
	return gen.Generate(doc, outputFile, opts)
}

func (g *Generator) generatePDF(doc *ast.AST, outputFile string, opts GeneratorOptions) error {
	g.logger.Info("PDF", "Generating PDF document...")

	gen := NewPDFGenerator(g.logger)
	return gen.Generate(doc, outputFile, opts)
}

func (g *Generator) generateDOCX(doc *ast.AST, outputFile string, opts GeneratorOptions) error {
	g.logger.Info("DOCX", "Generating DOCX document...")

	gen := NewDOCXGenerator(g.logger, opts.AssetRoot)
	return gen.Generate(doc, outputFile, opts)
}

func (g *Generator) generateMarkdown(doc *ast.AST, outputFile string, opts GeneratorOptions) error {
	g.logger.Info("MARKDOWN", "Generating Markdown document...")

	gen := NewMarkdownGenerator(g.logger)
	return gen.Generate(doc, outputFile, opts)
}

// RenderASTJSON serializa doc a JSON compacto — el mismo shape que llevaría
// un futuro `doclang build --format json` — sin escribir ningún archivo.
// Para callers in-process (el servidor MCP, issue #187/#189) que necesitan
// el AST completo, incluyendo los campos *HTML pre-renderizados. Espejo de
// slidelang/internal/generator/generator.go:RenderASTJSON (que delega en
// data.BuildVariables + serializer.SerializeToJSONCompact); acá
// doc.FrontMatter.BuildVariables() y json.Marshal son esos mismos dos pasos
// sin la indirección de un paquete propio, porque doclang no tenía ninguno
// de los dos hasta ahora.
func (g *Generator) RenderASTJSON(doc *ast.AST) ([]byte, error) {
	variables := doc.FrontMatter.BuildVariables()
	renderer.PopulateInlineHTML(doc, variables)
	renderer.PopulateLangRuns(doc, variables) // issue #63 — ver slidelang/internal/generator/generator.go:RenderASTJSON
	return json.Marshal(doc)
}

// RenderHTMLPreview renderiza doc a un HTML autocontenido en memoria, sin
// escribir ningún archivo — para callers in-process (el servidor MCP) que
// necesitan una vista previa. Fuerza modo browser (CDN) para todo elemento
// interactivo (mermaid/chart/map/plantuml/math): no hay Chromium disponible
// en este path, igual que el RenderHTMLPreview de slidelang.
//
// themeName vacío resuelve el theme del propio frontmatter del documento y,
// a falta de eso, a "professional" — el mismo orden de prioridad que
// `doclang build` (cli/build.go:getThemeName), MENOS el flag --theme del
// CLI, que no aplica a un caller in-process. Un themeName inválido no falla
// la llamada: se degrada a "professional" con un warning, igual que hace
// build.go — el theme resuelto acá siempre pasó ya por
// validateThemeName cuando viene de un cliente MCP (ver preview.go, ME-2).
func (g *Generator) RenderHTMLPreview(doc *ast.AST, themeName string) string {
	if themeName == "" && doc.FrontMatter != nil {
		themeName = doc.FrontMatter.Theme
	}
	if themeName == "" {
		themeName = "professional"
	}

	theme, err := document.NewThemeLoader().LoadTheme(themeName, false)
	if err != nil {
		g.logger.Warn("THEME: failed to load theme %q, falling back to professional: %v", themeName, err)
	}

	title := ""
	var headerFooterConfig *ast.HeaderFooterConfig
	var watermarkConfig *ast.WatermarkConfig
	if doc.FrontMatter != nil {
		title = doc.FrontMatter.Title
		headerFooterConfig = doc.FrontMatter.HeaderFooter
		watermarkConfig = doc.FrontMatter.Watermark
	}

	renderOpts := renderer.DocumentHTMLOptions{
		Title:          title,
		Theme:          theme.Name,
		ThemeVariables: theme.Variables,
		ShowHeaders:    theme.Name == "page-view",
		ShowFooters:    theme.Name == "page-view",
		HeaderFooter:   headerFooterConfig, // 🆕 header:/footer:/layout_defaults: (issue #117)
		Watermark:      watermarkConfig,    // watermark: front matter (issue #179)
		EmbedAssets:    true,
		PlantUMLMode:   "browser",
		MermaidMode:    "browser",
		ChartMode:      "browser",
		MapMode:        "browser",
		MathMode:       "browser",
	}

	// Honor an explicitly-declared `toc:` (issue #115 follow-up, PR 3) —
	// only when declared, so a document that says nothing about TOC keeps
	// today's preview default (no TOC) untouched. `doclang build` has its
	// own separate defaulting policy (front matter present ⇒ TOC on by
	// default, see cli/build.go) that deliberately does NOT apply here:
	// this preview path has no user affirmatively asking for a document
	// build, just a live look at the content.
	if doc.FrontMatter != nil && doc.FrontMatter.TOC != nil && doc.FrontMatter.TOC.Enabled != nil {
		renderOpts.TOC = *doc.FrontMatter.TOC.Enabled
		if doc.FrontMatter.TOC.Depth != nil {
			renderOpts.TOCDepth = *doc.FrontMatter.TOC.Depth
		}
	}

	return renderer.GenerateDocumentHTML(doc, renderOpts, renderer.NewDefaultRenderContext())
}
