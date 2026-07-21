// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
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
