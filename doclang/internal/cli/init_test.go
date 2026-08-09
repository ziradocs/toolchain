// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// runInit ejecuta `doclang init <name>` dentro de un directorio temporal
// aislado y devuelve el error de Execute (si lo hay).
func runInit(t *testing.T, name string) error {
	t.Helper()

	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir failed: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origWd); err != nil {
			t.Fatalf("os.Chdir (restore) failed: %v", err)
		}
	})

	cmd := NewInitCommand()
	cmd.SetArgs([]string{name})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.Execute()
}

// Issue #47: `doclang init ../../evil` escribía "../../evil.doclang" fuera
// del directorio actual — sin guard, el nombre de documento se trataba como
// un fragmento de ruta en vez de un nombre opaco. La garantía de que
// IsOpaquePathToken rechaza correctamente rutas/traversal ya está cubierta
// en core/util/path_test.go; este test verifica el cableado: que
// init.go de verdad la llama y que el rechazo ocurre ANTES de escribir nada.
func TestInit_RejectsPathTraversal(t *testing.T) {
	cases := []string{
		"../evil",
		"../../evil",
		"sub/evil",
		"/etc/evil",
		"a..b",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
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

			cmd := NewInitCommand()
			cmd.SetArgs([]string{name})
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			if err := cmd.Execute(); err == nil {
				t.Errorf("se esperaba error para el nombre %q, no hubo ninguno", name)
			}

			entries, err := os.ReadDir(tmpDir)
			if err != nil {
				t.Fatalf("os.ReadDir failed: %v", err)
			}
			if len(entries) != 0 {
				t.Errorf("no debería haberse escrito ningún archivo, se encontraron %d entradas en %s", len(entries), tmpDir)
			}
		})
	}
}

func TestInit_AcceptsPlainName(t *testing.T) {
	if err := runInit(t, "my-document"); err != nil {
		t.Fatalf("runInit falló para un nombre válido: %v", err)
	}
}

// TestInit_TemplateThenBuild covers issue #100's review finding #1: the
// `technical` template emits `numbering:` as the legacy map form
// (`numbering:\n  enabled: true\n  style: 1.1.1`), which predates
// FrontMatterNode's tri-state *bool. Before the parser accepted that shape,
// `doclang init --template technical && doclang build` failed outright with
// "cannot unmarshal !!map into bool" — this is the end-to-end round-trip
// that regression covers, not just the frontmatter-package unit tests.
//
// `report` also emits the same legacy `numbering:` map shape, but is
// deliberately NOT in this table: it has a separate, pre-existing bug
// (`header:\n  text: %s` interpolates the document name as a bare string,
// but `rawHeaderConfig.Text` expects a `left`/`center`/`right` map, so the
// build fails with "cannot unmarshal !!str `doc` into
// parser.rawHeaderFooterText") that's unrelated to #100/the numbering fix
// and out of scope here. Filed as a follow-up rather than folded into this
// change.
func TestInit_TemplateThenBuild(t *testing.T) {
	for _, tmpl := range []string{"default", "strict", "technical"} {
		t.Run(tmpl, func(t *testing.T) {
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

			initCmd := NewInitCommand()
			initCmd.SetArgs([]string{"doc", "--template", tmpl})
			initCmd.SilenceUsage = true
			initCmd.SilenceErrors = true
			if err := initCmd.Execute(); err != nil {
				t.Fatalf("doclang init --template %s failed: %v", tmpl, err)
			}

			outDir := filepath.Join(tmpDir, "out")
			buildCmd := NewBuildCommand(nil, nil, nil, nil, nil)
			buildCmd.SetArgs([]string{"doc.doclang", "--format", "markdown", "--output", outDir})
			buildCmd.SilenceUsage = true
			buildCmd.SilenceErrors = true
			if err := buildCmd.Execute(); err != nil {
				t.Fatalf("doclang build failed for --template %s output: %v", tmpl, err)
			}
		})
	}
}
