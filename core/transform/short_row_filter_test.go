// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
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

// spannedTable es una tabla de tres columnas cuya única fila de datos viene
// de un filtro con celdas fusionadas: cells la describe, rows es su vista
// plana tal como la dejó el filtro.
func spannedTable(row []ast.TableCell, flat []string) *ast.AST {
	pos := diagnostics.NewPosition(3, 1)
	table := ast.NewTableElement(pos)
	table.Headers = []string{"A", "B", "C"}
	table.Rows = [][]string{flat}
	table.Cells = [][]ast.TableCell{
		{{Content: "A", IsHeader: true, Scope: "col"}, {Content: "B", IsHeader: true, Scope: "col"}, {Content: "C", IsHeader: true, Scope: "col"}},
		row,
	}
	table.RowPositions = []diagnostics.Position{diagnostics.NewPosition(5, 1)}
	block := ast.NewContentBlock(diagnostics.NewPosition(1, 1), "content")
	block.Elements = []ast.Element{table}
	doc := ast.NewAST(diagnostics.NewPosition(1, 1))
	doc.ContentBlocks = []ast.ContentBlock{*block}
	return doc
}

func decodeTable(t *testing.T, doc *ast.AST) (*ast.AST, *ast.TableElement) {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ast.DecodeAST(raw)
	if err != nil {
		t.Fatal(err)
	}
	return decoded, decoded.ContentBlocks[0].Elements[0].(*ast.TableElement)
}

func table010(doc *ast.AST) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	for _, d := range linter.New().LintUnfiltered(doc) {
		if d.RuleID == "TABLE010" || d.RuleID == "TABLE003" {
			out = append(out, d)
		}
	}
	return out
}

// Una fila con colSpan cubre las columnas que suman sus spans: un filtro que
// deja [{M, colSpan: 3}] en una tabla de tres columnas no tiene fila corta.
// DecodeAST no le agrega celdas (el HTML sale con una sola celda de tres
// columnas, no con cinco) ni la avisa, y rows queda como la vista plana de
// esa celda. Una fila que cubre dos de tres columnas con una celda de
// colSpan: 2 recibe solo la que falta, con TABLE010.
func TestDecodeASTMeasuresRowsBySpans(t *testing.T) {
	t.Run("colSpan covers the row", func(t *testing.T) {
		doc, table := decodeTable(t, spannedTable([]ast.TableCell{{Content: "M", ColSpan: 3}}, []string{"M"}))
		if len(table.Cells[1]) != 1 || len(table.DecodePaddedRows()) != 0 {
			t.Fatalf("cells %+v padded %v, want the row untouched", table.Cells[1], table.DecodePaddedRows())
		}
		if !reflect.DeepEqual(table.Rows, [][]string{{"M", "M", "M"}}) {
			t.Fatalf("rows %q, want the flat view of the merged cell", table.Rows)
		}
		if d := table010(doc); len(d) != 0 {
			t.Fatalf("diagnostics for a complete row: %v", d)
		}
		html := renderer.GenerateDocumentHTML(doc, renderer.DocumentHTMLOptions{Title: "T"}, nil)
		if !strings.Contains(html, `<td colspan="3">M</td></tr>`) {
			t.Fatalf("HTML does not carry one three-column cell:\n%s", regexp.MustCompile(`(?s)<table.*?</table>`).FindString(html))
		}
	})
	t.Run("colSpan 2 and a missing column", func(t *testing.T) {
		doc, table := decodeTable(t, spannedTable([]ast.TableCell{{Content: "X", ColSpan: 2}}, []string{"X"}))
		if want := []ast.TableCell{{Content: "X", ColSpan: 2}, {}}; !reflect.DeepEqual(table.Cells[1], want) {
			t.Fatalf("cells %+v, want %+v", table.Cells[1], want)
		}
		if !reflect.DeepEqual(table.Rows, [][]string{{"X", "X", ""}}) {
			t.Fatalf("rows %q", table.Rows)
		}
		d := table010(doc)
		if len(d) != 1 || d[0].RuleID != "TABLE010" || d[0].Position.Line != 5 || !strings.Contains(d[0].Message, "has 2 cells where the header has 3") {
			t.Fatalf("want one TABLE010 on line 5, got %v", d)
		}
	})
}

// El aviso de una fila que completó el decode de un filtro sobrevive a los
// filtros que vienen después, que la reciben ya completa: con el filtro que
// recorta solo, primero o último, hay un TABLE010 en la línea de la fila.
func TestTABLE010SurvivesAFilterChain(t *testing.T) {
	if _, err := exec.LookPath("sed"); err != nil {
		t.Skip("sed no disponible en PATH")
	}
	trim := writeFilter(t, `sed -e 's/\["4","5","6"\]/["4"]/' -e 's/{"content":"4"},{"content":"5"},{"content":"6"}/{"content":"4"}/'`)
	identity := writeFilter(t, "cat")
	src := "# D\n\n| A | B | C |\n|---|---|---|\n| 1 | 2 | 3 |\n| 4 | 5 | 6 |\n"
	for name, chain := range map[string][]string{
		"trim":             {trim},
		"trim, identity":   {trim, identity},
		"identity, trim":   {identity, trim},
		"trim, identity×2": {trim, identity, identity},
	} {
		t.Run(name, func(t *testing.T) {
			doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "d.doclang")
			out, err := RunFilters(doc, chain, 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			d := table010(out)
			if len(d) != 1 || d[0].RuleID != "TABLE010" || d[0].Position.Line != 6 {
				t.Fatalf("want one TABLE010 on line 6, got %v", d)
			}
		})
	}
}
