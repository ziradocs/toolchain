// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/themes/document"
)

// bindingNames son las variables de superficie que lee el CSS del documento
// (core/renderer/document_html.go).
var bindingNames = []string{
	"--doclang-table-bg", "--doclang-table-color",
	"--doclang-panel-bg", "--doclang-panel-border", "--doclang-panel-color",
	"--doclang-chart-bg",
	"--doclang-kbd-bg", "--doclang-kbd-border", "--doclang-kbd-color",
	"--doclang-blockquote-bg", "--doclang-blockquote-border", "--doclang-blockquote-color", "--doclang-blockquote-footer-color",
	"--doclang-element-title-color", "--doclang-caption-color",
	"--doclang-code-tab-active-bg", "--doclang-code-tab-active-color", "--doclang-code-tab-accent",
	"--doclang-body-link-color", "--doclang-body-link-active-color", "--doclang-print-margin-bg",
	"--doclang-callout-info-bg", "--doclang-callout-info-border", "--doclang-callout-info-color",
	"--doclang-callout-warning-bg", "--doclang-callout-warning-border", "--doclang-callout-warning-color",
	"--doclang-callout-danger-bg", "--doclang-callout-danger-border", "--doclang-callout-danger-color",
	"--doclang-callout-success-bg", "--doclang-callout-success-border", "--doclang-callout-success-color",
	"--doclang-callout-tip-bg", "--doclang-callout-tip-border", "--doclang-callout-tip-color",
	"--doclang-blockquote-link-color",
	"--doclang-callout-info-link-color", "--doclang-callout-warning-link-color", "--doclang-callout-danger-link-color",
	"--doclang-callout-success-link-color", "--doclang-callout-tip-link-color",
}

// El render de los temas embebidos no cambia porque ninguno define una
// variable de superficie y htmlThemeVariables no les deriva ninguna. Si un
// tema embebido llega a definir una, su HTML/PDF cambia: que sea a propósito.
func TestHTMLThemeVariables_EmbeddedThemesUntouched(t *testing.T) {
	for name, theme := range document.EmbeddedThemes {
		for _, b := range bindingNames {
			if _, ok := theme.Variables[b]; ok {
				t.Errorf("el tema embebido %s define %s", name, b)
			}
		}
		got := htmlThemeVariables(theme.Variables, false)
		if !reflect.DeepEqual(got, map[string]string(theme.Variables)) {
			t.Errorf("htmlThemeVariables cambió las variables del tema embebido %s", name)
		}
	}
}

// TestCSSFallbacksMatchCore: la derivación mide el contraste contra los
// fallbacks del CSS del core; si el core cambia uno, esta tabla tiene que
// cambiar con él. También es la prueba de render que falla contra un core
// publicado sin las variables de superficie (GOWORK=off con un pin viejo):
// ahí el HTML no las lee y derivarlas no tendría efecto.
func TestCSSFallbacksMatchCore(t *testing.T) {
	doc, _ := parser.New(util.NewNoop()).ParseDocument("---\ntitle: T\n---\n\n# T\n\nText.\n", "t.doclang")
	html := renderer.GenerateDocumentHTML(doc, renderer.DocumentHTMLOptions{
		Title: "T", InteractiveViewer: true, TOC: true,
		MermaidMode: "offline-inline", ChartMode: "offline-inline", PlantUMLMode: "offline-inline",
		MapMode: "offline-inline", MathMode: "offline-inline",
	}, renderer.NewDefaultRenderContext())

	want := []string{}
	for name, v := range cssFallbacks {
		want = append(want, "var(--doclang-"+name+", "+v+")")
	}
	want = append(want,
		"var(--doclang-toc-bg, "+tocBgLiteral+")",
		"var(--doclang-toc-bg, "+themeToggleBgLiteral+")",
		"var(--doclang-toc-title-color, "+tocTitleLiteral+")",
		"var(--doclang-toc-link-color, "+tocLinkLiteral+")",
		"var(--doclang-toc-link-color, "+sidebarTocLinkLiteral+")",
		"var(--doclang-toc-subsection-color, "+tocSubsectionLiteral+")",
		"var(--doclang-quote-bg, "+quizBgLiteral+")",
		"var(--doclang-alert-success-bg, "+quizCorrectBgLiteral+")",
		"var(--doclang-alert-success-color, "+quizCorrectFgLiteral+")",
		"var(--doclang-text-light, "+headerFooterFgLiteral+")",
		"var(--doclang-body-link-color, LinkText)",
		"var(--doclang-print-margin-bg, transparent)",
	)
	for _, w := range want {
		if !strings.Contains(html, w) {
			t.Errorf("el CSS del documento no tiene %s", w)
		}
	}
}

