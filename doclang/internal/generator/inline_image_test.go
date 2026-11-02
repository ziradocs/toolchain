// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
)

// inlineImageSource tiene una imagen inline en un párrafo, en un ítem de
// lista y en una celda de tabla.
func inlineImageSource(lang, a, b, c string) string {
	return "---\ntitle: Inline\nlang: " + lang + "\nmode: flex\n---\n\n# Inline\n\n" +
		"Texto con ![uno](" + a + ") en medio.\n\n" +
		"- ítem con ![dos](" + b + ") al final\n\n" +
		"| A | B |\n|---|---|\n| ![tres](" + c + ") | x |\n"
}

func parseInline(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, diags := parser.New(newTestLogger()).ParseDocument(src, "inline.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse: %v", d)
		}
	}
	return doc
}

func writePNGs(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), pngOfSize(t, 96, 48), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestDOCX_InlineImagesInserted: una imagen inline de un párrafo, un ítem o
// una celda entra al DOCX a su tamaño natural (96x48 px = 1"x0.5"). Antes
// salía solo su alt como texto.
func TestDOCX_InlineImagesInserted(t *testing.T) {
	dir := t.TempDir()
	writePNGs(t, dir, "a.png", "b.png", "c.png")
	doc := parseInline(t, inlineImageSource("es", "a.png", "b.png", "c.png"))
	out := filepath.Join(t.TempDir(), "out.docx")
	logger := newTestLogger()
	if err := New(logger).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: dir, SourceDir: dir}); err != nil {
		t.Fatal(err)
	}
	if n := len(docxMediaFiles(t, out)); n != 3 {
		t.Errorf("word/media has %d images, want 3; warns = %v", n, logger.warns)
	}
	xml := docxDocumentXML(t, out)
	extents := regexp.MustCompile(`<wp:extent cx="(\d+)" cy="(\d+)"`).FindAllStringSubmatch(xml, -1)
	if len(extents) != 3 {
		t.Fatalf("want 3 inline drawings, got %d", len(extents))
	}
	for _, e := range extents {
		if e[1] != "914400" || e[2] != "457200" {
			t.Errorf("extent %sx%s EMU, want 914400x457200 (natural size)", e[1], e[2])
		}
	}
	text := docxVisibleText(xml)
	for _, want := range []string{"Texto con", "en medio.", "ítem con", "al final"} {
		if !strings.Contains(text, want) {
			t.Errorf("surrounding text %q lost: %q", want, text)
		}
	}
	for _, alt := range []string{"uno", "dos", "tres"} {
		if strings.Contains(text, alt) {
			t.Errorf("alt %q still printed as text: %q", alt, text)
		}
	}
}

// TestDOCX_InlineImagesUnavailable: una inline que falta o escapa de la raíz
// deja el marcador localizado con su alt, en los tres lugares.
func TestDOCX_InlineImagesUnavailable(t *testing.T) {
	for lang, note := range map[string]string{"es-MX": "Imagen no disponible", "en": "Image not available"} {
		dir := t.TempDir()
		doc := parseInline(t, inlineImageSource(lang, "falta.png", "../../fuera.png", "falta.png"))
		out := filepath.Join(t.TempDir(), "out.docx")
		if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: dir, SourceDir: dir}); err != nil {
			t.Fatal(err)
		}
		text := docxVisibleText(docxDocumentXML(t, out))
		for _, alt := range []string{"uno", "dos", "tres"} {
			if want := "[" + note + ": " + alt + "]"; !strings.Contains(text, want) {
				t.Errorf("lang %s: missing %q in %q", lang, want, text)
			}
		}
		if n := len(docxMediaFiles(t, out)); n != 0 {
			t.Errorf("lang %s: %d media files, want 0", lang, n)
		}
	}
}

// coreResolvesInlineImages dice si el core con el que se compila ya resuelve
// las imágenes inline en offline-inline (core/renderer resolveInlineImages).
// Contra un core publicado que todavía no lo hace, los tests de HTML y PDF
// de abajo se saltan.
func coreResolvesInlineImages(t *testing.T) bool {
	dir := t.TempDir()
	writePNGs(t, dir, "p.png")
	ctx := renderer.NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = dir
	html := renderer.RenderElementToHTML(ast.NewTextElement(diagnostics.NewPosition(1, 1), "![p](p.png)"), nil, ctx)
	return strings.Contains(html, "data:image/png")
}

