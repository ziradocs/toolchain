// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// TestGeneratePPTX_LangSpanEmitsRunLang covers issue #63: a [text]{lang=xx}
// inline span must become a run with pptxgo's per-run a:rPr/@lang (added in
// pptxgo's Paragraph.Lang, wired up alongside docxgo's SetLanguage) — not
// just literal, unresolved markdown syntax in the slide text.
func TestGeneratePPTX_LangSpanEmitsRunLang(t *testing.T) {
	dir := t.TempDir()

	doc := ast.NewAST(diagnostics.Position{Line: 1, Column: 1})
	doc.FrontMatter = ast.NewFrontMatterNode(diagnostics.Position{Line: 1, Column: 1})
	doc.FilePath = "test.slidelang"

	block := ast.NewContentBlock(diagnostics.Position{Line: 1, Column: 1}, "content")
	block.Title = "Slide"
	text := ast.NewTextElement(diagnostics.Position{Line: 1, Column: 1}, "Bonjour [le chat]{lang=fr} aussi")
	block.Elements = append(block.Elements, text)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{}); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	slideXML := zipEntryContent(t, filepath.Join(dir, "test.pptx"), "ppt/slides/slide1.xml")
	if !strings.Contains(slideXML, "le chat") {
		t.Fatalf("expected the lang-span text in the slide, slide1.xml:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, `lang="fr"`) {
		t.Errorf("expected a run with lang=\"fr\", slide1.xml:\n%s", slideXML)
	}
}

// TestGeneratePPTX_LangSpanInvalidTagDegradesToLiteral mirrors the
// doclang/docx.go gate: a tag that fails a11y.IsValidLangTag must not reach
// the XML. Issue #63 code review finding #5: it must degrade to the FULL
// literal match ("[texto]{lang=1x}"), not silently to just the inner text
// ("texto") — the latter hides the author's malformed markup instead of
// surfacing it, matching what core/renderer/sanitizer.go does for the same
// case.
func TestGeneratePPTX_LangSpanInvalidTagDegradesToLiteral(t *testing.T) {
	dir := t.TempDir()

	doc := ast.NewAST(diagnostics.Position{Line: 1, Column: 1})
	doc.FrontMatter = ast.NewFrontMatterNode(diagnostics.Position{Line: 1, Column: 1})
	doc.FilePath = "test.slidelang"

	block := ast.NewContentBlock(diagnostics.Position{Line: 1, Column: 1}, "content")
	block.Title = "Slide"
	// "1x" fails IsValidLangTag (must start with 2-8 letters).
	text := ast.NewTextElement(diagnostics.Position{Line: 1, Column: 1}, "[texto]{lang=1x}")
	block.Elements = append(block.Elements, text)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{}); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	slideXML := zipEntryContent(t, filepath.Join(dir, "test.pptx"), "ppt/slides/slide1.xml")
	// lang="..." (attribute) not "lang=" (substring) — the degraded literal
	// text itself contains "lang=1x", which would false-positive on a bare
	// substring check.
	if strings.Contains(slideXML, `lang="`) {
		t.Errorf("expected no lang attribute for an invalid tag, slide1.xml:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, "[texto]{lang=1x}") {
		t.Errorf("expected the full literal match to survive despite the invalid tag, slide1.xml:\n%s", slideXML)
	}
}

// TestGeneratePPTX_LangSpanNestedBold covers issue #63 code review finding
// #2: markdown INSIDE a [texto]{lang=xx} span (e.g. **bold**) must still be
// processed — before #63 the same text with no lang span at all rendered
// bold; wrapping it in a lang span regressed that to literal asterisks in
// the slide text.
func TestGeneratePPTX_LangSpanNestedBold(t *testing.T) {
	dir := t.TempDir()

	doc := ast.NewAST(diagnostics.Position{Line: 1, Column: 1})
	doc.FrontMatter = ast.NewFrontMatterNode(diagnostics.Position{Line: 1, Column: 1})
	doc.FilePath = "test.slidelang"

	block := ast.NewContentBlock(diagnostics.Position{Line: 1, Column: 1}, "content")
	block.Title = "Slide"
	text := ast.NewTextElement(diagnostics.Position{Line: 1, Column: 1}, "dijo [**le chat**]{lang=fr} y se fue")
	block.Elements = append(block.Elements, text)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{}); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	slideXML := zipEntryContent(t, filepath.Join(dir, "test.pptx"), "ppt/slides/slide1.xml")
	if strings.Contains(slideXML, "**") {
		t.Errorf("expected the nested bold markers to be processed, not left literal, slide1.xml:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, "le chat") {
		t.Errorf("expected the bold text to survive, slide1.xml:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, `lang="fr"`) {
		t.Errorf("expected the bold run to also carry lang=\"fr\", slide1.xml:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, `b="1"`) {
		t.Errorf("expected the bold run to still be bold, slide1.xml:\n%s", slideXML)
	}
}
