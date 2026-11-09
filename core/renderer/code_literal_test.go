// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Issue #487: en el HTML del documento el código es literal, `{{nombre}}`
// incluido, como en el AST, el lint, el DOCX y el Markdown. La etiqueta de
// una pestaña de code-group no es código y sigue como estaba.
func TestRenderElementToHTML_CodeIsLiteral(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"title": "Demo", "name": "<b>x</b>"}

	code := ast.NewCodeElement(pos, "yaml", "a: {{title}}\nb: {{name}}")
	out := RenderElementToHTML(code, vars, nil)
	if !strings.Contains(out, "a: {{title}}\nb: {{name}}") || strings.Contains(out, "Demo") {
		t.Errorf("code body must stay literal:\n%s", out)
	}

	group := ast.NewCodeGroupElement(pos)
	group.CodeBlocks = append(group.CodeBlocks, ast.CodeBlock{Language: "go", Label: "main.go", Content: "fmt.Println(\"{{title}}\")"})
	out = RenderElementToHTML(group, vars, nil)
	if !strings.Contains(out, "fmt.Println(&#34;{{title}}&#34;)") && !strings.Contains(out, "fmt.Println(&quot;{{title}}&quot;)") {
		t.Errorf("code-group body must stay literal:\n%s", out)
	}
	if strings.Contains(out, "Demo") {
		t.Errorf("code-group body was substituted:\n%s", out)
	}
}

// El documento completo: el bloque de código sale literal mientras el texto
// de alrededor sí sustituye.
func TestGenerateDocumentHTML_CodeIsLiteralProseIsNot(t *testing.T) {
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

	out := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, nil)
	if !strings.Contains(out, "a: {{title}}") {
		t.Errorf("the code block must keep {{title}}")
	}
	if !strings.Contains(out, "Título: Demo") {
		t.Errorf("prose must still substitute {{title}}")
	}
}
