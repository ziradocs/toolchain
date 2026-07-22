// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"regexp"
	"strings"

	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/core/util"
	"go.ziradocs.com/slidelang/internal/generator/css"
)

// TemplateBuilder construye templates HTML de forma modular usando el nuevo sistema CSS
type TemplateBuilder struct {
	Theme       string
	CustomCSS   string
	CustomJS    string
	Responsive  bool
	Minify      bool
	EmbedAssets bool     // Controla si CSS/JS se embeben o son archivos separados
	Modules     []string // Módulos a incluir
	// Modular configuration for CSS
	RequiredElements []string // Elements actually used in the presentation
	RequiredLayouts  []string // Layouts actually used in the presentation
	EnableNavigation bool     // Whether to include navigation CSS
	EnableUtilities  bool     // Whether to include utilities CSS
	// RenderMode controla la salida de librerías CDN (issue #92): en modos offline
	// (offline-assets/offline-inline) los diagramas/charts/mapas se pre-renderizan,
	// así que no se emiten los <script>/<link> de mermaid/chart.js/leaflet.
	RenderMode string
	// Note: CSS namespacing is always enabled by default
}

// NewTemplateBuilder crea un nuevo builder con configuración por defecto
func NewTemplateBuilder() *TemplateBuilder {
	return &TemplateBuilder{
		Theme:            "default",
		Responsive:       true,
		Minify:           false,
		EmbedAssets:      false,            // Por defecto, archivos separados
		Modules:          []string{"core"}, // Por defecto solo core
		RequiredElements: []string{"text"}, // Default to text only
		RequiredLayouts:  []string{},       // No layouts by default
		EnableNavigation: true,             // Default enable navigation
		EnableUtilities:  true,             // Default enable utilities
		// Note: CSS namespacing is always enabled by default
	}
}

// WithTheme establece el tema a usar
func (tb *TemplateBuilder) WithTheme(theme string) *TemplateBuilder {
	tb.Theme = theme
	return tb
}

// WithCustomCSS añade CSS personalizado
func (tb *TemplateBuilder) WithCustomCSS(css string) *TemplateBuilder {
	tb.CustomCSS = css
	return tb
}

// WithCustomJS añade JavaScript personalizado
func (tb *TemplateBuilder) WithCustomJS(js string) *TemplateBuilder {
	tb.CustomJS = js
	return tb
}

// WithResponsive habilita/deshabilita CSS responsive
func (tb *TemplateBuilder) WithResponsive(enabled bool) *TemplateBuilder {
	tb.Responsive = enabled
	return tb
}

// WithMinify habilita/deshabilita minificación
func (tb *TemplateBuilder) WithMinify(enabled bool) *TemplateBuilder {
	tb.Minify = enabled
	return tb
}

// WithEmbedAssets configura si los assets se embeben o son archivos separados
func (tb *TemplateBuilder) WithEmbedAssets(embed bool) *TemplateBuilder {
	tb.EmbedAssets = embed
	return tb
}

// WithModules configura qué módulos incluir
func (tb *TemplateBuilder) WithModules(modules []string) *TemplateBuilder {
	tb.Modules = modules
	return tb
}

// WithRequiredElements sets the elements that are actually used in the presentation
func (tb *TemplateBuilder) WithRequiredElements(elements []string) *TemplateBuilder {
	tb.RequiredElements = elements
	return tb
}

// WithRequiredLayouts sets the layouts that are actually used in the presentation
func (tb *TemplateBuilder) WithRequiredLayouts(layouts []string) *TemplateBuilder {
	tb.RequiredLayouts = layouts
	return tb
}

// WithNavigation enables/disables navigation CSS
func (tb *TemplateBuilder) WithNavigation(enabled bool) *TemplateBuilder {
	tb.EnableNavigation = enabled
	return tb
}

// WithUtilities enables/disables utilities CSS
func (tb *TemplateBuilder) WithUtilities(enabled bool) *TemplateBuilder {
	tb.EnableUtilities = enabled
	return tb
}

// WithRenderMode establece el modo de rendering (issue #92). En modos offline no
// se emiten las librerías CDN de mermaid/chart.js/leaflet.
func (tb *TemplateBuilder) WithRenderMode(mode string) *TemplateBuilder {
	tb.RenderMode = mode
	return tb
}

// isOffline indica si el builder está en un modo de rendering offline.
func (tb *TemplateBuilder) isOffline() bool {
	return renderer.IsOfflineRenderMode(tb.RenderMode)
}

// namespaceClass adds the slidelang- prefix to a class name (always enabled)
func (tb *TemplateBuilder) namespaceClass(className string) string {
	// Don't namespace classes that already have the prefix
	if strings.HasPrefix(className, "slidelang-") {
		return className
	}
	return "slidelang-" + className
}

// namespaceClasses adds the slidelang- prefix to multiple class names (always enabled)
func (tb *TemplateBuilder) namespaceClasses(classNames string) string {
	classes := strings.Fields(classNames)
	var namespacedClasses []string

	for _, class := range classes {
		namespacedClasses = append(namespacedClasses, tb.namespaceClass(class))
	}

	return strings.Join(namespacedClasses, " ")
}

