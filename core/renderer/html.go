// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/xref"
)

// RenderElementToHTML convierte un elemento AST a HTML.
// Soporta todos los tipos de elementos incluyendo mermaid, maps, charts, etc.
// ctx controla el modo de rendering (browser/offline-assets/offline-inline)
// de los elementos que pueden pre-renderizarse vía Chromium — mermaid,
// plantuml, chart, map, y cualquier elemento (p. ej. grid) que los anide.
// Un ctx nil se resuelve a NewDefaultRenderContext() (todo en "browser").
func RenderElementToHTML(element ast.Element, variables map[string]interface{}, ctx *RenderContext) string {
	switch elem := element.(type) {
	case *ast.TextElement:
		return renderTextElement(elem, variables)

	case *ast.PointsElement:
		return renderPointsElement(elem, variables)

	case *ast.CodeElement:
		return renderCodeElement(elem, variables)

	case *ast.ImageElement:
		return renderImageElement(elem, variables)

	case *ast.TableElement:
		return renderTableElement(elem, variables)

	case *ast.QuoteElement:
		return renderQuoteElement(elem, variables)

	case *ast.ChecklistElement:
		return renderChecklistElement(elem, variables)

	case *ast.MermaidElement:
		return renderMermaidElement(elem, variables, ctx)

	case *ast.PlantUMLElement:
		return renderPlantUMLElement(elem, variables, ctx)

	case *ast.ChartElement:
		return renderChartElement(elem, variables, ctx)

	case *ast.MapElement:
		return renderMapElement(elem, variables, ctx)

	case *ast.SpecialBlockElement:
		return renderSpecialBlockElement(elem, variables)

	case *ast.CodeGroupElement:
		return renderCodeGroupElement(elem, variables)

	case *ast.GridElement:
		return renderGridElement(elem, variables, ctx)

	case *ast.MathElement:
		return renderMathElement(elem, variables, ctx)

	default:
		return fmt.Sprintf("<!-- Unsupported element type: %T -->", element)
	}
}

// renderTextElement procesa elementos de texto con Markdown
func renderTextElement(elem *ast.TextElement, variables map[string]interface{}) string {
	var content string

	// Si es HTML crudo, no procesar como Markdown ni escapar el HTML
	// existente, pero SÍ escapar el valor de cada {{variable}} sustituida
	// (elem.Content ya es HTML de confianza — p. ej. un heading de
	// subsección con <strong>/<em>/<code> reales — así que no podemos
	// escaparlo todo con ProcessVariablesSecure sin corromperlo).
	// Ver docs/SECURITY_AUDIT_2026-07.md, CR-2.
	if elem.IsRawHTML {
		content = ProcessVariablesEscapeValues(elem.Content, variables)
	} else {
		content = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
	}

	// Si el contenido ya contiene un tag HTML block-level (h1-h6), no envolverlo en <p>
	// Esto es importante para DocLang donde los headers de subsección son HTML directo
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "<h1") || strings.HasPrefix(trimmed, "<h2") ||
		strings.HasPrefix(trimmed, "<h3") || strings.HasPrefix(trimmed, "<h4") ||
		strings.HasPrefix(trimmed, "<h5") || strings.HasPrefix(trimmed, "<h6") {
		return content
	}

	return fmt.Sprintf("<p>%s</p>", content)
}

// renderPointsElement procesa listas (ordenadas o no ordenadas)
func renderPointsElement(elem *ast.PointsElement, variables map[string]interface{}) string {
	var html strings.Builder

	if elem.ListType == "ordered" {
		html.WriteString("<ol>")
	} else {
		html.WriteString("<ul>")
	}

	for _, item := range elem.Items {
		content := ProcessTextWithVariablesAndMarkdownSecure(item.Content, variables)
		html.WriteString(fmt.Sprintf("<li>%s", content))

		// Procesar sub-items si existen
		if len(item.SubPoints) > 0 {
			html.WriteString("<ul>")
			for _, subItem := range item.SubPoints {
				subContent := ProcessTextWithVariablesAndMarkdownSecure(subItem.Content, variables)
				html.WriteString(fmt.Sprintf("<li>%s</li>", subContent))
			}
			html.WriteString("</ul>")
		}

		html.WriteString("</li>")
	}

	if elem.ListType == "ordered" {
		html.WriteString("</ol>")
	} else {
		html.WriteString("</ul>")
	}

	return html.String()
}

// renderCodeElement procesa bloques de código con syntax highlighting
func renderCodeElement(elem *ast.CodeElement, variables map[string]interface{}) string {
	content := ProcessVariables(elem.Content, variables)
	// Escapar HTML en el contenido del código para prevenir inyección
	content = EscapeHTML(content)
	language := elem.Language
	if language == "" {
		language = "plaintext"
	}
	// Sanitizar el nombre del lenguaje para prevenir inyección en el atributo class
	language = EscapeHTMLAttribute(language)
	return fmt.Sprintf(`<pre><code class="language-%s">%s</code></pre>`, language, content)
}

// renderImageElement procesa imágenes con caption opcional
func renderImageElement(elem *ast.ImageElement, variables map[string]interface{}) string {
	source := ProcessVariables(elem.Source, variables)
	alt := ProcessVariables(elem.Alt, variables)
	caption := ProcessVariables(elem.Caption, variables)

	// Sanitizar URL de la imagen para prevenir javascript: y data: URIs peligrosas
	source = SanitizeURL(source)
	// Escapar atributos para prevenir inyección
	alt = EscapeHTMLAttribute(alt)
	caption = EscapeHTML(caption)

	if source == "" {
		// Si la URL es peligrosa, no renderizar la imagen
		return fmt.Sprintf(`<div class="image-error">Image URL blocked for security reasons</div>`)
	}

	// issue #239: Number lo asigna el pase de numeración (xref.Transform,
	// built-in de #240) ANTES de renderizar — si Label estaba vacío nunca
	// corrió, Number queda en 0 y no se antepone nada (mismo comportamiento
	// que hoy). id=ancla usa xref.AnchorID — DEBE coincidir byte a byte con
	// el href="#..." que xref.ResolveRefs generó para los \ref a este label.
	idAttr := ""
	captionPrefix := ""
	if elem.Label != "" && elem.Number > 0 {
		idAttr = fmt.Sprintf(` id="%s"`, xref.AnchorID(elem.Label))
		captionPrefix = fmt.Sprintf("Figura %d: ", elem.Number)
	}

	if caption != "" {
		return fmt.Sprintf(`<figure%s><img src="%s" alt="%s"><figcaption>%s%s</figcaption></figure>`,
			idAttr, source, alt, captionPrefix, caption)
	}
	if idAttr != "" {
		return fmt.Sprintf(`<figure%s><img src="%s" alt="%s"></figure>`, idAttr, source, alt)
	}
	return fmt.Sprintf(`<img src="%s" alt="%s">`, source, alt)
}

