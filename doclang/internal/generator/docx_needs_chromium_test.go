// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestDOCXGenerator_NeedsChromiumRendering_MathOnly covers a bug found while
// closing issue #51/#38: needsChromiumRendering listed ChartElement,
// MapElement and MermaidElement, but NOT MathElement — even though
// renderMath (docx.go) also drives chromiumRenderer.RenderMathToPNG. A
// document whose only rich element is an equation never got Chromium
// initialized, so renderMath hit its nil guard and the equation silently
// vanished from the DOCX (only "nominally" supported). The regression
// condition is specifically a MathElement ALONE — with a chart or mermaid
// alongside it, Chromium already gets initialized for the other element and
// the gap stays masked.
func TestDOCXGenerator_NeedsChromiumRendering_MathOnly(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := astWithElements(ast.NewMathElement(pos, "E = mc^2"))

	gen := NewDOCXGenerator(newTestLogger(), t.TempDir())

	if !gen.needsChromiumRendering(doc) {
		t.Fatal("expected needsChromiumRendering to return true for a document whose only rich element is a MathElement")
	}
}
