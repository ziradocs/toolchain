// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer/chromium"
)

func TestSVGNaturalSizePx(t *testing.T) {
	for _, tc := range []struct {
		name string
		svg  string
		w, h float64
	}{
		{"width y height en px", `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="150"/>`, 300, 150},
		{"con unidad px", `<svg width="300px" height="150px"/>`, 300, 150},
		{"en pulgadas", `<svg width="2in" height="1in"/>`, 192, 96},
		{"en pt", `<svg width="72pt" height="36pt"/>`, 96, 48},
		{"en mm", `<svg width="25.4mm" height="12.7mm"/>`, 96, 48},
		{"en cm", `<svg width="2.54cm" height="1.27cm"/>`, 96, 48},
		{"solo viewBox", `<svg viewBox="0 0 400 100"/>`, 400, 100},
		{"viewBox con comas", `<svg viewBox="0,0,400,100"/>`, 400, 100},
		{"ancho y viewBox: alto por proporción", `<svg width="200" viewBox="0 0 400 100"/>`, 200, 50},
		{"alto y viewBox: ancho por proporción", `<svg height="50" viewBox="0 0 400 100"/>`, 200, 50},
		{"porcentaje cuenta como ausente", `<svg width="100%" height="100%" viewBox="0 0 640 480"/>`, 640, 480},
		{"sin nada: 300x150", `<svg xmlns="http://www.w3.org/2000/svg"/>`, 300, 150},
		{"con prólogo XML y DOCTYPE", `<?xml version="1.0" encoding="ISO-8859-1"?><!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"><!-- c --><svg width="10" height="20"/>`, 10, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, h, err := svgNaturalSizePx([]byte(tc.svg))
			if err != nil {
				t.Fatal(err)
			}
			if !near(w, tc.w) || !near(h, tc.h) {
				t.Errorf("got %.2fx%.2f, want %.2fx%.2f", w, h, tc.w, tc.h)
			}
		})
	}
	if _, _, err := svgNaturalSizePx([]byte(`<html><svg width="1" height="1"/></html>`)); err == nil {
		t.Error("a document whose root is not <svg> must fail")
	}
	if _, _, err := svgNaturalSizePx([]byte("\x89PNG\r\n")); err == nil {
		t.Error("binary data must fail")
	}
}

func TestIsSVGImageSource(t *testing.T) {
	for src, want := range map[string]bool{
		"diagrama.svg":           true,
		"img/Diagrama.SVG":       true,
		"diagrama.svg?v=2#frag":  true,
		"diagrama.png":           false,
		"https://x.test/a.svg":   false,
		"data:image/svg+xml,abc": false,
	} {
		if got := isSVGImageSource(src); got != want {
			t.Errorf("isSVGImageSource(%q) = %v, want %v", src, got, want)
		}
	}
}

// TestDOCXGenerator_NeedsChromiumRendering_SVGImage: un documento cuya única
// imagen rica es un SVG arranca Chromium; sin este caso el SVG se quedaría
// sin rasterizador aunque hubiera Chromium instalado.
func TestDOCXGenerator_NeedsChromiumRendering_SVGImage(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	gen := NewDOCXGenerator(newTestLogger(), t.TempDir())
	svgDoc := astWithElements(ast.NewImageElement(pos, "diagrama.svg", "Diagrama"))
	if !gen.needsChromiumRendering(svgDoc) || gen.canRenderWithoutChromium(svgDoc) {
		t.Error("an SVG image must ask for Chromium")
	}
	if !HasSVGImages(svgDoc) {
		t.Error("HasSVGImages missed the SVG image")
	}
	pngDoc := astWithElements(ast.NewImageElement(pos, "foto.png", "Foto"))
	if gen.needsChromiumRendering(pngDoc) || HasSVGImages(pngDoc) {
		t.Error("a PNG image must not ask for Chromium")
	}
}

const testSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="150" viewBox="0 0 300 150"><rect width="300" height="150" fill="#4a90d9"/></svg>`

func writeSVGDoc(t *testing.T) (string, *ast.AST) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "diagrama.svg"), []byte(testSVG), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, astWithElements(ast.NewImageElement(diagnostics.NewPosition(9, 1), "diagrama.svg", "Diagrama"))
}

// TestDOCXGenerator_SVGWithoutChromium_WarnsAndLeavesPlaceholder: sin
// Chromium el SVG no desaparece en silencio: queda el marcador de siempre y
// un aviso que dice qué hacer.
func TestDOCXGenerator_SVGWithoutChromium_WarnsAndLeavesPlaceholder(t *testing.T) {
	dir, doc := writeSVGDoc(t)
	logger := newTestLogger()
	gen := NewDOCXGenerator(logger, dir)
	output := filepath.Join(t.TempDir(), "out.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx", AssetRoot: dir, ChromiumPath: filepath.Join(t.TempDir(), "no-chromium")}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if gen.chromiumRenderer != nil {
		t.Skip("a Chromium was found anyway; this test needs it missing")
	}
	if xml := docxDocumentXML(t, output); !strings.Contains(xml, "[Imagen no disponible: Diagrama]") {
		t.Errorf("missing placeholder in document.xml:\n%s", xml)
	}
	found := false
	for _, w := range logger.warns {
		if strings.Contains(w, "line 9") && strings.Contains(w, "--chromium-path") {
			found = true
		}
	}
	if !found {
		t.Errorf("missing actionable warning; warns = %v", logger.warns)
	}
}

// TestDOCXGenerator_SVGImage_RasterizedAtNaturalSize (Chromium real): el SVG
// entra como PNG al tamaño que declara (300x150 px CSS = 3.125"x1.5625"),
// rasterizado a 2 px por px CSS. Antes se perdía con "unknown format".
func TestDOCXGenerator_SVGImage_RasterizedAtNaturalSize(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Chromium")
	}
	if _, ok := any((*chromium.ChromiumRenderer)(nil)).(svgImageRasterizer); !ok {
		t.Skip("this core has no RenderSVGImageToPNG")
	}
	dir, doc := writeSVGDoc(t)
	logger := newTestLogger()
	output := filepath.Join(t.TempDir(), "out.docx")
	if err := New(logger).Generate(doc, output, GeneratorOptions{Format: "docx", AssetRoot: dir}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	for _, w := range logger.warns {
		if strings.Contains(w, "Chromium") && strings.Contains(w, "not available") || strings.Contains(w, "Failed to initialize Chromium") {
			t.Skipf("no Chromium: %s", w)
		}
		if strings.Contains(w, "diagrama.svg") {
			t.Fatalf("unexpected warning: %s", w)
		}
	}
	xml := docxDocumentXML(t, output)
	m := regexp.MustCompile(`<wp:extent cx="(\d+)" cy="(\d+)"`).FindStringSubmatch(xml)
	if m == nil {
		t.Fatalf("no inline image in document.xml:\n%s", xml)
	}
	cx, _ := strconv.Atoi(m[1])
	cy, _ := strconv.Atoi(m[2])
	if !near(float64(cx)/914400, 3.125) || !near(float64(cy)/914400, 1.5625) {
		t.Errorf("extent = %.3fx%.3f in, want 3.125x1.5625", float64(cx)/914400, float64(cy)/914400)
	}

	r, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	var media []string
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "word/media/") {
			media = append(media, f.Name)
		}
	}
	if len(media) != 1 || !strings.HasSuffix(media[0], ".png") {
		t.Errorf("media = %v, want one PNG", media)
	}
}

// TestHasSVGImages_InlineAndNested: un SVG inline (párrafo, ítem, celda) o
// anidado en un callout también pide Chromium; antes solo contaban las
// imágenes de bloque del nivel de sección.
func TestHasSVGImages_InlineAndNested(t *testing.T) {
	for name, body := range map[string]string{
		"párrafo": "Texto ![e](evil.svg) aquí.\n",
		"ítem":    "- uno ![e](img/a.SVG?x=1)\n",
		"celda":   "| A |\n|---|\n| ![e](<d s.svg>) |\n",
		"callout": "::: info\n![e](nested.svg)\n:::\n",
	} {
		doc := parseInline(t, "---\ntitle: S\nmode: flex\n---\n\n# S\n\n"+body)
		if !HasSVGImages(doc) {
			t.Errorf("%s: SVG not detected", name)
		}
		if gen := NewDOCXGenerator(newTestLogger(), ""); !gen.needsChromiumRendering(doc) || gen.canRenderWithoutChromium(doc) {
			t.Errorf("%s: must ask for Chromium", name)
		}
	}
	for name, body := range map[string]string{
		"png inline": "Texto ![e](a.png) y [svg](link.svg).\n",
		"svg remoto": "![e](https://x.test/a.svg)\n",
	} {
		if HasSVGImages(parseInline(t, "---\ntitle: S\nmode: flex\n---\n\n# S\n\n"+body)) {
			t.Errorf("%s: false positive", name)
		}
	}
}

// TestSVGNaturalSizePx_NonFinite: NaN, Inf o un número que desborda no llegan
// a la captura ni a los EMU: cuentan como ausentes.
func TestSVGNaturalSizePx_NonFinite(t *testing.T) {
	for _, svg := range []string{
		`<svg width="NaN" height="Inf"/>`,
		`<svg width="1e999" height="10" viewBox="0 0 NaN 5"/>`,
		`<svg viewBox="0 0 Inf 100"/>`,
		`<svg width="1e308in" height="1e308in"/>`,
		`<svg width="1e300" viewBox="0 0 1e-300 1"/>`,
	} {
		w, h, err := svgNaturalSizePx([]byte(svg))
		if err != nil {
			t.Fatal(err)
		}
		if !svgFinitePositive(w) || !svgFinitePositive(h) {
			t.Errorf("%s: got %vx%v", svg, w, h)
		}
	}
}