func darkExternalVars() map[string]string {
	return map[string]string{
		"--doclang-page-bg":             "#0f172a",
		"--doclang-font-main":           "Arial, sans-serif",
		"--doclang-h1-color":            "#f1f5f9",
		"--doclang-text-color":          "#e2e8f0",
		"--doclang-text-light":          "#94a3b8",
		"--doclang-code-bg":             "#1e293b",
		"--doclang-code-color":          "#f8fafc",
		"--doclang-table-border":        "#334155",
		"--doclang-table-stripe-bg":     "#1e293b",
		"--doclang-link-color":          "#7dd3fc",
		"--doclang-quote-bg":            "#1e293b",
		"--doclang-quote-border":        "#38bdf8",
		"--doclang-alert-info-bg":       "#082f49",
		"--doclang-alert-info-color":    "#bae6fd",
		"--doclang-alert-info-border":   "#0ea5e9",
		"--doclang-alert-warning-bg":    "#451a03", // sin color propio: el del cuerpo
		"--doclang-alert-danger-bg":     "#fff5f5", // claro: conserva su texto literal
		"--doclang-alert-success-bg":    "#052e16",
		"--doclang-alert-success-color": "#bbf7d0",
	}
}

func TestHTMLThemeVariables_DarkTheme(t *testing.T) {
	in := darkExternalVars()
	got := htmlThemeVariables(in, true)

	want := map[string]string{
		"--doclang-page-surface-bg":         "#0f172a",
		"--doclang-print-margin-bg":         "#0f172a",
		"--doclang-body-link-color":         "#7dd3fc",
		"--doclang-table-bg":                "#0f172a",
		"--doclang-table-hover-bg":          "#1e293b",
		"--doclang-panel-bg":                "#1e293b",
		"--doclang-panel-border":            "#334155",
		"--doclang-kbd-bg":                  "#1e293b",
		"--doclang-kbd-border":              "#334155",
		"--doclang-header-footer-bg":        "#0f172a",
		"--doclang-h2-color":                "#f1f5f9",
		"--doclang-h3-color":                "#f1f5f9",
		"--doclang-element-title-color":     "#e2e8f0",
		"--doclang-blockquote-bg":           "#1e293b",
		"--doclang-blockquote-color":        "#e2e8f0",
		"--doclang-blockquote-border":       "#38bdf8",
		"--doclang-blockquote-link-color":   "#7dd3fc",
		"--doclang-callout-info-bg":         "#082f49",
		"--doclang-callout-info-color":      "#bae6fd",
		"--doclang-callout-info-border":     "#0ea5e9",
		"--doclang-callout-info-link-color": "#7dd3fc",
		"--doclang-callout-warning-bg":      "#451a03",
		"--doclang-callout-warning-color":   "#e2e8f0",
		"--doclang-callout-danger-bg":       "#fff5f5",
		"--doclang-callout-success-bg":      "#052e16",
		"--doclang-callout-success-color":   "#bbf7d0",
		"--doclang-code-tab-active-bg":      "#1e293b",
		"--doclang-sidebar-bg":              "#0f172a",
		"--doclang-toc-bg":                  "#0f172a",
		"--doclang-toc-title-color":         "#e2e8f0",
		"--doclang-toc-link-color":          "#e2e8f0",
		"--doclang-toc-subsection-color":    "#e2e8f0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	for _, k := range []string{
		"--doclang-chart-bg", "--doclang-table-color", "--doclang-panel-color", "--doclang-kbd-color",
		"--doclang-code-tab-accent", "--doclang-body-link-active-color",
		"--doclang-code-tab-active-color",     // el blanco de siempre se lee sobre #1e293b
		"--doclang-callout-danger-color",      // el literal se lee sobre el fondo claro
		"--doclang-callout-danger-link-color", // el azul del navegador se lee ahí
		"--doclang-table-stripe-bg",           // la define el tema
		"--doclang-quiz-bg",                   // usa --doclang-quote-bg, que se lee
		"--doclang-quiz-correct-bg", "--doclang-quiz-correct-color",
		"--doclang-h4-color", // #718096 da 4.2:1 sobre #0f172a, más de 3:1 para un título
	} {
		if v, ok := got[k]; ok && in[k] == "" {
			t.Errorf("%s no debe derivarse, salió %q", k, v)
		}
	}
	for k, v := range in {
		if got[k] != v {
			t.Errorf("%s del tema cambió: %q", k, got[k])
		}
	}
	if !reflect.DeepEqual(in, darkExternalVars()) {
		t.Error("htmlThemeVariables modificó el mapa de entrada")
	}
}

