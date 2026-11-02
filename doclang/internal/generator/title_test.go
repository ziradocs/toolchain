// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestDocumentTitle(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	withBlocks := func(fm *ast.FrontMatterNode, blocks ...ast.ContentBlock) *ast.AST {
		doc := ast.NewAST(pos)
		doc.FrontMatter = fm
		doc.ContentBlocks = blocks
		return doc
	}
	headless := *ast.NewContentBlock(pos, "content")
	h1 := *ast.NewContentBlock(pos, "title")
	h1.Heading = "Del H1"

	cases := []struct {
		name string
		doc  *ast.AST
		path string
		want string
	}{
		{"frontmatter gana", withBlocks(&ast.FrontMatterNode{Title: "Del FM"}, h1), "a/b.md", "Del FM"},
		{"primer h1 tras preámbulo", withBlocks(nil, headless, h1), "a/b.md", "Del H1"},
		{"frontmatter sin title no cambia", withBlocks(&ast.FrontMatterNode{Mode: "flex"}, h1), "a/b.md", ""},
		{"sin h1: nombre del archivo", withBlocks(nil, headless), "docs/arquitectura.backend.md", "arquitectura.backend"},
		{"sin h1 ni archivo: vacío", withBlocks(nil, headless), "", ""},
		{"nil", nil, "x.md", ""},
	}
	for _, tc := range cases {
		if got := DocumentTitle(tc.doc, tc.path); got != tc.want {
			t.Errorf("%s: want %q, got %q", tc.name, tc.want, got)
		}
	}
}
