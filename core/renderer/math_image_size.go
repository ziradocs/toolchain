// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"io"
	"net/url"
	"os"

	"go.ziradocs.com/core/v2/util"
)

// mathImageHeaderLimit acota lo que se lee de un render previo para medirlo:
// IHDR y pHYs van antes de IDAT, al principio del archivo.
const mathImageHeaderLimit = 1 << 20

// mathImageCSSWidth es el ancho, en px CSS, al que va el render previo de una
// ecuación (math-image-v1) cuando es un PNG local con pHYs: px × 96 / dpi.
// Chromium ignora pHYs, así que sin esto un render a devicePixelRatio 2 salía
// al doble en HTML y PDF, mientras el DOCX (que sí lo respeta) lo ponía a su
// tamaño. Sin pHYs, remota, fuera de AssetRoot o ilegible: ok=false y el
// <img> queda sin ancho, como antes. Solo lee la cabecera del archivo, ya
// confinado a AssetRoot. No aplica a las imágenes de bloque normales.
func mathImageCSSWidth(source string, ctx *RenderContext) (float64, bool) {
	if ctx == nil || ctx.AssetRoot == "" || source == "" {
		return 0, false
	}
	u, err := url.Parse(source)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return 0, false
	}
	resolved, err := util.ResolveConfinedPathFrom(ctx.AssetRoot, ctx.AssetBaseDir, LocalPathFromURL(source))
	if err != nil {
		return 0, false
	}
	f, err := os.Open(resolved)
	if err != nil {
		return 0, false
	}
	defer func() { _ = f.Close() }()
	head, err := io.ReadAll(io.LimitReader(f, mathImageHeaderLimit))
	if err != nil {
		return 0, false
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(head))
	if err != nil || cfg.Width <= 0 {
		return 0, false
	}
	dpi, ok := pngHorizontalDPI(head)
	if !ok {
		return 0, false
	}
	return float64(cfg.Width) * 96 / dpi, true
}

// pngHorizontalDPI lee la densidad horizontal del chunk pHYs de un PNG, si la
// declara en píxeles por metro (unidad 1). Se detiene en IDAT: pHYs no puede
// ir después.
func pngHorizontalDPI(data []byte) (float64, bool) {
	const signature = "\x89PNG\r\n\x1a\n"
	if !bytes.HasPrefix(data, []byte(signature)) {
		return 0, false
	}
	for off := len(signature); off+8 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[off:]))
		typ := string(data[off+4 : off+8])
		body := off + 8
		if n < 0 || body+n > len(data) {
			return 0, false
		}
		switch typ {
		case "pHYs":
			if n < 9 || data[body+8] != 1 {
				return 0, false
			}
			ppm := binary.BigEndian.Uint32(data[body:])
			if ppm == 0 {
				return 0, false
			}
			return float64(ppm) * 0.0254, true
		case "IDAT", "IEND":
			return 0, false
		}
		off = body + n + 4 // datos + CRC
	}
	return 0, false
}
