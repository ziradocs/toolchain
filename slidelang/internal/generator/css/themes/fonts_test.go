// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFontFixtureTheme writes a theme.json + font file combo under a fresh
// t.TempDir() and returns the *ExternalTheme loaded from disk via
// LoadExternalTheme — the only entry point that populates both .Path
// (needed to resolve "local" against the theme directory) and .Assets
// (loadAssets(), needed by the validator's font checks). manifestJSON must
// contain a "%s" placeholder for the font's relative path, so the caller
// controls the font's on-disk location relative to the theme.json.
func writeFontFixtureTheme(t *testing.T, manifestJSON string, fontRelPath string, fontBytes []byte) *ExternalTheme {
	t.Helper()
	dir := t.TempDir()

	fontFullPath := filepath.Join(dir, fontRelPath)
	if err := os.MkdirAll(filepath.Dir(fontFullPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fontFullPath, fontBytes, 0644); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0644); err != nil {
		t.Fatal(err)
	}

	theme, err := LoadExternalTheme(manifestPath)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}
	return theme
}

const fontManifestTemplate = `{
  "name": "test-theme",
  "version": "1.0.0",
  "description": "test",
  "author": "test",
  "variables": {
    "--slidelang-primary-color": "#000",
    "--slidelang-secondary-color": "#111",
    "--slidelang-font-main": "sans-serif",
    "--slidelang-font-size-base": "1rem",
    "--slidelang-line-height-base": "1.5",
    "--slidelang-background-color": "#fff",
    "--slidelang-text-color": "#000"
  },
  "assets": {
    "fonts": [%s]
  }
}`

func TestGenerateFontFaceCSS_EmitsDataURI(t *testing.T) {
	fontDescriptor := `{"name":"Test Font","local":"fonts/test.woff2","weight":"700","style":"normal","display":"block"}`
	manifest := fmt.Sprintf(fontManifestTemplate, fontDescriptor)
	theme := writeFontFixtureTheme(t, manifest, "fonts/test.woff2", []byte("fake woff2 bytes"))

	css := GenerateFontFaceCSS(theme)

	for _, want := range []string{
		"@font-face {",
		`font-family: "Test Font";`,
		"src: url(data:font/woff2;base64,",
		`format("woff2")`,
		"font-weight: 700;",
		"font-style: normal;",
		"font-display: block;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, css)
		}
	}
}

func TestGenerateFontFaceCSS_DefaultsToSwapDisplay(t *testing.T) {
	fontDescriptor := `{"name":"Test Font","local":"fonts/test.woff2"}`
	manifest := fmt.Sprintf(fontManifestTemplate, fontDescriptor)
	theme := writeFontFixtureTheme(t, manifest, "fonts/test.woff2", []byte("fake woff2 bytes"))

	css := GenerateFontFaceCSS(theme)

	if !strings.Contains(css, "font-display: swap;") {
		t.Errorf("expected default font-display: swap, got:\n%s", css)
	}
	// No weight/style declared -> no descriptor line for either, not a
	// guessed default.
	if strings.Contains(css, "font-weight:") {
		t.Errorf("expected no font-weight descriptor when none was declared, got:\n%s", css)
	}
	if strings.Contains(css, "font-style:") {
		t.Errorf("expected no font-style descriptor when none was declared, got:\n%s", css)
	}
}

func TestGenerateFontFaceCSS_LocalOutsideThemeDirRejected(t *testing.T) {
	fontDescriptor := `{"name":"Evil Font","local":"../../../etc/passwd"}`
	manifest := fmt.Sprintf(fontManifestTemplate, fontDescriptor)

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadExternalTheme(manifestPath)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}

	css := GenerateFontFaceCSS(theme)
	if css != "" {
		t.Errorf("expected an out-of-theme 'local' path to be rejected and emit nothing, got:\n%s", css)
	}
}

func TestGenerateFontFaceCSS_NameEscaped(t *testing.T) {
	maliciousName := `x"; } body { display: none } @font-face { font-family: "y`
	nameJSON, err := json.Marshal(maliciousName)
	if err != nil {
		t.Fatal(err)
	}
	fontDescriptor := `{"name":` + string(nameJSON) + `,"local":"fonts/test.woff2"}`
	manifest := fmt.Sprintf(fontManifestTemplate, fontDescriptor)
	theme := writeFontFixtureTheme(t, manifest, "fonts/test.woff2", []byte("fake woff2 bytes"))

	css := GenerateFontFaceCSS(theme)

	// The malicious name's embedded quotes must all be escaped, so the
	// font-family declaration has exactly ONE unescaped opening quote and
	// ONE unescaped closing quote — not extras that would mean the value
	// broke out of its CSS string. Counting raw '"' occurrences (2) would
	// wrongly pass here since every quote in the payload does get escaped;
	// what must be checked is that none of them survives UNESCAPED.
	line := findLine(t, css, "font-family:")
	unescapedQuotes := countUnescapedQuotes(line)
	if unescapedQuotes != 2 {
		t.Errorf("expected exactly 2 unescaped quotes (open+close) in the font-family declaration, got %d in: %s", unescapedQuotes, line)
	}
}

