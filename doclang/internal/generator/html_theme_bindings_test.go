// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/themes/document"
)

// bindingNames son las variables de superficie que lee el CSS del documento
// (core/renderer/document_html.go) y que htmlThemeVariables puede derivar.
var bindingNames = []string{
	"--doclang-table-bg", "--doclang-table-color",
	"--doclang-panel-bg", "--doclang-panel-border", "--doclang-panel-color",
	"--doclang-chart-bg",
	"--doclang-kbd-bg", "--doclang-kbd-border", "--doclang-kbd-color",
	"--doclang-blockquote-bg", "--doclang-blockquote-border", "--doclang-blockquote-color", "--doclang-blockquote-footer-color",
	"--doclang-element-title-color", "--doclang-caption-color",
	"--doclang-code-tab-active-bg", "--doclang-code-tab-active-color", "--doclang-code-tab-accent",
	"--doclang-body-link-color", "--doclang-print-margin-bg",
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

func darkExternalVars() map[string]string {
	return map[string]string{
		"--doclang-page-bg":           "#0f172a",
		"--doclang-font-main":         "Arial, sans-serif",
		"--doclang-h1-color":          "#f1f5f9",
		"--doclang-text-color":        "#e2e8f0",
		"--doclang-text-light":        "#94a3b8",
		"--doclang-code-bg":           "#1e293b",
		"--doclang-code-color":        "#f8fafc",
		"--doclang-table-border":      "#334155",
		"--doclang-table-stripe-bg":   "#1e293b",
		"--doclang-link-color":        "#7dd3fc",
		"--doclang-quote-bg":          "#1e293b",
		"--doclang-quote-border":      "#38bdf8",
		"--doclang-alert-info-bg":     "#082f49",
		"--doclang-alert-info-color":  "#bae6fd",
		"--doclang-alert-info-border": "#0ea5e9",
		// Fondo sin color propio: el texto sale de --doclang-text-color.
		"--doclang-alert-warning-bg": "#451a03",
	}
}

func TestHTMLThemeVariables_DerivesSurfacesForExternalTheme(t *testing.T) {
	in := darkExternalVars()
	got := htmlThemeVariables(in, true)

	want := map[string]string{
		"--doclang-body-link-color":         "#7dd3fc",
		"--doclang-table-bg":                "#0f172a",
		"--doclang-panel-bg":                "#1e293b",
		"--doclang-panel-border":            "#334155",
		"--doclang-kbd-bg":                  "#1e293b",
		"--doclang-kbd-border":              "#334155",
		"--doclang-blockquote-bg":           "#1e293b",
		"--doclang-blockquote-color":        "#e2e8f0", // sin quote-color: el texto del cuerpo
		"--doclang-blockquote-border":       "#38bdf8",
		"--doclang-blockquote-footer-color": "#94a3b8",
		"--doclang-callout-info-bg":         "#082f49",
		"--doclang-callout-info-color":      "#bae6fd",
		"--doclang-callout-info-border":     "#0ea5e9",
		"--doclang-callout-warning-bg":      "#451a03",
		"--doclang-callout-warning-color":   "#e2e8f0",
		"--doclang-element-title-color":     "#e2e8f0",
		"--doclang-caption-color":           "#94a3b8",
		"--doclang-code-tab-active-bg":      "#1e293b",
		"--doclang-code-tab-active-color":   "#f8fafc",
		"--doclang-print-margin-bg":         "#0f172a",
		// Vínculos dentro de superficies cuyo fondo salió del tema.
		"--doclang-blockquote-link-color":      "#7dd3fc",
		"--doclang-callout-info-link-color":    "#7dd3fc",
		"--doclang-callout-warning-link-color": "#7dd3fc",
		// Variables del contrato con fallback claro que el tema no definió.
		"--doclang-table-hover-bg":   "#1e293b",
		"--doclang-header-footer-bg": "#0f172a",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	// No se derivan: la gráfica se dibuja sobre blanco y los textos heredados
	// ya siguen al tema.
	for _, k := range []string{"--doclang-chart-bg", "--doclang-table-color", "--doclang-panel-color", "--doclang-kbd-color", "--doclang-code-tab-accent", "--doclang-callout-danger-bg",
		"--doclang-callout-danger-link-color", // la alerta danger conserva su fondo claro literal
		"--doclang-quiz-bg",                   // el quiz ya toma --doclang-quote-bg
		"--doclang-quiz-correct-color",        // el tema no definió --doclang-alert-success-bg
	} {
		if v, ok := got[k]; ok {
			t.Errorf("%s no debe derivarse, salió %q", k, v)
		}
	}
	// Las variables del tema siguen ahí, y el mapa de entrada no se tocó (el
	// DOCX y los diagramas leen el mismo mapa).
	for k, v := range in {
		if got[k] != v {
			t.Errorf("%s cambió: %q", k, got[k])
		}
	}
	if !reflect.DeepEqual(in, darkExternalVars()) {
		t.Error("htmlThemeVariables modificó el mapa de entrada")
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

func TestHTMLThemeVariables_SheetPrefersPageSurface(t *testing.T) {
	got := htmlThemeVariables(map[string]string{
		"--doclang-page-bg":         "#cccccc",
		"--doclang-page-surface-bg": "#fffff0",
	}, true)
	if got["--doclang-table-bg"] != "#fffff0" || got["--doclang-panel-bg"] != "#fffff0" {
		t.Errorf("la hoja debe ser --doclang-page-surface-bg: table=%q panel=%q", got["--doclang-table-bg"], got["--doclang-panel-bg"])
	}
	if got["--doclang-print-margin-bg"] != "#cccccc" {
		t.Errorf("el margen del PDF es el lienzo (--doclang-page-bg), salió %q", got["--doclang-print-margin-bg"])
	}
}

// Un valor que el saneo rechaza no se copia a otro nombre: la variable de
// origen ya se descarta (con aviso) al emitirse.
func TestHTMLThemeVariables_UnsafeSourceNotCopied(t *testing.T) {
	evil := `red; } </style><script>alert(1)</script>`
	got := htmlThemeVariables(map[string]string{
		"--doclang-page-bg":    evil,
		"--doclang-link-color": "url(x);",
		"--doclang-text-color": "#111111",
	}, true)
	for _, k := range []string{"--doclang-table-bg", "--doclang-print-margin-bg", "--doclang-body-link-color", "--doclang-panel-bg"} {
		if v, ok := got[k]; ok {
			t.Errorf("%s se derivó de un valor rechazado: %q", k, v)
		}
	}
}

// De punta a punta: un tema externo oscuro llega al :root del HTML con sus
// superficies, y el mismo mapa como tema embebido no.
func TestHTMLGenerate_ExternalThemeEmitsSurfaces(t *testing.T) {
	src := "---\ntitle: T\n---\n\n# T\n\nA [link](https://example.com/) and [K]{.kbd}.\n\n| A | B |\n|---|---|\n| 1 | 2 |\n"
	doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "t.doclang")

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
		"--doclang-panel-bg: #1e293b;",
		"--doclang-kbd-bg: #1e293b;",
		"--doclang-body-link-color: #7dd3fc;",
		"--doclang-print-margin-bg: #0f172a;",
	} {
		if !strings.Contains(html, decl) {
			t.Errorf("el HTML del tema externo no declara %q", decl)
		}
	}

	notExternal := render(false)
	for _, b := range bindingNames {
		if strings.Contains(notExternal, b+":") {
			t.Errorf("un tema no externo declara %s", b)
		}
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

// Un tema mínimo (solo lo obligatorio más un color de vínculo): cita y alertas
// conservan sus fondos claros literales, así que sus vínculos no toman el
// color claro del tema; la tabla, la fila alterna y el quiz toman la hoja.
func TestHTMLThemeVariables_MinimalDarkTheme(t *testing.T) {
	got := htmlThemeVariables(map[string]string{
		"--doclang-page-bg":          "#0f172a",
		"--doclang-font-main":        "Arial",
		"--doclang-h1-color":         "#ffffff",
		"--doclang-text-color":       "#e2e8f0",
		"--doclang-code-bg":          "#1e293b",
		"--doclang-table-border":     "#334155",
		"--doclang-link-color":       "#7dd3fc",
		"--doclang-alert-success-bg": "#052e16",
	}, true)
	want := map[string]string{
		"--doclang-body-link-color":            "#7dd3fc",
		"--doclang-table-bg":                   "#0f172a",
		"--doclang-table-stripe-bg":            "#0f172a",
		"--doclang-table-hover-bg":             "#0f172a",
		"--doclang-quiz-bg":                    "#0f172a",
		"--doclang-quiz-correct-color":         "#e2e8f0",
		"--doclang-callout-success-bg":         "#052e16",
		"--doclang-callout-success-link-color": "#7dd3fc",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	for _, k := range []string{"--doclang-blockquote-bg", "--doclang-blockquote-link-color", "--doclang-callout-info-bg", "--doclang-callout-info-link-color"} {
		if v, ok := got[k]; ok {
			t.Errorf("%s no debe derivarse sin el fondo de esa superficie, salió %q", k, v)
		}
	}
}