// Build construye el template HTML completo usando el nuevo sistema CSS modular
func (tb *TemplateBuilder) Build() string {
	var html strings.Builder

	// Nonce único para este build: autoriza en la CSP tanto el <meta> emitido
	// por buildHTMLHead como cada <style>/<script> inline que este método
	// escribe más abajo (ver docs/SECURITY_AUDIT_2026-07.md, BA-10). Si la
	// generación falla (solo posible por una fuente de entropía rota del
	// sistema), se degrada a servir sin CSP en vez de romper el build.
	cspNonce, nonceErr := renderer.GenerateCSPNonce()
	if nonceErr != nil {
		util.Debug("CSP", "failed to generate nonce, omitting CSP: %v", nonceErr)
		cspNonce = ""
	}
	// scriptNonceAttr NO se usa en <style>: style-src usa 'unsafe-inline'
	// (no nonce) porque Mermaid inyecta su CSS de tema en runtime vía un
	// <style> sin nonce y sin forma de asignarle uno — un style-src con
	// nonce lo bloquearía silenciosamente y rompería el render (verificado
	// en vivo). Ver el comentario de BuildDefaultOutputCSP.
	scriptNonceAttr := ""
	if cspNonce != "" {
		scriptNonceAttr = fmt.Sprintf(" nonce=%q", cspNonce)
	}

	// HTML Head
	html.WriteString(tb.buildHTMLHead(cspNonce))

	if tb.EmbedAssets {
		// Modo embebido (comportamiento original)
		html.WriteString("<style>\n")

		util.Debug("CSS", "Building CSS with required modules")
		// RequiredElements/RequiredLayouts/EnableNavigation/EnableUtilities se
		// omitían acá (el comentario decía "TODOS LOS MÓDULOS INCLUIDOS POR
		// DEFECTO", pero con estos 4 campos en su cero-valor,
		// CSSBuilder.Build() nunca cargaba CSS de elementos (checklists,
		// tablas, código, ...) ni de navigation.css en NINGÚN build embebido
		// - issue #128: --format pdf fuerza EmbedAssets y expuso el hueco al
		// dejar el contador/ayuda/menú de navegación sin ningún estilo
		// (ni position:fixed ni las reglas de @media print que los ocultan)
		// en vez de "todos los módulos"). Se pasa lo mismo que ya usa el
		// modo no-embebido (mismos campos que createTemplateBuilder llenó
		// en tb), para tener paridad entre ambos modos.
		cssConfig := css.CSSConfig{
			Theme:            tb.Theme,
			CustomCSS:        tb.CustomCSS,
			Responsive:       tb.Responsive,
			Minify:           tb.Minify,
			RequiredElements: tb.RequiredElements,
			RequiredLayouts:  tb.RequiredLayouts,
			EnableNavigation: tb.EnableNavigation,
			EnableUtilities:  tb.EnableUtilities,
			// Note: CSS namespacing is always enabled by default
		}

		generatedCSS, err := css.GenerateCSS(cssConfig)
		if err != nil {
			// Fallback to default if there's an error
			util.Debug("CSS", "CSS Generation Error: %v", err)
			util.Debug("CSS", "Falling back to default CSS config")
			defaultConfig := css.DefaultCSSConfig()
			generatedCSS, _ = css.GenerateCSS(defaultConfig)
		}

		util.Debug("CSS", "Generated CSS length: %d bytes", len(generatedCSS))

		html.WriteString(generatedCSS)
		html.WriteString("</style>\n")
	} else {
		// Modo archivos separados
		html.WriteString(`    <link rel="stylesheet" href="reset.css">` + "\n")
		html.WriteString(`    <link rel="stylesheet" href="presentation.css">` + "\n")

		// Incluir archivos CSS modulares si existen
		for _, module := range tb.Modules {
			switch module {
			case "navigation":
				html.WriteString(`    <link rel="stylesheet" href="navigation.css">` + "\n")
			case "responsive":
				html.WriteString(`    <link rel="stylesheet" href="responsive.css">` + "\n")
			}
		}
	}

	// External CDN libraries
	html.WriteString(tb.buildCDNIncludes())

	html.WriteString("</head>\n<body>\n")

	// HTML Body structure
	html.WriteString(tb.buildHTMLBody())

	// Append element template definitions for template processing
	html.WriteString("\n")
	html.WriteString(tb.GetElementTemplate())

	if tb.EmbedAssets {
		// JavaScript embebido usando la misma lógica modular que archivos separados
		html.WriteString("<script" + scriptNonceAttr + ">\n")

		// Usar la misma función modular que se usa para archivos separados
		html.WriteString(tb.BuildJSWithModules(tb.Modules))

		html.WriteString("</script>\n")
	} else {
		// JavaScript como archivo separado
		html.WriteString(`    <script src="presentation.js"></script>` + "\n")

		// Incluir archivos JS modulares si existen
		for _, module := range tb.Modules {
			switch module {
			case "navigation":
				html.WriteString(`    <script src="navigation.js"></script>` + "\n")
			case "utilities":
				html.WriteString(`    <script src="utilities.js"></script>` + "\n")
			case "mermaid":
				html.WriteString(`    <script src="mermaid.js"></script>` + "\n")
			case "charts":
				html.WriteString(`    <script src="charts.js"></script>` + "\n")
			case "maps":
				html.WriteString(`    <script src="maps.js"></script>` + "\n")
			}
		}
	}

	html.WriteString("</body>\n</html>")

	return html.String()
}

// BuildCSS genera el CSS como contenido separado para guardar en archivo
func (tb *TemplateBuilder) BuildCSS() (string, error) {
	util.Debug("CSS", "Building CSS with modular configuration")

	// Detectar si responsive debería ser modular
	modularResponsive := false
	for _, module := range tb.Modules {
		if module == "responsive" {
			modularResponsive = true
			break
		}
	}

	cssConfig := css.CSSConfig{
		Theme:             tb.Theme,
		CustomCSS:         tb.CustomCSS,
		Responsive:        tb.Responsive,
		ModularResponsive: modularResponsive,
		Minify:            tb.Minify,
		RequiredElements:  tb.RequiredElements,
		RequiredLayouts:   tb.RequiredLayouts,
		// EnableNavigation/EnableUtilities se fuerzan a false acá: BuildCSS()
		// arma presentation.css para el modo NO embebido, donde
		// navigation.css ya se escribe como archivo separado propio
		// (NavigationModuleGenerator.GenerateAssets, generateModularAssetsRefactored)
		// y se linkea aparte. tb.EnableNavigation sigue gateando si ese
		// archivo separado se genera; acá solo evita que su contenido se
		// duplique DENTRO de presentation.css (CSSBuilder.Build() antes
		// ignoraba este campo por completo - issue #128 lo hizo efectivo
		// para el modo embebido, ver el otro cssConfig en Build() más abajo).
		EnableNavigation: false,
		EnableUtilities:  false,
		// Note: CSS namespacing is always enabled by default
	}

	generatedCSS, err := css.GenerateCSS(cssConfig)
	if err != nil {
		// Fallback to default if there's an error
		util.Debug("CSS", "CSS Generation Error: %v", err)
		util.Debug("CSS", "Falling back to default CSS config")
		defaultConfig := css.DefaultCSSConfig()
		generatedCSS, _ = css.GenerateCSS(defaultConfig)
	}

	// Add debug information
	debugInfo := fmt.Sprintf("/* === DEBUG FROM TEMPLATE BUILDER === */\n/* Required Elements: %v */\n/* Theme: %s */\n\n", tb.RequiredElements, tb.Theme)

	return debugInfo + generatedCSS, nil
}

