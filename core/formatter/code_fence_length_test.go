// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Un bloque de código cuyo contenido tiene una línea ``` se escribe con una
// cerca más larga (CodeFenceFor), que el parser cierra solo en su propia línea:
// el round-trip conserva el contenido.
func TestFormatDocument_CodeContainingAFenceRoundTrips(t *testing.T) {
	src := "# A\n\n````md\n```go\nx := 1\n```\n````\n"
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse: %s", d.String())
		}
	}
	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	if !strings.Contains(out, "````md\n```go\nx := 1\n```\n````") {
		t.Fatalf("want a four-backtick fence around the inner one:\n%s", out)
	}
	before, after := codeContent(doc), codeContent(mustParseFenceDoc(t, out))
	if before == "" || before != after {
		t.Errorf("content changed across fmt: %q -> %q", before, after)
	}
	if CodeFenceFor("x\n```\n") != "````" || CodeFenceFor("x") != "```" {
		t.Error("CodeFenceFor")
	}
}

// Una pestaña de code-group con un ``` adentro ya no es "unsupported": se
// escribe con una cerca más larga y vuelve igual.
func TestFormatStrict_CodeGroupPayloadWithAFence(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  :::code-group\n  ````md [doc]\n  ```go\n  x\n  ```\n  ````\n  :::\n"
	doc, diags := parser.New(util.NewNoop()).Parse(src, "s.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse: %s", d.String())
		}
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	again, _ := parser.New(util.NewNoop()).Parse(out, "s.slidelang")
	group := func(d *ast.AST) string {
		for _, el := range d.ContentBlocks[0].Elements {
			if g, ok := el.(*ast.CodeGroupElement); ok && len(g.CodeBlocks) > 0 {
				return g.CodeBlocks[0].Content
			}
		}
		return ""
	}
	if group(doc) != "```go\nx\n```" || group(again) != group(doc) {
		t.Errorf("code-group payload: %q -> %q\n%s", group(doc), group(again), out)
	}
}

func mustParseFenceDoc(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "d.doclang")
	return doc
}

func codeContent(doc *ast.AST) string {
	var c string
	_ = ast.Walk(doc, func(n ast.Node) error {
		if e, ok := n.(*ast.CodeElement); ok {
			c = e.Content
		}
		return nil
	})
	return c
}