// renderTableElement procesa tablas con headers y rows
func renderTableElement(elem *ast.TableElement, variables map[string]interface{}) string {
	var html strings.Builder

	// issue #239: ver el comentario equivalente en renderImageElement.
	if elem.Label != "" && elem.Number > 0 {
		fmt.Fprintf(&html, `<table id="%s">`, xref.AnchorID(elem.Label))
	} else {
		html.WriteString("<table>")
	}

	// Headers
	if len(elem.Headers) > 0 {
		html.WriteString("<thead><tr>")
		for _, header := range elem.Headers {
			processedHeader := ProcessTextWithVariablesAndMarkdownSecure(header, variables)
			fmt.Fprintf(&html, `<th scope="col">%s</th>`, processedHeader)
		}
		html.WriteString("</tr></thead>")
	}

	// Rows
	html.WriteString("<tbody>")
	for _, row := range elem.Rows {
		html.WriteString("<tr>")
		for _, cell := range row {
			processedCell := ProcessTextWithVariablesAndMarkdownSecure(cell, variables)
			html.WriteString(fmt.Sprintf("<td>%s</td>", processedCell))
		}
		html.WriteString("</tr>")
	}
	html.WriteString("</tbody>")

	html.WriteString("</table>")

	// Caption opcional
	if elem.Caption != "" {
		caption := ProcessVariablesSecure(elem.Caption, variables)
		prefix := ""
		if elem.Label != "" && elem.Number > 0 {
			prefix = fmt.Sprintf("Tabla %d: ", elem.Number)
		}
		fmt.Fprintf(&html, `<p class="table-caption">%s%s</p>`, prefix, caption)
	}

	return html.String()
}

// renderQuoteElement procesa citas con autor y fuente opcionales
func renderQuoteElement(elem *ast.QuoteElement, variables map[string]interface{}) string {
	content := ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
	author := ProcessVariablesSecure(elem.Author, variables)
	source := ProcessVariablesSecure(elem.Source, variables)

	var html strings.Builder
	html.WriteString("<blockquote>")
	html.WriteString(fmt.Sprintf("<p>%s</p>", content))

	if author != "" || source != "" {
		html.WriteString("<footer>")
		if author != "" {
			html.WriteString(fmt.Sprintf("— %s", author))
		}
		if source != "" {
			if author != "" {
				html.WriteString(", ")
			}
			html.WriteString(fmt.Sprintf("<cite>%s</cite>", source))
		}
		html.WriteString("</footer>")
	}

	html.WriteString("</blockquote>")
	return html.String()
}

// renderChecklistElement procesa listas de tareas con checkboxes
func renderChecklistElement(elem *ast.ChecklistElement, variables map[string]interface{}) string {
	var html strings.Builder
	html.WriteString(`<ul class="checklist">`)

	for _, item := range elem.Items {
		content := ProcessTextWithVariablesAndMarkdownSecure(item.Content, variables)
		checked := ""
		if item.Checked {
			checked = "checked"
		}
		fmt.Fprintf(&html, `<li><input type="checkbox" %s disabled> %s`, checked, content)

		// Sub-items: el <ul> anidado debe vivir DENTRO del <li> del item
		// padre (issue #173, element-permitted-content/wcag) — un <ul>
		// hermano de <li> tras cerrarlo es contenido inválido bajo <ul>.
		if len(item.SubItems) > 0 {
			html.WriteString(`<ul class="checklist-sub">`)
			for _, subItem := range item.SubItems {
				subContent := ProcessTextWithVariablesAndMarkdownSecure(subItem.Content, variables)
				subChecked := ""
				if subItem.Checked {
					subChecked = "checked"
				}
				html.WriteString(fmt.Sprintf(`<li><input type="checkbox" %s disabled> %s</li>`, subChecked, subContent))
			}
			html.WriteString("</ul>")
		}

		html.WriteString("</li>")
	}

	html.WriteString("</ul>")
	return html.String()
}

// OfflineElementClasses son las clases CSS literales que renderChartElement/
// renderMermaidElement/renderMapElement (y sus 4 helpers *OfflineAssets/
// *OfflineInline, debajo) emiten para el HTML alcanzable en modos offline —
// issue #123: slidelang las usa para namespacear ese HTML (con el
// prefijo "slidelang-") antes de inyectarlo, ya que ninguna de estas clases
// lleva el prefijo que usa el resto de su generador.
//
// Vive junto a las funciones que las emiten (en vez de en el módulo
// consumidor, slidelang) para que la lista completa quede en el MISMO
// archivo que cualquier futuro cambio a esas funciones — si herramientas de
// análisis estático no bastan para mantenerlas en sync automáticamente
// entre módulos (slidelang/core son módulos Go separados),
// que al menos queden a la vista de quien edite este archivo. Se
// mantuvo una lista análoga hand-copied en slidelang antes de esto y ya
// divergió una vez (issue #123, 2 de ~13 entradas faltantes, encontrado en
// code-review) — slidelang ahora construye su reemplazo namespacing
// A PARTIR de esta lista en vez de copiarla a mano.
var OfflineElementClasses = []string{
	"chart-wrapper",
	"chart-title",
	"chart-image chart-offline",
	"chart-image chart-inline",
	"chart-error",
	"mermaid-container",
	"mermaid-title",
	"mermaid-diagram mermaid-offline",
	"mermaid-diagram mermaid-inline",
	"mermaid-error",
	"map-wrapper",
	"map-title",
	"map-image map-offline",
	"map-image map-inline",
	"map-error",
}

// renderMermaidElement procesa diagramas Mermaid
// Soporta 3 modos: browser (CDN), offline-assets (archivos), offline-inline (SVG embebido)
func renderMermaidElement(elem *ast.MermaidElement, variables map[string]interface{}, ctx *RenderContext) string {
	content := ProcessVariables(elem.Content, variables)
	title := ProcessVariablesSecure(elem.Title, variables)

	ctx = resolveRenderContext(ctx)

	var html strings.Builder
	html.WriteString(`<div class="mermaid-container">`)

	if title != "" {
		html.WriteString(fmt.Sprintf(`<div class="mermaid-title">%s</div>`, title))
	}

	// Renderizar según el modo
	switch ctx.MermaidMode {
	case "offline-assets":
		// Modo: Renderizar con Chromium y guardar en assets/diagrams/
		html.WriteString(renderMermaidOfflineAssets(content, ctx))

	case "offline-inline":
		// Modo: Renderizar con Chromium e insertar SVG inline
		html.WriteString(renderMermaidOfflineInline(content, ctx))

	default: // "browser" o vacío
		// Modo: Renderizar en browser con Mermaid.js CDN (modo actual)
		html.WriteString(renderMermaidBrowser(content))
	}

	html.WriteString(`</div>`)
	return html.String()
}

