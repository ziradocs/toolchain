// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/binary"
	"testing"
)

// Un chunk que declara un largo cercano a 2^31 no puede tumbar la lectura
// del pHYs. Con int de 32 bits, body+n daba la vuelta a negativo, pasaba el
// chequeo de límites y el siguiente off quedaba negativo: data[off:] entraba
// en panic. Con 64 bits no hay vuelta, así que la prueba solo falla sin el
// arreglo con GOARCH=386 (CI la corre así).
func TestPNGHorizontalDPIChunkLengthNearMaxInt32(t *testing.T) {
	data := []byte("\x89PNG\r\n\x1a\n")
	chunk := make([]byte, 8+32)
	binary.BigEndian.PutUint32(chunk, 0x7FFFFFF8)
	copy(chunk[4:], "tEXt")
	data = append(data, chunk...)

	if dpi, ok := pngHorizontalDPI(data); ok {
		t.Fatalf("pngHorizontalDPI = %v, true; want false for a chunk longer than the file", dpi)
	}
}
