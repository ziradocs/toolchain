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

func parseSlides(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).Parse(src, "t.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("does not parse: %s\n%s", d.String(), src)
		}
	}
	return doc
}

// transpile runs the same two steps as `slidelang fmt` and returns the strict
// text together with the AST it re-parses to.
func transpile(t *testing.T, src string) (string, *ast.AST) {
	t.Helper()
	out, err := FormatStrict(parseSlides(t, src))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	return out, parseSlides(t, out)
}

func codeContents(doc *ast.AST) []string {
	var out []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		if c, ok := n.(*ast.CodeElement); ok {
			out = append(out, c.Content)
		}
		return nil
	})
	return out
}

// A fenced block that ends a slide, ends the deck, or sits between other
// elements must come back with exactly the content it had. The strict parser
// reads blank lines after CODE as part of its body, so the blank line that
// used to separate one SLIDE from the next added a "\n" to the code.
func TestFlexToStrict_CodeContentKeepsItsTrailingNewlines(t *testing.T) {
	cases := map[string]string{
		"last element of a slide":  "---\nmode: flex\n---\n\n## One\n\n```go\nx := 1\n```\n\n---\n\n## Two\n\nbody\n",
		"last element of the deck": "---\nmode: flex\n---\n\n## One\n\ntext\n\n---\n\n## Two\n\n```go\nx := 1\n```\n",
		"between elements":         "---\nmode: flex\n---\n\n## One\n\n```go\nx := 1\n```\n\nafter\n\n---\n\n## Two\n\nbody\n",
		"blank line inside":        "---\nmode: flex\n---\n\n## One\n\n```go\nx := 1\n\ny := 2\n```\n\n---\n\n## Two\n\nbody\n",
		"trailing blank line":      "---\nmode: flex\n---\n\n## One\n\n```go\nx := 1\n\n```\n\n---\n\n## Two\n\nbody\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			want := codeContents(parseSlides(t, src))
			out, reparsed := transpile(t, src)
			got := codeContents(reparsed)
			if len(want) != len(got) {
				t.Fatalf("code blocks: %d before, %d after\n%s", len(want), len(got), out)
			}
			for i := range want {
				if want[i] != got[i] {
					t.Errorf("code block %d: %q before, %q after\n%s", i, want[i], got[i], out)
				}
			}
			again, err := FormatStrict(reparsed)
			if err != nil {
				t.Fatalf("second FormatStrict: %v", err)
			}
			if again != out {
				t.Errorf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, again)
			}
		})
	}
}

// A last slide with no title and no declared layout is "closing" once the
// linter has run. The formatter has to say so: a literal `SLIDE content` would
// declare the layout and the rule would no longer reclassify it.
func TestFormatStrict_WritesTheInferredClosingLayout(t *testing.T) {
	src := "---\nmode: flex\n---\n\n# Deck\n\n---\n\n## Middle\n\n- a\n\n---\n\nThanks\n"
	out, _ := transpile(t, src)
	if !strings.Contains(out, "SLIDE closing\n") {
		t.Fatalf("the last slide should be written as closing:\n%s", out)
	}
	if strings.Count(out, "SLIDE closing") != 1 {
		t.Errorf("only the last slide is inferred as closing:\n%s", out)
	}
}

func TestFormatStrict_KeepsADeclaredContentLayoutOnTheLastSlide(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE title\n  heading: \"Deck\"\n\nSLIDE content\n  TEXT\n    Thanks\n"
	out, err := FormatStrict(parseSlides(t, src))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if strings.Contains(out, "SLIDE closing") {
		t.Errorf("a declared layout must not be rewritten:\n%s", out)
	}
}