// renderMermaidBrowser genera HTML para renderizado browser (CDN).
// Delega en BuildMermaidDiv (mermaid_html.go), el único constructor del div
// escapado, para no re-copiar el patrón literal+EscapeHTML (issue #84).
func renderMermaidBrowser(content string) string {
	return BuildMermaidDiv(content)
}

// renderMermaidOfflineAssets renderiza con Chromium y guarda SVG como archivo
func renderMermaidOfflineAssets(content string, ctx *RenderContext) string {
	if ctx.MermaidFetcher == nil {
		// Fallback si no hay fetcher configurado
		return fmt.Sprintf(`<div class="mermaid-error">Mermaid fetcher not configured. Use --mermaid-mode=browser instead.</div>`)
	}

	// Renderizar y guardar SVG
	relativePath, err := ctx.MermaidFetcher.FetchAndSave(ctx.Ctx, content, filepath.Join(ctx.OutputDir, "assets"))
	if err != nil {
		return fmt.Sprintf(`<div class="mermaid-error">Failed to render Mermaid diagram: %v</div>`, err)
	}

	// Generar HTML con referencia al archivo
	return fmt.Sprintf(`<img src="assets/%s" alt="Mermaid Diagram" class="mermaid-diagram mermaid-offline" type="image/svg+xml">`,
		relativePath)
}

// renderMermaidOfflineInline renderiza con Chromium e inserta SVG inline
func renderMermaidOfflineInline(content string, ctx *RenderContext) string {
	if ctx.MermaidFetcher == nil {
		// Fallback si no hay fetcher configurado
		return fmt.Sprintf(`<div class="mermaid-error">Mermaid fetcher not configured. Use --mermaid-mode=browser instead.</div>`)
	}

	// Renderizar a SVG inline
	svgContent, err := ctx.MermaidFetcher.FetchInline(ctx.Ctx, content)
	if err != nil {
		return fmt.Sprintf(`<div class="mermaid-error">Failed to render Mermaid diagram: %v</div>`, err)
	}

	// Insertar SVG directamente en el HTML
	return fmt.Sprintf(`<div class="mermaid-diagram mermaid-inline">%s</div>`, svgContent)
}

// renderMathElement procesa ecuaciones/fórmulas LaTeX (issue #239-B).
// Motor: MathJax con salida SVG (autocontenida, sin web-fonts — no
// requiere tocar renderer/csp.go, a diferencia de KaTeX). Mismos 3 modos que
// Mermaid: browser (CDN, client-side), offline-assets/offline-inline
// (pre-renderizado vía Chromium a SVG, mismo mecanismo de fetcher).
func renderMathElement(elem *ast.MathElement, variables map[string]interface{}, ctx *RenderContext) string {
	content := ProcessVariables(elem.Content, variables)
	caption := EscapeHTML(ProcessVariables(elem.Caption, variables))

	ctx = resolveRenderContext(ctx)

	idAttr := ""
	numberSpan := ""
	if elem.Label != "" && elem.Number > 0 {
		idAttr = fmt.Sprintf(` id="%s"`, xref.AnchorID(elem.Label))
		numberSpan = fmt.Sprintf(`<span class="math-number">(%d)</span>`, elem.Number)
	}

	var html strings.Builder
	fmt.Fprintf(&html, `<div class="math-block"%s>`, idAttr)

	switch ctx.MathMode {
	case "offline-assets":
		html.WriteString(renderMathOfflineAssets(content, ctx))
	case "offline-inline":
		html.WriteString(renderMathOfflineInline(content, ctx))
	default: // "browser" o vacío
		html.WriteString(renderMathBrowser(content))
	}
	html.WriteString(numberSpan)

	if caption != "" {
		fmt.Fprintf(&html, `<div class="math-caption">%s</div>`, caption)
	}

	html.WriteString(`</div>`)
	return html.String()
}

// renderMathBrowser genera HTML para renderizado browser (CDN, client-side).
// Delega en BuildMathDiv (math_html.go), el único constructor del div
// escapado — mismo patrón que renderMermaidBrowser/BuildMermaidDiv.
func renderMathBrowser(content string) string {
	return BuildMathDiv(content)
}

// renderMathOfflineAssets renderiza con Chromium (MathJax→SVG) y guarda el
// SVG como archivo — mismo patrón que renderMermaidOfflineAssets.
func renderMathOfflineAssets(content string, ctx *RenderContext) string {
	if ctx.MathFetcher == nil {
		return `<div class="math-error">Math fetcher not configured. Use --math-mode=browser instead.</div>`
	}
	relativePath, err := ctx.MathFetcher.FetchAndSave(ctx.Ctx, content, filepath.Join(ctx.OutputDir, "assets"))
	if err != nil {
		return fmt.Sprintf(`<div class="math-error">Failed to render equation: %v</div>`, err)
	}
	return fmt.Sprintf(`<img src="assets/%s" alt="Equation" class="math-diagram math-offline" type="image/svg+xml">`,
		relativePath)
}

// renderMathOfflineInline renderiza con Chromium (MathJax→SVG) e inserta el
// SVG inline — mismo patrón que renderMermaidOfflineInline.
func renderMathOfflineInline(content string, ctx *RenderContext) string {
	if ctx.MathFetcher == nil {
		return `<div class="math-error">Math fetcher not configured. Use --math-mode=browser instead.</div>`
	}
	svgContent, err := ctx.MathFetcher.FetchInline(ctx.Ctx, content)
	if err != nil {
		return fmt.Sprintf(`<div class="math-error">Failed to render equation: %v</div>`, err)
	}
	return fmt.Sprintf(`<div class="math-diagram math-inline">%s</div>`, svgContent)
}

