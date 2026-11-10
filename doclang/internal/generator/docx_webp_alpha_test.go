// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// vp8Of es el chunk VP8 (con pérdida) del WebP fijo data y su tamaño.
func vp8Of(t *testing.T, data []byte) ([]byte, int, int) {
	t.Helper()
	vp8, ok := webpChunk(data, "VP8 ")
	if !ok || len(vp8) < 10 {
		t.Fatal("no VP8 chunk")
	}
	// Ancho y alto van en 14 bits a partir del byte 6 del encabezado VP8.
	return vp8, int(binary.LittleEndian.Uint16(vp8[6:8]) & 0x3fff), int(binary.LittleEndian.Uint16(vp8[8:10]) & 0x3fff)
}

// alphaChunk es un ALPH sin compresión ni filtro con alfa 255 en w×h
// píxeles, salvo los de translucent, que llevan 128.
func alphaChunk(w, h int, translucent ...int) []byte {
	alph := append([]byte{0}, bytes.Repeat([]byte{0xff}, w*h)...)
	for _, i := range translucent {
		alph[1+i] = 0x80
	}
	return alph
}

// withALPH es el WebP con pérdida data con un chunk ALPH (VP8X, ALPH, VP8),
// como lo escriben los codificadores que guardan el canal alfa aunque la
// imagen sea opaca. libwebp no lo hace (si no hay transparencia omite el
// ALPH), así que estas imágenes se arman aquí.
func withALPH(t *testing.T, data []byte, translucent ...int) []byte {
	t.Helper()
	vp8, w, h := vp8Of(t, data)
	vp8x := make([]byte, 10)
	vp8x[0] = 1 << 4 // alfa
	put24 := func(b []byte, v int) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }
	put24(vp8x[4:7], w-1)
	put24(vp8x[7:10], h-1)
	body := appendRIFFChunk([]byte("WEBP"), "VP8X", vp8x)
	body = appendRIFFChunk(body, "ALPH", alphaChunk(w, h, translucent...))
	body = appendRIFFChunk(body, "VP8 ", vp8)
	return append(binary.LittleEndian.AppendUint32([]byte("RIFF"), uint32(len(body))), body...)
}

// animatedWithALPH es un WebP animado de un cuadro, del tamaño del lienzo, con
// el VP8 de data y un ALPH.
func animatedWithALPH(t *testing.T, data []byte, translucent ...int) []byte {
	t.Helper()
	vp8, w, h := vp8Of(t, data)
	put24 := func(b []byte, v int) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }
	anmf := make([]byte, 16)
	put24(anmf[6:9], w-1)
	put24(anmf[9:12], h-1)
	put24(anmf[12:15], 100)
	anmf = appendRIFFChunk(anmf, "ALPH", alphaChunk(w, h, translucent...))
	anmf = appendRIFFChunk(anmf, "VP8 ", vp8)
	vp8x := make([]byte, 10)
	vp8x[0] = 1<<4 | 1<<1 // alfa y animación
	put24(vp8x[4:7], w-1)
	put24(vp8x[7:10], h-1)
	body := appendRIFFChunk([]byte("WEBP"), "VP8X", vp8x)
	body = appendRIFFChunk(body, "ANIM", make([]byte, 6))
	body = appendRIFFChunk(body, "ANMF", anmf)
	return append(binary.LittleEndian.AppendUint32([]byte("RIFF"), uint32(len(body))), body...)
}

// Un WebP con pérdida que trae ALPH pero es opaco entero entra como JPEG, fijo
// o como primer cuadro de una animación. Con un solo píxel translúcido sigue
// en PNG, con ese píxel translúcido. Uno sin pérdida sigue en PNG aunque sea
// opaco. Antes la clase salía de los chunks: con ALPH era PNG siempre.
func TestConvertWebPOpaqueAlphaIsJPEG(t *testing.T) {
	lossy := decodeWebPFixture(t, webpLossy24x16)
	for _, tc := range []struct {
		name string
		data []byte
		ext  string
	}{
		{"still, opaque ALPH", withALPH(t, lossy), ".jpg"},
		{"still, one translucent pixel", withALPH(t, lossy, 24*5+7), ".png"},
		{"animated, opaque ALPH", animatedWithALPH(t, lossy), ".jpg"},
		{"animated, one translucent pixel", animatedWithALPH(t, lossy, 24*5+7), ".png"},
		{"lossless, opaque", decodeWebPFixture(t, webpRed40x20), ".png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ext, img, err := convertWebPBytes(t, tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if ext != tc.ext {
				t.Errorf("converted to %s, want %s", ext, tc.ext)
			}
			if ext == ".png" && tc.ext == ".png" && bytes.Contains(tc.data, []byte("ALPH")) {
				if _, _, _, a := rgba8(img, 7, 5); a < 120 || a > 136 {
					t.Errorf("the translucent pixel has alpha %d, want about 128", a)
				}
				if _, _, _, a := rgba8(img, 0, 0); a != 255 {
					t.Errorf("an opaque pixel has alpha %d", a)
				}
			}
		})
	}
}

