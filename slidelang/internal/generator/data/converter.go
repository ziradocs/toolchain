// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"fmt"
	htmltemplate "html/template"
	"strings"

	"go.ziradocs.com/slidelang/internal/generator/config"
	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/core/util"
)

// ProcessVariables es un wrapper para el renderer compartido
// Mantiene compatibilidad con código existente
func ProcessVariables(text string, variables map[string]interface{}) string {
	return renderer.ProcessVariables(text, variables)
}

// BuildVariables ensambla el mapa de sustitución {{variable}} a partir del
// FrontMatter de un AST: los built-ins documentados title/author/date/theme,
// seguidos de las variables personalizadas del usuario (que pueden
// sobrescribir un built-in con el mismo nombre). Reusado tanto por el
// generador de slides (PrepareTemplateDataWithTheme) como por --format json
// (generateJSON), para que ambas salidas expongan exactamente el mismo
// conjunto de placeholders (issue #64). Delega en ast.FrontMatterNode.
// BuildVariables (core), que doclang's renderer/document_html.go
// también usa, para que ambos DSLs expongan el mismo conjunto de built-ins
// (issue #81). Retorna nil si el AST no tiene FrontMatter.
func BuildVariables(astNode *ast.AST) map[string]interface{} {
	return astNode.FrontMatter.BuildVariables()
}

// ProcessPageNumberFormat procesa el formato de números de página con variables dinámicas
func ProcessPageNumberFormat(format string, currentPage, totalPages int, variables map[string]interface{}) string {
	if format == "" {
		return ""
	}

	// Crear mapa de variables dinámicas específicas para números de página
	pageVariables := make(map[string]interface{})

	// Copiar variables existentes
	for k, v := range variables {
		pageVariables[k] = v
	}

	// Agregar variables dinámicas de numeración
	pageVariables["current"] = currentPage
	pageVariables["total"] = totalPages

	// Procesar variables usando la función existente
	return ProcessVariables(format, pageVariables)
}

// PrepareTemplateData convierte el AST a datos para el template
func PrepareTemplateData(astNode *ast.AST, log util.Logger) PresentationData {
	return PrepareTemplateDataWithTheme(astNode, "default", log)
}

// PrepareTemplateDataWithTheme convierte el AST a datos para el template con tema
// específico, en modo browser (rendering client-side). Punto de entrada histórico;
// los callers que necesiten pre-render offline usan PrepareTemplateDataWithRenderMode.
func PrepareTemplateDataWithTheme(astNode *ast.AST, themeName string, log util.Logger) PresentationData {
	// Modo browser: RenderElementToHTML nunca toca un fetcher para ninguno de
	// sus branches "browser", así que un RenderContext default (todo en
	// "browser", sin fetchers) es válido acá — no hace falta que el caller
	// arme uno.
	return PrepareTemplateDataWithRenderMode(astNode, themeName, "browser", log, renderer.NewDefaultRenderContext())
}

// PrepareTemplateDataWithRenderMode es como PrepareTemplateDataWithTheme pero con
// un modo de rendering (issue #92). En modos offline, mermaid/chart/map se
// pre-renderizan vía renderer.RenderElementToHTML (a la que se le pasa ctx
// explícitamente, issue #134/G1a) y su HTML queda en ElementData.PreRenderedHTML;
// además se omite la metadata client-side (Charts/Diagrams/Maps), que solo
// alimenta el JS que ya no se carga en offline.

// offlineElementClassReplacer namespacea las clases que
// renderer.RenderElementToHTML (core) emite para mermaid/chart/map
// en modo offline (issue #123): esas clases no llevan el prefijo
// "slidelang-" que usa el resto de este generador (son las mismas que usa
// doclang, que no namespacea nada), así que sin esto un estilo futuro
// dirigido a ".slidelang-chart-wrapper"/etc nunca alcanzaría el HTML
// pre-renderizado offline, aunque el nombre de clase sugiera que sí.
//
// Construido A PARTIR de renderer.OfflineElementClasses (fuente única en
// core, junto a las funciones que realmente emiten estas clases)
// en vez de hand-copiar la lista acá: una copia hand-copied anterior ya
// divergió una vez (2 de ~13 entradas faltantes — las variantes
// chart-inline/map-inline —, encontrado en code-review de este mismo PR).
var offlineElementClassReplacer = buildOfflineElementClassReplacer()

func buildOfflineElementClassReplacer() *strings.Replacer {
	pairs := make([]string, 0, len(renderer.OfflineElementClasses)*2)
	for _, classes := range renderer.OfflineElementClasses {
		namespaced := namespaceClassTokens(classes)
		pairs = append(pairs, `class="`+classes+`"`, `class="`+namespaced+`"`)
	}
	return strings.NewReplacer(pairs...)
}

// namespaceClassTokens agrega el prefijo "slidelang-" a cada clase de un
// valor de atributo class (que puede traer más de una separada por espacio,
// p. ej. "chart-image chart-offline").
func namespaceClassTokens(classes string) string {
	tokens := strings.Fields(classes)
	for i, t := range tokens {
		tokens[i] = "slidelang-" + t
	}
	return strings.Join(tokens, " ")
}

