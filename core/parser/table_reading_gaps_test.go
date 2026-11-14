// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Los huecos de lectura de tablas que quedaban después de #531 (v2.59.0).
// Cada prueba recorre todos los contextos de sepContexts: flex, ":::", el
// cuerpo strict, el callout tipado y la columna tipada.

// Un separador sin pipe inicial ("--- | ---"), justo después del encabezado,
// es el separador en todos los contextos. El cuerpo strict y la columna
// tipada ya lo leían así; en flex, en un ":::" y en un callout tipado cortaba
// la tabla, la línea quedaba como párrafo y las filas de abajo eran otra
// tabla. Sin "---" (":-- | --:", "--|--") es separador en la tabla de flex y
// de ":::", que revisa que las filas de abajo quepan; el cuerpo strict no lo
// revisa, y ahí sigue cortando la tabla, como antes.
func TestMarkdownTableBareSeparator(t *testing.T) {
	for _, tc := range []struct {
		name       string
		lines      []string
		want       wantTable
		sharedOnly bool
	}{
		{"dashes", []string{"| A | B |", "--- | ---", "| 1 | 2 |", "| 3 | 4 |"}, wantTable{headers: []string{"A", "B"}, rows: [][]string{{"1", "2"}, {"3", "4"}}}, false},
		{"closing pipe only", []string{"| A | B |", "--- | --- |", "| 1 | 2 |"}, wantTable{headers: []string{"A", "B"}, rows: [][]string{{"1", "2"}}}, false},
		{"no rows", []string{"| A | B |", "--- | ---"}, wantTable{headers: []string{"A", "B"}}, false},
		{"colons", []string{"| A | B |", ":-- | --:", "| 1 | 2 |"}, wantTable{headers: []string{"A", "B"}, rows: [][]string{{"1", "2"}}, align: []string{"left", "right"}}, true},
		{"short dashes", []string{"| A | B |", "--|--", "| 1 | 2 |"}, wantTable{headers: []string{"A", "B"}, rows: [][]string{{"1", "2"}}}, true},
	} {
		for _, c := range sepContexts() {
			if tc.sharedOnly && (c.pipeless || strings.Contains(c.name, "typed column")) {
				continue
			}
			t.Run(c.name+"/"+tc.name, func(t *testing.T) { c.check(t, tc.lines, tc.want) })
		}
	}
}

// Si las filas de abajo no caben en el encabezado, el separador sin pipe
// inicial no une la tabla: se lee como antes (dos tablas y un párrafo en
// flex), porque esas filas darían TABLE003, que es un error, en un documento
// que compilaba. Una línea de guiones sin "|" ("---") después del encabezado
// tampoco cambia: en flex no es parte de la tabla.
func TestMarkdownTableBareSeparatorKeepsTheOldReading(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
	}{
		{"wider rows below", []string{"| A | B |", "--- | ---", "| 1 | 2 | 3 |", "| 4 | 5 | 6 |"}},
		{"one cell per column is required", []string{"| A | B | C |", "--- | ---", "| 1 | 2 | 3 |"}},
		{"a line of dashes alone", []string{"| A | B |", "---", "| 1 | 2 |"}},
	} {
		for _, c := range sepContexts() {
			if c.pipeless || strings.Contains(c.name, "typed column") {
				// El cuerpo strict lee su propia tabla (ver
				// parseMarkdownTableElement); la columna tipada da un error
				// por la línea que no es un elemento, como antes.
				continue
			}
			if tc.name == "a line of dashes alone" && strings.Contains(c.name, "strict") {
				// En un ":::" de un documento strict, "---" cierra el bloque.
				continue
			}
			t.Run(c.name+"/"+tc.name, func(t *testing.T) {
				src := c.build(tc.lines)
				tables, diags := c.parseTables(src)
				if len(tables) != 2 || len(tables[0].Rows) != 0 || !reflect.DeepEqual(tables[1].Headers, []string{"1", "2", "3"}[:len(tables[1].Headers)]) {
					var got []string
					for _, tb := range tables {
						got = append(got, fmt.Sprintf("%q %q", tb.Headers, tb.Rows))
					}
					t.Fatalf("want the line to cut the table, got %v\n%s", got, src)
				}
				for _, d := range diags {
					if d.RuleID == "TABLE008" || d.IsError() {
						t.Fatalf("unexpected diagnostic %v\n%s", d, src)
					}
				}
			})
		}
	}
}