// cssPair modela una pareja texto/fondo del CSS del documento con sus cadenas
// de fallback, para medir el contraste que se pinta con y sin derivación.
type cssPair struct {
	what   string
	fg, bg []string // variables en orden de fallback; el último es el literal
	min    float64
}

var documentPairs = func() []cssPair {
	text := []string{"--doclang-text-color", "#2c3e50"}
	link := []string{"--doclang-body-link-color", "#0000ee"}
	page := []string{"--doclang-page-bg", "#ffffff"}
	table := []string{"--doclang-table-bg", "white"}
	stripe := []string{"--doclang-table-stripe-bg", "#f8fafc"}
	panel := []string{"--doclang-panel-bg", "#f8fafc"}
	sidebar := []string{"--doclang-sidebar-bg", "#f8f9fa"}
	pairs := []cssPair{
		{"texto sobre la hoja", text, page, 4.5},
		{"texto de la hoja de page-view", text, []string{"--doclang-page-surface-bg", "#ffffff"}, 4.5},
		{"tabla", text, table, 4.5},
		{"fila alterna", text, stripe, 4.5},
		{"panel", text, panel, 4.5},
		{"tecla", text, []string{"--doclang-kbd-bg", "#f7f7f7"}, 4.5},
		{"vínculo sobre la hoja", link, page, 4.5},
		{"vínculo en la tabla", link, table, 4.5},
		{"vínculo en la fila alterna", link, stripe, 4.5},
		{"vínculo en un panel", link, panel, 4.5},
		{"h2", []string{"--doclang-h2-color", "#2d3748"}, page, 3},
		{"h3", []string{"--doclang-h3-color", "#4a5568"}, page, 3},
		{"h4", []string{"--doclang-h4-color", "#718096"}, page, 3},
		{"título de elemento", []string{"--doclang-element-title-color", "#2d3748"}, page, 4.5},
		{"pie de foto", []string{"--doclang-caption-color", "#718096"}, page, 3},
		{"cita", []string{"--doclang-blockquote-color", "#4a5568"}, []string{"--doclang-blockquote-bg", "#f7fafc"}, 4.5},
		{"vínculo en la cita", []string{"--doclang-blockquote-link-color", "#0000ee"}, []string{"--doclang-blockquote-bg", "#f7fafc"}, 4.5},
		{"encabezado de page-view", []string{"--doclang-text-light", "#666"}, []string{"--doclang-header-footer-bg", "#fafafa"}, 4.5},
		{"pregunta de quiz", []string{"--doclang-quiz-question-color", "--doclang-text-color", "#2c3e50"}, []string{"--doclang-quiz-bg", "--doclang-quote-bg", "#f8fafc"}, 4.5},
		{"opción correcta", []string{"--doclang-quiz-correct-color", "--doclang-alert-success-color", "#22543d"}, []string{"--doclang-quiz-correct-bg", "--doclang-alert-success-bg", "#f0fff4"}, 4.5},
		{"pestaña activa", []string{"--doclang-code-tab-active-color", "#fff"}, []string{"--doclang-code-tab-active-bg", "#1e1e1e"}, 4.5},
		{"sidebar", text, sidebar, 4.5},
		{"vínculo del TOC del sidebar", []string{"--doclang-toc-link-color", "#4a5568"}, sidebar, 4.5},
		{"botón de modo oscuro", text, []string{"--doclang-toc-bg", "#e2e8f0"}, 4.5},
		{"título del TOC", []string{"--doclang-toc-title-color", "#1a202c"}, []string{"--doclang-toc-bg", "#f8f9fa"}, 3},
		{"vínculo del TOC", []string{"--doclang-toc-link-color", "#1a202c"}, []string{"--doclang-toc-bg", "#f8f9fa"}, 4.5},
	}
	for _, k := range []struct{ kind, bg, fg string }{
		{"info", "#ebf8ff", "#2c5282"}, {"warning", "#fffaf0", "#7c2d12"}, {"danger", "#fff5f5", "#742a2a"},
		{"success", "#f0fff4", "#22543d"}, {"tip", "#faf5ff", "#44337a"},
	} {
		bg := []string{"--doclang-callout-" + k.kind + "-bg", k.bg}
		pairs = append(pairs,
			cssPair{"alerta " + k.kind, []string{"--doclang-callout-" + k.kind + "-color", k.fg}, bg, 4.5},
			cssPair{"vínculo en alerta " + k.kind, []string{"--doclang-callout-" + k.kind + "-link-color", "#0000ee"}, bg, 4.5},
		)
	}
	return pairs
}()

