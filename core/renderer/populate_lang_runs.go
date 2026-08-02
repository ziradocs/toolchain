// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"regexp"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/ast"
)

// langSpanHTMLPattern extracts <span lang="xx">...</span> already
// materialized in a TextElement's raw HTML Content (IsRawHTML=true) — the
// case where the source [texto]{lang=xx} was consumed at PARSE time
// (core/parser/document_flex.go's subsection-heading path, which runs the
// full inline pipeline including the lang-span pass before PopulateLangRuns
// ever sees the raw markdown). Non-greedy: does not correctly separate a
// lang span nested inside another span (e.g. a lang span wrapping a
// class-styled span, reachable via [[x]{.danger}]{lang=fr}) — a rare
// combination inside a heading specifically, left as a known v1 limitation
// rather than growing this into a general HTML parser.
var langSpanHTMLPattern = regexp.MustCompile(`<span lang="([a-zA-Z0-9-]+)">(.*?)</span>`)

// PopulateLangRuns fills LangRuns on every prose-carrying element reachable
// from doc, deriving it FRESH from each element's own Content every time —
// see LangRun's doc comment for why. Unlike PopulateInlineHTML, this is not
// purely additive: it OVERWRITES LangRuns (never appends, never skips an
// element that already looks populated), because skipping is exactly the
// bypass a hostile --filter would need: forge LangRuns directly, rely on
// this function leaving a "populated-looking" field alone. There is nothing
// here for ast.ClearRenderedHTML to clear (see that function's own doc
// comment on *HTML fields) because there is nothing here to trust from
// upstream in the first place — every call to this function replaces
// LangRuns with what Content, read right now, actually says.
func PopulateLangRuns(doc *ast.AST) {
	if doc == nil {
		return
	}
	for i := range doc.ContentBlocks {
		populateContentBlockLangRuns(&doc.ContentBlocks[i])
	}
}

func populateContentBlockLangRuns(block *ast.ContentBlock) {
	for _, elem := range block.Elements {
		populateElementLangRuns(elem)
	}
}

// populateElementLangRuns covers the four prose carriers scoped for #63's
// v1 (TextElement, PointItem, ChecklistItem, QuoteElement.Content — see the
// plan's reasoning for why table cells and grid prose are out of scope) plus
// GridElement/ColumnElement, which carry no LangRuns field of their own but
// must still be walked to reach any of the four types nested inside a
// grid's columns.
//
// Deliberately NOT guarded by a coverage test the way populateElementHTML
// is: LangRuns is a NEW FIELD on an EXISTING type, and every type-coverage
// guard in this repo (verified while researching #63) compares Element
// TYPES against switch cases — none inspects struct fields — so a missing
// case here would fail silently regardless of a guard. The four-type scope
// is intentional and documented, not something CI can currently enforce.
func populateElementLangRuns(element ast.Element) {
	switch elem := element.(type) {
	case *ast.TextElement:
		elem.LangRuns = extractLangRuns(elem.Content, elem.IsRawHTML)

	case *ast.PointsElement:
		for i := range elem.Items {
			populatePointItemLangRuns(&elem.Items[i])
		}

	case *ast.QuoteElement:
		elem.LangRuns = extractLangRuns(elem.Content, false)

	case *ast.ChecklistElement:
		for i := range elem.Items {
			populateChecklistItemLangRuns(&elem.Items[i])
		}

	case *ast.GridElement:
		for i := range elem.Columns {
			populateColumnLangRuns(&elem.Columns[i])
		}
	}
}

func populateColumnLangRuns(col *ast.ColumnElement) {
	for _, nested := range col.Elements {
		populateElementLangRuns(nested)
	}
}

func populatePointItemLangRuns(item *ast.PointItem) {
	item.LangRuns = extractLangRuns(item.Content, false)
	for i := range item.SubPoints {
		populatePointItemLangRuns(&item.SubPoints[i])
	}
}

func populateChecklistItemLangRuns(item *ast.ChecklistItem) {
	item.LangRuns = extractLangRuns(item.Content, false)
	for i := range item.SubItems {
		populateChecklistItemLangRuns(&item.SubItems[i])
	}
}

// extractLangRuns derives LangRuns from raw content — never from
// pre-rendered *HTML fields, which are still empty at this point in the
// pipeline regardless (PopulateInlineHTML/PopulateLangRuns both run after
// the linter — see core/renderer/populate_inline_html.go's own note on this
// same ordering).
//
// isRawHTML selects which source shape to scan: false for ordinary Markdown
// Content (the [texto]{lang=xx} span, matched via inlineLangSpanPattern —
// shared with ProcessInlineMarkdownFormatsSecure so the two can never drift
// on what a valid span looks like); true for a TextElement whose Content was
// already materialized to HTML at parse time (## headings), where the span
// already became a literal <span lang="xx"> by the time this runs.
//
// Every extracted tag is re-validated against a11y.IsValidLangTag regardless
// of source shape — an element flowing through an external --filter could
// set IsRawHTML on any TextElement and forge a <span lang> of its own, so
// trusting the RawHTML path without validation would reopen exactly the
// bypass PopulateLangRuns's "always re-derive, never skip" design exists to
// close.
func extractLangRuns(content string, isRawHTML bool) []ast.LangRun {
	if isRawHTML {
		return extractLangRunsFromHTML(content)
	}
	return extractLangRunsFromMarkdown(content)
}

func extractLangRunsFromMarkdown(content string) []ast.LangRun {
	matches := inlineLangSpanPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	var runs []ast.LangRun
	for _, m := range matches {
		text, tag := m[1], m[2]
		if !a11y.IsValidLangTag(tag) {
			continue
		}
		runs = append(runs, ast.LangRun{Text: text, Lang: tag})
	}
	return runs
}

func extractLangRunsFromHTML(content string) []ast.LangRun {
	matches := langSpanHTMLPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	var runs []ast.LangRun
	for _, m := range matches {
		tag, text := m[1], m[2]
		if !a11y.IsValidLangTag(tag) {
			continue
		}
		runs = append(runs, ast.LangRun{Text: text, Lang: tag})
	}
	return runs
}
