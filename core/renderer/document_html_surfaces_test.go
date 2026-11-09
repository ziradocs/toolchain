// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/util"
)

// surfaceBindings son las variables de superficie del CSS del documento con
// el fallback exacto que reproduce el literal de antes. Un fallback distinto
// cambiaría el render de todo documento sin la variable, incluidos los temas
// embebidos (ninguno la define).
var surfaceBindings = []struct{ name, fallback string }{
	{"--doclang-table-bg", "white"},
	{"--doclang-table-color", "inherit"},
	{"--doclang-panel-bg", "#f8fafc"},
	{"--doclang-panel-border", "#e2e8f0"},
	{"--doclang-panel-color", "inherit"},
	{"--doclang-chart-bg", "white"},
	{"--doclang-kbd-bg", "#f7f7f7"},
	{"--doclang-kbd-border", "#ccc"},
	{"--doclang-kbd-color", "inherit"},
	{"--doclang-blockquote-bg", "#f7fafc"},
	{"--doclang-blockquote-border", "#3498db"},
	{"--doclang-blockquote-color", "#4a5568"},
	{"--doclang-blockquote-footer-color", "#718096"},
	{"--doclang-callout-info-bg", "#ebf8ff"},
	{"--doclang-callout-info-border", "#3182ce"},
	{"--doclang-callout-info-color", "#2c5282"},
	{"--doclang-callout-warning-bg", "#fffaf0"},
	{"--doclang-callout-warning-border", "#ed8936"},
	{"--doclang-callout-warning-color", "#7c2d12"},
	{"--doclang-callout-danger-bg", "#fff5f5"},
	{"--doclang-callout-danger-border", "#e53e3e"},
	{"--doclang-callout-danger-color", "#742a2a"},
	{"--doclang-callout-success-bg", "#f0fff4"},
	{"--doclang-callout-success-border", "#38a169"},
	{"--doclang-callout-success-color", "#22543d"},
	{"--doclang-callout-tip-bg", "#faf5ff"},
	{"--doclang-callout-tip-border", "#805ad5"},
	{"--doclang-callout-tip-color", "#44337a"},
	{"--doclang-element-title-color", "#2d3748"},
	{"--doclang-caption-color", "#718096"},
	{"--doclang-code-tab-active-bg", "#1e1e1e"},
	{"--doclang-code-tab-active-color", "#fff"},
	{"--doclang-code-tab-accent", "#3498db"},
	{"--doclang-body-link-color", "LinkText"},
	{"--doclang-body-link-color", "VisitedText"},
	{"--doclang-print-margin-bg", "transparent"},
	{"--doclang-blockquote-link-color", "LinkText"},
	{"--doclang-blockquote-link-color", "VisitedText"},
	{"--doclang-callout-info-link-color", "LinkText"},
	{"--doclang-callout-info-link-color", "VisitedText"},
	{"--doclang-callout-warning-link-color", "LinkText"},
	{"--doclang-callout-warning-link-color", "VisitedText"},
	{"--doclang-callout-danger-link-color", "LinkText"},
	{"--doclang-callout-danger-link-color", "VisitedText"},
	{"--doclang-callout-success-link-color", "LinkText"},
	{"--doclang-callout-success-link-color", "VisitedText"},
	{"--doclang-callout-tip-link-color", "LinkText"},
	{"--doclang-callout-tip-link-color", "VisitedText"},
	{"--doclang-body-link-active-color", "ActiveText"},
}

func TestDocumentStyles_SurfaceBindingsKeepLiteralFallbacks(t *testing.T) {
	css := generateDocumentStyles(DocumentHTMLOptions{}, util.NewNoop())
	for _, b := range surfaceBindings {
		want := "var(" + b.name + ", " + b.fallback + ")"
		if !strings.Contains(css, want) {
			t.Errorf("el CSS del documento no tiene %s", want)
		}
	}
}

