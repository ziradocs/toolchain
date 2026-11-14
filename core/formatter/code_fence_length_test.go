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
// cerca más larga (fence.For), que el parser cierra solo en su propia línea:
// el round-trip conserva el contenido y lo que sigue. Con una cerca de tres el
// bloque cerraba en el ``` de adentro y la cerca de cierre se tragaba el
// resto del documento.
func TestFormatDocument_CodeContainingAFenceRoundTrips(t *testing.T) {
	src := "# A\n\n````md\n```go\nx := 1\n```\n````\n\n## B\n\nfin\n"
	doc := mustParseFenceDoc(t, src)
	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	if !strings.Contains(out, "````md\n```go\nx := 1\n```\n````") {
		t.Fatalf("want a four-backtick fence around the inner one:\n%s", out)
	}
	again := mustParseFenceDoc(t, out)
	if before, after := fencedContents(doc), fencedContents(again); len(before) != 1 || strings.Join(before, "|") != strings.Join(after, "|") {
		t.Errorf("code changed across fmt: %q -> %q\n%s", before, after, out)
	}
	if !strings.Contains(out, "## B") || !strings.Contains(out, "fin") {
		t.Errorf("the rest of the document must survive:\n%s", out)
	}
}

// Una pestaña de code-group con un ``` adentro ya no es "unsupported": se
// escribe con una cerca más larga y vuelve igual, en flex y en strict.
func TestFormat_CodeGroupPayloadWithAFence(t *testing.T) {
	flex := "# A\n\n:::code-group\n````md [doc]\n```go\nx\n```\n````\n```sh [sh]\necho\n```\n:::\n"
	doc := mustParseFenceDoc(t, flex)
	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	if got, want := strings.Join(fencedContents(mustParseFenceDoc(t, out)), "|"), "```go\nx\n```|echo"; got != want {
		t.Errorf("flex code-group: %q, want %q\n%s", got, want, out)
	}

	strict := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  :::code-group\n  ````md [doc]\n  ```go\n  x\n  ```\n  ````\n  :::\n"
	sdoc, diags := parser.New(util.NewNoop()).Parse(strict, "s.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse: %s", d.String())
		}
	}
	out, err = FormatStrict(sdoc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	again, _ := parser.New(util.NewNoop()).Parse(out, "s.slidelang")
	if got := strings.Join(fencedContents(again), "|"); got != "```go\nx\n```" || got != strings.Join(fencedContents(sdoc), "|") {
		t.Errorf("strict code-group: %q -> %q\n%s", fencedContents(sdoc), got, out)
	}
}

func mustParseFenceDoc(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse: %s", d.String())
		}
	}
	return doc
}

// fencedContents es el contenido de cada bloque de código y de cada pestaña.
func fencedContents(doc *ast.AST) []string {
	var out []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch e := n.(type) {
		case *ast.CodeElement:
			out = append(out, e.Content)
		case *ast.CodeGroupElement:
			for _, b := range e.CodeBlocks {
				out = append(out, b.Content)
			}
		}
		return nil
	})
	return out
}
