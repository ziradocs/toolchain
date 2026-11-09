// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"math"
	"strings"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/doclang/v2/internal/inks"
)

// Variables de superficie del HTML/PDF para temas externos.
//
// El CSS del documento (core/renderer/document_html.go) lee los fondos y
// colores de tablas, paneles, teclas, citas, alertas, vínculos y el margen
// del PDF de variables propias (--doclang-table-bg, --doclang-panel-bg,
// --doclang-body-link-color, ...), con el literal de siempre como fallback.
// No lee --doclang-link-color, --doclang-quote-*, --doclang-alert-* ni
// --doclang-page-bg para esos elementos porque los temas embebidos los
// definen con valores que nunca se aplicaron ahí: leerlos cambiaría su
// render.
//
// Un tema externo trae las variables del contrato, no las de superficie.
// htmlThemeVariables deriva estas de aquellas, solo para temas externos, sin
// pisar ninguna que el tema defina por su cuenta, y midiendo el contraste
// (a11y.ContrastRatio) para no producir un par peor que el literal:
//
//   - Lo que el tema pide explícitamente (--doclang-quote-bg,
//     --doclang-alert-<tipo>-bg, --doclang-page-bg para el margen y la hoja
//     de page-view) se aplica, pero un fondo solo junto con un texto que se
//     lea sobre él a 4.5:1: el color propio del tema para esa superficie, el
//     literal de siempre o --doclang-text-color, el primero que dé. Si
//     ninguno da, la superficie conserva su par literal.
//   - Lo implícito (tabla, fila alterna, paneles, teclas, quiz, banda de
//     page-view, sidebar y TOC del viewer, títulos, h2-h4) solo se deriva
//     cuando el literal no se lee con el texto del tema. Un tema claro no
//     cambia ahí; uno oscuro recibe la hoja como fondo.
//   - Un vínculo toma --doclang-link-color solo si da 4.5:1 sobre todos los
//     fondos donde aparece; si no, se queda el del navegador; si ese
//     tampoco se lee (hoja oscura), un azul claro fijo, y si tampoco, el
//     texto de la superficie.
//   - Lo que va dentro de una superficie se mide contra el fondo de esa
//     superficie, no solo contra la página: las tintas de estado
//     (--doclang-<superficie>-<tinta>-color, solo donde la del documento no
//     se lee ahí), el vínculo del encabezado de la tabla, la tecla (que
//     hereda el texto de donde esté, pero pinta su propio fondo) y el par
//     del código en línea.
//   - El fondo de las gráficas no se deriva: Chart.js y la imagen de la
//     gráfica usan su propia paleta clara.
//
// Un color que no se puede medir (un gradiente) nunca cuenta como legible.
//
// Las variables derivadas son copias de los valores, no var(): así el par que
// se midió es exactamente el que se pinta. Con var() seguirían al modo oscuro
// del viewer, pero ese modo solo redefine algunas variables del contrato y
// una referencia podría terminar en un par que nadie midió. La excepción es
// el color del código (codePairs): puede ser var(--doclang-text-color),
// medido con el texto del tema en el modo claro y con el del modo oscuro
// sobre su fondo de código, porque ese modo cambia el fondo del código pero
// no su color.
//
// Los valores pasan por renderer.SanitizeCSSCustomProperty, como cualquier
// variable del tema; además aquí se descartan los que traen comentarios,
// escapes o comillas desbalanceadas, que no tienen razón de ser en un color.
func htmlThemeVariables(vars map[string]string, external bool) map[string]string {
	if !external || len(vars) == 0 {
		return vars
	}
	d := &themeDeriver{vars: vars, out: make(map[string]string, len(vars)+64)}
	for k, v := range vars {
		d.out[k] = v
	}
	d.derive()
	return d.out
}

const (
	minTextContrast  = 4.5 // texto normal y vínculos
	minLargeContrast = 3.0 // títulos y texto secundario
	browserLinkColor = "#0000ee"
	// darkThemeLinkColor es el vínculo de respaldo cuando la hoja es oscura y
	// el tema no trae un color de vínculo legible: el azul del navegador no
	// se lee sobre ella.
	darkThemeLinkColor = "#8ab4f8"
)

