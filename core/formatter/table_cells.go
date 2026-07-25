// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
)

// formatTableElement serializes a TableElement for either dialect. The
// underlying TABLE-block parser (elements.TableParser) is shared and not
// mode-gated (issue #20 widened its CanParse to flex, and its
// caption:/label:/cells: handling was never mode-gated to begin with), so
// caption/label/cells are representable in both strict and flex — only the
// choice of which form to emit depends on content, not on dialect.
//
// Using the pipe form when it isn't warranted would silently drop
// information: Label has no representation outside the TABLE block, and a
// Cells shape richer than what Headers/Rows already encode (row-scoped
// headers, colspan, rowspan) would be flattened away — precisely the
// accessibility-relevant structure issue #20 exists to carry. This was a
// real bug found in code review: doclang's formatter emitted the pipe form
// whenever Caption was empty, silently dropping Label and any non-trivial
// Cells structure on `fmt --write`.
func formatTableElement(e *ast.TableElement) (string, error) {
	if !tableNeedsCellsBlock(e) && e.Caption == "" && e.Label == "" {
		// formatPipeTable delimits with "|", not quotes — it doesn't go
		// through quote()/checkQuotable, so it doesn't inherit that
		// limitation (a literal quote in a header/cell is representable in
		// the pipe form unchanged; a literal "|" would be a different,
		// pre-existing problem, out of scope here).
		return formatPipeTable(e.Headers, e.Rows), nil
	}
	return formatTableCellsBlock(e)
}

// tableNeedsCellsBlock reports whether e must round-trip through the
// explicit TABLE "cells:" form rather than the flat pipe form: true when
// Label is set (not representable outside the TABLE block) or when
// e.Cells carries structure beyond what ast.DeriveCellsFromFlat(e.Headers,
// e.Rows) already encodes (row-scoped headers, colspan, rowspan).
func tableNeedsCellsBlock(e *ast.TableElement) bool {
	return e.Label != "" || !tableCellsAreSimple(e)
}

// tableCellsAreSimple reports whether e.Cells is exactly the shape
// ast.DeriveCellsFromFlat(e.Headers, e.Rows) would produce — i.e. the
// table carries no explicit cell structure beyond what the flat
// Headers/Rows view already encodes. ColSpan/RowSpan are compared via
// normalizeSpan since 0 and 1 both mean "no span".
//
// An empty e.Cells is treated as simple, not as "no structure could be
// derived": Cells is additive/omitempty, so nothing guarantees a caller
// that builds a TableElement directly (rather than through
// elements.TableParser, which always populates it) keeps it in sync with
// Headers/Rows — treating empty Cells as "not simple" would route such a
// table into formatTableCellsBlock, which would then have nothing to
// serialize and silently drop every row (worse than the bug this file
// fixes). formatTableCellsBlock has the matching fallback: it derives
// Cells from Headers/Rows itself when e.Cells is empty.
func tableCellsAreSimple(e *ast.TableElement) bool {
	if len(e.Cells) == 0 {
		return true
	}
	derived := ast.DeriveCellsFromFlat(e.Headers, e.Rows)
	if len(e.Cells) != len(derived) {
		return false
	}
	for i := range e.Cells {
		if len(e.Cells[i]) != len(derived[i]) {
			return false
		}
		for j := range e.Cells[i] {
			a, b := e.Cells[i][j], derived[i][j]
			if a.Content != b.Content || a.IsHeader != b.IsHeader || a.Scope != b.Scope ||
				normalizeSpan(a.ColSpan) != normalizeSpan(b.ColSpan) ||
				normalizeSpan(a.RowSpan) != normalizeSpan(b.RowSpan) {
				return false
			}
		}
	}
	return true
}

// normalizeSpan treats an unset span (0) and an explicit "no merge" span
// (1) as equivalent, matching ast.FlattenCellsToRows/clampSpan's own
// normalization.
func normalizeSpan(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// formatTableCellsBlock emits the TABLE form with an explicit "cells:" key
// — the source of truth for structure (see ast.FlattenCellsToRows) — plus
// optional caption:/label:. Headers/Rows are never emitted here: the
// parser (elements.TableParser) derives them back from Cells on reparse.
func formatTableCellsBlock(e *ast.TableElement) (string, error) {
	if e.Caption != "" {
		if err := checkQuotable("table", "caption", e.Caption); err != nil {
			return "", err
		}
	}
	if e.Label != "" {
		if err := checkQuotable("table", "label", e.Label); err != nil {
			return "", err
		}
	}

	cells := e.Cells
	if len(cells) == 0 {
		// See tableCellsAreSimple's doc comment: an unpopulated Cells is
		// derived on the fly rather than serialized as-is (which would
		// silently drop every row).
		cells = ast.DeriveCellsFromFlat(e.Headers, e.Rows)
	}
	cellsYAML, err := formatTableCellsYAML(cells)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("TABLE\n")
	if e.Caption != "" {
		fmt.Fprintf(&b, "  caption: %s\n", quote(e.Caption))
	}
	if e.Label != "" {
		fmt.Fprintf(&b, "  label: %s\n", quote(e.Label))
	}
	b.WriteString("  cells:\n")
	b.WriteString(indent(cellsYAML, 4))
	return strings.TrimRight(b.String(), "\n"), nil
}

// formatTableCellsYAML serializes cells as the flow-style YAML sequence the
// "cells:" authoring syntax expects (see parseCellsYAML in
// internal/elements/table.go): one block-sequence item per row, each row a
// flow sequence of flow mappings, e.g.
// `- [{content: A, header: true, colspan: 2}, {content: B, header: true}]`.
//
// Built via yaml.Node rather than hand-formatted strings (unlike quote()
// elsewhere in this package) because cell content is free-form prose that
// can contain quotes, colons, or commas — exactly what YAML's own quoting
// rules exist to handle safely, and hand-rolling that would re-implement a
// YAML encoder. Content only needs escaping *at all* because parseCellsYAML
// on the other end is a real yaml.Unmarshal, not a hand-rolled reader (like
// the rest of the legacy strict dialect), so round-tripping through the
// same library guarantees whatever is emitted here parses back correctly.
func formatTableCellsYAML(cells [][]ast.TableCell) (string, error) {
	outer := &yaml.Node{Kind: yaml.SequenceNode}
	for _, row := range cells {
		rowNode := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, c := range row {
			cellNode := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
			appendYAMLKV(cellNode, "content", c.Content)
			if c.IsHeader {
				appendYAMLKV(cellNode, "header", true)
			}
			if c.Scope != "" {
				appendYAMLKV(cellNode, "scope", c.Scope)
			}
			if c.ColSpan > 1 {
				appendYAMLKV(cellNode, "colspan", c.ColSpan)
			}
			if c.RowSpan > 1 {
				appendYAMLKV(cellNode, "rowspan", c.RowSpan)
			}
			rowNode.Content = append(rowNode.Content, cellNode)
		}
		outer.Content = append(outer.Content, rowNode)
	}

	out, err := yaml.Marshal(outer)
	if err != nil {
		return "", fmt.Errorf("marshaling table cells: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// appendYAMLKV appends a key/value pair of scalar nodes to a MappingNode's
// Content (yaml.Node stores mapping entries as a flat alternating
// key/value slice, not key-value pairs). Node.Encode picks the correct
// Kind/Tag/quoting style for the Go value automatically.
func appendYAMLKV(node *yaml.Node, key string, value interface{}) {
	keyNode := &yaml.Node{}
	_ = keyNode.Encode(key)
	valNode := &yaml.Node{}
	_ = valNode.Encode(value)
	node.Content = append(node.Content, keyNode, valNode)
}