func TestContentBlock_InfersClosingLayout(t *testing.T) {
	tests := []struct {
		name  string
		block ast.ContentBlock
		want  bool
	}{
		{"untitled content", ast.ContentBlock{BlockType: "content"}, true},
		{"no type", ast.ContentBlock{}, true},
		{"default type", ast.ContentBlock{BlockType: "default"}, true},
		{"declared layout", ast.ContentBlock{BlockType: "content", LayoutDeclared: true}, false},
		{"has a title", ast.ContentBlock{BlockType: "content", Title: "T"}, false},
		{"has a heading", ast.ContentBlock{BlockType: "content", Heading: "H"}, false},
		{"other layout", ast.ContentBlock{BlockType: "section"}, false},
	}
	for _, tt := range tests {
		if got := tt.block.InfersClosingLayout(); got != tt.want {
			t.Errorf("%s: InfersClosingLayout() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// A subsection heading keeps the emphasis the author wrote.
func TestFlexToStrict_HeadingKeepsInlineEmphasis(t *testing.T) {
	src := "---\nmode: flex\n---\n\n## One\n\n#### 1. **Bold** and *soft* and `code`\n\ntext\n"
	out, _ := transpile(t, src)
	if !strings.Contains(out, "SECTION \"1. **Bold** and *soft* and `code`\"") {
		t.Errorf("the heading lost its emphasis:\n%s", out)
	}
}

func TestAsDocumentHeading_WithoutSource(t *testing.T) {
	h := ast.NewRawHTMLTextElement(diagnostics.NewPosition(3, 1), `<h2 id="the-important-part">The <strong>important</strong> part</h2>`)
	h.Level = 2
	got, ok := asDocumentHeading(h)
	if !ok {
		t.Fatal("not recognised as a heading")
	}
	if got.text != "The important part" || got.level != 2 || got.id != "the-important-part" {
		t.Errorf("asDocumentHeading = %+v", got)
	}
}

// The source is only trusted while Content is still the HTML it produced.
func TestAsDocumentHeading_StaleSourceIsIgnored(t *testing.T) {
	h := ast.NewRawHTMLTextElement(diagnostics.NewPosition(3, 1), `<h2 id="x">Edited elsewhere</h2>`)
	h.Level = 2
	h.HeadingSource = "Original"
	h.HeadingAnchor = "x"
	h.HeadingContent = `<h2 id="x">Original</h2>`
	got, ok := asDocumentHeading(h)
	if !ok || got.text != "Edited elsewhere" {
		t.Errorf("asDocumentHeading = %+v, %v", got, ok)
	}
}

// A combo chart written with the nested YAML form carries one empty axis per
// series. Writing nothing back would re-parse as nil, a different AST.
func TestFormatChart_AllEmptySeriesAxesAreWritten(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(3, 1), "combo")
	chart.SeriesTypes = []string{"bar", "line"}
	chart.Series = []string{"A", "B"}
	chart.SeriesAxes = []string{"", ""}
	chart.Data = [][]interface{}{{"Q1", 1.0, 2.0}}
	out, err := FormatStrict(chartDoc(chart))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if !strings.Contains(out, `yAxisID: ["", ""]`) {
		t.Fatalf("yAxisID missing:\n%s", out)
	}
	reparsed := parseSlides(t, out)
	var got []string
	_ = ast.Walk(reparsed, func(n ast.Node) error {
		if c, ok := n.(*ast.ChartElement); ok {
			got = c.SeriesAxes
		}
		return nil
	})
	if len(got) != 2 || got[0] != "" || got[1] != "" {
		t.Errorf("SeriesAxes after round-trip = %#v", got)
	}
}

func imageContexts(doc *ast.AST) []ast.ImageContext {
	var out []ast.ImageContext
	for _, img := range collectImages(doc) {
		out = append(out, img.Context)
	}
	return out
}

// An image's context is inferred from the text around it, and flex and strict
// read that text differently. fmt has to carry the value the flex source had.
func TestFlexToStrict_ImageContextSurvives(t *testing.T) {
	cases := map[string]string{
		"cover":      "---\nmode: flex\n---\n\n# Deck\n\n## Sub\n\n![Logo](logo.png)\n\n---\n\n## Two\n\ntext\n",
		"standalone": "---\nmode: flex\n---\n\n# Deck\n\n---\n\n## Two\n\ntext\n\n- a\n- b\n- c\n\n![Photo](photo.png)\n",
		"gallery":    "---\nmode: flex\n---\n\n# Deck\n\n---\n\n## Two\n\n![A](a.png)\n![B](b.png)\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			want := imageContexts(parseSlides(t, src))
			out, reparsed := transpile(t, src)
			got := imageContexts(reparsed)
			if len(want) == 0 || len(want) != len(got) {
				t.Fatalf("images: %v before, %v after\n%s", want, got, out)
			}
			for i := range want {
				if want[i] != got[i] {
					t.Errorf("image %d: context %q before, %q after\n%s", i, want[i], got[i], out)
				}
			}
			again, err := FormatStrict(reparsed)
			if err != nil {
				t.Fatalf("second FormatStrict: %v", err)
			}
			if again != out {
				t.Errorf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, again)
			}
		})
	}
}

// A strict source whose images already infer the context they had gets no
// `context:` line: formatting it does not add noise.
func TestFormatStrict_ImageContextOnlyWhenNeeded(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"One\"\n  TEXT\n    a\n  TEXT\n    b\n  TEXT\n    c\n  IMAGE \"a.png\" \"alt\"\n\nSLIDE content\n  title: \"Two\"\n  TEXT\n    x\n"
	out, err := FormatStrict(parseSlides(t, src))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if strings.Contains(out, "context:") {
		t.Errorf("an inferable context must not be written:\n%s", out)
	}
	if out != src {
		t.Errorf("canonical strict text changed:\n--- want ---\n%s\n--- got ---\n%s", src, out)
	}
}
