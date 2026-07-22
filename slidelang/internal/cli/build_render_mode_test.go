// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const renderModeFixture = `---
title: Test Render Mode
mode: flex
---

# Slide 1

Contenido.
`

// TestRunBuild_InvalidRenderMode rechaza un --render-mode desconocido antes de
// escribir nada al disco (issue #92).
func TestRunBuild_InvalidRenderMode(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "test.slidelang")
	if err := os.WriteFile(inputFile, []byte(renderModeFixture), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	outDir := filepath.Join(tmpDir, "dist")
	opts := &BuildOptions{
		InputFile:  inputFile,
		OutputDir:  outDir,
		Format:     "html",
		Mode:       "auto",
		LogLevel:   "error",
		NoColors:   true,
		RenderMode: "bogus",
	}

	err := runBuild(opts, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for an invalid render-mode, got nil")
	}
	if !strings.Contains(err.Error(), "invalid render-mode") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "invalid render-mode")
	}
	// No debe haber escrito nada al output dir.
	if entries, _ := os.ReadDir(outDir); len(entries) > 0 {
		t.Errorf("expected no output on invalid render-mode, found %d entries", len(entries))
	}
}

// TestRunBuild_EmptyRenderModeTreatedAsBrowser: un BuildOptions programático (no
// vía cobra) deja RenderMode vacío; debe construir como browser, no rechazarse.
func TestRunBuild_EmptyRenderModeTreatedAsBrowser(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "test.slidelang")
	if err := os.WriteFile(inputFile, []byte(renderModeFixture), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	opts := &BuildOptions{
		InputFile: inputFile,
		OutputDir: filepath.Join(tmpDir, "dist"),
		Format:    "html",
		Mode:      "auto",
		LogLevel:  "error",
		NoColors:  true,
		// RenderMode vacío a propósito.
	}

	if err := runBuild(opts, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("empty render-mode should build like browser, got error: %v", err)
	}
}