// Sin estas reglas exactas los vínculos del cuerpo no siguen el tema, o
// (sin :where) le ganan a .toc-link y a .toc > ul > li > a, que tienen su
// propio color.
func TestDocumentStyles_BodyLinkRuleKeepsTypeSpecificity(t *testing.T) {
	css := generateDocumentStyles(DocumentHTMLOptions{}, util.NewNoop())
	for _, rule := range []string{
		"a:where(:link) {\n            color: var(--doclang-body-link-color, LinkText);",
		"a:where(:visited) {\n            color: var(--doclang-body-link-color, VisitedText);",
		"@page {",
	} {
		if !strings.Contains(css, rule) {
			t.Errorf("falta la regla %q", rule)
		}
	}
	if strings.Contains(css, "a:link") || strings.Contains(css, "a:visited") {
		t.Error("un a:link/a:visited sin :where sube la especificidad a (0,1,1) y recolorea el TOC del viewer")
	}
}

// Los literales que se volvieron variables no deben quedar sueltos en la regla
// de su elemento: si alguien reintroduce `background: white` en table, la
// variable deja de tener efecto.
func TestDocumentStyles_NoLiteralLeftInThemedRules(t *testing.T) {
	css := generateDocumentStyles(DocumentHTMLOptions{}, util.NewNoop())
	for _, sel := range []string{"table {", ".grid-column {", ".slidelang-kbd {", "blockquote {", ".chart {", ".chart-container {", ".map {", ".mermaid {", ".alert-info {"} {
		i := strings.Index(css, "        "+sel)
		if i < 0 {
			t.Fatalf("no encontré la regla %q", sel)
		}
		block := css[i : i+strings.Index(css[i:], "}")]
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if (strings.HasPrefix(line, "background:") || strings.HasPrefix(line, "color:") || strings.HasPrefix(line, "border-color:")) &&
				!strings.Contains(line, "var(--doclang-") {
				t.Errorf("%s conserva un literal sin variable: %s", sel, line)
			}
		}
	}
}

// El modo oscuro del viewer redefine variables del contrato. Si definiera las
// de superficie, el viewer oscuro de un tema embebido cambiaría de render; el
// arreglo de ese modo (tabla blanca con texto claro) es otro cambio.
func TestViewerDarkMode_DoesNotDefineSurfaceBindings(t *testing.T) {
	viewer := generateViewerStyles(DocumentHTMLOptions{InteractiveViewer: true})
	start := strings.Index(viewer, `html[data-theme="dark"] {`)
	if start < 0 {
		t.Fatal("no encontré el bloque del modo oscuro del viewer")
	}
	dark := viewer[start : start+strings.Index(viewer[start:], "}")]
	for _, b := range surfaceBindings {
		if strings.Contains(dark, b.name+":") {
			t.Errorf("el modo oscuro del viewer define %s", b.name)
		}
	}
	if strings.Contains(dark, "--_doclang-ink-") || strings.Contains(dark, "--doclang-table-header-link-color") || strings.Contains(dark, "--doclang-table-link-color") || strings.Contains(dark, "--doclang-table-stripe-link-color") || strings.Contains(dark, "--_doclang-table-link") {
		t.Error("el modo oscuro del viewer define una variable de tinta o de vínculo de encabezado")
	}
	for _, s := range documentInkSurfaces {
		for _, ink := range documentInks {
			if n := "--doclang-" + s.name + "-ink-" + ink + ":"; strings.Contains(dark, n) {
				t.Errorf("el modo oscuro del viewer define %s", n)
			}
		}
	}
}

// Todas las reglas de vínculo del documento tienen especificidad (0,0,1) y la
// de :active va después de todas: así un vínculo presionado vuelve a pintarse
// con ActiveText, como lo hacía la hoja del navegador antes de que existieran
// estas reglas de autor, también dentro de una cita o una alerta.
func TestDocumentStyles_ActiveLinkRuleComesLast(t *testing.T) {
	css := generateDocumentStyles(DocumentHTMLOptions{}, util.NewNoop())
	active := strings.Index(css, "a:where(:any-link:active) {")
	if active < 0 {
		t.Fatal("falta la regla a:where(:any-link:active)")
	}
	for _, sel := range []string{"a:where(:link)", "a:where(:visited)"} {
		if last := strings.LastIndex(css, sel); last > active {
			t.Errorf("hay una regla %s después de la de :active", sel)
		}
	}
	for _, kind := range []string{"blockquote", ".alert-info", ".alert-warning", ".alert-error, .alert-danger", ".alert-success", ".alert-tip"} {
		for _, pseudo := range []string{":link", ":visited"} {
			rule := ":where(" + kind + ") a:where(" + pseudo + ")"
			if !strings.Contains(css, rule) {
				t.Errorf("falta %s (el contenedor fuera de :where sube la especificidad y le gana a :active)", rule)
			}
		}
	}
}

