// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"golang.org/x/image/webp"
)

// Imágenes WebP de prueba, hechas con Pillow: una sin pérdida de 40×20 roja,
// una con pérdida de 30×30 azul con alfa 128 y una animada de dos cuadros de
// 8×8.
const (
	webpRed40x20    = "UklGRh4AAABXRUJQVlA4TBEAAAAvJ8AEAAdQjyLXo/+BiOh/AAA="
	webpAlpha30x30  = "UklGRmwAAABXRUJQVlA4WAoAAAAQAAAAHQAAHQAAQUxQSAoAAAABB1DAiAhERP8DVlA4IDwAAAAQAwCdASoeAB4APm00lkekIyIhKAgAgA2JZQB2AACAoKAA/vkd2//+QH/+QH/+QH/8gP/+IXeyAwAAAAA="
	webpAnimated8x8 = "UklGRsoAAABXRUJQVlA4WAoAAAACAAAABwAABwAAQU5JTQYAAAAAAAAAAABBTk1GSgAAAAAAAAAAAAcAAAcAAGQAAAJWUDggMgAAADABAJ0BKggACAABQCYloAADcAD+8ut///mwP/bz/wR6Af//0uD//pcH//S4P/SkAAAAQU5NRkwAAAAAAAAAAAAHAAAHAABkAAAAVlA4IDQAAAA0AQCdASoIAAgAAAAmJaAAA3AA/ukiH//3nz//ufP/+58/6M///yn7//I4//8jj/5QIAAA"
)

// Un WebP entra en el DOCX como PNG, de bloque y dentro de la prosa, con su
// tamaño natural, su transparencia y su texto alternativo; también si la
// extensión no dice WebP. Antes quedaba como "imagen no disponible". Uno
// animado, que no se puede decodificar, sigue quedando como no disponible.
func TestDOCX_WebPImagesAreInsertedAsPNG(t *testing.T) {
	root := t.TempDir()
	for name, b64 := range map[string]string{
		"roja.webp":      webpRed40x20,
		"alfa.webp":      webpAlpha30x30,
		"disfrazada.png": webpRed40x20,
		"animada.webp":   webpAnimated8x8,
	} {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "WebP"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "S"
	block.Elements = append(block.Elements,
		ast.NewImageElement(diagnostics.NewPosition(3, 1), "roja.webp", "Una imagen roja"),
		ast.NewImageElement(diagnostics.NewPosition(4, 1), "disfrazada.png", "Otra roja"),
		ast.NewTextElement(diagnostics.NewPosition(5, 1), "Texto con ![Azul translúcido](alfa.webp) adentro."),
		ast.NewImageElement(diagnostics.NewPosition(6, 1), "animada.webp", "Animada"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out := filepath.Join(t.TempDir(), "webp.docx")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: root, SourceDir: root}); err != nil {
		t.Fatal(err)
	}

	if got, want := docxDescrs(t, out, "word/document.xml"), []string{"Una imagen roja", "Otra roja", "Azul translúcido"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("image descriptions = %q, want %q", got, want)
	}
	xml := docxDocumentXML(t, out)
	if !strings.Contains(xml, "[Imagen no disponible: Animada]") {
		t.Error("the animated WebP is not left as unavailable")
	}
	if n := strings.Count(xml, "Imagen no disponible"); n != 1 {
		t.Errorf("%d images left as unavailable, want only the animated one", n)
	}
	// 40 px a 96 DPI son 381000 EMU de ancho.
	if !strings.Contains(xml, `cx="381000" cy="190500"`) {
		t.Error("the 40×20 WebP is not inserted at its natural size")
	}

	media := docxMediaFiles(t, out)
	var sizes []string
	alpha := false
	for name, data := range media {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Errorf("%s is not a PNG: %v", name, err)
			continue
		}
		b := img.Bounds()
		sizes = append(sizes, fmt.Sprintf("%dx%d", b.Dx(), b.Dy()))
		if _, _, _, a := img.At(0, 0).RGBA(); a < 0xffff && a > 0 {
			alpha = true
		}
	}
	sort.Strings(sizes)
	// La roja entra una vez por cada elemento que la usa, o una sola si
	// docxgo comparte el archivo: lo que importa es que estén las dos.
	if !strings.Contains(strings.Join(sizes, " "), "30x30") || !strings.Contains(strings.Join(sizes, " "), "40x20") {
		t.Errorf("media sizes = %v, want the 40x20 and the 30x30 PNGs", sizes)
	}
	if !alpha {
		t.Error("the translucent WebP lost its transparency")
	}
}

// writeWebP escribe el WebP en base64 b64 como dir/name.
func writeWebP(t *testing.T, dir, name, b64 string) string {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// bombWebP es un WebP sin pérdida que declara w×h en su encabezado VP8L y no
// trae datos: webp.DecodeConfig lo lee y Decode, que pediría w×h×4 bytes,
// nunca tiene que correr.
func bombWebP(w, h int) []byte {
	bits := uint32(w-1) | uint32(h-1)<<14
	chunk := []byte{0x2f, byte(bits), byte(bits >> 8), byte(bits >> 16), byte(bits >> 24), 0, 0, 0}
	le := func(n int) []byte { return []byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)} }
	out := append([]byte("RIFF"), le(4+8+len(chunk))...)
	out = append(out, "WEBP"...)
	out = append(out, "VP8L"...)
	out = append(out, le(len(chunk))...)
	return append(out, chunk...)
}

