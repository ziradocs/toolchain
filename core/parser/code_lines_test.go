// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strconv"
	"strings"
	"testing"
)

// recordedCodeLines son los números de línea (1-based) que codeLiteralLines marca.
func recordedCodeLines(src string) []int {
	var out []int
	for i, c := range codeLiteralLines(src) {
		if c {
			out = append(out, i+1)
		}
	}
	return out
}

// Las líneas que el parser lee como código cuentan sobre la fuente tal como la
// escribió el autor: dos anotaciones <!-- node-id --> antes de una cerca no
// corren sus líneas (lineMap), y el cuerpo de una columna tipada de un grid
// strict también avisa su código.
func TestCodeLiteralLines_SourcePositions(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      string
	}{
		{"node-id annotations before a fence", "# A\n\n<!-- node-id: a -->\nTexto.\n\n<!-- node-id: b -->\n```\n@include x\n```\n", "7 8 9"},
		{"strict typed column", "---\nmode: strict\ntitle: T\n---\n\nSECTION \"A\"\n  <<grid>>\n  <<column typed>>\n    CODE js\n      @include x\n  <<column>>\n  Right.\n  <<end>>\n", "9 10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, l := range recordedCodeLines(tc.src) {
				got = append(got, strconv.Itoa(l))
			}
			if strings.Join(got, " ") != tc.want {
				t.Errorf("code lines %v, want %s", got, tc.want)
			}
		})
	}
}
