// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// decodeWebPFixture es el WebP en base64 b64.
func decodeWebPFixture(t *testing.T, b64 string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// convertWebPBytes convierte data con convertWebP, sin caja de texto (no
// reduce), y devuelve la extensión y la imagen que salen.
func convertWebPBytes(t *testing.T, data []byte) (string, image.Image, error) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "animada.webp")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	g := &DOCXGenerator{tempDir: t.TempDir()}
	out, err := g.convertWebP(src)
	if err != nil {
		return "", nil, err
	}
	converted, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	ext := filepath.Ext(out)
	var img image.Image
	switch ext {
	case ".jpg":
		img, err = jpeg.Decode(bytes.NewReader(converted))
	case ".png":
		img, err = png.Decode(bytes.NewReader(converted))
	default:
		t.Fatalf("converted to %s", out)
	}
	if err != nil {
		t.Fatalf("%s does not decode: %v", out, err)
	}
	return ext, img, nil
}

// rgba8 es el color de img en (x, y) en 8 bits, sin premultiplicar.
func rgba8(img image.Image, x, y int) (r, g, b, a uint8) {
	c := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	c.Set(0, 0, img.At(x, y))
	return c.Pix[0], c.Pix[1], c.Pix[2], c.Pix[3]
}

// isRed dice si el color en (x, y) es rojo (con pérdida no es exacto) y con
// alfa entre minA y maxA.
func isRed(img image.Image, x, y int, minA, maxA uint8) bool {
	r, g, b, a := rgba8(img, x, y)
	return r > 200 && g < 60 && b < 60 && a >= minA && a <= maxA
}

// De un WebP animado entra el primer cuadro, con el tamaño del lienzo. Con
// pérdida, sin alfa y del tamaño del lienzo entra como JPEG; si no, como PNG.
// Antes x/image/webp no lo decodificaba y quedaba como no disponible.
func TestConvertWebPAnimatedTakesTheFirstFrame(t *testing.T) {
	for _, tc := range []struct {
		name, b64, ext string
		check          func(image.Image) bool
	}{
		{"lossy", webpAnimLossy16x12, ".jpg", func(img image.Image) bool {
			return isRed(img, 0, 0, 255, 255) && isRed(img, 15, 11, 255, 255)
		}},
		{"lossless", webpAnimLossless16x12, ".png", func(img image.Image) bool {
			r, g, b, a := rgba8(img, 8, 6)
			return r == 0 && g == 255 && b == 0 && a == 255
		}},
		{"alpha", webpAnimAlpha16x12, ".png", func(img image.Image) bool {
			return isRed(img, 8, 6, 120, 136)
		}},
		// Si la marca de alfa del lienzo pasara al VP8X del cuadro, x/image
		// lo rechazaría: un VP8 con la marca y sin ALPH.
		{"canvas alpha, opaque first frame", webpAnimCanvasAlpha16x12, ".jpg", func(img image.Image) bool {
			return isRed(img, 8, 6, 255, 255)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ext, img, err := convertWebPBytes(t, decodeWebPFixture(t, tc.b64))
			if err != nil {
				t.Fatal(err)
			}
			if ext != tc.ext {
				t.Errorf("converted to %s, want %s", ext, tc.ext)
			}
			if b := img.Bounds(); b.Dx() != 16 || b.Dy() != 12 {
				t.Errorf("converted %dx%d, want the 16x12 canvas", b.Dx(), b.Dy())
			}
			if !tc.check(img) {
				r, g, b, a := rgba8(img, 8, 6)
				t.Errorf("not the first frame: (8,6) is %d,%d,%d,%d", r, g, b, a)
			}
		})
	}
}

// Un primer cuadro más chico que el lienzo y desplazado se pone en su lugar
// sobre un lienzo transparente, y el resultado es un PNG del tamaño del
// lienzo. Los bordes del cuadro, 6×6 en (6,4), comprueban el ×2 del
// desplazamiento y el +1 del tamaño.
func TestConvertWebPAnimatedPlacesAnOffsetFrame(t *testing.T) {
	ext, img, err := convertWebPBytes(t, decodeWebPFixture(t, webpAnimOffset16x12))
	if err != nil {
		t.Fatal(err)
	}
	if ext != ".png" {
		t.Errorf("converted to %s, want .png (the canvas is partly transparent)", ext)
	}
	if b := img.Bounds(); b.Dx() != 16 || b.Dy() != 12 {
		t.Fatalf("converted %dx%d, want the 16x12 canvas", b.Dx(), b.Dy())
	}
	for _, p := range []image.Point{{6, 4}, {11, 4}, {6, 9}, {11, 9}} {
		if !isRed(img, p.X, p.Y, 255, 255) {
			r, g, b, a := rgba8(img, p.X, p.Y)
			t.Errorf("(%d,%d) inside the frame is %d,%d,%d,%d, want opaque red", p.X, p.Y, r, g, b, a)
		}
	}
	for _, p := range []image.Point{{0, 0}, {5, 4}, {6, 3}, {12, 9}, {11, 10}, {15, 11}} {
		if _, _, _, a := rgba8(img, p.X, p.Y); a != 0 {
			t.Errorf("(%d,%d) outside the frame has alpha %d, want transparent", p.X, p.Y, a)
		}
	}
}

