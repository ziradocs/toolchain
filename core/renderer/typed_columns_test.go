// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Issue #373: una columna tipada (Content vacío, Elements con el cuerpo) se
// dibuja con sus elementos anidados, en orden, y una columna cruda sale igual
// que antes.
func TestRenderGridElement_TypedColumnDrawsNestedElements(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	grid := ast.NewGridElement(pos)

	points := ast.NewPointsElement(pos)
	points.Items = []ast.PointItem{*ast.NewPointItem(pos, "first"), *ast.NewPointItem(pos, "second")}
	typed := ast.NewColumnElement(pos, "")
	typed.Elements = []ast.Element{ast.NewTextElement(pos, "Left **side**."), points}
	raw := ast.NewColumnElement(pos, "Right side.")
	grid.Columns = []ast.ColumnElement{*typed, *raw}

	html := RenderElementToHTML(grid, nil, NewDefaultRenderContext())

	if got := strings.Count(html, `<div class="grid-column">`); got != 2 {
		t.Fatalf("grid-column count = %d, want 2\n%s", got, html)
	}
	for _, want := range []string{"<strong>side</strong>", "first", "second", "Right side."} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q\n%s", want, html)
		}
	}
	// Orden: el texto va antes que los puntos dentro de la columna tipada, y
	// la columna cruda viene después de las dos.
	if strings.Index(html, "side</strong>") > strings.Index(html, "first") ||
		strings.Index(html, "second") > strings.Index(html, "Right side.") {
		t.Fatalf("nested elements are out of order\n%s", html)
	}
}

func TestRenderGridElement_RawColumnUnchanged(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	grid := ast.NewGridElement(pos)
	grid.Columns = []ast.ColumnElement{*ast.NewColumnElement(pos, "Only raw.")}

	html := RenderElementToHTML(grid, nil, NewDefaultRenderContext())
	if !strings.HasPrefix(html, `<div class="grid" data-columns="1"><div class="grid-column">`) ||
		!strings.Contains(html, "Only raw.") || !strings.HasSuffix(html, "</div></div>") {
		t.Fatalf("raw column html changed:\n%s", html)
	}
}