func PrepareTemplateDataWithRenderMode(astNode *ast.AST, themeName, renderMode string, log util.Logger, ctx *renderer.RenderContext) PresentationData {
	offline := renderer.IsOfflineRenderMode(renderMode)
	data := PresentationData{
		HasTitle: astNode.FrontMatter != nil,
	}

	if astNode.FrontMatter != nil {
		data.Title = astNode.FrontMatter.Title
		data.Author = astNode.FrontMatter.Author
		data.Date = astNode.FrontMatter.Date
	}

	// Obtener variables del frontmatter
	variables := BuildVariables(astNode)

	// Procesar configuración de headers y footers
	if astNode.FrontMatter != nil && astNode.FrontMatter.HeaderFooter != nil {
		data.HeaderFooter = ProcessHeaderFooterConfig(astNode.FrontMatter.HeaderFooter, variables, len(astNode.ContentBlocks))
		log.Info("GEN", "🎯 Processed header/footer configuration")
	}

	// Convertir slides
	log.Info("GEN", "🎨 Converting %d slides to template data", len(astNode.ContentBlocks))

	for i, slide := range astNode.ContentBlocks {
		// Detectar elementos interactivos
		hasInteractive, interactiveElements := detectInteractiveElements(slide.Elements)

		// Extraer presenter notes de los elementos
		notes := extractPresenterNotes(slide.Elements, variables)

		slideData := SlideData{
			Type:  slide.BlockType,
			Title: ProcessVariables(slide.Title, variables),
			// Propiedades específicas para slides tipo "title"
			Heading:   ProcessVariables(slide.Heading, variables),
			Subtitle:  ProcessVariables(slide.Subtitle, variables),
			Logo:      slide.Logo,
			IsTitle:   config.IsSlideTitle(slide.BlockType),
			IsContent: config.IsSlideContent(slide.BlockType),
			// Numeración del slide
			SlideNumber: i + 1,

			// NUEVOS CAMPOS PARA VISUALIZADOR AVANZADO
			Duration:            estimateSlideDuration(slide),
			Transition:          determineTransition(slide, i),
			HasInteractive:      hasInteractive,
			InteractiveElements: interactiveElements,
			Notes:               notes,
		}

		// Procesar numeración de páginas para este slide
		if data.HeaderFooter != nil {
			slideData.DisplayNumber, slideData.ShowPageNumber = calculateSlideNumbering(
				slideData.SlideNumber,
				slideData.Type,
				data.HeaderFooter,
				len(astNode.ContentBlocks),
			)
		}

		// Procesar overrides específicos del slide
		if slide.HeaderFooterOverride != nil {
			slideData.HeaderFooterOverride = ProcessSlideHeaderFooterOverride(slide.HeaderFooterOverride, variables)
		}
		// Convertir elementos (excluyendo presenter notes que ya se extrajeron)
		for j, element := range slide.Elements {
			// Skip presenter notes elements since they're now in the Notes field
			if directive, ok := element.(*ast.DirectiveNode); ok {
				if directive.Name == "notes" || directive.Name == "notes:" {
					continue
				}
			}

			elementData := ElementData{
				Type: string(element.GetType()),
				// NUEVOS CAMPOS PARA VISUALIZADOR AVANZADO
				ElementID:  generateElementID(element, i, j),
				SlideIndex: i,
			}

			switch elem := element.(type) {
			case *ast.TextElement:
				elementData.Content = ProcessVariables(elem.Content, variables)
			case *ast.CodeElement:
				elementData.Content = ProcessVariables(elem.Content, variables)
				elementData.Language = elem.Language
			case *ast.ImageElement:
				source := ProcessVariables(elem.Source, variables)
				// Validar el esquema para prevenir javascript: y data: URIs
				// peligrosas. NO se escapa aquí: el template ahora es
				// html/template, que aplica su propio escape de atributo/URL
				// al interpolar — escapar dos veces rompería query strings
				// (ver docs/SECURITY_AUDIT_2026-07.md, CR-4).
				// Si el esquema es peligroso, Source queda vacío: html/template
				// también rechazaría por su cuenta un data: URI de placeholder
				// (su propio filtro de URL no distingue "confiable" de
				// "atacante" en un string plano), así que el template renderiza
				// un aviso en vez de un <img> cuando Source == "".
				elementData.Source = renderer.ValidateURLScheme(source)
				// Alt/Caption: sin pre-escapar, por la misma razón que Source.
				elementData.Alt = ProcessVariables(elem.Alt, variables)
				elementData.Caption = ProcessVariables(elem.Caption, variables)
				elementData.Context = string(elem.Context)
			case *ast.PointsElement:
				elementData.Items = ConvertPointItemsWithVariables(elem.Items, variables)
				elementData.ListType = elem.ListType
			case *ast.TableElement:
				// Process headers with variables and markdown formatting
				processedHeaders := make([]string, len(elem.Headers))
				for i, header := range elem.Headers {
					processedHeaders[i] = ProcessTextWithVariablesAndMarkdown(header, variables)
				}
				elementData.Headers = processedHeaders

				// Process rows with variables and markdown formatting
				processedRows := make([][]string, len(elem.Rows))
				for i, row := range elem.Rows {
					processedRow := make([]string, len(row))
					for j, cell := range row {
						processedRow[j] = ProcessTextWithVariablesAndMarkdown(cell, variables)
					}
					processedRows[i] = processedRow
				}
				elementData.Rows = processedRows
				elementData.Caption = ProcessVariables(elem.Caption, variables)
			case *ast.SpecialBlockElement:
				elementData.BlockType = elem.BlockType
				elementData.Title = ProcessVariables(elem.Title, variables)
				elementData.Content = ProcessVariables(elem.Content, variables)
				elementData.Icon = elem.Icon
			case *ast.CodeGroupElement:
				elementData.CodeBlocks = ConvertCodeBlocksWithVariables(elem.CodeBlocks, variables)
			case *ast.MermaidElement:
				elementData.DiagramType = elem.DiagramType
				elementData.Content = ProcessVariables(elem.Content, variables)
				elementData.Title = ProcessVariables(elem.Title, variables)
			case *ast.ChartElement:
				elementData.ChartType = elem.ChartType
				elementData.SeriesTypes = elem.SeriesTypes
				elementData.ChartData = elem.Data
				elementData.Series = ProcessStringArray(elem.Series, variables)
				elementData.Labels = ProcessStringArray(elem.Labels, variables)
				elementData.Options = elem.Options
				elementData.Title = ProcessVariables(elem.Title, variables)
				elementData.IsJSONMode = elem.IsJSONMode
			case *ast.MapElement:
				elementData.MapType = elem.MapType
				elementData.Markers = ConvertMapMarkersWithVariables(elem.Markers, variables)
				elementData.Heatmap = elem.Heatmap
				elementData.Zoom = elem.Zoom
				elementData.Title = ProcessVariables(elem.Title, variables)
				elementData.MapOptions = ProcessMapOptions(elem.Options, variables)
			case *ast.QuoteElement:
				elementData.Content = ProcessVariables(elem.Content, variables)
				elementData.Author = ProcessVariables(elem.Author, variables)
				elementData.Source = ProcessVariables(elem.Source, variables)
			case *ast.ChecklistElement:
				elementData.ChecklistItems = ConvertChecklistItemsWithVariables(elem.Items, variables)
			case *ast.GridElement:
				elementData.Content = ProcessVariables(elem.Content, variables)
				elementData.Columns = ConvertColumnsWithVariables(elem.Columns, variables)
			case *ast.ColumnElement:
				elementData.Content = ProcessVariables(elem.Content, variables)
			case *ast.DirectiveNode:
				elementData.Content = elem.Name

				// Aplicar parámetros de directiva
				elementData.DirectiveName = elem.Name
				elementData.DirectiveParams = elem.Parameters

				// Configurar clases CSS basadas en el tipo de directiva.
				// No incluir "directive" aquí: el template ya tiene la clase
				// estática "slidelang-directive" hardcodeada, así que
				// agregarla también vía CSSClasses duplicaba la clase.
				elementData.CSSClasses = []string{"directive-" + elem.Name}

				// Configuraciones específicas por tipo de directiva
				switch elem.Name {
				case "timer":
					elementData.Title = "Timer"
					if duration, ok := elem.Parameters["duration"]; ok {
						elementData.Content = duration.(string)
					}
					elementData.CSSClasses = append(elementData.CSSClasses, "slide-timer")

				case "transition":
					elementData.Title = "Transition"
					if transType, ok := elem.Parameters["type"]; ok {
						elementData.CSSClasses = append(elementData.CSSClasses, "transition-"+transType.(string))
					}

				case "highlight":
					elementData.Title = "Highlight"
					if color, ok := elem.Parameters["color"]; ok {
						elementData.CSSClasses = append(elementData.CSSClasses, "highlight-"+color.(string))
					} else {
						elementData.CSSClasses = append(elementData.CSSClasses, "highlight-default")
					}

				case "center":
					elementData.CSSClasses = append(elementData.CSSClasses, "text-center")

				case "fade-in":
					elementData.CSSClasses = append(elementData.CSSClasses, "animate-fade-in")

				case "slide-up":
					elementData.CSSClasses = append(elementData.CSSClasses, "animate-slide-up")

				case "bounce":
					elementData.CSSClasses = append(elementData.CSSClasses, "animate-bounce")

				case "large":
					elementData.CSSClasses = append(elementData.CSSClasses, "text-large")

				case "small":
					elementData.CSSClasses = append(elementData.CSSClasses, "text-small")

				case "spacing-wide":
					elementData.CSSClasses = append(elementData.CSSClasses, "spacing-wide")

				case "margin-large":
					elementData.CSSClasses = append(elementData.CSSClasses, "margin-large")

				case "float-left":
					elementData.CSSClasses = append(elementData.CSSClasses, "float-left")

				case "float-right":
					elementData.CSSClasses = append(elementData.CSSClasses, "float-right")

				case "auto-play":
					elementData.CSSClasses = append(elementData.CSSClasses, "auto-play")

				case "no-transition":
					elementData.CSSClasses = append(elementData.CSSClasses, "no-transition")

				case "full-screen":
					elementData.CSSClasses = append(elementData.CSSClasses, "full-screen")

				default:
					// Directiva genérica
					elementData.Title = "Directive: " + elem.Name
				}
			default:
				log.Warn("Unknown element type: %T", element)
			}

			// En modos offline, pre-renderizar mermaid/chart/map vía el pipeline
			// compartido de core (issue #92). RenderElementToHTML despacha según
			// ctx (offline-assets → <img src="assets/...">, offline-inline →
			// SVG/data-URI inline), pasado explícitamente por el caller en vez de
			// leído de un global (issue #134/G1a). El HTML se inyecta DENTRO del
			// placeholder del template (que conserva role="img" + aria).
			//
			// Se pasa una copia del elemento con Title vacío: RenderElementToHTML
			// emite su propio <div class="*-title"> si el elemento tiene título, lo
			// que duplicaría el <h2> que el template de slidelang ya renderiza. El
			// <h2> de slidelang gana (es un heading real con id, mejor para el
			// outline de accesibilidad; ver #14).
			//
			// Nota sobre sustitución de {{variables}} en el path offline: mermaid y
			// map ya la aplican DENTRO de RenderElementToHTML usando el parámetro
			// `variables` que se les pasa (renderMermaidElement hace
			// ProcessVariables(elem.Content,...), buildMapConfig hace
			// ProcessVariablesSecure en cada marker.Label/Details) — no requieren
			// nada especial aquí. Chart es la excepción: su export offline
			// (GenerateChartConfigForExport/WithMode) construye el config
			// Chart.js SOLO desde el elemento, sin recibir `variables` en
			// absoluto, a diferencia del path browser (ConvertChartElementToChartJS
			// SÍ sustituye Labels/Series vía ProcessStringArray). Sin esto, un
			// chart con labels: ["{{quarter}}"] mostraría el placeholder literal
			// en el PNG/WebP offline en vez del valor sustituido (review de PR
			// #122). Se sustituye aquí, antes de RenderElementToHTML, reusando el
			// mismo helper que ya usa el path browser.
			if offline {
				var offlineElem ast.Element
				switch e := element.(type) {
				case *ast.MermaidElement:
					cp := *e
					cp.Title = ""
					offlineElem = &cp
				case *ast.ChartElement:
					cp := *e
					cp.Title = ""
					cp.Labels = ProcessStringArray(e.Labels, variables)
					cp.Series = ProcessStringArray(e.Series, variables)
					offlineElem = &cp
				case *ast.MapElement:
					cp := *e
					cp.Title = ""
					offlineElem = &cp
				}
				if offlineElem != nil {
					elementData.PreRenderedHTML = htmltemplate.HTML(
						offlineElementClassReplacer.Replace(renderer.RenderElementToHTML(offlineElem, variables, ctx)))
				}
			}

			slideData.Elements = append(slideData.Elements, elementData)
		}

		data.ContentBlocks = append(data.ContentBlocks, slideData)
	}

	// NUEVOS CAMPOS PARA VISUALIZADOR AVANZADO
	totalSlides := len(astNode.ContentBlocks)
	features := generateFeaturesSummary(astNode.ContentBlocks)
	requiredLibraries := getRequiredLibraries(astNode.ContentBlocks)

	// En modos offline, mermaid/chart/map se hornean como <img>/SVG y no hay
	// render client-side; el blob de metadata (Charts/Diagrams/Maps) queda vacío
	// (se omite más abajo). Para que el blob no quede internamente inconsistente
	// (flags/librerías "presentes" pero arrays vacíos), se apagan las flags
	// client-side y se quitan sus librerías CDN, coherente con que el HTML ya no
	// carga esas librerías (issue #92). HasCode/HasQuotes/HasNotes se conservan.
	if offline {
		if features != nil {
			features.HasMermaid = false
			features.HasCharts = false
			features.HasMaps = false
		}
		requiredLibraries = removeInteractiveLibraries(requiredLibraries)
	}

	// Actualizar campos del visualizador avanzado
	data.Version = "2.0.0"
	data.TotalSlides = totalSlides
	data.EstimatedDuration = calculateTotalDuration(data.ContentBlocks)
	data.Features = features
	data.RequiredLibraries = requiredLibraries
	data.Theme = themeName

	// La metadata client-side (Charts/Diagrams/Maps) alimenta el JS que lee el
	// blob <script id="slidelang-metadata"> y renderiza contra los CDNs. En modos
	// offline ese JS no se carga (el contenido ya viene pre-renderizado en
	// PreRenderedHTML), así que se omite para no dejar un blob muerto (issue #92).
	if !offline {
		// Generar metadata de charts para JavaScript rendering
		data.Charts = generateChartsMetadata(astNode.ContentBlocks, variables, log)

		// Generar metadata de mermaid para JavaScript rendering
		data.Diagrams = generateMermaidMetadata(astNode.ContentBlocks, variables, log)

		// Generar metadata de maps para JavaScript rendering
		data.Maps = generateMapsMetadata(astNode.ContentBlocks, variables)
	}

	log.Info("GEN", "✅ Template data prepared successfully (%d slides, theme: %s)", len(data.ContentBlocks), themeName)
	return data
}