// BuildJS genera el JavaScript como contenido separado para guardar en archivo
func (tb *TemplateBuilder) BuildJS() string {
	return tb.BuildJSWithModules(tb.Modules)
}

// BuildJSWithModules genera el JavaScript con módulos específicos y aplica namespacing
func (tb *TemplateBuilder) BuildJSWithModules(modules []string) string {
	var js strings.Builder

	// Incluir módulos según la lista
	for _, module := range modules {
		var moduleJS string
		switch module {
		case "core":
			moduleJS = GetCoreJS()
		case "navigation":
			moduleJS = GetNavigationJS()
		case "utilities":
			moduleJS = GetUtilitiesJS()
		case "mermaid":
			moduleJS = GetMermaidJS()
		case "charts":
			moduleJS = GetChartsJS()
		case "maps":
			moduleJS = GetMapsJS()
		case "directives":
			moduleJS = GetDirectivesJS()
		}

		// Apply namespacing to JavaScript selectors
		namespacedJS := tb.namespaceJavaScriptSelectors(moduleJS)
		js.WriteString(namespacedJS)
	}

	// JavaScript personalizado (también aplicar namespacing)
	if tb.CustomJS != "" {
		namespacedCustomJS := tb.namespaceJavaScriptSelectors(tb.CustomJS)
		js.WriteString(namespacedCustomJS)
	}

	// Inicialización (también aplicar namespacing)
	initJS := GetInitJS(modules)
	namespacedInitJS := tb.namespaceJavaScriptSelectors(initJS)
	js.WriteString(namespacedInitJS)

	return js.String()
}

func (tb *TemplateBuilder) buildHTMLHead(cspNonce string) string {
	head := `<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
`
	if cspNonce != "" {
		head += fmt.Sprintf("    <meta http-equiv=\"Content-Security-Policy\" content=\"%s\">\n", renderer.BuildDefaultOutputCSP(cspNonce))
	}
	head += `    <title>{{.Title}}{{if not .Title}}SlideLang Presentation{{end}}</title>
`
	return head
}

func (tb *TemplateBuilder) buildCDNIncludes() string {
	// En modos offline, mermaid/chart/map se pre-renderizan en build time, así que
	// las librerías CDN no se usan y no se emiten (issue #92). El contenido ya viene
	// como <img>/SVG/data-URI autocontenido, sin dependencia de red en runtime.
	if tb.isOffline() {
		return ""
	}

	// Incluir TODAS las librerías CDN por defecto, con Subresource Integrity (SRI)
	// para que un CDN comprometido no pueda inyectar contenido arbitrario en la página.
	// Los hashes están atados a la URL versionada exacta; si se bumpea una versión,
	// hay que recomputar el hash correspondiente (sha384, base64) contra la nueva URL.
	return `    <script src="https://cdn.jsdelivr.net/npm/mermaid@11.4.0/dist/mermaid.min.js" integrity="sha384-Wm9qzEgq4j1jEnuFK2FxKTlwuhbV2QqtGhcchvjDoKxeJ7WWAW7fysBq+1s6myfX" crossorigin="anonymous"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.js" integrity="sha384-FcQlsUOd0TJjROrBxhJdUhXTUgNJQxTMcxZe6nHbaEfFL1zjQ+bq/uRoBQxb0KMo" crossorigin="anonymous"></script>
    <link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css" integrity="sha384-sHL9NAb7lN7rfvG5lfHpm643Xkcjzp4jFvuavGOndn6pjVqS6ny56CAt3nsEVT4H" crossorigin="anonymous">
    <script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js" integrity="sha384-cxOPjt7s7Iz04uaHJceBmS+qpjv2JkIHNVcuOrM+YHwZOmJGBXI00mdUXEq65HTH" crossorigin="anonymous"></script>
`
}

