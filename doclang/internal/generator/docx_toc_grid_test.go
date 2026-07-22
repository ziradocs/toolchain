// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestDOCXGenerator_CollectHeadings_IncludesGridColumnHeadings covers issue
// #88: collectHeadings only walked block.Elements looking for
// *ast.TextElement, never recursing into GridElement.Columns[].Content.
// Meanwhile renderGrid (issue #56) does render "## "/"### "/"#### "
// prefixed lines inside a grid column as real Word Heading2/3/4 styled
// paragraphs via renderText — which Word's live TOC field (o:1-3)
// autodetects on refresh (F9). That made the static placeholder TOC
// (built from collectHeadings) diverge from what Word's own TOC field
// shows. The fix makes collectHeadings walk grid columns the same way.
func TestDOCXGenerator_CollectHeadings_IncludesGridColumnHeadings(t *testing.T) {
	logger := newTestLogger()
	gen := NewDOCXGenerator(logger, "")
	doc := astWithGrid() // "### Column Heading\nContenido columna A" in column A

	entries := gen.collectHeadings(doc)

	var found *TOCEntry
	for i := range entries {
		if entries[i].Title == "Column Heading" {
			found = &entries[i]
			break
		}
	}

	if found == nil {
		t.Fatalf("collectHeadings() did not find heading %q from inside a grid column; entries = %+v", "Column Heading", entries)
	}
	if found.Level != 3 {
		t.Errorf("collectHeadings() grid column heading level = %d, want %d (### is H3)", found.Level, 3)
	}
	if found.BookmarkID == "" {
		t.Errorf("collectHeadings() grid column heading has empty BookmarkID")
	}
}

// TestDOCXGenerator_CollectHeadings_IgnoresIndentedGridColumnLines guards
// against a divergence the initial #88 fix introduced: renderGrid (docx.go)
// only uses strings.TrimSpace(line) to decide whether a grid-column line is
// BLANK — the text it actually hands to renderText is the line UNTRIMMED,
// and renderText's heading regexes are anchored with "^" (no tolerance for
// leading whitespace). If collectHeadings matched against the TRIMMED line
// instead, an indented "  ## Heading" line inside a column (parseColumn
// preserves original indentation, see core/elements/grid.go)
// would be added to the static TOC even though renderGrid renders it as a
// plain paragraph, not a Word heading style — reintroducing the exact
// TOC/render divergence #88 was about, just in the opposite direction.
func TestDOCXGenerator_CollectHeadings_IgnoresIndentedGridColumnLines(t *testing.T) {
	logger := newTestLogger()
	gen := NewDOCXGenerator(logger, "")

	doc := newTestAST()
	grid := ast.NewGridElement(diagnostics.NewPosition(3, 1))
	col := ast.NewColumnElement(diagnostics.NewPosition(3, 1), "  ## Indented Heading\nOther text")
	grid.Columns = append(grid.Columns, *col)

	block := doc.ContentBlocks[0]
	block.Elements = append(block.Elements, grid)
	doc.ContentBlocks[0] = block

	entries := gen.collectHeadings(doc)

	for _, e := range entries {
		if e.Title == "Indented Heading" {
			t.Errorf("collectHeadings() treated an indented grid-column line as a heading (%+v), but renderGrid does not render it as one (renderText's ^## pattern requires no leading whitespace)", e)
		}
	}
}