// removeInteractiveLibraries quita las librerías CDN de rendering client-side
// (mermaid/chartjs/leaflet) de la lista de librerías requeridas, usado en modos
// offline donde esos elementos ya vienen pre-renderizados (issue #92).
func removeInteractiveLibraries(libs []string) []string {
	interactive := map[string]bool{"mermaid": true, "chartjs": true, "leaflet": true}
	out := make([]string, 0, len(libs))
	for _, lib := range libs {
		if !interactive[lib] {
			out = append(out, lib)
		}
	}
	return out
}

// ProcessTextWithVariablesAndMarkdown es un wrapper para el renderer compartido
// Mantiene compatibilidad con código existente - AHORA USA LA VERSIÓN SEGURA
func ProcessTextWithVariablesAndMarkdown(text string, variables map[string]interface{}) string {
	return renderer.ProcessTextWithVariablesAndMarkdownSecure(text, variables)
}

// ProcessHeaderFooterConfig convierte la configuración del AST a datos del template
func ProcessHeaderFooterConfig(config *ast.HeaderFooterConfig, variables map[string]interface{}, totalSlides int) *HeaderFooterData {
	if config == nil {
		return nil
	}

	data := &HeaderFooterData{
		TotalSlides:     totalSlides,
		CountableSlides: calculateCountableSlides(totalSlides), // Por ahora igual, pero puede cambiar
	}

	// Procesar header global
	if config.Header != nil {
		data.GlobalHeader = processHeaderData(config.Header, variables)
	}

	// Procesar footer global
	if config.Footer != nil {
		data.GlobalFooter = processFooterData(config.Footer, variables)
	}

	// Procesar layout defaults
	if config.LayoutDefaults != nil {
		data.LayoutDefaults = make(map[string]*LayoutHeaderFooterData)
		for layoutName, layoutConfig := range config.LayoutDefaults {
			layoutData := &LayoutHeaderFooterData{}

			if layoutConfig.Header != nil {
				layoutData.Header = processHeaderData(layoutConfig.Header, variables)
			}
			if layoutConfig.Footer != nil {
				layoutData.Footer = processFooterData(layoutConfig.Footer, variables)
			}

			data.LayoutDefaults[layoutName] = layoutData
		}
	}

	return data
}