func (tb *TemplateBuilder) buildHTMLBody() string {
	template := `    <div class="slidelang-presentation-container">
        {{range $index, $slide := .ContentBlocks}}
        <div class="slidelang-slide slidelang-{{$slide.Type}}-slide{{if eq $index 0}} slidelang-active{{end}}" 
             data-slide="{{$index}}"
             data-slide-type="{{$slide.Type}}"
             data-slide-title="{{if eq $slide.Type "title"}}{{$slide.Heading}}{{else}}{{$slide.Title}}{{end}}"
             data-duration="{{$slide.Duration}}"
             data-transition="{{$slide.Transition}}"
             data-interactive="{{$slide.HasInteractive}}"
             data-interactive-types="{{range $i, $elem := $slide.InteractiveElements}}{{if $i}},{{end}}{{$elem}}{{end}}"
             id="slidelang-slide-{{$index}}">
            {{/* Header del slide */}}
            {{template "slide-header" (dict "slide" $slide "presentation" $ "index" $index)}}
            
            {{if $slide.IsTitle}}
                {{/* Layout inteligente para slides de título */}}
                <div class="slidelang-title-slide-container">
                    {{/* Sección de título principal */}}
                    <div class="slidelang-title-section">
                        {{if $slide.Heading}}
                            <h1 class="slidelang-main-title">{{$slide.Heading}}</h1>
                        {{else if $slide.Title}}
                            <h1 class="slidelang-main-title">{{$slide.Title}}</h1>
                        {{else}}
                            {{template "slide-h1-fallback" (dict "index" $index)}}
                        {{end}}
                        {{if $slide.Subtitle}}
                            <h2 class="slidelang-subtitle">{{$slide.Subtitle}}</h2>
                        {{end}}
                    </div>
                    
                    {{/* Contenido de elementos del slide */}}
                    {{if $slide.Elements}}
                        <div class="slidelang-title-content">
                            {{template "slide-elements" (dict "elements" $slide.Elements "slide" $slide "presentation" $)}}
                        </div>
                    {{end}}
                </div>
            {{else}}
                {{/* Layout para slides de contenido */}}
                <div class="slidelang-content-wrapper">
                    {{if $slide.Title}}
                        <h1>{{$slide.Title}}</h1>
                    {{else}}
                        {{template "slide-h1-fallback" (dict "index" $index)}}
                    {{end}}
                    {{if $slide.Elements}}
                    <div class="slidelang-content-elements">
                        {{template "slide-elements" (dict "elements" $slide.Elements "slide" $slide "presentation" $)}}
                    </div>
                    {{end}}
                </div>
            {{end}}

            {{/* Footer del slide */}}
            {{template "slide-footer" (dict "slide" $slide "presentation" $ "index" $index)}}
        </div>
        {{end}}
    </div>
`

	// El contador/ayuda de navegación solo tiene sentido (y solo tiene CSS que
	// lo posicione/oculte en impresión, ver navigation.css) cuando la
	// navegación está habilitada — antes se emitían siempre, así que
	// --no-navigation (p. ej. combinado con --format pdf, que además fuerza
	// EmbedAssets) dejaba estos dos <div> cayendo al flujo normal del
	// documento como texto suelto sin estilo (issue #128, hallazgo de
	// code-review sobre PR #160).
	if tb.EnableNavigation {
		template += `
    {{/* Contador de slides: no es un landmark de navegación (no tiene controles
         interactivos) — el <nav> real, con los botones prev/next, lo inyecta
         navigation.js's createFloatingMenu(). Dos <nav> con el mismo
         aria-label confundiría la navegación por landmarks. */}}
    <div class="slidelang-nav-counter" aria-live="polite">
        <span id="slidelang-current-slide">1</span> / <span id="slidelang-total-slides">{{len .ContentBlocks}}</span>
    </div>

    <div class="slidelang-nav-help">
        <div>Navegar: ← → ↑ ↓</div>
    </div>
`
	}

	template += `
    {{/* SLIDELANG METADATA FOR ADVANCED VIEWER */}}
    <script type="application/json" id="slidelang-metadata">
    {
        "title": "{{.Title}}",
        "author": "{{.Author}}",
        "date": "{{.Date}}",
        "version": "{{.Version}}",
        "theme": "{{.Theme}}",
        "slides": [{{range $index, $slide := .ContentBlocks}}{{if $index}},{{end}}
            {
                "id": "slide-{{$index}}",
                "type": "{{$slide.Type}}",
                "title": "{{$slide.Title}}",
                "duration": {{$slide.Duration}},
                "transition": "{{$slide.Transition}}",
                "hasInteractive": {{$slide.HasInteractive}},
                "interactiveElements": [{{range $i, $elem := $slide.InteractiveElements}}{{if $i}},{{end}}"{{$elem}}"{{end}}],
                "notes": [{{range $i, $note := $slide.Notes}}{{if $i}},{{end}}"{{$note}}"{{end}}]
            }{{end}}
        ],
        "totalSlides": {{.TotalSlides}},
        "estimatedDuration": {{.EstimatedDuration}},
        "features": {
            "hasMermaid": {{.Features.HasMermaid}},
            "hasCharts": {{.Features.HasCharts}},
            "hasMaps": {{.Features.HasMaps}},
            "hasCode": {{.Features.HasCode}},
            "hasQuotes": {{.Features.HasQuotes}},
            "hasNotes": {{.Features.HasNotes}}
        },
        "charts": [{{range $i, $chart := .Charts}}{{if $i}},{{end}}
            {
                "id": "{{$chart.ID}}",
                "type": "{{$chart.Type}}",
                "config": {{$chart.Config | toJSON}}
            }{{end}}
        ],
        "diagrams": [{{range $i, $diagram := .Diagrams}}{{if $i}},{{end}}
            {
                "id": "{{$diagram.ID}}",
                "diagramType": "{{$diagram.DiagramType}}",
                "content": {{$diagram.Content | mermaidContent | toJSON}},
                "title": "{{$diagram.Title}}"
            }{{end}}
        ],
        "maps": [{{range $i, $map := .Maps}}{{if $i}},{{end}}
            {
                "id": "{{$map.ID}}",
                "mapType": "{{$map.MapType}}",
                "markers": {{$map.Markers | toJSON}},
                "options": {{$map.Options | toJSON}},
                "title": "{{$map.Title}}",
                "heatmap": {{$map.Heatmap}},
                "zoom": {{$map.Zoom}}
            }{{end}}
        ],
        "libraries": [{{range $i, $lib := .RequiredLibraries}}{{if $i}},{{end}}"{{$lib}}"{{end}}]
    }
    </script>`

	// Aplicar namespacing a las clases
	return tb.namespaceTemplateClasses(template)
}