// anmfOffset es el byte donde empieza el chunk ANMF número n (desde 0) de
// data.
func anmfOffset(t *testing.T, data []byte, n int) int {
	t.Helper()
	for off := 12; off+8 <= len(data); {
		size := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if string(data[off:off+4]) == "ANMF" {
			if n == 0 {
				return off
			}
			n--
		}
		off += 8 + size + size%2
	}
	t.Fatal("no such ANMF chunk")
	return 0
}

// animatedWebP arma un WebP animado de lienzo w×h con un solo cuadro: el
// primer ANMF de webpAnimLossy16x12 (16×12 en (0,0)) con su encabezado
// cambiado a fw×fh en (fx,fy).
func animatedWebP(t *testing.T, w, h, fx, fy, fw, fh int) []byte {
	t.Helper()
	src := decodeWebPFixture(t, webpAnimLossy16x12)
	off := anmfOffset(t, src, 0)
	size := int(binary.LittleEndian.Uint32(src[off+4 : off+8]))
	anmf := append([]byte{}, src[off+8:off+8+size]...)
	put24 := func(b []byte, v int) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }
	put24(anmf[0:3], fx/2)
	put24(anmf[3:6], fy/2)
	put24(anmf[6:9], fw-1)
	put24(anmf[9:12], fh-1)
	vp8x := make([]byte, 10)
	vp8x[0] = 1 << 1 // animación
	put24(vp8x[4:7], w-1)
	put24(vp8x[7:10], h-1)
	body := appendRIFFChunk([]byte("WEBP"), "VP8X", vp8x)
	body = appendRIFFChunk(body, "ANIM", make([]byte, 6))
	body = appendRIFFChunk(body, "ANMF", anmf)
	return append(binary.LittleEndian.AppendUint32([]byte("RIFF"), uint32(len(body))), body...)
}

// El builder de las pruebas da un WebP que se convierte, así que las que
// siguen fallan por lo que cambian y no por el armado.
func TestAnimatedWebPBuilder(t *testing.T) {
	ext, img, err := convertWebPBytes(t, animatedWebP(t, 16, 12, 0, 0, 16, 12))
	if err != nil {
		t.Fatal(err)
	}
	if ext != ".jpg" || img.Bounds().Dx() != 16 || !isRed(img, 8, 6, 255, 255) {
		t.Errorf("built WebP converted to %s %v", ext, img.Bounds())
	}
	ext, img, err = convertWebPBytes(t, animatedWebP(t, 40, 30, 10, 8, 16, 12))
	if err != nil {
		t.Fatal(err)
	}
	if ext != ".png" || img.Bounds().Dx() != 40 || !isRed(img, 10, 8, 255, 255) {
		t.Errorf("built offset WebP converted to %s %v", ext, img.Bounds())
	}
	if _, _, _, a := rgba8(img, 9, 8); a != 0 {
		t.Error("outside the frame is not transparent")
	}
}

// Un cuadro que se sale del lienzo es un WebP inválido: el tope de píxeles se
// aplica al lienzo, así que un cuadro más grande que él lo saltaría.
func TestConvertWebPAnimatedFrameOutsideTheCanvas(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		w, h, fx, fy, fw, fh int
	}{
		{"larger", 8, 8, 0, 0, 16, 12},
		{"offset past the right edge", 16, 12, 2, 0, 16, 12},
		{"offset past the bottom edge", 16, 12, 0, 2, 16, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := convertWebPBytes(t, animatedWebP(t, tc.w, tc.h, tc.fx, tc.fy, tc.fw, tc.fh))
			if !errors.Is(err, errWebPAnimation) {
				t.Errorf("err = %v, want errWebPAnimation", err)
			}
		})
	}
}

// Un WebP animado truncado da error sin panic, corte donde corte: a media
// ANMF del primer cuadro, en el segundo o en el encabezado. Y un byte
// cambiado en cualquier lugar tampoco provoca panic.
func TestConvertWebPAnimatedTruncatedOrDamaged(t *testing.T) {
	data := decodeWebPFixture(t, webpAnimAlpha16x12)
	off := anmfOffset(t, data, 0)
	size := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
	if _, _, err := convertWebPBytes(t, data[:off+8+size/2]); err == nil {
		t.Error("a WebP truncated in the middle of its first ANMF was converted")
	}
	for n := 0; n < len(data); n++ {
		if _, _, err := convertWebPBytes(t, data[:n]); err == nil {
			t.Errorf("a WebP truncated to %d of %d bytes was converted", n, len(data))
		}
	}
	for i := 12; i < len(data); i++ {
		damaged := append([]byte{}, data...)
		damaged[i] ^= 0xFF
		_, _, _ = convertWebPBytes(t, damaged) // basta con que no haya panic
	}
}

