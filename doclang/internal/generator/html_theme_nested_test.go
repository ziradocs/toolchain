// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/doclang/v2/themes/document"
)

// nestedDarkVars es un tema externo oscuro con la forma de los que reportaron
// el defecto: tintas claras para la página, alertas warning/danger/success con
// su fondo claro literal (definido por el tema, sin color propio), tip sin
// definir, y el fondo del encabezado de la tabla igual al color de vínculo.
func nestedDarkVars() map[string]string {
	return map[string]string{
		"--doclang-page-bg":            "#0f172a",
		"--doclang-font-main":          "Arial",
		"--doclang-h1-color":           "#f1f5f9",
		"--doclang-text-color":         "#e2e8f0",
		"--doclang-text-light":         "#94a3b8",
		"--doclang-code-bg":            "#131d33",
		"--doclang-code-color":         "#e2e8f0",
		"--doclang-code-inline-bg":     "#131d33",
		"--doclang-code-inline-color":  "#93c5fd",
		"--doclang-table-border":       "#334155",
		"--doclang-table-stripe-bg":    "#131d33",
		"--doclang-table-header-bg":    "#8b7fff",
		"--doclang-table-header-color": "#0b0b10",
		"--doclang-link-color":         "#8b7fff",
		"--doclang-quote-bg":           "#131d33",
		"--doclang-quote-color":        "#e2e8f0",
		"--doclang-alert-info-bg":      "#131d33",
		"--doclang-alert-info-color":   "#e2e8f0",
		"--doclang-alert-warning-bg":   "#fffaf0",
		"--doclang-alert-danger-bg":    "#fff5f5",
		"--doclang-alert-success-bg":   "#f0fff4",
		"--doclang-danger-color":       "#fca5a5",
		"--doclang-info-color":         "#93c5fd",
		"--doclang-success-color":      "#6ee7b7",
		"--doclang-warning-color":      "#fcd34d",
		"--doclang-accent-color":       "#8b7fff",
	}
}

// Las alertas que el tema dejó claras siguen claras (es lo que el tema pidió),
// pero las tintas claras del tema no se leen ahí: cada alerta lleva las suyas.
func TestHTMLThemeVariables_InksInsideLightAlertsOfDarkTheme(t *testing.T) {
	in := nestedDarkVars()
	got := htmlThemeVariables(in, true)
	for kind, bg := range map[string]string{"warning": "#fffaf0", "danger": "#fff5f5", "success": "#f0fff4"} {
		if got["--doclang-callout-"+kind+"-bg"] != bg {
			t.Errorf("callout-%s-bg = %q, want el fondo claro del tema %s", kind, got["--doclang-callout-"+kind+"-bg"], bg)
		}
	}
	for _, kind := range []string{"warning", "danger", "success", "tip"} {
		bg := resolve(got, []string{"--doclang-callout-" + kind + "-bg", cssFallbacks["callout-"+kind+"-bg"]})
		for _, ink := range document.Inks {
			name := "--doclang-callout-" + kind + "-" + ink + "-color"
			c, ok := got[name]
			if !ok {
				t.Errorf("%s no se derivó: %s del tema no se lee sobre %s", name, in["--doclang-"+ink+"-color"], bg)
				continue
			}
			if !reads(c, minTextContrast, bg) {
				t.Errorf("%s = %s no se lee sobre %s", name, c, bg)
			}
		}
	}
	// La alerta info es oscura: las tintas del tema se leen y no se tocan.
	for _, ink := range document.Inks {
		if v, ok := got["--doclang-callout-info-"+ink+"-color"]; ok {
			t.Errorf("callout-info-%s-color = %q: la tinta del tema ya se lee sobre #131d33", ink, v)
		}
	}
	checkPairs(t, "oscuro con alertas claras", in, true)
}