// renderPlantUMLElement procesa diagramas PlantUML
// Soporta 3 modos: browser (lazy), offline-assets (archivos), offline-inline (SVG embebido)
func renderPlantUMLElement(elem *ast.PlantUMLElement, variables map[string]interface{}, ctx *RenderContext) string {
	content := ProcessVariables(elem.Content, variables)
	title := ProcessVariablesSecure(elem.Title, variables)

	// Sanitizar contenido
	content = SanitizePlantUMLContent(content)

	ctx = resolveRenderContext(ctx)

	var html strings.Builder
	html.WriteString(`<div class="plantuml-container">`)

	if title != "" {
		html.WriteString(fmt.Sprintf(`<div class="plantuml-title">%s</div>`, title))
	}

	// Renderizar según el modo
	switch ctx.PlantUMLMode {
	case "offline-assets":
		// Modo: Descargar y guardar en assets/diagrams/
		html.WriteString(renderPlantUMLOfflineAssets(content, ctx))

	case "offline-inline":
		// Modo: Descargar SVG e insertar inline en el HTML
		html.WriteString(renderPlantUMLOfflineInline(content, ctx))

	default:
		// Modo browser (default): Lazy loading con JavaScript
		html.WriteString(renderPlantUMLBrowser(content, ctx))
	}

	html.WriteString(`</div>`)
	return html.String()
}

// renderPlantUMLBrowser genera HTML con lazy loading (modo por defecto)
func renderPlantUMLBrowser(content string, ctx *RenderContext) string {
	server := ctx.PlantUMLServer
	imageURL := GeneratePlantUMLSVGURL(content, server)

	var html strings.Builder

	// Loader animado (se oculta cuando carga la imagen)
	html.WriteString(`
		<div class="plantuml-loader">
			<div class="plantuml-spinner"></div>
			<div class="plantuml-loader-text">Generando diagrama...</div>
		</div>
`)

	// Usar <object> para SVG (mejor soporte que <img>). Sin onload= inline: un
	// script-src con nonce (ver csp.go) bloquea atributos onXXX= igual que
	// bloquearía un script inline sin nonce — el "loaded" se activa vía JS en
	// generateInitScripts/wireUpPlantUMLLoadedClass en su lugar.
	html.WriteString(fmt.Sprintf(`
		<object type="image/svg+xml" data="%s" class="plantuml-diagram">
			<img src="%s" alt="PlantUML Diagram" class="plantuml-fallback">
		</object>
`, imageURL, GeneratePlantUMLPNGURL(content, server)))

	return html.String()
}

// renderPlantUMLOfflineAssets descarga y guarda en assets/diagrams/
func renderPlantUMLOfflineAssets(content string, ctx *RenderContext) string {
	if ctx.Fetcher == nil {
		// Fallback a modo browser si no hay fetcher configurado
		return renderPlantUMLBrowser(content, ctx)
	}

	// Descargar imagen y guardar en assets
	assetPath, err := ctx.Fetcher.FetchDiagramToAssets(ctx.Ctx, content)
	if err != nil {
		// En caso de error, mostrar mensaje y fallback a browser mode
		return fmt.Sprintf(`<div class="plantuml-error">Error loading diagram: %s</div>`, err.Error())
	}

	// Renderizar <img> simple apuntando al asset local
	imageType := "image/svg+xml"
	if ctx.PlantUMLFormat == "png" {
		imageType = "image/png"
	}

	return fmt.Sprintf(`<img src="%s" alt="PlantUML Diagram" class="plantuml-diagram plantuml-offline" type="%s">`,
		assetPath, imageType)
}

// renderPlantUMLOfflineInline descarga SVG y lo inserta inline
func renderPlantUMLOfflineInline(content string, ctx *RenderContext) string {
	if ctx.Fetcher == nil {
		// Fallback a modo browser si no hay fetcher configurado
		return renderPlantUMLBrowser(content, ctx)
	}

	// Descargar SVG inline
	svgContent, err := ctx.Fetcher.FetchDiagramInline(ctx.Ctx, content)
	if err != nil {
		// En caso de error, mostrar mensaje y fallback a browser mode
		return fmt.Sprintf(`<div class="plantuml-error">Error loading diagram: %s</div>`, err.Error())
	}

	// Insertar SVG directamente (agregar clase para styling)
	svgContent = strings.Replace(svgContent, "<svg", `<svg class="plantuml-diagram plantuml-inline"`, 1)

	return svgContent
}

// ResolveChartJSONMode resuelve un ast.ChartElement en modo JSON directo
// (IsJSONMode + RawJSON) a un mapa de configuración con "type" ya resuelto:
// desde el JSON si lo trae, si no desde ChartType (el tag <<chart: TYPE>>),
// parcheado de vuelta en el mapa para que ambos callers vean siempre un
// config con "type" presente.
//
// Retorna (nil, "", nil) si elem no está en modo JSON — el caller debe
// reconstruir la config desde Data/Series por el camino normal (issue #55:
// antes slidelang y doclang resolvían esto de forma independiente,
// y solo uno de los dos respetaba RawJSON/IsJSONMode en absoluto — el mismo
// chart en modo JSON directo se renderizaba distinto entre los dos DSLs,
// issue histórico #11 — este helper es la única fuente de verdad ahora).
// Retorna err!=nil si SÍ está en modo JSON pero RawJSON no parsea; el caller
// decide cómo loguear/degradar (ninguno de los dos callers actuales trata un
// error acá como fatal, ambos caen de vuelta a reconstruir desde Data/Series).
//
// Nota de code-review: esto exige que el JSON top-level sea un objeto (falla
// con err!=nil para un array u otro valor top-level, aunque sea JSON
// sintácticamente válido). Antes de este helper, doclang's
// renderChartElement decodeaba a un `interface{}` genérico (aceptaba
// cualquier forma) y re-serializaba verbatim — pero Chart.js exige un
// objeto de config (`{type, data, options}`), así que un top-level no-objeto
// nunca produjo un chart funcional en NINGÚN de los dos DSLs; slidelang
// ya exigía objeto desde antes (decodeaba directo a `map[string]interface{}`
// — issue #55: esta función unifica doclang al mismo requisito, no lo
// afloja ni lo endurece de forma nueva).
func ResolveChartJSONMode(elem *ast.ChartElement) (config map[string]interface{}, chartType string, err error) {
	if !elem.IsJSONMode || len(elem.RawJSON) == 0 {
		return nil, "", nil
	}

	if err := json.Unmarshal(elem.RawJSON, &config); err != nil {
		return nil, "", err
	}

	chartType, _ = config["type"].(string)
	if chartType == "" && elem.ChartType != "" {
		chartType = elem.ChartType
		config["type"] = chartType
	}

	return config, chartType, nil
}

