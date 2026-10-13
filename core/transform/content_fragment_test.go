// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"fmt"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
	"strings"
	"testing"
)

func TestContentFragmentHeadingsAndStableAnchors(t *testing.T) {
	original := "Before\n## Same\nAfter\n### Same\n## Explicit {#author-anchor}"
	options := parser.ContentOptions{AnchorNamespace: "deck/slot/block"}
	r := NormalizeMarkdownContent(original, options)
	if !r.Receipt.Accepted {
		t.Fatalf("%+v", r.Receipt)
	}
	if len(r.Elements) != 5 {
		t.Fatalf("elements=%d", len(r.Elements))
	}
	h := r.Elements[1].(*ast.HeadingElement)
	if h.Level != 2 || h.Text != "Same" {
		t.Fatalf("%+v", h)
	}
	if r.Elements[4].(*ast.HeadingElement).Anchor != "author-anchor" {
		t.Fatal("explicit author anchor changed")
	}
	repeated := NormalizeMarkdownContent(original, options)
	if repeated.Source != r.Source {
		t.Fatal("unstable source")
	}
	if strings.Contains(r.Source, "SLIDE") || strings.Contains(r.Source, "ast_capabilities") {
		t.Fatal("fragment has document authority")
	}
	wrapper := "---\nmode: strict\nast_capabilities: [typed-headings-v1, content-markdown-v1]\n---\nSLIDE content\n" + r.Source + "\nSLIDE content\n  TEXT\n    Other\n"
	doc, diags := parser.New(util.NewNoop()).Parse(wrapper, "")
	for _, d := range diags {
		if d.IsError() {
			t.Fatal(d)
		}
	}
	if len(doc.ContentBlocks) != 2 || doc.ContentBlocks[0].Elements[1].(*ast.HeadingElement).Anchor != h.Anchor {
		t.Fatal("assembly changed authority")
	}
}
func TestContentFragmentLevelsOptInAndLegacy(t *testing.T) {
	for level := 1; level <= 6; level++ {
		r := NormalizeMarkdownContent(strings.Repeat("#", level)+" Title", parser.ContentOptions{AnchorNamespace: "levels"})
		if !r.Receipt.Accepted {
			t.Fatalf("level%d %+v", level, r.Receipt)
		}
	}
	source := "---\nmode: strict\nast_capabilities: [typed-headings-v1]\n---\nSLIDE content\n  SECTION \"Two\"\n    level: 2\n"
	_, ds := parser.New(util.NewNoop()).Parse(source, "")
	errors := false
	for _, d := range ds {
		errors = errors || d.IsError()
	}
	if !errors {
		t.Fatal("legacy meaning widened")
	}
}
func TestContentFragmentLiteralCode(t *testing.T) {
	for count := 0; count <= 3; count++ {
		for _, prefix := range []string{"", "  ", "\t"} {
			code := prefix + "one\n" + prefix + "\ttwo" + strings.Repeat("\n", count)
			source := "```bash\n" + code + "\n```\n\nAfter"
			r := NormalizeMarkdownContent(source, parser.ContentOptions{AnchorNamespace: fmt.Sprintf("code/%d/%s", count, prefix)})
			if !r.Receipt.Accepted {
				t.Fatalf("count%d prefix%q %+v", count, prefix, r.Receipt)
			}
			if r.Elements[0].(*ast.CodeElement).Content != code {
				t.Fatal("CODE bytes changed")
			}
			if r.Elements[1].(*ast.TextElement).Content != "After" {
				t.Fatal("following text consumed")
			}
		}
	}
}
func TestContentFragmentWarningAndRejects(t *testing.T) {
	r := NormalizeMarkdownContent("> [!WARNING]\n> Retain **this** body.", parser.ContentOptions{AnchorNamespace: "warning"})
	if !r.Receipt.Accepted {
		t.Fatalf("%+v", r.Receipt)
	}
	s := r.Elements[0].(*ast.SpecialBlockElement)
	if s.BlockType != "warning" || s.Content != "Retain **this** body." {
		t.Fatalf("%+v", s)
	}
	for _, text := range []string{"## A {#same}\n## B {#same}", "```bash\nmissing", "> [!WARNING]\n> first\n> second", "---\nmode: flex\n---"} {
		r = NormalizeMarkdownContent(text, parser.ContentOptions{AnchorNamespace: "reject"})
		if r.Receipt.Accepted || r.Source != "" {
			t.Fatalf("accepted %q", text)
		}
	}
}
