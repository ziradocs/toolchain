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

// TestGeneratePPTX_LangSpanInvalidTagIgnored mirrors the doclang/docx.go
// gate: a tag that fails a11y.IsValidLangTag must not reach the XML, even
// though pptxLangRe's own capture charset is wider than what the validator
// accepts.
func TestGeneratePPTX_LangSpanInvalidTagIgnored(t *testing.T) {
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
	if strings.Contains(slideXML, "lang=") {
		t.Errorf("expected no lang attribute for an invalid tag, slide1.xml:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, "texto") {
		t.Errorf("expected the run text to still render despite the invalid tag, slide1.xml:\n%s", slideXML)
	}
}
