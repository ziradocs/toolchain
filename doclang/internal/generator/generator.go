// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"context"
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
	ThemeExternal     bool              // El tema viene de un JSON externo (DOCX deriva su estilo de ThemeVariables)
	ShowHeaders       bool              // 🆕 Mostrar headers (page-view)
	ShowFooters       bool              // 🆕 Mostrar footers (page-view)
	InteractiveViewer bool              // 🆕 Viewer interactivo con sidebar, dark mode
	TOC               bool              // Table of contents
	TOCDepth          int               // TOC depth (1-6)
	Numbering         bool              // Section numbering
	PageBreaks        bool              // Page breaks between sections
	// Title es el título de respaldo para los metadatos (<title> del HTML,
	// título del PDF) de un documento SIN frontmatter — lo resuelve
	// DocumentTitle (primer `#`, o nombre del archivo). Con frontmatter
	// DocumentTitle devuelve su `title:`, así que este campo nunca le gana.
	// Vacío = sin respaldo.
	Title string
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
	// AllowRemoteImages deja que el PDF y el HTML offline muestren las
	// imágenes http(s) del documento (--allow-remote-images, opción
	// allowRemoteImages del export del MCP). Por defecto no se piden: el PDF
	// se imprime con Chromium en esta máquina, y un documento no confiable
	// le hacía pedir cualquier URL. Ver renderer.RenderContext.AllowRemoteImages.
	// El DOCX nunca descarga imágenes, con o sin esto.
	AllowRemoteImages bool
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
	// SourceDir es el directorio del archivo fuente. Una fuente de imagen
	// local relativa se resuelve contra él (como en cualquier herramienta de
	// markdown) y después se confina a AssetRoot. Vacío resuelve contra
	// AssetRoot, el comportamiento previo a este campo; solo cambia algo
	// cuando AssetRoot es un ancestro del archivo fuente (--asset-root ..).
	// Lo consumen el DOCX y, vía renderer.RenderContext.AssetBaseDir, el
	// inlineo de HTML offline-inline y PDF.
	SourceDir string
	// Ctx acota el trabajo con Chromium (arranque, capturas, PDF): al
	// cancelarse, el navegador se cierra y las llamadas pendientes vuelven
	// con error. nil equivale a context.Background().
	Ctx context.Context
	// FailOnUnavailable (--fail-on-unavailable) hace que la generación
	// falle con un *UnavailableError, sin escribir la salida, si algo
	// quedaría como marcador de no disponible: una imagen que falta, queda
	// fuera de AssetRoot o es remota y no se pidió, o un diagrama, chart,
	// mapa, ecuación o medio que no se pudo dibujar. Cubre DOCX, PDF y HTML
	// (en el HTML de modo browser casi nada se resuelve al compilar, así que
	// casi nada puede faltar). Apagado, el default, el marcador sale como
	// siempre y la generación termina bien.
	FailOnUnavailable bool
	// NoNetwork (--no-network): ningún pedido de red durante la generación.
	// PlantUML (servidor de PlantUML o Kroki), mermaid por Kroki y los mapas
	// quedan como marcador de no disponible en vez de pedirse, y Chromium,
	// si hace falta, arranca sin salida a la red (RestrictNetwork de core).
	// No cambia lo que carga el HTML de modo browser al abrirse en el
	// navegador del lector.
	NoNetwork bool
	// TileServer y TileUserAgent (--tile-server, --tile-user-agent): un
	// servidor de tiles propio para los mapas que se dibujan al compilar
	// (DOCX, PDF y HTML offline), en vez de OpenStreetMap y ArcGIS. Ver
	// renderer.MapConfig.WithTileServer.
	TileServer    string
	TileUserAgent string
	// TileAttribution (--tile-attribution) es el crédito que se dibuja en
	// el mapa cuando los tiles salen de TileServer.
	TileAttribution string
}

