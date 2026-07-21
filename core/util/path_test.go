// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfinedPath(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "home", "user", "themes")

	tests := []struct {
		name      string
		userPath  string
		wantErr   bool
		wantSufix string // suffix expected in the resolved path when no error
	}{
		{name: "simple relative file", userPath: "mytheme.json", wantErr: false, wantSufix: filepath.Join(base, "mytheme.json")},
		{name: "legitimate subdirectory", userPath: filepath.Join("sub", "theme.json"), wantErr: false, wantSufix: filepath.Join(base, "sub", "theme.json")},
		{name: "absolute path rejected", userPath: filepath.Join(string(filepath.Separator), "etc", "passwd"), wantErr: true},
		{name: "traversal escapes base", userPath: filepath.Join("..", "..", "..", "..", "etc", "passwd"), wantErr: true},
		{name: "traversal within base is fine", userPath: filepath.Join("sub", "..", "theme.json"), wantErr: false, wantSufix: filepath.Join(base, "theme.json")},
		{name: "empty path rejected", userPath: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveConfinedPath(base, tt.userPath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for userPath %q, got resolved path %q", tt.userPath, got)
				}
				if !errors.Is(err, ErrPathEscapesBase) {
					t.Errorf("expected ErrPathEscapesBase, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantSufix {
				t.Errorf("ResolveConfinedPath(%q, %q) = %q, want %q", base, tt.userPath, got, tt.wantSufix)
			}
		})
	}
}

// TestResolveConfinedPath_RejectsSymlinkEscape es una regresión encontrada
// en code-review: la comprobación léxica (Clean+Join+HasPrefix) por sí sola
// no detecta un symlink DENTRO de base cuyo destino real está fuera — un
// documento compartido en una carpeta/zip junto a un "logo.png" que en
// realidad es un symlink a un archivo sensible evadiría el confinamiento sin
// esta verificación adicional.
func TestResolveConfinedPath_RejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	secretPath := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(base, "logo.png")
	if err := os.Symlink(secretPath, linkPath); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveConfinedPath(base, "logo.png")
	if err == nil {
		t.Fatal("expected a symlink escaping base to be rejected")
	}
	if !errors.Is(err, ErrPathEscapesBase) {
		t.Errorf("expected ErrPathEscapesBase, got: %v", err)
	}
}

// TestResolveConfinedPath_AllowsSymlinkWithinBase confirma que un symlink
// cuyo destino real SIGUE dentro de base no se rechaza (no todo symlink es
// un escape).
func TestResolveConfinedPath_AllowsSymlinkWithinBase(t *testing.T) {
	base := t.TempDir()

	realPath := filepath.Join(base, "real-logo.png")
	if err := os.WriteFile(realPath, []byte("fake png bytes"), 0644); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(base, "logo.png")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveConfinedPath(base, "logo.png")
	if err != nil {
		t.Fatalf("expected a symlink resolving within base to be allowed, got: %v", err)
	}
	if resolved != linkPath {
		t.Errorf("expected resolved path %q, got %q", linkPath, resolved)
	}
}

// TestResolveConfinedPath_NonexistentPathNotRejectedBySymlinkCheck confirma
// que un path que aún no existe no se rechaza por la verificación de
// symlinks (EvalSymlinks falla con "not exist", no con un escape real) — la
// lectura posterior del llamador fallará por su cuenta con "not found".
func TestResolveConfinedPath_NonexistentPathNotRejectedBySymlinkCheck(t *testing.T) {
	base := t.TempDir()

	resolved, err := ResolveConfinedPath(base, "does-not-exist.png")
	if err != nil {
		t.Fatalf("expected a nonexistent-but-lexically-confined path to be allowed, got: %v", err)
	}
	if resolved != filepath.Join(base, "does-not-exist.png") {
		t.Errorf("unexpected resolved path: %q", resolved)
	}
}

func TestIsOpaquePathToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "simple name", input: "professional", want: true},
		{name: "name with hyphen", input: "modern-blue", want: true},
		{name: "absolute path rejected", input: "/etc/passwd", want: false},
		{name: "traversal rejected", input: "../../../../etc/passwd", want: false},
		{name: "forward slash rejected", input: "sub/theme", want: false},
		{name: "backslash rejected", input: `sub\theme`, want: false},
		{name: "empty rejected", input: "", want: false},
		{name: "double dot substring rejected", input: "theme..bak", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOpaquePathToken(tt.input); got != tt.want {
				t.Errorf("IsOpaquePathToken(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
