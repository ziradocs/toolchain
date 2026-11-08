// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/doclang/v2/themes/document"
)

var embeddedDOCXThemes = []string{"professional", "academic", "technical", "page-view"}

// docxThemeSample cubre texto, código, tabla y enlace en un solo documento.
func docxThemeSample() *ast.AST {
	doc := astWithTable()
	pos := diagnostics.NewPosition(7, 1)
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements,
		ast.NewTextElement(pos, "Texto con `codigo` y [enlace](https://example.com)."),
		ast.NewCodeElement(diagnostics.NewPosition(8, 1), "go", "package main\n"),
	)
	return doc
}

// Los 4 temas embebidos deben producir el mismo DOCX que antes: el estilo no
// cambia, y pasar sus variables al generador (como hace el pipeline) tampoco.
func TestDOCXEmbeddedThemes_Unchanged(t *testing.T) {
	// Huellas del DOCXStyle cableado, tomadas antes de este cambio.
	want := map[string]string{}
	for _, name := range embeddedDOCXThemes {
		want[name] = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%#v", GetStyleForTheme(name, newTestLogger())))))
	}
	for _, name := range embeddedDOCXThemes {
		theme := document.GetTheme(name)
		opts := GeneratorOptions{Theme: name, ThemeVariables: theme.Variables, ThemeExternal: theme.IsExternal}

		got := resolveDOCXStyle(name, opts, newTestLogger())
		if g := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%#v", got)))); g != want[name] {
			t.Errorf("%s: resolveDOCXStyle differs from GetStyleForTheme", name)
		}

		plain := buildDOCXXML(t, docxThemeSample(), GeneratorOptions{Theme: name})
		withVars := buildDOCXXML(t, docxThemeSample(), opts)
		if plain != withVars {
			t.Errorf("%s: DOCX XML changed when ThemeVariables are passed", name)
		}
	}
}

// Fija los valores de professional que usan las pruebas de saneamiento como
// "fallback" (si cambian, las pruebas deben enterarse).
func TestDOCXEmbeddedThemes_ProfessionalAnchors(t *testing.T) {
	s := GetStyleForTheme("professional", newTestLogger())
	if s.FontFamily != "Segoe UI" || s.H1Color != "1A202C" || s.TableHeaderBg != "5A67D8" {
		t.Fatalf("professional style changed: %+v", s)
	}
}

func externalThemeVars() map[string]string {
	return map[string]string{
		"--doclang-font-main":            "'Palatino Linotype', Georgia, serif",
		"--doclang-code-font":            "\"Fira Code\", monospace",
		"--doclang-h1-color":             "#aa0011",
		"--doclang-h2-color":             "#0a0",
		"--doclang-h3-color":             "#112233",
		"--doclang-h4-color":             "#445566",
		"--doclang-text-color":           "#223344",
		"--doclang-text-light":           "#778899",
		"--doclang-link-color":           "#FF00AA",
		"--doclang-code-bg":              "#101820",
		"--doclang-code-color":           "#e0e0e0",
		"--doclang-code-inline-color":    "#c0ffee",
		"--doclang-table-header-bg":      "#7a1fa2",
		"--doclang-table-header-color":   "#fefefe",
		"--doclang-table-border":         "#abcdef",
		"--doclang-table-stripe-bg":      "#f1e9f7",
		"--doclang-alert-info-color":     "#123abc",
		"--doclang-alert-warning-border": "#bada55",
		"--doclang-alert-danger-border":  "#d00d00",
		"--doclang-alert-success-border": "#0b8b0b",
		"--doclang-alert-tip-border":     "#6a5acd",
	}
}

func TestDOCXExternalTheme_UsesThemeVariables(t *testing.T) {
	s := styleFromThemeVariables(externalThemeVars(), newTestLogger())
	pro := GetStyleForTheme("professional", newTestLogger())

	checks := map[string][2]string{
		"FontFamily":       {s.FontFamily, "Palatino Linotype"},
		"CodeFontFamily":   {s.CodeFontFamily, "Fira Code"},
		"H1Color":          {s.H1Color, "AA0011"},
		"H2Color":          {s.H2Color, "00AA00"},
		"H3Color":          {s.H3Color, "112233"},
		"H4Color":          {s.H4Color, "445566"},
		"TextColor":        {s.TextColor, "223344"},
		"TextLightColor":   {s.TextLightColor, "778899"},
		"LinkColor":        {s.LinkColor, "FF00AA"},
		"CodeBlockBg":      {s.CodeBlockBg, "101820"},
		"CodeBlockColor":   {s.CodeBlockColor, "E0E0E0"},
		"CodeInlineColor":  {s.CodeInlineColor, "C0FFEE"},
		"TableHeaderBg":    {s.TableHeaderBg, "7A1FA2"},
		"TableHeaderColor": {s.TableHeaderColor, "FEFEFE"},
		"TableBorderColor": {s.TableBorderColor, "ABCDEF"},
		"TableStripeBg":    {s.TableStripeBg, "F1E9F7"},
		"InfoColor":        {s.InfoColor, "123ABC"},
		"WarningColor":     {s.WarningColor, "BADA55"},
		"DangerColor":      {s.DangerColor, "D00D00"},
		"SuccessColor":     {s.SuccessColor, "0B8B0B"},
		"TipColor":         {s.TipColor, "6A5ACD"},
	}
	for field, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", field, c[0], c[1])
		}
	}

	// Lo que el tema no define viene de professional.
	if s.H1Size != pro.H1Size || s.H1SpaceBefore != pro.H1SpaceBefore || !s.H1Bold ||
		s.FontSizeBase != pro.FontSizeBase || s.FontSizeCode != pro.FontSizeCode ||
		s.TextSpaceAfter != pro.TextSpaceAfter || s.ListIndent != pro.ListIndent ||
		s.TableStripeEnabled != pro.TableStripeEnabled {
		t.Errorf("sizes/spacing/bold should come from professional: %+v", s)
	}
	if got := styleFromThemeVariables(map[string]string{}, newTestLogger()); got != pro {
		t.Errorf("a theme with no variables should equal professional")
	}
}

