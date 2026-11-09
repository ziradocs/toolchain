// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

const literalCodeSource = "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  TEXT\n    Título: {{title}}\n  CODE yaml\n    a: {{title}}\n  :::code-group\n  ```go [main.go]\n  fmt.Println(\"{{title}}\")\n  ```\n  :::\n"

// Issue #487: en doclang el código es literal en todas las salidas. El
// contentHTML que devuelve get_ast (RenderASTJSON) no sustituye {{title}} en
// un CODE ni en un code-group, igual que el content, el HTML, el DOCX y el
// Markdown; la prosa sí sustituye.
func TestRenderASTJSON_CodeIsLiteral(t *testing.T) {
	doc, diags := parser.New(util.NewNoop()).ParseDocument(literalCodeSource, "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse: %s", d.String())
		}
	}
	out, err := New(newTestLogger()).RenderASTJSON(doc)
	if err != nil {
		t.Fatalf("RenderASTJSON: %v", err)
	}
	els := doc.ContentBlocks[0].Elements
	if got := els[1].(*ast.CodeElement).ContentHTML; got != "a: {{title}}" {
		t.Errorf("CODE contentHTML %q, want %q", got, "a: {{title}}")
	}
	if got := els[2].(*ast.CodeGroupElement).CodeBlocks[0].ContentHTML; !strings.Contains(got, "{{title}}") || strings.Contains(got, "Demo") {
		t.Errorf("code-group contentHTML was substituted: %q", got)
	}
	if got := els[0].(*ast.TextElement).ContentHTML; !strings.Contains(got, "Título: Demo") {
		t.Errorf("prose must still substitute {{title}}: %q", got)
	}
	if !strings.Contains(string(out), `"contentHTML":"a: {{title}}"`) {
		t.Errorf("the serialized AST must carry the literal contentHTML:\n%s", out)
	}
}

// El Markdown ya dejaba el código literal; esto lo fija.
func TestMarkdown_CodeIsLiteral(t *testing.T) {
	doc, _ := parser.New(util.NewNoop()).ParseDocument(literalCodeSource, "d.doclang")
	md := NewMarkdownGenerator(newTestLogger()).renderElement(doc.ContentBlocks[0].Elements[1])
	if !strings.Contains(md, "a: {{title}}") {
		t.Errorf("markdown code must keep {{title}}:\n%s", md)
	}
}
