// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"encoding/binary"
)

// defaultImageDPI es la resolución que se supone cuando el archivo no la
// declara: la de un px CSS, la misma que usan navegadores y Word.
const defaultImageDPI = 96.0

// imageDPI lee la resolución que declara una imagen: el chunk pHYs de un PNG
// (en píxeles por metro) o el segmento JFIF APP0 de un JPEG (en puntos por
// pulgada o por centímetro). Sin declaración, con una que no es física
// (unidad 0, solo proporción) o fuera de minImageDPI-maxImageDPI, devuelve
// defaultImageDPI en ambos ejes.
func imageDPI(data []byte) (float64, float64) {
	if x, y, ok := pngDPI(data); ok {
		return x, y
	}
	if x, y, ok := jfifDPI(data); ok {
		return x, y
	}
	return defaultImageDPI, defaultImageDPI
}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

func pngDPI(data []byte) (float64, float64, bool) {
	if !bytes.HasPrefix(data, pngSignature) {
		return 0, 0, false
	}
	const inchesPerMeter = 39.3701
	for off := len(pngSignature); off+8 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[off:]))
		typ := string(data[off+4 : off+8])
		body := off + 8
		if n < 0 || body+n > len(data) {
			return 0, 0, false
		}
		switch typ {
		case "pHYs":
			if n < 9 || data[body+8] != 1 { // 1 = metro
				return 0, 0, false
			}
			x := float64(binary.BigEndian.Uint32(data[body:])) / inchesPerMeter
			y := float64(binary.BigEndian.Uint32(data[body+4:])) / inchesPerMeter
			return validDPI(x, y)
		case "IDAT", "IEND": // pHYs va antes del primer IDAT
			return 0, 0, false
		}
		off = body + n + 4 // + CRC
	}
	return 0, 0, false
}

func jfifDPI(data []byte) (float64, float64, bool) {
	// SOI, APP0, largo (2), "JFIF\0", versión (2), unidades, Xdensity, Ydensity
	if len(data) < 18 || data[0] != 0xFF || data[1] != 0xD8 || data[2] != 0xFF || data[3] != 0xE0 ||
		string(data[6:11]) != "JFIF\x00" {
		return 0, 0, false
	}
	x := float64(binary.BigEndian.Uint16(data[14:]))
	y := float64(binary.BigEndian.Uint16(data[16:]))
	switch data[13] {
	case 1: // puntos por pulgada
		return validDPI(x, y)
	case 2: // puntos por centímetro
		return validDPI(x*2.54, y*2.54)
	}
	return 0, 0, false
}

// minImageDPI y maxImageDPI acotan la resolución declarada que se toma en
// serio. Fuera de ese rango la metadata casi siempre es basura (un pHYs en 0
// o en 0xffffffff, el 1 dpi que escriben algunos editores) y llevaría una
// imagen a metros o a una fracción de píxel; se ignora y vale 96.
const (
	minImageDPI = 72.0
	maxImageDPI = 600.0
)

func validDPI(x, y float64) (float64, float64, bool) {
	if x < minImageDPI || y < minImageDPI || x > maxImageDPI || y > maxImageDPI {
		return 0, 0, false
	}
	return x, y, true
}
