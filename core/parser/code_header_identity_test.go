// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// The code parser accepts whitespace separators and legacy CODE prefixes.
// The identity pre-pass must
// recognize the same header as the code parser, or a literal node-id comment
// at the end of the body is removed and assigned to the following element.
func TestCodeHeaderWhitespaceKeepsLiteralNodeID(t *testing.T) {
	for _, keyword := range []string{"CODE", "CODE{verbatim}", "CODEX", "CODE{verbatim}suffix"} {
		for _, separator := range []string{" ", "\t", "\u00a0", "\v", "\f"} {
			for _, document := range []bool{false, true} {
				t.Run(keyword+separator+"/"+map[bool]string{false: "slide", true: "document"}[document], func(t *testing.T) {
					head := "SLIDE content\n  title: \"T\""
					if document {
						head = "SECTION \"T\"\n  level: 1"
					}
					src := "---\nmode: strict\n---\n\n" + head + "\n  <!-- node-id: Snippet -->\n  " + keyword + separator + "html\n    <div>\n    <!-- node-id: Literal -->\n  TEXT\n    after"
					p := New(util.NewNoop())
					var doc *ast.AST
					if document {
						d, diags := p.ParseDocument(src, "t.doclang")
						for _, diagnostic := range diags {
							if diagnostic.IsError() {
								t.Fatalf("ParseDocument: %s", diagnostic.String())
							}
						}
						doc = d
					} else {
						d, diags := p.Parse(src, "t.slidelang")
						for _, diagnostic := range diags {
							if diagnostic.IsError() {
								t.Fatalf("Parse: %s", diagnostic.String())
							}
						}
						doc = d
					}
					if doc == nil || len(doc.ContentBlocks) != 1 || len(doc.ContentBlocks[0].Elements) != 2 {
						t.Fatalf("want one block containing CODE and TEXT, got %#v", doc)
					}
					code, ok := doc.ContentBlocks[0].Elements[0].(*ast.CodeElement)
					if !ok || code.Content != "<div>\n<!-- node-id: Literal -->" || code.NodeID != "Snippet" {
						t.Errorf("literal code or its identity changed: %#v", code)
					}
					text, ok := doc.ContentBlocks[0].Elements[1].(*ast.TextElement)
					if !ok || text.Content != "after" || text.NodeID != "" {
						t.Errorf("following TEXT acquired a literal identity: %#v", text)
					}
				})
			}
		}
	}
}

// Reusing the code parser's recognition in the identity pre-pass must not
// broaden the attribute itself: only a space, tab or end of line activates
// verbatim. Other accepted CODE headers retain plain-code dedenting.
func TestCodeIdentityRecognitionDoesNotBroadenVerbatim(t *testing.T) {
	for _, tc := range []struct {
		header string
		want   string
	}{
		{"CODE{verbatim} html", "  x"},
		{"CODE{verbatim}\thtml", "  x"},
		{"CODE{verbatim}", "  x"},
		{"CODE{verbatim}\u00a0html", "x"},
		{"CODE{verbatim}\vhtml", "x"},
		{"CODE{verbatim}\fhtml", "x"},
		{"CODE{verbatim}suffix html", "x"},
		{"CODEX html", "x"},
	} {
		t.Run(tc.header, func(t *testing.T) {
			code := codeOf(t, strictSlideBody(t, "  "+tc.header+"\n      x"))
			if code.Content != tc.want {
				t.Errorf("body %q, want %q", code.Content, tc.want)
			}
		})
	}
}
