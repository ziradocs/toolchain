// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadTheme_UntrustedTraversalRejected es una regresión para
// docs/SECURITY_AUDIT_2026-07.md ME-2: un nombre de tema no confiable
// (frontmatter) con ".." no debe leer archivos fuera de los searchPaths.
func TestLoadTheme_UntrustedTraversalRejected(t *testing.T) {
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

	_, err := loader.LoadTheme("../secret", false)
	if err == nil {
		t.Fatal("expected an error for an untrusted traversal theme name")
	}

	// Un nombre no confiable pero legítimo (sin separadores/".." ) sigue
	// intentando resolverse normalmente (aunque no exista, cae al fallback
	// professional con un error de "not found", no de traversal).
	_, err = loader.LoadTheme("nonexistent-legit-name", false)
	if err == nil {
		t.Fatal("expected a 'not found' error for a nonexistent theme")
	}
}

// TestLoadTheme_TrustedNameUnaffected confirma que el flag --theme del
// operador (trusted=true) sigue resolviendo nombres legítimos sin cambios.
func TestLoadTheme_TrustedNameUnaffected(t *testing.T) {
	loader := NewThemeLoader()

	theme, err := loader.LoadTheme("professional", true)
	if err != nil {
		t.Fatalf("expected the embedded 'professional' theme to load: %v", err)
	}
	if theme.Name == "" {
		t.Fatal("expected a non-empty theme name")
	}
}

func TestLoadTheme_EmbeddedThemeAlwaysAllowed(t *testing.T) {
	loader := NewThemeLoader()

	// Un tema embebido se resuelve por su mapa interno antes de cualquier
	// validación de path, sin importar trusted.
	theme, err := loader.LoadTheme("professional", false)
	if err != nil {
		t.Fatalf("expected the embedded 'professional' theme to load even when untrusted: %v", err)
	}
	if theme.Name == "" {
		t.Fatal("expected a non-empty theme name")
	}
}