// processHeaderData convierte configuración de header a datos del template
func processHeaderData(config *ast.HeaderConfig, variables map[string]interface{}) *HeaderData {
	if config == nil {
		return nil
	}

	data := &HeaderData{
		Enabled:    config.Enabled,
		Height:     ProcessVariables(config.Height, variables),
		Background: ProcessVariables(config.Background, variables),
	}

	if config.Text != nil {
		data.Text = &HeaderFooterTextData{
			Left:   ProcessVariables(config.Text.Left, variables),
			Center: ProcessVariables(config.Text.Center, variables),
			Right:  ProcessVariables(config.Text.Right, variables),
		}
	}

	if config.Logo != nil {
		data.Logo = &LogoData{
			Source:   ProcessVariables(config.Logo.Source, variables),
			Alt:      ProcessVariables(config.Logo.Alt, variables),
			Height:   ProcessVariables(config.Logo.Height, variables),
			Position: config.Logo.Position,
		}
	}

	if config.Border != nil {
		data.Border = &BorderData{
			Enabled:  config.Border.Enabled,
			Color:    ProcessVariables(config.Border.Color, variables),
			Width:    ProcessVariables(config.Border.Width, variables),
			Style:    config.Border.Style,
			Position: config.Border.Position,
		}
	}

	return data
}

