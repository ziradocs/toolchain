// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

// FuzzTableRowsMatchMain es diferencial contra el parser de filas de main
// (oracleParseYAMLTable): para todo cuerpo de TABLE que main lee sin
// diagnósticos y sin perder ninguna línea en silencio, el parser nuevo tiene
// que leer exactamente lo mismo (cabecera, filas, caption, label, líneas
// consumidas) y no dar ningún diagnóstico.
func FuzzTableRowsMatchMain(f *testing.F) {
	for _, seed := range []string{
		"    headers: [\"Tarea\", \"Estado\"]\n    rows:\n      [Revisar a|b, ok]",
		"    headers: [\"x\", \"y\"]\n    rows:\n      [a | b, c]\n      [d, e]",
		"    Item | Ref\n    [x] Monitor 27\" | [1]\n    [ ] Teclado | [2]",
		"    headers: [\"x\", \"y\"]\n    rows:\n      [\"c\\\"d\", \"e | f\"]\n      [\"g\", \"h\"]",
		"    Doc | N\n    [Docs](https://x.org) | 3\n    [x] listo | 4\n    [1] | cita\n    [Nombre] | valor",
		"    headers: [\"x\", \"y\"]\n    rows:\n      [\"a | b\", \"c\"]\n      [\"d\", \"e\"]",
		"    headers: [\"x\", \"y\"]\n    rows: [\n      [\"a\", \"b\"],\n      [\"c\", \"d\"]\n    ]\n    caption: \"C\"",
		"    headers: [\"x\", \"y\"]\n    rows:\n      [\"1\", \"2\"]\n\n      [x] | y\n    label: t",
		"    | a | b |\n    |---|---|\n    | 1 | 2 |",
		"    headers: [\"x\"]\n    rows:\n      [\"1\"]\n    caption: \"c\"\n    [nota al pie]",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		for _, mode := range []string{"strict", "flex"} {
			ctx := &ParseContext{Mode: mode, Lines: strings.Split(body, "\n")}
			pos := diagnostics.NewPosition(1, 1)
			h0, r0, c0, l0, _, _, _, d0, n0, dropped := oracleParseYAMLTable(ctx, 0, pos)
			if dropped || len(d0) != 0 {
				continue
			}
			h1, r1, c1, l1, _, _, _, d1, n1 := (&TableParser{}).parseYAMLTable(ctx, 0, pos)
			if len(d1) != 0 {
				t.Fatalf("%s: main read %q without diagnostics, the new parser gave %v", mode, body, d1)
			}
			if !reflect.DeepEqual(h0, h1) || !reflect.DeepEqual(r0, r1) || c0 != c1 || l0 != l1 || n0 != n1 {
				t.Fatalf("%s: %q\n main: headers %q rows %q caption %q label %q consumed %d\n  new: headers %q rows %q caption %q label %q consumed %d",
					mode, body, h0, r0, c0, l0, n0, h1, r1, c1, l1, n1)
			}
		}
	})
}
