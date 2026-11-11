// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/binary"
	"testing"
)

// Un chunk que declara un largo cercano a 2^31 no puede tumbar la lectura del
// pHYs de un PNG del DOCX. Con int de 32 bits, body+n daba la vuelta a
// negativo, pasaba el chequeo de límites y el siguiente off quedaba negativo:
// panic al indexar. Con 64 bits no hay vuelta, así que la prueba solo falla
// sin el arreglo con GOARCH=386 (CI la corre así). Es la misma prueba que
// TestPNGHorizontalDPIChunkLengthNearMaxInt32 de core/renderer.
func TestPNGDPIChunkLengthNearMaxInt32(t *testing.T) {
	data := []byte("\x89PNG\r\n\x1a\n")
	chunk := make([]byte, 8+32)
	binary.BigEndian.PutUint32(chunk, 0x7FFFFFF8)
	copy(chunk[4:], "tEXt")
	data = append(data, chunk...)

	if x, y, ok := pngDPI(data); ok {
		t.Fatalf("pngDPI = %v, %v, true; want false for a chunk longer than the file", x, y)
	}
}
