// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// TestFormatPipeTable_EscapesLiteralPipe_RoundTrips es el repro de una
// revisión sobre F10 (audit 2026-09-11): elements.splitMarkdownTableRow
// decodifica "\|" a "|" literal en la celda parseada, pero antes de este
// fix formatPipeTable reemitía esa celda uniendo con " | " SIN volver a
// escaparla — un cell.Rows[i][j] == "x | y" se reserializaba como
// "| x | y |", que el reparse lee como DOS celdas, no una. Antes de que el
// parser supiera decodificar "\|" esto era inalcanzable (ninguna celda
// parseada podía contener un "|" crudo); el fix del parser lo hizo posible
// y por lo tanto el del formatter tiene que acompañarlo en el mismo PR.
func TestFormatPipeTable_EscapesLiteralPipe_RoundTrips(t *testing.T) {
	table := ast.NewTableElement(diagnostics.NewPosition(3, 1))
	table.Headers = []string{"A", "B"}
	table.Rows = [][]string{{"x | y", "z"}}

	out, err := FormatStrict(chartDoc(table))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if !strings.Contains(out, `x \| y`) {
		t.Fatalf("la celda no se reescapó al formatear:\n%s", out)
	}

	reparsed, diags := parser.New(util.NewNoop()).Parse(out, "test.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("reparse produjo un error: %v\n%s", d, out)
		}
	}
	var reTable *ast.TableElement
	for _, block := range reparsed.ContentBlocks {
		for _, el := range block.Elements {
			if tb, ok := el.(*ast.TableElement); ok {
				reTable = tb
			}
		}
	}
	if reTable == nil {
		t.Fatalf("no se encontró el TableElement reparseado:\n%s", out)
	}
	if len(reTable.Rows) != 1 || len(reTable.Rows[0]) != 2 {
		t.Fatalf("Rows reparseado = %#v, want 1 fila de 2 celdas (el \"|\" reescapado no debe volver a partir la fila)", reTable.Rows)
	}
	if reTable.Rows[0][0] != "x | y" {
		t.Errorf("Rows[0][0] reparseado = %q, want \"x | y\"", reTable.Rows[0][0])
	}
}

// TestFormatPipeTable_CodeSpanPipe_NotDoubleEscaped confirma que un "|"
// DENTRO de un code span no se reescapa (los backticks ya lo protegen en el
// reparse) — escaparlo ahí metería un "\" visible dentro del <code>
// renderizado, que un code span no interpreta como escape.
func TestFormatPipeTable_CodeSpanPipe_NotDoubleEscaped(t *testing.T) {
	table := ast.NewTableElement(diagnostics.NewPosition(3, 1))
	table.Headers = []string{"Method", "Return"}
	table.Rows = [][]string{{"`getUser()`", "`User | null`"}}

	out, err := FormatStrict(chartDoc(table))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if strings.Contains(out, `\|`) {
		t.Fatalf("un \"|\" dentro de un code span no debería reescaparse:\n%s", out)
	}
	if !strings.Contains(out, "`User | null`") {
		t.Fatalf("el code span debería salir intacto:\n%s", out)
	}

	reparsed, diags := parser.New(util.NewNoop()).Parse(out, "test.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("reparse produjo un error: %v\n%s", d, out)
		}
	}
	var reTable *ast.TableElement
	for _, block := range reparsed.ContentBlocks {
		for _, el := range block.Elements {
			if tb, ok := el.(*ast.TableElement); ok {
				reTable = tb
			}
		}
	}
	if reTable == nil {
		t.Fatalf("no se encontró el TableElement reparseado:\n%s", out)
	}
	if len(reTable.Rows) != 1 || len(reTable.Rows[0]) != 2 {
		t.Fatalf("Rows reparseado = %#v, want 1 fila de 2 celdas", reTable.Rows)
	}
	if reTable.Rows[0][1] != "`User | null`" {
		t.Errorf("Rows[0][1] reparseado = %q, want \"`User | null`\"", reTable.Rows[0][1])
	}
}

// TestFormatPipeTable_OddBacktickCount_StillEscapesRoundTrips es el
// hallazgo de una revisión sobre el commit anterior: una celda con una
// cantidad IMPAR de backticks ("use ` for code | see docs", un solo
// backtick suelto) no tiene ningún code span real, pero el rastreo ciego de
// backticks dejaba el toggle "dentro de código" prendido para el resto de
// la celda — el "|" real que sigue quedaba sin escapar y el reparse lo leía
// como un separador de columna de más.
func TestFormatPipeTable_OddBacktickCount_StillEscapesRoundTrips(t *testing.T) {
	table := ast.NewTableElement(diagnostics.NewPosition(3, 1))
	table.Headers = []string{"A", "B"}
	table.Rows = [][]string{{"use ` for code | see docs", "z"}}

	out, err := FormatStrict(chartDoc(table))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}

	reparsed, diags := parser.New(util.NewNoop()).Parse(out, "test.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("reparse produjo un error: %v\n%s", d, out)
		}
	}
	var reTable *ast.TableElement
	for _, block := range reparsed.ContentBlocks {
		for _, el := range block.Elements {
			if tb, ok := el.(*ast.TableElement); ok {
				reTable = tb
			}
		}
	}
	if reTable == nil {
		t.Fatalf("no se encontró el TableElement reparseado:\n%s", out)
	}
	if len(reTable.Rows) != 1 || len(reTable.Rows[0]) != 2 {
		t.Fatalf("Rows reparseado = %#v, want 1 fila de 2 celdas:\n%s", reTable.Rows, out)
	}
	if reTable.Rows[0][0] != "use ` for code | see docs" {
		t.Errorf("Rows[0][0] reparseado = %q, want \"use ` for code | see docs\"", reTable.Rows[0][0])
	}
}