// processFooterData convierte configuración de footer a datos del template
func processFooterData(config *ast.FooterConfig, variables map[string]interface{}) *FooterData {
	if config == nil {
		return nil
	}

	data := &FooterData{
		Enabled:    config.Enabled,
		Height:     ProcessVariables(config.Height, variables),
		Background: ProcessVariables(config.Background, variables),
	}

	if config.Text != nil {
		data.Text = &HeaderFooterTextData{
			Left:   ProcessVariables(config.Text.Left, variables),
			Center: ProcessVariables(config.Text.Center, variables),
			Right:  ProcessVariables(config.Text.Right, variables),
		}
	}

	if config.PageNumbers != nil {
		data.PageNumbers = &PageNumbersData{
			Enabled:              config.PageNumbers.Enabled,
			Format:               ProcessVariables(config.PageNumbers.Format, variables),
			Position:             config.PageNumbers.Position,
			ExcludeTitleSlides:   config.PageNumbers.ExcludeTitleSlides,
			ExcludeClosingSlides: config.PageNumbers.ExcludeClosingSlides,
			StartFrom:            config.PageNumbers.StartFrom,
			Style:                config.PageNumbers.Style,
		}
	}

	if config.Border != nil {
		data.Border = &BorderData{
			Enabled:  config.Border.Enabled,
			Color:    ProcessVariables(config.Border.Color, variables),
			Width:    ProcessVariables(config.Border.Width, variables),
			Style:    config.Border.Style,
			Position: config.Border.Position,
		}
	}

	return data
}

// ProcessSlideHeaderFooterOverride procesa overrides específicos del slide
func ProcessSlideHeaderFooterOverride(override *ast.ContentBlockHeaderFooterOverride, variables map[string]interface{}) *SlideHeaderFooterData {
	if override == nil {
		return nil
	}

	data := &SlideHeaderFooterData{}

	if override.Header != nil {
		data.Header = processHeaderData(override.Header, variables)
	}

	if override.Footer != nil {
		data.Footer = processFooterData(override.Footer, variables)
	}

	return data
}

// calculateSlideNumbering calcula si se debe mostrar número en un slide y cuál número mostrar
func calculateSlideNumbering(slideNumber int, slideType string, headerFooterData *HeaderFooterData, totalSlides int) (displayNumber int, showNumber bool) {
	// Si no hay configuración de footer o números de página, no mostrar
	if headerFooterData == nil || headerFooterData.GlobalFooter == nil ||
		headerFooterData.GlobalFooter.PageNumbers == nil ||
		!headerFooterData.GlobalFooter.PageNumbers.Enabled {
		return 0, false
	}

	pageConfig := headerFooterData.GlobalFooter.PageNumbers

	// Verificar exclusiones por tipo de slide
	if pageConfig.ExcludeTitleSlides && isSlideTypeTitle(slideType) {
		return 0, false
	}

	if pageConfig.ExcludeClosingSlides && isSlideTypeClosing(slideType) {
		return 0, false
	}

	// Verificar si está antes del inicio
	if pageConfig.StartFrom > 0 && slideNumber < pageConfig.StartFrom {
		return 0, false
	}

	// Calcular número a mostrar
	startFrom := pageConfig.StartFrom
	if startFrom <= 0 {
		startFrom = 1
	}

	displayNumber = slideNumber - startFrom + 1
	if displayNumber <= 0 {
		return 0, false
	}

	return displayNumber, true
}

// calculateCountableSlides calcula el total de slides que cuentan para numeración
func calculateCountableSlides(totalSlides int) int {
	// Por ahora retorna el total, pero podría ser más inteligente
	// considerando exclusiones de title/closing slides
	return totalSlides
}

// isSlideTypeTitle verifica si un tipo de slide es considerado "title"
func isSlideTypeTitle(slideType string) bool {
	titleTypes := []string{"title", "cover", "intro", "presentation_title"}
	for _, titleType := range titleTypes {
		if slideType == titleType {
			return true
		}
	}
	return false
}

// isSlideTypeClosing verifica si un tipo de slide es considerado "closing"
func isSlideTypeClosing(slideType string) bool {
	closingTypes := []string{"closing", "end", "thank_you", "credits", "contact"}
	for _, closingType := range closingTypes {
		if slideType == closingType {
			return true
		}
	}
	return false
}

// === FUNCIONES AUXILIARES PARA VISUALIZADOR AVANZADO ===

// detectInteractiveElements analiza los elementos de un slide para detectar elementos interactivos
func detectInteractiveElements(elements []ast.Element) (bool, []string) {
	interactiveTypes := []string{}

	for _, elem := range elements {
		switch e := elem.(type) {
		case *ast.MermaidElement:
			interactiveTypes = append(interactiveTypes, "mermaid")
		case *ast.ChartElement:
			interactiveTypes = append(interactiveTypes, "chart")
		case *ast.MapElement:
			interactiveTypes = append(interactiveTypes, "map")
		case *ast.CodeElement:
			if strings.HasPrefix(strings.ToLower(e.Language), "mermaid") {
				interactiveTypes = append(interactiveTypes, "mermaid")
			} else {
				interactiveTypes = append(interactiveTypes, "code")
			}
		case *ast.SpecialBlockElement:
			switch strings.ToLower(e.BlockType) {
			case "mermaid", "diagram":
				interactiveTypes = append(interactiveTypes, "mermaid")
			case "chart", "charts":
				interactiveTypes = append(interactiveTypes, "chart")
			case "map", "maps":
				interactiveTypes = append(interactiveTypes, "map")
			case "code-group", "codegroup":
				interactiveTypes = append(interactiveTypes, "code")
			case "details", "collapsible":
				interactiveTypes = append(interactiveTypes, "interactive")
			}
		case *ast.QuoteElement:
			interactiveTypes = append(interactiveTypes, "quote")
		}
	}

	// Remover duplicados
	uniqueTypes := make([]string, 0)
	seen := make(map[string]bool)
	for _, t := range interactiveTypes {
		if !seen[t] {
			uniqueTypes = append(uniqueTypes, t)
			seen[t] = true
		}
	}

	return len(uniqueTypes) > 0, uniqueTypes
}

