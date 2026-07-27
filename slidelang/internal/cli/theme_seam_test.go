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
// linter.ThemeAware, para verificar que runBuild de verdad resuelve el tema
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

// TestRunBuild_ThemeVariablesReachLinterBeforeCheck cubre el cableado de
// extremo a extremo en slidelang: --lint-only (para no requerir Chromium ni
// generación real) más una regla ThemeAware inyectada — debe recibir el
// mapa de variables del tema "dark" (embebido, no vacío — ver
// slidelang/internal/generator/css/themes/variables.go) antes de que el
// lint termine, aunque resolveTheme viviera antes enterrado dentro del
// generador y --lint-only nunca lo alcanzara.
func TestRunBuild_ThemeVariablesReachLinterBeforeCheck(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "test.slidelang")
	source := `---
title: Theme Seam Test
mode: flex
---

# Slide 1

Contenido de prueba.
`
	if err := os.WriteFile(inputFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	probe := &themeVariablesProbe{}
	opts := &BuildOptions{
		InputFile: inputFile,
		Mode:      "auto",
		LogLevel:  "error",
		NoColors:  true,
		LintOnly:  true,
		Theme:     "dark",
	}

	if err := runBuild(opts, []linter.Rule{probe}, nil, nil, nil, nil); err != nil {
		t.Fatalf("runBuild() error = %v", err)
	}

	if probe.calls != 1 {
		t.Fatalf("expected SetThemeVariables to be called exactly once, got %d calls", probe.calls)
	}
	if len(probe.received) == 0 {
		t.Fatalf("expected a non-empty theme variables map for the 'dark' theme, got %+v", probe.received)
	}
}