func resolve(vars map[string]string, chain []string) string {
	for _, c := range chain[:len(chain)-1] {
		if v, ok := vars[c]; ok {
			return v
		}
	}
	return chain[len(chain)-1]
}

// checkPairs exige que ninguna pareja quede peor que sin derivación, salvo
// que llegue al mínimo; con strict, además, que todas lleguen al mínimo.
func checkPairs(t *testing.T, name string, theme map[string]string, strict bool) {
	t.Helper()
	after := htmlThemeVariables(theme, true)
	text := []string{"--doclang-text-color", "#2c3e50"}
	for _, p := range documentPairs {
		// Un vínculo sobre una superficie donde ni el texto del tema se lee
		// no se juzga: ningún color de vínculo arregla esa superficie (ver
		// el comentario del vínculo del cuerpo en htmlThemeVariables).
		if p.fg[0] == "--doclang-body-link-color" && !reads(resolve(after, text), minTextContrast, resolve(after, p.bg)) {
			continue
		}
		rb, okb := a11y.ContrastRatio(resolve(theme, p.fg), resolve(theme, p.bg))
		ra, oka := a11y.ContrastRatio(resolve(after, p.fg), resolve(after, p.bg))
		if !oka {
			continue
		}
		if okb && ra < rb-0.01 && ra < p.min {
			t.Errorf("%s: %s empeora de %.2f a %.2f (%s sobre %s)", name, p.what, rb, ra, resolve(after, p.fg), resolve(after, p.bg))
		}
		if strict && ra < p.min {
			t.Errorf("%s: %s queda en %.2f, menos de %.1f (%s sobre %s)", name, p.what, ra, p.min, resolve(after, p.fg), resolve(after, p.bg))
		}
	}
}

