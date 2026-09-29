// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// parseTableBlockAt corre TableParser sobre un bloque armado a mano y
// devuelve la tabla resultante junto con el contexto usado.
func parseTableBlockAt(t *testing.T, mode string, lines ...string) (*ast.TableElement, *ParseContext) {
	t.Helper()
	ctx := &ParseContext{Mode: mode, Lines: lines}
	result := (&TableParser{}).Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is %T, want *ast.TableElement", result.Element)
	}
	return table, ctx
}

// TestTableParser_TableBlockPipeRows_EdgePipesDoNotCreateColumns cubre el bug
// de https://github.com/ziradocs/toolchain/issues/379: una fila con pipes de
// borde dentro de un bloque TABLE se partía en una celda vacía al principio
// y otra al final, y la fila delimitadora (|---|) quedaba como fila de datos.
func TestTableParser_TableBlockPipeRows_EdgePipesDoNotCreateColumns(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string
		wantHeaders []string
		wantRows    [][]string
	}{
		{
			name: "edge pipes with delimiter row",
			lines: []string{
				"TABLE",
				"  | Métrica | Q2 | Q3 |",
				"  |---------|----|----|",
				"  | Ingresos | 10 | 12 |",
				"  | Costos | 4 | 5 |",
			},
			wantHeaders: []string{"Métrica", "Q2", "Q3"},
			wantRows:    [][]string{{"Ingresos", "10", "12"}, {"Costos", "4", "5"}},
		},
		{
			name: "edge pipes without delimiter row",
			lines: []string{
				"TABLE",
				"  | Métrica | Q2 | Q3 |",
				"  | Ingresos | 10 | 12 |",
			},
			wantHeaders: []string{"Métrica", "Q2", "Q3"},
			wantRows:    [][]string{{"Ingresos", "10", "12"}},
		},
		{
			name: "aligned delimiter row",
			lines: []string{
				"TABLE",
				"  | A | B | C |",
				"  |:--|:-:|--:|",
				"  | 1 | 2 | 3 |",
			},
			wantHeaders: []string{"A", "B", "C"},
			wantRows:    [][]string{{"1", "2", "3"}},
		},
		{
			name: "no edge pipes",
			lines: []string{
				"TABLE",
				"  Métrica | Q2 | Q3",
				"  Ingresos | 10 | 12",
			},
			wantHeaders: []string{"Métrica", "Q2", "Q3"},
			wantRows:    [][]string{{"Ingresos", "10", "12"}},
		},
		{
			name: "leading pipe only",
			lines: []string{
				"TABLE",
				"  | A | B | C",
				"  | 1 | 2 | 3",
			},
			wantHeaders: []string{"A", "B", "C"},
			wantRows:    [][]string{{"1", "2", "3"}},
		},
		{
			name: "trailing pipe only",
			lines: []string{
				"TABLE",
				"  A | B | C |",
				"  1 | 2 | 3 |",
			},
			wantHeaders: []string{"A", "B", "C"},
			wantRows:    [][]string{{"1", "2", "3"}},
		},
		{
			name: "empty cell in the middle is kept",
			lines: []string{
				"TABLE",
				"  | a | b | c |",
				"  | 1 || 3 |",
			},
			wantHeaders: []string{"a", "b", "c"},
			wantRows:    [][]string{{"1", "", "3"}},
		},
		{
			name: "empty first and last cells are kept, only the edge pipes go",
			lines: []string{
				"TABLE",
				"  | a | b | c |",
				"  |   | 2 |   |",
				"  || 2 ||",
			},
			wantHeaders: []string{"a", "b", "c"},
			wantRows:    [][]string{{"", "2", ""}, {"", "2", ""}},
		},
		{
			name: "empty header cell is kept",
			lines: []string{
				"TABLE",
				"  |  | Q2 | Q3 |",
				"  | Ingresos | 10 | 12 |",
			},
			wantHeaders: []string{"", "Q2", "Q3"},
			wantRows:    [][]string{{"Ingresos", "10", "12"}},
		},
		{
			name: "data cells that merely contain dashes are not a delimiter",
			lines: []string{
				"TABLE",
				"  | Code | Range |",
				"  | a---b | 1-2 |",
				"  | --- | x |",
			},
			wantHeaders: []string{"Code", "Range"},
			wantRows:    [][]string{{"a---b", "1-2"}, {"---", "x"}},
		},
		{
			name: "a later row of dashes is data, only the row after the header is the delimiter",
			lines: []string{
				"TABLE",
				"  | a | b |",
				"  |---|---|",
				"  | 1 | 2 |",
				"  | - | - |",
				"  |---|---|",
			},
			wantHeaders: []string{"a", "b"},
			wantRows:    [][]string{{"1", "2"}, {"-", "-"}, {"---", "---"}},
		},
		{
			name: "dash row right after the header is the delimiter, as in GFM",
			lines: []string{
				"TABLE",
				"  | a | b |",
				"  | - | - |",
				"  | 1 | 2 |",
			},
			wantHeaders: []string{"a", "b"},
			wantRows:    [][]string{{"1", "2"}},
		},
		{
			name: "a delimiter row with the wrong number of cells is data",
			lines: []string{
				"TABLE",
				"  | a | b |",
				"  |---|",
				"  | 1 | 2 |",
			},
			wantHeaders: []string{"a", "b"},
			wantRows:    [][]string{{"---"}, {"1", "2"}},
		},
	}
	for _, mode := range []string{"strict", "flex"} {
		for _, tt := range tests {
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				table, _ := parseTableBlockAt(t, mode, tt.lines...)
				if !reflect.DeepEqual(table.Headers, tt.wantHeaders) {
					t.Errorf("Headers = %#v, want %#v", table.Headers, tt.wantHeaders)
				}
				if !reflect.DeepEqual(table.Rows, tt.wantRows) {
					t.Errorf("Rows = %#v, want %#v", table.Rows, tt.wantRows)
				}
				if len(table.RowPositions) != len(table.Rows) {
					t.Errorf("len(RowPositions) = %d, want %d (one per data row)", len(table.RowPositions), len(table.Rows))
				}
			})
		}
	}
}