func TestDOCXExternalTheme_XMLContainsThemeValues(t *testing.T) {
	opts := GeneratorOptions{Theme: "mi-tema-externo", ThemeVariables: externalThemeVars(), ThemeExternal: true}
	xml := buildDOCXXML(t, docxThemeSample(), opts)

	for _, want := range []string{
		`w:ascii="Palatino Linotype"`,
		`w:ascii="Fira Code"`,
		`w:fill="7A1FA2"`,  // encabezado de tabla
		`w:color="ABCDEF"`, // borde de tabla
		`w:fill="F1E9F7"`,  // fila alterna
		`w:fill="101820"`,  // fondo de código
		`FF00AA`,           // enlace
		`AA0011`,           // h1
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("DOCX XML missing %q", want)
		}
	}
	if strings.Contains(xml, "Segoe UI") {
		t.Errorf("external font should replace Segoe UI")
	}
}

func TestDOCXExternalTheme_HostileValuesFallBackToProfessional(t *testing.T) {
	pro := GetStyleForTheme("professional", newTestLogger())
	hostile := map[string]string{
		"--doclang-font-main":           `Arial"><w:b/>`,
		"--doclang-code-font":           "url(x)",
		"--doclang-h1-color":            "red; }",
		"--doclang-h2-color":            "url(x)",
		"--doclang-h3-color":            "rgb(1,2,3)",
		"--doclang-h4-color":            "var(--x)",
		"--doclang-text-color":          "red",
		"--doclang-link-color":          `#fff"><w:b/>`,
		"--doclang-code-bg":             "#12345",
		"--doclang-code-color":          "#12345678",
		"--doclang-table-header-bg":     "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
		"--doclang-table-border":        "#ggg",
		"--doclang-alert-info-color":    `#000" w:val="x`,
		"--doclang-alert-danger-border": "",
	}
	s := styleFromThemeVariables(hostile, newTestLogger())
	if s != pro {
		t.Fatalf("hostile values must fall back to professional.\n got: %+v\nwant: %+v", s, pro)
	}

	xml := buildDOCXXML(t, docxThemeSample(), GeneratorOptions{Theme: "x", ThemeVariables: hostile, ThemeExternal: true})
	for _, bad := range []string{"red; }", "url(x)", `Arial"`, "<w:b/>Arial", "rgb(1,2,3)", "var(--x)", "linear-gradient"} {
		if strings.Contains(xml, bad) {
			t.Errorf("hostile value %q reached the XML", bad)
		}
	}
	if !strings.Contains(xml, "Segoe UI") {
		t.Errorf("expected professional font in XML")
	}
}

func TestSanitizeThemeColor(t *testing.T) {
	ok := map[string]string{"#abc": "AABBCC", "#AbCdEf": "ABCDEF", " #000 ": "000000", "#FFFFFF": "FFFFFF"}
	for in, want := range ok {
		if got, valid := sanitizeThemeColor(in); !valid || got != want {
			t.Errorf("sanitizeThemeColor(%q) = %q,%v want %q", in, got, valid, want)
		}
	}
	for _, in := range []string{"", "red", "#", "#ab", "#abcd", "#abcde", "#abcdeff", "#abcdef00", "abcdef", "rgb(0,0,0)", "var(--a)", "url(x)", "#ggg", "red; }"} {
		if got, valid := sanitizeThemeColor(in); valid {
			t.Errorf("sanitizeThemeColor(%q) accepted as %q", in, got)
		}
	}
}

func TestSanitizeThemeFont(t *testing.T) {
	ok := map[string]string{
		"'Segoe UI', sans-serif":    "Segoe UI",
		`"Fira Code", monospace`:    "Fira Code",
		"Georgia":                   "Georgia",
		"'Foo, Bar', serif":         "", // coma dentro de comillas: "Foo, Bar" no es válido
		"Source-Sans 3, sans-serif": "Source-Sans 3",
	}
	for in, want := range ok {
		got, valid := sanitizeThemeFont(in)
		if want == "" {
			if valid {
				t.Errorf("sanitizeThemeFont(%q) accepted as %q", in, got)
			}
			continue
		}
		if !valid || got != want {
			t.Errorf("sanitizeThemeFont(%q) = %q,%v want %q", in, got, valid, want)
		}
	}
	for _, in := range []string{"", "''", "sans-serif", "system-ui, Arial", "-apple-system, Arial", `Arial"><w:b/>`, "url(x)", "A;B", "a<b", strings.Repeat("a", 65)} {
		if got, valid := sanitizeThemeFont(in); valid {
			t.Errorf("sanitizeThemeFont(%q) accepted as %q", in, got)
		}
	}
	if _, valid := sanitizeThemeFont(strings.Repeat("a", 64)); !valid {
		t.Errorf("64-char name should be accepted")
	}
}
