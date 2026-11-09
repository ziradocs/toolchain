// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func labeledMath(image string) *ast.MathElement {
	m := ast.NewMathElement(diagnostics.NewPosition(1, 1), "E = mc^2")
	m.Label, m.Number, m.Caption, m.Image = "eq:a", 3, "Energía", image
	return m
}

func TestRenderMathElement_Image(t *testing.T) {
	dir := t.TempDir()
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if err := os.WriteFile(filepath.Join(dir, "eq-a.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"browser", "offline-assets", "offline-inline"} {
		ctx := NewDefaultRenderContext()
		ctx.MathMode, ctx.ImageMode, ctx.AssetRoot = mode, mode, dir
		ctx.MathFetcher = nil // sin Chromium: en offline, componer el LaTeX daría math-error
		out := renderMathElement(labeledMath("eq-a.png"), nil, ctx)
		for _, want := range []string{`<div class="math-block" id="eq-a">`, `<span class="math-number">(3)</span>`, `<div class="math-caption">Energía</div>`, `alt="E = mc^2"`, `class="math-diagram math-image"`} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: falta %q en %s", mode, want, out)
			}
		}
		wantSrc := `src="eq-a.png"`
		if mode == "offline-inline" {
			wantSrc = `src="data:image/png;base64,`
		}
		if !strings.Contains(out, wantSrc) || strings.Contains(out, "math-error") || strings.Contains(out, "math-content") {
			t.Errorf("%s: src o marcador inesperado: %s", mode, out)
		}
	}

	// Una fuente que no se puede mostrar deja el marcador de ecuación, con
	// el número y el ancla.
	for name, tc := range map[string]struct{ mode, image string }{
		"falta el archivo":      {"offline-inline", "no-existe.png"},
		"fuera de la raíz":      {"offline-inline", "../fuera.png"},
		"protocolo peligroso":   {"browser", "javascript:alert(1)"},
		"remota en PDF/offline": {"offline-inline", "https://example.com/eq.png"},
	} {
		ctx := NewDefaultRenderContext()
		ctx.MathMode, ctx.ImageMode, ctx.AssetRoot = tc.mode, tc.mode, dir
		out := renderMathElement(labeledMath(tc.image), nil, ctx)
		if !strings.Contains(out, "math-error") || !strings.Contains(out, `id="eq-a"`) || !strings.Contains(out, "(3)") || strings.Contains(out, "<img") {
			t.Errorf("%s: %s", name, out)
		}
	}
}

// Una fórmula dentro de un bloque especial también cuenta: el renderer la
// compone con el mismo MathFetcher.
func TestHasMathElements_Nested(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	note := ast.NewSpecialBlockElement(pos, "note", "")
	note.Elements = []ast.Element{ast.NewMathElement(pos, "x")}
	block.Elements = []ast.Element{note}
	doc.ContentBlocks = []ast.ContentBlock{*block}
	if !HasMathElements(doc) {
		t.Fatal("la fórmula anidada no se detectó")
	}
}