// TestTableParser_TableBlockPipeRows_DelimiterRowKeepsPositions verifica que
// saltar la fila delimitadora no desplaza las posiciones de las filas de
// datos: cada RowPosition apunta a su propia línea.
func TestTableParser_TableBlockPipeRows_DelimiterRowKeepsPositions(t *testing.T) {
	table, ctx := parseTableBlockAt(t, "strict",
		"TABLE",
		"  | A | B |",
		"  |---|---|",
		"  | 1 | 2 |",
		"  | 3 | 4 |",
	)
	if len(table.RowPositions) != 2 {
		t.Fatalf("len(RowPositions) = %d, want 2", len(table.RowPositions))
	}
	if table.RowPositions[0] != ctx.Position(3) || table.RowPositions[1] != ctx.Position(4) {
		t.Errorf("RowPositions = %+v, want %+v and %+v", table.RowPositions, ctx.Position(3), ctx.Position(4))
	}
}

// TestTableParser_ParseMarkdownTable_EmptyEdgeCellsSurvive fija que el
// recorte de bordes compartido no cambió la tabla markdown de flex: las
// celdas vacías legítimas siguen ahí.
func TestTableParser_ParseMarkdownTable_EmptyEdgeCellsSurvive(t *testing.T) {
	table, _ := parseTableBlockAt(t, "flex",
		"| a | b | c |",
		"|---|---|---|",
		"|   | 2 |   |",
		"| 1 || 3 |",
	)
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(table.Headers, want) {
		t.Errorf("Headers = %#v, want %#v", table.Headers, want)
	}
	if want := [][]string{{"", "2", ""}, {"1", "", "3"}}; !reflect.DeepEqual(table.Rows, want) {
		t.Errorf("Rows = %#v, want %#v", table.Rows, want)
	}
}

func TestTrimMarkdownRowEdges(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{"both edge pipes", "| a | b |", []string{" a ", " b "}},
		{"no edge pipes", "a | b", []string{"a ", " b"}},
		{"leading only", "| a | b", []string{" a ", " b"}},
		{"trailing only", "a | b |", []string{"a ", " b"}},
		{"surrounding whitespace on the line", "   | a | b |   ", []string{" a ", " b "}},
		{"empty middle cell", "| a || c |", []string{" a ", "", " c "}},
		{"empty first data cell", "|  | b |", []string{"  ", " b "}},
		{"empty last data cell", "| a |  |", []string{" a ", "  "}},
		{"only one cell per edge is trimmed", "|| a ||", []string{"", " a ", ""}},
		{"escaped trailing pipe is content", `| a | b \|`, []string{" a ", " b |"}},
		{"lone pipe", "|", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimMarkdownRowEdges(tt.line, SplitMarkdownTableRow(tt.line))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TrimMarkdownRowEdges(%q) = %#v, want %#v", tt.line, got, tt.want)
			}
		})
	}
}

func TestIsMarkdownSeparatorRow(t *testing.T) {
	tests := []struct {
		name  string
		cells []string
		want  bool
	}{
		{"plain", []string{"---", "---"}, true},
		{"long dashes with spaces", []string{" ------ ", " ---- "}, true},
		{"alignments", []string{":--", ":-:", "--:"}, true},
		{"single dash", []string{"-"}, true},
		{"no cells", nil, false},
		{"colon only", []string{":"}, false},
		{"two colons", []string{"::"}, false},
		{"empty cell", []string{"---", ""}, false},
		{"data", []string{"a---b", "---"}, false},
		{"mixed row", []string{"---", "x"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMarkdownSeparatorRow(tt.cells); got != tt.want {
				t.Errorf("isMarkdownSeparatorRow(%#v) = %v, want %v", tt.cells, got, tt.want)
			}
		})
	}
}
