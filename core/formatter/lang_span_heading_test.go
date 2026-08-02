// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// parseDocumentNoNormalize parses without the content normalizer, which
// restructures/re-emits subsection headers in a way that no longer exercises
// parseSubsectionHeader's own <hN>/IsRawHTML materialization — the exact
// mechanism this test targets. parseDocument (document_roundtrip_test.go),
// which DOES normalize, is the right tool for corpus-style round-trip tests,
// just not for this parser-level behavior.
func parseDocumentNoNormalize(t *testing.T, content string) *parser.DocumentFlexParser {
	t.Helper()
	return parser.NewDocumentFlexParser(content, util.NewNoop())
}

// TestFormatDocument_LangSpanInHeading_RoundTrips covers issue #63 code
// review finding #3: a [texto]{lang=xx} inside a heading (## texto) is
// materialized to <span lang="xx"> at parse time
// (core/parser/document_flex.go), so by the time FormatDocument runs there
// is no [texto]{lang=xx} left to re-emit — only the already-rendered
// <span>. FormatDocument must reconstruct the original source syntax via
// renderer.LangSpanHTMLToSource rather than either losing the language mark
// silently (stripTags alone) or refusing to format the document at all.
func TestFormatDocument_LangSpanInHeading_RoundTrips(t *testing.T) {
	src := "# Título\n\n## Bonjour [tout le monde]{lang=fr}\n\nTexto normal.\n"
	p := parseDocumentNoNormalize(t, src)
	doc, diags := p.Parse()
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("parse error: %s", d.Message)
		}
	}

	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	if !strings.Contains(out, "## Bonjour [tout le monde]{lang=fr}") {
		t.Errorf("expected the heading to round-trip with its lang span intact, got: %q", out)
	}
}

// TestFormatDocument_LangSpanInHeading_NonRoundTrippableTextDegrades
// exercises renderer.LangSpanHTMLToSource's own guard (advisor follow-up on
// finding #3) through FormatDocument: a heading whose materialized span text
// contains "[" cannot be re-emitted as [text]{lang=xx} — that source
// wouldn't re-match InlineLangSpanPattern on the next parse. Not reachable
// via a real [texto]{lang=xx} source today (a "[" inside the span's content
// keeps the parse-time regex from ever creating the span — see
// TestFormatDocument_PlainHeading_StillRoundTrips-adjacent probing), so this
// builds the RawHTML TextElement directly to simulate what a forged
// --filter (or the cross-nesting defect tracked separately for the
// sanitizer) could produce. Must degrade to plain text via stripTags, not
// error and not leak a literal <span> tag.
func TestFormatDocument_LangSpanInHeading_NonRoundTrippableTextDegrades(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewRawHTMLTextElement(pos, `<h2 id="x">Bonjour <span lang="fr">a [b] c</span></h2>`)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, elem)
	doc := ast.NewAST(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	if strings.Contains(out, "<span") {
		t.Errorf("expected the unmatchable span to degrade to plain text, got: %q", out)
	}
	if !strings.Contains(out, "## Bonjour a [b] c") {
		t.Errorf("expected the heading text to survive stripped of the span, got: %q", out)
	}
}

// TestFormatDocument_PlainHeading_StillRoundTrips confirms the lang-span
// handling is scoped correctly — an ordinary heading with no lang span
// (including one with **bold**/*italic*, the pre-existing lossy-but-accepted
// case) must still format without error.
func TestFormatDocument_PlainHeading_StillRoundTrips(t *testing.T) {
	src := "# Título\n\n## Bonjour **tout le monde**\n\nTexto normal.\n"
	p := parseDocumentNoNormalize(t, src)
	doc, diags := p.Parse()
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("parse error: %s", d.Message)
		}
	}

	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	if !strings.Contains(out, "## Bonjour") {
		t.Errorf("expected formatted output to contain the heading, got: %q", out)
	}
}
