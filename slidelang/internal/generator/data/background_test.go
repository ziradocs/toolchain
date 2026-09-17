// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestSlideBackground_AcceptsLocalImageReference(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	background := ast.NewDirectiveNode(pos, "background")
	background.Parameters["image"] = "assets/cover image.png"
	block := ast.NewContentBlock(pos, "content")
	block.Elements = []ast.Element{background}

	if got, want := string(slideBackground(*block)), `url("assets/cover image.png")`; got != want {
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

			if got := slideBackground(*block); got != "" {
				t.Errorf("slideBackground() = %q, se esperaba vacío para una referencia insegura", got)
			}
		})
	}
}
