// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/util"
)

// DocumentHTMLOptions configura la generación del documento HTML
type DocumentHTMLOptions struct {
	Title             string
	TOC               bool
	TOCDepth          int
	Numbering         bool
	PageBreaks        bool
	Theme             string
	ThemeVariables    map[string]string // 🆕 Variables CSS del tema
	ShowHeaders       bool              // 🆕 Mostrar headers (para page-view)
	ShowFooters       bool              // 🆕 Mostrar footers con numeración (para page-view)
	InteractiveViewer bool              // 🆕 Viewer interactivo con sidebar, dark mode, etc.
	EmbedAssets       bool
	CustomCSS         string
	CustomJS          string
	// PlantUML options
	PlantUMLMode   string // "browser", "offline-assets", "offline-inline"
	PlantUMLServer string // Custom PlantUML server URL
	PlantUMLFormat string // "svg" or "png"
	// Mermaid options
	MermaidMode string // "browser", "offline-assets", "offline-inline"
	// Chart.js options
	ChartMode string // "browser", "offline-assets", "offline-inline"
	// Leaflet Maps options
	MapMode string // "browser", "offline-assets", "offline-inline"
	// Math (LaTeX/MathJax) options (issue #239-B)
	MathMode string // "browser", "offline-assets", "offline-inline"
	// Image format options (for charts and maps in offline modes)
	ImageFormat string // "png" or "webp" (default: "png")
	WebPQuality int    // WebP quality: 1-100 (default: 85)
}

// GenerateDocumentHTML genera un documento HTML completo desde un AST. Usa
// el mismo renderer de elementos que SlideLang para mantener consistencia.
//
// ctx controla el rendering offline de mermaid/chart/map/plantuml (issue
// #92) — el caller lo arma explícitamente (issue #134/G1b, mismo patrón que
// RenderElementToHTML desde G1a) en vez de que este método construya sus
// propios fetchers a partir de un *ChromiumRenderer: eso movería una
// dependencia de renderer/chromium hacia este paquete puro. Un ctx nil
// (o con los fetchers en nil) degrada a los modos "browser" de cada
// elemento vía resolveRenderContext.
func GenerateDocumentHTML(doc *ast.AST, opts DocumentHTMLOptions, ctx *RenderContext) string {
	ctx = resolveRenderContext(ctx)

	// Nonce único para este documento: autoriza en la CSP tanto el <meta>
	// emitido por generateDocumentHeader como cada <style>/<script> inline
	// que este árbol de funciones escribe (ver
	// docs/SECURITY_AUDIT_2026-07.md, BA-10). Si la generación falla (solo
	// posible por una fuente de entropía rota del sistema), se degrada a
	// servir sin CSP en vez de romper el build.
	cspNonce, nonceErr := GenerateCSPNonce()
	if nonceErr != nil {
		ctx.Logger.Debug("CSP", "failed to generate nonce, omitting CSP: %v", nonceErr)
		cspNonce = ""
	}

	var html strings.Builder

	// Variables del frontmatter, incluyendo los built-ins title/author/date/
	// theme (issue #81 — antes solo se exponían las variables personalizadas
	// del usuario, así que `{{title}}` no se sustituía en doclang aunque sí
	// funcionara en slidelang).
	variables := doc.FrontMatter.BuildVariables()
	if variables == nil {
		variables = make(map[string]interface{})
	}

	// Header HTML
	html.WriteString(generateDocumentHeader(doc, opts, cspNonce, ctx.Logger))

	// Interactive Viewer: Sidebar con TOC
	if opts.InteractiveViewer && opts.TOC {
		html.WriteString(generateViewerSidebar(doc, opts, variables))
	}

	// Wrapper para contenido principal (necesario para layout con sidebar)
	if opts.InteractiveViewer {
		html.WriteString(`<main class="doclang-content">
    <div class="reading-progress"></div>
`)
	}

	// Table of Contents estático (solo si NO hay viewer interactivo)
	if opts.TOC && !opts.InteractiveViewer {
		html.WriteString(generateDocumentTOC(doc, opts, variables))
	}

	// Document Body
	html.WriteString(generateDocumentBody(doc, opts, variables, ctx))

	// Cerrar wrapper de contenido principal
	if opts.InteractiveViewer {
		html.WriteString(`</main>
`)
	}

	// Init Scripts (before closing body)
	html.WriteString(generateInitScripts(opts, cspNonce))

	// Viewer Scripts (si está habilitado)
	if opts.InteractiveViewer {
		html.WriteString(generateViewerScripts(opts, cspNonce))
	}

	// Footer HTML
	html.WriteString(generateDocumentFooter())

	return html.String()
}