// Una fila de solo guiones que no sigue al encabezado es una fila de datos si
// cabe, en los dos dialectos. Flex y ":::" la saltaban como separador sin
// aviso; strict ya la leía como datos. Lo mismo un segundo separador
// seguido del primero.
func TestMarkdownTableDashOnlyRowBelowTheSeparator(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		want  wantTable
	}{
		{"one cell per column", []string{"| A | B |", "|---|---|", "| 1 | 2 |", "| --- | --- |", "| :-- | --: |", "| 3 | 4 |"},
			wantTable{headers: []string{"A", "B"}, rows: [][]string{{"1", "2"}, {"---", "---"}, {":--", "--:"}, {"3", "4"}}}},
		{"a second separator", []string{"| A | B |", "|---|---|", "|---|---|", "| 1 | 2 |"},
			wantTable{headers: []string{"A", "B"}, rows: [][]string{{"---", "---"}, {"1", "2"}}}},
		{"one column", []string{"| A |", "| --- |", "| 1 |", "| --- |"},
			wantTable{headers: []string{"A"}, rows: [][]string{{"1"}, {"---"}}}},
	} {
		for _, c := range sepContexts() {
			t.Run(c.name+"/"+tc.name, func(t *testing.T) { c.check(t, tc.lines, tc.want) })
		}
	}
}

// Una fila de solo guiones a media tabla que no cabe se salta, como antes en
// flex y en ":::", pero ahora con un TABLE008 que la nombra. En el cuerpo
// strict y en la columna tipada es una fila de datos, como siempre (con su
// TABLE003): cambiarla a saltada cambiaría el AST de un documento con
// TABLE003 apagado por la política.
func TestMarkdownTableDashOnlyRowThatDoesNotFit(t *testing.T) {
	lines := []string{"| A | B |", "|---|---|", "| 1 | 2 |", "| --- |", "| 3 | 4 |"}
	for _, c := range sepContexts() {
		t.Run(c.name, func(t *testing.T) {
			src := c.build(lines)
			tables, diags := c.parseTables(src)
			if len(tables) != 1 {
				t.Fatalf("%d tables\n%s", len(tables), src)
			}
			rows := tables[0].Rows
			if c.pipeless || strings.Contains(c.name, "typed column") {
				if want := [][]string{{"1", "2"}, {"---"}, {"3", "4"}}; !reflect.DeepEqual(rows, want) {
					t.Fatalf("rows %q, want %q\n%s", rows, want, src)
				}
				return
			}
			if want := [][]string{{"1", "2"}, {"3", "4"}}; !reflect.DeepEqual(rows, want) {
				t.Fatalf("rows %q, want %q\n%s", rows, want, src)
			}
			for _, d := range diags {
				if d.RuleID == "TABLE008" && d.IsWarning() && strings.Contains(d.Message, `"| --- |"`) {
					return
				}
			}
			t.Fatalf("no TABLE008 naming the row: %v\n%s", diags, src)
		})
	}
}

// Un separador de guiones cortos ("|--|--|", GFM acepta uno o más guiones por
// celda) justo después del encabezado y con una celda por columna es el
// separador en los dos dialectos; antes era la primera fila de datos. En
// cualquier otra fila, o con otro número de celdas, sigue siendo de datos, y
// una celda de un solo guion ("| - | - |") también: un "-" suele marcar una
// celda vacía.
func TestMarkdownTableShortDashSeparator(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		want  wantTable
	}{
		{"two dashes", []string{"| A | B |", "|--|--|", "| 1 | 2 |"}, wantTable{headers: []string{"A", "B"}, rows: [][]string{{"1", "2"}}}},
		{"spaced", []string{"| A | B |", "| -- | -- |", "| 1 | 2 |"}, wantTable{headers: []string{"A", "B"}, rows: [][]string{{"1", "2"}}}},
		{"aligned", []string{"| A | B | C |", "|:--|--:|:--:|", "| 1 | 2 | 3 |"}, wantTable{headers: []string{"A", "B", "C"}, rows: [][]string{{"1", "2", "3"}}, align: []string{"left", "right", "center"}}},
		{"one dash stays data", []string{"| A | B |", "| - | - |", "| 1 | 2 |"}, wantTable{headers: []string{"A", "B"}, rows: [][]string{{"-", "-"}, {"1", "2"}}}},
		{"another number of cells stays data", []string{"| A | B |", "|--|", "| 1 | 2 |"}, wantTable{headers: []string{"A", "B"}, rows: [][]string{{"--"}, {"1", "2"}}}},
		{"below the separator stays data", []string{"| A | B |", "|---|---|", "|--|--|", "| 1 | 2 |"}, wantTable{headers: []string{"A", "B"}, rows: [][]string{{"--", "--"}, {"1", "2"}}}},
	} {
		for _, c := range sepContexts() {
			t.Run(c.name+"/"+tc.name, func(t *testing.T) { c.check(t, tc.lines, tc.want) })
		}
	}
}

// Una fila sin celdas de cells: ("- []") en una tabla de ancho cero es [] en
// el AST, como la fila "|" de una tabla pipe o la fila [] bajo rows:. Antes
// salía null.
func TestTableCellsEmptyRowInZeroWidthTable(t *testing.T) {
	for _, c := range sepContexts() {
		t.Run(c.name, func(t *testing.T) {
			src := c.build([]string{"TABLE", "  cells:", "    - []", "    - []"})
			tables, diags := c.parseTables(src)
			if len(tables) != 1 {
				t.Fatalf("%d tables\n%s", len(tables), src)
			}
			b, err := json.Marshal(tables[0].Rows)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != "[[],[]]" {
				t.Fatalf("rows %s, want [[],[]]\n%s", b, src)
			}
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("unexpected error %v\n%s", d, src)
				}
			}
		})
	}
}

