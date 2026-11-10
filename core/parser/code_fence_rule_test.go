// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

func flexDoc(t *testing.T, src string) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	return New(util.NewNoop()).ParseDocument(src, "d.md")
}

func codesOf(doc *ast.AST) []*ast.CodeElement {
	var out []*ast.CodeElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if c, ok := n.(*ast.CodeElement); ok {
			out = append(out, c)
		}
		return nil
	})
	return out
}

func textsOf(doc *ast.AST) []*ast.TextElement {
	var out []*ast.TextElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if c, ok := n.(*ast.TextElement); ok {
			out = append(out, c)
		}
		return nil
	})
	return out
}

// La cerca cierra con una línea de solo backticks, al menos tantos como la
// apertura: ``` cierra con ```` (y avisa CODE002, porque antes esa línea era
// código), ```` no cierra con ```.
func TestCodeFence_ClosesWithAtLeastAsManyBackticks(t *testing.T) {
	doc, diags := flexDoc(t, "# A\n\n```\nuno\n````\n\ndespués\n")
	codes := codesOf(doc)
	if len(codes) != 1 || codes[0].Content != "uno" {
		t.Fatalf("``` must close at ````: %#v", codes)
	}
	var code002 []diagnostics.Diagnostic
	for _, d := range diags {
		if d.RuleID == "CODE002" {
			code002 = append(code002, d)
		}
	}
	if len(code002) != 1 || code002[0].IsError() || code002[0].Position.Line != 5 {
		t.Errorf("want one CODE002 warning at line 5, got %v", diags)
	}

	doc, diags = flexDoc(t, "# A\n\n````md\n```go\nx := 1\n```\n````\n\n## B\n\nfin\n")
	codes = codesOf(doc)
	if len(codes) != 1 || codes[0].Content != "```go\nx := 1\n```" || codes[0].Language != "md" {
		t.Fatalf("```` must keep the inner ``` as content: %#v", codes)
	}
	for _, d := range diags {
		if d.RuleID == "CODE002" {
			t.Errorf("a four-backtick fence closed by four is not CODE002: %v", d)
		}
	}
	heading := false
	for _, tx := range textsOf(doc) {
		if tx.IsRawHTML && strings.Contains(tx.Content, ">B<") {
			heading = true
		}
	}
	if !heading {
		t.Error("the ## after the fence must still be a heading")
	}
}

// Una línea que empieza con ``` pero lleva un backtick en el info string no
// abre una cerca: es prosa y no se descarta.
func TestCodeFence_BacktickInInfoIsProse(t *testing.T) {
	doc, _ := flexDoc(t, "# A\n\n```js` es un code span suelto\n\ntexto\n")
	if len(codesOf(doc)) != 0 {
		t.Fatalf("no code block expected: %#v", codesOf(doc))
	}
	found := false
	for _, tx := range textsOf(doc) {
		if strings.Contains(tx.Content, "es un code span suelto") {
			found = true
		}
	}
	if !found {
		t.Error("the line must stay as prose")
	}
}

// Un elemento ya consumido con una cantidad impar de líneas que empiezan con
// ``` (un ::: con una cerca que no cierra, un párrafo con ```js`) no deja al
// parser "dentro de código": el ## siguiente sigue siendo un encabezado.
func TestCodeFence_TrackingDoesNotDriftAfterConsumedElements(t *testing.T) {
	for name, body := range map[string]string{
		"callout with a backtick-info line": ":::info\n```js` uno\n:::\n",
		"prose with backtick info":   "```js` uno\n```js` dos\n```js` tres\n",
	} {
		t.Run(name, func(t *testing.T) {
			doc, _ := flexDoc(t, "# A\n\n"+body+"\n## B\n\nfin\n")
			heading := false
			for _, tx := range textsOf(doc) {
				if tx.IsRawHTML && strings.Contains(tx.Content, ">B<") {
					heading = true
				}
			}
			if !heading {
				t.Errorf("the ## after the element must be a heading")
			}
		})
	}
}

// Los diagramas en cerca siguen la misma regla: ```mermaid cierra con ````,
// ````mermaid no cierra con ```.
func TestCodeFence_DiagramFencesUseTheSameRule(t *testing.T) {
	mermaids := func(doc *ast.AST) []*ast.MermaidElement {
		var out []*ast.MermaidElement
		_ = ast.Walk(doc, func(n ast.Node) error {
			if m, ok := n.(*ast.MermaidElement); ok {
				out = append(out, m)
			}
			return nil
		})
		return out
	}
	doc, _ := flexDoc(t, "# A\n\n```mermaid\ngraph TD\n`````\n\nfin\n")
	if m := mermaids(doc); len(m) != 1 || strings.TrimSpace(m[0].Content) != "graph TD" {
		t.Errorf("```mermaid must close at `````: %#v", m)
	}
	doc, _ = flexDoc(t, "# A\n\n````mermaid\ngraph TD\n```\nA-->B\n````\n")
	if m := mermaids(doc); len(m) != 1 || !strings.Contains(m[0].Content, "A-->B") {
		t.Errorf("````mermaid must not close at ```: %#v", m)
	}
}

// Una cerca de cuatro backticks dentro de un :::code-group guarda un ``` y un
// ":::" como contenido de la pestaña.
func TestCodeFence_CodeGroupTabWithALongerFence(t *testing.T) {
	doc, _ := flexDoc(t, "# A\n\n:::code-group\n````md [doc]\n```go\nx\n```\n:::\n````\n```sh [sh]\necho\n```\n:::\n")
	var group *ast.CodeGroupElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if g, ok := n.(*ast.CodeGroupElement); ok {
			group = g
		}
		return nil
	})
	if group == nil || len(group.CodeBlocks) != 2 || group.CodeBlocks[0].Content != "```go\nx\n```\n:::" || group.CodeBlocks[1].Content != "echo" {
		t.Fatalf("code group: %#v", group)
	}
}

// El pre-pase de node-id cierra la cerca donde la cierra el parser, así que la
// anotación de después nombra al párrafo siguiente.
func TestCodeFence_NodeIDAfterAFourBacktickFence(t *testing.T) {
	doc, diags := flexDoc(t, "# A\n\n````md\n```go\nx := 1\n```\n````\n\n<!-- node-id: Para -->\nTexto.\n")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse: %s", d.String())
		}
	}
	named := false
	for _, tx := range textsOf(doc) {
		if tx.Content == "Texto." {
			named = tx.NodeID == "Para"
		}
	}
	if !named {
		t.Error("the node-id after the fence must name the paragraph")
	}
}