// renderChartElement procesa gráficos con datos o JSON
// Soporta 3 modos: browser (CDN), offline-assets (PNG), offline-inline (PNG embebido)
func renderChartElement(elem *ast.ChartElement, variables map[string]interface{}, ctx *RenderContext) string {
	title := ProcessVariablesSecure(elem.Title, variables)

	ctx = resolveRenderContext(ctx)

	var html strings.Builder
	html.WriteString(`<div class="chart-wrapper">`)

	if title != "" {
		html.WriteString(fmt.Sprintf(`<div class="chart-title">%s</div>`, title))
	}

	// Generar configuración del chart
	var chartConfig string
	if rawConfig, _, err := ResolveChartJSONMode(elem); err == nil && rawConfig != nil {
		// No usar elem.RawJSON verbatim: es texto JSON escrito a mano por el
		// autor, nunca re-codificado por encoding/json, así que puede contener
		// literales "</script>" sin escapar. renderChartBrowser lo embebe en
		// un <script type="application/json"> (issue #19 / CR-1 del audit de
		// seguridad) - re-serializar vía json.Marshal aplica el escapado
		// HTML-safe por defecto de Go (<,>,& -> \u00xx), igual que ya hace
		// GenerateChartConfigWithMode para el modo estructurado.
		// elem.RawJSON ya fue validado con json.Valid() al parsear (ver
		// elements/chart.go), así que Marshal no debería fallar nunca acá; si
		// de todos modos falla, usar "{}" en vez de caer de vuelta al string
		// crudo (que reintroduciría la vulnerabilidad).
		chartConfig = "{}"
		if reEncoded, mErr := json.Marshal(rawConfig); mErr == nil {
			chartConfig = string(reEncoded)
		}
	} else if elem.IsJSONMode && len(elem.RawJSON) > 0 {
		// Modo JSON declarado pero RawJSON inválido — degradar a "{}" en vez
		// de reconstruir desde Data/Series (que para un chart en modo JSON
		// probablemente ni siquiera los tiene poblados).
		chartConfig = "{}"
	} else {
		// Para offline modes (que generan PNG), usar configuración optimizada
		// Para browser mode, usar configuración estándar
		if ctx.ChartMode == "offline-assets" || ctx.ChartMode == "offline-inline" {
			chartConfig = GenerateChartConfigForExport(elem)
		} else {
			chartConfig = GenerateChartConfig(elem)
		}
	}

	// Obtener dimensiones (usar defaults si no están especificadas) — única
	// fuente de verdad compartida con el gate de issue #164, ver
	// ChartDimensions.
	width, height := ChartDimensions(elem)

	// Renderizar según el modo
	switch ctx.ChartMode {
	case "offline-assets":
		// Modo: Renderizar (nativo si aplica, si no Chromium) y guardar PNG
		html.WriteString(renderChartOfflineAssets(elem, chartConfig, width, height, ctx))

	case "offline-inline":
		// Modo: Renderizar (nativo si aplica, si no Chromium) e insertar inline
		html.WriteString(renderChartOfflineInline(elem, chartConfig, width, height, ctx))

	default: // "browser" o vacío
		// Modo: Renderizar en browser con Chart.js CDN (modo actual)
		html.WriteString(renderChartBrowser(chartConfig, elem.Position.Line))
	}

	html.WriteString(`</div>`)
	return html.String()
}

