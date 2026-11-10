// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// errWebPAnimation es el error de un WebP animado cuyo contenedor o primer
// cuadro no se puede leer: truncado, con un chunk que se sale del archivo o
// con el cuadro fuera del lienzo.
var errWebPAnimation = errors.New("invalid animated WebP")

// webpFrame es el primer cuadro de un WebP animado, todavía sin decodificar.
type webpFrame struct {
	canvasW, canvasH int // el lienzo, del chunk VP8X
	x, y, w, h       int // el rectángulo del cuadro dentro del lienzo
	alph             []byte
	bitstream        []byte
	lossless         bool // VP8L; si no, VP8 (con pérdida)
}

// webpIsAnimated dice si data es un WebP animado: empieza con un chunk VP8X
// con la marca de animación. x/image/webp no decodifica esos.
func webpIsAnimated(data []byte) bool {
	const animationBit = 1 << 1
	return isWebP(data) && len(data) >= 21 && string(data[12:16]) == "VP8X" && data[20]&animationBit != 0
}

// webpFirstFrame lee el primer cuadro (el primer chunk ANMF) del WebP animado
// data, de lienzo canvasW×canvasH. Es el que muestra un navegador al imprimir
// una página con la imagen: en Word la animación no corre.
//
// Antes valida el contenedor entero (el largo del RIFF y cada chunk dentro
// del archivo), así que un WebP truncado da error aunque el corte caiga
// después del primer cuadro, y el rectángulo del cuadro, que tiene que caber
// en el lienzo: el tope de píxeles se aplica al lienzo, así que un cuadro que
// declare más que él lo saltaría.
func webpFirstFrame(data []byte, canvasW, canvasH int) (*webpFrame, error) {
	if err := validateRIFF(data); err != nil {
		return nil, err
	}
	anmf, ok := webpChunk(data, "ANMF")
	if !ok {
		return nil, fmt.Errorf("%w: no frame", errWebPAnimation)
	}
	// Encabezado del ANMF: X/2, Y/2, ancho-1 y alto-1 en 3 bytes cada uno,
	// la duración en 3 y un byte de marcas (fusión y desecho, que en el
	// primer cuadro sobre un lienzo transparente no cambian nada).
	const anmfHeader = 16
	if len(anmf) < anmfHeader {
		return nil, fmt.Errorf("%w: frame header of %d bytes", errWebPAnimation, len(anmf))
	}
	u24 := func(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }
	f := &webpFrame{
		canvasW: canvasW, canvasH: canvasH,
		x: 2 * u24(anmf[0:3]), y: 2 * u24(anmf[3:6]),
		w: u24(anmf[6:9]) + 1, h: u24(anmf[9:12]) + 1,
	}
	if f.x+f.w > canvasW || f.y+f.h > canvasH {
		return nil, fmt.Errorf("%w: frame %d×%d at (%d,%d) outside the %d×%d canvas", errWebPAnimation, f.w, f.h, f.x, f.y, canvasW, canvasH)
	}
	// Los datos del cuadro: un ALPH opcional y el VP8 o el VP8L.
	body := anmf[anmfHeader:]
	f.alph, _ = riffChunk(body, 0, "ALPH")
	if f.bitstream, ok = riffChunk(body, 0, "VP8 "); !ok {
		if f.bitstream, ok = riffChunk(body, 0, "VP8L"); !ok {
			return nil, fmt.Errorf("%w: frame without image data", errWebPAnimation)
		}
		f.lossless = true
	}
	return f, nil
}

// validateRIFF comprueba que el RIFF data quepa en el archivo y que cada uno
// de sus chunks quepa en el RIFF.
func validateRIFF(data []byte) error {
	if !isWebP(data) {
		return fmt.Errorf("%w: not a WebP", errWebPAnimation)
	}
	riffSize := int(binary.LittleEndian.Uint32(data[4:8]))
	if riffSize < 4 || riffSize > len(data)-8 {
		return fmt.Errorf("%w: RIFF of %d bytes in a file of %d", errWebPAnimation, riffSize, len(data))
	}
	end := 8 + riffSize
	for off := 12; off < end; {
		if off+8 > end {
			return fmt.Errorf("%w: truncated chunk header at byte %d", errWebPAnimation, off)
		}
		size := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if size > end-off-8 {
			return fmt.Errorf("%w: %q chunk of %d bytes past the end", errWebPAnimation, data[off:off+4], size)
		}
		off += 8 + size + size%2
	}
	return nil
}

// lossyOpaque dice si el cuadro entra como JPEG: con pérdida, sin alfa y
// cubriendo todo el lienzo. Uno más chico deja el resto del lienzo
// transparente, así que entra como PNG.
func (f *webpFrame) lossyOpaque() bool {
	return !f.lossless && f.alph == nil && f.covers()
}

// covers dice si el cuadro cubre todo el lienzo.
func (f *webpFrame) covers() bool {
	return f.x == 0 && f.y == 0 && f.w == f.canvasW && f.h == f.canvasH
}

// decode decodifica el cuadro y, si no cubre el lienzo, lo pone en su lugar
// sobre un lienzo transparente (el color de fondo del chunk ANIM es solo una
// sugerencia, y los navegadores dejan el fondo transparente).
//
// x/image/webp no lee animaciones, así que el cuadro se arma como un WebP
// fijo: un VP8X con el tamaño del cuadro (y la marca de alfa solo si trae
// ALPH: x/image rechaza un ALPH sin ella y un VP8 con ella y sin ALPH), el
// ALPH si lo hay y el VP8 o el VP8L. Con el VP8X, x/image comprueba que el
// tamaño del VP8 o del VP8L sea el del cuadro antes de reservar memoria.
func (f *webpFrame) decode() (image.Image, error) {
	const alphaBit = 1 << 4
	vp8x := make([]byte, 10)
	if f.alph != nil {
		vp8x[0] = alphaBit
	}
	putU24 := func(b []byte, v int) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }
	putU24(vp8x[4:7], f.w-1)
	putU24(vp8x[7:10], f.h-1)
	body := appendRIFFChunk([]byte("WEBP"), "VP8X", vp8x)
	if f.alph != nil {
		body = appendRIFFChunk(body, "ALPH", f.alph)
	}
	fourcc := "VP8 "
	if f.lossless {
		fourcc = "VP8L"
	}
	body = appendRIFFChunk(body, fourcc, f.bitstream)
	still := binary.LittleEndian.AppendUint32([]byte("RIFF"), uint32(len(body)))
	still = append(still, body...)

	img, err := webp.Decode(bytes.NewReader(still))
	if err != nil {
		return nil, err
	}
	if f.covers() {
		return img, nil
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, f.canvasW, f.canvasH))
	draw.Draw(canvas, image.Rect(f.x, f.y, f.x+f.w, f.y+f.h), img, img.Bounds().Min, draw.Src)
	return canvas, nil
}

// appendRIFFChunk agrega a dst un chunk RIFF fourcc con data, con el byte de
// relleno si su largo es impar.
func appendRIFFChunk(dst []byte, fourcc string, data []byte) []byte {
	dst = append(dst, fourcc...)
	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(data)))
	dst = append(dst, data...)
	if len(data)%2 == 1 {
		dst = append(dst, 0)
	}
	return dst
}