// Un WebP con ALPH ya convertido a JPEG no se vuelve a convertir: la ruta por
// contenido se busca con las dos extensiones.
func TestConvertWebPReusesAJPEGFromAlpha(t *testing.T) {
	src := filepath.Join(t.TempDir(), "foto.webp")
	if err := os.WriteFile(src, withALPH(t, decodeWebPFixture(t, webpLossy24x16)), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &DOCXGenerator{tempDir: t.TempDir()}
	first, err := g.convertWebP(src)
	if err != nil || filepath.Ext(first) != ".jpg" {
		t.Fatalf("first conversion = %q, %v", first, err)
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
}

// Un DOCX con fotos WebP con pérdida que traen ALPH opaco pesa mucho menos:
// entran como JPEG y no como PNG. La comparación es contra las mismas fotos
// con un píxel translúcido, que siguen en PNG como entraban todas antes.
//
// testdata/webp_foto_{1,2,3}.webp son fotos sintéticas de 320×240, con
// pérdida y sin ALPH, hechas con Pillow 12.1.1 y libwebp 1.5.0 con este
// programa (`python3 foto.py webp_foto_N.webp N`):
//
//	import math, sys
//	from PIL import Image
//
//	w, h, seed = 320, 240, int(sys.argv[2])
//	img = Image.new("RGB", (w, h))
//	px = img.load()
//	s = seed
//	for y in range(h):
//	    for x in range(w):
//	        s = (s * 1103515245 + 12345) & 0x7FFFFFFF
//	        n = (s >> 16) % 48
//	        px[x, y] = (
//	            int(110 + 90 * math.sin((x + 9 * seed) / 23) * math.cos(y / 31)) + n // 2,
//	            int(60 + 120 * y / h) + n // 3,
//	            int(80 + 70 * math.sin((x + y) / 17)) + n // 2,
//	        )
//	img.save(sys.argv[1], "WEBP", quality=80)
func TestDOCX_OpaqueAlphaPhotosAreSmaller(t *testing.T) {
	build := func(translucent ...int) (int64, map[string][]byte) {
		root := t.TempDir()
		pos := diagnostics.NewPosition(1, 1)
		doc := ast.NewAST(pos)
		doc.FrontMatter = ast.NewFrontMatterNode(pos)
		doc.FrontMatter.Title = "Fotos"
		block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
		block.Title = "S"
		for i := 1; i <= 3; i++ {
			data, err := os.ReadFile(filepath.Join("testdata", fmt.Sprintf("webp_foto_%d.webp", i)))
			if err != nil {
				t.Fatal(err)
			}
			name := fmt.Sprintf("foto%d.webp", i)
			if err := os.WriteFile(filepath.Join(root, name), withALPH(t, data, translucent...), 0o644); err != nil {
				t.Fatal(err)
			}
			block.Elements = append(block.Elements, ast.NewImageElement(diagnostics.NewPosition(2+i, 1), name, name))
		}
		doc.ContentBlocks = append(doc.ContentBlocks, *block)
		out := filepath.Join(t.TempDir(), "fotos.docx")
		if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: root, SourceDir: root}); err != nil {
			t.Fatal(err)
		}
		st, err := os.Stat(out)
		if err != nil {
			t.Fatal(err)
		}
		return st.Size(), docxMediaFiles(t, out)
	}
	opaque, media := build()
	for name, data := range media {
		if !bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
			t.Errorf("%s is not a JPEG", name)
		}
	}
	if len(media) != 3 {
		t.Errorf("%d media files, want 3", len(media))
	}
	translucent, _ := build(0)
	t.Logf("DOCX with 3 photos: %d bytes as JPEG, %d bytes as PNG", opaque, translucent)
	if opaque*3 > translucent {
		t.Errorf("DOCX of %d bytes with opaque photos, want less than a third of the %d bytes as PNG", opaque, translucent)
	}
}
