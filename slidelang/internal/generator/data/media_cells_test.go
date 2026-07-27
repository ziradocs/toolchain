// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// TestPrepareTemplateData_MediaElement covers issue #37: a MediaElement
// must reach ElementData with its fields populated, not the near-empty
// struct the converter's default case produces for unhandled types.
func TestPrepareTemplateData_MediaElement(t *testing.T) {
	astDoc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{{
			BlockType: "content",
			Elements: []ast.Element{
				&ast.MediaElement{MediaType: "video", Source: "demo.mp4", Controls: true, Loop: true},
			},
		}},
	}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	el := got.ContentBlocks[0].Elements[0]

	if el.MediaType != "video" {
		t.Errorf("MediaType = %q, want %q", el.MediaType, "video")
	}
	if el.Source != "demo.mp4" {
		t.Errorf("Source = %q, want %q", el.Source, "demo.mp4")
	}
	if !el.Controls || !el.Loop {
		t.Errorf("Controls/Loop not propagated: got Controls=%v Loop=%v", el.Controls, el.Loop)
	}
	if el.Autoplay || el.Muted {
		t.Errorf("Autoplay/Muted should be false (not set on the source element), got Autoplay=%v Muted=%v", el.Autoplay, el.Muted)
	}
}

// TestPrepareTemplateData_MediaElement_DangerousSourceBlocked mirrors the
// ImageElement security contract: a javascript:/data: source must not reach
// the template — Source ends up empty, which the template's {{if .Source}}
// branch treats as "blocked", the same way it already does for images.
func TestPrepareTemplateData_MediaElement_DangerousSourceBlocked(t *testing.T) {
	astDoc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{{
			BlockType: "content",
			Elements: []ast.Element{
				&ast.MediaElement{MediaType: "video", Source: "javascript:alert(1)"},
			},
		}},
	}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	el := got.ContentBlocks[0].Elements[0]

	if el.Source != "" {
		t.Errorf("expected a dangerous scheme to be blocked (empty Source), got %q", el.Source)
	}
}

// TestPrepareTemplateData_TableCells_SimpleTableLeavesCellsNil covers the
// gate: a table whose Cells is exactly the DeriveCellsFromFlat shape must
// NOT populate ElementData.Cells — otherwise the template's {{if .Cells}}
// branch would fire for every existing simple table, changing its HTML.
func TestPrepareTemplateData_TableCells_SimpleTableLeavesCellsNil(t *testing.T) {
	table := &ast.TableElement{
		Headers: []string{"H1", "H2"},
		Rows:    [][]string{{"a", "b"}},
	}
	table.Cells = ast.DeriveCellsFromFlat(table.Headers, table.Rows)

	astDoc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{{
			BlockType: "content",
			Elements:  []ast.Element{table},
		}},
	}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	el := got.ContentBlocks[0].Elements[0]

	if el.Cells != nil {
		t.Errorf("expected Cells to stay nil for a simple table, got %+v", el.Cells)
	}
	if len(el.Headers) != 2 || len(el.Rows) != 1 {
		t.Errorf("expected Headers/Rows to still be populated (unchanged legacy path), got Headers=%v Rows=%v", el.Headers, el.Rows)
	}
}

// TestPrepareTemplateData_TableCells_MergedTablePopulatesCells covers the
// other side: a table with a real colspan must populate ElementData.Cells,
// with Content processed the same way (ProcessTextWithVariablesAndMarkdown)
// as the Headers/Rows path.
func TestPrepareTemplateData_TableCells_MergedTablePopulatesCells(t *testing.T) {
	table := &ast.TableElement{
		Headers: []string{"Wide", ""},
		Rows:    [][]string{{"a", "b"}},
		Cells: [][]ast.TableCell{
			{{Content: "Wide", IsHeader: true, Scope: "col", ColSpan: 2}},
			{{Content: "a"}, {Content: "b"}},
		},
	}

	astDoc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{{
			BlockType: "content",
			Elements:  []ast.Element{table},
		}},
	}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	el := got.ContentBlocks[0].Elements[0]

	if len(el.Cells) != 2 {
		t.Fatalf("expected Cells to be populated with 2 rows, got %+v", el.Cells)
	}
	if el.Cells[0][0].ColSpan != 2 || !el.Cells[0][0].IsHeader {
		t.Errorf("expected the anchor cell's ColSpan/IsHeader to survive conversion, got %+v", el.Cells[0][0])
	}
}
