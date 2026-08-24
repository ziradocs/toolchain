// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateAssets_FontRequiresLocal is the validator half of §2.3's
// "auto-hospedar siempre": a font declared only via "url" (no "local")
// must be an Error, not silently accepted — GenerateFontFaceCSS's own
// runtime skip is not a substitute for surfacing this in `themes validate`.
func TestValidateAssets_FontRequiresLocal(t *testing.T) {
	manifest := `{
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
    "fonts": [{"name":"Remote Font","url":"https://fonts.example.com/test.woff2"}]
  }
}`
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadExternalTheme(path)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	if result.IsValid {
		t.Fatal("expected a url-only font asset to fail validation")
	}
	if !containsSubstring(result.Errors, "requires 'local'") {
		t.Errorf("expected an error about the missing 'local', got: %v", result.Errors)
	}
	if !containsSubstring(result.Errors, "must not declare 'url'") {
		t.Errorf("expected an error about the disallowed 'url', got: %v", result.Errors)
	}
}

// TestValidateAssets_FontWithLocalOnlyPasses confirms the rule above isn't
// simply always-fail: a font declared with only "local" is fine.
func TestValidateAssets_FontWithLocalOnlyPasses(t *testing.T) {
	manifest := `{
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
    "fonts": [{"name":"Local Font","local":"fonts/local.woff2"}]
  }
}`
	dir := t.TempDir()
	fontPath := filepath.Join(dir, "fonts", "local.woff2")
	if err := os.MkdirAll(filepath.Dir(fontPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fontPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadExternalTheme(path)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	if !result.IsValid {
		t.Errorf("expected a local-only font asset to pass validation, got errors: %v", result.Errors)
	}
}

// TestValidateFontFamilyBacking_WarnsWhenUnbacked is the exact failure mode
// elegant-minimal has today: a font-family stack names a family with no
// matching assets.fonts entry. Must be a Warning (still loads), not an
// Error.
func TestValidateFontFamilyBacking_WarnsWhenUnbacked(t *testing.T) {
	manifest := `{
  "name": "test-theme",
  "version": "1.0.0",
  "description": "test",
  "author": "test",
  "variables": {
    "--slidelang-primary-color": "#000",
    "--slidelang-secondary-color": "#111",
    "--slidelang-font-main": "'Crimson Text', 'Times New Roman', serif",
    "--slidelang-font-size-base": "1rem",
    "--slidelang-line-height-base": "1.5",
    "--slidelang-background-color": "#fff",
    "--slidelang-text-color": "#000"
  }
}`
	theme, err := LoadExternalThemeFromBytes([]byte(manifest), nil)
	if err != nil {
		t.Fatalf("LoadExternalThemeFromBytes: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	if !result.IsValid {
		t.Errorf("an unbacked font family must warn, not error: %v", result.Errors)
	}
	if !containsSubstring(result.Warnings, "Crimson Text") {
		t.Errorf("expected a warning naming 'Crimson Text', got: %v", result.Warnings)
	}
}

// TestValidateFontFamilyBacking_NoWarningWhenBacked confirms the rule only
// fires for the unbacked case: a font-family stack whose first entry
// matches an assets.fonts name (case-insensitively) must not warn.
func TestValidateFontFamilyBacking_NoWarningWhenBacked(t *testing.T) {
	manifest := `{
  "name": "test-theme",
  "version": "1.0.0",
  "description": "test",
  "author": "test",
  "variables": {
    "--slidelang-primary-color": "#000",
    "--slidelang-secondary-color": "#111",
    "--slidelang-font-main": "'crimson text', serif",
    "--slidelang-font-size-base": "1rem",
    "--slidelang-line-height-base": "1.5",
    "--slidelang-background-color": "#fff",
    "--slidelang-text-color": "#000"
  },
  "assets": {
    "fonts": [{"name":"Crimson Text","local":"fonts/crimson.woff2"}]
  }
}`
	theme, err := LoadExternalThemeFromBytes([]byte(manifest), nil)
	if err != nil {
		t.Fatalf("LoadExternalThemeFromBytes: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	if containsSubstring(result.Warnings, "Crimson Text") || containsSubstring(result.Warnings, "crimson text") {
		t.Errorf("expected no font-backing warning when the family is backed, got: %v", result.Warnings)
	}
}

// TestValidateFontFamilyBacking_NoWarningForGenericKeyword confirms a
// system-font-only stack (no theme-shipped font at all, a legitimate and
// common choice) never warns: its first entry is a generic CSS keyword.
func TestValidateFontFamilyBacking_NoWarningForGenericKeyword(t *testing.T) {
	manifest := `{
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
  }
}`
	theme, err := LoadExternalThemeFromBytes([]byte(manifest), nil)
	if err != nil {
		t.Fatalf("LoadExternalThemeFromBytes: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	for _, w := range result.Warnings {
		if strings.Contains(w, "font-main") {
			t.Errorf("expected no font-backing warning for a system-only stack, got: %v", result.Warnings)
		}
	}
}

func containsSubstring(items []string, substr string) bool {
	for _, item := range items {
		if strings.Contains(item, substr) {
			return true
		}
	}
	return false
}
