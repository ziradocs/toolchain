// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/doclang/v2/themes/document"
)

var embeddedDOCXThemes = []string{"professional", "academic", "technical", "page-view"}

// Huellas (sha256 de fmt.Sprintf("%#v", DOCXStyle)) de los 4 temas embebidos,
// calculadas con el código de origin/main antes de este cambio.
var embeddedStyleFingerprints = map[string]string{
	"professional": "dbd839a45393454744a0a69aa0e01885a260b409fb9b989fec0547550d1ef0b0",
	"academic":     "15cc2f3097b68707567d930bbafc4dac6f64b6e5989910e3a592f9f0fbd1a11c",
	"technical":    "3f237fefd476de5afe19e507ab2ec6706ea463099553b304df6af694af029615",
	"page-view":    "011b3bd5bb5d62b317be3dac90ad440429120faad65f055bbff5dcc2569c1a26",
}

func styleFingerprint(s DOCXStyle) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%#v", s))))
}

// Los 4 temas embebidos deben producir el mismo DOCX que en origin/main: el
// estilo coincide con la huella fija y el document.xml con el golden generado
// en origin/main (testdata/docx_theme_<tema>.xml), con o sin las variables del
// tema en las opciones (el pipeline las pasa siempre).
func TestDOCXEmbeddedThemes_Unchanged(t *testing.T) {
	for _, name := range embeddedDOCXThemes {
		theme := document.GetTheme(name)
		opts := GeneratorOptions{Theme: name, ThemeVariables: theme.Variables, ThemeExternal: theme.IsExternal}

		if got := styleFingerprint(resolveDOCXStyle(name, opts, newTestLogger())); got != embeddedStyleFingerprints[name] {
			t.Errorf("%s: style fingerprint %s, want %s", name, got, embeddedStyleFingerprints[name])
		}

		golden, err := os.ReadFile(filepath.Join("testdata", "docx_theme_"+name+".xml"))
		if err != nil {
			t.Fatal(err)
		}
		for label, o := range map[string]GeneratorOptions{"theme only": {Theme: name}, "with variables": opts} {
			if got := buildDOCXXML(t, docxThemeSample(), o); got != string(golden) {
				t.Errorf("%s (%s): document.xml differs from the origin/main golden", name, label)
			}
		}
	}
}

