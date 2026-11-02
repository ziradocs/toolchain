// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// withPHYs inserta un chunk pHYs (píxeles por metro, unidad 1) después del
// IHDR de un PNG.
func withPHYs(t *testing.T, data []byte, ppm uint32) []byte {
	t.Helper()
	body := make([]byte, 9)
	binary.BigEndian.PutUint32(body[0:], ppm)
	binary.BigEndian.PutUint32(body[4:], ppm)
	body[8] = 1
	chunk := make([]byte, 0, 21)
	chunk = binary.BigEndian.AppendUint32(chunk, 9)
	chunk = append(chunk, "pHYs"...)
	chunk = append(chunk, body...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	ihdrEnd := 8 + 8 + 13 + 4
	out := append([]byte{}, data[:ihdrEnd]...)
	out = append(out, chunk...)
	return append(out, data[ihdrEnd:]...)
}

func writeImage(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "img.png")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func sizeIn(g *DOCXGenerator, path string, caption bool) (float64, float64) {
	s := g.fittedImageSize(path, caption)
	return float64(s.WidthEMU) / 914400, float64(s.HeightEMU) / 914400
}

// TestFittedImageSize: una imagen se inserta a su tamaño natural (96 DPI si
// no declara otra resolución), nunca agrandada, y reducida con la regla de
// las figuras de mermaid. Antes iba siempre a 6" de ancho.
func TestFittedImageSize(t *testing.T) {
	g := &DOCXGenerator{contentWidthIn: 6.5, contentHeightIn: 9}

	// 4x4 px sin pHYs mide 4/96" (0.04"); sube al lado mínimo de 0.1".
	// Antes iba a 6"x6".
	if w, h := sizeIn(g, writeImage(t, pngOfSize(t, 4, 4)), false); !near(w, 0.1) || !near(h, 0.1) {
		t.Errorf("tiny image: got %.3fx%.3f, want 0.1 square", w, h)
	}

	// 288x144 px → 3"x1.5" a 96 DPI.
	if w, h := sizeIn(g, writeImage(t, pngOfSize(t, 288, 144)), false); !near(w, 3) || !near(h, 1.5) {
		t.Errorf("96 dpi: got %.2fx%.2f, want 3x1.5", w, h)
	}

	// Los mismos píxeles con pHYs de 300 DPI (11811 px/m) → 0.96"x0.48".
	hiDPI := withPHYs(t, pngOfSize(t, 288, 144), 11811)
	if w, h := sizeIn(g, writeImage(t, hiDPI), false); !near(w, 288/300.0) || !near(h, 144/300.0) {
		t.Errorf("300 dpi: got %.3fx%.3f, want %.3fx%.3f", w, h, 288/300.0, 144/300.0)
	}

	// Muy ancha: al ancho de la caja.
	if w, h := sizeIn(g, writeImage(t, pngOfSize(t, 1248, 312)), false); !near(w, 6.5) || !near(h, 1.625) {
		t.Errorf("wide image: got %.2fx%.2f, want 6.5x1.625", w, h)
	}

	// Muy alta con leyenda: al alto de página menos las reservas.
	w, h := sizeIn(g, writeImage(t, pngOfSize(t, 300, 1920)), true)
	wantH := 9 - figureSpacingReserveIn - figureCaptionReserveIn
	if !near(h, wantH) || !near(w, wantH*300/1920) {
		t.Errorf("tall image: got %.2fx%.2f, want height %.2f", w, h, wantH)
	}
}

func TestImageDPI(t *testing.T) {
	// JFIF: unidades 1 (dpi) y 2 (dpcm); 0 (solo proporción) cae a 96.
	jfif := func(units byte, x, y uint16) []byte {
		b := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 1, 1, units, 0, 0, 0, 0}
		binary.BigEndian.PutUint16(b[14:], x)
		binary.BigEndian.PutUint16(b[16:], y)
		return b
	}
	cases := []struct {
		name   string
		data   []byte
		wx, wy float64
	}{
		{"jfif dpi", jfif(1, 300, 150), 300, 150},
		{"jfif dpcm", jfif(2, 118, 118), 118 * 2.54, 118 * 2.54},
		{"jfif aspect only", jfif(0, 1, 1), 96, 96},
		{"jfif 1 dpi", jfif(1, 1, 1), 96, 96},
		{"jfif 2400 dpi", jfif(1, 2400, 2400), 96, 96},
		{"jfif 600 dpi", jfif(1, 600, 600), 600, 600},
		{"png sin pHYs", pngOfSize(t, 2, 2), 96, 96},
		{"png 72 dpi", withPHYs(t, pngOfSize(t, 2, 2), 2835), 2835 / 39.3701, 2835 / 39.3701},
		{"png 0xffffffff", withPHYs(t, pngOfSize(t, 2, 2), 0xffffffff), 96, 96},
		{"png 2400 dpi", withPHYs(t, pngOfSize(t, 2, 2), 94488), 96, 96},
		{"basura", []byte("no es una imagen"), 96, 96},
	}
	for _, c := range cases {
		if x, y := imageDPI(c.data); !near(x, c.wx) || !near(y, c.wy) {
			t.Errorf("%s: got %.2fx%.2f, want %.2fx%.2f", c.name, x, y, c.wx, c.wy)
		}
	}
}

