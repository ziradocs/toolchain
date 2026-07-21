// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// Issue #47: createTheme usaba os.Create/os.WriteFile, que truncan en
// silencio si el theme.json/styles.css/README.md de un tema ya existían —
// un segundo `themes create <name>` sobre el mismo nombre pisaba el tema sin
// avisar. Ahora debe rechazarse sin --force cuando el directorio destino ya
// tiene contenido.
func TestCreateTheme_RejectsExistingNonEmptyDir_WithoutForce(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "my-theme")
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	preexisting := filepath.Join(outputPath, "theme.json")
	if err := os.WriteFile(preexisting, []byte(`{"name":"old"}`), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}

	err := createTheme("my-theme", "business", outputPath, "", "", false, true, "", "", false)
	if err == nil {
		t.Fatal("se esperaba un error al crear sobre un directorio no vacío sin --force")
	}

	// Confirmar que el archivo preexistente no se tocó.
	content, readErr := os.ReadFile(preexisting)
	if readErr != nil {
		t.Fatalf("os.ReadFile failed: %v", readErr)
	}
	if string(content) != `{"name":"old"}` {
		t.Errorf("el archivo preexistente fue modificado a pesar del rechazo: %q", content)
	}
}

func TestCreateTheme_OverwritesExistingDir_WithForce(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "my-theme")
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	preexisting := filepath.Join(outputPath, "theme.json")
	if err := os.WriteFile(preexisting, []byte(`{"name":"old"}`), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}

	if err := createTheme("my-theme", "business", outputPath, "", "", false, true, "", "", true); err != nil {
		t.Fatalf("createTheme con --force falló: %v", err)
	}

	content, err := os.ReadFile(preexisting)
	if err != nil {
		t.Fatalf("os.ReadFile failed: %v", err)
	}
	if string(content) == `{"name":"old"}` {
		t.Error("con --force, theme.json debería haberse sobrescrito con el manifest nuevo")
	}
}

func TestCreateTheme_SucceedsOnEmptyOrNewDir_WithoutForce(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "brand-new-theme")
	if err := createTheme("brand-new-theme", "minimal", outputPath, "", "", false, true, "", "", false); err != nil {
		t.Fatalf("createTheme sobre un directorio nuevo no debería fallar: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputPath, "theme.json")); err != nil {
		t.Errorf("theme.json no se creó: %v", err)
	}
}

// Hallazgo de security-review de PR #147: cuando no se pasa --output, el
// nombre de tema sin validar alimentaba filepath.Join("themes", themeName)
// — exactamente el mismo vector de traversal que este PR cierra para
// `doclang init`, pero sin cerrar en el archivo hermano. Se ejercita vía
// NewThemesCmd real (no createTheme directo) porque el guard vive en el
// RunE, antes de calcular el outputPath por defecto.
func TestThemesCreate_RejectsPathTraversalInName(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir failed: %v", err)
	}
	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Fatalf("os.Chdir (restore) failed: %v", err)
		}
	}()

	cases := []string{"../evil", "../../evil", "sub/evil", "/etc/evil", "a..b"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := CreateThemeCmd()
			cmd.SetArgs([]string{name})
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			if err := cmd.Execute(); err == nil {
				t.Errorf("se esperaba error para el nombre %q, no hubo ninguno", name)
			}
			// Nada debió escribirse ni siquiera dentro de tmpDir/themes/.
			if _, statErr := os.Stat(filepath.Join(tmpDir, "themes")); statErr == nil {
				t.Errorf("no debería haberse creado themes/ para un nombre rechazado (%q)", name)
			}
		})
	}
}

// Hallazgo de security-review de PR #147: el guard de "directorio no vacío"
// usaba ReadDir/MkdirAll, que siguen symlinks de forma transparente — un
// symlink plantado en outputPath podía apuntar a otro directorio vacío,
// pasando el chequeo y haciendo que las escrituras aterrizaran ahí en vez
// de fallar. Debe rechazarse SIEMPRE que outputPath sea un symlink, incluso
// con --force.
func TestCreateTheme_RejectsSymlinkOutputPath(t *testing.T) {
	base := t.TempDir()
	victim := filepath.Join(base, "victim-empty-dir")
	if err := os.MkdirAll(victim, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	symlinkPath := filepath.Join(base, "themes-link")
	if err := os.Symlink(victim, symlinkPath); err != nil {
		t.Skipf("no se pudo crear symlink en este entorno: %v", err)
	}

	for _, force := range []bool{false, true} {
		err := createTheme("my-theme", "business", symlinkPath, "", "", false, true, "", "", force)
		if err == nil {
			t.Errorf("se esperaba error escribiendo a través de un symlink (force=%v), no hubo ninguno", force)
		}
	}

	entries, err := os.ReadDir(victim)
	if err != nil {
		t.Fatalf("os.ReadDir failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("no debería haberse escrito nada a través del symlink, se encontraron %d entradas en %s", len(entries), victim)
	}
}

// Hallazgo de code-review de PR #147: la versión anterior del guard trataba
// cualquier error de os.ReadDir (no solo "no existe") como "seguro para
// proceder" — un directorio sin permiso de lectura pasaba el chequeo en
// silencio. Se salta el test si el entorno corre como root (chmod 0000 no
// bloquea lecturas para root, el caso no sería representativo).
func TestCreateTheme_SurfacesRealErrorOnUnreadableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("corriendo como root: chmod 0000 no bloquea lecturas, el test no es representativo")
	}

	outputPath := filepath.Join(t.TempDir(), "unreadable-theme")
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	if err := os.Chmod(outputPath, 0000); err != nil {
		t.Fatalf("os.Chmod failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(outputPath, 0755) }) // permite que t.TempDir() limpie

	err := createTheme("unreadable-theme", "business", outputPath, "", "", false, true, "", "", false)
	if err == nil {
		t.Fatal("se esperaba un error de I/O real (no silenciado como \"seguro para proceder\")")
	}
}
