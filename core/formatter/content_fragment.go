// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// FormatContentFragment uses the ordinary strict serializer, returning only
// the indented element body. No frontmatter or SLIDE boundary is included.
// The caller must declare the returned body's required source capabilities.
func FormatContentFragment(nodes []ast.Element) (string, error) {
	doc := contentFragmentAST(nodes)
	out, err := FormatStrict(doc)
	if err != nil {
		return "", err
	}
	marker := "SLIDE content\n"
	at := strings.Index(out, marker)
	if at < 0 {
		return "", fmt.Errorf("fragment formatter did not produce its structural wrapper")
	}
	return out[at+len(marker):], nil
}

func contentFragmentAST(nodes []ast.Element) *ast.AST {
	doc := ast.NewAST(diagnostics.NewPosition(1, 1))
	block := ast.NewContentBlock(diagnostics.NewPosition(1, 1), "content")
	block.LayoutDeclared = true
	block.Elements = nodes
	doc.ContentBlocks = []ast.ContentBlock{*block}
	doc.FrontMatter = &ast.FrontMatterNode{Mode: "strict", ASTCapabilities: []string{ast.TypedHeadingsCapability, ast.ContentMarkdownCapability}}
	ast.SetTableContract(doc)
	return doc
}

// ContentFragmentCapabilities returns the exact declarations required by this body.
func ContentFragmentCapabilities(nodes []ast.Element) []string {
	return ast.UsedCapabilities(contentFragmentAST(nodes))
}