// renderChartBrowser genera HTML para renderizado browser (CDN)
func renderChartBrowser(chartConfig string, line int) string {
	chartID := fmt.Sprintf("chart-%d", line)

	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<div class="chart-container" data-chart-id="%s">`, chartID))
	html.WriteString(fmt.Sprintf(`<canvas id="%s"></canvas>`, chartID))
	html.WriteString(fmt.Sprintf(`<script type="application/json" class="chart-config">%s</script>`, chartConfig))
	html.WriteString(`</div>`)

	return html.String()
}

// renderChartOfflineAssets renderiza (nativo si aplica, si no con Chromium) y guarda PNG como archivo
func renderChartOfflineAssets(elem *ast.ChartElement, chartConfig string, width, height int, ctx *RenderContext) string {
	if ctx.ChartFetcher == nil {
		return `<div class="chart-error">Chart fetcher not configured. Use --chart-mode=browser instead.</div>`
	}

	// Renderizar y guardar PNG con dimensiones especificadas
	relativePath, err := ctx.ChartFetcher.FetchAndSave(ctx.Ctx, elem, chartConfig, filepath.Join(ctx.OutputDir, "assets"), width, height)
	if err != nil {
		return fmt.Sprintf(`<div class="chart-error">Failed to render chart: %v</div>`, err)
	}

	// Generar HTML con referencia al archivo
	return fmt.Sprintf(`<img src="assets/%s" alt="Chart" class="chart-image chart-offline">`, relativePath)
}

// renderChartOfflineInline renderiza (nativo si aplica, si no con Chromium) e inserta imagen inline (PNG o WebP)
func renderChartOfflineInline(elem *ast.ChartElement, chartConfig string, width, height int, ctx *RenderContext) string {
	if ctx.ChartFetcher == nil {
		return `<div class="chart-error">Chart fetcher not configured. Use --chart-mode=browser instead.</div>`
	}

	// Renderizar imagen inline con dimensiones especificadas
	imageData, err := ctx.ChartFetcher.FetchInline(ctx.Ctx, elem, chartConfig, width, height)
	if err != nil {
		return fmt.Sprintf(`<div class="chart-error">Failed to render chart: %v</div>`, err)
	}

	// Convertir a base64
	base64Data := base64Encode(imageData)

	// Determinar MIME type según formato
	mimeType := "image/png"
	if ctx.ChartFetcher.GetImageFormat() == "webp" {
		mimeType = "image/webp"
	}

	// Insertar imagen directamente en el HTML
	return fmt.Sprintf(`<img src="data:%s;base64,%s" alt="Chart" class="chart-image chart-inline">`, mimeType, base64Data)
}

// base64Encode codifica bytes a base64
func base64Encode(data []byte) string {
	// Implementación simple de base64
	const base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result strings.Builder

	for i := 0; i < len(data); i += 3 {
		b := [3]byte{}
		n := 0
		for j := 0; j < 3 && i+j < len(data); j++ {
			b[j] = data[i+j]
			n++
		}

		result.WriteByte(base64Table[(b[0]>>2)&0x3F])
		result.WriteByte(base64Table[((b[0]<<4)|(b[1]>>4))&0x3F])

		if n > 1 {
			result.WriteByte(base64Table[((b[1]<<2)|(b[2]>>6))&0x3F])
		} else {
			result.WriteByte('=')
		}

		if n > 2 {
			result.WriteByte(base64Table[b[2]&0x3F])
		} else {
			result.WriteByte('=')
		}
	}

	return result.String()
}

// resolveSeriesNames devuelve series, o nombres "Series N" por defecto para
// cualquier serie sin nombre (1-based) — única fuente de verdad para este
// fallback, antes triplicado independientemente entre la rama combo y la
// rama bar/line de GenerateChartConfigWithMode, y otra vez en
// native_chart.go's rasterización nativa (hallazgo de code-review sobre PR
// #163: tres copias que podían divergir en cómo cada backend etiqueta series
// sin nombre).
func resolveSeriesNames(series []string, numSeries int) []string {
	names := make([]string, numSeries)
	for i := range names {
		if i < len(series) {
			names[i] = series[i]
		} else {
			names[i] = fmt.Sprintf("Series %d", i+1)
		}
	}
	return names
}

// GenerateChartConfig genera la configuración JSON de Chart.js desde un ChartElement
// Exportada para uso en generadores DOCX/PDF
// Si forExport es true, optimiza fuentes y tamaños para PNG export
func GenerateChartConfig(elem *ast.ChartElement) string {
	return GenerateChartConfigWithMode(elem, false)
}

// GenerateChartConfigForExport genera configuración optimizada para exportación a PNG/PDF
func GenerateChartConfigForExport(elem *ast.ChartElement) string {
	return GenerateChartConfigWithMode(elem, true)
}

// GenerateChartConfigWithMode genera la configuración con modo específico
func GenerateChartConfigWithMode(elem *ast.ChartElement, forExport bool) string {
	config := make(map[string]interface{})

	// Tipo de chart
	chartType := elem.ChartType
	if chartType == "combo" {
		chartType = "bar" // Chart.js no tiene "combo", se maneja con mixed types
	}
	config["type"] = chartType

	// Preparar datos
	data := make(map[string]interface{})

	// Labels (primera columna o configurado)
	if len(elem.Data) > 0 && len(elem.Data[0]) > 0 {
		labels := make([]interface{}, 0)
		for _, row := range elem.Data {
			if len(row) > 0 {
				labels = append(labels, row[0])
			}
		}
		data["labels"] = labels
	} else if len(elem.Labels) > 0 {
		data["labels"] = elem.Labels
	}

	// Datasets
	datasets := make([]map[string]interface{}, 0)

	if elem.ChartType == "combo" && len(elem.SeriesTypes) > 0 {
		// Combo chart: cada serie puede tener su propio tipo
		numSeries := len(elem.SeriesTypes)
		names := resolveSeriesNames(elem.Series, numSeries)
		for i := 0; i < numSeries; i++ {
			dataset := make(map[string]interface{})

			dataset["label"] = names[i]

			// Tipo de chart para esta serie
			dataset["type"] = elem.SeriesTypes[i]

			// Datos de la serie (columna i+1)
			seriesData := make([]interface{}, 0)
			for _, row := range elem.Data {
				if len(row) > i+1 {
					seriesData = append(seriesData, row[i+1])
				}
			}
			dataset["data"] = seriesData

			// Colores por defecto
			colors := []string{"#3498db", "#2ecc71", "#e74c3c", "#f39c12", "#9b59b6", "#1abc9c"}
			dataset["backgroundColor"] = colors[i%len(colors)]
			dataset["borderColor"] = colors[i%len(colors)]
			dataset["borderWidth"] = 2

			datasets = append(datasets, dataset)
		}
	} else if elem.ChartType == "pie" || elem.ChartType == "doughnut" {
		// Pie/Doughnut: un solo dataset con múltiples valores
		dataset := make(map[string]interface{})
		dataset["label"] = "Data"

		// Datos
		values := make([]interface{}, 0)
		if len(elem.Data) > 0 {
			for _, row := range elem.Data {
				if len(row) > 1 {
					values = append(values, row[1])
				}
			}
		}
		dataset["data"] = values

		// Colores — ciclando la paleta con el módulo, igual que las demás
		// ramas (combo arriba, bar/line abajo): colors[:len(values)] paniqueaba
		// con "slice bounds out of range" en cuanto values tenía más filas que
		// colores (8), con input perfectamente válido (issue #244).
		colors := []string{"#3498db", "#2ecc71", "#e74c3c", "#f39c12", "#9b59b6", "#1abc9c", "#34495e", "#16a085"}
		backgroundColors := make([]string, len(values))
		for i := range values {
			backgroundColors[i] = colors[i%len(colors)]
		}
		dataset["backgroundColor"] = backgroundColors

		datasets = append(datasets, dataset)
	} else {
		// Charts normales (bar, line, etc.): una serie por columna (excepto primera)
		if len(elem.Data) > 0 && len(elem.Data[0]) > 1 {
			numSeries := len(elem.Data[0]) - 1 // -1 por la columna de labels
			names := resolveSeriesNames(elem.Series, numSeries)

			for i := 0; i < numSeries; i++ {
				dataset := make(map[string]interface{})

				dataset["label"] = names[i]

				// Datos
				seriesData := make([]interface{}, 0)
				for _, row := range elem.Data {
					if len(row) > i+1 {
						seriesData = append(seriesData, row[i+1])
					}
				}
				dataset["data"] = seriesData

				// Colores
				colors := []string{"#3498db", "#2ecc71", "#e74c3c", "#f39c12", "#9b59b6", "#1abc9c"}
				color := colors[i%len(colors)]

				if elem.ChartType == "line" {
					dataset["borderColor"] = color
					dataset["backgroundColor"] = color + "33" // 20% opacity
					dataset["fill"] = false
					dataset["tension"] = 0.4
				} else {
					dataset["backgroundColor"] = color
					dataset["borderColor"] = color
				}
				dataset["borderWidth"] = 2

				datasets = append(datasets, dataset)
			}
		}
	}

	data["datasets"] = datasets
	config["data"] = data

	// Options
	options := make(map[string]interface{})
	options["responsive"] = true
	options["maintainAspectRatio"] = false

	// Si es para export, agregar configuración optimizada para PNG/PDF
	if forExport {
		applyExportOptimizations(options)
	}

	// Merge con opciones del usuario si existen
	if elem.Options != nil {
		for k, v := range elem.Options {
			options[k] = v
		}
	}

	config["options"] = options

	// Convertir a JSON
	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to generate chart config: %s"}`, err.Error())
	}

	return string(jsonBytes)
}

