// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// La regla de cierre de una cerca flex es la de main: solo una línea ```
// exacta cierra, también una cerca abierta con cuatro backticks, y una línea
// de cuatro backticks dentro de una cerca de tres es contenido.
func TestCodeParser_FenceCloseRule(t *testing.T) {
	for _, tc := range []struct {
		src, content string
		consumed     int
	}{
		{"```\na\n````\nb\n```\nfin", "a\n````\nb", 5},
		{"````js\nq\n```\nfin", "q", 3},
		{"```\na\n```x\n```", "a\n```x", 4},
	} {
		ctx := &ParseContext{Mode: "flex", Lines: strings.Split(tc.src, "\n")}
		var recorded [][2]int
		ctx.RecordCode = func(line, n int) { recorded = append(recorded, [2]int{line, n}) }
		res := (&CodeParser{}).Parse(ctx, 0)
		code, ok := res.Element.(*ast.CodeElement)
		if !ok || code.Content != tc.content || res.ConsumedLines != tc.consumed {
			t.Errorf("%q: content %q consumed %d, want %q %d", tc.src, code.Content, res.ConsumedLines, tc.content, tc.consumed)
		}
		if len(recorded) != 1 || recorded[0] != [2]int{1, tc.consumed} {
			t.Errorf("%q: RecordCode got %v, want the %d consumed lines from line 1", tc.src, recorded, tc.consumed)
		}
	}
}

// Un code-group avisa a RecordCode su cabecera y cada pestaña completa, no las
// líneas sueltas entre pestañas.
func TestCodeGroupParser_RecordsHeaderAndTabs(t *testing.T) {
	src := ":::code-group\nsuelta\n```js [a]\nx\n```\n:::"
	ctx := &ParseContext{Mode: "flex", Lines: strings.Split(src, "\n"), LineOffset: 10}
	var recorded [][2]int
	ctx.RecordCode = func(line, n int) { recorded = append(recorded, [2]int{line, n}) }
	(&CodeGroupParser{}).Parse(ctx, 0)
	if want := [][2]int{{11, 1}, {13, 3}}; len(recorded) != 2 || recorded[0] != want[0] || recorded[1] != want[1] {
		t.Errorf("RecordCode got %v, want %v", recorded, want)
	}
}