// generateDocumentHeader genera el header HTML con estilos. cspNonce, si no
// está vacío, se emite como <meta http-equiv="Content-Security-Policy"> y
// debe ser el mismo nonce usado para cada <style>/<script> inline del
// documento (ver GenerateDocumentHTML).
func generateDocumentHeader(doc *ast.AST, opts DocumentHTMLOptions, cspNonce string, logger util.Logger) string {
	title := opts.Title
	if title == "" && doc.FrontMatter != nil && doc.FrontMatter.Title != "" {
		title = doc.FrontMatter.Title
	}
	if title == "" {
		title = "Document"
	}

	// Add classes based on options
	bodyClass := "doclang-document"
	if opts.ShowHeaders || opts.ShowFooters {
		bodyClass += " page-view-mode"
	}
	if opts.InteractiveViewer {
		bodyClass += " doclang-viewer"
	}

	cspMeta := ""
	if cspNonce != "" {
		cspMeta = fmt.Sprintf("    <meta http-equiv=\"Content-Security-Policy\" content=\"%s\">\n", BuildDefaultOutputCSP(cspNonce))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="es" data-theme="light">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
%s    <title>%s</title>
    %s
    %s
</head>
<body class="%s">
`, cspMeta, EscapeHTML(title), generateDocumentStyles(opts, logger), generateDocumentScripts(opts), bodyClass)
}

// generateDocumentStyles genera los estilos CSS del documento. No recibe
// nonce: style-src usa 'unsafe-inline' (ver BuildDefaultOutputCSP) porque
// Mermaid inyecta su CSS de tema en runtime vía un <style> sin nonce y sin
// forma de asignarle uno.
func generateDocumentStyles(opts DocumentHTMLOptions, logger util.Logger) string {
	var css strings.Builder

	// Generar variables CSS del tema si están disponibles
	if len(opts.ThemeVariables) > 0 {
		css.WriteString("    <style>\n")
		css.WriteString(generateThemeVariables(opts.ThemeVariables, logger))
		css.WriteString("    </style>\n")
	}

	css.WriteString("    <style>\n")
	css.WriteString(`        /* Reset and Base Styles */
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body.doclang-document {
            font-family: var(--doclang-font-main, 'Segoe UI', -apple-system, BlinkMacSystemFont, sans-serif);
            line-height: var(--doclang-line-height, 1.7);
            color: var(--doclang-text-color, #2c3e50);
            max-width: var(--doclang-page-max-width, 210mm);
            margin: var(--doclang-page-margin, 0 auto);
            padding: var(--doclang-page-padding, 20mm);
            background: var(--doclang-page-bg, #fff);
        }

        /* Typography */
        h1 {
            font-size: var(--doclang-h1-size, 2.5em);
            margin: 1.5em 0 0.8em 0;
            color: var(--doclang-h1-color, #1a202c);
            border-bottom: var(--doclang-h1-border, 3px solid) var(--doclang-h1-border-color, #3498db);
            padding-bottom: 0.4em;
            font-weight: var(--doclang-h1-weight, 700);
            font-family: var(--doclang-font-heading, inherit);
        }

        h2 {
            font-size: var(--doclang-h2-size, 2em);
            margin: 1.3em 0 0.6em 0;
            color: var(--doclang-h2-color, #2d3748);
            font-weight: 600;
            font-family: var(--doclang-font-heading, inherit);
        }

        h3 {
            font-size: var(--doclang-h3-size, 1.5em);
            margin: 1.1em 0 0.5em 0;
            color: var(--doclang-h3-color, #4a5568);
            font-weight: 600;
            font-family: var(--doclang-font-heading, inherit);
        }

        h4 {
            font-size: var(--doclang-h4-size, 1.2em);
            margin: 1em 0 0.4em 0;
            color: var(--doclang-h4-color, #718096);
            font-weight: 600;
            font-family: var(--doclang-font-heading, inherit);
        }

        p {
            margin: 0.8em 0;
            text-align: justify;
        }

        /* Lists */
        ul, ol {
            margin: 1em 0;
            padding-left: 2.5em;
        }

        li {
            margin: 0.5em 0;
            line-height: 1.8;
        }

        ul ul, ol ul, ul ol, ol ol {
            margin: 0.3em 0;
        }

        /* Code */
        code {
            background: var(--doclang-code-inline-bg, #f6f8fa);
            padding: 2px 6px;
            border-radius: var(--doclang-border-radius, 3px);
            font-family: var(--doclang-code-font, 'Consolas', 'Monaco', 'Courier New', monospace);
            font-size: 0.9em;
            color: var(--doclang-code-inline-color, #e83e8c);
            border: 1px solid var(--doclang-code-border, #e1e4e8);
        }

        pre {
            background: var(--doclang-code-bg, #1e1e1e);
            color: var(--doclang-code-color, #d4d4d4);
            padding: 1.2em;
            border-radius: var(--doclang-border-radius, 6px);
            overflow-x: auto;
            margin: 1.5em 0;
            line-height: 1.5;
            border: 1px solid #333;
        }

        pre code {
            background: none;
            padding: 0;
            color: inherit;
            border: none;
            font-size: 0.95em;
        }

        /* Tables */
        table {
            width: 100%;
            border-collapse: collapse;
            margin: 1.5em 0;
            background: white;
            box-shadow: var(--doclang-shadow-sm, 0 1px 3px rgba(0,0,0,0.1));
        }

        table th,
        table td {
            border: 1px solid var(--doclang-table-border, #e2e8f0);
            padding: 0.9em 1em;
            text-align: left;
        }

        table th {
            background: var(--doclang-table-header-bg, linear-gradient(135deg, #667eea 0%, #764ba2 100%));
            color: var(--doclang-table-header-color, white);
            font-weight: 600;
            text-transform: uppercase;
            font-size: 0.85em;
            letter-spacing: 0.5px;
        }

        table tr:nth-child(even) {
            background: var(--doclang-table-stripe-bg, #f8fafc);
        }

        table tr:hover {
            background: var(--doclang-table-hover-bg, #edf2f7);
            transition: background 0.2s ease;
        }

        .table-caption {
            text-align: center;
            font-style: italic;
            color: #718096;
            margin-top: 0.5em;
            font-size: 0.9em;
        }

        /* Blockquotes */
        blockquote {
            border-left: 4px solid #3498db;
            padding: 1em 1.5em;
            margin: 1.5em 0;
            background: #f7fafc;
            font-style: italic;
            color: #4a5568;
            border-radius: 0 4px 4px 0;
        }

        blockquote p {
            margin: 0.3em 0;
        }

        blockquote footer {
            margin-top: 0.8em;
            font-style: normal;
            font-size: 0.9em;
            color: #718096;
        }

        blockquote cite {
            font-style: normal;
        }

        /* Images */
        img {
            max-width: 100%;
            height: auto;
            display: block;
            margin: 1.5em auto;
            border-radius: 4px;
        }

        figure {
            margin: 1.5em 0;
            text-align: center;
        }

        figcaption {
            margin-top: 0.8em;
            font-style: italic;
            color: #718096;
            font-size: 0.9em;
        }

        /* Checklist */
        ul.checklist {
            list-style: none;
            padding-left: 0;
        }

        ul.checklist li {
            padding-left: 2em;
            position: relative;
        }

        ul.checklist input[type="checkbox"] {
            position: absolute;
            left: 0;
            top: 0.3em;
            width: 1.2em;
            height: 1.2em;
            cursor: default;
        }

        ul.checklist-sub {
            margin-top: 0.5em;
            padding-left: 2em;
        }

        /* Special Blocks */
        .alert {
            padding: 1.2em;
            margin: 1.5em 0;
            border-radius: 6px;
            border-left: 4px solid;
            position: relative;
        }

        .alert-icon {
            font-size: 1.3em;
            margin-right: 0.6em;
            vertical-align: middle;
        }

        .alert-title {
            font-weight: 600;
            display: block;
            margin-bottom: 0.5em;
        }

        .alert-content {
            margin-top: 0.5em;
        }

        .alert-info {
            background: #ebf8ff;
            border-color: #3182ce;
            color: #2c5282;
        }

        .alert-warning {
            background: #fffaf0;
            border-color: #ed8936;
            color: #7c2d12;
        }

        .alert-error, .alert-danger {
            background: #fff5f5;
            border-color: #e53e3e;
            color: #742a2a;
        }

        .alert-success {
            background: #f0fff4;
            border-color: #38a169;
            color: #22543d;
        }

        .alert-tip {
            background: #faf5ff;
            border-color: #805ad5;
            color: #44337a;
        }

        /* Mermaid Diagrams */
        .mermaid-title {
            text-align: center;
            font-weight: 600;
            margin-bottom: 1em;
            color: #2d3748;
        }

        .mermaid {
            text-align: center;
            margin: 1.5em 0;
            padding: 1.5em;
            background: #f8fafc;
            border-radius: 8px;
            border: 1px solid #e2e8f0;
        }

        /* PlantUML Diagrams */
        .plantuml-container {
            margin: 1.5em 0;
            text-align: center;
            position: relative;
            min-height: 200px; /* Reserve space for loader */
        }

        .plantuml-title {
            text-align: center;
            font-weight: 600;
            margin-bottom: 1em;
            color: #2d3748;
        }

        .plantuml-diagram,
        .plantuml-fallback {
            max-width: 100%;
            height: auto;
            background: #f8fafc;
            border-radius: 8px;
            border: 1px solid #e2e8f0;
            padding: 1.5em;
        }

        /* PlantUML Loader */
        .plantuml-loader {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            text-align: center;
            z-index: 1;
        }

        .plantuml-spinner {
            width: 50px;
            height: 50px;
            border: 4px solid #e2e8f0;
            border-top: 4px solid #3498db;
            border-radius: 50%;
            animation: plantuml-spin 1s linear infinite;
            margin: 0 auto 1em;
        }

        .plantuml-loader-text {
            color: #718096;
            font-size: 0.9em;
            font-weight: 500;
        }

        @keyframes plantuml-spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }

        /* Hide loader when diagram loads */
        .plantuml-container.loaded .plantuml-loader {
            display: none;
        }

        /* Offline PlantUML (no loader needed) */
        .plantuml-offline, .plantuml-inline {
            max-width: 100%;
            height: auto;
            display: block;
            margin: 0 auto;
        }

        /* PlantUML error message */
        .plantuml-error {
            padding: 1em;
            background: #fee;
            border: 1px solid #fcc;
            border-radius: 4px;
            color: #c00;
            font-family: monospace;
            font-size: 0.9em;
        }

        /* Charts */
        .chart-title {
            text-align: center;
            font-weight: 600;
            margin-bottom: 1em;
            color: #2d3748;
        }

        .chart-container {
            margin: 1.5em 0;
            padding: 1.5em;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            position: relative;
        }

        .chart-container canvas {
            max-height: 400px;
        }

        .chart-config {
            display: none;
        }

        /* Legacy chart class for compatibility */
        .chart {
            margin: 1.5em 0;
            padding: 1.5em;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            min-height: 300px;
            position: relative;
        }

        /* Maps */
        .map-title {
            text-align: center;
            font-weight: 600;
            margin-bottom: 1em;
            color: #2d3748;
        }

        .map {
            margin: 1.5em 0;
            padding: 0;
            background: #f8fafc;
            border-radius: 8px;
            border: 1px solid #e2e8f0;
            height: 500px;
            position: relative;
            overflow: hidden;
        }

        .map-marker {
            display: none; /* Hidden, used only for data */
        }

        /* Code Groups */
        .code-group {
            margin: 1.5em 0;
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }

        .code-group-tabs {
            display: flex;
            background: #2d3748;
            border-bottom: 2px solid #4a5568;
        }

        .code-group-tab {
            padding: 0.8em 1.5em;
            background: transparent;
            color: #a0aec0;
            border: none;
            cursor: pointer;
            font-size: 0.9em;
            font-weight: 500;
            transition: all 0.2s ease;
        }

        .code-group-tab:hover {
            background: #4a5568;
            color: #e2e8f0;
        }

        .code-group-tab.active {
            background: #1e1e1e;
            color: #fff;
            border-bottom: 2px solid #3498db;
        }

        .code-group-content {
            position: relative;
        }

        .code-group-block {
            display: none;
        }

        .code-group-block.active {
            display: block;
        }

        .code-group-block pre {
            margin: 0;
            border-radius: 0 0 8px 8px;
        }

        /* Grid Layout */
        .grid {
            display: grid;
            gap: 1.5em;
            margin: 1.5em 0;
        }

        .grid[data-columns="2"] {
            grid-template-columns: repeat(2, 1fr);
        }

        .grid[data-columns="3"] {
            grid-template-columns: repeat(3, 1fr);
        }

        .grid[data-columns="4"] {
            grid-template-columns: repeat(4, 1fr);
        }

        .grid-column {
            padding: 1.2em;
            background: #f8fafc;
            border-radius: 6px;
            border: 1px solid #e2e8f0;
        }

        /* Table of Contents */
        /* Table of Contents - Themed */
        .toc {
            background: var(--doclang-toc-bg, #f8f9fa);
            padding: 2em;
            border-radius: var(--doclang-border-radius, 8px);
            margin: 2em 0 3em 0;
            border-left: 4px solid var(--doclang-toc-border, #3498db);
            box-shadow: var(--doclang-shadow-sm, 0 1px 3px rgba(0,0,0,0.1));
        }

        .toc h2 {
            margin-top: 0;
            margin-bottom: 1em;
            color: var(--doclang-toc-title-color, #1a202c);
            font-size: 1.6em;
            font-weight: 600;
            font-family: var(--doclang-font-heading, inherit);
        }

        /* Reset de listas */
        .toc ul {
            list-style-type: none;
            margin: 0;
        }

        /* Nivel 1: Secciones principales (directo bajo .toc) */
        .toc > ul {
            padding-left: 0;
        }

        .toc > ul > li {
            margin: 0.8em 0;
            line-height: 1.6;
        }

        .toc > ul > li > a {
            color: var(--doclang-toc-link-color, #1a202c);
            text-decoration: none;
            font-weight: 600;
            font-size: 1.05em;
            display: inline-block;
            transition: color 0.2s ease;
        }

        .toc > ul > li > a:hover {
            color: var(--doclang-toc-link-hover, #3498db);
        }

        .toc > ul > li > a::before {
            content: "▸ ";
            color: var(--doclang-toc-accent, #3498db);
            font-weight: bold;
            margin-right: 0.5em;
        }

        /* Nivel 2+: Subsecciones anidadas */
        .toc li ul {
            margin-top: 0.4em;
            margin-left: 1.5em;
            padding-left: 1em;
            border-left: 2px solid var(--doclang-toc-border-nested, #cbd5e0);
        }

        .toc li ul li {
            margin: 0.3em 0;
            line-height: 1.5;
        }

        .toc li ul li a {
            color: var(--doclang-toc-subsection-color, #4a5568);
            text-decoration: none;
            font-weight: 500;
            font-size: 0.9em;
            display: inline-block;
            transition: color 0.2s ease;
        }

        .toc li ul li a:hover {
            color: var(--doclang-toc-link-hover, #3498db);
        }

        .toc li ul li a::before {
            content: "• ";
            color: var(--doclang-toc-accent, #a0aec0);
            opacity: 0.6;
            margin-right: 0.4em;
        }

        /* Nivel 3+: Sub-subsecciones */
        .toc li ul li ul {
            margin-left: 1em;
            padding-left: 0.8em;
            border-left-width: 1px;
        }

        .toc li ul li ul li a {
            font-size: 0.85em;
            font-weight: 400;
        }

        .toc li ul li ul li a::before {
            content: "◦ ";
            opacity: 0.5;
        }

        /* Page Breaks */
        .page-break {
            page-break-after: always;
            height: 0;
            margin: 0;
            padding: 0;
        }

        /* Page View Mode - Visual pages like Word/Google Docs */
        body.page-view-mode {
            background: var(--doclang-page-bg, #f5f5f5);
            padding: var(--doclang-page-break-margin, 40px) 20px;
            max-width: none;
        }

        body.page-view-mode .document-page {
            background: #ffffff;
            max-width: var(--doclang-page-max-width, 210mm);
            min-height: 297mm; /* A4 height */
            margin: 0 auto var(--doclang-page-break-margin, 40px) auto;
            padding: var(--doclang-page-padding, 25mm 30mm);
            box-shadow: var(--doclang-page-shadow, 0 2px 8px rgba(0,0,0,0.15));
            position: relative;
            box-sizing: border-box;
        }

        body.page-view-mode .document-page:last-child {
            margin-bottom: var(--doclang-page-break-margin, 40px);
        }

        body.page-view-mode .page-header {
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            height: var(--doclang-header-height, 15mm);
            background: var(--doclang-header-footer-bg, #fafafa);
            border-bottom: 1px solid var(--doclang-toc-border-nested, #e2e8f0);
            padding: 8px 30mm;
            font-size: 0.85em;
            color: var(--doclang-text-light, #666);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        body.page-view-mode .page-footer {
            position: absolute;
            bottom: 0;
            left: 0;
            right: 0;
            height: var(--doclang-footer-height, 15mm);
            background: var(--doclang-header-footer-bg, #fafafa);
            border-top: 1px solid var(--doclang-toc-border-nested, #e2e8f0);
            padding: 8px 30mm;
            font-size: 0.85em;
            color: var(--doclang-text-light, #666);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        body.page-view-mode .page-content {
            padding-top: calc(var(--doclang-header-height, 15mm) + 5mm);
            padding-bottom: calc(var(--doclang-footer-height, 15mm) + 5mm);
        }

        body.page-view-mode .toc {
            margin-left: 0;
            margin-right: 0;
        }

        /* Print Styles */
        @media print {
            body.doclang-document {
                max-width: 100%;
                padding: 15mm;
                font-size: 11pt;
            }

            h1 {
                page-break-after: avoid;
            }

            h2, h3, h4 {
                page-break-after: avoid;
            }

            table, figure, .chart, .mermaid, .plantuml-container, .map {
                page-break-inside: avoid;
            }

            pre {
                page-break-inside: avoid;
            }

            .toc {
                page-break-after: always;
            }
        }

        /* Responsive */
        @media (max-width: 768px) {
            body.doclang-document {
                padding: 10mm;
            }

            h1 {
                font-size: 2em;
            }

            h2 {
                font-size: 1.6em;
            }

            .grid[data-columns="2"],
            .grid[data-columns="3"],
            .grid[data-columns="4"] {
                grid-template-columns: 1fr;
            }
        }
    </style>`)

	// Interactive Viewer CSS
	if opts.InteractiveViewer {
		css.WriteString(generateViewerStyles(opts))
	}

	// Custom CSS
	if opts.CustomCSS != "" {
		css.WriteString("\n    <style>\n")
		css.WriteString(opts.CustomCSS)
		css.WriteString("\n    </style>")
	}

	return css.String()
}

// generateDocumentScripts genera los CDN scripts en el head
func generateDocumentScripts(opts DocumentHTMLOptions) string {
	var scripts strings.Builder

	// Mermaid.js para diagramas (solo en browser mode)
	mermaidMode := opts.MermaidMode
	if mermaidMode == "" {
		mermaidMode = "browser"
	}
	if mermaidMode == "browser" {
		scripts.WriteString("    " + MermaidCDNScriptTag)
		scripts.WriteString("\n")
	}

	// Chart.js para gráficos (solo en browser mode)
	chartMode := opts.ChartMode
	if chartMode == "" {
		chartMode = "browser"
	}
	if chartMode == "browser" {
		scripts.WriteString("    " + ChartJSCDNScriptTag)
		scripts.WriteString("\n")
	}

	// Leaflet para mapas (solo en browser mode)
	mapMode := opts.MapMode
	if mapMode == "" {
		mapMode = "browser"
	}
	if mapMode == "browser" {
		scripts.WriteString("    " + LeafletCDNCSSTag)
		scripts.WriteString("\n")
		scripts.WriteString("    " + LeafletCDNScriptTag)
		scripts.WriteString("\n")
	}

	// MathJax para ecuaciones (solo en browser mode) — issue #239-B. Sin
	// script de init separado: el bundle tex-svg tipografía \[...\]
	// automáticamente al cargar (comportamiento default del combined
	// component), no requiere configurar output:'svg' — ya es lo único que
	// ese bundle produce.
	mathMode := opts.MathMode
	if mathMode == "" {
		mathMode = "browser"
	}
	if mathMode == "browser" {
		scripts.WriteString("    " + MathCDNScriptTag)
		scripts.WriteString("\n")
	}

	return scripts.String()
}

// generateInitScripts genera el JavaScript de inicialización que va antes del </body>
func generateInitScripts(opts DocumentHTMLOptions, cspNonce string) string {
	var scripts strings.Builder
	scriptTag := "<script>"
	if cspNonce != "" {
		scriptTag = fmt.Sprintf("<script nonce=%q>", cspNonce)
	}

	// Inicialización
	scripts.WriteString("    " + scriptTag + `
        // Initialize Mermaid
        mermaid.initialize(` + MermaidInitConfigJS(true, MermaidExtra{Key: "flowchart", Value: map[string]bool{"htmlLabels": false}}) + `);

        // Initialize Charts and Code Groups
        document.addEventListener('DOMContentLoaded', function() {
            // Initialize Chart.js charts
            const chartContainers = document.querySelectorAll('.chart-container');

            chartContainers.forEach(container => {
                const canvas = container.querySelector('canvas');
                const configScript = container.querySelector('script.chart-config');

                if (canvas && configScript) {
                    try {
                        const config = JSON.parse(configScript.textContent);
                        const ctx = canvas.getContext('2d');
                        new Chart(ctx, config);
                    } catch (error) {
                        console.error('Error initializing chart:', error);
                    }
                }
            });

            // Ocultar el loader de PlantUML cuando el <object>/<img> termina
            // de cargar. Antes esto era un onload="..." inline en el propio
            // elemento — un script-src con nonce (ver csp.go) bloquea
            // atributos onXXX= igual que bloquearía un script inline sin nonce, así que se
            // asigna la propiedad .onload vía JS en su lugar (eso SÍ lo
            // permite el CSP: no es un atributo inline, es una asignación de
            // una propiedad JS normal dentro de un script ya autorizado).
            document.querySelectorAll('.plantuml-diagram').forEach(el => {
                // .plantuml-diagram (el <object>) es hijo directo de
                // .plantuml-container — mismo target que el onload= original
                // (this.parentElement.classList.add('loaded')).
                const markLoadedObj = () => el.parentElement && el.parentElement.classList.add('loaded');
                el.onload = markLoadedObj;
                // <object> no tiene un equivalente a .complete de <img> — si el
                // SVG ya estaba cacheado, 'load' pudo disparar antes de que este
                // script corriera. contentDocument no-nulo indica que el recurso
                // ya se cargó (mismo origen, por lo que es accesible).
                if (el.contentDocument) {
                    markLoadedObj();
                }
            });
            document.querySelectorAll('.plantuml-fallback').forEach(img => {
                // .plantuml-fallback (el <img>) vive DENTRO del <object>, que
                // a su vez es hijo de .plantuml-container — dos niveles, como
                // el onload= original (this.parentElement.parentElement...).
                const markLoaded = () => img.parentElement && img.parentElement.parentElement &&
                    img.parentElement.parentElement.classList.add('loaded');
                img.onload = markLoaded;
                // Si la imagen ya estaba cargada/cacheada antes de que este
                // script corriera, 'load' ya disparó y nunca lo veremos —
                // chequear .complete cubre esa carrera.
                if (img.complete && img.naturalWidth > 0) {
                    markLoaded();
                }
            });

            // Initialize maps with Leaflet
            const maps = document.querySelectorAll('.map');

            // isValidColor: allowlist estricta de hex o nombre de color CSS
            // conocido (debe reflejar exactamente cssNamedColors en
            // core/renderer/sanitizer.go). El servidor ya
            // valida/escapa color (renderer.SanitizeColor), pero se revalida
            // aquí porque dataset.* decodifica el HTML-escape y el valor se
            // interpola en un atributo style (ver docs/SECURITY_AUDIT_2026-07.md, AL-7).
            const hexColorPattern = /^#[0-9a-fA-F]{3,8}$/;
            const cssNamedColors = new Set([
                'black', 'silver', 'gray', 'white', 'maroon',
                'red', 'purple', 'fuchsia', 'green', 'lime',
                'olive', 'yellow', 'navy', 'blue', 'teal',
                'aqua', 'orange', 'pink', 'brown', 'cyan',
                'magenta', 'gold', 'indigo', 'violet', 'coral',
                'salmon', 'khaki', 'crimson', 'turquoise', 'orchid',
                'tomato', 'chocolate', 'darkgreen', 'darkblue',
                'darkred', 'lightblue', 'lightgreen', 'lightgray',
                'lightgrey', 'darkgray', 'darkgrey', 'transparent',
            ]);
            const isValidColor = (c) => typeof c === 'string' &&
                (hexColorPattern.test(c) || cssNamedColors.has(c.toLowerCase()));

            maps.forEach((mapDiv, index) => {
                const mapId = 'map-' + index;
                mapDiv.id = mapId;

                const type = mapDiv.dataset.type || 'world';
                const zoom = parseInt(mapDiv.dataset.zoom || '2');
                const heatmap = mapDiv.dataset.heatmap === 'true';

                // Initialize Leaflet map
                const map = L.map(mapId, {
                    scrollWheelZoom: true,
                    dragging: true,
                    zoomControl: true
                }).setView([20, 0], zoom);

                // Add OpenStreetMap tiles
                L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
                    attribution: '© OpenStreetMap contributors',
                    maxZoom: 19
                }).addTo(map);

                // Add markers
                const markers = mapDiv.querySelectorAll('.map-marker');
                const bounds = [];

                markers.forEach(marker => {
                    const lat = parseFloat(marker.dataset.lat);
                    const lng = parseFloat(marker.dataset.lng);
                    const label = marker.dataset.label || '';
                    const value = parseFloat(marker.dataset.value || '0');
                    const size = marker.dataset.size || 'medium';
                    const details = marker.dataset.details || '';

                    // dataset.* decodifica de vuelta el HTML-escape aplicado en el
                    // servidor, así que revalidamos el color aquí antes de
                    // interpolarlo en un atributo style (ver docs/SECURITY_AUDIT_2026-07.md, AL-7).
                    const rawColor = marker.dataset.color || '#2196F3';
                    const color = isValidColor(rawColor) ? rawColor : '#2196F3';

                    bounds.push([lat, lng]);

                    // Determine icon size based on size attribute
                    let iconSize = [25, 41];
                    if (size === 'small') iconSize = [20, 33];
                    if (size === 'large') iconSize = [30, 49];

                    // Create custom colored icon (color ya validado arriba)
                    const icon = L.divIcon({
                        className: 'custom-marker',
                        html: '<div style="background-color: ' + color + '; width: 20px; height: 20px; border-radius: 50%; border: 2px solid white; box-shadow: 0 2px 5px rgba(0,0,0,0.3);"></div>',
                        iconSize: [20, 20],
                        iconAnchor: [10, 10]
                    });

                    // Add marker with popup
                    const leafletMarker = L.marker([lat, lng], { icon: icon }).addTo(map);

                    // Construir el popup con nodos DOM (textContent) en vez de
                    // concatenar HTML: label/details vienen de dataset.*, que ya
                    // decodificó el escape del servidor, así que un string
                    // reabriría el XSS cerrado en Go (AL-7).
                    const popupContainer = document.createElement('div');
                    const labelEl = document.createElement('b');
                    labelEl.textContent = label;
                    popupContainer.appendChild(labelEl);
                    if (details) {
                        popupContainer.appendChild(document.createElement('br'));
                        popupContainer.appendChild(document.createTextNode(details));
                    }
                    if (value > 0) {
                        popupContainer.appendChild(document.createElement('br'));
                        popupContainer.appendChild(document.createTextNode('Value: ' + value));
                    }

                    leafletMarker.bindPopup(popupContainer);
                });

                // Fit map to markers bounds if available
                if (bounds.length > 0) {
                    map.fitBounds(bounds, { padding: [50, 50] });
                }
            });

            // Initialize code group tabs
            const codeGroups = document.querySelectorAll('.code-group');

            codeGroups.forEach(group => {
                const tabs = group.querySelectorAll('.code-group-tab');
                const blocks = group.querySelectorAll('.code-group-block');

                tabs.forEach((tab, index) => {
                    tab.addEventListener('click', () => {
                        tabs.forEach(t => t.classList.remove('active'));
                        blocks.forEach(b => b.classList.remove('active'));

                        tab.classList.add('active');
                        blocks[index].classList.add('active');
                    });
                });
            });

            // ===== PLANTUML CACHE MANAGER =====
            // Cache PlantUML diagrams locally for faster subsequent loads
            if ('caches' in window) {
                const CACHE_NAME = 'doclang-plantuml-cache-v1';
                const CACHE_DURATION = 7 * 24 * 60 * 60 * 1000; // 7 days

                // Function to cache PlantUML responses
                async function cachePlantUMLDiagram(url) {
                    try {
                        const cache = await caches.open(CACHE_NAME);
                        const response = await fetch(url);

                        if (response.ok) {
                            // Clone response and add timestamp header
                            const headers = new Headers(response.headers);
                            headers.append('X-Cache-Time', Date.now().toString());

                            const cachedResponse = new Response(response.body, {
                                status: response.status,
                                statusText: response.statusText,
                                headers: headers
                            });

                            await cache.put(url, cachedResponse);
                            console.log('✅ PlantUML cached:', url.substring(0, 60) + '...');
                        }
                    } catch (error) {
                        console.warn('⚠️ Failed to cache PlantUML:', error);
                    }
                }

                // Function to load from cache or network
                async function loadPlantUMLDiagram(url) {
                    try {
                        const cache = await caches.open(CACHE_NAME);
                        const cachedResponse = await cache.match(url);

                        if (cachedResponse) {
                            const cacheTime = cachedResponse.headers.get('X-Cache-Time');
                            const now = Date.now();

                            // Check if cache is still valid
                            if (cacheTime && (now - parseInt(cacheTime)) < CACHE_DURATION) {
                                console.log('💾 PlantUML from cache:', url.substring(0, 60) + '...');
                                return cachedResponse;
                            } else {
                                console.log('⏰ Cache expired, refetching...');
                                await cache.delete(url);
                            }
                        }

                        // Fetch from network and cache
                        const response = await fetch(url);
                        if (response.ok) {
                            await cachePlantUMLDiagram(url);
                        }
                        return response;
                    } catch (error) {
                        console.warn('⚠️ Failed to load PlantUML:', error);
                        // Fallback to direct fetch
                        return fetch(url);
                    }
                }

                // Intercept PlantUML diagram loads
                const plantumlObjects = document.querySelectorAll('object[data*="plantuml.com"]');
                plantumlObjects.forEach(obj => {
                    const url = obj.getAttribute('data');

                    // Pre-cache the diagram
                    loadPlantUMLDiagram(url).then(response => {
                        // Diagram will load automatically via the object tag
                        // This just ensures it's cached for next time
                    });
                });

                console.log('🗂️ PlantUML Cache initialized (' + plantumlObjects.length + ' diagrams)');
            }
        });
    </script>`)
	scripts.WriteString("\n")

	// Custom JS
	if opts.CustomJS != "" {
		scripts.WriteString("    " + scriptTag + "\n")
		scripts.WriteString(opts.CustomJS)
		scripts.WriteString("\n    </script>\n")
	}

	return scripts.String()
}

// generateDocumentTOC genera la tabla de contenidos
func generateDocumentTOC(doc *ast.AST, opts DocumentHTMLOptions, variables map[string]interface{}) string {
	var toc strings.Builder

	toc.WriteString(`<div class="toc">
    <h2>Tabla de Contenidos</h2>
    <ul>
`)

	sectionNum := 1
	for _, slide := range doc.ContentBlocks {
		// Para documentos, el primer slide puede usar Heading (tipo title) y los demás Title
		title := slide.Title
		if title == "" && slide.Heading != "" {
			title = slide.Heading
		}

		if title != "" {
			titleProcessed := ProcessVariables(title, variables)
			anchor := strings.ToLower(strings.ReplaceAll(titleProcessed, " ", "-"))
			anchor = sanitizeAnchor(anchor)
			titleEscaped := EscapeHTML(titleProcessed)

			if opts.Numbering {
				toc.WriteString(fmt.Sprintf(`        <li><a href="#%s">%d. %s</a></li>
`, anchor, sectionNum, titleEscaped))
			} else {
				toc.WriteString(fmt.Sprintf(`        <li><a href="#%s">%s</a></li>
`, anchor, titleEscaped))
			}

			// Buscar subsecciones (h2, h3, etc.) dentro del slide
			if opts.TOCDepth > 1 {
				subsections := extractSubsections(slide, opts.TOCDepth, variables)
				if len(subsections) > 0 {
					writeNestedTOC(&toc, subsections, 2, 0)
				}
			}

			sectionNum++
		}
	}

	toc.WriteString(`    </ul>
</div>
`)

	return toc.String()
}

// Subsection representa una subsección extraída del contenido
type Subsection struct {
	Title  string
	Anchor string
	Level  int
}

// writeNestedTOC escribe subsecciones con anidación jerárquica basada en niveles
func writeNestedTOC(toc *strings.Builder, subsections []Subsection, currentLevel int, startIdx int) int {
	if startIdx >= len(subsections) {
		return startIdx
	}

	toc.WriteString(strings.Repeat("    ", currentLevel) + "<ul>\n")

	i := startIdx
	for i < len(subsections) {
		sub := subsections[i]

		if sub.Level < currentLevel {
			// Nivel superior - volver atrás
			break
		} else if sub.Level == currentLevel {
			// Mismo nivel - escribir item
			indent := strings.Repeat("    ", currentLevel+1)
			toc.WriteString(fmt.Sprintf("%s<li><a href=\"#%s\">%s</a>", indent, sub.Anchor, sub.Title))

			// Ver si el siguiente item es de nivel más profundo
			if i+1 < len(subsections) && subsections[i+1].Level > currentLevel {
				toc.WriteString("\n")
				i = writeNestedTOC(toc, subsections, currentLevel+1, i+1)
				toc.WriteString(indent + "</li>\n")
			} else {
				toc.WriteString("</li>\n")
				i++
			}
		} else {
			// Nivel más profundo sin padre directo - tratarlo como si fuera el nivel actual
			indent := strings.Repeat("    ", currentLevel+1)
			toc.WriteString(fmt.Sprintf("%s<li><a href=\"#%s\">%s</a></li>\n", indent, sub.Anchor, sub.Title))
			i++
		}
	}

	toc.WriteString(strings.Repeat("    ", currentLevel) + "</ul>\n")
	return i
}

// extractSubsections extrae las subsecciones (h2, h3, etc.) de un slide
func extractSubsections(slide ast.ContentBlock, maxDepth int, variables map[string]interface{}) []Subsection {
	subsections := make([]Subsection, 0)

	for _, elem := range slide.Elements {
		// Los subsection headers se guardan como TextElement con HTML
		// (IsRawHTML=true, construido solo por parseSubsectionHeading, que ya
		// pasó el texto por ProcessInlineMarkdownSecureLine). Un TextElement
		// normal (IsRawHTML=false, el fallback genérico de TextParser) guarda
		// texto de usuario tal cual, sin escapar — un párrafo con HTML
		// literal como `<h2>...<a href="javascript:...">...</a></h2>`
		// coincide con el mismo patrón de búsqueda de abajo y, sin este
		// filtro, se extraía crudo hacia el TOC/sidebar: XSS de cero
		// interacción independiente del bug original de ME-9 (encontrado en
		// code-review de esta misma PR). Ver docs/SECURITY_AUDIT_2026-07.md,
		// ME-9 (issue #31).
		if textElem, ok := elem.(*ast.TextElement); ok && textElem.IsRawHTML {
			content := textElem.Content

			// Detectar headings h2, h3, h4, h5, h6
			for level := 2; level <= 6; level++ {
				if level-1 > maxDepth {
					break // No exceder la profundidad máxima
				}

				// Buscar patrón <h2 ...>texto</h2>, <h3 ...>texto</h3>, etc.
				// Puede tener atributos como id="..."
				openTagPrefix := fmt.Sprintf("<h%d", level)
				closeTag := fmt.Sprintf("</h%d>", level)

				if strings.Contains(content, openTagPrefix) {
					startIdx := strings.Index(content, openTagPrefix)
					if startIdx == -1 {
						continue
					}

					// Buscar el final del tag de apertura (>)
					openTagEnd := strings.Index(content[startIdx:], ">")
					if openTagEnd == -1 {
						continue
					}
					openTagEnd += startIdx + 1 // Posición después del >

					// Buscar el tag de cierre
					endIdx := strings.Index(content[openTagEnd:], closeTag)
					if endIdx == -1 {
						continue
					}
					endIdx += openTagEnd // Posición del </h>

					// Extraer el título (contenido entre tags). Ya llega HTML-seguro:
					// document_flex.go's parseSubsectionHeading (único lugar que
					// construye contenido <hN>) ya aplicó
					// ProcessInlineMarkdownSecureLine antes de armar el HTML —
					// re-procesarlo aquí con la variante insegura (ME-9) permitía
					// que un link `[x](javascript:...)` sobreviviera hasta el TOC
					// sin pasar por SanitizeURL. Ver docs/SECURITY_AUDIT_2026-07.md,
					// ME-9 (issue #31).
					title := content[openTagEnd:endIdx]

					// El título puede contener HTML interno como <strong>, <em>, <code>, etc.
					// Mantenerlo para que se renderice correctamente en el TOC. Usar
					// ProcessVariablesEscapeValues (no ProcessVariables): escapa el
					// valor de cada {{variable}} sustituida sin tocar el HTML de
					// alrededor (ver docs/SECURITY_AUDIT_2026-07.md, CR-2).
					titleProcessed := ProcessVariablesEscapeValues(title, variables)

					// Para el anchor, intentar extraerlo del atributo id si existe
					anchor := ""
					openTagContent := content[startIdx:openTagEnd]
					idAttrStart := strings.Index(openTagContent, `id="`)
					if idAttrStart != -1 {
						idValueStart := idAttrStart + 4 // después de id="
						idValueEnd := strings.Index(openTagContent[idValueStart:], `"`)
						if idValueEnd != -1 {
							anchor = openTagContent[idValueStart : idValueStart+idValueEnd]
						}
					}

					// Si no hay id, generar anchor desde el título
					if anchor == "" {
						anchorText := stripHTML(titleProcessed)
						anchor = strings.ToLower(strings.ReplaceAll(anchorText, " ", "-"))
						anchor = sanitizeAnchor(anchor)
					}

					subsections = append(subsections, Subsection{
						Title:  titleProcessed, // Mantener HTML para renderizado
						Anchor: anchor,
						Level:  level,
					})
				}
			}
		}
	}

	return subsections
}

// stripHTML elimina todas las etiquetas HTML de un string
func stripHTML(html string) string {
	// Eliminar etiquetas HTML comunes
	result := html
	result = strings.ReplaceAll(result, "<strong>", "")
	result = strings.ReplaceAll(result, "</strong>", "")
	result = strings.ReplaceAll(result, "<em>", "")
	result = strings.ReplaceAll(result, "</em>", "")
	result = strings.ReplaceAll(result, "<code>", "")
	result = strings.ReplaceAll(result, "</code>", "")
	result = strings.ReplaceAll(result, "<b>", "")
	result = strings.ReplaceAll(result, "</b>", "")
	result = strings.ReplaceAll(result, "<i>", "")
	result = strings.ReplaceAll(result, "</i>", "")
	result = strings.ReplaceAll(result, "<u>", "")
	result = strings.ReplaceAll(result, "</u>", "")

	// Eliminar cualquier otra etiqueta HTML con regex simple
	// Buscar patrón <tag> o </tag>
	inTag := false
	var cleaned strings.Builder
	for _, r := range result {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			cleaned.WriteRune(r)
		}
	}

	return cleaned.String()
}

// sanitizeAnchor limpia un anchor para usarlo en href
func sanitizeAnchor(anchor string) string {
	anchor = strings.ReplaceAll(anchor, ".", "")
	anchor = strings.ReplaceAll(anchor, ",", "")
	anchor = strings.ReplaceAll(anchor, ":", "")
	anchor = strings.ReplaceAll(anchor, ";", "")
	anchor = strings.ReplaceAll(anchor, "!", "")
	anchor = strings.ReplaceAll(anchor, "?", "")
	anchor = strings.ReplaceAll(anchor, "(", "")
	anchor = strings.ReplaceAll(anchor, ")", "")
	anchor = strings.ReplaceAll(anchor, "[", "")
	anchor = strings.ReplaceAll(anchor, "]", "")
	anchor = strings.ReplaceAll(anchor, "{", "")
	anchor = strings.ReplaceAll(anchor, "}", "")
	anchor = strings.ReplaceAll(anchor, "/", "")
	anchor = strings.ReplaceAll(anchor, "\\", "")
	anchor = strings.ReplaceAll(anchor, "'", "")
	anchor = strings.ReplaceAll(anchor, "\"", "")
	anchor = strings.ReplaceAll(anchor, "`", "")
	// Eliminar emojis y caracteres especiales (mantener solo letras, números, guiones)
	var cleaned strings.Builder
	for _, r := range anchor {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			cleaned.WriteRune(r)
		}
	}
	return cleaned.String()
}

// addIDsToHeaders agrega IDs a los headers H2/H3 que no tienen ID para navegación TOC
func addIDsToHeaders(html string) string {
	// Regex para encontrar headers H2 y H3 sin ID
	h2Regex := regexp.MustCompile(`<h2>(.*?)</h2>`)
	h3Regex := regexp.MustCompile(`<h3>(.*?)</h3>`)

	// Procesar H2
	html = h2Regex.ReplaceAllStringFunc(html, func(match string) string {
		// Extraer el contenido del header
		content := h2Regex.FindStringSubmatch(match)[1]
		// Limpiar HTML tags del contenido para generar el ID
		content = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(content, "")
		// Generar ID
		id := strings.ToLower(strings.ReplaceAll(content, " ", "-"))
		id = sanitizeAnchor(id)
		return fmt.Sprintf(`<h2 id="%s">%s</h2>`, id, h2Regex.FindStringSubmatch(match)[1])
	})

	// Procesar H3
	html = h3Regex.ReplaceAllStringFunc(html, func(match string) string {
		// Extraer el contenido del header
		content := h3Regex.FindStringSubmatch(match)[1]
		// Limpiar HTML tags del contenido para generar el ID
		content = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(content, "")
		// Generar ID
		id := strings.ToLower(strings.ReplaceAll(content, " ", "-"))
		id = sanitizeAnchor(id)
		return fmt.Sprintf(`<h3 id="%s">%s</h3>`, id, h3Regex.FindStringSubmatch(match)[1])
	})

	return html
}

// generateDocumentBody genera el cuerpo del documento
func generateDocumentBody(doc *ast.AST, opts DocumentHTMLOptions, variables map[string]interface{}, ctx *RenderContext) string {
	var body strings.Builder

	// Get document title for headers
	docTitle := opts.Title
	if docTitle == "" && doc.FrontMatter != nil && doc.FrontMatter.Title != "" {
		docTitle = doc.FrontMatter.Title
	}

	// Page-view mode: wrap content in page containers
	if opts.ShowHeaders || opts.ShowFooters {
		return generatePageViewBody(doc, opts, variables, docTitle, ctx)
	}

	// Standard mode: continuous flow
	sectionNum := 1
	for i, slide := range doc.ContentBlocks {
		// Para documentos, el primer slide puede usar Heading (tipo title) y los demás Title
		title := slide.Title
		if title == "" && slide.Heading != "" {
			title = slide.Heading
		}

		if title != "" {
			titleProcessed := ProcessVariables(title, variables)
			anchor := strings.ToLower(strings.ReplaceAll(titleProcessed, " ", "-"))
			anchor = sanitizeAnchor(anchor)
			titleEscaped := EscapeHTML(titleProcessed)

			if opts.Numbering {
				body.WriteString(fmt.Sprintf(`<h1 id="%s">%d. %s</h1>
`, anchor, sectionNum, titleEscaped))
			} else {
				body.WriteString(fmt.Sprintf(`<h1 id="%s">%s</h1>
`, anchor, titleEscaped))
			}
			sectionNum++
		}

		// Generate content for each element using the shared renderer
		for _, element := range slide.Elements {
			elementHTML := RenderElementToHTML(element, variables, ctx)
			// Add IDs to H2/H3 headers for TOC navigation
			elementHTML = addIDsToHeaders(elementHTML)
			body.WriteString(elementHTML)
			body.WriteString("\n")
		}

		// Page break between sections
		if opts.PageBreaks && i < len(doc.ContentBlocks)-1 {
			body.WriteString(`<div class="page-break"></div>
`)
		}
	}

	return body.String()
}

// generatePageViewBody genera el cuerpo con páginas visuales (Word-style)
func generatePageViewBody(doc *ast.AST, opts DocumentHTMLOptions, variables map[string]interface{}, docTitle string, ctx *RenderContext) string {
	var body strings.Builder

	// El título aparece repetido en cada header/footer de página; escaparlo
	// una sola vez aquí evita duplicar EscapeHTML en los 4 sitios de abajo.
	docTitleEscaped := EscapeHTML(docTitle)

	pageNum := 1
	sectionNum := 1

	// Start first page
	body.WriteString(`<div class="document-page">
`)

	if opts.ShowHeaders {
		body.WriteString(fmt.Sprintf(`    <div class="page-header">
        <span>%s</span>
        <span>Página %d</span>
    </div>
`, docTitleEscaped, pageNum))
	}

	body.WriteString(`    <div class="page-content">
`)

	// Generate content
	for i, slide := range doc.ContentBlocks {
		// Para documentos, el primer slide puede usar Heading (tipo title) y los demás Title
		title := slide.Title
		if title == "" && slide.Heading != "" {
			title = slide.Heading
		}

		if title != "" {
			titleProcessed := ProcessVariables(title, variables)
			anchor := strings.ToLower(strings.ReplaceAll(titleProcessed, " ", "-"))
			anchor = sanitizeAnchor(anchor)
			titleEscaped := EscapeHTML(titleProcessed)

			if opts.Numbering {
				body.WriteString(fmt.Sprintf(`<h1 id="%s">%d. %s</h1>
`, anchor, sectionNum, titleEscaped))
			} else {
				body.WriteString(fmt.Sprintf(`<h1 id="%s">%s</h1>
`, anchor, titleEscaped))
			}
			sectionNum++
		}

		// Generate content for each element
		for _, element := range slide.Elements {
			elementHTML := RenderElementToHTML(element, variables, ctx)
			// Add IDs to H2/H3 headers for TOC navigation
			elementHTML = addIDsToHeaders(elementHTML)
			body.WriteString(elementHTML)
			body.WriteString("\n")
		}

		// Insert page break after each major section in page-view
		if opts.PageBreaks && i < len(doc.ContentBlocks)-1 {
			// Close current page
			body.WriteString(`    </div>
`)
			if opts.ShowFooters {
				pageNum++
				body.WriteString(fmt.Sprintf(`    <div class="page-footer">
        <span>%s</span>
        <span>Página %d</span>
    </div>
`, docTitleEscaped, pageNum-1))
			}
			body.WriteString(`</div>
`)

			// Start new page
			body.WriteString(`<div class="document-page">
`)
			if opts.ShowHeaders {
				body.WriteString(fmt.Sprintf(`    <div class="page-header">
        <span>%s</span>
        <span>Página %d</span>
    </div>
`, docTitleEscaped, pageNum))
			}
			body.WriteString(`    <div class="page-content">
`)
		}
	}

	// Close last page
	body.WriteString(`    </div>
`)
	if opts.ShowFooters {
		body.WriteString(fmt.Sprintf(`    <div class="page-footer">
        <span>%s</span>
        <span>Página %d</span>
    </div>
`, docTitleEscaped, pageNum))
	}
	body.WriteString(`</div>
`)

	return body.String()
}

// generateDocumentFooter genera el footer HTML
func generateDocumentFooter() string {
	return `</body>
</html>
`
}

// generateThemeVariables genera las variables CSS del tema. Una variable
// cuyo nombre/valor no pase SanitizeCSSCustomProperty se omite en vez de
// interpolarse sin validar (ver docs/SECURITY_AUDIT_2026-07.md, BA-11) —
// se reporta vía logger, el ctx.Logger que el caller de GenerateDocumentHTML
// arma explícitamente (issue #134/G1c). Antes usaba util.Warn (el logger
// global de conveniencia del CLI), lo que solo funcionaba si el caller
// había llamado util.InitDefault — cierto para slidelang, nunca cierto
// para doclang, que arma su propio *util.Logger sin cablear el global,
// así que estos warnings de seguridad se perdían en silencio ahí.
func generateThemeVariables(vars map[string]string, logger util.Logger) string {
	names := make([]string, 0, len(vars))
	for variable := range vars {
		names = append(names, variable)
	}
	sort.Strings(names)

	var css strings.Builder
	css.WriteString(":root {\n")
	for _, variable := range names {
		safeValue, ok := SanitizeCSSCustomProperty(variable, vars[variable])
		if !ok {
			logger.Warn("theme variable %q rejected by SanitizeCSSCustomProperty (invalid name or value contains {}/<>/;/CR/LF) and will not be applied", variable)
			continue
		}
		css.WriteString(fmt.Sprintf("  %s: %s;\n", variable, safeValue))
	}
	css.WriteString("}\n")
	return css.String()
}

// ========================================
// INTERACTIVE VIEWER FUNCTIONS
// ========================================

// generateViewerSidebar genera el sidebar interactivo con TOC
func generateViewerSidebar(doc *ast.AST, opts DocumentHTMLOptions, variables map[string]interface{}) string {
	var sidebar strings.Builder

	sidebar.WriteString(`<aside class="doclang-sidebar" id="doclang-sidebar">
    <div class="sidebar-header">
        <h2 class="sidebar-title">Contenidos</h2>
        <button type="button" class="sidebar-toggle" id="sidebar-toggle" aria-label="Toggle sidebar">
            <span class="toggle-icon">☰</span>
        </button>
    </div>

    <nav class="sidebar-toc" id="sidebar-toc">
`)

	// Generate TOC items
	sectionNum := 1
	for _, slide := range doc.ContentBlocks {
		title := slide.Title
		if title == "" && slide.Heading != "" {
			title = slide.Heading
		}

		if title != "" {
			titleProcessed := ProcessVariables(title, variables)
			anchor := strings.ToLower(strings.ReplaceAll(titleProcessed, " ", "-"))
			anchor = sanitizeAnchor(anchor)
			titleEscaped := EscapeHTML(titleProcessed)

			if opts.Numbering {
				sidebar.WriteString(fmt.Sprintf(`        <a href="#%s" class="toc-link" data-section="%s">
            <span class="toc-number">%d.</span>
            <span class="toc-text">%s</span>
        </a>
`, anchor, anchor, sectionNum, titleEscaped))
			} else {
				sidebar.WriteString(fmt.Sprintf(`        <a href="#%s" class="toc-link" data-section="%s">
            <span class="toc-text">%s</span>
        </a>
`, anchor, anchor, titleEscaped))
			}

			// Subsecciones (si TOC depth > 1)
			if opts.TOCDepth > 1 {
				subsections := extractSubsections(slide, opts.TOCDepth, variables)
				if len(subsections) > 0 {
					sidebar.WriteString(`        <div class="toc-subsections">
`)
					for _, sub := range subsections {
						indent := strings.Repeat("    ", sub.Level-1)
						sidebar.WriteString(fmt.Sprintf(`%s<a href="#%s" class="toc-link toc-sub-link" data-section="%s">
%s    <span class="toc-text">%s</span>
%s</a>
`, indent, sub.Anchor, sub.Anchor, indent, sub.Title, indent))
					}
					sidebar.WriteString(`        </div>
`)
				}
			}

			sectionNum++
		}
	}

	sidebar.WriteString(`    </nav>

    <div class="sidebar-footer">
        <button type="button" class="theme-toggle" id="theme-toggle" aria-label="Toggle dark mode">
            <span class="theme-icon">🌙</span>
            <span class="theme-text">Dark Mode</span>
        </button>
    </div>
</aside>
`)

	return sidebar.String()
}

// generateViewerStyles genera los estilos CSS del viewer interactivo. Sin
// nonce: ver generateDocumentStyles.
func generateViewerStyles(opts DocumentHTMLOptions) string {
	return "    <style>" + `
        /* ========================================
           INTERACTIVE VIEWER STYLES
           ======================================== */

        /* Dark Mode Variables */
        html[data-theme="dark"] {
            --doclang-page-bg: #1a1a1a;
            --doclang-text-color: #e0e0e0;
            --doclang-text-light: #b0b0b0;
            --doclang-h1-color: #ffffff;
            --doclang-h2-color: #f0f0f0;
            --doclang-h3-color: #e0e0e0;
            --doclang-h4-color: #d0d0d0;
            --doclang-link-color: #6fa3ef;
            --doclang-link-hover-color: #8bb5f0;
            --doclang-code-bg: #2d2d2d;
            --doclang-code-inline-bg: #2d2d2d;
            --doclang-toc-bg: #242424;
            --doclang-toc-border: #404040;
            --doclang-toc-link-color: #e0e0e0;
            --doclang-toc-accent: #6fa3ef;
            --doclang-sidebar-bg: #1e1e1e;
            --doclang-sidebar-border: #333333;
        }

        /* Layout con Sidebar */
        body.doclang-viewer {
            margin: 0;
            padding: 0;
            display: flex;
            min-height: 100vh;
            background: var(--doclang-page-bg, #ffffff);
            transition: background-color 0.3s ease;
        }

        /* Sidebar Sticky */
        .doclang-sidebar {
            position: fixed;
            left: 0;
            top: 0;
            bottom: 0;
            width: 280px;
            background: var(--doclang-sidebar-bg, #f8f9fa);
            border-right: 1px solid var(--doclang-sidebar-border, #e2e8f0);
            display: flex;
            flex-direction: column;
            z-index: 1000;
            overflow: hidden;
            transition: transform 0.3s ease, background-color 0.3s ease;
        }

        .doclang-sidebar.collapsed {
            transform: translateX(-280px);
        }

        /* Sidebar Header */
        .sidebar-header {
            padding: 1.5rem 1.25rem 1rem;
            border-bottom: 1px solid var(--doclang-sidebar-border, #e2e8f0);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .sidebar-title {
            margin: 0;
            font-size: 1.25rem;
            font-weight: 600;
            color: var(--doclang-text-color, #1a202c);
            transition: color 0.3s ease;
        }

        .sidebar-toggle {
            background: none;
            border: none;
            font-size: 1.5rem;
            cursor: pointer;
            padding: 0.25rem 0.5rem;
            color: var(--doclang-text-color, #4a5568);
            border-radius: 4px;
            transition: background-color 0.2s ease, color 0.3s ease;
        }

        .sidebar-toggle:hover {
            background: var(--doclang-toc-bg, #e2e8f0);
        }

        /* Sidebar TOC */
        .sidebar-toc {
            flex: 1;
            overflow-y: auto;
            padding: 1rem 0;
        }

        .sidebar-toc::-webkit-scrollbar {
            width: 6px;
        }

        .sidebar-toc::-webkit-scrollbar-track {
            background: transparent;
        }

        .sidebar-toc::-webkit-scrollbar-thumb {
            background: var(--doclang-toc-border, #cbd5e0);
            border-radius: 3px;
        }

        .sidebar-toc::-webkit-scrollbar-thumb:hover {
            background: var(--doclang-toc-accent, #a0aec0);
        }

        /* TOC Links */
        .toc-link {
            display: flex;
            align-items: center;
            padding: 0.5rem 1.25rem;
            color: var(--doclang-toc-link-color, #4a5568);
            text-decoration: none;
            transition: all 0.2s ease;
            border-left: 3px solid transparent;
            font-size: 0.9rem;
        }

        .toc-link:hover {
            background: var(--doclang-toc-bg, rgba(0,0,0,0.04));
            color: var(--doclang-toc-link-hover, #3498db);
        }

        .toc-link.active {
            background: var(--doclang-toc-bg, rgba(52, 152, 219, 0.1));
            color: var(--doclang-toc-accent, #3498db);
            border-left-color: var(--doclang-toc-accent, #3498db);
            font-weight: 600;
        }

        .toc-number {
            margin-right: 0.5rem;
            color: var(--doclang-toc-accent, #3498db);
            font-weight: 600;
            font-size: 0.85rem;
        }

        .toc-text {
            flex: 1;
        }

        /* Subsecciones */
        .toc-subsections {
            margin-left: 0.5rem;
        }

        .toc-sub-link {
            padding: 0.4rem 1.25rem 0.4rem 2rem;
            font-size: 0.85rem;
            opacity: 0.9;
        }

        .toc-sub-link::before {
            content: "•";
            margin-right: 0.5rem;
            color: var(--doclang-toc-accent, #a0aec0);
        }

        /* Sidebar Footer */
        .sidebar-footer {
            padding: 1rem 1.25rem;
            border-top: 1px solid var(--doclang-sidebar-border, #e2e8f0);
        }

        /* Theme Toggle Button */
        .theme-toggle {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 0.5rem;
            padding: 0.75rem 1rem;
            background: var(--doclang-toc-bg, #e2e8f0);
            border: none;
            border-radius: 6px;
            color: var(--doclang-text-color, #4a5568);
            font-size: 0.9rem;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.2s ease;
        }

        .theme-toggle:hover {
            background: var(--doclang-toc-accent, #3498db);
            color: #ffffff;
            transform: translateY(-1px);
        }

        .theme-icon {
            font-size: 1.1rem;
        }

        /* Main Content Area */
        .doclang-content {
            flex: 1;
            margin-left: 280px;
            padding: 2rem 3rem;
            max-width: 1200px;
            transition: margin-left 0.3s ease;
            position: relative;
        }

        body.doclang-viewer.sidebar-collapsed .doclang-content {
            margin-left: 0;
        }

        /* Reading Progress Bar */
        .reading-progress {
            position: fixed;
            top: 0;
            left: 280px;
            right: 0;
            height: 3px;
            background: linear-gradient(
                to right,
                var(--doclang-toc-accent, #3498db) 0%,
                var(--doclang-toc-accent, #3498db) var(--scroll-progress, 0%),
                transparent var(--scroll-progress, 0%)
            );
            z-index: 999;
            transition: left 0.3s ease;
        }

        body.doclang-viewer.sidebar-collapsed .reading-progress {
            left: 0;
        }

        /* Back to Top Button */
        .back-to-top {
            position: fixed;
            bottom: 2rem;
            right: 2rem;
            width: 3rem;
            height: 3rem;
            background: var(--doclang-toc-accent, #3498db);
            color: white;
            border: none;
            border-radius: 50%;
            font-size: 1.5rem;
            cursor: pointer;
            opacity: 0;
            visibility: hidden;
            transition: all 0.3s ease;
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
            z-index: 998;
        }

        .back-to-top.visible {
            opacity: 1;
            visibility: visible;
        }

        .back-to-top:hover {
            background: var(--doclang-toc-link-hover, #2980b9);
            transform: translateY(-3px);
            box-shadow: 0 6px 16px rgba(0,0,0,0.2);
        }

        /* Mobile Responsive */
        @media (max-width: 768px) {
            .doclang-sidebar {
                transform: translateX(-280px);
            }

            .doclang-sidebar.open {
                transform: translateX(0);
            }

            .doclang-content {
                margin-left: 0 !important;
                padding: 1.5rem 1rem;
                max-width: 100%;
            }

            .reading-progress {
                left: 0 !important;
            }

            /* Overlay cuando sidebar está abierto en móvil */
            body.doclang-viewer.sidebar-open::after {
                content: '';
                position: fixed;
                top: 0;
                left: 0;
                right: 0;
                bottom: 0;
                background: rgba(0,0,0,0.5);
                z-index: 999;
            }
        }

        /* Print Styles - Ocultar viewer al imprimir */
        @media print {
            .doclang-sidebar,
            .reading-progress,
            .back-to-top {
                display: none !important;
            }

            .doclang-content {
                margin-left: 0 !important;
                max-width: 100% !important;
            }

            body.doclang-viewer {
                background: white !important;
            }
        }
    </style>
`
}

// generateViewerScripts genera los scripts JavaScript del viewer
func generateViewerScripts(opts DocumentHTMLOptions, cspNonce string) string {
	scriptTag := "<script>"
	if cspNonce != "" {
		scriptTag = fmt.Sprintf("<script nonce=%q>", cspNonce)
	}
	return "    " + scriptTag + `
        // ========================================
        // INTERACTIVE VIEWER SCRIPTS
        // ========================================

        document.addEventListener('DOMContentLoaded', function() {
            'use strict';

            // Elements
            const sidebar = document.getElementById('doclang-sidebar');
            const sidebarToggle = document.getElementById('sidebar-toggle');
            const themeToggle = document.getElementById('theme-toggle');
            const tocLinks = document.querySelectorAll('.toc-link');
            const progressBar = document.querySelector('.reading-progress');

            // State
            let currentSection = null;

            // ===== SIDEBAR TOGGLE =====
            if (sidebarToggle) {
                sidebarToggle.addEventListener('click', function() {
                    const isCollapsed = sidebar.classList.toggle('collapsed');
                    document.body.classList.toggle('sidebar-collapsed', isCollapsed);

                    // En móvil, usar 'open' en lugar de 'collapsed'
                    if (window.innerWidth <= 768) {
                        sidebar.classList.remove('collapsed');
                        sidebar.classList.toggle('open');
                        document.body.classList.toggle('sidebar-open');
                    }

                    // Save preference
                    localStorage.setItem('doclang-sidebar-collapsed', isCollapsed);
                });

                // Restore saved preference
                const savedState = localStorage.getItem('doclang-sidebar-collapsed');
                if (savedState === 'true') {
                    sidebar.classList.add('collapsed');
                    document.body.classList.add('sidebar-collapsed');
                }
            }

            // Cerrar sidebar al hacer click en overlay (móvil)
            if (window.innerWidth <= 768) {
                document.addEventListener('click', function(e) {
                    if (document.body.classList.contains('sidebar-open') &&
                        !sidebar.contains(e.target) &&
                        !sidebarToggle.contains(e.target)) {
                        sidebar.classList.remove('open');
                        document.body.classList.remove('sidebar-open');
                    }
                });
            }

            // ===== DARK MODE TOGGLE =====
            if (themeToggle) {
                const themeIcon = themeToggle.querySelector('.theme-icon');
                const themeText = themeToggle.querySelector('.theme-text');

                themeToggle.addEventListener('click', function() {
                    const html = document.documentElement;
                    const currentTheme = html.getAttribute('data-theme');
                    const newTheme = currentTheme === 'dark' ? 'light' : 'dark';

                    html.setAttribute('data-theme', newTheme);

                    // Update button
                    if (newTheme === 'dark') {
                        themeIcon.textContent = '☀️';
                        themeText.textContent = 'Light Mode';
                    } else {
                        themeIcon.textContent = '🌙';
                        themeText.textContent = 'Dark Mode';
                    }

                    // Save preference
                    localStorage.setItem('doclang-theme', newTheme);
                });

                // Restore saved theme
                const savedTheme = localStorage.getItem('doclang-theme');
                if (savedTheme === 'dark') {
                    document.documentElement.setAttribute('data-theme', 'dark');
                    themeIcon.textContent = '☀️';
                    themeText.textContent = 'Light Mode';
                }
            }

            // ===== SMOOTH SCROLL =====
            console.log('🔗 TOC Links found:', tocLinks.length);
            tocLinks.forEach(link => {
                link.addEventListener('click', function(e) {
                    e.preventDefault();
                    const targetId = this.getAttribute('href').substring(1);
                    const targetElement = document.getElementById(targetId);

                    console.log('📍 Click on TOC link:', targetId, 'Element found:', !!targetElement);

                    if (targetElement) {
                        targetElement.scrollIntoView({
                            behavior: 'smooth',
                            block: 'start'
                        });

                        // Cerrar sidebar en móvil después de click
                        if (window.innerWidth <= 768) {
                            sidebar.classList.remove('open');
                            document.body.classList.remove('sidebar-open');
                        }
                    } else {
                        console.error('❌ Target element not found:', targetId);
                    }
                });
            });

            // ===== SCROLL SPY (Active Section Highlight) =====
            function updateActiveSection() {
                const scrollPosition = window.scrollY + 100;

                // Encontrar la sección actual
                const sections = document.querySelectorAll('h1[id], h2[id], h3[id]');
                let activeSection = null;

                sections.forEach(section => {
                    if (section.offsetTop <= scrollPosition) {
                        activeSection = section.getAttribute('id');
                    }
                });

                // Actualizar TOC links
                if (activeSection && activeSection !== currentSection) {
                    currentSection = activeSection;

                    tocLinks.forEach(link => {
                        const linkSection = link.getAttribute('data-section');
                        if (linkSection === activeSection) {
                            link.classList.add('active');

                            // Scroll into view if needed
                            const tocNav = link.closest('.sidebar-toc');
                            if (tocNav) {
                                const linkTop = link.offsetTop;
                                const navHeight = tocNav.clientHeight;
                                const scrollTop = tocNav.scrollTop;

                                if (linkTop < scrollTop || linkTop > scrollTop + navHeight - 50) {
                                    link.scrollIntoView({
                                        behavior: 'smooth',
                                        block: 'center'
                                    });
                                }
                            }
                        } else {
                            link.classList.remove('active');
                        }
                    });
                }
            }

            // ===== READING PROGRESS =====
            function updateProgress() {
                if (!progressBar) return;

                const winScroll = document.documentElement.scrollTop || document.body.scrollTop;
                const height = document.documentElement.scrollHeight - document.documentElement.clientHeight;
                const scrolled = (winScroll / height) * 100;

                progressBar.style.setProperty('--scroll-progress', scrolled + '%');
            }

            // ===== BACK TO TOP BUTTON =====
            let backToTopBtn = document.createElement('button');
            backToTopBtn.className = 'back-to-top';
            backToTopBtn.innerHTML = '↑';
            backToTopBtn.setAttribute('aria-label', 'Back to top');
            document.body.appendChild(backToTopBtn);

            backToTopBtn.addEventListener('click', function() {
                window.scrollTo({
                    top: 0,
                    behavior: 'smooth'
                });
            });

            function updateBackToTop() {
                if (window.scrollY > 300) {
                    backToTopBtn.classList.add('visible');
                } else {
                    backToTopBtn.classList.remove('visible');
                }
            }

            // ===== EVENT LISTENERS =====
            let scrollTimeout;
            window.addEventListener('scroll', function() {
                clearTimeout(scrollTimeout);
                scrollTimeout = setTimeout(function() {
                    updateActiveSection();
                    updateProgress();
                    updateBackToTop();
                }, 50);
            }, { passive: true });

            // Initial update
            updateActiveSection();
            updateProgress();
            updateBackToTop();

            // ===== KEYBOARD NAVIGATION =====
            document.addEventListener('keydown', function(e) {
                // 'B' para toggle sidebar
                if (e.key === 'b' || e.key === 'B') {
                    if (!e.ctrlKey && !e.metaKey && document.activeElement.tagName !== 'INPUT') {
                        sidebarToggle?.click();
                    }
                }

                // 'D' para toggle dark mode
                if (e.key === 'd' || e.key === 'D') {
                    if (!e.ctrlKey && !e.metaKey && document.activeElement.tagName !== 'INPUT') {
                        themeToggle?.click();
                    }
                }
            });

            console.log('📚 DocLang Interactive Viewer initialized');
        });
    </script>
`
}
