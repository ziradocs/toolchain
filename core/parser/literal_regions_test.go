// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// El pre-pase de node-id cierra una cerca de código de cuatro backticks donde
// la cierra CodeParser (la primera línea "```"), así que la anotación que sigue
// nombra al párrafo siguiente en vez de quedarse como contenido literal.
func TestNodeID_AfterFourBacktickCodeFence(t *testing.T) {
	src := "# A\n\n````txt\nx\n```\n\n<!-- node-id: Para -->\nTexto.\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse: %s", d.String())
		}
	}
	found := false
	_ = ast.Walk(doc, func(n ast.Node) error {
		if text, ok := n.(*ast.TextElement); ok && text.Content == "Texto." {
			found = text.NodeID == "Para"
		}
		return nil
	})
	if !found {
		t.Errorf("the node-id after the fence must name the paragraph")
	}
}