// En el DOCX, de bloque y en la prosa, un WebP animado entra con el tamaño
// del lienzo y su texto alternativo; uno truncado queda como no disponible,
// con un aviso. Ningún WebP crudo llega a word/media.
func TestDOCX_AnimatedWebPIsInserted(t *testing.T) {
	root := t.TempDir()
	for name, b64 := range map[string]string{
		"rojo.webp":        webpAnimLossy16x12,
		"verde.webp":       webpAnimLossless16x12,
		"cuadrado.webp":    webpAnimOffset16x12,
		"translucido.webp": webpAnimAlpha16x12,
	} {
		writeWebP(t, root, name, b64)
	}
	data := decodeWebPFixture(t, webpAnimLossy16x12)
	off := anmfOffset(t, data, 0)
	if err := os.WriteFile(filepath.Join(root, "truncada.webp"), data[:off+20], 0o644); err != nil {
		t.Fatal(err)
	}
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "WebP animado"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "S"
	block.Elements = append(block.Elements,
		ast.NewImageElement(diagnostics.NewPosition(3, 1), "rojo.webp", "Rojo"),
		ast.NewImageElement(diagnostics.NewPosition(4, 1), "verde.webp", "Verde"),
		ast.NewImageElement(diagnostics.NewPosition(5, 1), "cuadrado.webp", "Cuadrado"),
		ast.NewTextElement(diagnostics.NewPosition(6, 1), "Texto con ![Translúcido](translucido.webp) adentro."),
		ast.NewImageElement(diagnostics.NewPosition(7, 1), "truncada.webp", "Truncada"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	logger := newTestLogger()
	out := filepath.Join(t.TempDir(), "animada.docx")
	if err := New(logger).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: root, SourceDir: root}); err != nil {
		t.Fatal(err)
	}
	if got, want := docxDescrs(t, out, "word/document.xml"), []string{"Rojo", "Verde", "Cuadrado", "Translúcido"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("image descriptions = %q, want %q", got, want)
	}
	xml := docxDocumentXML(t, out)
	if n := strings.Count(xml, "Imagen no disponible"); n != 1 || !strings.Contains(xml, "[Imagen no disponible: Truncada]") {
		t.Errorf("%d unavailable markers, want only the truncated WebP", n)
	}
	// 16×12 px a 96 ppp: el lienzo, también para el cuadro de 6×6.
	if n := strings.Count(xml, `cx="152400" cy="114300"`); n < 4 {
		t.Errorf("%d images at the 16x12 canvas size, want 4", n)
	}
	warned := false
	for _, w := range logger.warns {
		warned = warned || strings.Contains(w, "truncada.webp") && strings.Contains(w, "could not be inserted")
	}
	if !warned {
		t.Errorf("no warning about the truncated WebP: %q", logger.warns)
	}
	var jpegs, pngs int
	for name, data := range docxMediaFiles(t, out) {
		switch {
		case isWebP(data):
			t.Errorf("%s is a raw WebP", name)
		case bytes.HasPrefix(data, []byte{0xFF, 0xD8}):
			jpegs++
		case bytes.HasPrefix(data, []byte("\x89PNG")):
			pngs++
		default:
			t.Errorf("%s is neither JPEG nor PNG", name)
		}
	}
	if jpegs != 1 || pngs != 3 {
		t.Errorf("media: %d JPEG and %d PNG, want 1 and 3", jpegs, pngs)
	}
}

// El tope de píxeles cuenta el lienzo de un WebP animado: uno de 8000×6000
// (48 megapíxeles) con un cuadro válido queda como no disponible enseguida,
// con un aviso que da sus dimensiones. Sin el tope el cuadro se pondría
// sobre un lienzo de 192 MB y entraría.
func TestDOCX_HugeAnimatedWebPCanvasIsNotDecoded(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lienzo.webp"), animatedWebP(t, 8000, 6000, 0, 0, 16, 12), 0o644); err != nil {
		t.Fatal(err)
	}
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "WebP"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "S"
	block.Elements = append(block.Elements, ast.NewImageElement(diagnostics.NewPosition(3, 1), "lienzo.webp", "Lienzo"))
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	logger := newTestLogger()
	out := filepath.Join(t.TempDir(), "lienzo.docx")
	start := time.Now()
	if err := New(logger).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: root, SourceDir: root}); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("generation took %v: the huge canvas was allocated", d)
	}
	if !strings.Contains(docxDocumentXML(t, out), "[Imagen no disponible: Lienzo]") {
		t.Error("the WebP with a huge canvas is not left as unavailable")
	}
	warned := false
	for _, w := range logger.warns {
		warned = warned || strings.Contains(w, "lienzo.webp") && strings.Contains(w, "too large") && strings.Contains(w, "8000×6000")
	}
	if !warned {
		t.Errorf("no warning about the size: %q", logger.warns)
	}
}
