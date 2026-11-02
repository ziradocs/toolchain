// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"slices"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestChromiumRequirements_SVGImageInDOCX: una imagen SVG local en un DOCX
// pide Chromium (se rasteriza con él), igual que mermaid y math; un PNG no.
// El export del MCP usa esta lista para fallar antes de generar.
func TestChromiumRequirements_SVGImageInDOCX(t *testing.T) {
	doc := func(src string) *ast.AST {
		pos := diagnostics.NewPosition(1, 1)
		d := ast.NewAST(pos)
		b := ast.NewContentBlock(pos, "content")
		b.Elements = append(b.Elements, ast.NewImageElement(pos, src, "x"))
		d.ContentBlocks = append(d.ContentBlocks, *b)
		return d
	}
	if got := ChromiumRequirements(doc("diagrama.svg"), "docx", "", ""); !slices.Contains(got, "SVG images in DOCX") {
		t.Errorf("svg in docx: %v", got)
	}
	if got := ChromiumRequirements(doc("foto.png"), "docx", "", ""); len(got) != 0 {
		t.Errorf("png in docx: %v", got)
	}
	if got := ChromiumRequirements(doc("diagrama.svg"), "html", "browser", ""); len(got) != 0 {
		t.Errorf("svg in browser html: %v", got)
	}
}
