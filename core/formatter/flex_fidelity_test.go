// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"errors"
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

// The window that makes two images a gallery is a count of lines, so the
// `context:` line written for one image can change what the other infers. The
// pass has to read its own output back until nothing differs.
func TestFlexToStrict_ImageContextsSettle(t *testing.T) {
	cases := map[string]string{
		// Two images a few lines apart where only the first is a cover.
		"cover and a nearby image": "---\nmode: flex\n---\n\n# Deck\n\n![Cover](cover.png)\n\ntext\n\n![Other](other.png)\n\n---\n\n## Two\n\nbody\n",
		// Several images in a row, some of them galleries in flex.
		"a row of images": "---\nmode: flex\n---\n\n# Deck\n\n---\n\n## Two\n\n![A](a.png)\n![B](b.png)\n![C](c.png)\n\ntext\n\n![D](d.png)\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			want := imageContexts(parseSlides(t, src))
			out, reparsed := transpile(t, src)
			got := imageContexts(reparsed)
			if len(want) != len(got) {
				t.Fatalf("images: %v before, %v after\n%s", want, got, out)
			}
			for i := range want {
				if want[i] != got[i] {
					t.Errorf("image %d: context %q before, %q after\n%s", i, want[i], got[i], out)
				}
			}
		})
	}
}

// Node identities are written as comments after the contexts are placed. The
// parser removes those lines before it infers anything, so they must not move a
// context.
func TestFormatStrict_NodeIDsDoNotMoveImageContexts(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  IMAGE \"a.png\" \"a\"\n  TEXT\n    x\n  TEXT\n    y\n  IMAGE \"b.png\" \"b\"\n"
	doc := parseSlides(t, src)
	imgs := collectImages(doc)
	if len(imgs) != 2 || imgs[0].Context != ast.ImageContextGallery {
		t.Fatalf("fixture should read as a gallery, got %v", imageContexts(doc))
	}
	imgs[1].NodeID = "Second"
	doc.ContentBlocks[0].NodeID = "Slide"
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if !strings.Contains(out, "<!-- node-id: Second -->") {
		t.Fatalf("the node identity was not written:\n%s", out)
	}
	got := imageContexts(parseSlides(t, out))
	if len(got) != 2 || got[0] != imgs[0].Context || got[1] != imgs[1].Context {
		t.Errorf("contexts %v after, %v before\n%s", got, imageContexts(doc), out)
	}
}

// Images at every distance from each other, with and without a cover, so the
// gallery window and the other heuristics are crossed at their edges.
func TestFlexToStrict_ImageContextsAtEveryDistance(t *testing.T) {
	for _, cover := range []bool{false, true} {
		for gap := 0; gap <= 8; gap++ {
			var b strings.Builder
			b.WriteString("---\nmode: flex\n---\n\n# Deck\n\n")
			if cover {
				b.WriteString("![Cover](cover.png)\n\n")
			}
			b.WriteString("---\n\n## Two\n\n![A](a.png)\n")
			for i := 0; i < gap; i++ {
				b.WriteString("\npara " + string(rune('a'+i)) + "\n")
			}
			b.WriteString("\n![B](b.png)\n\ntail\n")
			src := b.String()
			t.Run(strings.ReplaceAll(strings.TrimSpace(strings.Join([]string{map[bool]string{true: "cover", false: "plain"}[cover], string(rune('0' + gap))}, "-")), " ", "_"), func(t *testing.T) {
				want := imageContexts(parseSlides(t, src))
				out, reparsed := transpile(t, src)
				got := imageContexts(reparsed)
				if len(want) != len(got) {
					t.Fatalf("images: %v before, %v after\n%s", want, got, out)
				}
				for i := range want {
					if want[i] != got[i] {
						t.Errorf("image %d: context %q before, %q after\n%s", i, want[i], got[i], out)
					}
				}
			})
		}
	}
}

