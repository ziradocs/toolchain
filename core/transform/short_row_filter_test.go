// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"os/exec"
	"reflect"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Un filtro que le quita celdas a una fila no deja una fila corta en el AST
// que sigue al lint y a los generadores: DecodeAST la completa con celdas
// vacías y la anota para el TABLE010 del linter (ver
// ast.PadShortTableRows y TableElement.DecodePaddedRows).
func TestFilterThatTrimsARowGetsItCompleted(t *testing.T) {
	if _, err := exec.LookPath("sed"); err != nil {
		t.Skip("sed no disponible en PATH")
	}
	src := "# D\n\n| A | B | C |\n|---|---|---|\n| 1 | 2 | 3 |\n| 4 | 5 | 6 |\n"
	doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "d.doclang")
	trim := writeFilter(t, `sed -e 's/\["4","5","6"\]/["4"]/' -e 's/{"content":"4"},{"content":"5"},{"content":"6"}/{"content":"4"}/'`)
	out, err := RunFilters(doc, []string{trim}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var table *ast.TableElement
	_ = ast.Walk(out, func(n ast.Node) error {
		if tbl, ok := n.(*ast.TableElement); ok {
			table = tbl
		}
		return nil
	})
	if table == nil {
		t.Fatal("no table")
	}
	if want := [][]string{{"1", "2", "3"}, {"4", "", ""}}; !reflect.DeepEqual(table.Rows, want) {
		t.Fatalf("rows %q, want %q", table.Rows, want)
	}
	if len(table.Cells) != 3 || len(table.Cells[2]) != 3 {
		t.Fatalf("cells not completed: %+v", table.Cells)
	}
	if padded := table.DecodePaddedRows(); len(padded) != 1 || padded[1] != 1 || table.RowPositions[1].Line != 6 {
		t.Fatalf("padded rows %v, want the second row (line 6), which had 1 cell", padded)
	}
}
