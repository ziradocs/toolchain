// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Issue #487: con RenderContext.LiteralCode el código es literal, `{{nombre}}`
// incluido, como en el AST. La etiqueta de una pestaña de code-group no es
// código y sigue como estaba. Sin la opción (el valor cero, lo que usa
// slidelang) el código sustituye como siempre.
func TestRenderElementToHTML_LiteralCodeOption(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"title": "Demo"}

	code := ast.NewCodeElement(pos, "yaml", "a: {{title}}")
	group := ast.NewCodeGroupElement(pos)
	group.CodeBlocks = append(group.CodeBlocks, ast.CodeBlock{Language: "go", Label: "main.go", Content: "b: {{title}}"})

	for _, tc := range []struct {
		ctx       *RenderContext
		code, grp string
	}{
		{&RenderContext{LiteralCode: true}, "a: {{title}}", "b: {{title}}"},
		{nil, "a: Demo", "b: Demo"},
		{&RenderContext{}, "a: Demo", "b: Demo"},
	} {
		if out := RenderElementToHTML(code, vars, tc.ctx); !strings.Contains(out, tc.code) {
			t.Errorf("ctx %+v: code %q not in:\n%s", tc.ctx, tc.code, out)
		}
		if out := RenderElementToHTML(group, vars, tc.ctx); !strings.Contains(out, tc.grp) {
			t.Errorf("ctx %+v: code-group %q not in:\n%s", tc.ctx, tc.grp, out)
		}
	}
}

// El documento completo con la opción: el bloque de código sale literal y el
// texto de alrededor sigue sustituyendo.
func TestGenerateDocumentHTML_LiteralCodeOption(t *testing.T) {
	doc := ast.NewAST(diagnostics.NewPosition(1, 1))
	doc.FrontMatter = ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	doc.FrontMatter.Title = "Demo"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "title")
	block.Heading = "A"
	block.Elements = []ast.Element{
		ast.NewTextElement(diagnostics.NewPosition(3, 1), "Título: {{title}}"),
		ast.NewCodeElement(diagnostics.NewPosition(4, 1), "yaml", "a: {{title}}"),
	}
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, &RenderContext{LiteralCode: true})
	if !strings.Contains(out, "a: {{title}}") {
		t.Errorf("the code block must keep {{title}}")
	}
	if !strings.Contains(out, "Título: Demo") {
		t.Errorf("prose must still substitute {{title}}")
	}
	if out := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, nil); !strings.Contains(out, "a: Demo") {
		t.Errorf("without the option the code block substitutes, as before")
	}
}

// PopulateInlineHTMLWithOptions con LiteralCode deja el contentHTML del código
// literal; PopulateInlineHTML (sin opciones) sustituye, como siempre.
func TestPopulateInlineHTML_LiteralCodeOption(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"title": "Demo"}
	for _, tc := range []struct {
		literal bool
		want    string
	}{{true, "a: {{title}}"}, {false, "a: Demo"}} {
		code := ast.NewCodeElement(pos, "yaml", "a: {{title}}")
		PopulateInlineHTMLWithOptions(newTestDoc(code), vars, InlineHTMLOptions{LiteralCode: tc.literal})
		if code.ContentHTML != tc.want {
			t.Errorf("literal=%v: contentHTML %q, want %q", tc.literal, code.ContentHTML, tc.want)
		}
	}
}