// An image inside a ::: block is raw text for strict, so it is not an image when
// the formatted text is read back and its context cannot be kept. That has to be
// an error naming the element, not a silent loss of the cover's context.
func TestFlexToStrict_ImageInsideABlockIsReported(t *testing.T) {
	cases := map[string]string{
		"cover and a card image": "---\nmode: flex\n---\n\n# Deck\n\n## Sub\n\n![Logo](logo.png)\n\n---\n\n## Two\n\n:::card\n![In](in.png)\n:::\n\n![Z](z.png)\n",
		"only a columns image":   "---\nmode: flex\n---\n\n## One\n\n:::columns\n![In](in.png)\n:::\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := FormatStrict(parseSlides(t, src))
			var uerr *UnsupportedElementError
			if !errors.As(err, &uerr) {
				t.Fatalf("want an UnsupportedElementError, got %v", err)
			}
		})
	}
}

func elementShape(doc *ast.AST) []string {
	var out []string
	for _, b := range doc.ContentBlocks {
		out = append(out, "slide")
		for _, el := range b.Elements {
			s := string(el.GetType())
			if d, ok := el.(*ast.DirectiveNode); ok && d.Name == "notes" {
				s += "=" + d.Parameters["content"].(string)
			}
			out = append(out, s)
		}
	}
	return out
}

// The multi-line body of @notes runs until a blank line, so an element written
// right behind it was read as more notes: the slide lost elements and the notes
// grew. The formatter now closes the body with a blank line and writes empty
// notes as @notes "".
func TestFlexToStrict_NotesDoNotSwallowTheNextElements(t *testing.T) {
	cases := map[string]string{
		"single line then heading and quote": "---\nmode: flex\n---\n\n## One\n\ntext\n\n@notes \"Say this first.\"\n\n### Sub\n\n> a quote\n\n---\n\n## Two\n\nbody\n",
		"multi-line block then text":         "---\nmode: flex\n---\n\n## One\n\n@notes:\nfirst line\nsecond line\n\nafter the notes\n\n---\n\n## Two\n\nbody\n",
		"notes last in the slide":            "---\nmode: flex\n---\n\n## One\n\ntext\n\n@notes \"Last.\"\n\n---\n\n## Two\n\nbody\n",
		"empty notes then text":              "---\nmode: flex\n---\n\n## One\n\n@notes \"\"\n\ntext after\n\n---\n\n## Two\n\nbody\n",
		"notes that start with a hash":       "---\nmode: flex\n---\n\n## One\n\n@notes \"# not a heading\"\n\ntext after\n\n---\n\n## Two\n\nbody\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			want := elementShape(parseSlides(t, src))
			out, reparsed := transpile(t, src)
			got := elementShape(reparsed)
			if strings.Join(want, "|") != strings.Join(got, "|") {
				t.Errorf("shape changed\n before: %q\n after:  %q\n%s", want, got, out)
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

func TestFormatNotes(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		want      string
		multiline bool
		wantErr   bool
	}{
		{"empty", "", `@notes ""`, false, false},
		{"one line", "Say this.", "@notes\n  Say this.", true, false},
		{"two lines", "a\nb", "@notes\n  a\n  b", true, false},
		{"hash start, one line", "# Heading", `@notes "# Heading"`, false, false},
		{"hash start, two lines", "a\n# b", "", false, true},
		{"empty line inside", "a\n\nb", "", false, true},
		{"padded line", " a", `@notes " a"`, false, false},
	}
	for _, tt := range tests {
		got, multiline, err := formatNotes(tt.content)
		if (err != nil) != tt.wantErr || got != tt.want || multiline != tt.multiline {
			t.Errorf("%s: formatNotes(%q) = %q, %v, %v; want %q, %v, wantErr=%v", tt.name, tt.content, got, multiline, err, tt.want, tt.multiline, tt.wantErr)
		}
	}
}

// A flex ::: block with a heading, fenced code, a table or an image inside
// carries those as Elements next to the raw Content. Strict reads the body as
// raw lines, so the text would come back without them: fmt has to say so.
func TestFlexToStrict_NestedBlockElementsAreRefused(t *testing.T) {
	cases := map[string]string{
		"heading": "---\nmode: flex\n---\n\n## One\n\n:::card\n### Title\nbody\n:::\n",
		"code":    "---\nmode: flex\n---\n\n## One\n\n:::tabs\n```go\nx := 1\n```\n:::\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := FormatStrict(parseSlides(t, src))
			var uerr *UnsupportedElementError
			if !errors.As(err, &uerr) || uerr.NodeType != "special_block" {
				t.Fatalf("want an UnsupportedElementError for special_block, got %v", err)
			}
			if !strings.Contains(uerr.Reason, ":::") {
				t.Errorf("the reason should name the block: %s", uerr.Reason)
			}
		})
	}
}