func externalThemeVars() map[string]string {
	return map[string]string{
		"--doclang-font-main":            "'Palatino Linotype', Georgia, serif",
		"--doclang-code-font":            "\"Fira Code\", monospace",
		"--doclang-h1-color":             "#aa0011",
		"--doclang-h2-color":             "#080",
		"--doclang-h3-color":             "#112233",
		"--doclang-h4-color":             "#445566",
		"--doclang-text-color":           "#223344",
		"--doclang-text-light":           "#778899",
		"--doclang-link-color":           "#0044cc",
		"--doclang-code-bg":              "#101820",
		"--doclang-code-color":           "#e0e0e0",
		"--doclang-code-inline-color":    "#7a1f5a",
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
		"H2Color":          {s.H2Color, "008800"},
		"H3Color":          {s.H3Color, "112233"},
		"H4Color":          {s.H4Color, "445566"},
		"TextColor":        {s.TextColor, "223344"},
		"TextLightColor":   {s.TextLightColor, "778899"},
		"LinkColor":        {s.LinkColor, "0044CC"},
		"CodeBlockBg":      {s.CodeBlockBg, "101820"},
		"CodeBlockColor":   {s.CodeBlockColor, "E0E0E0"},
		"CodeInlineColor":  {s.CodeInlineColor, "7A1F5A"},
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

func externalOpts(vars map[string]string) GeneratorOptions {
	return GeneratorOptions{Theme: "mi-tema-externo", ThemeVariables: vars, ThemeExternal: true}
}

func TestDOCXExternalTheme_XMLContainsThemeValues(t *testing.T) {
	xml := buildDOCXXML(t, docxThemeSample(), externalOpts(externalThemeVars()))

	for _, want := range []string{
		`w:ascii="Palatino Linotype"`,
		`w:ascii="Fira Code"`,
		`w:fill="7A1FA2"`,  // encabezado de tabla
		`w:color="ABCDEF"`, // borde de tabla
		`w:fill="F1E9F7"`,  // fila alterna
		`w:fill="101820"`,  // fondo de código
		`0044CC`,           // enlace
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

// Los bloques especiales de un tema externo usan las variables de alerta, en
// el document.xml real; los de un embebido conservan sus colores.
func TestDOCXExternalTheme_SpecialBlocksUseAlertVariables(t *testing.T) {
	xml := buildDOCXXML(t, docxThemeSample(), externalOpts(externalThemeVars()))
	for _, c := range []string{"123ABC", "BADA55", "D00D00", "0B8B0B", "6A5ACD"} {
		if !strings.Contains(xml, `w:val="`+c+`"`) {
			t.Errorf("special block color %s missing from document.xml", c)
		}
	}
	// Los colores cableados de blockConfig ya no salen en un tema externo.
	for _, c := range []string{"1F77B4", "FF9800", "F44336", "4CAF50", "9C27B0"} {
		if strings.Contains(xml, `w:val="`+c+`"`) {
			t.Errorf("hardcoded block color %s still used by an external theme", c)
		}
	}
	// Un embebido sigue con blockConfig (FF9800 es el warning).
	emb := buildDOCXXML(t, docxThemeSample(), GeneratorOptions{Theme: "professional"})
	if !strings.Contains(emb, `w:val="FF9800"`) || strings.Contains(emb, "BADA55") {
		t.Errorf("embedded theme special blocks changed")
	}
}

// Un tema oscuro (texto claro sobre fondo oscuro) no se puede pintar en papel
// blanco: sus colores de texto se descartan y queda professional.
func TestDOCXExternalTheme_DarkThemeStaysReadable(t *testing.T) {
	dark := map[string]string{
		"--doclang-page-bg":            "#1e1e1e",
		"--doclang-text-color":         "#e0e0e0",
		"--doclang-text-light":         "#cccccc",
		"--doclang-h1-color":           "#ffffff",
		"--doclang-h2-color":           "#f0f0f0",
		"--doclang-h3-color":           "#eeeeee",
		"--doclang-h4-color":           "#dddddd",
		"--doclang-link-color":         "#9cdcfe",
		"--doclang-code-inline-color":  "#ce9178",
		"--doclang-table-header-color": "#ffffff",
		"--doclang-table-header-bg":    "#ffffff",
		"--doclang-code-color":         "#101010",
		"--doclang-code-bg":            "#202020",
	}
	pro := GetStyleForTheme("professional", newTestLogger())
	s := styleFromThemeVariables(dark, newTestLogger())
	if s != pro {
		t.Fatalf("dark theme should fall back to professional.\n got: %+v\nwant: %+v", s, pro)
	}
	xml := buildDOCXXML(t, docxThemeSample(), externalOpts(dark))
	for _, bad := range []string{"E0E0E0", "FFFFFF", "F0F0F0", "9CDCFE", "CE9178"} {
		// FFFFFF solo es legítimo como color de encabezado de tabla de professional (sobre 5A67D8).
		if bad == "FFFFFF" {
			continue
		}
		if strings.Contains(xml, `w:val="`+bad+`"`) {
			t.Errorf("light text color %s reached the document", bad)
		}
	}
	for _, c := range []string{pro.TextColor, pro.H1Color, pro.LinkColor} {
		if !strings.Contains(xml, `w:val="`+c+`"`) {
			t.Errorf("professional color %s missing from document", c)
		}
	}
}

func TestDOCXExternalTheme_ContrastThresholds(t *testing.T) {
	pro := GetStyleForTheme("professional", newTestLogger())
	cases := []struct {
		name  string
		vars  map[string]string
		field func(DOCXStyle) string
		want  string
	}{
		// 767676 sobre blanco: 4.54, pasa el mínimo de texto.
		{"text 4.5 ok", map[string]string{"--doclang-text-color": "#767676"}, func(s DOCXStyle) string { return s.TextColor }, "767676"},
		// 777777: 4.48, no llega.
		{"text 4.48 fails", map[string]string{"--doclang-text-color": "#777777"}, func(s DOCXStyle) string { return s.TextColor }, pro.TextColor},
		{"link fails", map[string]string{"--doclang-link-color": "#888888"}, func(s DOCXStyle) string { return s.LinkColor }, pro.LinkColor},
		{"inline code fails", map[string]string{"--doclang-code-inline-color": "#888888"}, func(s DOCXStyle) string { return s.CodeInlineColor }, pro.CodeInlineColor},
		// 949494: 3.03, pasa el mínimo de 3 de títulos y texto claro.
		{"h1 3.0 ok", map[string]string{"--doclang-h1-color": "#949494"}, func(s DOCXStyle) string { return s.H1Color }, "949494"},
		{"h4 fails", map[string]string{"--doclang-h4-color": "#aaaaaa"}, func(s DOCXStyle) string { return s.H4Color }, pro.H4Color},
		{"light fails", map[string]string{"--doclang-text-light": "#bbbbbb"}, func(s DOCXStyle) string { return s.TextLightColor }, pro.TextLightColor},
	}
	for _, c := range cases {
		if got := c.field(styleFromThemeVariables(c.vars, newTestLogger())); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

// Fondo y texto se resuelven juntos: si uno falla (inválido, ausente o sin
// contraste entre sí), los dos toman el valor de professional.
func TestDOCXExternalTheme_PairsFallBackTogether(t *testing.T) {
	pro := GetStyleForTheme("professional", newTestLogger())
	cases := []map[string]string{
		{"--doclang-code-bg": "#ffffff"},                                    // texto ausente
		{"--doclang-code-bg": "#ffffff", "--doclang-code-color": "red"},     // texto inválido
		{"--doclang-code-bg": "url(x)", "--doclang-code-color": "#ffffff"},  // fondo inválido
		{"--doclang-code-bg": "#ffffff", "--doclang-code-color": "#fafafa"}, // sin contraste
		{"--doclang-code-color": "#101010"},                                 // fondo ausente
	}
	for i, vars := range cases {
		s := styleFromThemeVariables(vars, newTestLogger())
		if s.CodeBlockBg != pro.CodeBlockBg || s.CodeBlockColor != pro.CodeBlockColor {
			t.Errorf("code case %d: got %s/%s, want professional %s/%s", i, s.CodeBlockBg, s.CodeBlockColor, pro.CodeBlockBg, pro.CodeBlockColor)
		}
		tv := map[string]string{}
		for k, v := range vars {
			tv[strings.Replace(k, "code-bg", "table-header-bg", 1)] = v
			tv[strings.Replace(strings.Replace(k, "code-color", "table-header-color", 1), "code-bg", "table-header-bg", 1)] = v
		}
		s = styleFromThemeVariables(tv, newTestLogger())
		if s.TableHeaderBg != pro.TableHeaderBg || s.TableHeaderColor != pro.TableHeaderColor {
			t.Errorf("table case %d: got %s/%s, want professional %s/%s", i, s.TableHeaderBg, s.TableHeaderColor, pro.TableHeaderBg, pro.TableHeaderColor)
		}
	}
	// Con ambos válidos y contrastados, se toman los del tema.
	s := styleFromThemeVariables(map[string]string{"--doclang-code-bg": "#fafafa", "--doclang-code-color": "#101010"}, newTestLogger())
	if s.CodeBlockBg != "FAFAFA" || s.CodeBlockColor != "101010" {
		t.Errorf("valid pair not applied: %s/%s", s.CodeBlockBg, s.CodeBlockColor)
	}
}

// docxXMLParts devuelve todas las partes XML de un DOCX generado.
func docxXMLParts(t *testing.T, doc *ast.AST, opts GeneratorOptions) map[string]string {
	t.Helper()
	opts.Format = "docx"
	path := filepath.Join(t.TempDir(), "out.docx")
	if err := New(newTestLogger()).Generate(doc, path, opts); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	parts := map[string]string{}
	for _, f := range r.File {
		if !strings.HasSuffix(f.Name, ".xml") && !strings.HasSuffix(f.Name, ".rels") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		parts[f.Name] = string(b)
	}
	return parts
}

var (
	xmlColorAttrRe = regexp.MustCompile(`w:(?:color|fill|val)="([^"]*)"`)
	xmlFontAttrRe  = regexp.MustCompile(`w:(?:ascii|hAnsi|cs|eastAsia)="([^"]*)"`)
)

func TestDOCXExternalTheme_HostileValuesFallBackToProfessional(t *testing.T) {
	pro := GetStyleForTheme("professional", newTestLogger())
	hostile := map[string]string{
		"--doclang-font-main":            `Arial"><w:b/>`,
		"--doclang-code-font":            "url(x)",
		"--doclang-h1-color":             "red; }",
		"--doclang-h2-color":             "url(x)",
		"--doclang-h3-color":             "rgb(1,2,3)",
		"--doclang-h4-color":             "var(--x)",
		"--doclang-text-color":           "red",
		"--doclang-text-light":           `#000"><w:i/>`,
		"--doclang-link-color":           `#000"><w:b/>`,
		"--doclang-code-inline-color":    "#12",
		"--doclang-code-bg":              "#12345",
		"--doclang-code-color":           "#12345678",
		"--doclang-table-header-bg":      "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
		"--doclang-table-header-color":   "#000",
		"--doclang-table-border":         "#ggg",
		"--doclang-table-stripe-bg":      "red; }",
		"--doclang-alert-info-color":     `#000" w:val="x`,
		"--doclang-alert-warning-border": "url(x)",
		"--doclang-alert-danger-border":  "",
		"--doclang-alert-success-border": "var(--x)",
		"--doclang-alert-tip-border":     "red; }",
	}
	if s := styleFromThemeVariables(hostile, newTestLogger()); s != pro {
		t.Fatalf("hostile values must fall back to professional.\n got: %+v\nwant: %+v", s, pro)
	}

	proParts := docxXMLParts(t, docxThemeSample(), GeneratorOptions{Theme: "professional"})
	parts := docxXMLParts(t, docxThemeSample(), externalOpts(hostile))
	// Referencia: un tema externo que no define ninguna variable conocida.
	blank := docxXMLParts(t, docxThemeSample(), externalOpts(map[string]string{"--doclang-unknown": "x"}))
	knownFonts := map[string]bool{}
	for _, xmlText := range proParts {
		for _, m := range xmlFontAttrRe.FindAllStringSubmatch(xmlText, -1) {
			knownFonts[m[1]] = true
		}
	}
	if _, ok := parts["word/numbering.xml"]; !ok {
		t.Fatal("sample must produce word/numbering.xml (lists)")
	}
	for name, xmlText := range parts {
		for _, bad := range []string{"red; }", "url(x)", `Arial"`, "<w:b/>Arial", "rgb(1,2,3)", "var(--x)", "linear-gradient", "#ggg", `w:val="x`} {
			if strings.Contains(xmlText, bad) {
				t.Errorf("%s: hostile value %q reached the XML", name, bad)
			}
		}
		// Todo atributo de color es hex de 6 dígitos (o "auto"/valor de enumeración de Word).
		for _, m := range xmlColorAttrRe.FindAllStringSubmatch(xmlText, -1) {
			if v := m[1]; strings.ContainsAny(v, `<>";{}()`) {
				t.Errorf("%s: suspicious attribute value %q", name, v)
			}
		}
		// Toda fuente es una de las de professional.
		for _, m := range xmlFontAttrRe.FindAllStringSubmatch(xmlText, -1) {
			if v := m[1]; !knownFonts[v] {
				t.Errorf("%s: unexpected font %q", name, v)
			}
		}
	}
	// El resultado es el mismo que sin ninguna variable (professional), parte por parte.
	for _, part := range []string{"word/document.xml", "word/numbering.xml"} {
		if parts[part] != blank[part] {
			t.Errorf("%s differs from the output of a theme with no usable variables", part)
		}
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
	// La primera familia no genérica gana.
	for in, want := range map[string]string{
		"system-ui, 'Inter', sans-serif":                        "Inter",
		"-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto": "Segoe UI",
	} {
		if got, valid := sanitizeThemeFont(in); !valid || got != want {
			t.Errorf("sanitizeThemeFont(%q) = %q,%v want %q", in, got, valid, want)
		}
	}
	for _, in := range []string{"", "''", "sans-serif", "system-ui, serif", "Café", `Arial"><w:b/>`, "url(x)", "A;B", "a<b", "system-ui, A;B", strings.Repeat("a", 65)} {
		if got, valid := sanitizeThemeFont(in); valid {
			t.Errorf("sanitizeThemeFont(%q) accepted as %q", in, got)
		}
	}
	if _, valid := sanitizeThemeFont(strings.Repeat("a", 64)); !valid {
		t.Errorf("64-char name should be accepted")
	}
}
