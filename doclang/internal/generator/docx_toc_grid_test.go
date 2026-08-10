// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"
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

// TestDOCXGenerator_CollectHeadings_CoversLevels5And6 covers a finding from
// the #40 code review: renderText's Level > 0 short-circuit routes level
// 5/6 headings to renderHeading (styled Heading4, a REAL Word heading), but
// collectHeadings only ever recognized h2/h3/h4 via regex — so the static
// TOC silently omitted headings that exist as genuine Word headings in the
// body. collectHeadings must now use the same Level-aware detection
// renderText/renderHeading use.
func TestDOCXGenerator_CollectHeadings_CoversLevels5And6(t *testing.T) {
	logger := newTestLogger()
	gen := NewDOCXGenerator(logger, "")

	subDos := ast.NewRawHTMLTextElement(diagnostics.NewPosition(2, 1), `<h2 id="sub-dos">Sub dos</h2>`)
	subDos.Level = 2
	detalle := ast.NewRawHTMLTextElement(diagnostics.NewPosition(3, 1), `<h5 id="detalle-profundo">Detalle profundo</h5>`)
	detalle.Level = 5

	doc := astWithElements(subDos, detalle)

	entries := gen.collectHeadings(doc)

	byTitle := map[string]TOCEntry{}
	for _, e := range entries {
		byTitle[e.Title] = e
	}

	if e, ok := byTitle["Sub dos"]; !ok || e.Level != 2 {
		t.Errorf("collectHeadings() missing/wrong level for %q: %+v (entries=%+v)", "Sub dos", e, entries)
	}
	if e, ok := byTitle["Detalle profundo"]; !ok || e.Level != 5 {
		t.Errorf("collectHeadings() missing/wrong level for %q: %+v (entries=%+v) — a level-5 heading exists as a real Word heading in the body but was omitted from the static TOC", "Detalle profundo", e, entries)
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

// TestDOCXGenerator_RenderTOC_FieldCoversOutlineLevel4 covers a finding from
// external review of #49: collectHeadings became Level-aware and now lists
// level 5/6 headings (which renderHeading styles as Heading4, outline level
// 4) in the static TOC placeholder, but the REAL Word TOC field (the "o"
// switch on NewTOCField) was still pinned to "1-3" — outline levels 1-3
// only cover Heading1-3. As soon as the reader refreshes the field (F9) or
// reopens the document, Word rebuilds the TOC from that switch and drops
// every Heading4-styled entry (levels 4, 5, and 6), diverging from the
// static placeholder that collectHeadings produced. The switch must extend
// to "1-4" to match what renderHeading can actually produce.
func TestDOCXGenerator_RenderTOC_FieldCoversOutlineLevel4(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)

	deep := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1), `<h5 id="deep">Deep</h5>`)
	deep.Level = 5
	doc := astWithElements(deep)

	output := filepath.Join(t.TempDir(), "toc.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx", TOC: true}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	xml := docxDocumentXML(t, output)
	// docxgo v2.12.0's buildTOCCode emits a single backslash per switch, as
	// OOXML/Word expects (docxgo commit d66e4cb, "single-backslash TOC
	// switches"). Older docxgo (<=v2.6.0, pinned when this test was
	// written) emitted a literal double backslash, which Word treats as an
	// unrecognized switch — so the outline-level restriction this test
	// guards was never actually applied by Word before that fix.
	if !strings.Contains(xml, `TOC \o &#34;1-4&#34;`) {
		t.Errorf("expected the TOC field's outline switch to cover level 4 (\\o \"1-4\"), document.xml:\n%s", xml)
	}
}
