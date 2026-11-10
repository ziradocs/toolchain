// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

func strictCode(t *testing.T, header string) (*ast.CodeElement, []string) {
	t.Helper()
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  " + header + "\n    a = 1\n"
	doc, diags := New(util.NewNoop()).Parse(src, "t.slidelang")
	var errs []string
	for _, d := range diags {
		if d.IsError() {
			errs = append(errs, d.Message)
		}
	}
	for _, el := range doc.ContentBlocks[0].Elements {
		if c, ok := el.(*ast.CodeElement); ok {
			return c, errs
		}
	}
	t.Fatal("no code element")
	return nil, nil
}

// A second word that starts with { or [ is not a file name: it stays in the
// language, the way a flex fence keeps it, so fmt can write what flex stored.
// Highlighted lines have no meaning of their own in strict.
func TestStrictCodeHeader_BraceOrBracketStaysInTheLanguage(t *testing.T) {
	for header, want := range map[string]string{
		"CODE python {1,3-5}":       "python {1,3-5}",
		"CODE ts [tab label]":       "ts [tab label]",
		"CODE python  {1,3-5}":      "python  {1,3-5}",
		"CODE python {1,3-5} extra": "python {1,3-5} extra",
	} {
		c, errs := strictCode(t, header)
		if len(errs) != 0 {
			t.Errorf("%q: unexpected errors %v", header, errs)
		}
		if c.Language != want || c.Filename != "" {
			t.Errorf("%q: language %q filename %q, want %q and none", header, c.Language, c.Filename, want)
		}
	}
}

// The forms that were valid keep their meaning.
func TestStrictCodeHeader_ExistingFormsAreUnchanged(t *testing.T) {
	c, errs := strictCode(t, "CODE ts renewals.ts")
	if len(errs) != 0 || c.Language != "ts" || c.Filename != "renewals.ts" {
		t.Errorf("language %q filename %q errors %v", c.Language, c.Filename, errs)
	}
	c, errs = strictCode(t, "CODE python")
	if len(errs) != 0 || c.Language != "python" || c.Filename != "" {
		t.Errorf("language %q filename %q errors %v", c.Language, c.Filename, errs)
	}
	_, errs = strictCode(t, "CODE ts a.ts extra")
	if len(errs) == 0 {
		t.Error("a fourth word after a file name is still an error")
	}
}

// Strict code groups use fences with a [label], not a CODE header, and a CODE
// line next to them reads as before.
func TestStrictCodeHeader_CodeGroupsAreUnaffected(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  :::code-group\n  ```ts [a.ts]\n  const a = 1\n  ```\n  ```py [b.py]\n  a = 1\n  ```\n  :::\n  CODE ts b.ts\n    x\n"
	doc, diags := New(util.NewNoop()).Parse(src, "t.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error: %s", d.Message)
		}
	}
	var group *ast.CodeGroupElement
	var code *ast.CodeElement
	for _, el := range doc.ContentBlocks[0].Elements {
		switch n := el.(type) {
		case *ast.CodeGroupElement:
			group = n
		case *ast.CodeElement:
			code = n
		}
	}
	if group == nil || len(group.CodeBlocks) != 2 || group.CodeBlocks[0].Label != "a.ts" {
		t.Fatalf("code group read wrong: %+v", group)
	}
	if code == nil || code.Language != "ts" || code.Filename != "b.ts" {
		t.Errorf("CODE after the group: %+v", code)
	}
}

func TestStrictCodeHeader_DocumentsReadItToo(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n  CODE python {1,3-5}\n    a = 1\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "t.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error: %s", d.Message)
		}
	}
	for _, el := range doc.ContentBlocks[0].Elements {
		if c, ok := el.(*ast.CodeElement); ok {
			if c.Language != "python {1,3-5}" || c.Filename != "" {
				t.Errorf("language %q filename %q", c.Language, c.Filename)
			}
			return
		}
	}
	t.Fatal("no code element")
}

// The rule needs the whole word CODE: a line that only starts with those letters
// keeps being read as it was before (as an error here), and is not given a language.
func TestStrictCodeHeader_NeedsTheWholeWordCODE(t *testing.T) {
	c, errs := strictCode(t, "CODEX python {1}")
	if len(errs) == 0 {
		t.Errorf("CODEX python {1}: expected the error it always gave, got language %q", c.Language)
	}
	if c != nil && c.Language == "X python {1}" {
		t.Errorf("CODEX must not be read as CODE with the language %q", c.Language)
	}
}
