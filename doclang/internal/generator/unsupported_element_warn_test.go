// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// astWithUnhandledElement builds a minimal AST containing a
// *ast.DirectiveNode — permanently excluded from both generators'
// element-coverage guard (element_coverage_test.go: doclang has no
// established behavior for @directives in a document context), so it
// reliably exercises the `default:` branch of renderElement in each,
// unlike a type a future PR might start handling.
func astWithUnhandledElement() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)

	front := ast.NewFrontMatterNode(pos)
	front.Title = "Unhandled Element Fixture"
	doc.FrontMatter = front

	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "Section"
	block.Elements = append(block.Elements,
		ast.NewDirectiveNode(diagnostics.NewPosition(3, 1), "notes"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

// TestDOCXGenerator_UnknownElementType_WarnsWithoutFormatArtifacts is the
// regression for the logger.Warn("DOCX", ...) bug fixed in this same commit
// (issue #35): util.Logger.Warn(message string, args ...interface{}) has no
// category parameter (unlike Info/Debug), so passing "DOCX" as if it were
// one made the format string literally "DOCX" and dumped the real message as
// a %!(EXTRA ...) artifact instead of the intended text.
func TestDOCXGenerator_UnknownElementType_WarnsWithoutFormatArtifacts(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithUnhandledElement()

	output := filepath.Join(t.TempDir(), "unhandled.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(logger.warns) == 0 {
		t.Fatal("expected a warning for the unhandled DirectiveNode, got none")
	}
	found := false
	for _, w := range logger.warns {
		if strings.Contains(w, "%!(EXTRA") {
			t.Errorf("warning contains a Go fmt EXTRA artifact (the bug this test guards against): %q", w)
		}
		if strings.Contains(w, "DOCX: Unknown element type") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning containing %q, got %+v", "DOCX: Unknown element type", logger.warns)
	}
}

// TestMarkdownGenerator_UnknownElementType_Warns mirrors the DOCX test above
// for markdown.go, which already formatted its warning correctly — this
// pins that it keeps doing so as coverage grows (issue #35).
func TestMarkdownGenerator_UnknownElementType_Warns(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithUnhandledElement()

	output := filepath.Join(t.TempDir(), "unhandled.md")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "markdown"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	found := false
	for _, w := range logger.warns {
		if strings.Contains(w, "%!(EXTRA") {
			t.Errorf("warning contains a Go fmt EXTRA artifact: %q", w)
		}
		if strings.Contains(w, "MARKDOWN: Unknown element type") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning containing %q, got %+v", "MARKDOWN: Unknown element type", logger.warns)
	}
}