// GetElementTemplate returns the element template with namespaced classes
func (tb *TemplateBuilder) GetElementTemplate() string {
	template := `{{define "element"}}
        {{if eq .Type "text"}}
            <div class="slidelang-element slidelang-text" 
                 id="slidelang-element-text-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="text"
                 data-slide="{{.SlideIndex}}">
                <p>{{.Content | markdownInline}}</p>
            </div>        {{else if eq .Type "points"}}
            <div class="slidelang-element slidelang-points" 
                 id="slidelang-element-points-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="points"
                 data-slide="{{.SlideIndex}}">
                {{if eq .ListType "ordered"}}
                    <ol>
                        {{range .Items}}
                            <li>{{.Content | markdown}}
                                {{if .SubPoints}}
                                    <ol>
                                        {{range .SubPoints}}
                                            <li>{{.Content | markdown}}</li>
                                        {{end}}
                                    </ol>
                                {{end}}
                            </li>
                        {{end}}
                    </ol>
                {{else}}
                    <ul>
                        {{range .Items}}
                            <li>{{.Content | markdown}}
                                {{if .SubPoints}}
                                    <ul>
                                        {{range .SubPoints}}
                                            <li>{{.Content | markdown}}</li>
                                        {{end}}
                                    </ul>
                                {{end}}
                            </li>
                        {{end}}
                    </ul>
                {{end}}
            </div>
        {{else if eq .Type "code"}}
            <div class="slidelang-element slidelang-code" 
                 id="slidelang-element-code-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="code"
                 data-language="{{.Language}}"
                 data-slide="{{.SlideIndex}}">
                {{if .Language}}<div class="language">{{.Language}}</div>{{end}}
                <pre><code{{if .Language}} class="language-{{.Language}}"{{end}}>{{.Content}}</code></pre>
            </div>
        {{else if eq .Type "image"}}
            <div class="slidelang-element slidelang-image slidelang-image-context-{{.Context}}"
                 id="slidelang-element-image-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="image"
                 data-slide="{{.SlideIndex}}"
                 data-context="{{.Context}}">
                {{if .Source}}
                <img src="{{.Source}}"
                     alt="{{.Alt}}"
                     loading="lazy"
                     style="object-fit: contain;">
                {{else}}
                <div class="slidelang-image-blocked" role="img" aria-label="{{.Alt}}" style="padding:1em;text-align:center;color:#a94442;background:#f2dede;border:1px solid #ebccd1;border-radius:4px;">Image blocked for security</div>
                {{end}}
                {{if .Caption}}<div class="caption">{{.Caption}}</div>{{end}}
            </div>
        {{else if eq .Type "table"}}
            <div class="slidelang-element slidelang-table" 
                 id="slidelang-element-table-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="table"
                 data-slide="{{.SlideIndex}}">
                {{if .Caption}}<h2>{{.Caption}}</h2>{{end}}
                <table>
                    <thead>
                        <tr>
                            {{range .Headers}}
                                <th>{{. | raw}}</th>
                            {{end}}
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Rows}}
                            <tr>
                                {{range .}}
                                    <td>{{. | raw}}</td>
                                {{end}}
                            </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        {{else if eq .Type "code_group"}}
            <div class="slidelang-element slidelang-code-group" 
                 id="slidelang-element-code-group-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="code_group"
                 data-slide="{{.SlideIndex}}">
                <div class="tabs">
                    {{range $i, $block := .CodeBlocks}}
                        <button type="button" class="slidelang-tab{{if eq $i 0}} active{{end}}" data-tab-index="{{$i}}">
                            {{if .Label}}{{.Label}}{{else}}{{.Language}}{{end}}
                        </button>
                    {{end}}
                </div>
                <div class="code-content">
                    {{range $i, $block := .CodeBlocks}}
                        <div class="slidelang-code-block{{if eq $i 0}} active{{end}}" id="code-block-{{$i}}">
                            <pre><code{{if .Language}} class="language-{{.Language}}"{{end}}>{{.Content}}</code></pre>
                        </div>
                    {{end}}
                </div>
            </div>        {{else if eq .Type "special_block"}}
            {{if eq .BlockType "details"}}
                <div class="slidelang-element slidelang-special-block slidelang-details"
                     id="slidelang-element-special-block-{{.SlideIndex}}-{{.ElementID}}"
                     data-element-type="special_block"
                     data-block-type="{{.BlockType}}"
                     data-slide="{{.SlideIndex}}">
                    {{if .Icon}}<span class="slidelang-icon">{{.Icon}}</span>{{end}}
                    {{if .Title}}<div class="slidelang-title">{{.Title | markdown}}</div>{{end}}
                    <div class="slidelang-content">{{.Content | markdown}}</div>
                </div>
            {{else}}
                <div class="slidelang-element slidelang-special-block slidelang-{{.BlockType}}" 
                     id="slidelang-element-special-block-{{.SlideIndex}}-{{.ElementID}}"
                     data-element-type="special_block"
                     data-block-type="{{.BlockType}}"
                     data-slide="{{.SlideIndex}}">
                    {{if .Icon}}<span class="slidelang-icon">{{.Icon}}</span>{{end}}
                    {{if .Title}}<div class="slidelang-title">{{.Title | markdown}}</div>{{end}}
                    <div class="slidelang-content">{{.Content | markdown}}</div>
                </div>
            {{end}}
        {{else if eq .Type "mermaid"}}
            <div class="slidelang-element slidelang-mermaid"
                 id="slidelang-element-mermaid-wrapper-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="mermaid"
                 data-slide="{{.SlideIndex}}">
                {{if .Title}}<h2 id="slidelang-element-mermaid-title-{{.SlideIndex}}-{{.ElementID}}">{{.Title}}</h2>{{end}}
                <div class="slidelang-mermaid"
                     id="slidelang-element-mermaid-{{.SlideIndex}}-{{.ElementID}}"
                     data-diagram-type="{{.DiagramType}}"
                     role="img"
                     {{if .Title}}aria-labelledby="slidelang-element-mermaid-title-{{.SlideIndex}}-{{.ElementID}}"{{else}}aria-label="Diagrama {{.DiagramType}}"{{end}}>{{.PreRenderedHTML}}</div>
            </div>        {{else if eq .Type "chart"}}
            <div class="slidelang-element slidelang-chart"
                 id="slidelang-element-chart-wrapper-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="chart"
                 data-chart-type="{{.ChartType | chartJSType}}"
                 data-slide="{{.SlideIndex}}">
                {{if .Title}}<h2 id="slidelang-element-chart-title-{{.SlideIndex}}-{{.ElementID}}">{{.Title}}</h2>{{end}}
                {{if .PreRenderedHTML}}<div class="slidelang-chart-container"
                     id="slidelang-element-chart-{{.SlideIndex}}-{{.ElementID}}"
                     role="img"
                     {{if .Title}}aria-labelledby="slidelang-element-chart-title-{{.SlideIndex}}-{{.ElementID}}"{{else}}aria-label="Gráfico"{{end}}>{{.PreRenderedHTML}}</div>{{else}}<div class="slidelang-chart-container">
                    <canvas class="slidelang-chart-canvas"
                            id="slidelang-element-chart-{{.SlideIndex}}-{{.ElementID}}"
                            data-chart-type="{{.ChartType | chartJSType}}"
                            data-chart-original-type="{{.ChartType}}"
                            role="img"
                            {{if .Title}}aria-labelledby="slidelang-element-chart-title-{{.SlideIndex}}-{{.ElementID}}"{{else}}aria-label="Gráfico"{{end}}></canvas>
                </div>{{end}}
            </div>
        {{else if eq .Type "map"}}
            <div class="element map"
                 id="slidelang-element-map-wrapper-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="map"
                 data-slide="{{.SlideIndex}}">
                {{if .Title}}<h2 id="slidelang-element-map-title-{{.SlideIndex}}-{{.ElementID}}">{{.Title}}</h2>{{end}}
                {{if .MapOptions.title}}{{if .Title}}<h3 class="map-title" id="slidelang-element-map-subtitle-{{.SlideIndex}}-{{.ElementID}}">{{.MapOptions.title}}</h3>{{else}}<h2 class="map-title" id="slidelang-element-map-subtitle-{{.SlideIndex}}-{{.ElementID}}">{{.MapOptions.title}}</h2>{{end}}{{end}}
                <div class="map-container"
                     id="slidelang-element-map-{{.SlideIndex}}-{{.ElementID}}"
                     role="img"
                     {{if .Title}}aria-labelledby="slidelang-element-map-title-{{.SlideIndex}}-{{.ElementID}}"{{else if .MapOptions.title}}aria-labelledby="slidelang-element-map-subtitle-{{.SlideIndex}}-{{.ElementID}}"{{else}}aria-label="Mapa"{{end}}>{{.PreRenderedHTML}}</div>
            </div>        {{else if eq .Type "quote"}}
            <div class="element quote" 
                 id="slidelang-element-quote-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="quote"
                 data-slide="{{.SlideIndex}}">
                <blockquote>
                    <p>{{.Content | markdownInline}}</p>
                    {{if .Author}}
                        {{/* div, no <footer>: <footer> as a child of <blockquote> (not a
                             sectioning ancestor) gets an implicit role="contentinfo" landmark,
                             so N quotes on one page produce N unnamed duplicate landmarks. */}}
                        <div class="quote-footer">
                            <cite>{{.Author}}</cite>
                            {{if .Source}}, <span class="source">{{.Source}}</span>{{end}}
                        </div>
                    {{end}}
                </blockquote>
            </div>        {{else if eq .Type "checklist"}}
            <div class="element checklist" 
                 id="slidelang-element-checklist-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="checklist"
                 data-slide="{{.SlideIndex}}">
                <ul class="slidelang-checklist-items">
                    {{range .ChecklistItems}}
                        <li class="slidelang-checklist-item {{if .Checked}}checked{{end}}">
                            <label class="slidelang-checklist-label">
                                <input type="checkbox" class="slidelang-checklist-checkbox" {{if .Checked}}checked{{end}} disabled>
                                <span class="slidelang-checklist-checkmark">
                                    <svg width="12" height="12" viewBox="0 0 12 12" class="slidelang-checkmark-icon">
                                        <path d="M3.5 6L5 7.5L8.5 4" stroke="currentColor" stroke-width="2" fill="none"/>
                                    </svg>
                                </span>
                                <span class="slidelang-checklist-content">{{.Content | markdown}}</span>
                            </label>
                            {{if .SubItems}}
                                <ul class="slidelang-checklist-subitems">
                                    {{range .SubItems}}
                                        <li class="slidelang-checklist-item {{if .Checked}}checked{{end}}">
                                            <label class="slidelang-checklist-label">
                                                <input type="checkbox" class="slidelang-checklist-checkbox" {{if .Checked}}checked{{end}} disabled>
                                                <span class="slidelang-checklist-checkmark">
                                                    <svg width="12" height="12" viewBox="0 0 12 12" class="slidelang-checkmark-icon">
                                                        <path d="M3.5 6L5 7.5L8.5 4" stroke="currentColor" stroke-width="2" fill="none"/>
                                                    </svg>
                                                </span>
                                                <span class="slidelang-checklist-content">{{.Content | markdown}}</span>
                                            </label>
                                        </li>
                                    {{end}}
                                </ul>
                            {{end}}
                        </li>
                    {{end}}
                </ul>
            </div>        {{else if eq .Type "directive"}}
            {{- if eq .DirectiveName "timer" -}}
                <!-- Slide Timer -->
                <div class="slidelang-element slidelang-directive slidelang-slide-timer" 
                     id="slidelang-element-directive-{{.SlideIndex}}-{{.ElementID}}"
                     data-element-type="directive"
                     data-directive-name="{{.DirectiveName}}"
                     data-slide="{{.SlideIndex}}"
                     data-directive="timer" 
                     data-duration="{{.Content}}">
                    <div class="timer-display">
                        <span class="timer-time">{{.Content}}s</span>
                        <div class="timer-progress"></div>
                    </div>
                </div>
            {{- else if eq .DirectiveName "transition" -}}
                <!-- Transition Directive -->
                <div class="slidelang-element slidelang-directive slidelang-transition-marker" 
                     id="slidelang-element-directive-{{.SlideIndex}}-{{.ElementID}}"
                     data-element-type="directive"
                     data-directive-name="{{.DirectiveName}}"
                     data-slide="{{.SlideIndex}}"
                     data-directive="transition" 
                     {{directiveDataAttrs .DirectiveParams}}>
                </div>
            {{- else -}}
                <!-- Generic Directive -->
                <div class="slidelang-element slidelang-directive {{range .CSSClasses}}slidelang-{{.}} {{end}}" 
                     id="slidelang-element-directive-{{.SlideIndex}}-{{.ElementID}}"
                     data-element-type="directive"
                     data-directive-name="{{.DirectiveName}}"
                     data-slide="{{.SlideIndex}}"
                     data-directive="{{.DirectiveName}}"
                     {{directiveDataAttrs .DirectiveParams}}>
                    {{- if and .Content (ne .DirectiveName "notes") -}}
                        <span class="directive-indicator">@{{.DirectiveName}}</span>
                    {{- end -}}
                </div>
            {{- end -}}
        {{else if eq .Type "grid"}}
            <div class="slidelang-element slidelang-grid" 
                 id="slidelang-element-grid-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="grid"
                 data-slide="{{.SlideIndex}}">
                {{if .Content}}
                    <div class="slidelang-content slidelang-grid-content">{{.Content | markdown}}</div>
                {{end}}
                {{range .Columns}}
                    <div class="slidelang-element slidelang-column"
                         data-element-type="column">
                        <div class="slidelang-content">{{.Content | markdown}}</div>
                    </div>
                {{end}}
            </div>
        {{else if eq .Type "column"}}
            <div class="slidelang-element slidelang-column" 
                 id="slidelang-element-column-{{.SlideIndex}}-{{.ElementID}}"
                 data-element-type="column"
                 data-slide="{{.SlideIndex}}">
                <div class="slidelang-content">{{.Content | markdown}}</div>
            </div>
        {{end}}
    {{end}}

    {{/* Template para elementos del slide */}}
    {{define "slide-elements"}}
        {{/* Contexto: recibe elementos, slide y presentation */}}
        {{range .elements}}
            {{template "element" .}}
        {{end}}
    {{end}}

    {{/* Issue #94: <h1> de fallback, solo para lectores de pantalla, para
         slides sin título propio — de lo contrario un elemento con título
         propio (chart/mermaid/map/table, h2) sería el primer encabezado del
         slide sin ningún h1 ancestro. Template compartido por el layout de
         título y el de contenido para que no diverjan entre sí. */}}
    {{define "slide-h1-fallback"}}
        <h1 class="slidelang-sr-only">Slide {{add1 .index}}</h1>
    {{end}}

    {{/* Template para header del slide */}}
    {{define "slide-header"}}
        {{/* Contexto: recibe un objeto con slide y presentationData */}}
        {{$slide := .slide}}
        {{$presentation := .presentation}}
        
        {{/* Solo renderizar si hay configuración de headers/footers */}}
        {{if $presentation.HeaderFooter}}
            {{/* Determinar configuración de header según prioridad */}}
            {{$finalHeaderConfig := $presentation.HeaderFooter.GlobalHeader}}
            
            {{/* Aplicar layout defaults si existe */}}
            {{if and $presentation.HeaderFooter.LayoutDefaults $slide.Type}}
                {{if index $presentation.HeaderFooter.LayoutDefaults $slide.Type}}
                    {{$layoutConfig := index $presentation.HeaderFooter.LayoutDefaults $slide.Type}}
                    {{if $layoutConfig.Header}}
                        {{$finalHeaderConfig = $layoutConfig.Header}}
                    {{end}}
                {{end}}
            {{end}}
            
            {{/* Aplicar overrides del slide si existe */}}
            {{if and $slide.HeaderFooterOverride $slide.HeaderFooterOverride.Header}}
                {{$finalHeaderConfig = $slide.HeaderFooterOverride.Header}}
            {{end}}
            
            {{/* Renderizar header solo si está habilitado */}}
            {{if and $finalHeaderConfig $finalHeaderConfig.Enabled}}
                <div class="slide-header" 
                     {{if $finalHeaderConfig.Height}}style="height: {{$finalHeaderConfig.Height}}; {{if $finalHeaderConfig.Background}}background: {{$finalHeaderConfig.Background}};{{end}}"{{end}}>
                    
                    {{/* Borde superior */}}
                    {{if and $finalHeaderConfig.Border $finalHeaderConfig.Border.Enabled (or (eq $finalHeaderConfig.Border.Position "top") (eq $finalHeaderConfig.Border.Position "both"))}}
                        <div class="header-border header-border-top" 
                             style="border-top: {{if $finalHeaderConfig.Border.Width}}{{$finalHeaderConfig.Border.Width}}{{else}}1px{{end}} {{if $finalHeaderConfig.Border.Style}}{{$finalHeaderConfig.Border.Style}}{{else}}solid{{end}} {{if $finalHeaderConfig.Border.Color}}{{$finalHeaderConfig.Border.Color}}{{else}}#ccc{{end}};"></div>
                    {{end}}
                    
                    <div class="header-content">
                        {{/* Logo */}}
                        {{if $finalHeaderConfig.Logo}}
                            <div class="slidelang-header-logo {{if $finalHeaderConfig.Logo.Position}}slidelang-logo-{{$finalHeaderConfig.Logo.Position}}{{else}}slidelang-logo-left{{end}}">
                                <img src="{{$finalHeaderConfig.Logo.Source}}"
                                     alt="{{if $finalHeaderConfig.Logo.Alt}}{{$finalHeaderConfig.Logo.Alt}}{{else}}Logo{{end}}"
                                     {{if $finalHeaderConfig.Logo.Height}}style="height: {{$finalHeaderConfig.Logo.Height}};"{{end}}>
                            </div>
                        {{end}}
                        
                        {{/* Texto del header */}}
                        {{if $finalHeaderConfig.Text}}
                            <div class="header-text">
                                {{if $finalHeaderConfig.Text.Left}}
                                    <div class="header-text-left">{{$finalHeaderConfig.Text.Left}}</div>
                                {{end}}
                                {{if $finalHeaderConfig.Text.Center}}
                                    <div class="header-text-center">{{$finalHeaderConfig.Text.Center}}</div>
                                {{end}}
                                {{if $finalHeaderConfig.Text.Right}}
                                    <div class="header-text-right">{{$finalHeaderConfig.Text.Right}}</div>
                                {{end}}
                            </div>
                        {{end}}
                    </div>
                    
                    {{/* Borde inferior */}}
                    {{if and $finalHeaderConfig.Border $finalHeaderConfig.Border.Enabled (or (eq $finalHeaderConfig.Border.Position "bottom") (eq $finalHeaderConfig.Border.Position "both"))}}
                        <div class="header-border header-border-bottom" 
                             style="border-bottom: {{if $finalHeaderConfig.Border.Width}}{{$finalHeaderConfig.Border.Width}}{{else}}1px{{end}} {{if $finalHeaderConfig.Border.Style}}{{$finalHeaderConfig.Border.Style}}{{else}}solid{{end}} {{if $finalHeaderConfig.Border.Color}}{{$finalHeaderConfig.Border.Color}}{{else}}#ccc{{end}};"></div>
                    {{end}}
                </div>
            {{end}}
        {{end}}
    {{end}}
    
    {{/* Template para footer del slide */}}
    {{define "slide-footer"}}
        {{/* Contexto: recibe un objeto con slide y presentationData */}}
        {{$slide := .slide}}
        {{$presentation := .presentation}}
        
        {{/* Solo renderizar si hay configuración de headers/footers */}}
        {{if $presentation.HeaderFooter}}
            {{/* Determinar configuración de footer según prioridad */}}
            {{$finalFooterConfig := $presentation.HeaderFooter.GlobalFooter}}
            
            {{/* Aplicar layout defaults si existe */}}
            {{if and $presentation.HeaderFooter.LayoutDefaults $slide.Type}}
                {{if index $presentation.HeaderFooter.LayoutDefaults $slide.Type}}
                    {{$layoutConfig := index $presentation.HeaderFooter.LayoutDefaults $slide.Type}}
                    {{if $layoutConfig.Footer}}
                        {{$finalFooterConfig = $layoutConfig.Footer}}
                    {{end}}
                {{end}}
            {{end}}
            
            {{/* Aplicar overrides del slide si existe */}}
            {{if and $slide.HeaderFooterOverride $slide.HeaderFooterOverride.Footer}}
                {{$finalFooterConfig = $slide.HeaderFooterOverride.Footer}}
            {{end}}
            
            {{/* Renderizar footer solo si está habilitado */}}
            {{if and $finalFooterConfig $finalFooterConfig.Enabled}}
                <div class="slide-footer" 
                     {{if $finalFooterConfig.Height}}style="height: {{$finalFooterConfig.Height}}; {{if $finalFooterConfig.Background}}background: {{$finalFooterConfig.Background}};{{end}}"{{end}}>
                    
                    {{/* Borde superior */}}
                    {{if and $finalFooterConfig.Border $finalFooterConfig.Border.Enabled (or (eq $finalFooterConfig.Border.Position "top") (eq $finalFooterConfig.Border.Position "both"))}}
                        <div class="footer-border footer-border-top" 
                             style="border-top: {{if $finalFooterConfig.Border.Width}}{{$finalFooterConfig.Border.Width}}{{else}}1px{{end}} {{if $finalFooterConfig.Border.Style}}{{$finalFooterConfig.Border.Style}}{{else}}solid{{end}} {{if $finalFooterConfig.Border.Color}}{{$finalFooterConfig.Border.Color}}{{else}}#ccc{{end}};"></div>
                    {{end}}
                    
                    <div class="footer-content">
                        {{/* Texto del footer */}}
                        {{if $finalFooterConfig.Text}}
                            <div class="footer-text">
                                {{if $finalFooterConfig.Text.Left}}
                                    <div class="footer-text-left">{{$finalFooterConfig.Text.Left}}</div>
                                {{end}}
                                {{if $finalFooterConfig.Text.Center}}
                                    <div class="footer-text-center">{{$finalFooterConfig.Text.Center}}</div>
                                {{end}}
                                {{if $finalFooterConfig.Text.Right}}
                                    <div class="footer-text-right">{{$finalFooterConfig.Text.Right}}</div>
                                {{end}}
                            </div>
                        {{end}}
                        
                        {{/* Números de página */}}
                        {{if and $finalFooterConfig.PageNumbers $finalFooterConfig.PageNumbers.Enabled $slide.ShowPageNumber}}
                            <div class="slidelang-page-numbers slidelang-page-numbers-{{if $finalFooterConfig.PageNumbers.Position}}{{$finalFooterConfig.PageNumbers.Position}}{{else}}right{{end}} slidelang-page-numbers-style-{{if $finalFooterConfig.PageNumbers.Style}}{{$finalFooterConfig.PageNumbers.Style}}{{else}}normal{{end}}">
                                <span class="page-number-text">{{processPageFormat $finalFooterConfig.PageNumbers.Format $slide.DisplayNumber $presentation.HeaderFooter.TotalSlides}}</span>
                            </div>
                        {{end}}
                    </div>
                    
                    {{/* Borde inferior */}}
                    {{if and $finalFooterConfig.Border $finalFooterConfig.Border.Enabled (or (eq $finalFooterConfig.Border.Position "bottom") (eq $finalFooterConfig.Border.Position "both"))}}
                        <div class="footer-border footer-border-bottom" 
                             style="border-bottom: {{if $finalFooterConfig.Border.Width}}{{$finalFooterConfig.Border.Width}}{{else}}1px{{end}} {{if $finalFooterConfig.Border.Style}}{{$finalFooterConfig.Border.Style}}{{else}}solid{{end}} {{if $finalFooterConfig.Border.Color}}{{$finalFooterConfig.Border.Color}}{{else}}#ccc{{end}};"></div>
                    {{end}}
                </div>
            {{end}}
        {{end}}
    {{end}}`

	// Apply namespacing to classes
	return tb.namespaceTemplateClasses(template)
}