// estimateSlideDuration calcula la duración estimada de un slide en segundos
func estimateSlideDuration(slide ast.ContentBlock) int {
	baseDuration := 30 // 30 segundos por defecto

	switch slide.Type {
	case "title":
		baseDuration = 10
	case "section":
		baseDuration = 15
	case "closing":
		baseDuration = 20
	}

	// Contar palabras y elementos complejos
	wordCount := 0
	hasComplexElements := false

	for _, elem := range slide.Elements {
		switch e := elem.(type) {
		case *ast.TextElement:
			wordCount += len(strings.Fields(e.Content))
		case *ast.PointsElement:
			// Contar palabras en los puntos
			for _, item := range e.Items {
				wordCount += countWordsInPointItem(item)
			}
		case *ast.MermaidElement, *ast.ChartElement, *ast.MapElement:
			hasComplexElements = true
		case *ast.SpecialBlockElement:
			if e.BlockType == "mermaid" || e.BlockType == "chart" || e.BlockType == "map" {
				hasComplexElements = true
			}
			wordCount += len(strings.Fields(e.Content))
		}
	}

	// Calcular tiempo de lectura (200 palabras por minuto)
	readingTime := 15 // mínimo 15 segundos
	if wordCount > 0 {
		readingTime = (wordCount * 60) / 200
	}

	// Agregar tiempo extra por elementos complejos
	if hasComplexElements {
		baseDuration += 15
	}

	// Retornar el mayor entre duración base y tiempo de lectura
	if readingTime > baseDuration {
		return readingTime
	}
	return baseDuration
}

// countWordsInPointItem cuenta palabras recursivamente en items de puntos
func countWordsInPointItem(item ast.PointItem) int {
	count := len(strings.Fields(item.Content))
	for _, subItem := range item.SubPoints {
		count += countWordsInPointItem(subItem)
	}
	return count
}

// generateFeaturesSummary analiza todos los slides para generar un resumen de características
func generateFeaturesSummary(slides []ast.ContentBlock) *PresentationFeatures {
	features := &PresentationFeatures{}

	for _, slide := range slides {
		for _, elem := range slide.Elements {
			switch e := elem.(type) {
			case *ast.MermaidElement:
				features.HasMermaid = true
			case *ast.ChartElement:
				features.HasCharts = true
			case *ast.MapElement:
				features.HasMaps = true
			case *ast.CodeElement:
				if strings.HasPrefix(strings.ToLower(e.Language), "mermaid") {
					features.HasMermaid = true
				} else {
					features.HasCode = true
				}
			case *ast.QuoteElement:
				features.HasQuotes = true
			case *ast.DirectiveNode:
				// Detectar notas del presentador
				if e.Name == "notes" || e.Name == "notes:" {
					features.HasNotes = true
				}
			case *ast.SpecialBlockElement:
				switch strings.ToLower(e.BlockType) {
				case "mermaid", "diagram":
					features.HasMermaid = true
				case "chart", "charts":
					features.HasCharts = true
				case "map", "maps":
					features.HasMaps = true
				case "code-group", "codegroup":
					features.HasCode = true
				}
			}
		}
	}

	return features
}

// getRequiredLibraries determina qué librerías JavaScript se necesitan
func getRequiredLibraries(slides []ast.ContentBlock) []string {
	libraries := []string{}
	seen := make(map[string]bool)

	for _, slide := range slides {
		for _, elem := range slide.Elements {
			switch e := elem.(type) {
			case *ast.MermaidElement:
				if !seen["mermaid"] {
					libraries = append(libraries, "mermaid")
					seen["mermaid"] = true
				}
			case *ast.ChartElement:
				if !seen["chartjs"] {
					libraries = append(libraries, "chartjs")
					seen["chartjs"] = true
				}
			case *ast.MapElement:
				if !seen["leaflet"] {
					libraries = append(libraries, "leaflet")
					seen["leaflet"] = true
				}
			case *ast.CodeElement:
				if strings.HasPrefix(strings.ToLower(e.Language), "mermaid") && !seen["mermaid"] {
					libraries = append(libraries, "mermaid")
					seen["mermaid"] = true
				}
			case *ast.SpecialBlockElement:
				switch strings.ToLower(e.BlockType) {
				case "mermaid", "diagram":
					if !seen["mermaid"] {
						libraries = append(libraries, "mermaid")
						seen["mermaid"] = true
					}
				case "chart", "charts":
					if !seen["chartjs"] {
						libraries = append(libraries, "chartjs")
						seen["chartjs"] = true
					}
				case "map", "maps":
					if !seen["leaflet"] {
						libraries = append(libraries, "leaflet")
						seen["leaflet"] = true
					}
				}
			}
		}
	}

	return libraries
}

// calculateTotalDuration suma las duraciones de todos los slides
func calculateTotalDuration(slides []SlideData) int {
	total := 0
	for _, slide := range slides {
		total += slide.Duration
	}
	return total
}

// determineTransition determina el tipo de transición para un slide
func determineTransition(slide ast.ContentBlock, index int) string {
	// Por defecto usar "fade", pero se puede personalizar según el tipo o configuración
	switch slide.Type {
	case "title", "section":
		return "fade"
	case "closing":
		return "fade"
	default:
		return "fade"
	}
}

// generateElementID genera un ID único para un elemento
func generateElementID(elem ast.Element, slideIndex, elementIndex int) string {
	// Solo devolvemos el índice del elemento, el template se encarga del resto
	return fmt.Sprintf("%d", elementIndex)
}

// max retorna el mayor de dos enteros
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// extractPresenterNotes extracts presenter notes from slide elements
func extractPresenterNotes(elements []ast.Element, variables map[string]interface{}) []string {
	var notes []string

	for _, element := range elements {
		if directive, ok := element.(*ast.DirectiveNode); ok {
			if directive.Name == "notes" || directive.Name == "notes:" {
				var noteContent string
				if content, exists := directive.Parameters["content"]; exists {
					noteContent = content.(string)
				}
				// Process variables in note content
				processedNote := ProcessVariables(noteContent, variables)
				if processedNote != "" {
					notes = append(notes, processedNote)
				}
			}
		}
	}

	return notes
}

