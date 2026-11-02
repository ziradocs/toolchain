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

// TestResolveConfinedPathFrom: la ruta se resuelve contra el directorio del
// archivo fuente y se confina a la raíz, que puede ser un ancestro de ese
// directorio (doclang build --asset-root ..).
func TestResolveConfinedPathFrom(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "proj")
	for _, dir := range []string{filepath.Join(src, "img"), filepath.Join(root, "shared")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	ok := []struct{ userPath, want string }{
		{"logo.png", filepath.Join(src, "logo.png")},
		{"./img/a.png", filepath.Join(src, "img", "a.png")},
		{"../shared/b.png", filepath.Join(root, "shared", "b.png")},
	}
	for _, tc := range ok {
		got, err := ResolveConfinedPathFrom(root, src, tc.userPath)
		if err != nil {
			t.Errorf("%q: error inesperado: %v", tc.userPath, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.userPath, got, tc.want)
		}
	}

	for _, userPath := range []string{"../../outside.png", "/etc/passwd", "../shared/../../x.png", ""} {
		if _, err := ResolveConfinedPathFrom(root, src, userPath); !errors.Is(err, ErrPathEscapesBase) {
			t.Errorf("%q: se esperaba ErrPathEscapesBase, got %v", userPath, err)
		}
	}

	// baseDir fuera de la raíz: aunque la ruta no tenga "..", escapa.
	if _, err := ResolveConfinedPathFrom(src, root, "shared/b.png"); !errors.Is(err, ErrPathEscapesBase) {
		t.Errorf("baseDir fuera de root: se esperaba ErrPathEscapesBase, got %v", err)
	}

	// baseDir vacío conserva el comportamiento de ResolveConfinedPath.
	got, err := ResolveConfinedPathFrom(root, "", "proj/logo.png")
	if err != nil || got != filepath.Join(root, "proj", "logo.png") {
		t.Errorf("baseDir vacío: got %q, %v", got, err)
	}
}

func TestResolveConfinedPathFrom_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.png")
	if err := os.WriteFile(secret, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "proj")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(src, "logo.png")); err != nil {
		t.Skipf("symlinks no disponibles: %v", err)
	}
	if _, err := ResolveConfinedPathFrom(root, src, "logo.png"); !errors.Is(err, ErrPathEscapesBase) {
		t.Errorf("symlink fuera de root: se esperaba ErrPathEscapesBase, got %v", err)
	}
}
