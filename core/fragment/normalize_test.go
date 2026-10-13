// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package fragment

import (
	"fmt"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
	"os"
	"path/filepath"
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

func TestContentFragmentLocalLinks(t *testing.T) {
	for _, source := range []string{"## Title {#author-anchor}\n[Go](#author-anchor)", "Inline `[Go](#not-a-link)`", "```bash\n[Go](#not-a-navigation-link)\n```"} {
		r := NormalizeMarkdownContent(source, parser.ContentOptions{AnchorNamespace: "links"})
		if !r.Receipt.Accepted {
			t.Fatalf("%+v", r.Receipt)
		}
	}
	r := NormalizeMarkdownContent("## Title\n[Go](#heading-title)", parser.ContentOptions{AnchorNamespace: "links"})
	if r.Receipt.Accepted {
		t.Fatal("accepted broken auto-anchor link")
	}
}

func TestContentFragmentRejectsHiddenDocumentMarkers(t *testing.T) {
	for _, source := range []string{"Before\nSLIDE content", "Before\n<!-- node-id: X -->", "Before\nSECTION \"X\"", "    ## Literal or indented code"} {
		r := NormalizeMarkdownContent(source, parser.ContentOptions{AnchorNamespace: "boundaries"})
		if r.Receipt.Accepted {
			t.Fatalf("accepted %q", source)
		}
	}
}

func TestContentFragmentNavigationAndLiteralFields(t *testing.T) {
	literal := "<<chart: bar>>\n  data: [[\"N\", 2], [\"S\", 3]]\n  series: [\"Registro\"]\n  unit: \"[lectura](#sin-enlace)\"\n<<end>>"
	r := NormalizeMarkdownContent(literal, parser.ContentOptions{AnchorNamespace: "literalchart"})
	if !r.Receipt.Accepted {
		t.Fatalf("%+v", r.Receipt)
	}
	r = NormalizeMarkdownContent("## Seccion\n[ir](#heading-seccion \"título\")", parser.ContentOptions{AnchorNamespace: "refs"})
	if r.Receipt.Accepted {
		t.Fatal("title-bearing local navigation lost its target without a diagnostic")
	}
}
func TestContentFragmentReceiptCoordinates(t *testing.T) {
	r := NormalizeMarkdownContent("## Heading\nText\n\n```bash\ncommand\n```", parser.ContentOptions{AnchorNamespace: "coordinates"})
	if !r.Receipt.Accepted {
		t.Fatalf("%+v", r.Receipt)
	}
	lines := strings.Split(r.Source, "\n")
	for _, n := range r.Receipt.Nodes {
		line := n.EffectivePosition.Line
		if line < 1 || line > len(lines) {
			t.Fatalf("position outside hash-bound artifact: %+v", n)
		}
		expected := map[string]string{"heading": "SECTION", "text": "TEXT", "code": "CODE"}[n.Type]
		if expected != "" && !strings.Contains(lines[line-1], expected) {
			t.Fatalf("coordinate does not resolve node: %+v %q", n, lines[line-1])
		}
	}
}

func TestContentFragmentPublicationFreshAndRejected(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "fragment.strict")
	receipt := filepath.Join(dir, "receipt.json")
	r := NormalizeMarkdownContent("## Original", parser.ContentOptions{AnchorNamespace: "publication"})
	if !r.Receipt.Accepted {
		t.Fatalf("%+v", r.Receipt)
	}
	if err := PublishContentFragment(out, receipt, r); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(out)
	if err != nil || string(source) != r.Source {
		t.Fatalf("source %q %v", source, err)
	}
	if err := PublishContentFragment(out, receipt, r); err == nil {
		t.Fatal("overwrote existing destinations")
	}
	rejected := NormalizeMarkdownContent("```bash\nUnclosed", parser.ContentOptions{AnchorNamespace: "publication"})
	noSource := filepath.Join(dir, "rejected.strict")
	side := filepath.Join(dir, "rejected.json")
	if err := PublishContentFragment(noSource, side, rejected); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(noSource); !os.IsNotExist(err) {
		t.Fatal("rejected source published")
	}
	if _, err := os.Lstat(side); err != nil {
		t.Fatal(err)
	}
}