// ProcessStringArray procesa variables en un array de strings
func ProcessStringArray(stringArray []string, variables map[string]interface{}) []string {
	if stringArray == nil {
		return nil
	}

	result := make([]string, len(stringArray))
	for i, str := range stringArray {
		result[i] = ProcessVariables(str, variables)
	}
	return result
}

// Chart conversion functions

// ChartJSConfig representa una configuración completa de Chart.js
type ChartJSConfig struct {
	Type    string                 `json:"type"`
	Data    ChartJSData            `json:"data"`
	Options map[string]interface{} `json:"options"`
}

// ChartJSData representa la estructura de datos de Chart.js
type ChartJSData struct {
	Labels   []string         `json:"labels"`
	Datasets []ChartJSDataset `json:"datasets"`
}

// ChartJSDataset representa un dataset de Chart.js
type ChartJSDataset struct {
	Label           string        `json:"label"`
	Data            []interface{} `json:"data"`
	Type            string        `json:"type,omitempty"`
	BackgroundColor interface{}   `json:"backgroundColor,omitempty"`
	BorderColor     interface{}   `json:"borderColor,omitempty"`
	BorderWidth     int           `json:"borderWidth,omitempty"`
	YAxisID         string        `json:"yAxisID,omitempty"`
	Tension         float64       `json:"tension,omitempty"`
}

// ConvertChartElementToChartJS convierte un ChartElement del AST a configuración Chart.js
func ConvertChartElementToChartJS(chart *ast.ChartElement, elementID string, variables map[string]interface{}) ChartJSConfig {
	config := ChartJSConfig{
		Type: convertChartType(chart.ChartType),
		Data: ChartJSData{
			Labels:   ProcessStringArray(chart.Labels, variables),
			Datasets: []ChartJSDataset{},
		},
		Options: make(map[string]interface{}),
	}

	// Configurar opciones básicas
	config.Options["responsive"] = true
	config.Options["maintainAspectRatio"] = false

	// Si es un chart combo, cambiar a tipo 'bar' y configurar escalas
	if chart.ChartType == "combo" {
		config.Type = "bar"
		config.Options["scales"] = createComboChartScales()
	}

	// Procesar datasets
	if len(chart.Series) > 0 {
		config.Data.Datasets = createDatasetsFromSeries(chart, variables)
	} else if len(chart.Data) > 0 {
		config.Data.Datasets = createDatasetsFromData(chart, variables)
	}

	// Si no hay labels explícitos pero hay datos, extraer labels de los datos
	if len(config.Data.Labels) == 0 && len(chart.Data) > 0 {
		config.Data.Labels = extractLabelsFromData(chart.Data)
	}

	// Agregar opciones personalizadas
	if chart.Options != nil {
		mergeOptions(config.Options, chart.Options)
	}

	// Agregar título si existe
	if chart.Title != "" {
		processedTitle := ProcessVariables(chart.Title, variables)
		plugins, exists := config.Options["plugins"].(map[string]interface{})
		if !exists {
			plugins = make(map[string]interface{})
			config.Options["plugins"] = plugins
		}
		plugins["title"] = map[string]interface{}{
			"display": true,
			"text":    processedTitle,
		}
	}

	return config
}

// convertChartType convierte tipos de chart SlideLang a Chart.js
func convertChartType(chartType string) string {
	switch chartType {
	case "combo":
		return "bar" // Chart.js usa 'bar' para mixed charts
	case "doughnut", "pie", "line", "bar", "area", "scatter", "bubble", "polarArea", "radar":
		return chartType
	default:
		return "bar" // fallback
	}
}

// createComboChartScales crea la configuración de escalas para combo charts
func createComboChartScales() map[string]interface{} {
	return map[string]interface{}{
		"y": map[string]interface{}{
			"beginAtZero": true,
			"position":    "left",
		},
		"y1": map[string]interface{}{
			"type":        "linear",
			"display":     true,
			"position":    "right",
			"beginAtZero": true,
			"grid": map[string]interface{}{
				"drawOnChartArea": false,
			},
		},
	}
}

// createDatasetsFromSeries crea datasets cuando hay series definidas
func createDatasetsFromSeries(chart *ast.ChartElement, variables map[string]interface{}) []ChartJSDataset {
	datasets := make([]ChartJSDataset, 0, len(chart.Series))

	for i, seriesName := range chart.Series {
		processedName := ProcessVariables(seriesName, variables)

		// Extraer datos para esta serie (columna i+1 de todas las filas)
		data := make([]interface{}, 0)
		for _, row := range chart.Data {
			if len(row) > i+1 {
				data = append(data, row[i+1])
			}
		}

		dataset := ChartJSDataset{
			Label:       processedName,
			Data:        data,
			BorderWidth: 2,
			Tension:     0.1,
		}

		// Para combo charts, asignar tipo específico y escala Y
		if chart.ChartType == "combo" && len(chart.SeriesTypes) > i {
			dataset.Type = chart.SeriesTypes[i]
			if i > 0 { // Segunda serie y siguientes usan escala Y secundaria
				dataset.YAxisID = "y1"
			}
		}

		datasets = append(datasets, dataset)
	}

	return datasets
}

// createDatasetsFromData crea datasets cuando solo hay datos sin series
func createDatasetsFromData(chart *ast.ChartElement, variables map[string]interface{}) []ChartJSDataset {
	if len(chart.Data) == 0 {
		return []ChartJSDataset{}
	}

	data := make([]interface{}, 0)

	// Estándar Chart.js: para pie/doughnut, los datos vienen como array simple
	// Los datos ya vienen parseados correctamente como [[890, 640, 390, 280]]
	// Simplemente extraemos todos los valores
	if len(chart.Data) == 1 {
		// Data inline: [890, 640, 390, 280] -> usar todos los valores
		data = append(data, chart.Data[0]...)
	} else {
		// Datos tabulares: extraer segunda columna (primera es label)
		for _, row := range chart.Data {
			if len(row) > 1 {
				data = append(data, row[1])
			} else if len(row) == 1 {
				data = append(data, row[0])
			}
		}
	}

	// Crear dataset estándar Chart.js
	dataset := ChartJSDataset{
		Data:        data,
		BorderWidth: 2,
		Tension:     0.1,
	}

	// Solo agregar label si no es pie/doughnut (Chart.js estándar)
	if chart.ChartType != "pie" && chart.ChartType != "doughnut" {
		dataset.Label = "Dataset 1"
	}

	return []ChartJSDataset{dataset}
}

