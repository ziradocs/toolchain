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
		res := (&CodeParser{}).Parse(ctx, 0)
		code, ok := res.Element.(*ast.CodeElement)
		if !ok || code.Content != tc.content || res.ConsumedLines != tc.consumed {
			t.Errorf("%q: content %q consumed %d, want %q %d", tc.src, code.Content, res.ConsumedLines, tc.content, tc.consumed)
		}
	}
}
