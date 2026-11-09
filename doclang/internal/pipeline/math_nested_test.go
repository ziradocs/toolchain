// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// El chequeo previo del export ve una fórmula anidada en un bloque especial,
// igual que el generador.
func TestChromiumRequirements_NestedMathInOfflineHTML(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	note := ast.NewSpecialBlockElement(pos, "note", "")
	note.Elements = []ast.Element{ast.NewMathElement(pos, "x")}
	block.Elements = []ast.Element{note}
	doc.ContentBlocks = []ast.ContentBlock{*block}
	for _, mode := range []string{"offline-inline", "offline-assets"} {
		if reasons := ChromiumRequirements(doc, "html", mode, "chromium"); len(reasons) == 0 {
			t.Errorf("%s: no pidió Chromium para la fórmula anidada", mode)
		}
	}
}
