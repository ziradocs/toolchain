// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"testing"
	"time"
)

// testChunk es un chunk RIFF fourcc con data, con su byte de relleno si pad y
// el largo es impar.
func testChunk(fourcc string, data []byte, pad bool) []byte {
	out := binary.LittleEndian.AppendUint32([]byte(fourcc), uint32(len(data)))
	out = append(out, data...)
	if pad && len(data)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// testRIFF es un contenedor RIFF con body (que empieza con "WEBP").
func testRIFF(body []byte) []byte {
	return append(binary.LittleEndian.AppendUint32([]byte("RIFF"), uint32(len(body))), body...)
}

// testTopChunks son los chunks de primer nivel de un WebP válido.
func testTopChunks(t testing.TB, data []byte) []riffSub {
	t.Helper()
	var out []riffSub
	for off := 12; off+8 <= len(data); {
		size := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		out = append(out, riffSub{string(data[off : off+4]), data[off+8 : off+8+size]})
		off += 8 + size + size%2
	}
	return out
}

// animParts son las piezas de webpAnimLossy16x12 (VP8X, ANIM, dos ANMF; el
// segundo cuadro azul) y el ALPH de webpAnimAlpha16x12, para armar variantes
// del primer ANMF.
type animParts struct {
	top        []riffSub
	hdr        []byte // encabezado de 16 bytes del primer ANMF
	vp8, alph  []byte
	vp8x, anim []byte
}

func newAnimParts(t testing.TB) animParts {
	t.Helper()
	top := testTopChunks(t, decodeWebPFixture(t, webpAnimLossy16x12))
	alphaTop := testTopChunks(t, decodeWebPFixture(t, webpAnimAlpha16x12))
	vp8, ok := riffChunk(top[2].data, 16, "VP8 ")
	alph, ok2 := riffChunk(alphaTop[2].data, 16, "ALPH")
	if len(top) != 4 || !ok || !ok2 {
		t.Fatal("unexpected fixture layout")
	}
	return animParts{top: top, hdr: top[2].data[:16], vp8: vp8, alph: alph, vp8x: top[0].data, anim: top[1].data}
}

// withFirst es el WebP con el primer ANMF cambiado por payload (con su
// relleno si pad) y extra al final del RIFF.
func (p animParts) withFirst(payload []byte, pad bool, extra ...byte) []byte {
	body := []byte("WEBP")
	for i, c := range p.top {
		if i == 2 {
			body = append(body, testChunk("ANMF", payload, pad)...)
			continue
		}
		body = append(body, testChunk(c.fourcc, c.data, true)...)
	}
	return testRIFF(append(body, extra...))
}

// frame es el encabezado del primer ANMF seguido de los chunks dados.
func (p animParts) frame(chunks ...[]byte) []byte {
	return append(append([]byte{}, p.hdr...), bytes.Join(chunks, nil)...)
}

// webpFirstFrame es tan estricto como libwebp. Cada variante se probó con
// Pillow 12.1.1 y libwebp 1.5.0 (WebPAnimDecoder): las que libwebp abre se
// convierten, y las que rechaza dan error. Un ANMF sin datos de imagen se
// salta, como en libwebp, y entra el cuadro siguiente (azul).
func TestWebPFirstFrameIsAsStrictAsLibwebp(t *testing.T) {
	p := newAnimParts(t)
	ck := func(fourcc string, data []byte) []byte { return testChunk(fourcc, data, true) }
	odd := p.frame(testChunk("VP8 ", append(append([]byte{}, p.vp8...), 0), false))
	// Un WebP de un solo cuadro, con el ANMF al final.
	single := func(anmf []byte, pad bool) []byte {
		return testRIFF(bytes.Join([][]byte{[]byte("WEBP"), ck("VP8X", p.vp8x), ck("ANIM", p.anim), testChunk("ANMF", anmf, pad)}, nil))
	}
	afterRIFF := append(testRIFF(bytes.Join([][]byte{[]byte("WEBP"), ck("VP8X", p.vp8x), ck("ANIM", p.anim)}, nil)), ck("ANMF", p.top[2].data)...)
	for _, tc := range []struct {
		name string
		data []byte
		want string // "red", "blue" o "" (error)
	}{
		{"as encoded", p.withFirst(p.top[2].data, true), "red"},
		{"unknown chunk after the bitstream", p.withFirst(p.frame(ck("VP8 ", p.vp8), ck("XYZW", []byte("ab"))), true), "red"},
		{"ALPH, VP8 and an unknown chunk", p.withFirst(p.frame(ck("ALPH", p.alph), ck("VP8 ", p.vp8), ck("XYZW", []byte("ab"))), true), "red"},
		{"odd ANMF padded at the top level", p.withFirst(odd, true), "red"},
		{"odd last ANMF padded", single(odd, true), "red"},
		{"odd last top-level chunk padded", p.withFirst(p.top[2].data, true, ck("XYZW", []byte("abc"))...), "red"},
		{"empty ANMF", p.withFirst(p.hdr, true), "blue"},
		{"ANMF with only an unknown chunk", p.withFirst(p.frame(ck("XYZW", []byte("ab"))), true), "blue"},

		{"unknown chunk before the bitstream", p.withFirst(p.frame(ck("XYZW", []byte("ab")), ck("VP8 ", p.vp8)), true), ""},
		{"unknown chunk before ALPH", p.withFirst(p.frame(ck("XYZW", []byte("ab")), ck("ALPH", p.alph), ck("VP8 ", p.vp8)), true), ""},
		{"ALPH after VP8", p.withFirst(p.frame(ck("VP8 ", p.vp8), ck("ALPH", p.alph)), true), ""},
		{"ALPH after VP8 and an unknown chunk", p.withFirst(p.frame(ck("VP8 ", p.vp8), ck("XYZW", []byte("ab")), ck("ALPH", p.alph)), true), ""},
		{"two ALPH", p.withFirst(p.frame(ck("ALPH", p.alph), ck("ALPH", p.alph), ck("VP8 ", p.vp8)), true), ""},
		{"two VP8", p.withFirst(p.frame(ck("VP8 ", p.vp8), ck("VP8 ", p.vp8)), true), ""},
		{"VP8L after ALPH and VP8", p.withFirst(p.frame(ck("ALPH", p.alph), ck("VP8 ", p.vp8), ck("VP8L", []byte{0x2f, 0, 0, 0, 0})), true), ""},
		{"ALPH without a bitstream", p.withFirst(p.frame(ck("ALPH", p.alph)), true), ""},
		{"3 stray bytes in the ANMF", p.withFirst(p.frame(ck("VP8 ", p.vp8), []byte("xyz")), true), ""},
		{"4 stray bytes in the ANMF", p.withFirst(p.frame(ck("VP8 ", p.vp8), []byte("wxyz")), true), ""},
		{"ANMF header of 10 bytes", p.withFirst(p.hdr[:10], true), ""},
		{"odd last ANMF without padding", single(odd, false), ""},
		{"odd last top-level chunk without padding", p.withFirst(p.top[2].data, true, testChunk("XYZW", []byte("abc"), false)...), ""},
		{"4 stray bytes in the RIFF", p.withFirst(p.top[2].data, true, []byte("wxyz")...), ""},
		{"6 stray bytes in the RIFF", p.withFirst(p.top[2].data, true, []byte("uvwxyz")...), ""},
		{"VP8 outside a frame", p.withFirst(p.top[2].data, true, ck("VP8 ", p.vp8)...), ""},
		{"ANMF after the end of the RIFF", afterRIFF, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ext, img, err := convertWebPBytes(t, tc.data)
			if tc.want == "" {
				if !errors.Is(err, errWebPAnimation) {
					t.Errorf("err = %v, want errWebPAnimation (libwebp rejects it)", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("libwebp opens it, but: %v", err)
			}
			r, g, b, _ := rgba8(img, 8, 6)
			got := "other"
			switch {
			case r > 200 && g < 60 && b < 60:
				got = "red"
			case b > 200 && r < 60 && g < 60:
				got = "blue"
			}
			if got != tc.want {
				t.Errorf("converted to %s with (8,6) = %d,%d,%d, want %s", ext, r, g, b, tc.want)
			}
		})
	}
}

// Un chunk que declara 0xFFFFFFF8 o 0xFFFFFFF0 bytes da error y no cuelga.
// Con int de 32 bits ese largo sale negativo: sin la aritmética en uint64,
// el recorrido no avanzaba o retrocedía. Corre igual con GOARCH=386.
func TestWebPFirstFrameHugeChunkSize(t *testing.T) {
	p := newAnimParts(t)
	for _, size := range []uint32{0xFFFFFFF8, 0xFFFFFFF0, 0xFFFFFFFF, 0x80000000} {
		bad := binary.LittleEndian.AppendUint32([]byte("XYZW"), size)
		for name, data := range map[string][]byte{
			"top level":   p.withFirst(p.top[2].data, true, append(bad, make([]byte, 8)...)...),
			"in the ANMF": p.withFirst(p.frame(testChunk("VP8 ", p.vp8, true), bad, make([]byte, 8)), true),
		} {
			done := make(chan error, 1)
			go func() {
				_, err := webpFirstFrame(data, 16, 12)
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, errWebPAnimation) {
					t.Errorf("%s, size %#x: err = %v, want errWebPAnimation", name, size, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("%s, size %#x: webpFirstFrame hangs", name, size)
			}
		}
	}
}

// Un cuadro desplazado sobre un lienzo más grande que la caja se pone
// directo sobre el lienzo reducido, sin reservar uno del tamaño completo: el
// cuadro de 16×12 en (10,8) de un lienzo de 40×30 queda, a la mitad, de 8×6
// en (5,4).
func TestWebPFrameDecodeComposesOnTheReducedCanvas(t *testing.T) {
	data := animatedWebP(t, 40, 30, 10, 8, 16, 12)
	f, err := webpFirstFrame(data, 40, 30)
	if err != nil {
		t.Fatal(err)
	}
	img, err := f.decode(20, 15)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 20 || b.Dy() != 15 {
		t.Fatalf("composed %dx%d, want the 20x15 reduced canvas", b.Dx(), b.Dy())
	}
	for _, pt := range []image.Point{{5, 4}, {12, 9}, {8, 6}} {
		if !isRed(img, pt.X, pt.Y, 250, 255) {
			r, g, b, a := rgba8(img, pt.X, pt.Y)
			t.Errorf("(%d,%d) inside the frame is %d,%d,%d,%d", pt.X, pt.Y, r, g, b, a)
		}
	}
	for _, pt := range []image.Point{{4, 4}, {5, 3}, {13, 9}, {12, 10}, {0, 0}, {19, 14}} {
		if _, _, _, a := rgba8(img, pt.X, pt.Y); a != 0 {
			t.Errorf("(%d,%d) outside the frame has alpha %d", pt.X, pt.Y, a)
		}
	}
	// Sin caja (0×0) el lienzo queda de su tamaño.
	if img, err := f.decode(0, 0); err != nil || img.Bounds().Dx() != 40 {
		t.Errorf("without a box: %v, %v", img.Bounds(), err)
	}
}
