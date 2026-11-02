// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Un párrafo hecho solo de imágenes inline sale en el PPTX como imágenes; uno
// con texto sigue siendo texto.
func TestPptxImageOnlyParagraph(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	got := pptxImageOnlyParagraph(ast.NewTextElement(pos, "![a](a.png){width=30%} ![b](img/b.png)"))
	if len(got) != 2 || got[0].Source != "a.png" || got[0].Alt != "a" || got[1].Source != "img/b.png" {
		t.Fatalf("got %#v", got)
	}
	for _, text := range []string{"Texto ![a](a.png)", "![a](a.png) fin", "![a](javascript:x)", "sin imágenes"} {
		if got := pptxImageOnlyParagraph(ast.NewTextElement(pos, text)); got != nil {
			t.Errorf("%q: %#v", text, got)
		}
	}
}
