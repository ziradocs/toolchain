// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
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
		"falta el archivo":                   {"offline-inline", "no-existe.png"},
		"fuera de la raíz":                   {"offline-inline", "../fuera.png"},
		"fuera de la raíz en offline-assets": {"offline-assets", "../fuera.png"},
		"protocolo peligroso":                {"browser", "javascript:alert(1)"},
		"remota en PDF/offline":              {"offline-inline", "https://example.com/eq.png"},
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

// pngWithPHYs codifica un PNG de w×h y, con dpi > 0, le agrega un chunk pHYs
// (píxeles por metro) después del IHDR.
func pngWithPHYs(t *testing.T, w, h int, dpi float64) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if dpi <= 0 {
		return data
	}
	ppm := uint32(dpi/0.0254 + 0.5)
	body := binary.BigEndian.AppendUint32(nil, ppm)
	body = binary.BigEndian.AppendUint32(body, ppm)
	body = append(body, 1)
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	chunk = append(chunk, "pHYs"...)
	chunk = append(chunk, body...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	ihdrEnd := 8 + 8 + 13 + 4
	out := append([]byte{}, data[:ihdrEnd]...)
	out = append(out, chunk...)
	return append(out, data[ihdrEnd:]...)
}

// Un render previo PNG con pHYs va a su tamaño físico en HTML/PDF (Chromium
// ignora pHYs), el mismo que le da el DOCX; sin pHYs queda como antes. Las
// imágenes de bloque normales no cambian.
func TestRenderMathElement_ImageSizedByPHYs(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"narrow-192.png": pngWithPHYs(t, 400, 80, 192),
		"wide-192.png":   pngWithPHYs(t, 1280, 200, 192),
		"narrow.png":     pngWithPHYs(t, 400, 80, 0),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"browser", "offline-assets", "offline-inline"} {
		for image, want := range map[string]string{
			"narrow-192.png":             ` style="width:200px;max-width:100%">`,
			"wide-192.png":               ` style="width:640px;max-width:100%">`, // topa con la columna por max-width
			"narrow.png":                 `class="math-diagram math-image">`,
			"https://example.com/eq.png": `class="math-diagram math-image">`,
		} {
			if mode != "browser" && strings.HasPrefix(image, "https:") {
				continue // remota bloqueada en salidas de build
			}
			ctx := NewDefaultRenderContext()
			ctx.MathMode, ctx.ImageMode, ctx.AssetRoot = mode, mode, dir
			out := renderMathElement(labeledMath(image), nil, ctx)
			if !strings.Contains(out, want) {
				t.Errorf("%s %s: falta %q en %s", mode, image, want, out)
			}
		}
		ctx := NewDefaultRenderContext()
		ctx.ImageMode, ctx.AssetRoot = mode, dir
		img := ast.NewImageElement(diagnostics.NewPosition(1, 1), "narrow-192.png", "foto")
		if out := renderImageElement(img, nil, ctx); strings.Contains(out, "style=") {
			t.Errorf("%s: una imagen de bloque cambió de tamaño: %s", mode, out)
		}
	}
}

// TestMathImageCSSWidth_QueryString: "eq.png?v=2" es el archivo eq.png, como
// en el navegador: el ancho por pHYs sale igual. Antes se buscaba el
// archivo "eq.png?v=2" y la fórmula quedaba sin ancho.
func TestMathImageCSSWidth_QueryString(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "eq.png"), pngWithPHYs(t, 400, 80, 192), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := NewDefaultRenderContext()
	ctx.AssetRoot = dir
	for _, src := range []string{"eq.png", "eq.png?v=2", "eq.png#frag"} {
		if w, ok := mathImageCSSWidth(src, ctx); !ok || w < 199.9 || w > 200.1 {
			t.Errorf("%s: width %v ok %v, want 200", src, w, ok)
		}
	}
}