func findLine(t *testing.T, text, contains string) string {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, contains) {
			return line
		}
	}
	t.Fatalf("no line containing %q found in:\n%s", contains, text)
	return ""
}

func countUnescapedQuotes(s string) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '"' {
			count++
		}
	}
	return count
}

func TestGenerateFontFaceCSS_UnsupportedExtensionSkipped(t *testing.T) {
	fontDescriptor := `{"name":"Test Font","local":"fonts/test.eot"}`
	manifest := fmt.Sprintf(fontManifestTemplate, fontDescriptor)
	theme := writeFontFixtureTheme(t, manifest, "fonts/test.eot", []byte("fake eot bytes"))

	css := GenerateFontFaceCSS(theme)
	if css != "" {
		t.Errorf("expected an unsupported font extension to be skipped, got:\n%s", css)
	}
}

func TestGenerateFontFaceCSS_URLOnlyFontSkipped(t *testing.T) {
	fontDescriptor := `{"name":"Test Font","url":"https://fonts.example.com/test.woff2"}`
	manifest := fmt.Sprintf(fontManifestTemplate, fontDescriptor)

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadExternalTheme(manifestPath)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}

	css := GenerateFontFaceCSS(theme)
	if css != "" {
		t.Errorf("expected a font declared only via 'url' (no 'local') to be skipped, got:\n%s", css)
	}
}

func TestGenerateFontFaceCSS_NoFontsReturnsEmptyString(t *testing.T) {
	theme, err := LoadExternalThemeFromBytes([]byte(minimalManifest), nil)
	if err != nil {
		t.Fatalf("LoadExternalThemeFromBytes: %v", err)
	}
	if got := GenerateFontFaceCSS(theme); got != "" {
		t.Errorf("expected no assets.fonts to produce an empty string byte-for-byte, got:\n%s", got)
	}
}

func TestFontFormatFor(t *testing.T) {
	cases := []struct {
		path       string
		wantFormat string
		wantMime   string
		wantOK     bool
	}{
		{"fonts/a.woff2", "woff2", "font/woff2", true},
		{"fonts/a.WOFF2", "woff2", "font/woff2", true},
		{"fonts/a.woff", "woff", "font/woff", true},
		{"fonts/a.ttf", "truetype", "font/ttf", true},
		{"fonts/a.otf", "opentype", "font/otf", true},
		{"fonts/a.eot", "", "", false},
	}
	for _, c := range cases {
		format, mime, ok := fontFormatFor(c.path)
		if ok != c.wantOK || format != c.wantFormat || mime != c.wantMime {
			t.Errorf("fontFormatFor(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.path, format, mime, ok, c.wantFormat, c.wantMime, c.wantOK)
		}
	}
}

func TestValidatedFontWeight(t *testing.T) {
	cases := map[string]string{
		"normal":    "normal",
		"bold":      "bold",
		"700":       "700",
		"1":         "1",
		"1000":      "1000",
		"1001":      "",
		"0":         "",
		"100 900":   "100 900",
		"900 100":   "", // lo > hi
		"":          "",
		"not-a-num": "",
	}
	for in, want := range cases {
		if got := validatedFontWeight(in); got != want {
			t.Errorf("validatedFontWeight(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidatedFontStyle(t *testing.T) {
	cases := map[string]string{
		"normal":  "normal",
		"italic":  "italic",
		"oblique": "oblique",
		"slanted": "",
		"":        "",
	}
	for in, want := range cases {
		if got := validatedFontStyle(in); got != want {
			t.Errorf("validatedFontStyle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidatedFontDisplay(t *testing.T) {
	cases := map[string]string{
		"auto":     "auto",
		"block":    "block",
		"swap":     "swap",
		"fallback": "fallback",
		"optional": "optional",
		"":         "swap",
		"bogus":    "swap",
	}
	for in, want := range cases {
		if got := validatedFontDisplay(in); got != want {
			t.Errorf("validatedFontDisplay(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCSSEscapeString(t *testing.T) {
	cases := map[string]string{
		"Inter":    `"Inter"`,
		`Foo"Bar`:  `"Foo\"Bar"`,
		`Foo\Bar`:  `"Foo\\Bar"`,
		"Foo\nBar": `"FooBar"`,
	}
	for in, want := range cases {
		if got := cssEscapeString(in); got != want {
			t.Errorf("cssEscapeString(%q) = %q, want %q", in, got, want)
		}
	}
}
