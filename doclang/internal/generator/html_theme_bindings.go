// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/renderer"
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
//     fondos donde aparece; si no, se queda el del navegador, y si ese
//     tampoco se lee (hoja oscura), un azul claro fijo.
//   - El fondo de las gráficas no se deriva: Chart.js y la imagen de la
//     gráfica usan su propia paleta clara.
//
// Un color que no se puede medir (un gradiente) nunca cuenta como legible.
//
// Las variables derivadas son copias de los valores, no var(): así el par que
// se midió es exactamente el que se pinta. Con var() seguirían al modo oscuro
// del viewer, pero ese modo solo redefine algunas variables del contrato y
// una referencia podría terminar en un par que nadie midió.
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
)

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
	if strings.Contains(v, "/*") || strings.Contains(v, "*/") || strings.Contains(v, `\`) {
		return false
	}
	return strings.Count(v, `"`)%2 == 0 && strings.Count(v, `'`)%2 == 0
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
// claro de respaldo.
func (d *themeDeriver) link(name string, bgs ...string) {
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
	if reads(darkThemeLinkColor, minTextContrast, bgs...) {
		d.set(name, darkThemeLinkColor)
	}
}

func (d *themeDeriver) derive() {
	text, _ := d.get("text-color")
	textLight, _ := d.get("text-light")

	// La hoja. En page-view la superficie es --doclang-page-surface-bg (sin
	// ella, blanca); un tema externo que no la define se pinta con su
	// --doclang-page-bg, como en el modo normal. El margen del PDF, también.
	if page, ok := d.get("page-bg"); ok {
		d.set("page-surface-bg", page)
		d.set("print-margin-bg", page)
	}
	sheet := d.resolved("page-surface-bg", cssFallbacks["page-surface-bg"])

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
	if d.implicitBg("kbd-bg", cssFallbacks["kbd-bg"], text, stripe, sheet) && hasBorder {
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
		d.implicitFg(h, cssFallbacks[h], minLargeContrast, []string{sheet}, h1, text)
	}
	d.implicitFg("element-title-color", cssFallbacks["element-title-color"], minTextContrast, []string{sheet}, text)
	d.implicitFg("caption-color", cssFallbacks["caption-color"], minLargeContrast, []string{sheet}, textLight, text)

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
		d.link("blockquote-link-color", bg)
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
			d.link("callout-"+kind+"-link-color", bg)
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
	// es --doclang-text-color.
	d.implicitBg("sidebar-bg", cssFallbacks["sidebar-bg"], text, sheet)
	sidebarBg := d.resolved("sidebar-bg", cssFallbacks["sidebar-bg"])
	d.implicitBg("toc-bg", themeToggleBgLiteral, text, sheet)
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

	// Vínculo del cuerpo: sobre la hoja y las superficies donde aparece.
	d.link("body-link-color", sheet, tableBg, stripe, panelBg)
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
