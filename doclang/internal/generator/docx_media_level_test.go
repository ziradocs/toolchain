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

// astWithElements builds a minimal one-section AST wrapping the given
// elements, for tests that only care about a single element's rendering.
func astWithElements(elems ...ast.Element) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)

	front := ast.NewFrontMatterNode(pos)
	front.Title = "Fixture"
	doc.FrontMatter = front

	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "Section"
	block.Elements = append(block.Elements, elems...)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

// TestDOCXGenerator_RenderMedia_EmitsStyledLinkText covers issue #36: a
// MediaElement with a safe source must render the real URL as visible,
// link-styled text (docxgo has no video/audio embedding). It does NOT
// assert a <w:hyperlink> field: docxgo v2.1.1's AddHyperlink registers the
// relationship but never wraps the run in one (see the comment on
// renderMedia in docx.go) — the link is visually styled but not actually
// clickable, a known upstream limitation, not something this code can fix.
func TestDOCXGenerator_RenderMedia_EmitsStyledLinkText(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithElements(ast.NewMediaElement(diagnostics.NewPosition(1, 1), "video", "https://example.com/demo.mp4"))

	output := filepath.Join(t.TempDir(), "media.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if !strings.Contains(xml, "video: https://example.com/demo.mp4") {
		t.Errorf("generated DOCX missing the media link display text, document.xml:\n%s", xml)
	}
	if !strings.Contains(xml, `w:color w:val="0000FF"`) {
		t.Errorf("expected the hyperlink-blue styling AddHyperlink applies, document.xml:\n%s", xml)
	}
}

// TestDOCXGenerator_RenderMedia_EmptySourceGetsDistinctPlaceholder and
// TestDOCXGenerator_RenderMedia_BlockedSourceGetsDistinctPlaceholder mirror
// core/renderer's renderMediaElement contract (core/renderer/html_test.go):
// an empty source and a SanitizeURL-blocked source must produce visibly
// different messages, and neither must go through AddHyperlink's styling —
// only a genuinely safe, non-empty source does.
func TestDOCXGenerator_RenderMedia_EmptySourceGetsDistinctPlaceholder(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithElements(ast.NewMediaElement(diagnostics.NewPosition(1, 1), "video", ""))

	output := filepath.Join(t.TempDir(), "media.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if !strings.Contains(xml, "video sin fuente") {
		t.Errorf("expected the empty-source placeholder, got document.xml:\n%s", xml)
	}
	if strings.Contains(xml, `w:color w:val="0000FF"`) {
		t.Error("an empty source must not go through AddHyperlink's blue link styling")
	}
}

func TestDOCXGenerator_RenderMedia_BlockedSourceGetsDistinctPlaceholder(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithElements(ast.NewMediaElement(diagnostics.NewPosition(1, 1), "audio", "javascript:alert(1)"))

	output := filepath.Join(t.TempDir(), "media.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if !strings.Contains(xml, "audio bloqueado por seguridad") {
		t.Errorf("expected the blocked-source placeholder, got document.xml:\n%s", xml)
	}
	if strings.Contains(xml, `w:color w:val="0000FF"`) {
		t.Error("a blocked source must not go through AddHyperlink's blue link styling")
	}
}

// TestDOCXGenerator_RenderMedia_QueryStringNotDoubleEscaped covers a finding
// from the #40 code review: renderMedia used to sanitize Source with
// renderer.SanitizeURL, which layers EscapeHTMLAttribute on top of the
// scheme allowlist — turning "&" into the literal string "&amp;" BEFORE the
// docx XML writer ever sees it. The writer then XML-escapes that "&" too
// (correctly, on its own terms), producing a double-escaped "&amp;amp;" in
// document.xml — which round-trips back to the WRONG string ("&amp;b=2"
// instead of "&b=2") when a real XML parser reads it. ValidateURLScheme
// hands the writer the raw "&" so it gets escaped exactly once, the
// standard/correct amount for XML text content.
func TestDOCXGenerator_RenderMedia_QueryStringNotDoubleEscaped(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	source := "https://cdn.example.com/v.mp4?a=1&b=2"
	doc := astWithElements(ast.NewMediaElement(diagnostics.NewPosition(1, 1), "video", source))

	output := filepath.Join(t.TempDir(), "media.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if strings.Contains(xml, "&amp;amp;") {
		t.Errorf("media URL query string got double XML-escaped, document.xml:\n%s", xml)
	}
	if !strings.Contains(xml, "&amp;b=2") {
		t.Errorf("expected a single, correct XML-escape of the query string's \"&\", document.xml:\n%s", xml)
	}
}

