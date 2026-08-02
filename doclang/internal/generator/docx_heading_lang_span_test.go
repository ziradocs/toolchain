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

// TestDOCXGenerator_CollectHeadings_StripsLangSpanMarkup covers issue #63
// code review finding #1: a heading's Content is already-rendered HTML by
// parse time (parser.parseSubsectionHeader), so a [texto]{lang=xx} span
// inside a heading shows up here as literal <span lang="fr">...</span>
// markup. Before this fix, that markup leaked into the static TOC entry's
// display text AND corrupted sanitizeBookmarkID's output (the ASCII
// allowlist strips the markup's punctuation but not its tag/attribute
// letters, e.g. "Hola_spanlangfrmondespan" instead of "Hola_monde").
func TestDOCXGenerator_CollectHeadings_StripsLangSpanMarkup(t *testing.T) {
	logger := newTestLogger()
	gen := NewDOCXGenerator(logger, "")

	heading := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1),
		`<h2 id="x">Bonjour <span lang="fr">tout le monde</span></h2>`)
	heading.Level = 2
	doc := astWithElements(heading)

	entries := gen.collectHeadings(doc)
	var entry *TOCEntry
	for i := range entries {
		if entries[i].Level == 2 {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		t.Fatalf("collectHeadings() did not find the H2 entry: %+v", entries)
	}
	if entry.Title != "Bonjour tout le monde" {
		t.Errorf("TOCEntry.Title = %q, want %q", entry.Title, "Bonjour tout le monde")
	}
	if strings.Contains(entry.BookmarkID, "span") || strings.Contains(entry.BookmarkID, "lang") {
		t.Errorf("TOCEntry.BookmarkID leaked span/lang markup: %q", entry.BookmarkID)
	}
	if entry.BookmarkID != "Bonjour_tout_le_monde" {
		t.Errorf("TOCEntry.BookmarkID = %q, want %q", entry.BookmarkID, "Bonjour_tout_le_monde")
	}
}

// TestDOCXGenerator_RenderHeading_LangSpan covers the render side of the
// same finding: the heading paragraph itself must NOT show literal
// "<span lang=...>" text, must carry <w:lang w:val="fr"> on the run for the
// marked passage, and that run must still get the heading's own style
// (size/color/bold) — not the body-prose style renderInlineMarkdown would
// have applied if reused here.
func TestDOCXGenerator_RenderHeading_LangSpan(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)

	heading := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1),
		`<h2 id="x">Bonjour <span lang="fr">tout le monde</span></h2>`)
	heading.Level = 2
	doc := astWithElements(heading)

	output := filepath.Join(t.TempDir(), "heading-lang.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if strings.Contains(xml, "<span") {
		t.Errorf("expected no literal <span> markup in the heading, document.xml:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:t>tout le monde</w:t>`) {
		t.Errorf("expected the lang-span text to survive as its own run, document.xml:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:lang w:val="fr">`) {
		t.Errorf("expected a <w:lang w:val=\"fr\"> on the lang-span run, document.xml:\n%s", xml)
	}

	idx := strings.Index(xml, `<w:t>tout le monde</w:t>`)
	runStart := strings.LastIndex(xml[:idx], "<w:r>")
	run := xml[runStart:idx]
	if !strings.Contains(run, `<w:sz`) {
		t.Errorf("expected the lang-span run to still carry the heading's own size, run:\n%s", run)
	}
}

// TestDOCXGenerator_RenderHeading_NestedBold covers a pre-existing (not
// #63-specific) instance of the same bug class this fix closes: **bold**
// inside a heading also materializes to already-rendered <strong> HTML by
// parse time, and previously showed up as literal asterisks-turned-tags
// text in the DOCX heading.
func TestDOCXGenerator_RenderHeading_NestedBold(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)

	heading := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1),
		`<h2 id="x">Bonjour <strong>tout le monde</strong></h2>`)
	heading.Level = 2
	doc := astWithElements(heading)

	output := filepath.Join(t.TempDir(), "heading-bold.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if strings.Contains(xml, "<strong") {
		t.Errorf("expected no literal <strong> markup in the heading, document.xml:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:t>tout le monde</w:t>`) {
		t.Errorf("expected the bold text to survive as its own run, document.xml:\n%s", xml)
	}

	idx := strings.Index(xml, `<w:t>tout le monde</w:t>`)
	runStart := strings.LastIndex(xml[:idx], "<w:r>")
	run := xml[runStart:idx]
	if !strings.Contains(run, `<w:b w:val="true">`) {
		t.Errorf("expected the bold run to carry <w:b>, run:\n%s", run)
	}
}
