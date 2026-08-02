// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestPopulateLangRuns_Markdown(t *testing.T) {
	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), "dijo [bonjour tout le monde]{lang=fr} y se fue")
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 {
		t.Fatalf("expected 1 LangRun, got %d: %+v", len(got), got)
	}
	if got[0].Text != "bonjour tout le monde" || got[0].Lang != "fr" {
		t.Errorf("unexpected run: %+v", got[0])
	}
}

// TestPopulateLangRuns_SkipsCodeSpans covers issue #63 code review finding
// #7's extraction-side follow-up: since ProcessInlineMarkdownFormatsSecure
// now protects `código` before any other pass runs (finding #7), a
// [x]{lang=fr} inside a code span no longer becomes a <span lang="fr"> in
// the HTML — it stays literal inside <code>. LangRuns must agree: reporting
// a run the HTML never produced would let a consumer (e.g. a DOCX/PPTX
// generator, or a linter rule) act on a language mark that isn't actually
// visible/applied anywhere in the rendered output.
func TestPopulateLangRuns_SkipsCodeSpans(t *testing.T) {
	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), "`[a]{lang=fr}` pero [b]{lang=de} sí")
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 || got[0].Text != "b" || got[0].Lang != "de" {
		t.Errorf("expected only the span outside the code, got %+v", got)
	}
}

// TestPopulateLangRuns_PreservesCodeSpanInsideLangText covers an advisor
// follow-up on TestPopulateLangRuns_SkipsCodeSpans: a lang span whose text
// CONTAINS a code span (as opposed to being entirely inside one) must keep
// that code span verbatim in Text — extractLangRunsFromMarkdown must not
// blank out code ranges before matching (that would also erase code
// content nested INSIDE a lang span, not just spans that live entirely
// inside code), only exclude lang-span matches that overlap a code range.
func TestPopulateLangRuns_PreservesCodeSpanInsideLangText(t *testing.T) {
	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), "[texto con `code`]{lang=fr}")
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 || got[0].Text != "texto con `code`" || got[0].Lang != "fr" {
		t.Errorf("expected the code span preserved verbatim in Text, got %+v", got)
	}
}

// TestPopulateLangRuns_MarkdownAgreesWithHTML states the actual contract
// directly instead of approximating it with per-case assertions (advisor
// follow-up): whatever the real HTML pipeline decides — does this input
// produce a <span lang="..."> in the HTML or not — LangRuns must agree. A
// prior fix (blanking code spans before matching) violated this for text
// nested inside a lang span; a second attempt (containment-only exclusion)
// violated it for a lang-span match that crosses a code span's boundary
// ("[a`b]{lang=fr}`c" — sanitizer.go's code pass swallows "]{lang=fr}"
// whole, so the HTML never gets a <span lang> there either); a third
// attempt (crossesCode scanning unsplit content) violated it across a "\n"
// (code-review finding on this same PR — see
// TestPopulateLangRuns_CodeRangesDoNotCrossLines). This table exercises the
// HTML side via ProcessInlineMarkdownSecureMultiline — the actual entry
// point every real caller uses, per-line split included — not
// ProcessInlineMarkdownFormatsSecure directly, which is exactly the gap
// that let the multi-line divergence through undetected the first time.
func TestPopulateLangRuns_MarkdownAgreesWithHTML(t *testing.T) {
	inputs := []string{
		"[b]{lang=de}",
		"`[a]{lang=fr}`",
		"[texto con `code`]{lang=fr}",
		"[a`b]{lang=fr}`c",
		"[a `x` b `y` c]{lang=fr}",
		"line one with `code\n[span text]{lang=fr}` more",
		"`[a]{lang=fr}`\n[b]{lang=de}",
	}
	for _, in := range inputs {
		html := ProcessInlineMarkdownSecureMultiline(in)

		elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), in)
		doc := newTestDoc(elem)
		PopulateLangRuns(doc, nil)
		runs := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns

		hasSpan := strings.Contains(html, `<span lang=`)
		hasRuns := len(runs) > 0
		if hasSpan != hasRuns {
			t.Errorf("%q: HTML/LangRuns divergence — html=%q hasSpan=%v runs=%+v", in, html, hasSpan, runs)
		}
	}
}

// TestPopulateLangRuns_RawHTML_CodeSpanAgreesWithHTML covers the RawHTML
// producer side of the same invariant as
// TestPopulateLangRuns_MarkdownAgreesWithHTML (advisor follow-up): a
// subsection heading's Content is already fully materialized HTML by parse
// time (core/parser/document_flex.go calls ProcessInlineMarkdownSecureLine
// before PopulateLangRuns ever runs), so "`[a]{lang=fr}`" there is already
// "<code>[a]{lang=fr}</code>" — no <span lang> for extractLangRunsFromHTML
// to find. This confirms the contract holds by construction on this path
// too, not just on extractLangRunsFromMarkdown.
func TestPopulateLangRuns_RawHTML_CodeSpanAgreesWithHTML(t *testing.T) {
	content := ProcessInlineMarkdownSecureLine("`[a]{lang=fr}`")
	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), content)
	elem.IsRawHTML = true
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 0 {
		t.Errorf("expected zero runs (code span, no <span lang> materialized), got %+v", got)
	}
}

