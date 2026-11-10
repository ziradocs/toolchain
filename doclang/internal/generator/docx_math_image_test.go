// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// mathWithImage arma una fórmula con render previo. El campo se fija por
// reflexión porque este módulo compila también contra el core publicado,
// que puede no tener MathElement.Image; con ese core la prueba se salta.
func mathWithImage(t *testing.T, latex, label string, number int, image string) *ast.MathElement {
	t.Helper()
	m := ast.NewMathElement(diagnostics.NewPosition(1, 1), latex)
	m.Label, m.Number = label, number
	field := reflect.ValueOf(m).Elem().FieldByName("Image")
	if !field.IsValid() {
		t.Skip("el core enlazado no tiene math-image-v1")
	}
	field.SetString(image)
	return m
}

func TestMathImageChromiumGates(t *testing.T) {
	plain := ast.NewMathElement(diagnostics.NewPosition(1, 1), "x")
	rendered := mathWithImage(t, "x", "", 0, "eq.png")
	svg := mathWithImage(t, "x", "", 0, "eq.svg")
	if !MathNeedsChromiumDOCX(plain) || MathNeedsChromiumDOCX(rendered) || !MathNeedsChromiumDOCX(svg) {
		t.Fatal("MathNeedsChromiumDOCX: sin render y SVG sí, PNG no")
	}
	if HasUnrenderedMath(astWithElements(rendered)) || !HasUnrenderedMath(astWithElements(rendered, plain)) {
		t.Fatal("HasUnrenderedMath debe contar solo las fórmulas sin render")
	}
	gen := NewDOCXGenerator(newTestLogger(), t.TempDir())
	if gen.needsChromiumRendering(astWithElements(rendered)) || !gen.canRenderWithoutChromium(astWithElements(rendered)) {
		t.Fatal("una fórmula con render PNG no necesita Chromium en DOCX")
	}
	if !gen.needsChromiumRendering(astWithElements(svg)) || gen.canRenderWithoutChromium(astWithElements(svg)) {
		t.Fatal("una fórmula con render SVG se rasteriza con Chromium")
	}
}

// El render previo entra al DOCX sin Chromium y la fórmula conserva su
// número; uno que falta o es remoto deja el marcador de ecuación, sin
// descargar nada.
func TestDOCX_MathImage(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 96, 24))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "eq.png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := astWithElements(
		mathWithImage(t, "E = mc^2", "eq:a", 2, "eq.png"),
		mathWithImage(t, "a = b", "eq:b", 5, "falta.png"),
		mathWithImage(t, "c = d", "", 0, "https://example.com/eq.png"),
	)
	logger := newTestLogger()
	gen := NewDOCXGenerator(logger, root)
	out := filepath.Join(t.TempDir(), "out.docx")
	if err := gen.Generate(doc, out, GeneratorOptions{Format: "docx", ChromiumPath: filepath.Join(t.TempDir(), "no-chromium")}); err != nil {
		t.Fatal(err)
	}
	if gen.chromiumRenderer != nil {
		t.Fatal("se inició Chromium para fórmulas con render previo")
	}
	xml := docxDocumentXML(t, out)
	text := docxVisibleText(xml)
	if !strings.Contains(text, "(2)") {
		t.Errorf("falta el número de la ecuación: %q", text)
	}
	// La que falta conserva su número debajo del marcador, como en HTML.
	if !strings.Contains(text, "[Ecuación no disponible](5)") {
		t.Errorf("el marcador perdió el número: %q", text)
	}
	// La imagen va pegada a su número: el único párrafo con keepNext.
	if n := strings.Count(xml, "<w:keepNext"); n != 1 {
		t.Errorf("keepNext = %d, want 1", n)
	}
	if n := strings.Count(text, "[Ecuación no disponible]"); n != 2 {
		t.Errorf("marcadores = %d, want 2 (falta y remota): %q", n, text)
	}
	if n := strings.Count(xml, "<a:blip "); n != 1 {
		t.Errorf("imágenes insertadas = %d, want 1", n)
	}
	warns := strings.Join(logger.warns, "\n")
	if !strings.Contains(warns, "falta.png") || !strings.Contains(warns, "never fetches remote images") {
		t.Errorf("avisos = %s", warns)
	}
}
