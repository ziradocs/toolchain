// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// strictSlideBody parses a strict slide whose body is `body` (already indented
// as the test wants it) and returns the elements of the first slide. The final
// newline of body is dropped: the parser reads a line left empty at the end of
// the input as a blank line of the code, which TestStrictCode_BlankLinesAfterTheBody
// covers on its own.
func strictSlideBody(t *testing.T, body string) []ast.Element {
	t.Helper()
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n" + strings.TrimSuffix(body, "\n")
	doc, diags := New(util.NewNoop()).Parse(src, "t.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("does not parse: %s\n%s", d.String(), src)
		}
	}
	if len(doc.ContentBlocks) == 0 {
		t.Fatal("no slide")
	}
	return doc.ContentBlocks[0].Elements
}

func codeOf(t *testing.T, els []ast.Element) *ast.CodeElement {
	t.Helper()
	for _, el := range els {
		if c, ok := el.(*ast.CodeElement); ok {
			return c
		}
	}
	t.Fatalf("no code element among %d elements", len(els))
	return nil
}

// The body of a CODE block is every line indented deeper than its header. A
// first line that is more indented than a later one used to end the block there
// and the rest of the body vanished with no diagnostic (issue #400).
func TestStrictCode_FirstLineDeeperThanLaterLines(t *testing.T) {
	els := strictSlideBody(t, "  CODE typescript\n        foo();\n      }\n")
	if len(els) != 1 {
		t.Fatalf("want one element, got %d", len(els))
	}
	if got, want := codeOf(t, els).Content, "  foo();\n}"; got != want {
		t.Errorf("content %q, want %q", got, want)
	}
}

// A tab is one character of indentation, not four columns that only a run of
// spaces can remove: spaces followed by a tab used to stay in the content.
func TestStrictCode_SpacesThenTabIsDedented(t *testing.T) {
	els := strictSlideBody(t, "  CODE typescript\n    \tx\n    \ty\n")
	if got, want := codeOf(t, els).Content, "x\ny"; got != want {
		t.Errorf("content %q, want %q", got, want)
	}
}

func TestStrictCode_IndentShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"canonical, two spaces under the header", "  CODE go\n    func a() {\n    \tb()\n    }\n", "func a() {\n\tb()\n}"},
		{"deeper than two, the first line is the least indented", "  CODE go\n        a()\n          b()\n", "a()\n  b()"},
		{"tabs inside a space-indented body are content", "  CODE go\n    a\n    \tb\n", "a\n\tb"},
		{"blank line in the middle", "  CODE go\n    a\n\n    b\n", "a\n\nb"},
		{"whitespace-only line keeps what exceeds the base", "  CODE go\n    a\n      \n    b\n", "a\n  \nb"},
		{"whitespace-only line shorter than the base is empty", "  CODE go\n    a\n  \n    b\n", "a\n\nb"},
		{"no body", "  CODE go\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := codeOf(t, strictSlideBody(t, c.body)).Content; got != c.want {
				t.Errorf("content %q, want %q", got, c.want)
			}
		})
	}
}

// The next element at the header's own indentation ends the block and is not
// swallowed, whatever the indentation of the body was.
func TestStrictCode_NextElementAtHeaderIndentIsNotSwallowed(t *testing.T) {
	for name, body := range map[string]string{
		"canonical":    "  CODE go\n    a()\n  TEXT\n    after\n",
		"first deeper": "  CODE go\n        a()\n      }\n  TEXT\n    after\n",
		"empty body":   "  CODE go\n  TEXT\n    after\n",
	} {
		t.Run(name, func(t *testing.T) {
			els := strictSlideBody(t, body)
			if len(els) != 2 {
				t.Fatalf("want a code and a text element, got %d: %#v", len(els), els)
			}
			txt, ok := els[1].(*ast.TextElement)
			if !ok || !strings.Contains(txt.Content, "after") {
				t.Errorf("second element is %#v, want the TEXT", els[1])
			}
		})
	}
}

// The same grammar in a strict document.
func TestStrictCode_DocumentSection(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSECTION \"S\"\n  level: 1\n  CODE typescript\n        foo();\n      }"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "t.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("does not parse: %s", d.String())
		}
	}
	if got, want := codeOf(t, doc.ContentBlocks[0].Elements).Content, "  foo();\n}"; got != want {
		t.Errorf("content %q, want %q", got, want)
	}
}

// Blank lines that follow the body are read as part of the code, as they always
// were: the formatter relies on it to write a code body that ends in newlines.
func TestStrictCode_BlankLinesAfterTheBody(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  CODE go\n    a\n\n\n  TEXT\n    after\n"
	doc, _ := New(util.NewNoop()).Parse(src, "t.slidelang")
	els := doc.ContentBlocks[0].Elements
	if got, want := codeOf(t, els).Content, "a\n\n"; got != want {
		t.Errorf("content %q, want %q", got, want)
	}
	if len(els) != 2 {
		t.Errorf("want the code and the TEXT, got %d elements", len(els))
	}
}