// runContext devuelve opts.Ctx, o context.Background() si no se fijó.
func (opts GeneratorOptions) runContext() context.Context {
	if opts.Ctx != nil {
		return opts.Ctx
	}
	return context.Background()
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

	// Los encabezados tipados (typed-headings-v1) se bajan a su forma legada
	// antes de cualquier generador: HTML, PDF, TOC, numeración, Markdown y
	// DOCX consumen solo esa forma, y renderer.HeadingHTML garantiza que es
	// idéntica a la de un documento sin el opt-in. El AST del caller no se
	// modifica.
	doc = renderer.LowerTypedHeadings(doc)

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
	gen.sourceDir = opts.SourceDir
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
	// El código es literal (issue #487): el contentHTML de un bloque de código
	// es su content escapado, igual que el HTML del build.
	literal := renderer.InlineHTMLOptions{LiteralCode: true}
	// El título de una sección se escribe escapado, sin Markdown, en el <h1>,
	// el TOC y el anchor; su titleHTML tiene que decir lo mismo. Por interfaz
	// para compilar contra un core publicado sin la opción (que ya lo deja
	// como texto plano).
	if plain, ok := any(&literal).(interface{ SetPlainBlockTitles(bool) }); ok {
		plain.SetPlainBlockTitles(true)
	}
	// La prosa de un ":::bloque" (su contentHTML y el de la prosa entre sus
	// elementos) como la dibuja el HTML: listas numeradas y anidadas, y un <p>
	// por párrafo. Por interfaz, por el mismo motivo; un core sin la opción
	// deja el contentHTML de antes.
	if prose, ok := any(&literal).(interface{ SetBlockProse(bool) }); ok {
		prose.SetBlockProse(true)
	}
	renderer.PopulateInlineHTMLWithOptions(doc, variables, literal)
	renderer.PopulateLangRunsWithOptions(doc, variables, literal) // issue #63 — ver slidelang/internal/generator/generator.go:RenderASTJSON
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
// `doclang build` (pipeline.ThemeName), MENOS el flag --theme del
// CLI, que no aplica a un caller in-process. Un themeName inválido no falla
// la llamada: se degrada a "professional" con un warning, igual que hace
// build.go — el theme resuelto acá siempre pasó ya por
// validateThemeName cuando viene de un cliente MCP (ver preview.go, ME-2).
func (g *Generator) RenderHTMLPreview(doc *ast.AST, themeName string) string {
	doc = renderer.LowerTypedHeadings(doc) // ver Generate
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

	// Sin ruta de archivo: una vista previa en memoria solo puede tomar el
	// título del frontmatter o del primer `#` (ver DocumentTitle).
	title := DocumentTitle(doc, "")
	var headerFooterConfig *ast.HeaderFooterConfig
	var watermarkConfig *ast.WatermarkConfig
	if doc.FrontMatter != nil {
		headerFooterConfig = doc.FrontMatter.HeaderFooter
		watermarkConfig = doc.FrontMatter.Watermark
	}

	renderOpts := renderer.DocumentHTMLOptions{
		Title:          title,
		Theme:          theme.Name,
		ThemeVariables: htmlThemeVariables(theme.Variables, theme.IsExternal),
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

	ctx := renderer.NewDefaultRenderContext()
	ctx.LiteralCode = true // issue #487, como en el build
	return renderer.GenerateDocumentHTML(doc, renderOpts, ctx)
}

// remoteImagesSetter es RenderContext.SetAllowRemoteImages de core. Se pide
// por interfaz porque este módulo compila también contra el core publicado,
// que puede no tenerlo; ese core no bloquea las imágenes remotas, así que
// ahí no hay nada que permitir.
type remoteImagesSetter interface{ SetAllowRemoteImages(bool) }

// allowRemoteImages aplica GeneratorOptions.AllowRemoteImages al contexto de
// render (ver renderer.RenderContext.AllowRemoteImages).
func allowRemoteImages(ctx any, allow bool) {
	if s, ok := ctx.(remoteImagesSetter); ok {
		s.SetAllowRemoteImages(allow)
	}
}