// A block with only prose is not affected: Content is all there is to keep.
func TestFlexToStrict_PlainSpecialBlockStillTranspiles(t *testing.T) {
	src := "---\nmode: flex\n---\n\n## One\n\n:::info Heads up\nJust prose here.\n:::\n"
	out, _ := transpile(t, src)
	if !strings.Contains(out, ":::info Heads up") {
		t.Errorf("the block was not written:\n%s", out)
	}
}

// With nested blocks the innermost one that loses elements is the one named,
// and a block that holds other blocks does not make the pairing silently pass.
func TestFlexToStrict_NestedBlockErrorNamesTheInnermostBlock(t *testing.T) {
	src := "---\nmode: flex\n---\n\n## One\n\n:::tabs Outer\n::: card Inner\n### Heading\nbody\n:::\n:::\n"
	_, err := FormatStrict(parseSlides(t, src))
	var uerr *UnsupportedElementError
	if !errors.As(err, &uerr) || uerr.NodeType != "special_block" {
		t.Fatalf("want an UnsupportedElementError for special_block, got %v", err)
	}
	if !strings.Contains(uerr.Reason, "card Inner") {
		t.Errorf("the reason should name the innermost block, got: %s", uerr.Reason)
	}
}

func TestFlexToStrict_NestedBlocksInDocumentsAreRefused(t *testing.T) {
	src := "---\nmode: flex\n---\n\n# Doc\n\n:::card T\n### H\nbody\n:::\n"
	p := parser.New(util.NewNoop())
	doc, diags := p.ParseDocument(src, "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("does not parse: %s", d.String())
		}
	}
	_, err := FormatDocumentStrict(doc)
	var uerr *UnsupportedElementError
	if !errors.As(err, &uerr) || uerr.NodeType != "special_block" {
		t.Fatalf("want an UnsupportedElementError for special_block, got %v", err)
	}
}

func TestCheckNestedBlockElements_FailsClosedWhenTheTextDoesNotReadBack(t *testing.T) {
	doc := parseSlides(t, "---\nmode: flex\n---\n\n## One\n\n:::card\n### H\nbody\n:::\n")
	err := checkNestedBlockElements(doc, nil)
	var uerr *UnsupportedElementError
	if !errors.As(err, &uerr) {
		t.Fatalf("want an UnsupportedElementError, got %v", err)
	}
	// Different number of blocks: an empty slide read back.
	empty := parseSlides(t, "---\nmode: strict\n---\n\nSLIDE content\n  title: \"x\"\n")
	if err := checkNestedBlockElements(doc, empty); err == nil {
		t.Error("a block that did not come back must be reported")
	}
}

func TestFlexToStrict_NestedBlockErrorNamesTheColumn(t *testing.T) {
	src := "---\nmode: flex\n---\n\n## One\n\n:::: grid\n::: column\n### H\ntext\n:::\n::: column\nother\n:::\n::::\n"
	_, err := FormatStrict(parseSlides(t, src))
	var uerr *UnsupportedElementError
	if !errors.As(err, &uerr) {
		t.Fatalf("want an UnsupportedElementError, got %v", err)
	}
	if !strings.Contains(uerr.Reason, "column") {
		t.Errorf("the reason should name the column that loses its heading, got: %s", uerr.Reason)
	}
}
