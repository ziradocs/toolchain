// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// El nodo tipado y su forma legada renderizan byte a byte igual, con y sin
// variables, y la bajada no modifica el AST original.
func TestTypedHeadingRendersLikeLegacy(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"who": "<b>team</b>"}
	for _, text := range []string{"Plain", "With **bold** and `code`", "Hi {{who}}", "<script>x</script>", "[hola]{lang=es}"} {
		typed := ast.NewHeadingElement(pos, 3, text, "heading-x")
		legacy := ast.NewRawHTMLTextElement(pos, HeadingHTML(3, text, "heading-x"))
		legacy.Level = 3
		if got, want := RenderElementToHTML(typed, vars, nil), RenderElementToHTML(legacy, vars, nil); got != want {
			t.Fatalf("%q renders differently\n typed: %s\nlegacy: %s", text, got, want)
		}
		populateElementHTML(typed, vars)
		populateElementHTML(legacy, vars)
		if got := LegacyHeadingElement(typed).ContentHTML; got != legacy.ContentHTML {
			t.Fatalf("%q inline HTML differs\n typed: %s\nlegacy: %s", text, got, legacy.ContentHTML)
		}
	}
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = []ast.Element{ast.NewHeadingElement(pos, 4, "A", "a")}
	doc.ContentBlocks = []ast.ContentBlock{*block}
	lowered := LowerTypedHeadings(doc)
	if _, ok := lowered.ContentBlocks[0].Elements[0].(*ast.TextElement); !ok {
		t.Fatal("heading was not lowered")
	}
	if _, ok := doc.ContentBlocks[0].Elements[0].(*ast.HeadingElement); !ok {
		t.Fatal("lowering mutated the caller's AST")
	}
}