// Las tintas de estado leen primero --_doclang-ink-<tinta> (interna), que
// cada superficie con fondo propio copia de su --doclang-<superficie>-ink-<tinta>.
// Sin esas variables la cadena termina en la tinta del documento y su
// literal de siempre, así que el render de los temas embebidos no cambia.
func TestDocumentStyles_InkSurfaces(t *testing.T) {
	css := generateDocumentStyles(DocumentHTMLOptions{}, util.NewNoop())
	literals := map[string]string{"danger": "#dc2626", "info": "#2563eb", "success": "#15803d", "warning": "#b45309", "accent": "#7c3aed"}
	compact := strings.Join(strings.Fields(css), " ")
	for _, ink := range documentInks {
		want := ".slidelang-text-" + ink + " { color: var(--_doclang-ink-" + ink + ", var(--doclang-" + ink + "-color, " + literals[ink] + ")); }"
		if !strings.Contains(compact, want) {
			t.Errorf("falta la regla %s", want)
		}
	}
	names := map[string]bool{}
	for _, s := range documentInkSurfaces {
		names[s.name] = true
		want := s.selector + " {"
		for _, ink := range documentInks {
			want += " --_doclang-ink-" + ink + ": var(--doclang-" + s.name + "-ink-" + ink + ");"
		}
		if !strings.Contains(css, want+" }") {
			t.Errorf("falta el bloque de tintas de %s: %s }", s.name, want)
		}
	}
	for _, n := range []string{"table", "table-stripe", "table-header", "panel", "blockquote", "callout-info", "callout-warning", "callout-danger", "callout-success", "callout-tip", "quiz"} {
		if !names[n] {
			t.Errorf("la superficie %s no fija sus tintas", n)
		}
	}
}

// Un vínculo en una celda o en el encabezado de una tabla lleva su propia
// variable y, sin ella, el color de vínculo del cuerpo. Entre las reglas de
// vínculo gana la última, así que las de la tabla van después de las de la
// cita y las alertas (una tabla dentro de una alerta) y antes de :active.
func TestDocumentStyles_TableLinksAndRuleOrder(t *testing.T) {
	css := generateDocumentStyles(DocumentHTMLOptions{}, util.NewNoop())
	rules := []string{
		":where(blockquote) a:where(:link) { color: var(--doclang-blockquote-link-color, LinkText); }",
		":where(.alert-tip) a:where(:visited) { color: var(--doclang-callout-tip-link-color, VisitedText); }",
		":where(td) a:where(:link) { color: var(--_doclang-table-link, var(--doclang-body-link-color, LinkText)); }",
		":where(td) a:where(:visited) { color: var(--_doclang-table-link, var(--doclang-body-link-color, VisitedText)); }",
		"table { --_doclang-table-link: var(--doclang-table-link-color); }",
		"table tr:nth-child(even) { --_doclang-table-link: var(--doclang-table-stripe-link-color, var(--doclang-table-link-color)); }",
		":where(th) a:where(:link) { color: var(--doclang-table-header-link-color, var(--doclang-body-link-color, LinkText)); }",
		":where(th) a:where(:visited) { color: var(--doclang-table-header-link-color, var(--doclang-body-link-color, VisitedText)); }",
		"a:where(:any-link:active) {",
	}
	prev := -1
	for _, r := range rules {
		i := strings.Index(css, r)
		if i < 0 {
			t.Fatalf("falta la regla %s", r)
		}
		if i < prev {
			t.Errorf("la regla %s está fuera de orden", r)
		}
		prev = i
	}
}
