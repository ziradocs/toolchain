// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// nodeIDApplied dice si algún nodo del AST de src quedó con el node-id id, y si
// el texto de la anotación sigue en el AST (no se quitó por estar en una región
// literal).
func nodeIDApplied(t *testing.T, src, id string) (applied, keptAsText bool) {
	t.Helper()
	doc, _ := New(util.NewNoop()).ParseDocument(src, "")
	_ = ast.Walk(doc, func(n ast.Node) error {
		if node, ok := n.(ast.IdentityNode); ok && node.GetNodeID() == id {
			applied = true
		}
		switch e := n.(type) {
		case *ast.CodeElement:
			keptAsText = keptAsText || strings.Contains(e.Content, "node-id: "+id)
		case *ast.CodeGroupElement:
			for _, b := range e.CodeBlocks {
				keptAsText = keptAsText || strings.Contains(b.Content, "node-id: "+id)
			}
		case *ast.TextElement:
			keptAsText = keptAsText || strings.Contains(e.Content, "node-id: "+id)
		}
		return nil
	})
	return applied, keptAsText
}

// El pre-pase de `<!-- node-id -->` corre antes de parsear y conserva sus
// reglas de regiones literales (las de main). Estas pruebas las fijan para que
// mover la regla de @include al parser no las cambie.
func TestNodeIDPrePass_LiteralRegions(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		literal   bool
	}{
		// En flex, una línea `CODE …` abre un cuerpo literal para el pre-pase
		// (regla histórica, flexCodeHeaders).
		{"flex CODE header", "# A\n\nCODE js\n  <!-- node-id: n1 -->\n  x\n", true},
		// Un ::: dentro de una pestaña no cierra el code-group.
		{"::: inside a code-group tab", "# A\n\n:::code-group\n```js [a]\n:::\n<!-- node-id: n1 -->\n```\n:::\n", true},
		// Una cerca de cuatro solo cierra con cuatro para el pre-pase.
		{"four-backtick fence", "# A\n\n````js\nq\n```\n<!-- node-id: n1 -->\nfin\n````\n", true},
		// Fuera de toda región, la anotación se aplica.
		{"outside", "# A\n\n<!-- node-id: n1 -->\nTexto.\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			applied, kept := nodeIDApplied(t, tc.src, "n1")
			if tc.literal && (applied || !kept) {
				t.Errorf("the annotation is inside a literal region: applied=%v, kept as text=%v", applied, kept)
			}
			if !tc.literal && (!applied || kept) {
				t.Errorf("the annotation must apply: applied=%v, kept as text=%v", applied, kept)
			}
		})
	}
}
