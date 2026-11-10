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
// cuadro no se puede leer: truncado, con un chunk que se sale del RIFF, con
// los chunks del cuadro fuera de orden o con el cuadro fuera del lienzo.
var errWebPAnimation = errors.New("invalid animated WebP")

// webpFrame es el primer cuadro de un WebP animado, todavía sin decodificar.
type webpFrame struct {
	canvasW, canvasH int // el lienzo, del chunk VP8X
	x, y, w, h       int // el rectángulo del cuadro dentro del lienzo
	alph             []byte
	bitstream        []byte
	lossless         bool   // VP8L; si no, VP8 (con pérdida)
	icc              []byte // el ICCP del contenedor, si lo hay
}

// webpIsAnimated dice si data es un WebP animado: empieza con un chunk VP8X
// con la marca de animación. x/image/webp no decodifica esos.
func webpIsAnimated(data []byte) bool {
	const animationBit = 1 << 1
	return isWebP(data) && len(data) >= 21 && string(data[12:16]) == "VP8X" && data[20]&animationBit != 0
}

// webpFirstFrame lee el primer cuadro del WebP animado data, de lienzo
// canvasW×canvasH. Es el que muestra un navegador al imprimir una página con
// la imagen: en Word la animación no corre.
//
// Es tan estricto como libwebp (lo que libwebp rechaza da error aquí), y
// revisa así todos los cuadros aunque solo use el primero:
//   - El RIFF tiene que caber en el archivo y cada chunk, con su byte de
//     relleno, en el RIFF; solo se lee lo que está dentro del RIFF. Así un
//     WebP truncado da error aunque el corte caiga después del primer cuadro.
//   - Un ALPH, VP8 o VP8L fuera de un ANMF es un error.
//   - Dentro del ANMF va un ALPH opcional y luego el VP8 o el VP8L (un VP8L
//     no lleva ALPH); otro ALPH, VP8 o VP8L después, o un chunk desconocido
//     antes, es un error. Un ANMF sin ninguno de los tres se salta, como en
//     libwebp, y el primer cuadro es el siguiente.
//   - El cuadro tiene que caber en el lienzo: el tope de píxeles se aplica al
//     lienzo, así que un cuadro que declare más que él lo saltaría.
func webpFirstFrame(data []byte, canvasW, canvasH int) (*webpFrame, error) {
	if !isWebP(data) {
		return nil, fmt.Errorf("%w: not a WebP", errWebPAnimation)
	}
	riffSize := uint64(binary.LittleEndian.Uint32(data[4:8]))
	if riffSize < 4 || riffSize > uint64(len(data)-8) {
		return nil, fmt.Errorf("%w: RIFF of %d bytes in a file of %d", errWebPAnimation, riffSize, len(data))
	}
	// Una sola pasada: se recuerda el primer ICCP y el primer cuadro, y se
	// sigue hasta el final para revisar todos los chunks.
	var (
		icc   []byte
		first webpFrame
		found bool
	)
	for top := (riffCursor{data: data[12 : 8+int(riffSize)]}); top.more(); {
		id, body, err := top.next()
		if err != nil {
			return nil, err
		}
		switch string(id[:]) {
		case "ALPH", "VP8 ", "VP8L":
			return nil, fmt.Errorf("%w: %q chunk outside a frame", errWebPAnimation, string(id[:]))
		case "ICCP":
			if icc == nil {
				icc = body
			}
		case "ANMF":
			f, ok, err := parseANMF(body, canvasW, canvasH)
			if err != nil {
				return nil, err
			}
			if ok && !found {
				first, found = f, true
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("%w: no frame", errWebPAnimation)
	}
	first.icc = icc
	return &first, nil
}

// parseANMF lee el cuadro del ANMF anmf. ok es false, sin error, si el ANMF
// no trae datos de imagen (libwebp lo salta). El cuadro va por valor: con
// muchos ANMF, solo se reserva el primero.
func parseANMF(anmf []byte, canvasW, canvasH int) (f webpFrame, ok bool, err error) {
	// Encabezado: X/2, Y/2, ancho-1 y alto-1 en 3 bytes cada uno, la duración
	// en 3 y un byte de marcas (fusión y desecho, que en el primer cuadro
	// sobre un lienzo transparente no cambian nada).
	const anmfHeader = 16
	if len(anmf) < anmfHeader {
		return webpFrame{}, false, fmt.Errorf("%w: frame header of %d bytes", errWebPAnimation, len(anmf))
	}
	u24 := func(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }
	f = webpFrame{
		canvasW: canvasW, canvasH: canvasH,
		x: 2 * u24(anmf[0:3]), y: 2 * u24(anmf[3:6]),
		w: u24(anmf[6:9]) + 1, h: u24(anmf[9:12]) + 1,
	}
	// Lo leído hasta ahora: nada, el ALPH, el bitstream, o un chunk
	// desconocido antes de cualquier dato de imagen.
	const (
		start = iota
		afterALPH
		afterBitstream
		noImage
	)
	state := start
	// El último subchunk puede venir sin relleno: el del ANMF hace de suyo.
	for c := (riffCursor{data: anmf[anmfHeader:], unpaddedLast: true}); c.more(); {
		id, body, err := c.next()
		if err != nil {
			return webpFrame{}, false, err
		}
		fourcc := string(id[:])
		isImage := fourcc == "ALPH" || fourcc == "VP8 " || fourcc == "VP8L"
		switch {
		case state == afterBitstream:
			if isImage {
				return webpFrame{}, false, fmt.Errorf("%w: %q chunk after the frame bitstream", errWebPAnimation, string(id[:]))
			}
		case state == start && fourcc == "ALPH":
			f.alph, state = body, afterALPH
		case (state == start || state == afterALPH) && fourcc == "VP8 ":
			f.bitstream, state = body, afterBitstream
		case state == start && fourcc == "VP8L": // un VP8L no lleva ALPH
			f.bitstream, f.lossless, state = body, true, afterBitstream
		case isImage || state == afterALPH:
			return webpFrame{}, false, fmt.Errorf("%w: frame chunks out of order", errWebPAnimation)
		default:
			state = noImage
		}
	}
	switch state {
	case afterALPH:
		return webpFrame{}, false, fmt.Errorf("%w: frame chunks out of order", errWebPAnimation)
	case start, noImage:
		return webpFrame{}, false, nil
	}
	if f.x+f.w > canvasW || f.y+f.h > canvasH {
		return webpFrame{}, false, fmt.Errorf("%w: frame %d×%d at (%d,%d) outside the %d×%d canvas", errWebPAnimation, f.w, f.h, f.x, f.y, canvasW, canvasH)
	}
	return f, true, nil
}

// riffCursor recorre una lista de chunks RIFF sin armarla: con millones de
// chunks vacíos, una lista pesaría varias veces el archivo. Cada chunk, con
// su byte de relleno si su largo es impar, tiene que caber en data; con
// unpaddedLast el último puede venir sin él. Unos bytes sobrantes que no
// alcanzan para un encabezado de chunk son un error. Los largos se comparan
// en uint64: con int de 32 bits, uno como 0xFFFFFFF8 saldría negativo.
type riffCursor struct {
	data         []byte
	off          int
	unpaddedLast bool
}

// more dice si quedan bytes por recorrer.
func (c *riffCursor) more() bool { return c.off < len(c.data) }

// next es el siguiente chunk: su fourcc y su contenido, que apunta a data.
func (c *riffCursor) next() (id [4]byte, body []byte, err error) {
	data, off := c.data, c.off
	if len(data)-off < 8 {
		return id, nil, fmt.Errorf("%w: %d stray bytes at the end of a chunk list", errWebPAnimation, len(data)-off)
	}
	size := uint64(binary.LittleEndian.Uint32(data[off+4 : off+8]))
	rest := uint64(len(data) - off - 8)
	padded := size + size%2
	if padded > rest && (!c.unpaddedLast || size != rest) {
		return id, nil, fmt.Errorf("%w: %q chunk of %d bytes past the end", errWebPAnimation, data[off:off+4], size)
	}
	copy(id[:], data[off:off+4])
	c.off = off + 8 + int(min(padded, rest))
	return id, data[off+8 : off+8+int(size)], nil
}

// covers dice si el cuadro cubre todo el lienzo. Uno más chico deja el resto
// del lienzo transparente, así que entra como PNG.
func (f *webpFrame) covers() bool {
	return f.x == 0 && f.y == 0 && f.w == f.canvasW && f.h == f.canvasH
}

// decode decodifica el cuadro y, si no cubre el lienzo, lo pone en su lugar
// sobre un lienzo transparente (el color de fondo del chunk ANIM es solo una
// sugerencia, y los navegadores dejan el fondo transparente). Si el lienzo no
// cabe en maxW×maxH, el lienzo transparente ya es el reducido (ver
// fitSize) y el cuadro se reduce sobre él: no se reserva uno del tamaño
// completo. Un cuadro que cubre el lienzo sale tal cual y lo reduce
// downscaleToFit.
//
// x/image/webp no lee animaciones, así que el cuadro se arma como un WebP
// fijo: un VP8X con el tamaño del cuadro (y la marca de alfa solo si trae
// ALPH: x/image rechaza un ALPH sin ella y un VP8 con ella y sin ALPH), el
// ALPH si lo hay y el VP8 o el VP8L. Con el VP8X, x/image reserva el alfa
// (w×h bytes) con el tamaño del cuadro, que cabe en el lienzo, y comprueba
// que el tamaño del VP8 o del VP8L sea el del cuadro antes de decodificarlo.
func (f *webpFrame) decode(maxW, maxH int) (image.Image, error) {
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
	nw, nh, scaled := fitSize(f.canvasW, f.canvasH, maxW, maxH)
	canvas := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	if !scaled {
		draw.Draw(canvas, image.Rect(f.x, f.y, f.x+f.w, f.y+f.h), img, img.Bounds().Min, draw.Src)
		return canvas, nil
	}
	sx, sy := float64(nw)/float64(f.canvasW), float64(nh)/float64(f.canvasH)
	at := func(v int, s float64) int { return int(float64(v)*s + 0.5) }
	// Un cuadro muy chico puede quedar de 0 px al reducirse: se le deja 1,
	// dentro del lienzo (en el borde derecho o inferior, el último).
	span := func(v, n int, s float64, size int) (int, int) {
		lo, hi := at(v, s), at(v+n, s)
		if lo == hi {
			if hi < size {
				hi++
			} else {
				lo--
			}
		}
		return lo, hi
	}
	x0, x1 := span(f.x, f.w, sx, nw)
	y0, y1 := span(f.y, f.h, sy, nh)
	draw.CatmullRom.Scale(canvas, image.Rect(x0, y0, x1, y1), img, img.Bounds(), draw.Src, nil)
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
