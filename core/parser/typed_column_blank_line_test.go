// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// The body of a typed column ends at its last non-blank line, and the blank
// lines after it are separation: they used to be reported as a body that is not
// indented under its marker (issue #430).
func TestStrictTypedColumn_BlankLinesAfterTheBodyAreSeparation(t *testing.T) {
	blanks := map[string]string{
		"no blank line":        "",
		"one blank line":       "\n",
		"two blank lines":      "\n\n",
		"whitespace-only line": "    \n",
	}
	for name, blank := range blanks {
		t.Run("before the next column/"+name, func(t *testing.T) {
			els := strictSlideBody(t, "  <<grid>>\n  <<column typed>>\n    TEXT\n      Left.\n"+blank+"  <<column>>\n  Right.\n  <<end>>\n  TEXT\n    After.\n")
			g := onlyGrid(t, els)
			if len(g.Columns) != 2 || len(g.Columns[0].Elements) != 1 || g.Columns[1].Content != "Right." {
				t.Errorf("columns %d, typed elements %d, raw %q", len(g.Columns), len(g.Columns[0].Elements), g.Columns[1].Content)
			}
		})
		t.Run("before <<end>>/"+name, func(t *testing.T) {
			els := strictSlideBody(t, "  <<grid>>\n  <<column>>\n  Left.\n  <<column typed>>\n    TEXT\n      Right.\n"+blank+"  <<end>>\n  TEXT\n    After.\n")
			g := onlyGrid(t, els)
			if len(g.Columns) != 2 || g.Columns[0].Content != "Left." || len(g.Columns[1].Elements) != 1 {
				t.Errorf("columns %d, raw %q, typed elements %d", len(g.Columns), g.Columns[0].Content, len(g.Columns[1].Elements))
			}
		})
	}
}

// Whatever the blank lines, the grid is the one without them and the element
// after it is still read.
func onlyGrid(t *testing.T, els []ast.Element) *ast.GridElement {
	t.Helper()
	if len(els) != 2 {
		t.Fatalf("want the grid and the TEXT after it, got %d elements", len(els))
	}
	g, ok := els[0].(*ast.GridElement)
	if !ok {
		t.Fatalf("first element is %#v, want a grid", els[0])
	}
	return g
}
