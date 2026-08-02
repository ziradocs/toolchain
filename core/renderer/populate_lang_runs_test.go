// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestPopulateLangRuns_Markdown(t *testing.T) {
	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), "dijo [bonjour tout le monde]{lang=fr} y se fue")
	doc := newTestDoc(elem)

	PopulateLangRuns(doc)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 {
		t.Fatalf("expected 1 LangRun, got %d: %+v", len(got), got)
	}
	if got[0].Text != "bonjour tout le monde" || got[0].Lang != "fr" {
		t.Errorf("unexpected run: %+v", got[0])
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

	PopulateLangRuns(doc)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 {
		t.Fatalf("expected 1 LangRun, got %d: %+v", len(got), got)
	}
	if got[0].Text != "tout le monde" || got[0].Lang != "fr" {
		t.Errorf("unexpected run: %+v", got[0])
	}
}

// TestPopulateLangRuns_RawHTML_NestedFormatting confirms a <strong>/<em>
// nested inside the materialized <span lang> survives as part of Text — the
// RawHTML path can't recover the original markdown, only the HTML that was
// already emitted (see extractLangRuns's doc comment).
func TestPopulateLangRuns_RawHTML_NestedFormatting(t *testing.T) {
	elem := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1),
		`<h2 id="x"><span lang="fr">a <strong>b</strong> c</span></h2>`)
	doc := newTestDoc(elem)

	PopulateLangRuns(doc)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if len(got) != 1 || got[0].Text != "a <strong>b</strong> c" || got[0].Lang != "fr" {
		t.Errorf("unexpected runs: %+v", got)
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
			PopulateLangRuns(doc)
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

	PopulateLangRuns(doc)

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

	PopulateLangRuns(doc)

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

	PopulateLangRuns(doc)

	gotGrid := doc.ContentBlocks[0].Elements[0].(*ast.GridElement)
	gotText := gotGrid.Columns[0].Elements[0].(*ast.TextElement)
	if len(gotText.LangRuns) != 1 || gotText.LangRuns[0].Lang != "fr" {
		t.Errorf("nested TextElement inside grid column not populated: %+v", gotText.LangRuns)
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

		PopulateLangRuns(doc)

		got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
		if len(got) != 0 {
			t.Errorf("expected forged LangRuns to be replaced with the true (empty) derivation, got %+v", got)
		}
	})

	t.Run("rawHTML path", func(t *testing.T) {
		elem := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1), `<h2 id="x">no lang span here</h2>`)
		elem.LangRuns = []ast.LangRun{{Text: "forged", Lang: "xx-forged-by-filter"}}
		doc := newTestDoc(elem)

		PopulateLangRuns(doc)

		got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
		if len(got) != 0 {
			t.Errorf("expected forged LangRuns to be replaced with the true (empty) derivation, got %+v", got)
		}
	})
}

func TestPopulateLangRuns_Idempotent(t *testing.T) {
	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), "[bonjour]{lang=fr} et [hallo]{lang=de}")
	doc := newTestDoc(elem)

	PopulateLangRuns(doc)
	first := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns

	PopulateLangRuns(doc)
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
	PopulateLangRuns(nil) // must not panic
}

func TestPopulateLangRuns_NoSpansYieldsNilNotEmptySlice(t *testing.T) {
	elem := ast.NewTextElement(diagnostics.NewPosition(1, 1), "plain text, no spans at all")
	doc := newTestDoc(elem)

	PopulateLangRuns(doc)

	got := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).LangRuns
	if got != nil {
		t.Errorf("expected nil LangRuns (omitempty-friendly), got %+v", got)
	}
}