// extractLabelsFromData extrae labels de la primera columna de datos
func extractLabelsFromData(data [][]interface{}) []string {
	labels := make([]string, 0, len(data))
	for _, row := range data {
		if len(row) > 0 {
			labels = append(labels, fmt.Sprintf("%v", row[0]))
		}
	}
	return labels
}

// mergeOptions combina opciones personalizadas con la configuración base
func mergeOptions(target, source map[string]interface{}) {
	for key, value := range source {
		if existingValue, exists := target[key]; exists {
			// Si ambos valores son mapas, hacer merge recursivo
			if existingMap, ok := existingValue.(map[string]interface{}); ok {
				if sourceMap, ok := value.(map[string]interface{}); ok {
					mergeOptions(existingMap, sourceMap)
					continue
				}
			}
		}

		// Manejar callbacks especiales de Chart.js
		if key == "callbacks" {
			if callbacksMap, ok := value.(map[string]interface{}); ok {
				processedCallbacks := make(map[string]interface{})
				for cbKey, cbValue := range callbacksMap {
					if cbString, ok := cbValue.(string); ok {
						// Marcar como función JavaScript para que el template la procese
						processedCallbacks[cbKey] = map[string]interface{}{
							"_function": true,
							"body":      cbString,
						}
					} else {
						processedCallbacks[cbKey] = cbValue
					}
				}
				target[key] = processedCallbacks
				continue
			}
		}

		target[key] = value
	}
}

// generateChartsMetadata extrae todos los charts de los slides y genera metadata Chart.js
func generateChartsMetadata(slides []ast.ContentBlock, variables map[string]interface{}, log util.Logger) []ChartMetadata {
	var charts []ChartMetadata

	for slideIndex, slide := range slides {
		for elementIndex, element := range slide.Elements {
			if chartElement, ok := element.(*ast.ChartElement); ok {
				elementID := generateElementID(element, slideIndex, elementIndex)
				canvasID := fmt.Sprintf("slidelang-element-chart-%d-%s", slideIndex, elementID)

				var metadata ChartMetadata
				usedRawJSON := false

				// Modo JSON directo: usar la config tal cual, sin reconstruirla
				// desde Data/Series. renderer.ResolveChartJSONMode es la misma
				// fuente de verdad que usa doclang (renderer.renderChartElement)
				// para esta resolución — issue #55: antes cada CLI la
				// reimplementaba de forma independiente, causando el bug
				// histórico #11 (el mismo chart en modo JSON directo se
				// renderizaba distinto entre los dos DSLs).
				if rawConfig, chartType, err := renderer.ResolveChartJSONMode(chartElement); err != nil {
					log.Warn("Chart en modo JSON con RawJSON inválido, reconstruyendo desde Data/Series: %v", err)
				} else if rawConfig != nil {
					metadata = ChartMetadata{
						ID:     canvasID,
						Type:   chartType,
						Config: rawConfig,
					}
					usedRawJSON = true
				}

				if !usedRawJSON {
					// Convertir ChartElement a configuración Chart.js
					chartConfig := ConvertChartElementToChartJS(chartElement, elementID, variables)

					metadata = ChartMetadata{
						ID:   canvasID,
						Type: chartConfig.Type,
						Config: map[string]interface{}{
							"type":    chartConfig.Type,
							"data":    chartConfig.Data,
							"options": chartConfig.Options,
						},
					}
				}

				charts = append(charts, metadata)
			}
		}
	}

	return charts
}

// generateMermaidMetadata extrae todos los diagramas Mermaid de los slides y genera metadata
func generateMermaidMetadata(slides []ast.ContentBlock, variables map[string]interface{}, log util.Logger) []MermaidMetadata {
	var diagrams []MermaidMetadata

	for slideIndex, slide := range slides {
		for elementIndex, element := range slide.Elements {
			if mermaidElement, ok := element.(*ast.MermaidElement); ok {
				elementID := generateElementID(element, slideIndex, elementIndex)
				diagramID := fmt.Sprintf("slidelang-element-mermaid-%d-%s", slideIndex, elementID)

				// Process content with variables
				processedContent := ProcessVariables(mermaidElement.Content, variables)

				// DEBUG: Log del contenido del diagrama
				// Crear metadata
				metadata := MermaidMetadata{
					ID:          diagramID,
					DiagramType: mermaidElement.DiagramType,
					Content:     processedContent,
					Title:       ProcessVariables(mermaidElement.Title, variables),
				}

				diagrams = append(diagrams, metadata)
			}
		}
	}

	return diagrams
}

// generateMapsMetadata extrae todos los mapas de los slides y genera metadata
func generateMapsMetadata(slides []ast.ContentBlock, variables map[string]interface{}) []MapMetadata {
	var maps []MapMetadata

	for slideIndex, slide := range slides {
		for elementIndex, element := range slide.Elements {
			if mapElement, ok := element.(*ast.MapElement); ok {
				elementID := generateElementID(element, slideIndex, elementIndex)
				mapID := fmt.Sprintf("slidelang-element-map-%d-%s", slideIndex, elementID)

				// Convertir marcadores con variables
				markers := ConvertMapMarkersWithVariables(mapElement.Markers, variables)

				// Crear metadata
				metadata := MapMetadata{
					ID:      mapID,
					MapType: mapElement.MapType,
					Markers: markers,
					Options: ProcessMapOptions(mapElement.Options, variables),
					Title:   ProcessVariables(mapElement.Title, variables),
					Heatmap: mapElement.Heatmap,
					Zoom:    mapElement.Zoom,
				}

				maps = append(maps, metadata)
			}
		}
	}

	return maps
}
