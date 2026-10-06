// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// `CODE{verbatim}` fixes the structural indentation of the body (the header's
// plus two spaces) instead of deducing it from the lines that share it, so a
// body whose lines are all indented, or whose first line is the most indented,
// keeps that indentation.
func TestStrictCodeVerbatim_KeepsTheIndentationPastTheStructure(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"every line indented with spaces", "  CODE{verbatim} go\n      a := 1\n      b := 2", "  a := 1\n  b := 2"},
		{"every line starts with a tab", "  CODE{verbatim} go\n    \tfoo()\n    \tbar()", "\tfoo()\n\tbar()"},
		{"first line deeper than later", "  CODE{verbatim} go\n        x\n      y", "    x\n  y"},
		{"no indentation past the structure", "  CODE{verbatim} go\n    a\n    b", "a\nb"},
		{"blank line in the middle", "  CODE{verbatim} go\n      a\n\n      b", "  a\n\n  b"},
		{"whitespace-only line keeps what exceeds the base", "  CODE{verbatim} go\n      a\n      \n      b", "  a\n  \n  b"},
		{"nothing but a whitespace-only line", "  CODE{verbatim} go\n      ", "  "},
		// A body written by hand with less indentation than header + 2 loses
		// only the indentation it has.
		{"less than the structure", "  CODE{verbatim} go\n   a\n   b", "a\nb"},
		{"one line less than the structure", "  CODE{verbatim} go\n    a\n   b", "a\nb"},
		{"no language", "  CODE{verbatim}\n      a", "  a"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code := codeOf(t, strictSlideBody(t, c.body))
			if code.Content != c.want {
				t.Errorf("content %q, want %q", code.Content, c.want)
			}
		})
	}
}

func TestStrictCodeVerbatim_HeaderKeepsLanguageAndFilename(t *testing.T) {
	for header, want := range map[string][2]string{
		"CODE{verbatim} go":                  {"go", ""},
		"CODE{verbatim} go main.go":          {"go", "main.go"},
		"CODE{verbatim}":                     {"", ""},
		"CODE{verbatim} python {1,3-5}":      {"python {1,3-5}", ""},
		"CODE{verbatim}  ts   renewals.ts  ": {"ts", "renewals.ts"},
	} {
		code := codeOf(t, strictSlideBody(t, "  "+header+"\n      a"))
		if code.Language != want[0] || code.Filename != want[1] || code.Content != "  a" {
			t.Errorf("%q: language %q filename %q content %q, want %q %q and %q", header, code.Language, code.Filename, code.Content, want[0], want[1], "  a")
		}
	}
}

// The same body without the attribute is read as before: the shared
// indentation is structure.
func TestStrictCodeVerbatim_PlainCodeIsUnchanged(t *testing.T) {
	code := codeOf(t, strictSlideBody(t, "  CODE go\n      a := 1\n      b := 2"))
	if code.Content != "a := 1\nb := 2" {
		t.Errorf("content %q", code.Content)
	}
}

// The block ends where any CODE body does: at a line that is not indented deeper
// than its header. The next element is not swallowed.
func TestStrictCodeVerbatim_EndsAtTheNextElement(t *testing.T) {
	els := strictSlideBody(t, "  CODE{verbatim} go\n      a\n  TEXT\n    after")
	if len(els) != 2 {
		t.Fatalf("want the code and the TEXT, got %d elements", len(els))
	}
	if txt, ok := els[1].(*ast.TextElement); !ok || !strings.Contains(txt.Content, "after") {
		t.Errorf("second element is %#v", els[1])
	}
}

// A TEXT paragraph stops where a CODE{verbatim} starts, as it does for CODE.
func TestStrictCodeVerbatim_EndsAParagraph(t *testing.T) {
	els := strictSlideBody(t, "  TEXT\n    A paragraph\n  CODE{verbatim} go\n      a")
	if len(els) != 2 {
		t.Fatalf("want a TEXT and a code, got %d elements: %#v", len(els), els)
	}
	if _, ok := els[1].(*ast.CodeElement); !ok {
		t.Errorf("second element is %#v, want the code", els[1])
	}
}

// A node-id above a CODE{verbatim} names the code, and a line that looks like a
// node-id inside the body is code, even when the body has lines that are less
// indented than its first one.
func TestStrictCodeVerbatim_NodeIDs(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  <!-- node-id: Snippet -->\n  CODE{verbatim} html\n        <div>\n      <!-- node-id: NotAnID -->\n  TEXT\n    after\n"
	doc, diags := New(util.NewNoop()).Parse(src, "t.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("does not parse: %s", d.String())
		}
	}
	els := doc.ContentBlocks[0].Elements
	code := codeOf(t, els)
	if code.NodeID != "Snippet" {
		t.Errorf("node id %q, want Snippet", code.NodeID)
	}
	if want := "    <div>\n  <!-- node-id: NotAnID -->"; code.Content != want {
		t.Errorf("content %q, want %q", code.Content, want)
	}
}

// The same grammar in a strict document.
func TestStrictCodeVerbatim_DocumentSection(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSECTION \"S\"\n  level: 1\n  CODE{verbatim} go\n      a := 1"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "t.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("does not parse: %s", d.String())
		}
	}
	if got := codeOf(t, doc.ContentBlocks[0].Elements).Content; got != "  a := 1" {
		t.Errorf("content %q", got)
	}
}
