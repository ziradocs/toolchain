// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeModeFixture escribe un .doclang mínimo cuyo frontmatter declara mode
// (o ninguno, si mode == "").
func writeModeFixture(t *testing.T, dir, mode string) string {
	t.Helper()
	frontmatter := "title: \"Mode Fixture\"\n"
	if mode != "" {
		frontmatter += "mode: " + mode + "\n"
	}
	content := "---\n" + frontmatter + "---\n\n# Fixture\n\nHello world.\n"
	path := filepath.Join(dir, "fixture.doclang")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	return path
}

// TestE2E_StrictModeIsRejected fija el contrato de la fase 0: un documento
// que declara `mode: strict` NO se construye. Antes se construía en
// silencio como flex —normalizador incluido— pese a declarar el dialecto
// que promete no ser reinterpretado.
func TestE2E_StrictModeIsRejected(t *testing.T) {
	dir := t.TempDir()
	fixture := writeModeFixture(t, dir, "strict")

	err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", fixture, "--output", dir, "--lint-only"})
	if err == nil {
		t.Fatal("expected `doclang build` to fail on a document declaring mode: strict, got nil error")
	}
	if !strings.Contains(err.Error(), "parse errors found") {
		t.Errorf("expected the build to abort on the parse-error path, got: %v", err)
	}
}

// El rechazo debe alcanzar también a fmt: formatear un doc strict lo
// reescribiría en dialecto flex, que es exactamente lo contrario de lo que
// el autor declaró.
func TestE2E_StrictModeIsRejectedByFmt(t *testing.T) {
	dir := t.TempDir()
	fixture := writeModeFixture(t, dir, "strict")

	err := runCLI(t, Options{Name: "doclang", Version: "test"}, []string{"fmt", fixture})
	if err == nil {
		t.Fatal("expected `doclang fmt` to fail on a document declaring mode: strict, got nil error")
	}
	if !strings.Contains(err.Error(), "MODE001") {
		t.Errorf("expected the fmt error to name MODE001, got: %v", err)
	}

	// El archivo no se tocó: fmt sin --write nunca escribe, pero la
	// aserción vale como guard contra un futuro reorden que ponga el
	// rechazo después de la escritura.
	after, readErr := os.ReadFile(fixture)
	if readErr != nil {
		t.Fatalf("failed to re-read fixture: %v", readErr)
	}
	if !strings.Contains(string(after), "mode: strict") {
		t.Error("fmt rewrote a rejected strict document")
	}
}

// Control negativo, y la garantía de compatibilidad que importa: los modos
// que el corpus de examples/ realmente usa siguen construyendo. Sin esto,
// un rechazo demasiado goloso (p. ej. gateado por prefijo) pasaría
// desapercibido.
func TestE2E_NonStrictModesStillBuild(t *testing.T) {
	for _, mode := range []string{"flex", "flex-full", "auto", ""} {
		name := mode
		if name == "" {
			name = "unset"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			fixture := writeModeFixture(t, dir, mode)

			if err := runCLI(t, Options{Name: "doclang", Version: "test"},
				[]string{"build", fixture, "--output", dir, "--lint-only"}); err != nil {
				t.Fatalf("expected mode %q to keep building, got: %v", mode, err)
			}
		})
	}
}