// applyExportOptimizations aplica configuración optimizada para exportación a PNG/PDF
func applyExportOptimizations(options map[string]interface{}) {
	// Layout padding
	options["layout"] = map[string]interface{}{
		"padding": 30,
	}

	// Plugins (leyenda, título)
	plugins := make(map[string]interface{})
	if existingPlugins, ok := options["plugins"].(map[string]interface{}); ok {
		plugins = existingPlugins
	}

	// Leyenda con fuentes grandes
	legend := make(map[string]interface{})
	if existingLegend, ok := plugins["legend"].(map[string]interface{}); ok {
		legend = existingLegend
	}
	legend["position"] = "top"
	legend["labels"] = map[string]interface{}{
		"font": map[string]interface{}{
			"size":   20,
			"weight": "bold",
		},
		"padding":       15,
		"usePointStyle": false,
		"boxWidth":      20,
		"boxHeight":     20,
	}
	plugins["legend"] = legend

	// Título con fuentes grandes (si existe)
	if title, ok := plugins["title"].(map[string]interface{}); ok {
		title["font"] = map[string]interface{}{
			"size":   28,
			"weight": "bold",
		}
		title["padding"] = 25
	}

	options["plugins"] = plugins

	// Escalas (ejes) con fuentes grandes
	scales := make(map[string]interface{})
	if existingScales, ok := options["scales"].(map[string]interface{}); ok {
		scales = existingScales
	}

	// Eje X
	x := make(map[string]interface{})
	if existingX, ok := scales["x"].(map[string]interface{}); ok {
		x = existingX
	}
	x["ticks"] = map[string]interface{}{
		"font": map[string]interface{}{
			"size":   18,
			"weight": "normal",
		},
		"padding": 12,
	}
	x["grid"] = map[string]interface{}{
		"lineWidth": 1.5,
		"display":   true,
	}
	scales["x"] = x

	// Eje Y
	y := make(map[string]interface{})
	if existingY, ok := scales["y"].(map[string]interface{}); ok {
		y = existingY
	}
	y["ticks"] = map[string]interface{}{
		"font": map[string]interface{}{
			"size":   18,
			"weight": "normal",
		},
		"padding": 12,
	}
	y["grid"] = map[string]interface{}{
		"lineWidth": 1.5,
		"display":   true,
	}
	if _, hasBeginAtZero := y["beginAtZero"]; !hasBeginAtZero {
		y["beginAtZero"] = true
	}
	scales["y"] = y

	// Eje Y1 (para combo charts)
	if existingY1, ok := scales["y1"].(map[string]interface{}); ok {
		y1 := existingY1
		y1["ticks"] = map[string]interface{}{
			"font": map[string]interface{}{
				"size":   18,
				"weight": "normal",
			},
			"padding": 12,
		}
		y1["grid"] = map[string]interface{}{
			"lineWidth": 1.5,
			"display":   true,
		}
		scales["y1"] = y1
	}

	options["scales"] = scales
}

// renderMapElement procesa mapas con marcadores
// Soporta 3 modos: browser (CDN), offline-assets (PNG), offline-inline (PNG embebido)
func renderMapElement(elem *ast.MapElement, variables map[string]interface{}, ctx *RenderContext) string {
	title := ProcessVariablesSecure(elem.Title, variables)

	ctx = resolveRenderContext(ctx)

	var html strings.Builder
	html.WriteString(`<div class="map-wrapper">`)

	if title != "" {
		html.WriteString(fmt.Sprintf(`<div class="map-title">%s</div>`, title))
	}

	// Construir MapConfig desde el elemento
	mapConfig := buildMapConfig(elem, variables)

	// Obtener dimensiones (usar defaults si no están especificadas)
	width := 800
	height := 600
	if elem.Width > 0 {
		width = elem.Width
	}
	if elem.Height > 0 {
		height = elem.Height
	}

	// Renderizar según el modo
	switch ctx.MapMode {
	case "offline-assets":
		// Modo: Renderizar con Chromium y guardar PNG
		html.WriteString(renderMapOfflineAssets(mapConfig, width, height, ctx))

	case "offline-inline":
		// Modo: Renderizar con Chromium e insertar PNG inline
		html.WriteString(renderMapOfflineInline(mapConfig, width, height, ctx))

	default: // "browser" o vacío
		// Modo: Renderizar en browser con Leaflet CDN (modo actual)
		html.WriteString(renderMapBrowser(elem, variables))
	}

	html.WriteString(`</div>`)
	return html.String()
}

// buildMapConfig construye MapConfig desde MapElement
func buildMapConfig(elem *ast.MapElement, variables map[string]interface{}) MapConfig {
	config := MapConfig{
		MapType: elem.MapType,
		Zoom:    elem.Zoom,
		Markers: make([]MapMarker, 0, len(elem.Markers)),
	}

	// Configurar center si existe
	if elem.Center != nil {
		config.CenterLat = elem.Center.Lat
		config.CenterLng = elem.Center.Lng
	}

	// Convertir marcadores
	for _, marker := range elem.Markers {
		label := ProcessVariablesSecure(marker.Label, variables)
		details := ProcessVariablesSecure(marker.Details, variables)

		config.Markers = append(config.Markers, MapMarker{
			Lat:     marker.Lat,
			Lng:     marker.Lng,
			Label:   label,
			Details: details,
			Color:   marker.Color,
			Value:   marker.Value,
		})
	}

	return config
}

// renderMapBrowser genera HTML para renderizado browser (CDN)
func renderMapBrowser(elem *ast.MapElement, variables map[string]interface{}) string {
	var html strings.Builder

	// Sanitizar el tipo de mapa para atributo
	mapType := EscapeHTMLAttribute(elem.MapType)
	html.WriteString(fmt.Sprintf(`<div class="map" data-type="%s"`, mapType))

	if elem.Heatmap {
		html.WriteString(` data-heatmap="true"`)
	}

	if elem.Zoom > 0 {
		html.WriteString(fmt.Sprintf(` data-zoom="%d"`, elem.Zoom))
	}

	if elem.Center != nil {
		html.WriteString(fmt.Sprintf(` data-center-lat="%f" data-center-lng="%f"`,
			elem.Center.Lat, elem.Center.Lng))
	}

	html.WriteString(">")

	// Marcadores
	for _, marker := range elem.Markers {
		// ProcessVariablesSecure ya escapa HTML internamente (EscapeHTML) —
		// llamar EscapeHTMLAttribute aquí encima re-escapaba el resultado
		// ya escapado (p.ej. "&" → "&amp;" → "&amp;amp;"), mostrando
		// entidades dobles en el popup del marcador ("Café &amp;amp; bar"
		// en vez de "Café & bar"). No es un problema de seguridad
		// (doble-escape es más restrictivo, no menos), pero es un bug
		// cosmético (#68). Se conserva solo la normalización de espacio en
		// blanco de EscapeHTMLAttribute (NormalizeAttributeWhitespace,
		// ver sanitizer.go) — sin ella, un salto de línea/tab literal en
		// el label/details de un marcador quedaría intacto dentro del
		// atributo HTML generado.
		label := NormalizeAttributeWhitespace(ProcessVariablesSecure(marker.Label, variables))
		details := NormalizeAttributeWhitespace(ProcessVariablesSecure(marker.Details, variables))

		html.WriteString(fmt.Sprintf(`<div class="map-marker" data-lat="%f" data-lng="%f" data-label="%s"`,
			marker.Lat, marker.Lng, label))

		if marker.Value > 0 {
			html.WriteString(fmt.Sprintf(` data-value="%f"`, marker.Value))
		}
		if marker.Color != "" {
			color := EscapeHTMLAttribute(SanitizeColor(marker.Color))
			html.WriteString(fmt.Sprintf(` data-color="%s"`, color))
		}
		if marker.Size != "" {
			size := EscapeHTMLAttribute(marker.Size)
			html.WriteString(fmt.Sprintf(` data-size="%s"`, size))
		}
		if details != "" {
			html.WriteString(fmt.Sprintf(` data-details="%s"`, details))
		}

		html.WriteString("></div>")
	}

	html.WriteString("</div>")
	return html.String()
}

