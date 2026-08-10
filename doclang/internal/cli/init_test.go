// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
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
// `report` is included too (issue #115): it emitted the same legacy
// `numbering:` map form plus its own separate bug (`header:\n  text: %s`
// interpolated the document name as a bare string, but
// `rawHeaderConfig.Text` only accepted a `left`/`center`/`right` map), which
// is now fixed both in the template (emits the map shape) and in the parser
// (accepts the scalar shorthand as `center`) — see
// TestInit_ReportTemplateFrontMatterIsLive below for the assertion that the
// value actually lands where it should, not just "the build doesn't error".
func TestInit_TemplateThenBuild(t *testing.T) {
	for _, tmpl := range []string{"default", "strict", "technical", "report"} {
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

// TestInit_ReportTemplateFrontMatterIsLive covers what
// TestInit_TemplateThenBuild's "the build doesn't error" assertion can't:
// that `report`'s header/footer front matter actually parses into the
// fields it's supposed to, not just that no key with a typo happens to be
// silently dropped by yaml's non-strict Unmarshal (which is exactly how
// `footer:\n  page-numbers: true` went unnoticed before issue #115— a
// wrong key was dropped rather than rejected, and the round-trip test
// still built cleanly).
func TestInit_ReportTemplateFrontMatterIsLive(t *testing.T) {
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

	const docName = "doc"

	initCmd := NewInitCommand()
	initCmd.SetArgs([]string{docName, "--template", "report"})
	initCmd.SilenceUsage = true
	initCmd.SilenceErrors = true
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("doclang init --template report failed: %v", err)
	}

	content, err := os.ReadFile(docName + ".doclang")
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	log := util.NewConsoleLogger(util.LevelInfo, false)
	doc, diags := parser.New(log).ParseDocument(string(content), docName+".doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if doc == nil || doc.FrontMatter == nil {
		t.Fatal("ParseDocument returned no front matter")
	}

	hf := doc.FrontMatter.HeaderFooter
	if hf == nil || hf.Header == nil || hf.Header.Text == nil {
		t.Fatal("FrontMatter.HeaderFooter.Header.Text should not be nil")
	}
	if hf.Header.Text.Center != docName {
		t.Errorf("Header.Text.Center = %q, want %q", hf.Header.Text.Center, docName)
	}

	if hf.Footer == nil || hf.Footer.PageNumbers == nil {
		t.Fatal("FrontMatter.HeaderFooter.Footer.PageNumbers should not be nil")
	}
	if !hf.Footer.PageNumbers.Enabled {
		t.Error("Footer.PageNumbers.Enabled = false, want true")
	}
}