// TestDOCXGenerator_RenderText_LevelWithRawMarkdownContent covers a finding
// from the #40 code review: the Level > 0 short-circuit only tried the
// end-anchored HTML pattern, so a TextElement with Level populated but
// Content still in raw Markdown form (`## Resumen`) — reachable via an
// external --filter AST, or before HTML normalization runs — rendered the
// literal "## Resumen" as the heading text instead of "Resumen".
func TestDOCXGenerator_RenderText_LevelWithRawMarkdownContent(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	el := ast.NewTextElement(diagnostics.NewPosition(1, 1), "## Resumen")
	el.Level = 2
	doc := astWithElements(el)

	output := filepath.Join(t.TempDir(), "heading.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if strings.Contains(xml, "## Resumen") {
		t.Errorf("raw Markdown heading markup leaked into the heading text, document.xml:\n%s", xml)
	}
	if !strings.Contains(xml, ">Resumen<") {
		t.Errorf("expected the extracted heading text \"Resumen\", document.xml:\n%s", xml)
	}
}

// TestDOCXGenerator_RenderText_LevelWithTrailingContentIsNotSilentlyDropped
// covers a finding from external review of #49: headingHTMLPattern and
// markdownHeadingPattern stopped requiring the match to consume the whole
// string (removing the "$" anchor was needed to tolerate a trailing
// newline, see the other test in this file) but initially had NO trailing
// anchor at all, so Content combining a heading with genuinely different
// content after it (`<h2>Título</h2><p>Contenido importante</p>`, reachable
// via an external --filter AST) matched only up to the first closing tag —
// the heading text was extracted correctly, but everything after it
// (a whole paragraph) silently vanished instead of being preserved or at
// least surfaced as raw text. The patterns now require only trailing
// WHITESPACE after the match ("\s*$"), so genuine extra content makes the
// match fail entirely and Content falls through to the raw-text fallback
// instead of being dropped.
func TestDOCXGenerator_RenderText_LevelWithTrailingContentIsNotSilentlyDropped(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	el := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1), `<h2>Título</h2><p>Contenido importante</p>`)
	el.Level = 2
	doc := astWithElements(el)

	output := filepath.Join(t.TempDir(), "heading.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	if !strings.Contains(xml, "Contenido importante") {
		t.Errorf("trailing content after the heading tag was silently dropped, document.xml:\n%s", xml)
	}
}

// TestDOCXGenerator_RenderText_UsesLevelInsteadOfRegex covers issue #22: a
// TextElement with Level populated must use it directly rather than
// re-parsing the rendered <hN> — pinned by exercising levels the old
// regex-only path never handled at all (1, 5) alongside a level it did (3),
// to prove Level, not the regex, is now driving the level selection.
func TestDOCXGenerator_RenderText_UsesLevelInsteadOfRegex(t *testing.T) {
	tests := []struct {
		name       string
		level      int
		content    string
		wantText   string
		wantPStyle string
	}{
		{
			name:       "level 1 now gets its own Heading1 style (previously unreachable — no regex ever matched h1)",
			level:      1,
			content:    `<h1 id="top">Top Level</h1>`,
			wantText:   "Top Level",
			wantPStyle: "Heading1",
		},
		{
			name:       "level 3 matches the pre-existing regex path's level exactly",
			level:      3,
			content:    `<h3 id="mid">Middle</h3>`,
			wantText:   "Middle",
			wantPStyle: "Heading3",
		},
		{
			name:       "level 5 degrades explicitly to Heading4 (no H5 style data exists in docx_styles.go)",
			level:      5,
			content:    `<h5 id="deep">Deep</h5>`,
			wantText:   "Deep",
			wantPStyle: "Heading4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := newTestLogger()
			gen := New(logger)
			el := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1), tt.content)
			el.Level = tt.level
			doc := astWithElements(el)

			output := filepath.Join(t.TempDir(), "heading.docx")
			if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			xml := docxDocumentXML(t, output)
			if !strings.Contains(xml, tt.wantText) {
				t.Errorf("missing extracted heading text %q, document.xml:\n%s", tt.wantText, xml)
			}
			if strings.Contains(xml, "<h"+string(rune('0'+tt.level))) {
				t.Errorf("raw HTML tag leaked into document.xml instead of being extracted")
			}
			if !strings.Contains(xml, `w:val="`+tt.wantPStyle+`"`) {
				t.Errorf("expected paragraph style %q, document.xml:\n%s", tt.wantPStyle, xml)
			}
		})
	}
}