// renderMapOfflineAssets renderiza con Chromium y guarda PNG como archivo
func renderMapOfflineAssets(mapConfig MapConfig, width, height int, ctx *RenderContext) string {
	if ctx.MapFetcher == nil {
		return `<div class="map-error">Map fetcher not configured. Use --map-mode=browser instead.</div>`
	}

	// Renderizar y guardar PNG con dimensiones especificadas
	relativePath, err := ctx.MapFetcher.FetchAndSave(ctx.Ctx, mapConfig, filepath.Join(ctx.OutputDir, "assets"), width, height)
	if err != nil {
		return fmt.Sprintf(`<div class="map-error">Failed to render map: %v</div>`, err)
	}

	// Generar HTML con referencia al archivo
	return fmt.Sprintf(`<img src="assets/%s" alt="Map" class="map-image map-offline">`, relativePath)
}

// renderMapOfflineInline renderiza con Chromium e inserta imagen inline (PNG o WebP)
func renderMapOfflineInline(mapConfig MapConfig, width, height int, ctx *RenderContext) string {
	if ctx.MapFetcher == nil {
		return `<div class="map-error">Map fetcher not configured. Use --map-mode=browser instead.</div>`
	}

	// Renderizar imagen inline con dimensiones especificadas
	imageData, err := ctx.MapFetcher.FetchInline(ctx.Ctx, mapConfig, width, height)
	if err != nil {
		return fmt.Sprintf(`<div class="map-error">Failed to render map: %v</div>`, err)
	}

	// Convertir a base64
	base64Data := base64Encode(imageData)

	// Determinar MIME type según formato
	mimeType := "image/png"
	if ctx.MapFetcher.GetImageFormat() == "webp" {
		mimeType = "image/webp"
	}

	// Insertar imagen directamente en el HTML
	return fmt.Sprintf(`<img src="data:%s;base64,%s" alt="Map" class="map-image map-inline">`, mimeType, base64Data)
}

// renderSpecialBlockElement procesa bloques especiales (info, warning, etc.)
func renderSpecialBlockElement(elem *ast.SpecialBlockElement, variables map[string]interface{}) string {
	title := ProcessVariablesSecure(elem.Title, variables)
	content := ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
	icon := EscapeHTML(elem.Icon)
	blockType := EscapeHTMLAttribute(elem.BlockType)

	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<div class="alert alert-%s">`, blockType))

	if icon != "" {
		html.WriteString(fmt.Sprintf(`<span class="alert-icon">%s</span>`, icon))
	}

	if title != "" {
		html.WriteString(fmt.Sprintf(`<strong class="alert-title">%s</strong>`, title))
	}

	html.WriteString(fmt.Sprintf(`<div class="alert-content">%s</div>`, content))
	html.WriteString("</div>")

	return html.String()
}

// renderCodeGroupElement procesa grupos de código con tabs
func renderCodeGroupElement(elem *ast.CodeGroupElement, variables map[string]interface{}) string {
	var html strings.Builder

	html.WriteString(`<div class="code-group">`)

	// Tabs
	html.WriteString(`<div class="code-group-tabs">`)
	for i, block := range elem.CodeBlocks {
		activeClass := ""
		if i == 0 {
			activeClass = " active"
		}
		label := EscapeHTML(block.Label)
		fmt.Fprintf(&html, `<button type="button" class="code-group-tab%s" data-index="%d">%s</button>`,
			activeClass, i, label)
	}
	html.WriteString("</div>")

	// Code blocks
	html.WriteString(`<div class="code-group-content">`)
	for i, block := range elem.CodeBlocks {
		activeClass := ""
		if i == 0 {
			activeClass = " active"
		}
		content := ProcessVariables(block.Content, variables)
		content = EscapeHTML(content)
		language := EscapeHTMLAttribute(block.Language)
		html.WriteString(fmt.Sprintf(`<div class="code-group-block%s" data-index="%d">`, activeClass, i))
		html.WriteString(fmt.Sprintf(`<pre><code class="language-%s">%s</code></pre>`, language, content))
		html.WriteString("</div>")
	}
	html.WriteString("</div>")

	html.WriteString("</div>")
	return html.String()
}

// renderGridElement procesa layouts de grid con columnas. Recibe ctx para
// propagarlo a los elementos anidados (mermaid/chart/map dentro de una
// columna de grid también deben respetar el modo de rendering offline).
func renderGridElement(elem *ast.GridElement, variables map[string]interface{}, ctx *RenderContext) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<div class="grid" data-columns="%d">`, len(elem.Columns)))

	// Prosa suelta dentro del grid pero fuera de cualquier columna (issue #9)
	if elem.Content != "" {
		html.WriteString(`<div class="grid-content">`)
		html.WriteString(ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables))
		html.WriteString(`</div>`)
	}

	for _, column := range elem.Columns {
		html.WriteString(`<div class="grid-column">`)

		// Si la columna tiene contenido de texto
		if column.Content != "" {
			content := ProcessTextWithVariablesAndMarkdownSecure(column.Content, variables)
			html.WriteString(content)
		}

		// Si la columna tiene elementos anidados
		for _, element := range column.Elements {
			html.WriteString(RenderElementToHTML(element, variables, ctx))
		}

		html.WriteString("</div>")
	}

	html.WriteString("</div>")
	return html.String()
}
