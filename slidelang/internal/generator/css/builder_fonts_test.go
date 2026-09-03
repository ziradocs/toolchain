// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package css

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeExternalThemeWithFont writes <dir>/<name>/theme.json + a font file,
// declaring the font via assets.fonts. Mirrors writeMinimalExternalTheme
// (builder_namespace_test.go) but adds the assets.fonts section §2.3 needs.
func writeExternalThemeWithFont(t *testing.T, root, name string) {
	t.Helper()
	themeDir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(themeDir, "fonts"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "name": "` + name + `",
  "version": "1.0.0",
  "description": "test theme",
  "author": "test",
  "variables": {
    "--slidelang-background-color": "#ffffff",
    "--slidelang-text-color": "#111111",
    "--slidelang-font-main": "'Brand Sans', sans-serif"
  },
  "assets": {
    "fonts": [{"name":"Brand Sans","local":"fonts/brand.woff2","weight":"400"}]
  }
}`
	if err := os.WriteFile(filepath.Join(themeDir, "theme.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "fonts", "brand.woff2"), []byte("fake woff2 bytes"), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestCSSBuilder_EmitsThemeFonts is the end-to-end regression for §2.3
// (motor-temas-v2.md): a theme declaring assets.fonts must produce an
// embedded @font-face in the real Build() bundle, driven through the real
// disk-loading path (ThemeLoader → GetExternalTheme), the same one
// TestCSSBuilder_ExternalThemeCSSIsNamespaced uses for §2.1.
func TestCSSBuilder_EmitsThemeFonts(t *testing.T) {
	dir := t.TempDir()
	writeExternalThemeWithFont(t, dir, "font-test-theme")
	t.Setenv("SLIDELANG_THEMES_PATH", dir)

	out := NewCSSBuilder().
		WithTheme("font-test-theme").
		WithRequiredElements([]string{"text"}).
		Build()

	if !strings.Contains(out, "/* === THEME FONTS === */") {
		t.Fatal("expected a THEME FONTS block — theme failed to load or GenerateFontFaceCSS wasn't called, check manifest validation")
	}
	if !strings.Contains(out, `font-family: "Brand Sans";`) {
		t.Error("expected the @font-face to declare font-family \"Brand Sans\"")
	}
	if !strings.Contains(out, "src: url(data:font/woff2;base64,") {
		t.Error("expected the font to be embedded as a data: URI")
	}
}

// TestCSSBuilder_NoFontsThemeUnchanged is the byte-for-byte non-regression:
// no theme in the repo declares assets.fonts today, so every existing deck
// must produce IDENTICAL output to before §2.3 — not even an empty THEME
// FONTS comment block.
func TestCSSBuilder_NoFontsThemeUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeMinimalExternalTheme(t, dir, "no-font-theme", `.slide blockquote {
    background: var(--bg-code);
}`)
	t.Setenv("SLIDELANG_THEMES_PATH", dir)

	out := NewCSSBuilder().
		WithTheme("no-font-theme").
		WithRequiredElements([]string{"text"}).
		Build()

	if strings.Contains(out, "THEME FONTS") {
		t.Error("expected no THEME FONTS block at all when the theme declares no fonts")
	}
	if strings.Contains(out, "@font-face") {
		t.Error("expected no @font-face at all when the theme declares no fonts")
	}
}
