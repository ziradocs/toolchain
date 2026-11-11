// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"
	"testing"
)

// `:::grid` sin espacio es un grid, así que ya no sale el aviso de bloque
// especial desconocido que daba como special_block de tipo "grid".
func TestLint_NoSpaceGridIsNotUnknownSpecialBlock(t *testing.T) {
	src := "---\nmode: flex\n---\n# Bloque\n\n:::grid\n:::column\nIzquierda\n:::\n:::column\nDerecha\n:::\n:::\n"
	for _, d := range lintSource(t, src) {
		if strings.Contains(d.Message, "Unknown special block type") {
			t.Errorf("aviso inesperado en la línea %d: %s", d.Position.Line, d.Message)
		}
	}
}