// TestPopulateLangRuns_CodeRangesDoNotCrossLines covers a real code-review
// finding on this same PR: extractLangRunsFromMarkdown's crossesCode used
// to scan inlineCodePattern over the whole, unsplit Content — but every
// real HTML-rendering caller (ProcessInlineMarkdownSecure/Multiline) splits
// Content into lines FIRST, so a backtick can never pair across a "\n" in
// the actual rendered HTML. An unclosed backtick on one line, coincidentally
// closed by an unrelated backtick on a LATER line, used to fabricate a
// cross-line code range that swallowed a genuine [x]{lang=fr} span on the
// second line, wrongly dropping it from LangRuns even though the real HTML
// contains a <span lang="fr"> for it.
func TestPopulateLangRuns_CodeRangesDoNotCrossLines(t *testing.T) {
	content := "line one with `code\n[span text]{lang=fr}` more"

	html := ProcessInlineMarkdownSecureMultiline(content)
	if !strings.Contains(html, `<span lang="fr">span text</span>`) {
		t.Fatalf("precondition failed: expected the real HTML pipeline to emit a lang span, got %q", html)
	}

	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), content)
	doc := newTestDoc(elem)
	PopulateLangRuns(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 || got[0].Text != "span text" || got[0].Lang != "fr" {
		t.Errorf("expected the lang span on line 2 to survive, got %+v", got)
	}
}

// TestPopulateLangRuns_RawHTML cubre el branch IsRawHTML de TextElement — el
// caso de un heading (## texto), donde [x]{lang=fr} ya se materializó a
// <span lang="fr">x</span> en tiempo de parseo, antes de que este paso
// corra.
func TestPopulateLangRuns_RawHTML(t *testing.T) {
	elem := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1),
		`<h2 id="x">Bonjour <span lang="fr">tout le monde</span></h2>`)
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 {
		t.Fatalf("expected 1 LangRun, got %d: %+v", len(got), got)
	}
	if got[0].Text != "tout le monde" || got[0].Lang != "fr" {
		t.Errorf("unexpected run: %+v", got[0])
	}
}

// TestPopulateLangRuns_RawHTML_NestedFormatting confirms a <strong>/<em>
// nested inside the materialized <span lang> is stripped down to plain text
// in Text (issue #63 code review, finding #6) — the RawHTML path can't
// recover the original markdown, only the HTML that was already emitted,
// but LangRun.Text must still be plain text either way (see LangRun's doc
// comment): a consumer reading Text has no reason to expect it might contain
// raw HTML tags on this path but not the Markdown path.
func TestPopulateLangRuns_RawHTML_NestedFormatting(t *testing.T) {
	elem := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1),
		`<h2 id="x"><span lang="fr">a <strong>b</strong> c</span></h2>`)
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 || got[0].Text != "a b c" || got[0].Lang != "fr" {
		t.Errorf("unexpected runs: %+v", got)
	}
}

// TestPopulateLangRuns_RawHTML_EntitiesDecoded confirms the RawHTML path
// decodes the HTML entities EscapeHTML introduced at parse time (issue #63
// code review, finding #6, advisor follow-up) — without this, "a & b" would
// come back as the literal string "a &amp; b", diverging from what the
// equivalent Markdown-path span ([a & b]{lang=fr}, whose Content is never
// escaped) would produce for the same author-intended text.
func TestPopulateLangRuns_RawHTML_EntitiesDecoded(t *testing.T) {
	elem := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1),
		`<h2 id="x"><span lang="fr">a &amp; b &lt;c&gt;</span></h2>`)
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 || got[0].Text != "a & b <c>" || got[0].Lang != "fr" {
		t.Errorf("unexpected runs: %+v", got)
	}
}

