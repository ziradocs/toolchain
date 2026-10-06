// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// The inline Markdown of a ###..###### heading survives fmt (issue #260): the
// parser keeps the text the author wrote next to the HTML it rendered, and the
// formatter writes that text back instead of stripping the tags. The comparison
// is the full AST including the rendered HTML, so a heading that lost its
// <strong> or <code> fails it.
var headingEmphasisShapes = []string{
	"**Foo**",
	"Con `código` aquí",
	"*cursiva* y [enlace](https://example.com)",
	"Mixto ***ambos*** y [en](https://example.com) fin",
	"**negrita con `código` dentro**",
}

func TestFlexToStrict_SlideHeadingKeepsInlineEmphasis(t *testing.T) {
	for _, text := range headingEmphasisShapes {
		t.Run(text, func(t *testing.T) {
			src := "---\nmode: flex\n---\n\n## One\n\n### " + text + "\n\nbody\n"
			diffs, out := flexCorpusASTDiffs(t, src)
			if len(diffs) != 0 {
				t.Fatalf("the AST changed:\n  %s\n%s", strings.Join(diffs, "\n  "), out)
			}
			if !strings.Contains(out, `SECTION "`+text+`"`) {
				t.Errorf("the heading text was not written back as authored:\n%s", out)
			}
		})
	}
}

func TestFormatDocument_HeadingKeepsInlineEmphasis(t *testing.T) {
	for _, text := range headingEmphasisShapes {
		t.Run(text, func(t *testing.T) {
			src := "---\nmode: flex\ntitle: T\n---\n\n# Section\n\n### " + text + "\n\nbody\n"
			doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "d.doclang")
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("does not parse: %s", d.String())
				}
			}
			out, err := FormatDocument(doc)
			if err != nil {
				t.Fatalf("FormatDocument: %v", err)
			}
			if !strings.Contains(out, "### "+text+"\n") {
				t.Errorf("the heading lost its inline formatting:\n%s", out)
			}
		})
	}
}