// Un vínculo en el encabezado de la tabla cuyo fondo es el color de vínculo
// del tema (1.00:1) toma un color que se lea ahí: aquí, el texto del
// encabezado, porque ni el del tema ni los azules de respaldo se leen.
func TestHTMLThemeVariables_TableHeaderLink(t *testing.T) {
	got := htmlThemeVariables(nestedDarkVars(), true)
	if got["--doclang-table-header-link-color"] != "#0b0b10" {
		t.Errorf("table-header-link-color = %q, want #0b0b10", got["--doclang-table-header-link-color"])
	}
	for _, ink := range document.Inks {
		if c := got["--doclang-table-header-"+ink+"-color"]; !reads(c, minTextContrast, "#8b7fff") {
			t.Errorf("table-header-%s-color = %q no se lee sobre #8b7fff", ink, c)
		}
	}
	// Si el vínculo del cuerpo se lee sobre el encabezado, no hace falta uno
	// propio; con el degradado literal (sin --doclang-table-header-bg) no se
	// mide nada.
	vars := nestedDarkVars()
	vars["--doclang-table-header-bg"] = "#1e293b"
	vars["--doclang-table-header-color"] = "#f1f5f9"
	if v, ok := htmlThemeVariables(vars, true)["--doclang-table-header-link-color"]; ok {
		t.Errorf("el vínculo del cuerpo se lee sobre #1e293b, salió table-header-link-color = %q", v)
	}
	delete(vars, "--doclang-table-header-bg")
	got = htmlThemeVariables(vars, true)
	for k, v := range got {
		if strings.HasPrefix(k, "--doclang-table-header-") && vars[k] == "" {
			t.Errorf("sin fondo medible del encabezado se derivó %s = %q", k, v)
		}
	}
}

// La tecla hereda el texto de donde esté. Con un tema claro cuyo encabezado de
// tabla tiene texto claro, la tecla del encabezado quedaba claro sobre
// #f7f7f7 (1.02:1): lleva el texto del cuerpo.
func TestHTMLThemeVariables_KbdColorWhenAnInheritedTextFails(t *testing.T) {
	light := map[string]string{
		"--doclang-page-bg": "#ffffff", "--doclang-text-color": "#0f172a",
		"--doclang-table-header-bg": "#0f172a", "--doclang-table-header-color": "#f1f5f9",
	}
	got := htmlThemeVariables(light, true)
	if got["--doclang-kbd-color"] != "#0f172a" {
		t.Errorf("kbd-color = %q, want el texto del cuerpo #0f172a", got["--doclang-kbd-color"])
	}
	if _, ok := got["--doclang-kbd-bg"]; ok {
		t.Error("el fondo literal de la tecla se lee con el texto del cuerpo: no se deriva")
	}
	// Si todos los textos que hereda se leen sobre su fondo, la tecla sigue
	// heredando.
	light["--doclang-table-header-color"] = "#0f172a"
	light["--doclang-table-header-bg"] = "#e2e8f0"
	if v, ok := htmlThemeVariables(light, true)["--doclang-kbd-color"]; ok {
		t.Errorf("kbd-color = %q: todos los textos heredados se leen sobre #f7f7f7", v)
	}
}

// Un color de tecla propio del tema cuenta al derivar el fondo: con texto
// oscuro en la tecla, el fondo literal claro se queda.
func TestHTMLThemeVariables_KbdBgFollowsThemeKbdColor(t *testing.T) {
	vars := minimalDarkVars()
	vars["--doclang-kbd-color"] = "#1a1a1a"
	if v, ok := htmlThemeVariables(vars, true)["--doclang-kbd-bg"]; ok {
		t.Errorf("kbd-bg = %q: el #1a1a1a del tema se lee sobre el literal #f7f7f7", v)
	}
	if got := htmlThemeVariables(minimalDarkVars(), true)["--doclang-kbd-bg"]; got != "#0f172a" {
		t.Errorf("sin color de tecla, kbd-bg = %q, want la hoja", got)
	}
}