func TestHTMLThemeVariables_ContrastNeverWorse(t *testing.T) {
	for name, theme := range document.EmbeddedThemes {
		checkPairs(t, name+" como externo", theme.Variables, false)
	}
	checkPairs(t, "oscuro", darkExternalVars(), true)
	checkPairs(t, "oscuro mínimo", minimalDarkVars(), true)
	checkPairs(t, "crema", map[string]string{
		"--doclang-page-bg": "#fdf6e3", "--doclang-text-color": "#3b2f1e", "--doclang-link-color": "#8b4513",
		"--doclang-quote-bg": "#f3e9cf", "--doclang-quote-border": "#b58900", "--doclang-quote-color": "#5c4a2e",
		"--doclang-table-border": "#d6c7a1", "--doclang-code-bg": "#2b2b2b",
	}, false)
}

// professional copiado como tema externo es el caso de un tema claro que ya
// existe: lo que su literal ya resolvía bien no se toca.
func TestHTMLThemeVariables_LightThemeKeepsLegibleLiterals(t *testing.T) {
	in := document.GetProfessionalTheme().Variables
	got := htmlThemeVariables(in, true)
	for _, k := range []string{
		"--doclang-body-link-color",         // #3498db da 3.15:1 sobre blanco: queda el del navegador
		"--doclang-callout-info-link-color", // ídem sobre el fondo de la alerta
		"--doclang-blockquote-link-color",   // ídem
		"--doclang-callout-warning-color",   // el literal se lee: conserva su tinte
		"--doclang-callout-success-color",   // ídem
		"--doclang-callout-danger-color",    // ídem
		"--doclang-code-tab-active-color",   // el blanco se lee sobre #1e1e1e
		"--doclang-kbd-bg", "--doclang-panel-bg", "--doclang-table-bg", "--doclang-table-hover-bg",
		"--doclang-quiz-bg", "--doclang-sidebar-bg", "--doclang-element-title-color", "--doclang-caption-color",
	} {
		if v, ok := got[k]; ok && v != in[k] {
			t.Errorf("%s no debe derivarse en un tema claro, salió %q", k, v)
		}
	}
}

func minimalDarkVars() map[string]string {
	return map[string]string{
		"--doclang-page-bg":      "#0f172a",
		"--doclang-font-main":    "Arial",
		"--doclang-h1-color":     "#ffffff",
		"--doclang-text-color":   "#e2e8f0",
		"--doclang-code-bg":      "#1e293b",
		"--doclang-table-border": "#334155",
	}
}