// cssFallbacks son los fallbacks del CSS del documento para cada variable:
// lo que se pinta cuando no está. TestCSSFallbacksMatchCore verifica que
// coinciden con core/renderer/document_html.go.
var cssFallbacks = map[string]string{
	"table-bg":                "white",
	"table-stripe-bg":         "#f8fafc",
	"table-hover-bg":          "#edf2f7",
	"panel-bg":                "#f8fafc",
	"kbd-bg":                  "#f7f7f7",
	"blockquote-color":        "#4a5568",
	"blockquote-footer-color": "#718096",
	"callout-info-color":      "#2c5282",
	"callout-warning-color":   "#7c2d12",
	"callout-danger-color":    "#742a2a",
	"callout-success-color":   "#22543d",
	"callout-tip-color":       "#44337a",
	"element-title-color":     "#2d3748",
	"caption-color":           "#718096",
	"code-tab-active-color":   "#fff",
	"h2-color":                "#2d3748",
	"h3-color":                "#4a5568",
	"h4-color":                "#718096",
	"page-surface-bg":         "#ffffff",
	"header-footer-bg":        "#fafafa",
	"sidebar-bg":              "#f8f9fa",
	"blockquote-bg":           "#f7fafc",
	"callout-info-bg":         "#ebf8ff",
	"callout-warning-bg":      "#fffaf0",
	"callout-danger-bg":       "#fff5f5",
	"callout-success-bg":      "#f0fff4",
	"callout-tip-bg":          "#faf5ff",
	"code-inline-bg":          "#f6f8fa",
	"code-inline-color":       "#e83e8c",
	"code-bg":                 "#1e1e1e",
	"code-color":              "#d4d4d4",
}

// Fallbacks anidados o compartidos: el TOC estático y el botón de modo oscuro
// del viewer leen --doclang-toc-bg con literales distintos, y lo mismo pasa
// con el color de vínculo del TOC estático y el del sidebar.
const (
	tocBgLiteral          = "#f8f9fa" // .toc
	themeToggleBgLiteral  = "#e2e8f0" // .theme-toggle (viewer)
	tocTitleLiteral       = "#1a202c" // .toc h2
	tocLinkLiteral        = "#1a202c" // .toc > ul > li > a
	sidebarTocLinkLiteral = "#4a5568" // .toc-link (viewer)
	tocSubsectionLiteral  = "#4a5568" // .toc li ul li a
	quizBgLiteral         = "#f8fafc"
	quizCorrectBgLiteral  = "#f0fff4"
	quizCorrectFgLiteral  = "#22543d"
	headerFooterFgLiteral = "#666"
	bodyTextLiteral       = "#2c3e50" // body.doclang-document
	tableHeaderFgLiteral  = "white"   // table th (sobre el degradado literal)
)

// Tintas de estado ([x]{.danger}, .info, .success, .warning, .accent,
// inks.Names). El CSS del documento las pinta con --doclang-<tinta>-color y,
// dentro de una superficie con fondo propio (inks.Surfaces), con
// --doclang-<superficie>-ink-<tinta> si está (core/renderer/document_html.go,
// documentInkSurfaceCSS).
// inkLiterals son los fallbacks del CSS para cada tinta; inkDeep e inkLight,
// respaldos de la misma familia de color para un fondo donde el literal no
// se lee: más oscuros (fondos claros que no son blancos) y más claros
// (fondos oscuros).
var (
	inkLiterals = map[string]string{"danger": "#dc2626", "info": "#2563eb", "success": "#15803d", "warning": "#b45309", "accent": "#7c3aed"}
	inkDeep     = map[string]string{"danger": "#991b1b", "info": "#1e40af", "success": "#166534", "warning": "#92400e", "accent": "#5b21b6"}
	inkLight    = map[string]string{"danger": "#fca5a5", "info": "#93c5fd", "success": "#86efac", "warning": "#fcd34d", "accent": "#c4b5fd"}
)

// codeInlineDeepColor es el respaldo del código en línea cuando el rosa
// literal no se lee sobre su fondo (tampoco sobre el gris literal: 3.59:1).
const codeInlineDeepColor = "#c2185b"

type themeDeriver struct {
	vars map[string]string
	out  map[string]string
}

// get devuelve una variable del tema si está definida y es segura.
func (d *themeDeriver) get(name string) (string, bool) {
	v, ok := d.vars["--doclang-"+name]
	if !ok || !safeDerivedValue("--doclang-"+name, v) {
		return "", false
	}
	return v, true
}

func (d *themeDeriver) defined(name string) bool {
	_, ok := d.vars["--doclang-"+name]
	return ok
}

// set fija una variable derivada salvo que el tema la defina.
func (d *themeDeriver) set(name, value string) {
	if !d.defined(name) {
		d.out["--doclang-"+name] = value
	}
}

// themed es el valor que sale del tema para name (propio y seguro, o
// derivado aquí), o "" si se pinta el fallback del CSS.
func (d *themeDeriver) themed(name string) string {
	if d.defined(name) {
		v, _ := d.get(name)
		return v
	}
	return d.out["--doclang-"+name]
}