func TestLangSpanHTMLToSource(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple span", `<span lang="fr">tout le monde</span>`, `[tout le monde]{lang=fr}`},
		{"invalid tag left as HTML", `<span lang="es_MX">x</span>`, `<span lang="es_MX">x</span>`},
		{
			"non-round-trippable text (contains '[') left as HTML",
			`<span lang="fr">a [b] c</span>`,
			`<span lang="fr">a [b] c</span>`,
		},
		{"no span, passthrough", `plain text`, `plain text`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LangSpanHTMLToSource(tt.in); got != tt.want {
				t.Errorf("LangSpanHTMLToSource(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPopulateLangRuns_InvalidTagFiltered(t *testing.T) {
	tests := []struct {
		name    string
		content string
		rawHTML bool
	}{
		{"markdown, malformed tag (underscore)", "[x]{lang=es_MX}", false},
		{"markdown, dangling extension singleton", "[x]{lang=en-a}", false},
		{"rawHTML, malformed tag never reaches the regex (not well-formed span)", `<span lang="es_MX">x</span>`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var elem *ast.TextElement
			if tt.rawHTML {
				elem = ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1), tt.content)
			} else {
				elem = ast.NewTextElement(diagnostics.NewPosition(1, 1), tt.content)
			}
			doc := newTestDoc(elem)
			PopulateLangRuns(doc, nil)
			got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
			if len(got) != 0 {
				t.Errorf("expected no LangRuns for a malformed tag, got %+v", got)
			}
		})
	}
}

func TestPopulateLangRuns_QuoteElement(t *testing.T) {
	elem := ast.NewQuoteElement(diagnostics.NewPosition(1, 1), "she said [c'est la vie]{lang=fr}")
	elem.Author = "[Jane]{lang=fr}" // Author must NOT get LangRuns — see QuoteElement.LangRuns doc comment
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.QuoteElement)
	if len(got.LangRuns) != 1 || got.LangRuns[0].Text != "c'est la vie" || got.LangRuns[0].Lang != "fr" {
		t.Errorf("unexpected Content LangRuns: %+v", got.LangRuns)
	}
}

func TestPopulateLangRuns_PointsAndChecklistRecursion(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	points := ast.NewPointsElement(pos)
	sub := ast.NewPointItem(pos, "sub [bonjour]{lang=fr}")
	item := ast.NewPointItem(pos, "main [hallo]{lang=de}")
	item.SubPoints = append(item.SubPoints, *sub)
	points.Items = append(points.Items, *item)

	checklist := ast.NewChecklistElement(pos)
	subCheck := ast.NewChecklistItem(pos, "subcheck [ciao]{lang=it}", false)
	check := ast.NewChecklistItem(pos, "check [salut]{lang=fr}", true)
	check.SubItems = append(check.SubItems, *subCheck)
	checklist.Items = append(checklist.Items, *check)

	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, points, checklist)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	PopulateLangRuns(doc, nil)

	gotPoints := doc.ContentBlocks[0].Elements[0].(*ast.PointsElement)
	if len(gotPoints.Items[0].LangRuns) != 1 || gotPoints.Items[0].LangRuns[0].Lang != "de" {
		t.Errorf("PointItem LangRuns not populated correctly: %+v", gotPoints.Items[0].LangRuns)
	}
	if len(gotPoints.Items[0].SubPoints[0].LangRuns) != 1 || gotPoints.Items[0].SubPoints[0].LangRuns[0].Lang != "fr" {
		t.Errorf("PointItem.SubPoints[0] LangRuns not populated (recursion): %+v", gotPoints.Items[0].SubPoints[0].LangRuns)
	}

	gotChecklist := doc.ContentBlocks[0].Elements[1].(*ast.ChecklistElement)
	if len(gotChecklist.Items[0].LangRuns) != 1 || gotChecklist.Items[0].LangRuns[0].Lang != "fr" {
		t.Errorf("ChecklistItem LangRuns not populated correctly: %+v", gotChecklist.Items[0].LangRuns)
	}
	if len(gotChecklist.Items[0].SubItems[0].LangRuns) != 1 || gotChecklist.Items[0].SubItems[0].LangRuns[0].Lang != "it" {
		t.Errorf("ChecklistItem.SubItems[0] LangRuns not populated (recursion): %+v", gotChecklist.Items[0].SubItems[0].LangRuns)
	}
}

func TestPopulateLangRuns_GridColumnRecursion(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	nested := ast.NewTextElement(pos, "[bonjour]{lang=fr}")
	col := ast.ColumnElement{Elements: []ast.Element{nested}}
	grid := ast.NewGridElement(pos)
	grid.Columns = append(grid.Columns, col)

	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, grid)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	PopulateLangRuns(doc, nil)

	gotGrid := doc.ContentBlocks[0].Elements[0].(*ast.GridElement)
	gotText := gotGrid.Columns[0].Elements[0].(*ast.TextElement)
	if len(gotText.LangRuns) != 1 || gotText.LangRuns[0].Lang != "fr" {
		t.Errorf("nested TextElement inside grid column not populated: %+v", gotText.LangRuns)
	}
}