// TestHTML_InlineImagesOfflineInline: el HTML offline-inline trae las tres
// imágenes inline como data: URI, sin ningún src relativo.
func TestHTML_InlineImagesOfflineInline(t *testing.T) {
	if !coreResolvesInlineImages(t) {
		t.Skip("this core does not resolve inline images yet")
	}
	dir := t.TempDir()
	writePNGs(t, dir, "a.png", "b.png", "c.png")
	doc := parseInline(t, inlineImageSource("es", "a.png", "b.png", "c.png"))
	outDir := t.TempDir()
	out := filepath.Join(outDir, "out.html")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "html", ImageMode: "offline-inline", AssetRoot: dir, SourceDir: dir}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, alt := range []string{"uno", "dos", "tres"} {
		if !regexp.MustCompile(`<img src="data:image/png;base64,[^"]+" alt="` + alt + `">`).MatchString(html) {
			t.Errorf("inline image %q not inlined", alt)
		}
	}
	if regexp.MustCompile(`<img src="[abc]\.png"`).MatchString(html) {
		t.Error("a relative src was left in offline-inline HTML")
	}
}

// TestPDF_InlineImagesEmbedded (Chromium real): el PDF lleva las tres
// imágenes inline como imágenes de verdad. Antes no llevaba ninguna, solo
// el ícono de imagen rota.
func TestPDF_InlineImagesEmbedded(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Chromium")
	}
	if !coreResolvesInlineImages(t) {
		t.Skip("this core does not resolve inline images yet")
	}
	dir := t.TempDir()
	writePNGs(t, dir, "a.png", "b.png", "c.png")
	doc := parseInline(t, inlineImageSource("es", "a.png", "b.png", "c.png"))
	out := filepath.Join(t.TempDir(), "out.pdf")
	logger := newTestLogger()
	if err := New(logger).Generate(doc, out, GeneratorOptions{Format: "pdf", AssetRoot: dir, SourceDir: dir}); err != nil {
		if strings.Contains(err.Error(), "hromium") {
			t.Skipf("no Chromium: %v", err)
		}
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(data, []byte("/Subtype /Image")); n < 3 {
		t.Errorf("PDF has %d image XObjects, want at least 3", n)
	}
	for _, w := range logger.warns {
		if strings.Contains(w, "[IMAGE]") {
			t.Errorf("unexpected image warning: %s", w)
		}
	}
}

// TestDOCX_InlineImageInCellFitsColumn: una imagen inline grande en una
// celda se reduce al ancho de su columna (menos los márgenes de la celda),
// no al de la caja de texto de la página. Antes un PNG de 4000x3000 salía
// a 6.27" en una columna de 3.22".
func TestDOCX_InlineImageInCellFitsColumn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "huge.png"), pngOfSize(t, 4000, 3000), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "---\ntitle: T\nmode: flex\n---\n\n# T\n\n| Pic | Text |\n|---|---|\n| ![big](huge.png) | some words here |\n"
	out := filepath.Join(t.TempDir(), "out.docx")
	if err := New(newTestLogger()).Generate(parseInline(t, src), out, GeneratorOptions{Format: "docx", AssetRoot: dir, SourceDir: dir}); err != nil {
		t.Fatal(err)
	}
	xml := docxDocumentXML(t, out)
	grid := regexp.MustCompile(`<w:gridCol w:w="(\d+)"`).FindStringSubmatch(xml)
	ext := regexp.MustCompile(`<wp:extent cx="(\d+)" cy="(\d+)"`).FindStringSubmatch(xml)
	if grid == nil || ext == nil {
		t.Fatalf("no grid or no image:\n%s", xml)
	}
	col, _ := strconv.Atoi(grid[1])
	cx, _ := strconv.Atoi(ext[1])
	cy, _ := strconv.Atoi(ext[2])
	maxIn := float64(col-docxCellPaddingTwips) / 1440
	if w := float64(cx) / 914400; w > maxIn+0.01 {
		t.Errorf("image is %.2f\" wide in a column of %.2f\" (text width %.2f\")", w, maxIn, float64(col)/1440)
	}
	if r := float64(cx) / float64(cy); r < 1.32 || r > 1.35 {
		t.Errorf("aspect ratio %.3f, want 4:3", r)
	}
}