// Un tema oscuro mínimo (solo lo obligatorio): sin color de vínculo, el del
// navegador no se lee y se usa el azul claro de respaldo; h2-h4 toman h1.
func TestHTMLThemeVariables_MinimalDarkTheme(t *testing.T) {
	got := htmlThemeVariables(minimalDarkVars(), true)
	want := map[string]string{
		"--doclang-body-link-color": darkThemeLinkColor,
		"--doclang-table-bg":        "#0f172a",
		"--doclang-table-stripe-bg": "#0f172a",
		"--doclang-quiz-bg":         "#0f172a",
		"--doclang-h2-color":        "#ffffff",
		"--doclang-h3-color":        "#ffffff",
		"--doclang-page-surface-bg": "#0f172a",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	// Cita y alertas conservan sus fondos claros literales: sus vínculos se
	// quedan con el azul del navegador, que ahí sí se lee.
	for _, k := range []string{"--doclang-blockquote-bg", "--doclang-blockquote-link-color", "--doclang-callout-info-bg", "--doclang-callout-info-link-color"} {
		if v, ok := got[k]; ok {
			t.Errorf("%s no debe derivarse sin el fondo de esa superficie, salió %q", k, v)
		}
	}

	// Un color de vínculo del tema que no se lee sobre la hoja tampoco se usa.
	vars := minimalDarkVars()
	vars["--doclang-link-color"] = "#1e3a8a"
	if got := htmlThemeVariables(vars, true)["--doclang-body-link-color"]; got != darkThemeLinkColor {
		t.Errorf("vínculo ilegible del tema: body-link-color = %q, want %q", got, darkThemeLinkColor)
	}
}

func TestHTMLThemeVariables_QuizCorrectHalfPair(t *testing.T) {
	// Solo el color, claro: el fondo literal (#f0fff4) no lo deja leer.
	vars := minimalDarkVars()
	vars["--doclang-alert-success-color"] = "#bbf7d0"
	got := htmlThemeVariables(vars, true)
	if got["--doclang-quiz-correct-bg"] != "#0f172a" {
		t.Errorf("quiz-correct-bg = %q, want la hoja", got["--doclang-quiz-correct-bg"])
	}
	// Solo el fondo, oscuro: el texto literal (#22543d) no se lee.
	vars = minimalDarkVars()
	vars["--doclang-alert-success-bg"] = "#052e16"
	got = htmlThemeVariables(vars, true)
	if got["--doclang-quiz-correct-color"] != "#e2e8f0" {
		t.Errorf("quiz-correct-color = %q, want el texto del cuerpo", got["--doclang-quiz-correct-color"])
	}
}

func TestHTMLThemeVariables_ThemeOwnBindingsWin(t *testing.T) {
	in := darkExternalVars()
	in["--doclang-table-bg"] = "#000000"
	in["--doclang-body-link-color"] = "#ffcc00"
	got := htmlThemeVariables(in, true)
	if got["--doclang-table-bg"] != "#000000" || got["--doclang-body-link-color"] != "#ffcc00" {
		t.Errorf("una variable de superficie del tema fue pisada: table-bg=%q link=%q",
			got["--doclang-table-bg"], got["--doclang-body-link-color"])
	}
}

// page-view: la hoja es --doclang-page-surface-bg; sin ella un tema oscuro
// pintaba texto claro sobre blanco. Si el tema la define, se respeta.
func TestHTMLThemeVariables_PageViewSurface(t *testing.T) {
	got := htmlThemeVariables(minimalDarkVars(), true)
	if got["--doclang-page-surface-bg"] != "#0f172a" {
		t.Errorf("page-surface-bg = %q, want --doclang-page-bg", got["--doclang-page-surface-bg"])
	}
	vars := minimalDarkVars()
	vars["--doclang-page-surface-bg"] = "#111827"
	got = htmlThemeVariables(vars, true)
	if got["--doclang-page-surface-bg"] != "#111827" || got["--doclang-table-bg"] != "#111827" {
		t.Errorf("la hoja propia del tema no se respetó: surface=%q table=%q", got["--doclang-page-surface-bg"], got["--doclang-table-bg"])
	}
	if got["--doclang-print-margin-bg"] != "#0f172a" {
		t.Errorf("el margen del PDF es el lienzo (--doclang-page-bg), salió %q", got["--doclang-print-margin-bg"])
	}
}

// Un valor inseguro no se copia a otro nombre: además del saneo de siempre,
// la derivación rechaza comentarios, escapes y comillas desbalanceadas.
func TestHTMLThemeVariables_UnsafeSourceNotCopied(t *testing.T) {
	for _, evil := range []string{
		`red; } </style><script>alert(1)</script>`,
		"#000 /* x",
		"#000 */",
		`"#000`,
		`'#000`,
		`\23 000`,
	} {
		vars := minimalDarkVars()
		vars["--doclang-page-bg"] = evil
		vars["--doclang-link-color"] = evil
		vars["--doclang-quote-bg"] = evil
		got := htmlThemeVariables(vars, true)
		for _, k := range []string{"--doclang-page-surface-bg", "--doclang-print-margin-bg", "--doclang-body-link-color", "--doclang-blockquote-bg"} {
			if v, ok := got[k]; ok && v == evil {
				t.Errorf("%s se derivó de un valor rechazado: %q", k, v)
			}
		}
	}
	// Un fondo propio del tema que no es seguro no habilita el vínculo de la
	// cita: ese fondo se descarta al emitirse y queda el claro literal.
	vars := minimalDarkVars()
	vars["--doclang-link-color"] = "#7dd3fc"
	vars["--doclang-blockquote-bg"] = "#0f172a /* x"
	if v, ok := htmlThemeVariables(vars, true)["--doclang-blockquote-link-color"]; ok {
		t.Errorf("blockquote-link-color se derivó sobre un fondo inseguro: %q", v)
	}
}

// --- Las tres rutas que arman el HTML aplican la derivación ---------------

const pathDoc = "---\ntitle: T\n---\n\n# T\n\nA [link](https://example.com/) and [K]{.kbd}.\n\n| A | B |\n|---|---|\n| 1 | 2 |\n"

func pathTestDoc(t *testing.T) *ast.AST {
	t.Helper()
	doc, _ := parser.New(util.NewNoop()).ParseDocument(pathDoc, "t.doclang")
	if doc == nil {
		t.Fatal("ParseDocument devolvió nil")
	}
	return doc
}

func TestHTMLGenerate_ExternalThemeEmitsSurfaces(t *testing.T) {
	doc := pathTestDoc(t)
	render := func(external bool) string {
		out := filepath.Join(t.TempDir(), "t.html")
		err := NewHTMLGenerator(util.NewNoop()).Generate(doc, out, GeneratorOptions{
			Format:         "html",
			Theme:          "qa-dark",
			ThemeVariables: darkExternalVars(),
			ThemeExternal:  external,
		})
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	html := render(true)
	for _, decl := range []string{
		"--doclang-table-bg: #0f172a;",
		"--doclang-kbd-bg: #1e293b;",
		"--doclang-body-link-color: #7dd3fc;",
		"--doclang-print-margin-bg: #0f172a;",
		// Y el CSS que las lee (falla contra un core sin las variables).
		"background: var(--doclang-table-bg, white);",
		"color: var(--doclang-body-link-color, LinkText);",
	} {
		if !strings.Contains(html, decl) {
			t.Errorf("el HTML del tema externo no tiene %q", decl)
		}
	}

	notExternal := render(false)
	for _, b := range bindingNames {
		if strings.Contains(notExternal, b+":") {
			t.Errorf("un tema no externo declara %s", b)
		}
	}
}

func TestPDFDocumentOptions_DerivesForExternalTheme(t *testing.T) {
	opts := GeneratorOptions{Format: "pdf", ThemeVariables: darkExternalVars(), ThemeExternal: true}
	got := pdfDocumentOptions(opts, "T").ThemeVariables
	if got["--doclang-table-bg"] != "#0f172a" || got["--doclang-print-margin-bg"] != "#0f172a" {
		t.Errorf("el PDF no recibe las superficies derivadas: table-bg=%q margin=%q", got["--doclang-table-bg"], got["--doclang-print-margin-bg"])
	}
	opts.ThemeExternal = false
	if _, ok := pdfDocumentOptions(opts, "T").ThemeVariables["--doclang-table-bg"]; ok {
		t.Error("un tema no externo recibe superficies derivadas en el PDF")
	}
}

func TestRenderHTMLPreview_DerivesForExternalTheme(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "themes"), 0o755); err != nil {
		t.Fatal(err)
	}
	theme := map[string]any{"name": "qa-preview-dark", "variables": darkExternalVars()}
	b, _ := json.Marshal(theme)
	if err := os.WriteFile(filepath.Join(dir, "themes", "qa-preview-dark.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir) // el loader busca en ./themes

	html := New(util.NewNoop()).RenderHTMLPreview(pathTestDoc(t), "qa-preview-dark")
	if !strings.Contains(html, "--doclang-table-bg: #0f172a;") {
		t.Error("la vista previa no recibe las superficies derivadas del tema externo")
	}
	embedded := New(util.NewNoop()).RenderHTMLPreview(pathTestDoc(t), "professional")
	if strings.Contains(embedded, "--doclang-table-bg:") {
		t.Error("la vista previa de un tema embebido declara una superficie derivada")
	}
}

// La lista de variables estándar del tema documenta las de superficie.
func TestStandardVariables_ListSurfaceBindings(t *testing.T) {
	std := map[string]bool{}
	for _, v := range document.StandardVariables {
		std[v] = true
	}
	for _, b := range bindingNames {
		if !std[b] {
			t.Errorf("document.StandardVariables no lista %s", b)
		}
	}
}

// Con header/footer, Chromium imprime en el margen del PDF con texto negro.
// Si el tema pinta el margen, el texto toma un color del tema legible ahí; si
// no hay ninguno, el margen no se pinta.
func TestPDFMarginForChrome(t *testing.T) {
	dark := htmlThemeVariables(darkExternalVars(), true)
	vars, color := pdfMarginForChrome(dark, darkExternalVars())
	if color != "#94a3b8" || vars["--doclang-print-margin-bg"] != "#0f172a" {
		t.Errorf("tema oscuro: color=%q margen=%q, want #94a3b8 sobre #0f172a", color, vars["--doclang-print-margin-bg"])
	}

	unreadable := map[string]string{"--doclang-print-margin-bg": "#0f172a", "--doclang-text-color": "#111111"}
	vars, color = pdfMarginForChrome(unreadable, nil)
	if color != "" {
		t.Errorf("sin texto legible no debe haber color, salió %q", color)
	}
	if _, ok := vars["--doclang-print-margin-bg"]; ok {
		t.Error("sin texto legible el margen no debe pintarse")
	}
	if _, ok := unreadable["--doclang-print-margin-bg"]; !ok {
		t.Error("pdfMarginForChrome modificó el mapa de entrada")
	}

	embedded := document.GetProfessionalTheme().Variables
	if vars, color := pdfMarginForChrome(embedded, embedded); color != "" || !reflect.DeepEqual(vars, map[string]string(embedded)) {
		t.Error("un tema sin margen pintado no debe cambiar")
	}

	// Un margen que el tema pintó por su cuenta se respeta aunque no haya
	// texto legible: es la variable del tema.
	if vars, _ := pdfMarginForChrome(unreadable, unreadable); vars["--doclang-print-margin-bg"] != "#0f172a" {
		t.Error("se quitó un --doclang-print-margin-bg definido por el tema")
	}

	if got := withChromeColor("<span>x</span>", ""); got != "<span>x</span>" {
		t.Errorf("sin color el template no cambia, salió %q", got)
	}
	if got := withChromeColor("<span>x</span>", "#94a3b8"); got != `<div style="color:#94a3b8; width:100%;"><span>x</span></div>` {
		t.Errorf("withChromeColor = %q", got)
	}
}

// Un tema claro copiado con la página oscura y sus colores de TOC oscuros: el
// fondo del TOC no puede pasar a la hoja oscura, porque esos colores
// quedarían ilegibles. Antes se medía solo el texto del cuerpo.
func TestHTMLThemeVariables_TOCKeepsLiteralWhenThemeColorsWouldFail(t *testing.T) {
	vars := map[string]string{}
	for k, v := range document.GetProfessionalTheme().Variables {
		vars[k] = v
	}
	delete(vars, "--doclang-toc-bg")
	vars["--doclang-page-bg"] = "#0f172a"
	vars["--doclang-text-color"] = "#e2e8f0"
	got := htmlThemeVariables(vars, true)
	if v, ok := got["--doclang-toc-bg"]; ok {
		t.Errorf("toc-bg = %q: los --doclang-toc-*-color del tema no se leen ahí", v)
	}
	if v, ok := got["--doclang-sidebar-bg"]; ok {
		t.Errorf("sidebar-bg = %q: --doclang-toc-link-color del tema no se lee ahí", v)
	}
	checkPairs(t, "professional con página oscura", vars, false)
}
