// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package css

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMinimalExternalTheme writes <dir>/<name>/theme.json + styles.css. The
// manifest declares every variable ValidateTheme requires that
// extractVariables' defaultVars does NOT backfill (--slidelang-background-
// color, --slidelang-text-color) plus the one the test's styles.css
// references, so LoadTheme's validation step does not reject the theme and
// silently fall back to "default" before we ever reach CSSBuilder.Build().
func writeMinimalExternalTheme(t *testing.T, root, name, stylesCSS string) {
	t.Helper()
	themeDir := filepath.Join(root, name)
	if err := os.MkdirAll(themeDir, 0755); err != nil {
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
    "--slidelang-bg-code": "#f7fafc"
  }
}`
	if err := os.WriteFile(filepath.Join(themeDir, "theme.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "styles.css"), []byte(stylesCSS), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestCSSBuilder_ExternalThemeCSSIsNamespaced is the end-to-end regression
// for §2.1 (docs/developer/motor-temas-v2.md): CSSBuilder.Build() used to
// write an external theme's styles.css into the "EXTERNAL THEME CSS" block
// raw, so var(--bg-code) never resolved against the --slidelang-bg-code the
// :root block above it actually emits. This drives the real disk-loading
// path (ThemeLoader → GetExternalTheme), not just the namespacer directly.
func TestCSSBuilder_ExternalThemeCSSIsNamespaced(t *testing.T) {
	dir := t.TempDir()
	writeMinimalExternalTheme(t, dir, "unprefixed-test-theme", `.slide blockquote {
    border-left: 4px solid var(--primary-color);
    background: var(--bg-code);
}`)
	t.Setenv("SLIDELANG_THEMES_PATH", dir)

	out := NewCSSBuilder().
		WithTheme("unprefixed-test-theme").
		WithRequiredElements([]string{"text"}).
		Build()

	if !strings.Contains(out, "/* === EXTERNAL THEME CSS === */") {
		t.Fatal("expected the external theme's CSS block to be present — theme failed to load, check manifest validation")
	}
	if strings.Contains(out, "var(--bg-code)") {
		t.Error("found un-namespaced var(--bg-code) in the bundle — external theme CSS is not going through NamespaceStylesheet")
	}
	if !strings.Contains(out, "var(--slidelang-bg-code)") {
		t.Error("expected var(--slidelang-bg-code) in the bundle")
	}
	if !strings.Contains(out, "var(--slidelang-primary-color)") {
		t.Error("expected var(--slidelang-primary-color) in the bundle")
	}
}

// TestCSSBuilder_ExternalThemeDeclarationIsNamespaced covers the other half
// of the §2.1 decision: a theme author's own local custom-property
// *declaration* (not just its usages) must be namespaced too, so a helper
// like --helper-spacing declared and used within the same styles.css keeps
// working after namespacing.
func TestCSSBuilder_ExternalThemeDeclarationIsNamespaced(t *testing.T) {
	dir := t.TempDir()
	writeMinimalExternalTheme(t, dir, "local-decl-theme", `:root {
    --helper-spacing: 4px;
}
.slide .card {
    padding: var(--helper-spacing);
}`)
	t.Setenv("SLIDELANG_THEMES_PATH", dir)

	out := NewCSSBuilder().
		WithTheme("local-decl-theme").
		WithRequiredElements([]string{"text"}).
		Build()

	if !strings.Contains(out, "--slidelang-helper-spacing: 4px") {
		t.Error("expected the local declaration to be namespaced to --slidelang-helper-spacing")
	}
	if !strings.Contains(out, "var(--slidelang-helper-spacing)") {
		t.Error("expected the usage to resolve against the namespaced declaration")
	}
}

// TestCSSBuilder_ModernBlueLiveRegression loads the real, currently-shipped
// modern-blue theme from slidelang/themes/ — not a synthetic fixture — and
// checks the exact blockquote rule motor-temas-v2.md §2.1 calls out
// (styles.css:154-160): three var() references with no --slidelang- prefix,
// none of which resolved against the theme's own :root block. Its source
// CSS was itself corrected (code-review finding on PR #223: namespacing
// only the var()s left the RULE dead regardless — the generated HTML only
// ever emits class="slidelang-slide", so ".slide blockquote" never matched
// anything no matter what its declarations resolved to), so this test now
// checks the SELECTOR too, not just the variable names — that is the part
// a var()-only assertion cannot catch.
func TestCSSBuilder_ModernBlueLiveRegression(t *testing.T) {
	themesDir, err := filepath.Abs(filepath.Join("..", "..", "..", "themes"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(themesDir, "modern-blue", "styles.css")); err != nil {
		t.Skipf("real themes dir not found at %s (%v) — skipping live-theme regression", themesDir, err)
	}
	t.Setenv("SLIDELANG_THEMES_PATH", themesDir)

	out := NewCSSBuilder().
		WithTheme("modern-blue").
		WithRequiredElements([]string{"text", "quotes"}).
		Build()

	if !strings.Contains(out, "/* === EXTERNAL THEME CSS === */") {
		t.Fatal("modern-blue failed to load as an external theme — check its theme.json against ValidateTheme's required variables")
	}
	for _, unwanted := range []string{"var(--primary-color)", "var(--secondary-color)", "var(--bg-code)"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("found un-namespaced %s in the bundle — modern-blue's blockquote is still not resolving", unwanted)
		}
	}
	for _, wanted := range []string{"var(--slidelang-primary-color)", "var(--slidelang-secondary-color)", "var(--slidelang-bg-code)"} {
		if !strings.Contains(out, wanted) {
			t.Errorf("expected %s in the bundle", wanted)
		}
	}

	// The selector check: the generated HTML's slide container only ever
	// carries class="slidelang-slide" (see
	// internal/generator/template/integration_test.go), so a blockquote
	// rule still scoped to the bare ".slide" never matches any element,
	// no matter how correctly its var()s resolve.
	if strings.Contains(out, ".slide blockquote") {
		t.Error("found un-namespaced selector '.slide blockquote' in the bundle — this rule can never match slidelang-slide, so the blockquote stays unstyled regardless of its variables")
	}
	if !strings.Contains(out, ".slidelang-slide blockquote") {
		t.Error("expected the namespaced selector '.slidelang-slide blockquote' in the bundle")
	}
}

// TestCSSBuilder_StartupTechLiveRegression is the second-round PR #223
// finding: CSSFileLoader.ApplyNamespacing's default ExcludeClasses used to
// list plain HTML tag names ("table", "button", etc.) meant to guard bare
// tag selectors that classRegex can never match anyway (it requires a
// leading "."), whose only real effect was silently skipping a genuine
// CLASS sharing that name — confirmed for ".table", the engine's own
// slidelang-table wrapper class. Conversely ".tabs" (the code-group's
// bare <div class="tabs"> container, template/base.go) was WRONGLY
// prefixed to ".slidelang-tabs", which the real markup never carries
// either. Loads the real, currently-shipped startup-tech theme — not a
// synthetic fixture.
func TestCSSBuilder_StartupTechLiveRegression(t *testing.T) {
	themesDir, err := filepath.Abs(filepath.Join("..", "..", "..", "themes"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(themesDir, "startup-tech", "styles.css")); err != nil {
		t.Skipf("real themes dir not found at %s (%v) — skipping live-theme regression", themesDir, err)
	}
	t.Setenv("SLIDELANG_THEMES_PATH", themesDir)

	out := NewCSSBuilder().
		WithTheme("startup-tech").
		WithRequiredElements([]string{"text", "tables", "code"}).
		Build()

	if !strings.Contains(out, "/* === EXTERNAL THEME CSS === */") {
		t.Fatal("startup-tech failed to load as an external theme — check its theme.json against ValidateTheme's required variables")
	}

	if strings.Contains(out, ".slidelang-element.table ") {
		t.Error("found un-namespaced selector '.slidelang-element.table' — this can never match the real slidelang-table wrapper class, so table rules stay unstyled")
	}
	if !strings.Contains(out, ".slidelang-element.slidelang-table") {
		t.Error("expected the namespaced selector '.slidelang-element.slidelang-table' in the bundle")
	}

	if strings.Contains(out, ".slidelang-code-group .slidelang-tabs") {
		t.Error("found over-prefixed selector '.slidelang-code-group .slidelang-tabs' — the code-group's bare <div class=\"tabs\"> container never carries this class")
	}
	if !strings.Contains(out, ".slidelang-element.slidelang-code-group .tabs") {
		t.Error("expected the (correctly bare) selector '.slidelang-element.slidelang-code-group .tabs' in the bundle")
	}

	if !strings.Contains(out, ".slidelang-tab.active") {
		t.Error("expected the (correctly bare) compound selector '.slidelang-tab.active' in the bundle")
	}
}
