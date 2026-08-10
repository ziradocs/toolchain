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

// astWithTwoSections builds a preamble block (Title empty, Heading set —
// the shape resolveSectionTitle treats as unnumbered) followed by two
// numbered content blocks, for exercising opts.TOC/Numbering/PageBreaks
// together the way build.go actually populates a real document.
func astWithTwoSections() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	front := ast.NewFrontMatterNode(pos)
	front.Title = "Two Sections"
	doc.FrontMatter = front

	preamble := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "title")
	preamble.Heading = "Executive Summary"
	preamble.Elements = append(preamble.Elements, ast.NewTextElement(diagnostics.NewPosition(2, 2), "Summary text"))

	block1 := ast.NewContentBlock(diagnostics.NewPosition(3, 1), "content")
	block1.Title = "Introduction"
	block1.Elements = append(block1.Elements, ast.NewTextElement(diagnostics.NewPosition(3, 2), "Intro text"))

	block2 := ast.NewContentBlock(diagnostics.NewPosition(4, 1), "content")
	block2.Title = "Conclusion"
	block2.Elements = append(block2.Elements, ast.NewTextElement(diagnostics.NewPosition(4, 2), "Conclusion text"))

	doc.ContentBlocks = append(doc.ContentBlocks, *preamble, *block1, *block2)
	return doc
}

// TestDOCXGenerator_TOCFalse_OmitsTOCField covers the issue #115 follow-up:
// before opts.TOC was wired in, the DOCX generator emitted the TOC field and
// its static placeholder unconditionally, so `--toc=false`/`toc: false` had
// no effect on DOCX output (unlike html/pdf/markdown, where it already
// worked).
func TestDOCXGenerator_TOCFalse_OmitsTOCField(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithTwoSections()

	output := filepath.Join(t.TempDir(), "no-toc.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx", TOC: false}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if strings.Contains(xml, "TOC") {
		t.Errorf("expected no TOC field in document.xml when opts.TOC is false, got:\n%s", xml)
	}
	if strings.Contains(xml, "Tabla de Contenidos") {
		t.Errorf("expected no TOC placeholder title in document.xml when opts.TOC is false, got:\n%s", xml)
	}
}

// TestDOCXGenerator_Numbering_PrefixesSectionHeadings covers the other half
// of the same bug: `numbering:`, which #100 fixed for HTML/Markdown, never
// applied to DOCX either. With opts.Numbering true, level-1 section
// headings get a "N. " prefix; the preamble block (Title empty, Heading
// set) must render its heading but NOT consume a number, matching
// resolveSectionTitle's contract for HTML/Markdown.
func TestDOCXGenerator_Numbering_PrefixesSectionHeadings(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithTwoSections()

	output := filepath.Join(t.TempDir(), "numbered.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx", Numbering: true}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	for _, expected := range []string{"Executive Summary", "1. Introduction", "2. Conclusion"} {
		if !strings.Contains(xml, expected) {
			t.Errorf("expected %q in document.xml, got:\n%s", expected, xml)
		}
	}
	if strings.Contains(xml, "1. Executive Summary") {
		t.Errorf("preamble heading should not be numbered, got:\n%s", xml)
	}
}

// TestDOCXGenerator_Numbering_False_LeavesHeadingsPlain is the symmetric
// case: opts.Numbering false (the zero value, and today's default when a
// caller doesn't set it) must not prefix anything.
func TestDOCXGenerator_Numbering_False_LeavesHeadingsPlain(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithTwoSections()

	output := filepath.Join(t.TempDir(), "unnumbered.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if strings.Contains(xml, "1. Introduction") || strings.Contains(xml, "2. Conclusion") {
		t.Errorf("expected unnumbered headings when opts.Numbering is false, got:\n%s", xml)
	}
}

// TestDOCXGenerator_PageBreaks_EmitsPageBreakRun covers opts.PageBreaks,
// unwired before this change. A page break run must appear between the two
// content sections, but never after the last one.
func TestDOCXGenerator_PageBreaks_EmitsPageBreakRun(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithTwoSections()

	output := filepath.Join(t.TempDir(), "pagebreaks.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx", PageBreaks: true}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	count := strings.Count(xml, `w:br w:type="page"`)
	if count != 2 {
		t.Errorf("expected 2 page breaks (one between preamble/Introduction, one between Introduction/Conclusion), got %d in document.xml:\n%s", count, xml)
	}
}

// TestDOCXGenerator_TOC_MatchesBodyTitlesAndNumbering covers a code review
// finding on the TOC/Numbering wiring above: collectHeadings used to read
// block.Title directly, so with TOC+Numbering both on, the static TOC
// placeholder disagreed with the body it points to — it skipped the
// preamble (whose text lives in block.Heading, not Title) and never
// prefixed numbers, while the body (via resolveSectionTitle in Generate)
// showed the preamble and "N. " prefixes. Assert both surfaces render the
// exact same section titles.
func TestDOCXGenerator_TOC_MatchesBodyTitlesAndNumbering(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithTwoSections()

	output := filepath.Join(t.TempDir(), "toc-numbered.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx", TOC: true, Numbering: true}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	for _, expected := range []string{"Executive Summary", "1. Introduction", "2. Conclusion"} {
		count := strings.Count(xml, expected)
		if count < 2 {
			t.Errorf("expected %q to appear at least twice (TOC placeholder + body heading), got %d in document.xml:\n%s", expected, count, xml)
		}
	}
	if strings.Contains(xml, "1. Executive Summary") {
		t.Errorf("preamble heading should not be numbered in the TOC placeholder, got:\n%s", xml)
	}
}

// TestDOCXGenerator_PageBreaks_False_EmitsNone is the symmetric case.
func TestDOCXGenerator_PageBreaks_False_EmitsNone(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithTwoSections()

	output := filepath.Join(t.TempDir(), "no-pagebreaks.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if strings.Contains(xml, `w:type="page"`) {
		t.Errorf("expected no page break run when opts.PageBreaks is false, got:\n%s", xml)
	}
}
