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
// cerca más larga, que el parser cierra solo en su propia línea (CommonMark):
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
	again, _ := parser.New(util.NewNoop()).ParseDocument(out, "d.doclang")
	var before, after string
	for _, d := range []*ast.AST{doc, again} {
		_ = ast.Walk(d, func(n ast.Node) error {
			if c, ok := n.(*ast.CodeElement); ok {
				if d == doc {
					before = c.Content
				} else {
					after = c.Content
				}
			}
			return nil
		})
	}
	if before == "" || before != after {
		t.Errorf("content changed across fmt: %q -> %q", before, after)
	}
}