// Un WebP con pérdida y sin alfa entra como JPEG; uno sin pérdida o con alfa,
// como PNG. El perfil de color pasa al archivo nuevo: un iCCP en el PNG, un
// APP2 ICC_PROFILE en el JPEG. Ningún WebP crudo llega a word/media.
func TestDOCX_WebPFormatAndColorProfile(t *testing.T) {
	root := t.TempDir()
	for name, b64 := range map[string]string{
		"verde.webp":     webpLossy24x16,
		"verde-icc.webp": webpLossyICC24x16,
		"roja-icc.webp":  webpLosslessICC8x8,
		"alfa.webp":      webpAlpha30x30,
	} {
		writeWebP(t, root, name, b64)
	}
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "WebP"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "S"
	for i, name := range []string{"verde.webp", "verde-icc.webp", "roja-icc.webp", "alfa.webp"} {
		block.Elements = append(block.Elements, ast.NewImageElement(diagnostics.NewPosition(3+i, 1), name, name))
	}
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	out := filepath.Join(t.TempDir(), "webp.docx")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: root, SourceDir: root}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(docxDocumentXML(t, out), "no disponible") {
		t.Fatal("a WebP was left as unavailable")
	}
	var jpegs, pngs, jpegICC, pngICC int
	for name, data := range docxMediaFiles(t, out) {
		switch {
		case isWebP(data):
			t.Errorf("%s is a raw WebP", name)
		case bytes.HasPrefix(data, []byte{0xFF, 0xD8}):
			jpegs++
			if bytes.Contains(data, []byte("ICC_PROFILE\x00\x01\x01")) {
				jpegICC++
			}
			if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		case bytes.HasPrefix(data, []byte("\x89PNG")):
			pngs++
			if bytes.Contains(data, []byte("iCCPICC Profile")) {
				pngICC++
			}
			if _, err := png.Decode(bytes.NewReader(data)); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		default:
			t.Errorf("%s is neither JPEG nor PNG", name)
		}
	}
	if jpegs != 2 || pngs != 2 || jpegICC != 1 || pngICC != 1 {
		t.Errorf("media: %d JPEG (%d with ICC), %d PNG (%d with ICC); want 2 (1) and 2 (1)", jpegs, jpegICC, pngs, pngICC)
	}
}

// El perfil que se escribe es el del WebP, entero.
func TestEncodeWithICCKeepsTheProfile(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(webpLosslessICC8x8)
	if err != nil {
		t.Fatal(err)
	}
	icc, _ := webpChunk(data, "ICCP")
	if len(icc) != 588 {
		t.Fatalf("ICC profile of %d bytes, want 588", len(icc))
	}
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	pngData, err := encodePNGWithICC(img, icc)
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(pngData, []byte("iCCPICC Profile\x00\x00"))
	if i < 0 {
		t.Fatal("no iCCP chunk")
	}
	size := int(binary.BigEndian.Uint32(pngData[i-4 : i]))
	zr, err := zlib.NewReader(bytes.NewReader(pngData[i+4+len("ICC Profile\x00\x00") : i+4+size]))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(zr)
	if err != nil || !bytes.Equal(got, icc) {
		t.Errorf("iCCP profile differs (%v)", err)
	}
	if _, err := png.Decode(bytes.NewReader(pngData)); err != nil {
		t.Errorf("PNG with iCCP does not decode: %v", err)
	}
	jpegData, err := encodeJPEGWithICC(img, icc)
	if err != nil {
		t.Fatal(err)
	}
	j := bytes.Index(jpegData, []byte("ICC_PROFILE\x00\x01\x01"))
	if j < 0 || !bytes.Equal(jpegData[j+14:j+14+len(icc)], icc) {
		t.Error("APP2 ICC_PROFILE missing or different")
	}
	if _, err := jpeg.Decode(bytes.NewReader(jpegData)); err != nil {
		t.Errorf("JPEG with APP2 does not decode: %v", err)
	}
}

