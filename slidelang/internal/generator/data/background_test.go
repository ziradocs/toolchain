// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

func TestSlideBackground_AcceptsLocalImageReference(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	background := ast.NewDirectiveNode(pos, "background")
	background.Parameters["image"] = "assets/cover image.png"
	block := ast.NewContentBlock(pos, "content")
	block.Elements = []ast.Element{background}

	if got, want := string(slideBackground(*block, nil, nil)), `url("assets/cover image.png")`; got != want {
		t.Errorf("slideBackground() = %q, se esperaba %q", got, want)
	}
}

func TestSlideBackground_RejectsCSSInjection(t *testing.T) {
	for _, value := range []string{
		`image.png"); color: red; /*`, // referencia de imagen
		`rgb(0, 0, 0); color: red`,    // color que antes pasaba por el prefijo rgb(
	} {
		t.Run(value, func(t *testing.T) {
			pos := diagnostics.NewPosition(1, 1)
			background := ast.NewDirectiveNode(pos, "background")
			background.Parameters["value"] = value
			block := ast.NewContentBlock(pos, "content")
			block.Elements = []ast.Element{background}

			if got := slideBackground(*block, nil, nil); got != "" {
				t.Errorf("slideBackground() = %q, se esperaba vacío para una referencia insegura", got)
			}
		})
	}
}

func TestSlideBackground_AcceptsDataImage(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	background := ast.NewDirectiveNode(pos, "background")
	background.Parameters["image"] = "data:image/png;base64,AA=="
	block := ast.NewContentBlock(pos, "content")
	block.Elements = []ast.Element{background}

	if got, want := string(slideBackground(*block, nil, nil)), `url("data:image/png;base64,AA==")`; got != want {
		t.Errorf("slideBackground() = %q, se esperaba %q", got, want)
	}
}

func TestSlideBackground_InlinesLocalImageOffline(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "background.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pos := diagnostics.NewPosition(1, 1)
	background := ast.NewDirectiveNode(pos, "background")
	background.Parameters["image"] = "background.png"
	block := ast.NewContentBlock(pos, "content")
	block.Elements = []ast.Element{background}

	got := string(slideBackground(*block, nil, &renderer.RenderContext{ImageMode: "offline-inline", AssetRoot: dir}))
	if !strings.HasPrefix(got, `url("data:image/png;base64,`) {
		t.Errorf("slideBackground() no inlineó el fondo local: %q", got)
	}
}