// TestPopulateLangRuns_SpecialBlockGridColumnContent covers issue #63 code
// review finding #9: SpecialBlockElement, GridElement and ColumnElement each
// carry their own loose Content string (a callout body, prose inside a grid
// but outside any column, prose inside a column) that populateElementHTML
// already treats as a first-class prose field (see populate_inline_html.go's
// TitleHTML/ContentHTML cases for the same three types) but that
// populateElementLangRuns used to skip entirely.
func TestPopulateLangRuns_SpecialBlockGridColumnContent(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	special := ast.NewSpecialBlockElement(pos, "info", "ceci [est important]{lang=fr}")

	grid := ast.NewGridElement(pos)
	grid.Content = "grid [loose prose]{lang=fr}"
	col := ast.ColumnElement{Content: "col [prose]{lang=fr}"}
	grid.Columns = append(grid.Columns, col)

	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, special, grid)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	PopulateLangRuns(doc, nil)

	gotSpecial := doc.ContentBlocks[0].Elements[0].(*ast.SpecialBlockElement)
	if len(gotSpecial.LangRuns) != 1 || gotSpecial.LangRuns[0].Text != "est important" || gotSpecial.LangRuns[0].Lang != "fr" {
		t.Errorf("SpecialBlockElement.LangRuns not populated from Content: %+v", gotSpecial.LangRuns)
	}

	gotGrid := doc.ContentBlocks[0].Elements[1].(*ast.GridElement)
	if len(gotGrid.LangRuns) != 1 || gotGrid.LangRuns[0].Text != "loose prose" || gotGrid.LangRuns[0].Lang != "fr" {
		t.Errorf("GridElement.LangRuns not populated from Content: %+v", gotGrid.LangRuns)
	}
	if len(gotGrid.Columns[0].LangRuns) != 1 || gotGrid.Columns[0].LangRuns[0].Text != "prose" || gotGrid.Columns[0].LangRuns[0].Lang != "fr" {
		t.Errorf("ColumnElement.LangRuns not populated from Content: %+v", gotGrid.Columns[0].LangRuns)
	}
}

// TestPopulateLangRuns_Variables covers issue #63 code review finding #10:
// a lang span reaching an element only through a {{variable}} substitution
// (not present literally in Content) must still be found — same order as
// ProcessTextWithVariablesAndMarkdownSecure (variables first, then the
// markdown/span pass).
func TestPopulateLangRuns_Variables(t *testing.T) {
	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), "dijo {{saludo}} y se fue")
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, map[string]interface{}{"saludo": "[bonjour]{lang=fr}"})

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 || got[0].Text != "bonjour" || got[0].Lang != "fr" {
		t.Errorf("expected a LangRun surfaced via variable substitution, got %+v", got)
	}
}

// TestPopulateLangRuns_AlwaysOverwrites is the security-relevant test: an
// element arriving with LangRuns already set (e.g. forged by a hostile
// --filter) must have that value REPLACED, never trusted or left alone —
// for BOTH the markdown and the RawHTML extraction path. Skipping either
// path would let a filter set IsRawHTML (or not) specifically to dodge
// re-derivation.
func TestPopulateLangRuns_AlwaysOverwrites(t *testing.T) {
	t.Run("markdown path", func(t *testing.T) {
		elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), "no lang span here")
		elem.LangRuns = []ast.LangRun{{Text: "forged", Lang: "xx-forged-by-filter"}}
		doc := newTestDoc(elem)

		PopulateLangRuns(doc, nil)

		got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
		if len(got) != 0 {
			t.Errorf("expected forged LangRuns to be replaced with the true (empty) derivation, got %+v", got)
		}
	})

	t.Run("rawHTML path", func(t *testing.T) {
		elem := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1), `<h2 id="x">no lang span here</h2>`)
		elem.LangRuns = []ast.LangRun{{Text: "forged", Lang: "xx-forged-by-filter"}}
		doc := newTestDoc(elem)

		PopulateLangRuns(doc, nil)

		got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
		if len(got) != 0 {
			t.Errorf("expected forged LangRuns to be replaced with the true (empty) derivation, got %+v", got)
		}
	})
}

func TestPopulateLangRuns_Idempotent(t *testing.T) {
	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), "[bonjour]{lang=fr} et [hallo]{lang=de}")
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, nil)
	first := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns

	PopulateLangRuns(doc, nil)
	second := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns

	if len(first) != len(second) {
		t.Fatalf("run count changed across calls: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("run %d changed across calls: %+v vs %+v", i, first[i], second[i])
		}
	}
}

func TestPopulateLangRuns_NilSafety(t *testing.T) {
	PopulateLangRuns(nil, nil) // must not panic
}

func TestPopulateLangRuns_NoSpansYieldsNilNotEmptySlice(t *testing.T) {
	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), "plain text, no spans at all")
	doc := newTestDoc(elem)

	PopulateLangRuns(doc, nil)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if got != nil {
		t.Errorf("expected nil LangRuns (omitempty-friendly), got %+v", got)
	}
}