// Un WebP que declara más de docxWebPMaxPixels no se decodifica: queda como
// no disponible enseguida, con un aviso que dice por qué. Antes uno de 40
// bytes con 16383×16383 tardaba 11 s y pedía 1.2 GB por referencia.
func TestDOCX_HugeWebPIsNotDecoded(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bomba.webp"), bombWebP(16383, 16383), 0o644); err != nil {
		t.Fatal(err)
	}
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "WebP"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "S"
	for i := 0; i < 3; i++ {
		block.Elements = append(block.Elements, ast.NewImageElement(diagnostics.NewPosition(3+i, 1), "bomba.webp", "Bomba"))
	}
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	logger := newTestLogger()
	out := filepath.Join(t.TempDir(), "bomba.docx")
	start := time.Now()
	if err := New(logger).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: root, SourceDir: root}); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("generation took %v: the huge WebP was decoded", d)
	}
	if n := strings.Count(docxDocumentXML(t, out), "[Imagen no disponible: Bomba]"); n != 3 {
		t.Errorf("%d unavailable markers, want 3", n)
	}
	warned := false
	for _, w := range logger.warns {
		warned = warned || strings.Contains(w, "bomba.webp") && strings.Contains(w, "too large") && strings.Contains(w, "16383×16383")
	}
	if !warned {
		t.Errorf("no warning about the size: %q", logger.warns)
	}
}

// Un WebP ya convertido (mismo contenido) no se vuelve a convertir, y con el
// contexto cancelado no se decodifica.
func TestConvertWebPReusesAndRespectsContext(t *testing.T) {
	src := writeWebP(t, t.TempDir(), "roja.webp", webpRed40x20)
	g := &DOCXGenerator{tempDir: t.TempDir()}
	first, err := g.convertWebP(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first, []byte("ya convertida"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := g.convertWebP(src)
	if err != nil || second != first {
		t.Fatalf("second conversion = %q, %v; want %q", second, err, first)
	}
	if data, _ := os.ReadFile(second); string(data) != "ya convertida" {
		t.Error("the WebP was converted again")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g = &DOCXGenerator{tempDir: t.TempDir(), ctx: ctx}
	if _, err := g.convertWebP(src); !errors.Is(err, context.Canceled) {
		t.Errorf("with a cancelled context: %v, want context.Canceled", err)
	}
}

// Un chunk de largo impar lleva un byte de relleno: el siguiente empieza
// después de él.
func TestWebPChunkSkipsPadding(t *testing.T) {
	data := []byte("RIFF\x00\x00\x00\x00WEBP" + "EXIF\x03\x00\x00\x00abc\x00" + "ICCP\x02\x00\x00\x00pq")
	got, ok := webpChunk(data, "ICCP")
	if !ok || string(got) != "pq" {
		t.Errorf("webpChunk = %q, %v; want \"pq\"", got, ok)
	}
	if _, ok := webpChunk(data[:len(data)-1], "ICCP"); ok {
		t.Error("a truncated chunk was returned")
	}
}

// Un WebP más grande que la caja de texto a 220 ppp se reduce a ella,
// conservando la proporción y la transparencia, y se sigue mostrando al ancho
// de la caja. Uno más chico no se toca.
func TestDownscaleToFitKeepsDisplaySize(t *testing.T) {
	g := &DOCXGenerator{contentWidthIn: 6.5, contentHeightIn: 9}
	maxW, maxH := g.webpMaxPixelSize()
	if maxW != 1430 || maxH != 1980 {
		t.Fatalf("max size %dx%d, want 1430x1980", maxW, maxH)
	}
	big := image.NewNRGBA(image.Rect(0, 0, 3840, 2160))
	for i := range big.Pix {
		big.Pix[i] = 0x80
	}
	small := downscaleToFit(big, maxW, maxH, true)
	if b := small.Bounds(); b.Dx() != 1430 || b.Dy() != 804 {
		t.Errorf("downscaled to %dx%d, want 1430x804", b.Dx(), b.Dy())
	}
	if _, ok := small.(*image.NRGBA); !ok {
		t.Errorf("downscaled image is %T, want *image.NRGBA to keep the alpha", small)
	}
	if _, _, _, a := small.At(10, 10).RGBA(); a == 0xffff {
		t.Error("the transparency was lost")
	}
	if got := downscaleToFit(big, 4000, 4000, true); got != image.Image(big) {
		t.Error("an image that fits was resampled")
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, small); err != nil {
		t.Fatal(err)
	}
	full, reduced := writeImage(t, pngOfSize(t, 3840, 2160)), writeImage(t, buf.Bytes())
	wFull, hFull := sizeIn(g, full, false)
	wSmall, hSmall := sizeIn(g, reduced, false)
	if math.Abs(wFull-wSmall) > 0.01 || math.Abs(hFull-hSmall) > 0.01 {
		t.Errorf("display size %.2fx%.2f in, want %.2fx%.2f as without downscaling", wSmall, hSmall, wFull, hFull)
	}
}

// convertWebP reduce a la caja de texto: con una caja de 0.1×0.1 pulgadas
// (22×22 px a 220 ppp) el WebP de 40×20 sale de 22×11.
func TestConvertWebPDownscalesToTheContentBox(t *testing.T) {
	src := writeWebP(t, t.TempDir(), "roja.webp", webpRed40x20)
	g := &DOCXGenerator{tempDir: t.TempDir(), contentWidthIn: 0.1, contentHeightIn: 0.1}
	out, err := g.convertWebP(src)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 22 || cfg.Height != 11 {
		t.Errorf("converted %dx%d (%v), want 22x11", cfg.Width, cfg.Height, err)
	}
}
