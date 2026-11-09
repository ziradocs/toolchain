// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// Una cerca de código de cuatro backticks cierra en su línea de cuatro
// backticks (CommonMark), no en un ``` interno: el bloque guarda el ``` como
// contenido, y el pre-pase de node-id coincide, así que la anotación de
// después nombra al párrafo siguiente.
func TestFourBacktickCodeFence_ParserAndNodeIDAgree(t *testing.T) {
	src := "# A\n\n````md\n```go\nx := 1\n```\n````\n\n<!-- node-id: Para -->\nTexto.\n\n## B\n\nfin\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse: %s", d.String())
		}
	}
	var code *ast.CodeElement
	named, heading := false, false
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch e := n.(type) {
		case *ast.CodeElement:
			code = e
		case *ast.TextElement:
			if e.Content == "Texto." {
				named = e.NodeID == "Para"
			}
			if e.IsRawHTML && strings.Contains(e.Content, "B") {
				heading = true
			}
		}
		return nil
	})
	if code == nil || code.Content != "```go\nx := 1\n```" || code.Language != "md" {
		t.Fatalf("want one md code block holding the inner fence, got %#v", code)
	}
	if !named {
		t.Errorf("the node-id after the fence must name the paragraph")
	}
	if !heading {
		t.Errorf("the ## after the fence must still be a heading")
	}
}