// namespaceTemplateClasses processes HTML template strings to add namespacing to class attributes
func (tb *TemplateBuilder) namespaceTemplateClasses(template string) string {
	// Use regex to find class="..." patterns and namespace them
	// This handles both static and dynamic classes
	re := regexp.MustCompile(`class="([^"]*)"`)

	return re.ReplaceAllStringFunc(template, func(match string) string {
		// Extract the class value
		classes := strings.TrimPrefix(strings.TrimSuffix(match, `"`), `class="`)

		// Handle Go template expressions within class values
		if strings.Contains(classes, "{{") {
			// For class attributes containing template expressions,
			// skip automatic namespacing to avoid parsing issues.
			// These should be manually namespaced in the template itself.
			return match
		}

		// Simple static classes - namespace them
		namespacedClasses := tb.namespaceClasses(classes)
		return `class="` + namespacedClasses + `"`
	})
}

// namespaceJavaScriptSelectors processes JavaScript strings to add namespacing to CSS class selectors
func (tb *TemplateBuilder) namespaceJavaScriptSelectors(js string) string {
	// Replace querySelector and querySelectorAll calls with namespaced class selectors
	// Pattern: querySelector('.classname') or querySelectorAll('.classname')
	re := regexp.MustCompile(`(querySelector(?:All)?)\(['"]\.([^'"]*)['"]\)`)

	return re.ReplaceAllStringFunc(js, func(match string) string {
		// Extract the parts using regex groups
		parts := re.FindStringSubmatch(match)
		if len(parts) >= 3 {
			method := parts[1]   // querySelector or querySelectorAll
			selector := parts[2] // the class name

			// Add namespace prefix to the class
			namespacedSelector := tb.namespaceClass(selector)

			// Return the updated selector
			return method + "('" + "." + namespacedSelector + "')"
		}
		return match
	})
}
