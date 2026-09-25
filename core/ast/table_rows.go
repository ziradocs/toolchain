// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"fmt"
	"reflect"
)

// HasTableRows reports whether this table uses the opt-in authored form.
func (t *TableElement) HasTableRows() bool { return t.TableRows != nil }

// DeriveTableViews computes legacy read-only views from the authored rows.
// Identity and section are deliberately absent from these projections.
func DeriveTableViews(rows []TableRow) ([]string, [][]string, [][]TableCell) {
	cells := make([][]TableCell, len(rows))
	for i, row := range rows {
		cells[i] = make([]TableCell, len(row.Cells))
		for j, c := range row.Cells {
			cells[i][j] = TableCell{Content: c.Content, IsHeader: c.IsHeader,
				Scope: c.Scope, ColSpan: c.ColSpan, RowSpan: c.RowSpan}
		}
	}
	headers, flatRows := FlattenCellsToRows(cells)
	return headers, flatRows, cells
}

// SyncTableViews replaces compatibility projections. Use only when the
// authored rows have just been parsed or deliberately transformed.
func (t *TableElement) SyncTableViews() {
	if !t.HasTableRows() {
		return
	}
	t.Headers, t.Rows, t.Cells = DeriveTableViews(t.TableRows)
	t.RowPositions = nil
}

// ValidateTableRows rejects invalid authored structure and divergent
// projections. It never repairs a JSON/filter response silently.
func ValidateTableRows(t *TableElement) error {
	if !t.HasTableRows() {
		return nil
	}
	if len(t.TableRows) == 0 {
		return fmt.Errorf("tableRows must contain at least one authored row")
	}
	if t.TableRows[0].Cells == nil {
		return fmt.Errorf("table row %q must declare cells (use [] for a fully covered row)", t.TableRows[0].NodeID)
	}
	width := 0
	for _, c := range t.TableRows[0].Cells {
		span := normalizedTableSpan(c.ColSpan)
		if span > MaxCellSpan-width {
			return fmt.Errorf("tableRows first row exceeds supported width %d", MaxCellSpan)
		}
		width += span
	}
	if width == 0 || width > MaxCellSpan {
		return fmt.Errorf("tableRows first row has invalid width %d", width)
	}
	if len(t.TableRows) > 1_000_000/width {
		return fmt.Errorf("tableRows grid exceeds the supported size")
	}
	occupied := make([][]bool, len(t.TableRows))
	for i := range occupied {
		occupied[i] = make([]bool, width)
	}
	stage := 0 // header, body, footer
	for i, row := range t.TableRows {
		section := row.Section
		if section == "" {
			section = "body"
		}
		var next int
		switch section {
		case "header":
			next = 0
		case "body":
			next = 1
		case "footer":
			next = 2
		default:
			return fmt.Errorf("table row %q has invalid section %q", row.NodeID, row.Section)
		}
		if next < stage {
			return fmt.Errorf("table row %q section %q follows a later section", row.NodeID, section)
		}
		stage = next
		if row.Cells == nil {
			return fmt.Errorf("table row %q must declare cells (use [] for a fully covered row)", row.NodeID)
		}
		col := 0
		for _, cell := range row.Cells {
			for col < width && occupied[i][col] {
				col++
			}
			cs, rs := normalizedTableSpan(cell.ColSpan), normalizedTableSpan(cell.RowSpan)
			if cell.ColSpan < 0 || cell.RowSpan < 0 || cs > MaxCellSpan || rs > MaxCellSpan || col+cs > width || i+rs > len(t.TableRows) {
				return fmt.Errorf("table cell %q in row %q has span outside the table grid", cell.NodeID, row.NodeID)
			}
			if cell.Scope != "" && (cell.Scope != "row" && cell.Scope != "col" || !cell.IsHeader) {
				return fmt.Errorf("table cell %q has invalid header scope %q", cell.NodeID, cell.Scope)
			}
			for r := i + 1; r < i+rs; r++ {
				targetSection := t.TableRows[r].Section
				if targetSection == "" {
					targetSection = "body"
				}
				if targetSection != section {
					return fmt.Errorf("table cell %q rowspan crosses %s/%s section boundary", cell.NodeID, section, targetSection)
				}
			}
			for r := i; r < i+rs; r++ {
				for c := col; c < col+cs; c++ {
					if occupied[r][c] {
						return fmt.Errorf("table cell %q in row %q overlaps another span", cell.NodeID, row.NodeID)
					}
					occupied[r][c] = true
				}
			}
			col += cs
		}
		for c, filled := range occupied[i] {
			if !filled {
				return fmt.Errorf("table row %q leaves uncovered column %d", row.NodeID, c+1)
			}
		}
	}
	headers, rows, cells := DeriveTableViews(t.TableRows)
	if !reflect.DeepEqual(t.Headers, headers) || !reflect.DeepEqual(t.Rows, rows) || !reflect.DeepEqual(t.Cells, cells) || len(t.RowPositions) != 0 {
		return fmt.Errorf("tableRows projections headers/rows/cells/rowPositions conflict with authored rows")
	}
	return nil
}

func normalizedTableSpan(span int) int {
	if span <= 1 {
		return 1
	}
	return span
}