// TestFittedImageSize_ExtremeImages son los casos del review de #453: con
// domain.NewImageSizeInches, que trunca a píxeles enteros, un lado de menos
// de 1 px quedaba en 0. Con los dos en 0 docxgo fallaba y la imagen salía
// como "[Image not found]"; con uno solo, cy="0" e invisible.
func TestFittedImageSize_ExtremeImages(t *testing.T) {
	g := &DOCXGenerator{contentWidthIn: 6.5, contentHeightIn: 9}
	cases := []struct {
		name  string
		data  []byte
		wantW float64
		wantH float64
	}{
		// 2400 dpi está fuera del rango: se toma 96, 16 px = 0.167".
		{"icono 16x16 a 2400 dpi", withPHYs(t, pngOfSize(t, 16, 16), 94488), 16.0 / 96, 16.0 / 96},
		// 1 px a 144 dpi = 0.007": sube al mínimo, cuadrado.
		{"1x1 a 144 dpi", withPHYs(t, pngOfSize(t, 1, 1), 5669), 0.1, 0.1},
		// Franja: al ancho de la caja y el alto sube al mínimo; la proporción
		// no se puede conservar sin salirse de la página.
		{"franja 3000x1", pngOfSize(t, 3000, 1), 6.5, 0.1},
		// pHYs absurdo: se ignora.
		{"pHYs 0xffffffff", withPHYs(t, pngOfSize(t, 96, 48), 0xffffffff), 1, 0.5},
		// Retina chica: 40x20 px a 192 dpi = 0.208"x0.104", ya sobre el mínimo.
		{"retina 40x20 a 192 dpi", withPHYs(t, pngOfSize(t, 40, 20), 7559), 40.0 / 192, 20.0 / 192},
		// 30x10 px a 96 dpi = 0.31"x0.10": cabe escalando, proporción intacta.
		{"30x6 a 96 dpi", pngOfSize(t, 30, 6), 0.5, 0.1},
	}
	for _, c := range cases {
		s := g.fittedImageSize(writeImage(t, c.data), false)
		if s.WidthEMU <= 0 || s.HeightEMU <= 0 || s.WidthPx <= 0 || s.HeightPx <= 0 {
			t.Errorf("%s: tamaño con un lado en 0: %+v", c.name, s)
			continue
		}
		w, h := float64(s.WidthEMU)/914400, float64(s.HeightEMU)/914400
		if !near(w, c.wantW) || !near(h, c.wantH) {
			t.Errorf("%s: got %.3fx%.3f, want %.3fx%.3f", c.name, w, h, c.wantW, c.wantH)
		}
	}
}

// TestDOCX_ExtremeImagesAreInserted: los mismos casos de punta a punta. Cada
// imagen termina en word/media/, ningún extent es 0 y ninguna sale como
// "not found". Un archivo que existe pero no es una imagen se reporta como
// "could not be inserted", no como "not found".
func TestDOCX_ExtremeImagesAreInserted(t *testing.T) {
	root := t.TempDir()
	files := map[string][]byte{
		"icon.png":   withPHYs(t, pngOfSize(t, 16, 16), 94488),
		"dot.png":    withPHYs(t, pngOfSize(t, 1, 1), 5669),
		"strip.png":  pngOfSize(t, 3000, 1),
		"huge.png":   withPHYs(t, pngOfSize(t, 8, 8), 0xffffffff),
		"broken.png": []byte("esto no es un PNG"),
	}
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "Imágenes"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "S"
	for i, name := range []string{"icon.png", "dot.png", "strip.png", "huge.png", "broken.png"} {
		if err := os.WriteFile(filepath.Join(root, name), files[name], 0o644); err != nil {
			t.Fatal(err)
		}
		block.Elements = append(block.Elements, ast.NewImageElement(diagnostics.NewPosition(3+i, 1), name, name))
	}
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	logger := newTestLogger()
	out := filepath.Join(t.TempDir(), "img.docx")
	if err := New(logger).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: root, SourceDir: root}); err != nil {
		t.Fatal(err)
	}
	xml := docxDocumentXML(t, out)
	if strings.Contains(xml, "not found") {
		t.Errorf("una imagen existente salió como not found")
	}
	if !strings.Contains(xml, "[Image could not be inserted: broken.png]") {
		t.Errorf("falta el marcador de broken.png")
	}
	if strings.Contains(xml, `cx="0"`) || strings.Contains(xml, `cy="0"`) {
		t.Errorf("document.xml tiene un extent en 0")
	}
	if n := len(docxMediaFiles(t, out)); n != 4 {
		t.Errorf("word/media/ tiene %d imágenes, want 4", n)
	}
	warned := false
	for _, w := range logger.warns {
		if strings.Contains(w, "broken.png") && strings.Contains(w, "could not be inserted") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("falta el warning de broken.png: %q", logger.warns)
	}
}
