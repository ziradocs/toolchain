// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadTheme_UntrustedTraversalRejected es una regresión para
// docs/SECURITY_AUDIT_2026-07.md ME-2: un nombre de tema no confiable
// (frontmatter) con ".." no debe leer archivos fuera de externalPaths.
func TestLoadTheme_UntrustedTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "secret.json"), []byte(`{"name":"leaked"}`), 0644); err != nil {
		t.Fatal(err)
	}

	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		t.Fatal(err)
	}

	loader := NewThemeLoaderWithPaths([]string{themesDir})

	if _, err := loader.LoadTheme("../secret", false); err == nil {
		t.Fatal("expected an error for an untrusted traversal theme name")
	}
}

// TestLoadTheme_UntrustedRawPathShortcutBlocked es una regresión para el
// gap más amplio de ME-2: findAndLoadExternalTheme trataba cualquier name
// que "pareciera" una ruta (contuviera "/", "\" o terminara en ".json")
// como una ruta de archivo cruda, evadiendo por completo externalPaths.
func TestLoadTheme_UntrustedRawPathShortcutBlocked(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.json")
	if err := os.WriteFile(secretPath, []byte(`{"name":"leaked"}`), 0644); err != nil {
		t.Fatal(err)
	}

	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		t.Fatal(err)
	}

	loader := NewThemeLoaderWithPaths([]string{themesDir})

	// Una ruta absoluta como "nombre de tema" no confiable no debe cargarse
	// directamente, aunque el archivo exista.
	if _, err := loader.LoadTheme(secretPath, false); err == nil {
		t.Fatal("expected an untrusted absolute-path theme name to be rejected")
	}
}

// TestLoadTheme_TrustedRawPathStillWorks confirma que el operador (trusted)
// sigue pudiendo cargar un tema por ruta explícita, preservando la UX.
func TestLoadTheme_TrustedRawPathStillWorks(t *testing.T) {
	dir := t.TempDir()
	themePath := filepath.Join(dir, "custom.json")
	manifest := `{
		"name": "custom",
		"version": "1.0.0",
		"description": "test",
		"author": "test",
		"compatibility": {"min_version": "1.0.0"},
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
	if err := os.WriteFile(themePath, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewThemeLoaderWithPaths([]string{})

	theme, err := loader.LoadTheme(themePath, true)
	if err != nil {
		t.Fatalf("expected a trusted raw theme path to load: %v", err)
	}
	if theme.Name != "custom" {
		t.Errorf("expected theme name 'custom', got %q", theme.Name)
	}
}

func TestLoadTheme_EmbeddedThemeAlwaysAllowed(t *testing.T) {
	loader := NewThemeLoaderWithPaths([]string{})

	theme, err := loader.LoadTheme("default", false)
	if err != nil {
		t.Fatalf("expected the embedded 'default' theme to load even when untrusted: %v", err)
	}
	if theme.Name == "" {
		t.Fatal("expected a non-empty theme name")
	}
}
