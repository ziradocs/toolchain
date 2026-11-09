// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func inNote(elems ...ast.Element) *ast.SpecialBlockElement {
	note := ast.NewSpecialBlockElement(diagnostics.NewPosition(1, 1), "note", "")
	note.Elements = elems
	return note
}

// Una fórmula dentro de un bloque especial pide Chromium en HTML offline:
// antes la compuerta miraba solo el primer nivel y la fórmula salía como
// "Ecuación no disponible".
func TestHTMLOffline_NestedMathStartsChromium(t *testing.T) {
	doc := astWithElements(inNote(ast.NewMathElement(diagnostics.NewPosition(1, 1), "x")))
	if !HasUnrenderedMath(doc) {
		t.Fatal("HasUnrenderedMath no vio la fórmula anidada")
	}
	for _, mode := range []string{"offline-inline", "offline-assets"} {
		out := filepath.Join(t.TempDir(), "out.html")
		err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{
			Format: "html", MathMode: mode, ImageMode: mode,
			ChromiumPath: filepath.Join(t.TempDir(), "no-chromium"),
		})
		// Con un Chromium inexistente, pedirlo es fallar al iniciarlo.
		if err == nil || !strings.Contains(err.Error(), "Chromium") {
			t.Errorf("%s: no se pidió Chromium para la fórmula anidada: %v", mode, err)
		}
	}
}

// Una fórmula con render previo arriba no apaga Chromium para otra anidada
// sin render.
func TestHasUnrenderedMath_MixedNested(t *testing.T) {
	rendered := mathWithImage(t, "E = mc^2", "eq:a", 1, "eq.png")
	if HasUnrenderedMath(astWithElements(rendered, inNote(mathWithImage(t, "y", "", 0, "y.png")))) {
		t.Fatal("todas tienen render: no hace falta Chromium")
	}
	if !HasUnrenderedMath(astWithElements(rendered, inNote(ast.NewMathElement(diagnostics.NewPosition(1, 1), "x")))) {
		t.Fatal("la anidada sin render no pidió Chromium")
	}
}
