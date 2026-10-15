// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package fragment

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

func TestContentFragmentQuoteAttributionRealWitnesses(t *testing.T) {
	raw, err := os.ReadFile("testdata/quote-attribution-e32.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Witnesses []struct {
			CaseID   string
			Markdown string
			Body     string
			Author   string
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Witnesses) != 10 {
		t.Fatalf("witnesses=%d", len(fixture.Witnesses))
	}
	for i, w := range fixture.Witnesses {
		for _, terminal := range []string{"", "\n"} {
			r := NormalizeMarkdownContent(w.Markdown+terminal, parser.ContentOptions{AnchorNamespace: w.CaseID})
			if !r.Receipt.Accepted || len(r.Elements) != 1 {
				t.Fatalf("witness %d: %+v", i, r.Receipt)
			}
			q, ok := r.Elements[0].(*ast.QuoteElement)
			if !ok || q.Content != w.Body || q.Author != w.Author {
				t.Fatalf("witness %d: %#v", i, r.Elements[0])
			}
			if r.Receipt.OriginalMarkdown != w.Markdown+terminal || len(r.Receipt.Nodes) != 1 {
				t.Fatalf("lost original/mapping %d", i)
			}
		}
	}
}

func TestContentFragmentQuoteAttributionBoundaries(t *testing.T) {
	cases := []struct {
		name, markdown, body, author string
		accepted                     bool
	}{
		{"direct", "> First\n> — Author", "First", "Author", true},
		{"dash", "> First\n>\n> -- Author", "First", "Author", true},
		{"paragraphs", "> First\n>\n> Second\n>\n> — Author", "First\n\nSecond", "Author", true},
		{"multiple internal paragraphs", "> First\n>\n>\n> Second\n>\n> — Author", "First\n\n\nSecond", "Author", true},
		{"no author", "> First\n>\n> Second", "First\n\nSecond", "", true},
		{"ambiguous final separators", "> First\n>\n>\n> — Author", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NormalizeMarkdownContent(c.markdown, parser.ContentOptions{AnchorNamespace: "quote/control"})
			if r.Receipt.Accepted != c.accepted {
				t.Fatalf("%+v", r.Receipt)
			}
			if !c.accepted {
				if r.Source != "" || r.Receipt.OriginalMarkdown != c.markdown {
					t.Fatal("rejected source/original changed")
				}
				return
			}
			q := r.Elements[0].(*ast.QuoteElement)
			if q.Content != c.body || q.Author != c.author {
				t.Fatalf("%#v", q)
			}
			// An ordinary document still uses the historical flex grammar.
			legacy, diags := parser.New(util.NewNoop()).Parse("---\nmode: flex\n---\n# Slide\n\n"+c.markdown, "quote.slidelang")
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("legacy: %+v", diags)
				}
			}
			l := legacy.ContentBlocks[0].Elements[0].(*ast.QuoteElement)
			want := c.body
			if c.author != "" && strings.Contains(c.markdown, "\n>\n>") {
				want += "\n"
			}
			if l.Content != want || l.Author != c.author {
				t.Fatalf("legacy changed: %#v want %q", l, want)
			}
		})
	}
}
