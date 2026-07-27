// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// astWithUnhandledElement builds a minimal AST containing a
// *ast.ColumnElement — permanently excluded from both generators'
// element-coverage guard (element_coverage_test.go: it's a sub-element of
// GridElement.Columns, consumed via its own dedicated GridElement case, and
// never appears directly in block.Elements/section.Elements), so it
// reliably exercises the `default:` branch of renderElement in each, unlike
// a type a future PR might start handling. *ast.DirectiveNode used to be
// this fixture, but it now has its own case in both generators (a warning,
// not silence — see markdown.go/docx.go), so it no longer reaches
// `default:`. ast.Element is a sealed interface (element() unexported), so
// this file can't define its own stub type instead — it has to be a real,
// deliberately-uncovered core/ast type.
func astWithUnhandledElement() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)

	front := ast.NewFrontMatterNode(pos)
	front.Title = "Unhandled Element Fixture"
	doc.FrontMatter = front

	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "Section"
	block.Elements = append(block.Elements,
		ast.NewColumnElement(diagnostics.NewPosition(3, 1), "orphan column"),
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

// astWithDirectiveNode builds a minimal AST containing an *ast.DirectiveNode
// (an @notes-style directive), reachable in doclang — unlike slidelang,
// which filters these upstream (extractPresenterNotes), doclang's
// DocumentFlexParser lets them through to block.Elements untouched.
func astWithDirectiveNode() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)

	front := ast.NewFrontMatterNode(pos)
	front.Title = "Directive Fixture"
	doc.FrontMatter = front

	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "Section"
	block.Elements = append(block.Elements,
		ast.NewDirectiveNode(diagnostics.NewPosition(7, 1), "notes"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

// TestMarkdownGenerator_DirectiveNode_WarnsActionably covers the case both
// generators now handle explicitly: a directive (@notes, @timer, …) has no
// presenter-notes view in a document, so its content is NOT rendered — but
// unlike the generic `default:` fallback (which said "Unknown element
// type", misleading since the type is perfectly known), the warning must
// name the directive and explain why, without regressing into the
// logger.Warn %!(EXTRA) formatting bug fixed alongside the coverage guard
// (issue #35/#50).
func TestMarkdownGenerator_DirectiveNode_WarnsActionably(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithDirectiveNode()

	output := filepath.Join(t.TempDir(), "directive.md")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "markdown"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	found := false
	for _, w := range logger.warns {
		if strings.Contains(w, "%!(EXTRA") {
			t.Errorf("warning contains a Go fmt EXTRA artifact: %q", w)
		}
		if strings.Contains(w, "Unknown element type") {
			t.Errorf("a DirectiveNode has its own case now — it must not fall through to the generic \"Unknown element type\" warning: %q", w)
		}
		if strings.Contains(w, "@notes") && strings.Contains(w, "línea 7") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning naming the directive (@notes) and its line (7), got %+v", logger.warns)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if strings.Contains(string(data), "notes") {
		t.Errorf("directive content must not be rendered into the document, got:\n%s", data)
	}
}

// TestDOCXGenerator_DirectiveNode_WarnsActionably mirrors the Markdown test
// above for docx.go.
func TestDOCXGenerator_DirectiveNode_WarnsActionably(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithDirectiveNode()

	output := filepath.Join(t.TempDir(), "directive.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	found := false
	for _, w := range logger.warns {
		if strings.Contains(w, "%!(EXTRA") {
			t.Errorf("warning contains a Go fmt EXTRA artifact: %q", w)
		}
		if strings.Contains(w, "Unknown element type") {
			t.Errorf("a DirectiveNode has its own case now — it must not fall through to the generic \"Unknown element type\" warning: %q", w)
		}
		if strings.Contains(w, "@notes") && strings.Contains(w, "línea 7") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning naming the directive (@notes) and its line (7), got %+v", logger.warns)
	}
}
