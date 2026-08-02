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

// TestDOCXGenerator_RenderInlineMarkdown_LangSpan_InvalidTagDegradesToLiteral
// covers the same defense-in-depth gate as core/renderer/sanitizer.go: a tag
// that doesn't pass a11y.IsValidLangTag must not reach the XML unvalidated,
// even though the regex's own capture charset (`[a-zA-Z0-9-]+`) is wider
// than what the validator accepts. Issue #63 code review finding #5: an
// invalid tag must degrade to the FULL literal match
// ("[texto]{lang=1x}"), not silently to just the inner text ("texto") —
// the latter hides the author's malformed markup instead of surfacing it,
// matching what core/renderer/sanitizer.go does for the same case.
func TestDOCXGenerator_RenderInlineMarkdown_LangSpan_InvalidTagDegradesToLiteral(t *testing.T) {
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
	if !strings.Contains(xml, `<w:t>[texto]{lang=1x}</w:t>`) {
		t.Errorf("expected the full literal match to survive despite the invalid tag, document.xml:\n%s", xml)
	}
}

// TestDOCXGenerator_RenderInlineMarkdown_LangSpan_NestedBold covers issue
// #63 code review finding #2: markdown INSIDE a [texto]{lang=xx} span
// (e.g. **bold**) must still be processed — before #63 the same text with
// no lang span at all rendered bold; wrapping it in a lang span regressed
// that to literal asterisks in the run text. The bold run must ALSO carry
// the language, not just the plain-text runs around it.
func TestDOCXGenerator_RenderInlineMarkdown_LangSpan_NestedBold(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)

	text := ast.NewTextElement(diagnostics.NewPosition(1, 1), "dijo [**tout le monde**]{lang=fr} y se fue")
	doc := astWithElements(text)

	output := filepath.Join(t.TempDir(), "lang-nested-bold.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if strings.Contains(xml, "**") {
		t.Errorf("expected the nested bold markers to be processed, not left literal, document.xml:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:t>tout le monde</w:t>`) {
		t.Errorf("expected the bold text to survive as its own run, document.xml:\n%s", xml)
	}

	idx := strings.Index(xml, `<w:t>tout le monde</w:t>`)
	if idx == -1 {
		t.Fatal("bold run not found")
	}
	runStart := strings.LastIndex(xml[:idx], "<w:r>")
	run := xml[runStart:idx]
	if !strings.Contains(run, `w:val="fr"`) {
		t.Errorf("expected the bold run to also carry <w:lang w:val=\"fr\">, run:\n%s", run)
	}
	if !strings.Contains(run, `<w:b w:val="true">`) {
		t.Errorf("expected the bold run to still be bold, run:\n%s", run)
	}
}
