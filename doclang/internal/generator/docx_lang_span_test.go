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

// TestDOCXGenerator_RenderInlineMarkdown_LangSpan covers issue #63: a
// [text]{lang=xx} inline span in body prose must become a run with
// docxgo's per-run Language override (domain.Run.SetLanguage, added in
// docxgo v2.12.0), so a screen reader switches pronunciation for that run
// — not just a stripped-down plain-text run.
func TestDOCXGenerator_RenderInlineMarkdown_LangSpan(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)

	text := ast.NewTextElement(diagnostics.NewPosition(1, 1), "Bonjour le [monde](https://example.com) et [le chat]{lang=fr} aussi")
	doc := astWithElements(text)

	output := filepath.Join(t.TempDir(), "lang.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if !strings.Contains(xml, `<w:t>le chat</w:t>`) {
		t.Fatalf("expected the lang-span text to survive as its own run, document.xml:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:lang w:val="fr">`) {
		t.Errorf("expected a <w:lang w:val=\"fr\"> on the lang-span run, document.xml:\n%s", xml)
	}
}

// TestDOCXGenerator_RenderInlineMarkdown_LangSpan_InvalidTagIgnored covers
// the same defense-in-depth gate as core/renderer/sanitizer.go: a tag that
// doesn't pass a11y.IsValidLangTag must not reach the XML unvalidated, even
// though the regex's own capture charset (`[a-zA-Z0-9-]+`) is wider than
// what the validator accepts.
func TestDOCXGenerator_RenderInlineMarkdown_LangSpan_InvalidTagIgnored(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)

	// "1x" fails IsValidLangTag (must start with 2-8 letters).
	text := ast.NewTextElement(diagnostics.NewPosition(1, 1), "[texto]{lang=1x}")
	doc := astWithElements(text)

	output := filepath.Join(t.TempDir(), "lang-invalid.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if strings.Contains(xml, "<w:lang") {
		t.Errorf("expected no <w:lang> element for an invalid tag, document.xml:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:t>texto</w:t>`) {
		t.Errorf("expected the run text to still render despite the invalid tag, document.xml:\n%s", xml)
	}
}
