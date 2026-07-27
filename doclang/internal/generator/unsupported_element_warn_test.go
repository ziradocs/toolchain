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

// astWithMediaElement builds a minimal AST containing a *ast.MediaElement —
// currently unhandled by both generators (issue #36), so it exercises the
// `default:` branch of renderElement in each.
func astWithMediaElement() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)

	front := ast.NewFrontMatterNode(pos)
	front.Title = "Media Fixture"
	doc.FrontMatter = front

	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "Section"
	block.Elements = append(block.Elements,
		ast.NewMediaElement(diagnostics.NewPosition(3, 1), "video", "demo.mp4"),
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
	doc := astWithMediaElement()

	output := filepath.Join(t.TempDir(), "media.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(logger.warns) == 0 {
		t.Fatal("expected a warning for the unhandled MediaElement, got none")
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
	doc := astWithMediaElement()

	output := filepath.Join(t.TempDir(), "media.md")
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
