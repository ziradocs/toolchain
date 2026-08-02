// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestFormatDocument_PreservesLangWithoutRaw and
// TestFormatStrict_PreservesLangWithoutRaw are regressions from code
// review: frontMatterOverrides built its map from
// Mode/Title/Author/Date/Theme/Variables but omitted the newly added Lang
// field, so any AST whose FrontMatter.Raw is empty (built via
// ast.NewFrontMatterNode, or decoded from --format json — Raw is
// json:"-") silently lost `lang:` on reformat.
func TestFormatDocument_PreservesLangWithoutRaw(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "Doc"
	doc.FrontMatter.Lang = "fr"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Elements = append(block.Elements, ast.NewTextElement(pos, "Contenu."))
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: unexpected error: %v", err)
	}
	if !strings.Contains(out, "lang: fr\n") {
		t.Errorf("expected formatted frontmatter to preserve %q, got:\n%s", "lang: fr", out)
	}
}

func TestFormatStrict_PreservesLangWithoutRaw(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Mode = "strict"
	doc.FrontMatter.Lang = "pt-BR"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: unexpected error: %v", err)
	}
	if !strings.Contains(out, "lang: pt-BR\n") {
		t.Errorf("expected formatted frontmatter to preserve %q, got:\n%s", "lang: pt-BR", out)
	}
}
