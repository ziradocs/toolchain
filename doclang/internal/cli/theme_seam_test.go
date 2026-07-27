// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

// themeVariablesProbe registra el mapa de variables de tema que recibió vía
// linter.ThemeAware, para verificar que build.go de verdad resuelve el tema
// ANTES de correr el linter (issue #30) — no solo que el tipo compile.
type themeVariablesProbe struct {
	received map[string]string
	calls    int
}

func (p *themeVariablesProbe) Check(node ast.Node) []diagnostics.Diagnostic { return nil }

func (p *themeVariablesProbe) SetThemeVariables(vars map[string]string) {
	p.received = vars
	p.calls++
}

// TestBuild_ThemeVariablesReachLinterBeforeCheck cubre el cableado de
// extremo a extremo: un documento mínimo, `--lint-only` (para no requerir
// Chromium/generación real), y una regla ThemeAware inyectada vía
// NewBuildCommand — debe recibir el mapa de variables del tema
// "professional" (no vacío: ver doclang/themes/document/themes.go) antes
// de que el lint termine.
func TestBuild_ThemeVariablesReachLinterBeforeCheck(t *testing.T) {
	tmpDir := t.TempDir()
	docPath := filepath.Join(tmpDir, "doc.doclang")
	content := "---\ntitle: \"Test Document\"\n---\n\n# Introduction\n\nSome content.\n"
	if err := os.WriteFile(docPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	probe := &themeVariablesProbe{}
	cmd := NewBuildCommand([]linter.Rule{probe}, nil, nil, nil, nil)
	cmd.SetArgs([]string{docPath, "--lint-only", "--theme", "professional"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("build --lint-only failed: %v", err)
	}

	if probe.calls != 1 {
		t.Fatalf("expected SetThemeVariables to be called exactly once, got %d calls", probe.calls)
	}
	if len(probe.received) == 0 {
		t.Fatalf("expected a non-empty theme variables map for the 'professional' theme, got %+v", probe.received)
	}
}
