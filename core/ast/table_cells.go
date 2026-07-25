// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

// MaxCellSpan caps ColSpan/RowSpan when flattening explicit `cells:`
// authoring input into a rectangular grid (FlattenCellsToRows). Without a
// cap, an absurd span (e.g. `colspan: 999999999`, ~15 bytes of YAML) would
// make the flattening loop below expand it into a slice with that many
// entries at PARSE time — a near-zero-cost denial-of-service hitting every
// output format (html/json/--lint-only) plus DecodeAST for any externally
// supplied AST JSON. 1000 is far beyond any real document's column/row
// count while staying cheap to allocate.
const MaxCellSpan = 1000

// DeriveCellsFromFlat builds TableElement.Cells from the flat Headers/Rows
// view that every table parser already produces today (markdown pipe,
// simple YAML, strict). The first row (if there are headers) is marked
// IsHeader+Scope="col" (the same criterion renderTableElement already uses
// to emit `<th scope="col">` for each header); body rows become plain
// cells with no span. This is the "simple" half of the issue #20 seam — the
// "merged" half is FlattenCellsToRows, used when the author declares an
// explicit `cells:` block.
func DeriveCellsFromFlat(headers []string, rows [][]string) [][]TableCell {
	if len(headers) == 0 && len(rows) == 0 {
		return [][]TableCell{}
	}

	cells := make([][]TableCell, 0, len(rows)+1)

	if len(headers) > 0 {
		headerRow := make([]TableCell, len(headers))
		for i, h := range headers {
			headerRow[i] = TableCell{Content: h, IsHeader: true, Scope: "col"}
		}
		cells = append(cells, headerRow)
	}

	for _, row := range rows {
		bodyRow := make([]TableCell, len(row))
		for i, c := range row {
			bodyRow[i] = TableCell{Content: c}
		}
		cells = append(cells, bodyRow)
	}

	return cells
}

// cellCarry tracks a RowSpan>1 cell that keeps occupying a column in the
// rows following the one that declared it.
type cellCarry struct {
	content  string
	isHeader bool
	left     int // remaining rows (including the current one) this cell occupies
}

// FlattenCellsToRows derives the flat Headers/Rows view from Cells when the
// author declared explicit merged cells (YAML `cells:` block, issue #20).
// The derivation expands each colspan/rowspan by repeating the cell's
// content into every slot it covers, producing a RECTANGULAR grid (every
// row the same width) — this is deliberate: it's what keeps
// linter.ElementStructureRule (TABLE003, Error severity) from reporting a
// false positive for "inconsistent column count" on a legitimately merged
// table, since TABLE003 compares len(row) against len(Headers) with no
// concept of span.
//
// Known, accepted limitation: if a cell spec overlaps (two cells claiming
// the same slot via crossed colspan/rowspan) the result of this function may
// not reflect a valid HTML layout — Cells itself (the source of truth)
// preserves the declared values without corruption; only the derived flat
// view can be imprecise in that edge case. The same applies to an
// undersized row (fewer own cells declared than the columns not already
// covered by a carry): the gap is filled with an empty placeholder instead
// of dropping every carry beyond it (see the loop below).
func FlattenCellsToRows(cells [][]TableCell) ([]string, [][]string) {
	if len(cells) == 0 {
		return []string{}, [][]string{}
	}

	carries := map[int]*cellCarry{}
	var grid [][]string
	maxWidth := 0

	for _, srcRow := range cells {
		var outRow []string
		col := 0
		srcIdx := 0

		for srcIdx < len(srcRow) || carryExistsAtOrBeyond(carries, col) {
			if c, ok := carries[col]; ok && c.left > 0 {
				outRow = append(outRow, c.content)
				col++
				continue
			}
			if srcIdx >= len(srcRow) {
				// No carry at this exact column, no more own cells declared
				// for this row, but a carry exists further out (undersized/
				// malformed row) — emit an empty placeholder and keep
				// scanning toward it instead of silently dropping every
				// carry beyond this gap.
				outRow = append(outRow, "")
				col++
				continue
			}

			cell := srcRow[srcIdx]
			span := clampSpan(cell.ColSpan)
			rspan := clampSpan(cell.RowSpan)
			for k := 0; k < span; k++ {
				outRow = append(outRow, cell.Content)
				if rspan > 1 {
					carries[col+k] = &cellCarry{content: cell.Content, isHeader: cell.IsHeader, left: rspan}
				}
			}
			col += span
			srcIdx++
		}

		// This row "consumes" one turn of every active rowspan carry,
		// including the ones just created above (left started at rspan,
		// which counts the current row).
		for c, oc := range carries {
			oc.left--
			if oc.left <= 0 {
				delete(carries, c)
			}
		}

		if len(outRow) > maxWidth {
			maxWidth = len(outRow)
		}
		grid = append(grid, outRow)
	}

	for i := range grid {
		for len(grid[i]) < maxWidth {
			grid[i] = append(grid[i], "")
		}
	}

	firstRowAllHeader := len(cells[0]) > 0
	for _, c := range cells[0] {
		if !c.IsHeader {
			firstRowAllHeader = false
			break
		}
	}

	if firstRowAllHeader {
		return grid[0], grid[1:]
	}
	return []string{}, grid
}

// clampSpan normalizes a declared ColSpan/RowSpan to the [1, MaxCellSpan]
// range: values below 1 mean "no span" (1), values above MaxCellSpan are
// clamped to prevent the DoS described on MaxCellSpan's doc comment.
func clampSpan(span int) int {
	if span < 1 {
		return 1
	}
	if span > MaxCellSpan {
		return MaxCellSpan
	}
	return span
}

func carryExistsAtOrBeyond(carries map[int]*cellCarry, col int) bool {
	for c := range carries {
		if c >= col {
			return true
		}
	}
	return false
}