// resolved es lo que se pinta para name: lo del tema o el fallback del CSS.
// Una variable del tema insegura se descarta al emitirse, así que también
// cae al fallback.
func (d *themeDeriver) resolved(name, fallback string) string {
	if v := d.themed(name); v != "" {
		return v
	}
	return fallback
}

func safeDerivedValue(name, v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	if _, ok := renderer.SanitizeCSSCustomProperty(name, v); !ok {
		return false
	}
	// SanitizeCSSCustomProperty ya rechaza comentarios, cadenas abiertas y
	// paréntesis sin cerrar; además, un color no lleva barra invertida.
	return !strings.Contains(v, `\`)
}

// reads dice si fg se lee sobre todos los fondos con el mínimo dado. Un color
// que no se puede medir no cuenta como legible.
func reads(fg string, min float64, bgs ...string) bool {
	if fg == "" || len(bgs) == 0 {
		return false
	}
	for _, bg := range bgs {
		r, ok := a11y.ContrastRatio(fg, bg)
		if !ok || r < min {
			return false
		}
	}
	return true
}

// pickText devuelve el primer candidato que se lee sobre bgs.
func pickText(min float64, bgs []string, candidates ...string) (string, bool) {
	for _, c := range candidates {
		if reads(c, min, bgs...) {
			return c, true
		}
	}
	return "", false
}

// implicitBg deriva el fondo de una superficie que el tema no nombra, solo si
// el literal no se lee con su texto fg; toma el primer candidato que sí.
func (d *themeDeriver) implicitBg(name, literal, fg string, candidates ...string) bool {
	if d.defined(name) || fg == "" || reads(fg, minTextContrast, literal) {
		return false
	}
	for _, bg := range candidates {
		if bg != "" && reads(fg, minTextContrast, bg) {
			d.set(name, bg)
			return true
		}
	}
	return false
}

// fgReq es un color de texto que se pinta sobre una superficie y el contraste
// mínimo que necesita.
type fgReq struct {
	color string
	min   float64
}

// implicitBgFor es implicitBg para una superficie donde se pintan varios
// textos: el fondo se deriva si el primero (el texto principal) no se lee
// sobre el literal, y solo con un candidato donde se lean todos.
func (d *themeDeriver) implicitBgFor(name, literal string, fgs []fgReq, candidates ...string) {
	if d.defined(name) || len(fgs) == 0 || fgs[0].color == "" || reads(fgs[0].color, fgs[0].min, literal) {
		return
	}
	for _, bg := range candidates {
		ok := bg != ""
		for _, f := range fgs {
			ok = ok && reads(f.color, f.min, bg)
		}
		if ok {
			d.set(name, bg)
			return
		}
	}
}

// implicitFg deriva un color de texto que el tema no nombra, solo si el
// literal no se lee sobre bgs.
func (d *themeDeriver) implicitFg(name, literal string, min float64, bgs []string, candidates ...string) {
	if d.defined(name) || reads(literal, min, bgs...) {
		return
	}
	if c, ok := pickText(min, bgs, candidates...); ok {
		d.set(name, c)
	}
}

// explicitSurface aplica un fondo que el tema pide para una superficie con
// texto propio, junto con el primero de fgCandidates que se lea sobre él (el
// literal no hace falta fijarlo). Devuelve false si la superficie conserva
// su par literal.
func (d *themeDeriver) explicitSurface(bgName, fgName, bg string, fgCandidates ...string) bool {
	c, ok := pickText(minTextContrast, []string{bg}, fgCandidates...)
	if !ok {
		return false
	}
	d.set(bgName, bg)
	if c != cssFallbacks[fgName] {
		d.set(fgName, c)
	}
	return true
}

// link elige el color de vínculo sobre bgs: el del tema si se lee; si no,
// ninguno (queda el del navegador) mientras ese se lea; si tampoco, el azul
// claro de respaldo, y si tampoco, el texto de la superficie (fg), que sigue
// subrayado.
func (d *themeDeriver) link(name, fg string, bgs ...string) {
	if d.defined(name) {
		return
	}
	if l, ok := d.get("link-color"); ok && reads(l, minTextContrast, bgs...) {
		d.set(name, l)
		return
	}
	if reads(browserLinkColor, minTextContrast, bgs...) {
		return
	}
	if c, ok := pickText(minTextContrast, bgs, darkThemeLinkColor, fg); ok {
		d.set(name, c)
	}
}

func (d *themeDeriver) derive() {
	text, _ := d.get("text-color")
	textLight, _ := d.get("text-light")

	// La hoja. En page-view la superficie es --doclang-page-surface-bg (sin
	// ella, blanca) y --doclang-page-bg es el lienzo alrededor; en el modo
	// normal la página es --doclang-page-bg. Un tema externo sin hoja propia
	// cuyo texto no se lee sobre el blanco (un tema oscuro) pinta la hoja
	// con su --doclang-page-bg; uno claro con lienzo gris conserva la hoja
	// blanca. El margen del PDF toma --doclang-page-bg.
	page, hasPage := d.get("page-bg")
	if hasPage {
		d.implicitBg("page-surface-bg", cssFallbacks["page-surface-bg"], text, page)
		d.set("print-margin-bg", page)
	}
	sheet := d.resolved("page-surface-bg", cssFallbacks["page-surface-bg"])
	// Los textos del cuerpo se pintan sobre la página del modo normal y
	// sobre la hoja de page-view: se miden contra las dos.
	pageBgs := []string{sheet}
	if hasPage && page != sheet {
		pageBgs = append(pageBgs, page)
	}

	// Tabla: fondo, fila alterna y hover.
	d.implicitBg("table-bg", cssFallbacks["table-bg"], text, sheet)
	d.implicitBg("table-stripe-bg", cssFallbacks["table-stripe-bg"], text, sheet)
	tableBg := d.resolved("table-bg", cssFallbacks["table-bg"])
	stripe := d.resolved("table-stripe-bg", cssFallbacks["table-stripe-bg"])
	d.implicitBg("table-hover-bg", cssFallbacks["table-hover-bg"], text, stripe, sheet)

	// Paneles y teclas; el borde sigue al fondo solo si el fondo se derivó.
	border, hasBorder := d.get("table-border")
	if d.implicitBg("panel-bg", cssFallbacks["panel-bg"], text, stripe, sheet) && hasBorder {
		d.set("panel-border", border)
	}
	// La tecla se mide con su propio color si el tema lo trae: derivarle
	// un fondo oscuro con el texto del cuerpo dejaría ilegible ese color.
	kbdFg := text
	if c, ok := d.get("kbd-color"); ok {
		kbdFg = c
	}
	if d.implicitBg("kbd-bg", cssFallbacks["kbd-bg"], kbdFg, stripe, sheet) && hasBorder {
		d.set("kbd-border", border)
	}
	panelBg := d.resolved("panel-bg", cssFallbacks["panel-bg"])

	// Encabezado y pie de page-view (su texto es --doclang-text-light).
	hfText := textLight
	if hfText == "" {
		hfText = headerFooterFgLiteral
	}
	d.implicitBg("header-footer-bg", cssFallbacks["header-footer-bg"], hfText, sheet)

	// Títulos de la página y de los elementos, sobre la hoja.
	h1, _ := d.get("h1-color")
	for _, h := range []string{"h2-color", "h3-color", "h4-color"} {
		d.implicitFg(h, cssFallbacks[h], minLargeContrast, pageBgs, h1, text)
	}
	d.implicitFg("element-title-color", cssFallbacks["element-title-color"], minTextContrast, pageBgs, text)
	d.implicitFg("caption-color", cssFallbacks["caption-color"], minLargeContrast, pageBgs, textLight, text)

	// Cita.
	if bg, ok := d.get("quote-bg"); ok {
		own, _ := d.get("quote-color")
		if d.explicitSurface("blockquote-bg", "blockquote-color", bg, own, cssFallbacks["blockquote-color"], text) {
			d.implicitFg("blockquote-footer-color", cssFallbacks["blockquote-footer-color"], minLargeContrast, []string{bg}, textLight, text)
		}
	}
	if b, ok := d.get("quote-border"); ok {
		d.set("blockquote-border", b)
	}
	if bg := d.themed("blockquote-bg"); bg != "" {
		d.link("blockquote-link-color", d.resolved("blockquote-color", cssFallbacks["blockquote-color"]), bg)
	}

	// Alertas.
	for _, kind := range []string{"info", "warning", "danger", "success", "tip"} {
		bgName, fgName := "callout-"+kind+"-bg", "callout-"+kind+"-color"
		if bg, ok := d.get("alert-" + kind + "-bg"); ok {
			own, _ := d.get("alert-" + kind + "-color")
			d.explicitSurface(bgName, fgName, bg, own, cssFallbacks[fgName], text)
		}
		if b, ok := d.get("alert-" + kind + "-border"); ok {
			d.set("callout-"+kind+"-border", b)
		}
		if bg := d.themed(bgName); bg != "" {
			d.link("callout-"+kind+"-link-color", d.resolved(fgName, cssFallbacks[fgName]), bg)
		}
	}

	// Quiz y poll: fondo (cae a --doclang-quote-bg) y opción correcta.
	quizText, ok := d.get("quiz-question-color")
	if !ok {
		quizText = text
	}
	quizBg := quizBgLiteral
	if q, ok := d.get("quote-bg"); ok {
		quizBg = q
	}
	d.implicitBg("quiz-bg", quizBg, quizText, stripe, sheet)
	d.quizCorrect(text, stripe, sheet)

	// Pestaña activa de un code-group: el fondo del bloque de código, con el
	// blanco de siempre si se lee o el texto del código si no.
	if bg, ok := d.get("code-bg"); ok {
		codeColor, _ := d.get("code-color")
		if c, ok := pickText(minTextContrast, []string{bg}, cssFallbacks["code-tab-active-color"], codeColor); ok {
			d.set("code-tab-active-bg", bg)
			if c != cssFallbacks["code-tab-active-color"] {
				d.set("code-tab-active-color", c)
			}
		}
	}

	// Viewer: sidebar y TOC. El texto del sidebar y del botón de modo oscuro
	// es --doclang-text-color; sobre el fondo del TOC y del sidebar también
	// se pintan los --doclang-toc-*-color, así que un fondo nuevo solo se
	// toma si además se leen los que el tema definió (los que no definió se
	// derivan abajo, del texto del cuerpo).
	tocTitle, hasTocTitle := d.get("toc-title-color")
	tocLink, hasTocLink := d.get("toc-link-color")
	tocSub, hasTocSub := d.get("toc-subsection-color")
	sidebarFgs := []fgReq{{text, minTextContrast}}
	if hasTocLink {
		sidebarFgs = append(sidebarFgs, fgReq{tocLink, minTextContrast})
	}
	d.implicitBgFor("sidebar-bg", cssFallbacks["sidebar-bg"], sidebarFgs, sheet)
	sidebarBg := d.resolved("sidebar-bg", cssFallbacks["sidebar-bg"])
	tocFgs := []fgReq{{text, minTextContrast}}
	if hasTocTitle {
		tocFgs = append(tocFgs, fgReq{tocTitle, minLargeContrast})
	}
	if hasTocLink {
		tocFgs = append(tocFgs, fgReq{tocLink, minTextContrast})
	}
	if hasTocSub {
		tocFgs = append(tocFgs, fgReq{tocSub, minTextContrast})
	}
	d.implicitBgFor("toc-bg", themeToggleBgLiteral, tocFgs, sheet)
	tocBg := d.resolved("toc-bg", tocBgLiteral)
	d.implicitFg("toc-title-color", tocTitleLiteral, minLargeContrast, []string{tocBg}, text)
	d.implicitFg("toc-subsection-color", tocSubsectionLiteral, minTextContrast, []string{tocBg}, text)
	// --doclang-toc-link-color pinta el TOC estático y el del sidebar, con
	// literales distintos: se deriva si cualquiera de los dos no se lee.
	if !d.defined("toc-link-color") &&
		(!reads(tocLinkLiteral, minTextContrast, tocBg) || !reads(sidebarTocLinkLiteral, minTextContrast, sidebarBg)) {
		if c, ok := pickText(minTextContrast, []string{tocBg, sidebarBg}, text); ok {
			d.set("toc-link-color", c)
		}
	}

	// Tabla: con fondo del tema (propio o derivado) su texto deja de
	// heredar. Heredado, dentro de una alerta que conserva su fondo claro
	// tomaba el texto oscuro de la alerta sobre la tabla oscura.
	bodyText := text
	if bodyText == "" {
		bodyText = bodyTextLiteral
	}
	if !d.defined("table-color") && text != "" && (d.themed("table-bg") != "" || d.themed("table-stripe-bg") != "") &&
		reads(text, minTextContrast, tableBg, stripe) {
		d.set("table-color", text)
	}
	tableFg := bodyText
	if v := d.themed("table-color"); v != "" {
		tableFg = v
	}

	// Vínculos del cuerpo y de la tabla.
	d.bodyAndTableLinks(text, pageBgs, panelBg, tableBg, stripe, tableFg)

	// Contenido anidado: lo que va dentro de las superficies (tintas,
	// vínculo y tecla en el encabezado de la tabla, código en línea).
	panelFg := bodyText
	if v := d.themed("panel-color"); v != "" {
		panelFg = v
	}
	thBg, _ := d.get("table-header-bg") // sin él, el degradado literal: no se mide
	thFg, ok := d.get("table-header-color")
	if !ok {
		thFg = tableHeaderFgLiteral
	}
	quizBgResolved := d.resolved("quiz-bg", quizBg)
	surfaces := []inkSurface{
		{"table", tableBg, tableFg},
		{"table-stripe", stripe, tableFg},
		{"table-header", thBg, thFg},
		{"panel", panelBg, panelFg},
		{"blockquote", d.resolved("blockquote-bg", cssFallbacks["blockquote-bg"]), d.resolved("blockquote-color", cssFallbacks["blockquote-color"])},
	}
	for _, kind := range []string{"info", "warning", "danger", "success", "tip"} {
		surfaces = append(surfaces, inkSurface{"callout-" + kind,
			d.resolved("callout-"+kind+"-bg", cssFallbacks["callout-"+kind+"-bg"]),
			d.resolved("callout-"+kind+"-color", cssFallbacks["callout-"+kind+"-color"])})
	}
	surfaces = append(surfaces, inkSurface{"quiz", quizBgResolved, quizText})
	d.inks(pageBgs, bodyText, surfaces)
	d.tableHeaderLink(thBg, thFg)
	d.kbdColor(bodyText, surfaces)
	d.codePairs(text, stripe, sheet)
}

// bodyAndTableLinks elige el vínculo del cuerpo y, aparte, el de las celdas
// de la tabla (--doclang-table-link-color, que el CSS lee en td).
//
// El del cuerpo se mide sobre la página, la hoja y los paneles: el del tema
// si se lee ahí; si no, ninguno (el del navegador) mientras ese se lea; si
// tampoco, el azul claro de respaldo. Si ninguno se lee en todos, se acepta
// el primero que se lea sobre la página y la hoja, y solo si tampoco hay
// uno así, el texto del cuerpo (el vínculo sigue subrayado). Una superficie
// donde ni el texto del tema se lee (un panel claro bajo texto claro) no
// cuenta: ahí ningún vínculo arregla nada.
//
// La tabla y su fila alterna ya no bajan el vínculo de todo el documento:
// si el del cuerpo no se lee sobre la tabla, la tabla lleva el suyo
// (--doclang-table-link-color: el del tema, el del navegador, el azul claro
// o el texto de la tabla), y si el de la tabla no se lee sobre la fila
// alterna, la fila alterna el suyo (--doclang-table-stripe-link-color).
// Cada uno se mide solo contra su fondo, aunque el texto del tema no se lea
// ahí: un vínculo legible sobre una fila alterna que el tema dejó ilegible
// es mejor que ninguno.
func (d *themeDeriver) bodyAndTableLinks(text string, pageBgs []string, panelBg, tableBg, stripe, tableFg string) {
	themeLink, _ := d.get("link-color")
	readable := func(fg string, bgs ...string) []string {
		var out []string
		for _, bg := range bgs {
			if fg == "" || reads(fg, minTextContrast, bg) {
				out = append(out, bg)
			}
		}
		return out
	}
	if !d.defined("body-link-color") {
		all := readable(text, append(append([]string{}, pageBgs...), panelBg)...)
		page := readable(text, pageBgs...)
		chosen := false
		for _, bgs := range [][]string{all, page} {
			if themeLink != "" && reads(themeLink, minTextContrast, bgs...) {
				d.set("body-link-color", themeLink)
				chosen = true
				break
			}
			if reads(browserLinkColor, minTextContrast, bgs...) {
				chosen = true // queda el del navegador
				break
			}
			if reads(darkThemeLinkColor, minTextContrast, bgs...) {
				d.set("body-link-color", darkThemeLinkColor)
				chosen = true
				break
			}
		}
		if !chosen && text != "" && reads(text, minTextContrast, pageBgs...) {
			d.set("body-link-color", text)
		}
	}
	// Celdas: la tabla y la fila alterna, cada una con su fondo. La fila
	// alterna hereda el vínculo de la tabla si no tiene uno propio.
	body := d.themed("body-link-color")
	if body == "" {
		body = browserLinkColor
	}
	pickFor := func(bg string, fallbackFg string) (string, bool) {
		return pickText(minTextContrast, []string{bg}, themeLink, browserLinkColor, darkThemeLinkColor, fallbackFg, blackOrWhite(bg))
	}
	tableLink := body
	if !d.defined("table-link-color") && !reads(body, minTextContrast, tableBg) {
		if c, ok := pickFor(tableBg, tableFg); ok {
			d.set("table-link-color", c)
		}
	}
	if v := d.themed("table-link-color"); v != "" {
		tableLink = v
	}
	if !d.defined("table-stripe-link-color") && stripe != tableBg && !reads(tableLink, minTextContrast, stripe) {
		if c, ok := pickFor(stripe, tableFg); ok && c != tableLink {
			d.set("table-stripe-link-color", c)
		}
	}
}

// inkSurface es una superficie con fondo propio donde puede ir texto con
// tinta de estado: el nombre de sus variables, su fondo y su texto.
type inkSurface struct {
	name, bg, fg string
}

// inks deriva las tintas de estado. La del documento (--doclang-<tinta>-color)
// solo si el tema no la define y el literal no se lee sobre la página; la de
// cada superficie (--doclang-<superficie>-ink-<tinta>) solo donde la del
// documento no se lee sobre su fondo: el literal, su versión oscura, su
// versión clara, el texto de la superficie o, como último recurso, el negro o
// el blanco, el que más contraste dé. Un fondo que no se puede medir (el
// degradado literal del encabezado de la tabla) se deja como está.
func (d *themeDeriver) inks(pageBgs []string, bodyText string, surfaces []inkSurface) {
	for _, ink := range inks.Names {
		name := ink + "-color"
		if !d.defined(name) && !reads(inkLiterals[ink], minTextContrast, pageBgs...) {
			if c, ok := pickText(minTextContrast, pageBgs, inkDeep[ink], inkLight[ink], bodyText); ok {
				d.set(name, c)
			}
		}
		doc := d.resolved(name, inkLiterals[ink])
		for _, s := range surfaces {
			if s.bg == "" || reads(doc, minTextContrast, s.bg) {
				continue
			}
			if _, ok := a11y.ContrastRatio("#000000", s.bg); !ok {
				continue
			}
			c, ok := pickText(minTextContrast, []string{s.bg}, inkLiterals[ink], inkDeep[ink], inkLight[ink], s.fg)
			if !ok {
				c = blackOrWhite(s.bg)
			}
			d.set(strings.TrimPrefix(inks.Variable(s.name, ink), "--doclang-"), c)
		}
	}
}

// blackOrWhite es el negro o el blanco, el que más contraste dé sobre bg.
func blackOrWhite(bg string) string {
	b, _ := a11y.ContrastRatio("#000000", bg)
	w, _ := a11y.ContrastRatio("#ffffff", bg)
	if w > b {
		return "#ffffff"
	}
	return "#000000"
}

// tableHeaderLink: un vínculo en el encabezado de la tabla toma el del cuerpo
// mientras se lea sobre ese fondo; si no, el del tema, el del navegador, el
// azul claro de respaldo, el texto del encabezado o el negro o el blanco, el
// primero que se lea.
func (d *themeDeriver) tableHeaderLink(thBg, thFg string) {
	if thBg == "" || d.defined("table-header-link-color") {
		return
	}
	cur := d.themed("body-link-color")
	if cur == "" {
		cur = browserLinkColor
	}
	if reads(cur, minTextContrast, thBg) {
		return
	}
	themeLink, _ := d.get("link-color")
	if c, ok := pickText(minTextContrast, []string{thBg}, themeLink, browserLinkColor, darkThemeLinkColor, thFg, blackOrWhite(thBg)); ok {
		d.set("table-header-link-color", c)
	}
}

// kbdColor: sin --doclang-kbd-color la tecla hereda el texto de donde esté
// (la página, una alerta, el encabezado de la tabla...), pero su fondo es
// siempre --doclang-kbd-bg. Si alguno de esos textos no se lee sobre él, la
// tecla lleva color propio: el texto del cuerpo o, si tampoco, el negro o el
// blanco de siempre. A diferencia de las tintas, aquí cuenta también una
// superficie cuyo fondo no se puede medir (el degradado literal del
// encabezado de la tabla): la tecla pinta su propio fondo, que sí se mide, y
// el texto que hereda de esa superficie se conoce.
func (d *themeDeriver) kbdColor(bodyText string, surfaces []inkSurface) {
	if d.defined("kbd-color") {
		return
	}
	bg := d.resolved("kbd-bg", cssFallbacks["kbd-bg"])
	inherited := []string{bodyText}
	for _, s := range surfaces {
		inherited = append(inherited, s.fg)
	}
	for _, fg := range inherited {
		if !reads(fg, minTextContrast, bg) {
			if c, ok := pickText(minTextContrast, []string{bg}, bodyText, "#1a202c", "#ffffff"); ok {
				d.set("kbd-color", c)
			}
			return
		}
	}
}

// Modo oscuro del viewer (html[data-theme="dark"], también con
// theme_mode: dark): redefine algunas variables del contrato encima de las
// del tema, entre ellas el fondo del código y del código en línea y el
// texto del cuerpo, pero no los colores del código. Un color de código
// derivado aquí sigue pintándose en ese modo sobre el fondo oscuro.
const (
	viewerDarkCodeBg = "#2d2d2d" // --doclang-code-bg y --doclang-code-inline-bg
	viewerDarkText   = "#e0e0e0" // --doclang-text-color
	textColorRef     = "var(--doclang-text-color)"
)

// codeCandidate es un color candidato para el código: lo que se emite y lo
// que se pinta en el modo claro y en el oscuro del viewer.
type codeCandidate struct{ emit, light, dark string }

// codePairs: el código en línea y el bloque de código pintan su propio
// fondo, así que solo se mide su par (ver selfPainted).
func (d *themeDeriver) codePairs(text, stripe, sheet string) {
	codeBg, _ := d.get("code-bg")
	codeColor, _ := d.get("code-color")
	// El texto del cuerpo como referencia y no como copia: en el modo
	// oscuro del viewer sigue al texto claro de ese modo, que se lee sobre
	// su fondo de código; una copia quedaría oscura sobre oscuro.
	textRef := codeCandidate{}
	if text != "" {
		textRef = codeCandidate{textColorRef, text, viewerDarkText}
	}
	// Sin ninguna mitad del par, el rosa literal no se lee sobre el gris
	// literal (3.59:1) ni sobre el fondo del modo oscuro (3.61:1). El texto
	// del tema (como referencia) se lee en el modo oscuro; en el claro, sobre
	// el gris literal si el tema es claro, y si no, sobre la hoja o el fondo
	// del bloque de código del tema, que pasa a ser el del código en línea.
	fullPair := false
	if !d.defined("code-inline-bg") && !d.defined("code-inline-color") && text != "" &&
		!reads(text, minTextContrast, cssFallbacks["code-inline-bg"]) {
		for _, bg := range []string{sheet, codeBg} {
			if bg != "" && reads(text, minTextContrast, bg) {
				d.set("code-inline-bg", bg)
				d.set("code-inline-color", textColorRef)
				fullPair = true
				break
			}
		}
	}
	if !fullPair {
		d.selfPainted("code-inline-bg", "code-inline-color",
			[]string{stripe, sheet, codeBg, "#1e1e1e", text},
			[]codeCandidate{{codeInlineDeepColor, codeInlineDeepColor, codeInlineDeepColor}, textRef,
				{codeColor, codeColor, codeColor}})
	}
	d.selfPainted("code-bg", "code-color",
		[]string{stripe, sheet, cssFallbacks["code-bg"], text},
		[]codeCandidate{textRef, {"#1a202c", "#1a202c", "#1a202c"}})
}

// selfPainted ajusta un par fondo/texto de un elemento que pinta su propio
// fondo cuando no se lee. Si el tema puso solo el color, toma el primer fondo
// de bgCandidates donde se lea. Si no puso el color, el primero de
// fgCandidates que se lea sobre el fondo y que en el modo oscuro del viewer
// (fondo viewerDarkCodeBg) no quede peor que lo de antes. Un par completo del
// tema no se toca.
func (d *themeDeriver) selfPainted(bgName, fgName string, bgCandidates []string, fgCandidates []codeCandidate) {
	bg := d.resolved(bgName, cssFallbacks[bgName])
	fg := d.resolved(fgName, cssFallbacks[fgName])
	if reads(fg, minTextContrast, bg) {
		return
	}
	fgFromTheme, bgFromTheme := d.defined(fgName), d.defined(bgName)
	if fgFromTheme && !bgFromTheme {
		for _, b := range bgCandidates {
			if b != "" && reads(fg, minTextContrast, b) {
				d.set(bgName, b)
				return
			}
		}
		return
	}
	if fgFromTheme {
		return
	}
	before, _ := a11y.ContrastRatio(fg, viewerDarkCodeBg)
	darkMin := math.Min(minTextContrast, before)
	for _, c := range fgCandidates {
		if c.emit == "" || !reads(c.light, minTextContrast, bg) || !reads(c.dark, darkMin, viewerDarkCodeBg) {
			continue
		}
		d.set(fgName, c.emit)
		return
	}
}

// quizCorrect arregla la opción correcta del quiz cuando el tema define solo
// una mitad del par --doclang-alert-success-bg / -color, que el quiz usa como
// respaldo y que así queda ilegible.
func (d *themeDeriver) quizCorrect(text, stripe, sheet string) {
	if d.defined("quiz-correct-bg") || d.defined("quiz-correct-color") {
		return
	}
	bg, bgFromTheme := d.get("alert-success-bg")
	if !bgFromTheme {
		bg = quizCorrectBgLiteral
	}
	fg, fgFromTheme := d.get("alert-success-color")
	if !fgFromTheme {
		fg = quizCorrectFgLiteral
	}
	if reads(fg, minTextContrast, bg) {
		return
	}
	fixFg := func() bool {
		if c, ok := pickText(minTextContrast, []string{bg}, text, quizCorrectFgLiteral); ok {
			d.set("quiz-correct-color", c)
			return true
		}
		return false
	}
	fixBg := func() bool {
		for _, b := range []string{stripe, sheet, quizCorrectBgLiteral} {
			if reads(fg, minTextContrast, b) {
				d.set("quiz-correct-bg", b)
				return true
			}
		}
		return false
	}
	// Se conserva la mitad que puso el tema y se ajusta la otra.
	if fgFromTheme && !bgFromTheme {
		_ = fixBg() || fixFg()
		return
	}
	_ = fixFg() || fixBg()
}