// checkRows lee la única tabla de lines en el contexto c y compara sus filas;
// warn, si no está vacío, es un fragmento de un TABLE008 que tiene que estar.
func checkRows(t *testing.T, c sepContext, lines []string, wantHeaders []string, wantRows [][]string, warn string) {
	t.Helper()
	src := c.build(lines)
	tables, diags := c.parseTables(src)
	if len(tables) != 1 {
		t.Fatalf("%d tables\n%s", len(tables), src)
	}
	got := tables[0]
	if !reflect.DeepEqual(got.Headers, wantHeaders) || !reflect.DeepEqual(got.Rows, wantRows) {
		t.Fatalf("got headers=%q rows=%q, want headers=%q rows=%q\n%s", got.Headers, got.Rows, wantHeaders, wantRows, src)
	}
	found := warn == ""
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error %v\n%s", d, src)
		}
		if d.RuleID == "TABLE008" || d.RuleID == "TABLE007" {
			if warn != "" && d.RuleID == "TABLE008" && d.IsWarning() && strings.Contains(d.Message, warn) {
				found = true
				continue
			}
			t.Fatalf("unexpected diagnostic %v\n%s", d, src)
		}
	}
	if !found {
		t.Fatalf("no TABLE008 with %q: %v\n%s", warn, diags, src)
	}
}

// Una fila bajo rows: se lee a cualquier sangría mayor que la de rows:, con
// todas las filas del bloque a la sangría de la primera. Hasta v2.59.0 solo
// se leía una fila que empezara exactamente a seis espacios del comienzo de
// la línea, y cualquier otra se perdía con TABLE008.
func TestTableRowsAtAnyIndentation(t *testing.T) {
	ab := []string{"A", "B"}
	for _, tc := range []struct {
		name  string
		lines []string
		rows  [][]string
		warn  string
	}{
		{"one space more than rows:", []string{"TABLE", "  headers: [\"A\", \"B\"]", "  rows:", "   [\"1\", \"2\"]", "   [\"3\", \"4\"]"}, [][]string{{"1", "2"}, {"3", "4"}}, ""},
		{"eight spaces more than rows:", []string{"TABLE", "  headers: [\"A\", \"B\"]", "  rows:", "          [\"1\", \"2\"]"}, [][]string{{"1", "2"}}, ""},
		{"properties at four", []string{"TABLE", "    headers: [\"A\", \"B\"]", "    rows:", "      [\"1\", \"2\"]", "      []"}, [][]string{{"1", "2"}, {}}, ""},
		{"a row at another indentation", []string{"TABLE", "  headers: [\"A\", \"B\"]", "  rows:", "     [\"1\", \"2\"]", "       [\"3\", \"4\"]", "     [\"5\", \"6\"]"}, [][]string{{"1", "2"}, {"5", "6"}}, `indented differently from the first row, so it was ignored: "[\"3\", \"4\"]"`},
		{"a row that does not fit", []string{"TABLE", "  headers: [\"A\", \"B\"]", "  rows:", "     [\"1\", \"2\"]", "     [\"3\"]"}, [][]string{{"1", "2"}}, `has 1 cells where the headers have 2, so it was ignored: "[\"3\"]"`},
	} {
		for _, c := range sepContexts() {
			t.Run(c.name+"/"+tc.name, func(t *testing.T) { checkRows(t, c, tc.lines, ab, tc.rows, tc.warn) })
		}
	}
}

// Un rows: [ de varias líneas: con "]" en su propia línea, con "]]" al final
// de la última fila, y con las primeras filas en la línea de rows:. El "]"
// que cierra el arreglo no queda en la última celda (antes salía "4\"]"), y
// las filas de la línea de rows: se leen (antes se perdían con TABLE007).
func TestTableRowsMultiLineArray(t *testing.T) {
	ab := []string{"A", "B"}
	want := [][]string{{"1", "2"}, {"3", "4"}}
	for _, tc := range []struct {
		name  string
		lines []string
	}{
		{"closing line", []string{"TABLE", "  headers: [\"A\", \"B\"]", "  rows: [", "    [\"1\", \"2\"],", "    [\"3\", \"4\"]", "  ]"}},
		{"closed on the last row", []string{"TABLE", "  headers: [\"A\", \"B\"]", "  rows: [", "    [\"1\", \"2\"],", "    [\"3\", \"4\"]]"}},
		{"closed on the last row at six", []string{"TABLE", "  headers: [\"A\", \"B\"]", "  rows: [", "      [\"1\", \"2\"],", "      [\"3\", \"4\"]]"}},
		{"first row on the rows: line", []string{"TABLE", "  headers: [\"A\", \"B\"]", "  rows: [[\"1\", \"2\"],", "         [\"3\", \"4\"]]"}},
	} {
		for _, c := range sepContexts() {
			t.Run(c.name+"/"+tc.name, func(t *testing.T) { checkRows(t, c, tc.lines, ab, want, "") })
		}
	}
}