// El código en línea pinta su propio fondo: se conserva la mitad del par que
// puso el tema y se ajusta la otra.
func TestHTMLThemeVariables_CodeInlinePair(t *testing.T) {
	onlyColor := minimalDarkVars()
	onlyColor["--doclang-code-inline-color"] = "#93c5fd"
	got := htmlThemeVariables(onlyColor, true)
	if bg := got["--doclang-code-inline-bg"]; !reads("#93c5fd", minTextContrast, bg) {
		t.Errorf("solo color: code-inline-bg = %q no deja leer #93c5fd", bg)
	}
	if _, ok := got["--doclang-code-inline-color"]; ok && got["--doclang-code-inline-color"] != "#93c5fd" {
		t.Error("se pisó el color del tema")
	}

	onlyBg := minimalDarkVars()
	onlyBg["--doclang-code-inline-bg"] = "#1e293b"
	if c := htmlThemeVariables(onlyBg, true)["--doclang-code-inline-color"]; !reads(c, minTextContrast, "#1e293b") {
		t.Errorf("solo fondo: code-inline-color = %q no se lee sobre #1e293b", c)
	}

	// Ninguna mitad: el rosa literal da 3.59:1 sobre el gris literal.
	if c := htmlThemeVariables(minimalDarkVars(), true)["--doclang-code-inline-color"]; c != codeInlineDeepColor {
		t.Errorf("sin par: code-inline-color = %q, want %s", c, codeInlineDeepColor)
	}

	// Un par completo del tema no se toca, aunque no se lea.
	both := minimalDarkVars()
	both["--doclang-code-inline-bg"] = "#f6f8fa"
	both["--doclang-code-inline-color"] = "#e83e8c"
	got = htmlThemeVariables(both, true)
	if got["--doclang-code-inline-bg"] != "#f6f8fa" || got["--doclang-code-inline-color"] != "#e83e8c" {
		t.Error("se cambió un par de código en línea que el tema definió completo")
	}
}

// El bloque de código también pinta su fondo: un tema que lo aclara sin dar
// color dejaba el gris claro literal encima (1.09:1 sobre #f5f5f5).
func TestHTMLThemeVariables_CodeBlockPair(t *testing.T) {
	vars := map[string]string{"--doclang-page-bg": "#ffffff", "--doclang-text-color": "#2c3e50", "--doclang-code-bg": "#f5f5f5"}
	if got := htmlThemeVariables(vars, true)["--doclang-code-color"]; got != "#2c3e50" {
		t.Errorf("code-color = %q, want el texto del cuerpo", got)
	}
	// Con el fondo oscuro literal el gris claro se lee: nada que derivar.
	if v, ok := htmlThemeVariables(minimalDarkVars(), true)["--doclang-code-color"]; ok {
		t.Errorf("code-color = %q: #d4d4d4 se lee sobre #1e293b", v)
	}
}

// Un tema oscuro sin tintas propias: los literales no se leen sobre su página
// y toma los respaldos claros.
func TestHTMLThemeVariables_DocumentInksOnDarkPage(t *testing.T) {
	got := htmlThemeVariables(minimalDarkVars(), true)
	for _, ink := range document.Inks {
		if got["--doclang-"+ink+"-color"] != inkLight[ink] {
			t.Errorf("%s-color = %q, want %s", ink, got["--doclang-"+ink+"-color"], inkLight[ink])
		}
	}
	// Las del tema mandan, aunque no se lean.
	vars := minimalDarkVars()
	vars["--doclang-danger-color"] = "#dc2626"
	if got := htmlThemeVariables(vars, true)["--doclang-danger-color"]; got != "#dc2626" {
		t.Errorf("se pisó la tinta del tema: %q", got)
	}
}

// Toda tinta que se deriva (--doclang-...-<tinta>-color) está en el
// vocabulario documentado (document.StandardVariables); las de superficie,
// además, son las que TestCSSFallbacksMatchCore busca en el CSS del core.
func TestHTMLThemeVariables_DerivedInksAreDocumented(t *testing.T) {
	known := map[string]bool{}
	for _, v := range document.StandardVariables {
		known[v] = true
	}
	for _, theme := range []map[string]string{nestedDarkVars(), darkExternalVars(), minimalDarkVars(), document.GetProfessionalTheme().Variables} {
		for k := range htmlThemeVariables(theme, true) {
			isInk := strings.HasSuffix(k, "-link-color")
			for _, ink := range document.Inks {
				isInk = isInk || strings.HasSuffix(k, "-"+ink+"-color")
			}
			if _, own := theme[k]; isInk && !own && !known[k] {
				t.Errorf("se derivó %s, que no está en document.StandardVariables", k)
			}
		}
	}
}
