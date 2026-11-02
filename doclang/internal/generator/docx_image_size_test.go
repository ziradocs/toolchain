// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
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

	// 4x4 px sin pHYs → 4/96". Antes, 6"x6".
	if w, h := sizeIn(g, writeImage(t, pngOfSize(t, 4, 4)), false); !near(w, 4.0/96) || !near(h, 4.0/96) {
		t.Errorf("tiny image: got %.3fx%.3f, want %.3f square", w, h, 4.0/96)
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
		{"png sin pHYs", pngOfSize(t, 2, 2), 96, 96},
		{"png 72 dpi", withPHYs(t, pngOfSize(t, 2, 2), 2835), 2835 / 39.3701, 2835 / 39.3701},
		{"basura", []byte("no es una imagen"), 96, 96},
	}
	for _, c := range cases {
		if x, y := imageDPI(c.data); !near(x, c.wx) || !near(y, c.wy) {
			t.Errorf("%s: got %.2fx%.2f, want %.2fx%.2f", c.name, x, y, c.wx, c.wy)
		}
	}
}
