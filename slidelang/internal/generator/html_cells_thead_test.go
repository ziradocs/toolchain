// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// TestRenderHTMLPreview_MergedTableCells_WrapsHeaderRowInThead covers a
// finding from the #41 code review: the template's {{if .Cells}} branch
// (added for issue #20) emitted every row inside a single <tbody>, with no
// <thead> at all — diverging both from the legacy {{else}} branch just
// below it (still <thead>-wrapped) and from doclang's HTML output for the
// identical AST (core/renderer/html.go's renderTableCells). Uses
// astWithMergedTable from pptx_merged_cells_test.go, whose Cells[0] is a
// header row (IsHeader+Scope=col) spanning both columns.
func TestRenderHTMLPreview_MergedTableCells_WrapsHeaderRowInThead(t *testing.T) {
	doc := astWithMergedTable()

	g := New(util.NewNoop())
	html, err := g.RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	theadIdx := strings.Index(html, "<thead>")
	if theadIdx == -1 {
		t.Fatalf("expected a <thead> in the merged-cells table output, got:\n%s", html)
	}
	theadEnd := strings.Index(html, "</thead>")
	if theadEnd == -1 || theadEnd < theadIdx {
		t.Fatalf("expected a matching </thead>, got:\n%s", html)
	}
	if !strings.Contains(html[theadIdx:theadEnd], "Wide Header") {
		t.Errorf("expected the header cell's text inside <thead>, got:\n%s", html[theadIdx:theadEnd])
	}

	tbodyIdx := strings.Index(html, "<tbody>")
	if tbodyIdx == -1 || tbodyIdx < theadEnd {
		t.Fatalf("expected <tbody> to follow </thead>, got:\n%s", html)
	}
	if strings.Contains(html[tbodyIdx:], "Wide Header") {
		t.Errorf("the header row must not also be repeated inside <tbody>, got:\n%s", html)
	}
	if !strings.Contains(html[tbodyIdx:], "A1") || !strings.Contains(html[tbodyIdx:], "B1") {
		t.Errorf("expected the body row's cells inside <tbody>, got:\n%s", html)
	}
}
