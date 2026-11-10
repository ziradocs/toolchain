// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/image/webp"
)

// docxWebPMaxPixels es el tope de píxeles de un WebP que el DOCX convierte.
// Un WebP de pocos bytes puede declarar 16383×16383: decodificarlo pide más de
// un gigabyte y varios segundos por cada vez que aparece. Pasado el tope la
// imagen queda como no disponible, con aviso.
const docxWebPMaxPixels = 40_000_000

// docxWebPJPEGQuality es la calidad del JPEG en que entra un WebP con pérdida
// y sin alfa: ya perdió información al comprimirse, así que un PNG sin
// pérdida solo agranda el DOCX (un WebP de 8 MB daba un DOCX de 55 MB).
const docxWebPJPEGQuality = 92

// errWebPTooLarge es el error de un WebP que pasa de docxWebPMaxPixels.
var errWebPTooLarge = errors.New("WebP image too large")

// fileIsWebP dice si el archivo en p es una imagen WebP, por su contenido
// (un contenedor RIFF de tipo WEBP) y no por la extensión. Un error de
// lectura cuenta como "no".
func fileIsWebP(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 12)
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return isWebP(head)
}

// isWebP dice si data empieza como un contenedor RIFF de tipo WEBP.
func isWebP(data []byte) bool {
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

// convertWebP convierte la imagen WebP en webpPath a un formato que Word
// inserta y la escribe en el directorio temporal del DOCX: Word no inserta
// WebP, y docxgo tampoco lo lee. Un WebP con pérdida y sin alfa entra como
// JPEG; uno sin pérdida o con alfa, como PNG, que conserva la transparencia.
// El perfil de color (ICCP) pasa al archivo nuevo. Devuelve su ruta.
//
// Primero lee solo el encabezado: pasado docxWebPMaxPixels no decodifica. Un
// archivo ya convertido (mismo contenido) no se vuelve a convertir. Un WebP
// animado o dañado da error. En todos los casos de error la imagen queda como
// no disponible, como cualquier otra que no se puede leer.
func (g *DOCXGenerator) convertWebP(webpPath string) (string, error) {
	data, err := os.ReadFile(webpPath)
	if err != nil {
		return "", err
	}
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("cannot decode WebP image: %w", err)
	}
	if int64(cfg.Width)*int64(cfg.Height) > docxWebPMaxPixels {
		return "", fmt.Errorf("%w: %d×%d pixels, the limit is %d megapixels", errWebPTooLarge, cfg.Width, cfg.Height, docxWebPMaxPixels/1_000_000)
	}
	// Con pérdida es un chunk "VP8 " (con espacio); el alfa de uno con
	// pérdida va en un chunk ALPH. webp.DecodeConfig no sirve para esto: con
	// un encabezado VP8X da YCbCr también para uno sin pérdida.
	_, lossy := webpChunk(data, "VP8 ")
	_, alpha := webpChunk(data, "ALPH")
	lossyOpaque := lossy && !alpha
	ext := ".png"
	if lossyOpaque {
		ext = ".jpg"
	}
	sum := sha256.Sum256(data)
	outPath := filepath.Join(g.tempDir, "webp_"+hex.EncodeToString(sum[:8])+ext)
	if _, err := os.Stat(outPath); err == nil {
		return outPath, nil
	}
	if err := g.runCtx().Err(); err != nil {
		return "", err
	}
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("cannot decode WebP image: %w", err)
	}
	icc, _ := webpChunk(data, "ICCP")
	var out []byte
	if lossyOpaque {
		out, err = encodeJPEGWithICC(img, icc)
	} else {
		out, err = encodePNGWithICC(img, icc)
	}
	if err != nil {
		return "", fmt.Errorf("cannot convert WebP image: %w", err)
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return "", err
	}
	return outPath, nil
}

// webpChunk es el contenido del primer chunk fourcc del contenedor RIFF de
// un WebP, y si está.
func webpChunk(data []byte, fourcc string) ([]byte, bool) {
	for off := 12; off+8 <= len(data); {
		size := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		start := off + 8
		if size < 0 || size > len(data)-start {
			return nil, false
		}
		if string(data[off:off+4]) == fourcc {
			return data[start : start+size], true
		}
		off = start + size + size%2
	}
	return nil, false
}

// encodePNGWithICC codifica img como PNG con la compresión más rápida (el
// tamaño lo decide sobre todo la imagen) y con el perfil icc en un chunk iCCP
// si lo hay.
func encodePNGWithICC(img image.Image, icc []byte) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	if len(icc) == 0 {
		return out, nil
	}
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	if _, err := zw.Write(icc); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	// Nombre del perfil, su terminador y el método de compresión (0, zlib).
	body := append([]byte("ICC Profile\x00\x00"), z.Bytes()...)
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	chunk = append(chunk, "iCCP"...)
	chunk = append(chunk, body...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	// El iCCP va antes de los datos de la imagen. El IHDR es el primer chunk
	// y termina en el byte 33: firma de 8, largo, tipo, 13 de datos y CRC.
	const ihdrEnd = 8 + 4 + 4 + 13 + 4
	return append(append(append([]byte{}, out[:ihdrEnd]...), chunk...), out[ihdrEnd:]...), nil
}

// encodeJPEGWithICC codifica img como JPEG con el perfil icc en segmentos
// APP2 ICC_PROFILE si lo hay.
func encodeJPEGWithICC(img image.Image, icc []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: docxWebPJPEGQuality}); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	// 65535 del largo del segmento, menos 2 del propio largo y 14 de
	// "ICC_PROFILE\0" con los dos contadores.
	const maxChunk = 65535 - 2 - 14
	count := (len(icc) + maxChunk - 1) / maxChunk
	if count == 0 || count > 255 {
		return out, nil
	}
	var segs []byte
	for i := 0; i < count; i++ {
		part := icc[i*maxChunk : min(len(icc), (i+1)*maxChunk)]
		segs = append(segs, 0xFF, 0xE2)
		segs = binary.BigEndian.AppendUint16(segs, uint16(2+14+len(part)))
		segs = append(segs, "ICC_PROFILE\x00"...)
		segs = append(segs, byte(i+1), byte(count))
		segs = append(segs, part...)
	}
	// Los segmentos van justo después del SOI (FF D8).
	return append(append(append([]byte{}, out[:2]...), segs...), out[2:]...), nil
}
