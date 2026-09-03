// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
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
		return renderImageElement(elem, variables, ctx)

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

	case *ast.MediaElement:
		return renderMediaElement(elem, variables, ctx)

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
		fmt.Fprintf(&html, "<li>%s", content)

		// Procesar sub-items si existen
		if len(item.SubPoints) > 0 {
			html.WriteString("<ul>")
			for _, subItem := range item.SubPoints {
				subContent := ProcessTextWithVariablesAndMarkdownSecure(subItem.Content, variables)
				fmt.Fprintf(&html, "<li>%s</li>", subContent)
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

// renderImageElement procesa imágenes con caption opcional. ctx puede ser
// nil (mismo contrato que el resto de RenderElementToHTML) — solo se
// consulta para el inlineo de fuentes locales bajo offline-inline (issue
// #167, ver TryInlineLocalImage).
func renderImageElement(elem *ast.ImageElement, variables map[string]interface{}, ctx *RenderContext) string {
	source := ProcessVariables(elem.Source, variables)
	alt := ProcessVariables(elem.Alt, variables)
	caption := ProcessVariables(elem.Caption, variables)

	// TryInlineLocalImage produce su propio data: URI ya seguro para
	// interpolar (el alfabeto base64 no puede romper un atributo entre
	// comillas) — NO pasa por SanitizeURL, que rechaza el scheme "data"
	// (sanitizer.go) y descartaría exactamente lo que esta rama acaba de
	// construir. Solo se intenta cuando corresponde (ctx.ImageMode ==
	// "offline-inline" y la fuente es local); en cualquier otro caso cae
	// al SanitizeURL(source) de siempre, comportamiento sin cambios.
	if inlined, ok := TryInlineLocalImage(source, ctx); ok {
		source = inlined
	} else {
		// Sanitizar URL de la imagen para prevenir javascript: y data: URIs peligrosas
		source = SanitizeURL(source)
	}
	// Escapar atributos para prevenir inyección
	alt = EscapeHTMLAttribute(alt)
	caption = EscapeHTML(caption)

	if source == "" {
		// Si la URL es peligrosa, no renderizar la imagen
		return `<div class="image-error">Image URL blocked for security reasons</div>`
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

// TryInlineLocalImage lee una imagen local del filesystem y la devuelve
// como data: URI, para el modo offline-inline (issue #167): el pipeline de
// PDF inyecta el HTML final en about:blank vía Page.SetDocumentContent
// (docs/SECURITY_AUDIT_2026-07.md, AL-5), que no tiene base URL contra la
// cual una <img src="ruta/relativa"> pueda resolver — sin inlinear, esa
// imagen queda rota en CUALQUIER PDF, exista o no el archivo. Retorna
// ("", false) cuando no aplica o falla; el caller cae al SanitizeURL(source)
// de siempre, sin cambio de comportamiento.
//
// Deliberadamente NO pasa por SanitizeURL/ValidateURLScheme: ambas rechazan
// el scheme "data" (sanitizer.go), que es justo lo que esta función
// produce — mismo criterio que ya usan renderChartOfflineInline y
// renderMapOfflineInline más abajo en este archivo, que tampoco sanitizan
// su propio data: URI. Los bytes los lee ESTE proceso Go en build time,
// desde una ruta confinada a ctx.AssetRoot (util.ResolveConfinedPath —
// mismo confinamiento AL-4 que ya aplica a la incrustación de imágenes en
// DOCX/PPTX) — nada se busca desde el contexto de la página renderizada,
// así que el modelo de amenaza de AL-5 (una página no confiable
// alcanzando fuera de su sandbox) no aplica acá.
//
// Exportada: slidelang tiene su propio camino de render de ImageElement
// (data/converter.go + template/base.go, no pasa por RenderElementToHTML),
// pero quiere el mismo comportamiento — reusar esta función evita una
// segunda implementación de una ruta con confinamiento de filesystem.
//
// A diferencia de los demás métodos de este archivo que leen ctx.Logger,
// esta función es alcanzable SIN pasar antes por resolveRenderContext —
// ninguna de las dos rutas que la llaman (renderImageElement más arriba, vía
// RenderElementToHTML; el converter.go de slidelang, directo) garantiza que
// ctx ya fue normalizado. Un *RenderContext armado a mano con Logger sin
// asignar (zero-value nil) truena con nil-pointer en el primer .Warn() de
// abajo — hallazgo de code-review; ya pasó una vez en esta misma serie, con
// los literales de slidelang en offline.go que salieron sin Logger y
// necesitaron un commit de seguimiento. logger acá abajo es la misma
// normalización que resolveRenderContext le aplicaría a ctx.Logger, sin
// mutar el ctx del caller.
func TryInlineLocalImage(source string, ctx *RenderContext) (string, bool) {
	return tryInlineLocalAsset(source, ctx, 0, "IMAGE", "image")
}

// maxInlineMediaBytes limita TryInlineLocalMedia (issue #181): un video/
// audio local puede ser mucho más grande que cualquier imagen real, y
// base64 infla ~33% el resultado que cruza el socket CDP vía
// Page.SetDocumentContent — un archivo de 100MB se vuelve ~134MB de HTML
// inyectado. TryInlineLocalImage (arriba) NO tiene este cap a propósito:
// ya está publicada (v2.19.0) y agregarle un límite ahí sería una
// regresión de comportamiento para cualquier imagen real por encima del
// cap, que volvería a caer en el "src relativo roto" que #167 existió
// para arreglar. 64 MiB es generoso para cualquier imagen real (no
// debería regresar nada de #167) pero acota el peor caso de video.
const maxInlineMediaBytes = 64 * 1024 * 1024

// TryInlineLocalMedia es el equivalente de TryInlineLocalImage (issue
// #167) para <video>/<audio> locales bajo offline-inline (issue #181) —
// mismo confinamiento AL-4 (util.ResolveConfinedPath), mismo contrato de
// retorno ("", false) cuando no aplica o falla. Nombre separado en vez de
// generalizar TryInlineLocalImage a "TryInlineLocalAsset": esa función ya
// está publicada (core/v2.19.0) y renombrarla forzaría a los consumidores
// a migrar en el mismo release — mantenerla intacta y agregar esta al
// lado es un cambio puramente aditivo.
//
// La justificación real de este símbolo es --format html
// --render-mode offline-inline, NO --format pdf: headless Chromium
// capturando un <video>/<audio> en un PDF solo muestra el poster/primer
// frame en el mejor caso (ver el doc comment de renderMediaElement), así
// que inlinear los bytes de un video en un PDF que no puede reproducirlo
// no compra mucho. En un HTML offline-inline autocontenido, el video
// local sí reproduce de verdad.
func TryInlineLocalMedia(source string, ctx *RenderContext) (string, bool) {
	return tryInlineLocalAsset(source, ctx, maxInlineMediaBytes, "MEDIA", "media source")
}

// tryInlineLocalAsset es la lógica compartida entre TryInlineLocalImage y
// TryInlineLocalMedia. maxBytes <= 0 significa sin límite (el contrato
// histórico de TryInlineLocalImage); tag/kind alimentan el mensaje de
// log — p. ej. tag="IMAGE" kind="image" reproduce exactamente el texto
// que TryInlineLocalImage ya emitía antes de esta extracción.
func tryInlineLocalAsset(source string, ctx *RenderContext, maxBytes int64, tag, kind string) (string, bool) {
	if ctx == nil || ctx.ImageMode != "offline-inline" || source == "" || ctx.AssetRoot == "" {
		return "", false
	}
	logger := ctx.Logger
	if logger == nil {
		logger = util.NewNoop()
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme != "" {
		// Remoto (http/https/mailto/...) o ya es un data: URI — nada que inlinear.
		return "", false
	}

	resolved, err := util.ResolveConfinedPath(ctx.AssetRoot, source)
	if err != nil {
		logger.Warn("[%s] local %s %q rejected: %v", tag, kind, source, err)
		return "", false
	}

	if maxBytes > 0 {
		if info, err := os.Stat(resolved); err == nil && info.Size() > maxBytes {
			logger.Warn("[%s] local %s %q (%d bytes) exceeds the %d byte inline limit, left unresolved", tag, kind, source, info.Size(), maxBytes)
			return "", false
		}
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		logger.Warn("[%s] failed to read local %s %q: %v", tag, kind, source, err)
		return "", false
	}

	mimeType := mime.TypeByExtension(filepath.Ext(resolved))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Encode(data)), true
}

// tableUsesCellStructure reports whether elem.Cells declares anything that
// isn't exactly the shape ast.DeriveCellsFromFlat would produce for a
// simple table (row 0 entirely IsHeader+Scope="col"+no span, every body row
// entirely non-header+no scope+no span) — the case where it's worth paying
// for the more expensive render path (renderTableCells).
//
// This used to only check ColSpan/RowSpan/Scope=="row", which missed a
// real-world case: a `cells:` author can mark a cell IsHeader (with
// scope=="" or "col") inside a body row, or lead with a first row that
// isn't fully header — with the narrower check, that table fell through to
// the Headers/Rows path below, which renders every cell as a plain `<td>`
// while the JSON (elem.Cells) still says isHeader:true, so HTML and JSON
// disagreed and the a11y-relevant `<th>` was silently lost. Comparing
// against the full simple-table shape instead of just span/row-scope
// catches that.
func tableUsesCellStructure(elem *ast.TableElement) bool {
	for i, row := range elem.Cells {
		for _, cell := range row {
			if cell.ColSpan > 1 || cell.RowSpan > 1 {
				return true
			}
			if i == 0 {
				if !cell.IsHeader || cell.Scope != "col" {
					return true
				}
			} else if cell.IsHeader || cell.Scope != "" {
				return true
			}
		}
	}
	return false
}

// renderMediaElement processes embedded audio/video (issue #21).
// elem.MediaType is validated against the fixed "video"/"audio" allowlist
// before being used as a tag name — never interpolated raw — because,
// unlike the rest of this element's fields, MediaType can arrive from an
// external filter via the JSON --filter pipeline (issue #240), not just
// this package's own parser; same defensive pattern as SanitizeColor/
// inlineSpanTokens. Source goes through SanitizeURL (blocks javascript:/
// data:/vbscript:/file:), same as renderImageElement — unless
// TryInlineLocalMedia (issue #181) already resolved it to a data: URI, in
// which case SanitizeURL is skipped entirely (it rejects the "data" scheme
// by design; see the equivalent comment on renderImageElement).
//
// ctx can be nil (same contract as the rest of RenderElementToHTML) — only
// consulted for local-source inlining under offline-inline.
//
// PDF/offline caveat: under chromedp (renderer/chromium) a <video>/<audio>
// doesn't play real content during headless capture — the tag is still
// emitted (with its controls, if Controls=true) showing the initial frame/
// poster, not a limitation introduced here but inherent to capturing video
// with a headless browser with no user interaction. This is why
// TryInlineLocalMedia's real payoff is --format html --render-mode
// offline-inline, not --format pdf — see its doc comment.
func renderMediaElement(elem *ast.MediaElement, variables map[string]interface{}, ctx *RenderContext) string {
	tag := "video"
	if elem.MediaType == "audio" {
		tag = "audio"
	}

	source := ProcessVariables(elem.Source, variables)
	if strings.TrimSpace(source) == "" {
		// Distinct from the SanitizeURL block below: an empty src is missing
		// data, not a blocked dangerous scheme — reporting it as "blocked
		// for security" would mislead the author into thinking SanitizeURL
		// rejected something when nothing was ever provided.
		return `<div class="media-error">Media element has no source</div>`
	}
	if inlined, ok := TryInlineLocalMedia(source, ctx); ok {
		source = inlined
	} else {
		source = SanitizeURL(source)
	}
	if source == "" {
		return `<div class="media-error">Media source blocked for security reasons</div>`
	}

	var attrs strings.Builder
	if elem.Controls {
		attrs.WriteString(" controls")
	}
	if elem.Autoplay {
		attrs.WriteString(" autoplay")
	}
	if elem.Loop {
		attrs.WriteString(" loop")
	}
	if elem.Muted {
		attrs.WriteString(" muted")
	}

	return fmt.Sprintf(`<%s src="%s"%s></%s>`, tag, source, attrs.String(), tag)
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

	if tableUsesCellStructure(elem) {
		renderTableCells(&html, elem.Cells, variables)
	} else {
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
				fmt.Fprintf(&html, "<td>%s</td>", processedCell)
			}
			html.WriteString("</tr>")
		}
		html.WriteString("</tbody>")
	}

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

// renderTableCells emits <thead>/<tbody> from the real cell structure
// (issue #20), honoring colspan/rowspan/scope per cell — unlike
// renderTableElement's Headers/Rows path, which can express none of the
// three. The leading row is treated as <thead> only if ALL of its cells are
// IsHeader AND none declares RowSpan > 1 (issue #51) — a RowSpan on a row-0
// cell reaches into row 1, which this function puts inside <tbody>, and a
// rowspan crossing the <thead>/<tbody> boundary has no consistent
// browser/AT interpretation. This deliberately DIVERGES from
// ast.FlattenCellsToRows, which still derives Headers/Rows on IsHeader
// alone: that flat view feeds doclang's markdown/DOCX output, where a
// table with no Headers renders with no header row (markdown) or is
// dropped outright (`docx.go`'s `if len(elem.Headers) == 0 { return nil
// }`) — strictly worse than the malformed <thead> this guard prevents.
// Same criterion as slidelang's cellsLeadIsHeader
// (data/converter.go, PR #50, issue #51's fix on the core side).
func renderTableCells(html *strings.Builder, cells [][]ast.TableCell, variables map[string]interface{}) {
	if len(cells) == 0 {
		return
	}

	leadIsHeader := len(cells[0]) > 0
	for _, c := range cells[0] {
		if !c.IsHeader || c.RowSpan > 1 {
			leadIsHeader = false
			break
		}
	}

	bodyStart := 0
	if leadIsHeader {
		html.WriteString("<thead>")
		writeTableCellRow(html, cells[0], variables)
		html.WriteString("</thead>")
		bodyStart = 1
	}

	html.WriteString("<tbody>")
	for _, row := range cells[bodyStart:] {
		writeTableCellRow(html, row, variables)
	}
	html.WriteString("</tbody>")
}

// writeTableCellRow emits a <tr> with one cell per ast.TableCell.
// cell.Scope is validated against the fixed "row"/"col" allowlist before
// being interpolated — any other value is discarded instead of being
// emitted raw into the attribute (same defensive pattern as
// SanitizeColor/inlineSpanTokens: never interpolate an arbitrary
// author-supplied value without going through an allowlist).
func writeTableCellRow(html *strings.Builder, row []ast.TableCell, variables map[string]interface{}) {
	html.WriteString("<tr>")
	for _, cell := range row {
		tag := "td"
		var attrs strings.Builder
		if cell.IsHeader {
			tag = "th"
			if cell.Scope == "row" || cell.Scope == "col" {
				fmt.Fprintf(&attrs, ` scope="%s"`, cell.Scope)
			}
		}
		if cell.ColSpan > 1 {
			fmt.Fprintf(&attrs, ` colspan="%d"`, cell.ColSpan)
		}
		if cell.RowSpan > 1 {
			fmt.Fprintf(&attrs, ` rowspan="%d"`, cell.RowSpan)
		}
		processedContent := ProcessTextWithVariablesAndMarkdownSecure(cell.Content, variables)
		fmt.Fprintf(html, "<%s%s>%s</%s>", tag, attrs.String(), processedContent, tag)
	}
	html.WriteString("</tr>")
}

// renderQuoteElement procesa citas con autor y fuente opcionales
func renderQuoteElement(elem *ast.QuoteElement, variables map[string]interface{}) string {
	content := ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
	author := ProcessVariablesSecure(elem.Author, variables)
	source := ProcessVariablesSecure(elem.Source, variables)

	var html strings.Builder
	html.WriteString("<blockquote>")
	fmt.Fprintf(&html, "<p>%s</p>", content)

	if author != "" || source != "" {
		html.WriteString("<footer>")
		if author != "" {
			fmt.Fprintf(&html, "— %s", author)
		}
		if source != "" {
			if author != "" {
				html.WriteString(", ")
			}
			fmt.Fprintf(&html, "<cite>%s</cite>", source)
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
				fmt.Fprintf(&html, `<li><input type="checkbox" %s disabled> %s</li>`, subChecked, subContent)
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
		fmt.Fprintf(&html, `<div class="mermaid-title">%s</div>`, title)
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
		return `<div class="mermaid-error">Mermaid fetcher not configured. Use --mermaid-mode=browser instead.</div>`
	}

	// Renderizar y guardar SVG
	relativePath, err := ctx.MermaidFetcher.FetchAndSave(ctx.Ctx, content, filepath.Join(ctx.OutputDir, "assets"))
	if err != nil {
		ctx.Logger.Warn("[MERMAID] failed to render diagram to assets: %v", err)
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
		return `<div class="mermaid-error">Mermaid fetcher not configured. Use --mermaid-mode=browser instead.</div>`
	}

	// Renderizar a SVG inline
	svgContent, err := ctx.MermaidFetcher.FetchInline(ctx.Ctx, content)
	if err != nil {
		ctx.Logger.Warn("[MERMAID] failed to render inline diagram: %v", err)
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
		ctx.Logger.Warn("[MATH] failed to render equation to assets: %v", err)
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
		ctx.Logger.Warn("[MATH] failed to render inline equation: %v", err)
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
		fmt.Fprintf(&html, `<div class="plantuml-title">%s</div>`, title)
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
	fmt.Fprintf(&html, `
		<object type="image/svg+xml" data="%s" class="plantuml-diagram">
			<img src="%s" alt="PlantUML Diagram" class="plantuml-fallback">
		</object>
`, imageURL, GeneratePlantUMLPNGURL(content, server))

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
		ctx.Logger.Warn("[PLANTUML] failed to fetch diagram to assets: %v", err)
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
		ctx.Logger.Warn("[PLANTUML] failed to fetch inline diagram: %v", err)
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
		fmt.Fprintf(&html, `<div class="chart-title">%s</div>`, title)
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
		// Llama a GenerateChartConfigWithTheme directamente (no a los dos
		// wrappers exportados de arriba, ni a GenerateChartConfigWithMode)
		// para poder pasarle AMBOS grupos de tema del contexto — este es el
		// único call site del paquete con un *RenderContext en scope, así
		// que es el único lugar donde un tema resuelto puede entrar.
		//
		// Que sea la entrada CON tema es lo que hace que el cableado exista
		// de verdad: un hallazgo de code-review encontró que este call site
		// seguía en GenerateChartConfigWithMode, que delega con
		// ChartThemeColors{} — o sea que los tokens nuevos existían en la
		// API, tenían tests, y jamás llegaban a un chart real.
		forExport := IsOfflineRenderMode(ctx.ChartMode)
		chartConfig = GenerateChartConfigWithTheme(elem, forExport, ctx.ChartCategoricalColors, ctx.ChartThemeColors)
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
	fmt.Fprintf(&html, `<div class="chart-container" data-chart-id="%s">`, chartID)
	fmt.Fprintf(&html, `<canvas id="%s"></canvas>`, chartID)
	fmt.Fprintf(&html, `<script type="application/json" class="chart-config">%s</script>`, chartConfig)
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
		ctx.Logger.Warn("[CHART] failed to render chart to assets: %v", err)
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
		ctx.Logger.Warn("[CHART] failed to render inline chart: %v", err)
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
	return GenerateChartConfigWithMode(elem, false, nil)
}

// GenerateChartConfigForExport genera configuración optimizada para exportación a PNG/PDF
func GenerateChartConfigForExport(elem *ast.ChartElement) string {
	return GenerateChartConfigWithMode(elem, true, nil)
}

// MergeChartOptions fusiona recursivamente el bloque `options:` del autor
// (source) sobre los defaults del renderer (target), mutando target in
// place: cuando una clave existe como mapa en ambos lados, sus hojas se
// combinan en vez de que source reemplace el mapa completo — así un
// `options: plugins: title: ...` del autor no borra un default hermano ya
// puesto en ese mismo mapa (p. ej. plugins.legend, puesto por
// applyExportOptimizations). Única fuente de verdad para este merge, consumida tanto por
// GenerateChartConfigWithMode (doclang) como por slidelang/.../mergeOptions,
// en la línea de lo que issue #55 hizo con ResolveChartJSONMode — antes cada
// CLI mezclaba estas opciones de forma distinta (uno superficial, el otro
// recursivo) y el mismo chart con el mismo `options:` producía configs
// distintas según el DSL, la misma clase de bug que el issue histórico #11.
func MergeChartOptions(target, source map[string]interface{}) {
	for key, value := range source {
		if existingMap, ok := target[key].(map[string]interface{}); ok {
			if sourceMap, ok := value.(map[string]interface{}); ok {
				MergeChartOptions(existingMap, sourceMap)
				continue
			}
		}
		target[key] = value
	}
}

// defaultChartColors6/8 son las paletas categóricas hardcodeadas que
// GenerateChartConfigWithMode usaba inline en cada rama antes de que
// RenderContext.ChartCategoricalColors pudiera reemplazarlas — se
// mantienen como dos tamaños distintos (no unificados en una sola de 8)
// para no cambiar el output de ningún caller existente cuando no hay
// override: combo y bar/line siempre ciclaron 6 colores, pie/doughnut
// siempre cicló 8 (issue #244).
var (
	defaultChartColors6 = []string{"#3498db", "#2ecc71", "#e74c3c", "#f39c12", "#9b59b6", "#1abc9c"}
	defaultChartColors8 = []string{"#3498db", "#2ecc71", "#e74c3c", "#f39c12", "#9b59b6", "#1abc9c", "#34495e", "#16a085"}
)

// chartCategoricalPalette devuelve override si no está vacío, o fallback
// en caso contrario — así un RenderContext.ChartCategoricalColors nil
// (todo caller hoy) reproduce exactamente la paleta literal que reemplaza.
func chartCategoricalPalette(override, fallback []string) []string {
	if len(override) > 0 {
		return override
	}
	return fallback
}

// hex6ColorRe/hex3ColorRe recognize the two hex forms chartAreaFillColor can
// safely expand — "#rrggbb" and its "#rgb" shorthand.
var (
	hex6ColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	hex3ColorRe = regexp.MustCompile(`^#([0-9a-fA-F])([0-9a-fA-F])([0-9a-fA-F])$`)
)

// chartAreaFillColor returns a translucent (~20% opacity) variant of color
// for a line chart's area fill, suffixing a hex alpha channel — but only
// when color is actually a 6- or 3-digit hex string. Before
// RenderContext.ChartCategoricalColors existed, color could only ever be
// one of this file's own hardcoded literals (always "#rrggbb"), so
// `color + "33"` was safe by construction. Now color can be anything a
// theme's chart-cat-* token resolves to — rgb()/rgba()/hsl()/a named
// color/already-alpha'd 8-digit hex — and blindly concatenating onto any
// of those produces broken CSS (e.g. "red33", or "#abc33", which is 5 hex
// digits and invalid) that a canvas 2D context silently ignores, dropping
// back to its own default fill (code-review finding on PR #224). For
// anything outside the two safely-expandable forms, this degrades to the
// opaque color instead of guessing — losing the translucency touch, never
// producing invalid output.
func chartAreaFillColor(color string) string {
	if hex6ColorRe.MatchString(color) {
		return color + "33"
	}
	if m := hex3ColorRe.FindStringSubmatch(color); m != nil {
		return "#" + m[1] + m[1] + m[2] + m[2] + m[3] + m[3] + "33"
	}
	return color
}

// GenerateChartConfigWithMode genera la configuración con modo específico.
// categoricalColors, si no está vacío, reemplaza la paleta hardcodeada de
// cada rama de tipo de chart de abajo (motor-temas-v2.md §2.2's
// chart-cat-* — un set ORDENADO indexado por módulo, igual que las
// paletas fijas que reemplaza). nil reproduce el comportamiento de
// siempre byte por byte — es lo que pasan las dos funciones exportadas de
// arriba, y lo único que cualquier caller externo de hoy (doclang's
// docx.go, vía GenerateChartConfigForExport) puede seguir observando: el
// camino que sí resuelve un tema real (slidelang, vía RenderContext) llega
// en un PR aparte.
func GenerateChartConfigWithMode(elem *ast.ChartElement, forExport bool, categoricalColors []string) string {
	return GenerateChartConfigWithTheme(elem, forExport, categoricalColors, ChartThemeColors{})
}

// GenerateChartConfigWithTheme es GenerateChartConfigWithMode más los tokens
// chart-* no categóricos (ChartThemeColors). Entrada NUEVA en vez de un
// parámetro más en la existente: GenerateChartConfigWithMode y
// GenerateChartConfigForExport ya las consume doclang (docx.go) por nombre, y
// CI corre workspace-integration contra el core DEL ÁRBOL además de
// build-test contra el PUBLICADO — cambiarle la firma a cualquiera de las dos
// rompería uno de los dos gates sin importar el orden de merge.
//
// themeColors solo tiene efecto con forExport=true: el camino de navegador ya
// lo cubre PR #228 desde el cliente. Zero value reproduce el config de
// siempre byte por byte.
func GenerateChartConfigWithTheme(elem *ast.ChartElement, forExport bool, categoricalColors []string, themeColors ChartThemeColors) string {
	config := make(map[string]interface{})

	// Tipo de chart
	chartType := elem.ChartType
	if chartType == "combo" {
		chartType = "bar" // Chart.js no tiene "combo", se maneja con mixed types
	}
	config["type"] = chartType

	// Preparar datos
	data := make(map[string]interface{})

	// Labels (primera columna o configurado). NO aplican a treemap: ese
	// controlador no consume data.labels en absoluto — cada hoja lleva su
	// propia etiqueta dentro de dataset.tree, y emitir un data.labels
	// paralelo solo mete ruido en el config.
	if chartType != "treemap" {
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
			colors := chartCategoricalPalette(categoricalColors, defaultChartColors6)
			dataset["backgroundColor"] = colors[i%len(colors)]
			dataset["borderColor"] = colors[i%len(colors)]
			dataset["borderWidth"] = 2

			datasets = append(datasets, dataset)
		}
	} else if elem.ChartType == "treemap" {
		// Treemap (chartjs-chart-treemap): la forma del dataset no se parece
		// a la de ningún tipo nativo — no hay data[] plano ni data.labels,
		// sino un `tree` de objetos más `key` (qué campo trae el número) y
		// `groups` (por qué campo agrupar). Se alimenta de la MISMA matriz
		// que todos los demás tipos del DSL (columna 0 = etiqueta de la hoja,
		// columna 1 = valor), así que el parser no necesita sintaxis propia.
		//
		// Todo lo que se emite acá es serializable a JSON: `captions` dibuja
		// el nombre del grupo sin ninguna función (display:true por defecto),
		// a diferencia de `labels.formatter`, que SÍ exige un callback JS y
		// por lo tanto no sobreviviría a este json.Marshal ni al
		// <script type="application/json"> por donde viaja el config.
		tree := make([]map[string]interface{}, 0, len(elem.Data))
		for _, row := range elem.Data {
			if len(row) > 1 {
				tree = append(tree, map[string]interface{}{
					"label": row[0],
					"value": row[1],
				})
			}
		}

		dataset := make(map[string]interface{})
		dataset["label"] = "Data"
		if len(elem.Series) > 0 && elem.Series[0] != "" {
			dataset["label"] = elem.Series[0]
		}
		dataset["tree"] = tree
		dataset["key"] = "value"
		// groups NO es opcional aunque cada hoja sea única: sin él, el
		// formatter por defecto de labels solo dibuja el número — el nombre
		// de la hoja se saca del campo por el que se agrupa. Verificado en
		// Chromium contra el plugin real.
		dataset["groups"] = []string{"label"}
		// labels.display trae su propio formatter por defecto (nombre + valor
		// dentro del rectángulo), así que NO hace falta el callback JS que
		// documenta el plugin — sigue siendo config puramente serializable.
		// Su color por defecto es negro, que sobre el fondo de abajo da 6.7:1
		// de contraste (AA), así que tampoco se sobreescribe.
		dataset["labels"] = map[string]interface{}{"display": true}
		dataset["spacing"] = 0.5
		dataset["borderWidth"] = 1
		dataset["borderColor"] = "#ffffff"

		// Un solo color, NO la paleta ciclada de las demás ramas: en un
		// TreemapElement backgroundColor no es indexable — verificado en
		// Chromium, un arreglo llega crudo a options.backgroundColor y el
		// rectángulo termina sin relleno. Pintar cada hoja distinto exigiría
		// una opción "scriptable" (una función JS), que no sobrevive al
		// json.Marshal de acá ni al <script type="application/json"> por
		// donde viaja el config. Tampoco se pierde información: en un
		// treemap el dato lo codifica el ÁREA, no el color.
		dataset["backgroundColor"] = "#3498db"

		datasets = append(datasets, dataset)
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
		colors := chartCategoricalPalette(categoricalColors, defaultChartColors8)
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
				colors := chartCategoricalPalette(categoricalColors, defaultChartColors6)
				color := colors[i%len(colors)]

				if elem.ChartType == "line" {
					dataset["borderColor"] = color
					dataset["backgroundColor"] = chartAreaFillColor(color) // ~20% opacity when safely expandable
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

	if chartType == "treemap" {
		// La leyenda de un treemap solo repite el label del dataset ("Data"),
		// no discrimina nada — cada rectángulo ya trae su nombre dentro. Se
		// apaga por defecto. Quitar dataset.label en su lugar NO sirve: la
		// leyenda entonces dibuja "undefined" (verificado en Chromium).
		//
		// El merge de abajo (MergeChartOptions) es recursivo por clave, así
		// que un options.plugins.title del autor NO pisa este default: solo
		// se pierde si el autor toca options.plugins.legend directamente.
		options["plugins"] = map[string]interface{}{
			"legend": map[string]interface{}{"display": false},
		}
	}

	// Si es para export, agregar configuración optimizada para PNG/PDF
	if forExport {
		applyExportOptimizations(options)
		if chartType == "treemap" {
			// applyExportOptimizations arma scales.x/scales.y con
			// grid.display:true, pensado para los tipos cartesianos. El
			// controlador de treemap declara sus propios ejes ocultos, así
			// que dejar ese bloque puesto le pinta una rejilla y unos ticks
			// encima al treemap. Se quita acá y no dentro de la función
			// porque las escalas son lo ÚNICO que no aplica: el padding, la
			// leyenda y el título grandes sí se quieren igual.
			delete(options, "scales")
		}
	}

	// Merge con opciones del usuario si existen
	if elem.Options != nil {
		MergeChartOptions(options, elem.Options)
	}

	config["options"] = options

	// El tema se aplica AL FINAL, después del merge, y nunca antes. Un
	// hallazgo de code-review mostró por qué: aplicarlo dentro de
	// applyExportOptimizations dejaba dos ramas muertas, porque en ese
	// momento `options` todavía no tiene ni plugins.title ni scales.y1 —
	// esos los aporta el autor y llegan recién en MergeChartOptions. Correr
	// después arregla eso y, de paso, permite la semántica correcta: el
	// autor SIEMPRE gana (cada color se pone solo si la clave no existe),
	// igual que los guards `=== undefined` de charts.js en el navegador.
	if forExport {
		applyChartThemeColors(config, chartType, themeColors)
	}

	// Convertir a JSON
	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to generate chart config: %s"}`, err.Error())
	}

	return string(jsonBytes)
}

// applyExportOptimizations aplica configuración optimizada para exportación a PNG/PDF
// radialChartTypes son los tipos que NO dibujan sobre ejes cartesianos sino
// sobre una escala radial "r". Espeja CHART_SCALE_DIMENSIONS de charts.js
// (PR #228) para que el export y el navegador coincidan.
var radialChartTypes = map[string]bool{"radar": true, "polarArea": true}

// setColorIfUnset pone color en child[key] solo si nadie lo puso antes,
// creando el bloque si hace falta. Es la traducción del guard
// `=== undefined` que usa charts.js: el tema es un DEFAULT, nunca pisa lo
// que el autor escribió en su bloque options:.
func setColorIfUnset(parent map[string]interface{}, block, color string) {
	if color == "" {
		return
	}
	child, ok := parent[block].(map[string]interface{})
	if !ok {
		child = make(map[string]interface{})
		parent[block] = child
	}
	if _, exists := child["color"]; !exists {
		child["color"] = color
	}
}

// applyChartThemeColors superpone los tokens chart-* no categóricos sobre la
// config ya completa (después de MergeChartOptions). Es el equivalente
// server-side de applyExtensionChartColors en charts.js, y se escribió
// espejándolo a propósito: mantener las dos rutas convergentes por
// construcción es lo que evita que el mismo token se vea distinto en el
// navegador y en el PNG, que es la clase de divergencia que ya cerró un
// hallazgo de revisión de #224.
//
// chart-surface NO se maneja acá: no es una opción de Chart.js (el canvas es
// transparente), sino el fondo de la página — entra por buildChartHTML, ver
// RenderChartToPNGWithSurface.
func applyChartThemeColors(config map[string]interface{}, chartType string, themeColors ChartThemeColors) {
	if themeColors.IsZero() {
		return
	}
	options, ok := config["options"].(map[string]interface{})
	if !ok {
		return
	}

	// Escalas. Pueden no existir: para treemap se borran arriba a propósito,
	// y ahí no hay ninguna que pintar.
	if scales, ok := options["scales"].(map[string]interface{}); ok {
		// radar/polarArea dibujan sobre "r", que applyExportOptimizations no
		// crea (solo arma x/y). Sin esto, esos dos tipos —que SIEMPRE pasan
		// por Chromium, porque no tienen rasterizador nativo— terminaban con
		// x/y temátizados que no se ven y su escala real sin tocar (hallazgo
		// de code-review).
		if radialChartTypes[chartType] {
			if _, exists := scales["r"]; !exists {
				scales["r"] = make(map[string]interface{})
			}
		}
		for id, raw := range scales {
			scale, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			setColorIfUnset(scale, "grid", themeColors.Grid)
			setColorIfUnset(scale, "ticks", themeColors.Axis)
			setColorIfUnset(scale, "border", themeColors.Axis)
			// Una escala radial dibuja dos elementos más que una cartesiana
			// no tiene: los rayos y las etiquetas alrededor del círculo.
			// charts.js ya los pinta en el navegador; sin esto el PNG se
			// vería a medio tematizar.
			if id == "r" || radialChartTypes[chartType] {
				setColorIfUnset(scale, "angleLines", themeColors.Grid)
				setColorIfUnset(scale, "pointLabels", themeColors.Axis)
			}
		}
	}

	if themeColors.Label != "" {
		plugins, ok := options["plugins"].(map[string]interface{})
		if !ok {
			plugins = make(map[string]interface{})
			options["plugins"] = plugins
		}
		if legend, ok := plugins["legend"].(map[string]interface{}); ok {
			setColorIfUnset(legend, "labels", themeColors.Label)
		} else {
			plugins["legend"] = map[string]interface{}{
				"labels": map[string]interface{}{"color": themeColors.Label},
			}
		}
		// El título solo existe si el autor lo declaró; por eso se pinta acá
		// y no en applyExportOptimizations, que corre antes del merge y nunca
		// lo ve.
		if title, ok := plugins["title"].(map[string]interface{}); ok {
			if _, exists := title["color"]; !exists {
				title["color"] = themeColors.Label
			}
		}
		applyTreemapLabelColor(config, chartType, themeColors.Label)
	}
}

// applyTreemapLabelColor pinta las etiquetas DENTRO de los rectángulos de un
// treemap. Es el único tipo donde chart-label no llega por la leyenda: la
// config generada la apaga (solo repetiría el label del dataset) y borra las
// escalas, así que el único texto visible del chart vive en
// dataset.labels.color — cuyo default en chartjs-chart-treemap@4.2.0 (el
// bundle que fija cdn_tags.go) es "black", verificado en su fuente:
// labels:{align:"center",color:"black",display:!1,...}. Nuestro generador sí
// enciende display, así que sin esto las etiquetas quedaban negras incluso
// sobre una superficie oscura (hallazgo de code-review).
func applyTreemapLabelColor(config map[string]interface{}, chartType, label string) {
	if chartType != "treemap" {
		return
	}
	data, ok := config["data"].(map[string]interface{})
	if !ok {
		return
	}
	datasets, ok := data["datasets"].([]map[string]interface{})
	if !ok {
		return
	}
	for _, dataset := range datasets {
		setColorIfUnset(dataset, "labels", label)
	}
}

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
		fmt.Fprintf(&html, `<div class="map-title">%s</div>`, title)
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
	fmt.Fprintf(&html, `<div class="map" data-type="%s"`, mapType)

	if elem.Heatmap {
		html.WriteString(` data-heatmap="true"`)
	}

	if elem.Zoom > 0 {
		fmt.Fprintf(&html, ` data-zoom="%d"`, elem.Zoom)
	}

	if elem.Center != nil {
		fmt.Fprintf(&html, ` data-center-lat="%f" data-center-lng="%f"`,
			elem.Center.Lat, elem.Center.Lng)
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

		fmt.Fprintf(&html, `<div class="map-marker" data-lat="%f" data-lng="%f" data-label="%s"`,
			marker.Lat, marker.Lng, label)

		if marker.Value > 0 {
			fmt.Fprintf(&html, ` data-value="%f"`, marker.Value)
		}
		if marker.Color != "" {
			color := EscapeHTMLAttribute(SanitizeColor(marker.Color))
			fmt.Fprintf(&html, ` data-color="%s"`, color)
		}
		if marker.Size != "" {
			size := EscapeHTMLAttribute(marker.Size)
			fmt.Fprintf(&html, ` data-size="%s"`, size)
		}
		if details != "" {
			fmt.Fprintf(&html, ` data-details="%s"`, details)
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
		ctx.Logger.Warn("[MAP] failed to render map to assets: %v", err)
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
		ctx.Logger.Warn("[MAP] failed to render inline map: %v", err)
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
	fmt.Fprintf(&html, `<div class="alert alert-%s">`, blockType)

	if icon != "" {
		fmt.Fprintf(&html, `<span class="alert-icon">%s</span>`, icon)
	}

	if title != "" {
		fmt.Fprintf(&html, `<strong class="alert-title">%s</strong>`, title)
	}

	fmt.Fprintf(&html, `<div class="alert-content">%s</div>`, content)
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
		fmt.Fprintf(&html, `<div class="code-group-block%s" data-index="%d">`, activeClass, i)
		fmt.Fprintf(&html, `<pre><code class="language-%s">%s</code></pre>`, language, content)
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

	fmt.Fprintf(&html, `<div class="grid" data-columns="%d">`, len(elem.Columns))

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
