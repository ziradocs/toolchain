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

// themeManifestWithFont builds a minimal, otherwise-valid theme.json
// declaring a single font asset at the given 'local' path — shared by the
// TestValidateAssets_FontFile* cases below, which each vary only what
// exists on disk at that path.
func themeManifestWithFont(localPath string) string {
	return `{
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
    "fonts": [{"name":"Local Font","local":"` + localPath + `"}]
  }
}`
}

// TestValidateAssets_FontFileMissing is a code-review-flagged gap: before
// this test existed, `themes validate --strict` passed a theme whose font
// 'local' pointed at a file that plain doesn't exist — loadAssets
// (external.go) only os.Stats the path to fill in Size and silently
// swallows the error otherwise, and the old validateAssets only checked
// that Path was a non-empty string. The build would then also fail
// silently: buildFontFaceRule hits the same missing file, logs a warning,
// and just omits the @font-face rule — exactly the invisible
// system-font-fallback failure mode §2.3 exists to eliminate.
func TestValidateAssets_FontFileMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(path, []byte(themeManifestWithFont("fonts/does-not-exist.woff2")), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadExternalTheme(path)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	if result.IsValid {
		t.Fatal("expected a font asset whose 'local' file doesn't exist to fail validation")
	}
	if !containsSubstring(result.Errors, "does not exist") {
		t.Errorf("expected an error about the missing file, got: %v", result.Errors)
	}
}

// TestValidateAssets_FontFileUnsupportedExtension mirrors buildFontFaceRule's
// own extension allowlist (fonts.go's fontFormatFor) — a font with a real
// file but an extension the build doesn't know how to emit a format() hint
// for should fail validation, not silently build without the @font-face.
func TestValidateAssets_FontFileUnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	fontPath := filepath.Join(dir, "fonts", "local.eot")
	if err := os.MkdirAll(filepath.Dir(fontPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fontPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(path, []byte(themeManifestWithFont("fonts/local.eot")), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadExternalTheme(path)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	if result.IsValid {
		t.Fatal("expected an unsupported font extension to fail validation")
	}
	if !containsSubstring(result.Errors, "unsupported font extension") {
		t.Errorf("expected an error about the unsupported extension, got: %v", result.Errors)
	}
}

// TestValidateAssets_FontFileIsDirectory covers 'local' pointing at a
// directory instead of a file — os.Stat alone (what loadAssets already
// does) doesn't distinguish the two, so this needs its own IsDir check.
func TestValidateAssets_FontFileIsDirectory(t *testing.T) {
	dir := t.TempDir()
	fontDir := filepath.Join(dir, "fonts", "local.woff2")
	if err := os.MkdirAll(fontDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(path, []byte(themeManifestWithFont("fonts/local.woff2")), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadExternalTheme(path)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	if result.IsValid {
		t.Fatal("expected a directory at 'local' to fail validation")
	}
	if !containsSubstring(result.Errors, "is a directory") {
		t.Errorf("expected an error about 'local' being a directory, got: %v", result.Errors)
	}
}

// TestValidateAssets_FontFileTraversalRejected confirms the validator
// rejects an out-of-theme 'local' the same way buildFontFaceRule's
// util.ResolveConfinedPath call does at build time — a font backed by a
// dangling reference to a file outside the theme directory must never be
// reported valid.
func TestValidateAssets_FontFileTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(path, []byte(themeManifestWithFont("../../../etc/passwd")), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadExternalTheme(path)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	if result.IsValid {
		t.Fatal("expected an out-of-theme 'local' path to fail validation")
	}
	if !containsSubstring(result.Errors, "invalid 'local' path") {
		t.Errorf("expected an error about the invalid path, got: %v", result.Errors)
	}
}

// TestValidateAssets_FontNameMissing is a code-review-flagged gap: a font
// asset with a real, readable file but no 'name' passed validation before
// this check existed, yet buildFontFaceRule (fonts.go) drops any entry
// with an empty Name outright — an @font-face needs a font-family, so
// there's nothing sensible to emit. The theme silently shipped a valid
// font file backing zero @font-face rules — the exact "fell back to a
// system font with no visible error" failure mode §2.3 exists to
// eliminate, reached through the 'name' field instead of 'local' this
// time.
func TestValidateAssets_FontNameMissing(t *testing.T) {
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
    "fonts": [{"local":"fonts/local.woff2"}]
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
	if result.IsValid {
		t.Fatal("expected a font asset with no 'name' to fail validation")
	}
	if !containsSubstring(result.Errors, "requires 'name'") {
		t.Errorf("expected an error about the missing 'name', got: %v", result.Errors)
	}
}

// TestValidateAssets_FontFileNotRegular covers 'local' pointing at a FIFO
// with a font extension — os.Stat's IsDir() alone (the old check) doesn't
// catch this, and worse than a validation false positive, os.ReadFile on a
// FIFO at build time (buildFontFaceRule) blocks forever with no
// diagnostic. Skipped on platforms without a FIFO syscall (Windows).
func TestValidateAssets_FontFileNotRegular(t *testing.T) {
	dir := t.TempDir()
	fontPath := filepath.Join(dir, "fonts", "local.woff2")
	if err := os.MkdirAll(filepath.Dir(fontPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := mkfifoForTest(fontPath); err != nil {
		t.Skipf("FIFO not supported on this platform: %v", err)
	}
	path := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(path, []byte(themeManifestWithFont("fonts/local.woff2")), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadExternalTheme(path)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	if result.IsValid {
		t.Fatal("expected a FIFO at 'local' to fail validation")
	}
	if !containsSubstring(result.Errors, "not a regular file") {
		t.Errorf("expected an error about 'local' not being a regular file, got: %v", result.Errors)
	}
}

// TestValidateAssets_FontFileUnreadable covers a real, regular font file
// the process can't read (permission denied). Skipped when the test
// process can read anything regardless of mode bits (running as root, or
// a filesystem that ignores them) — os.Chmod succeeding is not proof of
// that, so this actually attempts the read rather than trusting the mode
// bits alone.
func TestValidateAssets_FontFileUnreadable(t *testing.T) {
	dir := t.TempDir()
	fontPath := filepath.Join(dir, "fonts", "local.woff2")
	if err := os.MkdirAll(filepath.Dir(fontPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fontPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fontPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(fontPath, 0644); err != nil {
			t.Errorf("cleanup: chmod: %v", err)
		}
	})

	if f, err := os.Open(fontPath); err == nil {
		_ = f.Close()
		t.Skip("process can read a 0000-mode file (likely running as root); permission check not testable here")
	}

	path := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(path, []byte(themeManifestWithFont("fonts/local.woff2")), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadExternalTheme(path)
	if err != nil {
		t.Fatalf("LoadExternalTheme: %v", err)
	}

	result := NewThemeValidator().ValidateThemeDetailed(theme)
	if result.IsValid {
		t.Fatal("expected an unreadable 'local' file to fail validation")
	}
	if !containsSubstring(result.Errors, "not readable") {
		t.Errorf("expected an error about 'local' not being readable, got: %v", result.Errors)
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
