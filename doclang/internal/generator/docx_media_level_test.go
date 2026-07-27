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
