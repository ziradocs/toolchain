// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestFlexToStrict_AttributedQuoteAfterCode(t *testing.T) {
	src := "---\nmode: flex\n---\n\n---\nlayout: content\n---\n# Example\n\nThis is not an approval.\n\n```python\nstatus = \"pending\"\nprint(\"TABLE | SLIDE | ---\")\n```\n\n> Verify the result.\n>\n> — Example author, https://example.org/reference\n"
	doc := parseSlides(t, src)
	if len(doc.ContentBlocks) != 1 || len(doc.ContentBlocks[0].Elements) != 3 {
		t.Fatalf("unexpected slide shape: %+v", doc.ContentBlocks)
	}
	quote := doc.ContentBlocks[0].Elements[2].(*ast.QuoteElement)
	if quote.Content != "Verify the result.\n" || quote.Author != "Example author, https://example.org/reference" {
		t.Fatalf("unexpected quote: %+v", quote)
	}
	code := doc.ContentBlocks[0].Elements[1].(*ast.CodeElement)
	// The fence delimiter's preceding newline is not part of the parsed
	// payload. Compare the AST payload exactly, including any retained blanks.
	if code.Language != "python" || code.Content != "status = \"pending\"\nprint(\"TABLE | SLIDE | ---\")" {
		t.Fatalf("unexpected code: %+v", code)
	}
	doc.ContentBlocks[0].NodeID = "ExampleSlide"
	for i, id := range []string{"ExampleText", "ExampleCode", "ExampleQuote"} {
		doc.ContentBlocks[0].Elements[i].(ast.IdentityNode).SetNodeID(id)
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	got := parseSlides(t, out)
	if len(got.ContentBlocks) != 1 || !reflect.DeepEqual(normalizeBlock(doc.ContentBlocks[0]), normalizeBlock(got.ContentBlocks[0])) {
		t.Fatalf("content or identities changed:\nwant: %s\ngot: %s\n%s", toJSON(t, doc.ContentBlocks), toJSON(t, got.ContentBlocks), out)
	}
	again, err := FormatStrict(got)
	if err != nil || again != out {
		t.Fatalf("format is not idempotent: %v\n%s\n%s", err, out, again)
	}
}

func TestFlexToStrict_AttributedQuoteBeforeChecklist(t *testing.T) {
	src := "---\nmode: flex\ntitle: Roundtrip\nlang: es-MX\n---\n\n---\nlayout: content\n---\n# Example\n\n> Do not omit the condition.\n>\n> — Example author, https://example.org/reference\n\n- [x] Repeated action\n- [ ] Repeated action\n"
	doc := parseSlides(t, src)
	if len(doc.ContentBlocks) != 1 || len(doc.ContentBlocks[0].Elements) != 2 {
		t.Fatalf("unexpected slide shape: %+v", doc.ContentBlocks)
	}
	quote := doc.ContentBlocks[0].Elements[0].(*ast.QuoteElement)
	if quote.Content != "Do not omit the condition.\n" || quote.Author != "Example author, https://example.org/reference" || quote.Source != "" {
		t.Fatalf("unexpected quote: %+v", quote)
	}
	checklist := doc.ContentBlocks[0].Elements[1].(*ast.ChecklistElement)
	if len(checklist.Items) != 2 || checklist.Items[0].Content != "Repeated action" || checklist.Items[1].Content != "Repeated action" || !checklist.Items[0].Checked || checklist.Items[1].Checked {
		t.Fatalf("unexpected checklist: %+v", checklist)
	}
	doc.ContentBlocks[0].NodeID = "ExampleSlide"
	quote.NodeID = "ExampleQuote"
	checklist.NodeID = "ExampleChecklist"
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	got := parseSlides(t, out)
	want := normalizeForComparison(doc)
	want.FrontMatter.Mode = "strict"
	if !reflect.DeepEqual(want, normalizeForComparison(got)) {
		t.Fatalf("content, metadata or identities changed:\nwant: %s\ngot: %s\n%s", toJSON(t, want), toJSON(t, normalizeForComparison(got)), out)
	}
	again, err := FormatStrict(got)
	if err != nil || again != out {
		t.Fatalf("format is not idempotent: %v\n%s\n%s", err, out, again)
	}
}

func TestFormatStrict_QuoteTrailingBlankLinesBeforeMetadata(t *testing.T) {
	for _, count := range []int{1, 2} {
		for _, metadata := range []string{"author", "source", "both"} {
			t.Run(metadata+strings.Repeat("_blank", count), func(t *testing.T) {
				q := ast.NewQuoteElement(diagnostics.NewPosition(3, 1), "Example."+strings.Repeat("\n", count))
				if metadata != "source" {
					q.Author = "Example author"
				}
				if metadata != "author" {
					q.Source = "https://example.org/reference"
				}
				doc := chartDoc(q)
				out, err := FormatStrict(doc)
				if err != nil {
					t.Fatalf("FormatStrict: %v", err)
				}
				got := parseSlides(t, out)
				if !reflect.DeepEqual(quoteContents(doc), quoteContents(got)) {
					t.Fatalf("quote changed: %q -> %q\n%s", quoteContents(doc), quoteContents(got), out)
				}
			})
		}
	}
}

func TestFormatStrict_QuoteLeadingBlankLinesWithMetadataStillRefused(t *testing.T) {
	for _, content := range []string{"\nExample.", "\n", "\n\n"} {
		q := ast.NewQuoteElement(diagnostics.NewPosition(3, 1), content)
		q.Author = "Example author"
		q.Source = "https://example.org/reference"
		_, err := FormatStrict(chartDoc(q))
		var unsupported *UnsupportedElementError
		if !errors.As(err, &unsupported) || unsupported.NodeType != "quote" {
			t.Errorf("content %q: want unsupported quote, got %v", content, err)
		}
	}
}
