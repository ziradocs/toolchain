// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

func strictDocTable(t *testing.T, body string) (*ast.TableElement, []diagnostics.Diagnostic) {
	t.Helper()
	src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  TABLE\n    headers: [\"x\", \"y\"]\n" + body
	doc, diags := New(util.NewNoop()).ParseDocument(src, "t.doclang")
	for _, el := range doc.ContentBlocks[0].Elements {
		if table, ok := el.(*ast.TableElement); ok {
			return table, diags
		}
	}
	t.Fatalf("no table element; diagnostics %v", diags)
	return nil, nil
}

// Issue #488: `rows:` con el arreglo entero en una sola línea da las mismas
// filas (y las mismas celdas) que la forma de una fila por línea.
func TestTableInlineRows_SameRowsAsMultiLine(t *testing.T) {
	for _, tc := range []struct {
		name, inline, multi string
	}{
		{"two rows", `    rows: [["a","b"],["c","d"]]` + "\n", "    rows: [\n      [\"a\",\"b\"],\n      [\"c\",\"d\"]\n    ]\n"},
		{"one row", `    rows: [["a","b"]]` + "\n", "    rows: [\n      [\"a\",\"b\"]\n    ]\n"},
		{"spacing", `    rows: [ ["a", "b"] , ["c", "d"] ]` + "\n", "    rows: [\n      [\"a\", \"b\"],\n      [\"c\", \"d\"]\n    ]\n"},
		{"commas and brackets inside quotes", `    rows: [["a, b","[x]"],["c\","d"]]` + "\n", "    rows: [\n      [\"a, b\",\"[x]\"],\n      [\"c\\\",\"d\"]\n    ]\n"},
		{"unquoted cells", `    rows: [[a, b], [c, d]]` + "\n", "    rows: [\n      [a, b],\n      [c, d]\n    ]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inline, diags := strictDocTable(t, tc.inline)
			if len(diags) != 0 {
				t.Fatalf("inline: want no diagnostics, got %v", diags)
			}
			multi, diags := strictDocTable(t, tc.multi)
			if len(diags) != 0 {
				t.Fatalf("multi-line: want no diagnostics, got %v", diags)
			}
			if len(inline.Rows) == 0 {
				t.Fatal("inline rows were dropped")
			}
			if !reflect.DeepEqual(inline.Rows, multi.Rows) || !reflect.DeepEqual(inline.Cells, multi.Cells) || !reflect.DeepEqual(inline.Headers, multi.Headers) {
				t.Errorf("inline and multi-line differ:\ninline rows %q\nmulti rows  %q", inline.Rows, multi.Rows)
			}
			for _, pos := range inline.RowPositions {
				if pos.Line != 9 {
					t.Errorf("an inline row is positioned at line %d, want the rows: line 9", pos.Line)
				}
			}
		})
	}
}

func TestTableInlineRows_TheIssueRepro(t *testing.T) {
	table, diags := strictDocTable(t, `    rows: [["a","b"],["c","d"]]`+"\n")
	if len(diags) != 0 {
		t.Fatalf("want no diagnostics, got %v", diags)
	}
	if want := [][]string{{"a", "b"}, {"c", "d"}}; !reflect.DeepEqual(table.Rows, want) {
		t.Errorf("rows %q, want %q", table.Rows, want)
	}
}

// `rows: []` sigue siendo una tabla sin filas declarada a propósito: no hay
// nada que no se haya podido leer, así que no hay TABLE007 (el linter sigue
// dando TABLE002).
func TestTableInlineRows_EmptyArrayIsNoRows(t *testing.T) {
	table, diags := strictDocTable(t, "    rows: []\n")
	if len(diags) != 0 || len(table.Rows) != 0 {
		t.Errorf("want zero rows and no diagnostics, got %q %v", table.Rows, diags)
	}
}

// Un valor en línea que no se puede leer es un error en strict, con su línea
// y el código TABLE007, en vez de una tabla vacía con solo el TABLE002.
func TestTableInlineRows_MalformedIsAnErrorInStrict(t *testing.T) {
	for name, body := range map[string]string{
		"unclosed":                       `    rows: [["a","b"],["c","d"]` + "\n",
		"open row":                       `    rows: [["a","b"],["c"` + "\n",
		"not an array":                   "    rows: a, b\n",
		"bare cells":                     `    rows: ["a","b"]` + "\n",
		"missing comma":                  `    rows: [["a","b"] ["c","d"]]` + "\n",
		"trailing comma":                 `    rows: [["a","b"],]` + "\n",
		"trailing comma after the array": `    rows: [["a","b"]],` + "\n",
		"inline plus a line":             `    rows: [["a","b"]]` + "\n" + `      ["c","d"]` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			table, diags := strictDocTable(t, body)
			if len(diags) != 1 || !diags[0].IsError() || diags[0].RuleID != "TABLE007" || diags[0].Position.Line != 9 {
				t.Fatalf("want one TABLE007 error at line 9, got %v", diags)
			}
			if !strings.Contains(diags[0].Message, "rows:") {
				t.Errorf("the message should name rows:, got %q", diags[0].Message)
			}
			if len(table.Rows) != 0 {
				t.Errorf("a malformed value must not produce partial rows, got %q", table.Rows)
			}
		})
	}
}

// Los dos dialectos comparten el parser de TABLE: en un slide strict también
// se leen las filas en línea, y fuera de strict el valor ilegible es un warning.
func TestTableInlineRows_OtherDialects(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TABLE\n    headers: [\"x\", \"y\"]\n    rows: [[\"a\",\"b\"],[\"c\",\"d\"]]\n"
	doc, diags := New(util.NewNoop()).Parse(src, "t.slidelang")
	if len(diags) != 0 {
		t.Fatalf("slide: want no diagnostics, got %v", diags)
	}
	table := doc.ContentBlocks[0].Elements[0].(*ast.TableElement)
	if want := [][]string{{"a", "b"}, {"c", "d"}}; !reflect.DeepEqual(table.Rows, want) {
		t.Errorf("slide rows %q, want %q", table.Rows, want)
	}

	src = "# A\n\nTABLE\n  headers: [\"x\", \"y\"]\n  rows: [[\"a\",\"b\"],[\"c\",\"d\"]]\n"
	doc, diags = New(util.NewNoop()).ParseDocument(src, "t.doclang")
	if len(diags) != 0 {
		t.Fatalf("flex: want no diagnostics, got %v", diags)
	}
	var flexTable *ast.TableElement
	for _, el := range doc.ContentBlocks[0].Elements {
		if tb, ok := el.(*ast.TableElement); ok {
			flexTable = tb
		}
	}
	if flexTable == nil || !reflect.DeepEqual(flexTable.Rows, [][]string{{"a", "b"}, {"c", "d"}}) {
		t.Fatalf("flex rows: got %#v", flexTable)
	}

	src = "# A\n\nTABLE\n  headers: [\"x\", \"y\"]\n  rows: [[\"a\",\"b\"],\n"
	_, diags = New(util.NewNoop()).ParseDocument(src, "t.doclang")
	if len(diags) != 1 || diags[0].IsError() || diags[0].RuleID != "TABLE007" || diags[0].Position.Line != 5 {
		t.Errorf("flex: want one TABLE007 warning at line 5, got %v", diags)
	}
}

// Las filas de la forma de varias líneas se leen con cualquier sangría que no
// sea menor que la de las propiedades del TABLE, y una línea en blanco entre
// filas no corta la lista. Antes solo contaba una línea con seis espacios y
// "[" exactos, y el resto se descartaba sin aviso.
func TestTableRows_MultiLineAnyIndentAndBlankLines(t *testing.T) {
	want := [][]string{{"a", "b"}, {"c", "d"}}
	for name, body := range map[string]string{
		"deeper rows":            "    rows: [\n        [\"a\",\"b\"],\n        [\"c\",\"d\"]\n    ]\n",
		"rows at property depth": "    rows: [\n    [\"a\",\"b\"],\n    [\"c\",\"d\"]\n    ]\n",
		"blank line between":     "    rows: [\n      [\"a\",\"b\"],\n\n      [\"c\",\"d\"]\n    ]\n",
		"bare rows:":             "    rows:\n      [\"a\",\"b\"]\n      [\"c\",\"d\"]\n",
	} {
		t.Run(name, func(t *testing.T) {
			table, diags := strictDocTable(t, body+"    caption: \"Cap\"\n")
			if len(diags) != 0 {
				t.Fatalf("want no diagnostics, got %v", diags)
			}
			if !reflect.DeepEqual(table.Rows, want) {
				t.Errorf("rows %q, want %q", table.Rows, want)
			}
			if table.Caption != "Cap" {
				t.Errorf("the property after the rows was lost: caption %q", table.Caption)
			}
		})
	}
}

// Un TABLE dentro de una columna tipada lleva sus filas más sangradas que
// seis espacios: también se leen.
func TestTableRows_InsideATypedColumn(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  <<grid>>\n  <<column typed>>\n    TABLE\n      headers: [\"x\", \"y\"]\n      rows: [\n        [\"a\",\"b\"]\n      ]\n  <<end>>\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "t.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error: %v", d)
		}
	}
	var table *ast.TableElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if tb, ok := n.(*ast.TableElement); ok {
			table = tb
		}
		return nil
	})
	if table == nil || !reflect.DeepEqual(table.Rows, [][]string{{"a", "b"}}) {
		t.Fatalf("want the row inside the column, got %#v", table)
	}
}

// Lo que está entre las filas y no se puede leer como fila es TABLE007 en su
// línea (error en strict, warning en flex), no un descarte callado. Las filas
// buenas de alrededor se conservan.
func TestTableRows_UnreadableLineIsTABLE007(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		line int
	}{
		"row not closed":      {"    rows: [\n      [\"a\",\"b\"],\n      [\"c\",\"d\"\n    ]\n", 11},
		"list marker":         {"    rows:\n      [\"a\",\"b\"]\n      - [\"c\",\"d\"]\n", 11},
		"row after a comment": {"    rows:\n      [\"a\",\"b\"]\n      // comentario\n      [\"c\",\"d\"]\n", 12},
	} {
		t.Run(name, func(t *testing.T) {
			table, diags := strictDocTable(t, tc.body)
			if len(diags) != 1 || !diags[0].IsError() || diags[0].RuleID != "TABLE007" || diags[0].Position.Line != tc.line {
				t.Fatalf("want one TABLE007 error at line %d, got %v", tc.line, diags)
			}
			if !reflect.DeepEqual(table.Rows, [][]string{{"a", "b"}}) {
				t.Errorf("the readable row must stay, got %q", table.Rows)
			}
		})
	}

	src := "# A\n\nTABLE\n  headers: [\"x\", \"y\"]\n  rows: [\n    [\"a\",\"b\"\n  ]\n"
	_, diags := New(util.NewNoop()).ParseDocument(src, "t.doclang")
	if len(diags) != 1 || diags[0].IsError() || diags[0].RuleID != "TABLE007" || diags[0].Position.Line != 6 {
		t.Errorf("flex: want one TABLE007 warning at line 6, got %v", diags)
	}
}

// Una fila debajo de un `rows:` en línea, también con una línea en blanco de
// por medio, es TABLE007.
func TestTableInlineRows_RowAfterABlankLine(t *testing.T) {
	table, diags := strictDocTable(t, "    rows: [[\"a\",\"b\"]]\n\n      [\"c\",\"d\"]\n")
	if len(diags) != 1 || !diags[0].IsError() || diags[0].RuleID != "TABLE007" || !strings.Contains(diags[0].Message, "line 11") {
		t.Fatalf("want one TABLE007 error naming line 11, got %v", diags)
	}
	if len(table.Rows) != 0 {
		t.Errorf("want no partial rows, got %q", table.Rows)
	}
}

// Las dos formas leen igual las mismas celdas también cuando las comillas no
// cierran: el backslash no es escape, y `"c\"d"` es la celda `c\"d` (el corte
// por comas de siempre).
func TestTableInlineRows_OddQuotesReadLikeMultiLine(t *testing.T) {
	inline, diags := strictDocTable(t, `    rows: [["c\"d", "e"]]`+"\n")
	if len(diags) != 0 {
		t.Fatalf("inline: want no diagnostics, got %v", diags)
	}
	multi, diags := strictDocTable(t, "    rows:\n      [\"c\\\"d\", \"e\"]\n")
	if len(diags) != 0 {
		t.Fatalf("multi-line: want no diagnostics, got %v", diags)
	}
	if want := [][]string{{`c\"d`, "e"}}; !reflect.DeepEqual(inline.Rows, want) || !reflect.DeepEqual(multi.Rows, want) {
		t.Errorf("inline %q, multi-line %q, want %q", inline.Rows, multi.Rows, want)
	}
}

func TestTableInlineRows_TrailingCommaMessage(t *testing.T) {
	_, diags := strictDocTable(t, `    rows: [["a"]],`+"\n")
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "a trailing comma after the closing ]") {
		t.Errorf("want the trailing-comma message, got %v", diags)
	}
}

// Una línea que no es fila (una propiedad que TABLE no conoce, un texto sin
// corchetes) cierra la lista, como antes de `rows:`, sin TABLE007: un deck
// que compilaba no se rompe.
func TestTableRows_OtherLinesCloseTheList(t *testing.T) {
	for name, body := range map[string]string{
		"unknown property": "    rows:\n      [\"a\", \"b\"]\n    align: center\n    caption: \"C\"\n",
		"no brackets":      "    rows:\n      [\"a\", \"b\"]\n      \"c\",\"d\"\n    caption: \"C\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			table, diags := strictDocTable(t, body)
			if len(diags) != 0 {
				t.Fatalf("want no diagnostics, got %v", diags)
			}
			if !reflect.DeepEqual(table.Rows, [][]string{{"a", "b"}}) || table.Caption != "C" {
				t.Errorf("rows %q caption %q", table.Rows, table.Caption)
			}
		})
	}
}

// En un slide (strict o flex) TABLE007 es warning: el deck sigue compilando.
// Solo el documento strict lo sube a error.
func TestTableRows_TABLE007IsAWarningInSlides(t *testing.T) {
	for name, tc := range map[string]struct {
		src  string
		line int
	}{
		"strict inline":    {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TABLE\n    headers: [\"x\", \"y\"]\n    rows: [[\"a\",\"b\"]\n", 9},
		"strict multiline": {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TABLE\n    headers: [\"x\", \"y\"]\n    rows:\n      [\"a\",\"b\"\n", 10},
		"flex":             {"---\nmode: flex\ntitle: T\n---\n\n# A\n\nTABLE\n  headers: [\"x\", \"y\"]\n  rows: [[\"a\",\"b\"]\n", 10},
	} {
		t.Run(name, func(t *testing.T) {
			_, diags := New(util.NewNoop()).Parse(tc.src, "t.slidelang")
			var got []diagnostics.Diagnostic
			for _, d := range diags {
				if d.RuleID == "TABLE007" {
					got = append(got, d)
				}
			}
			if len(got) != 1 || got[0].IsError() || got[0].Position.Line != tc.line {
				t.Errorf("want one TABLE007 warning at line %d, got %v", tc.line, diags)
			}
		})
	}

	// st2/t5: una propiedad desconocida después de las filas no da TABLE007.
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TABLE\n    headers: [\"x\", \"y\"]\n    rows:\n      [\"a\", \"b\"]\n    align: center\n"
	doc, diags := New(util.NewNoop()).Parse(src, "t.slidelang")
	for _, d := range diags {
		if d.RuleID == "TABLE007" || d.IsError() {
			t.Errorf("unexpected diagnostic: %v", d)
		}
	}
	if table := doc.ContentBlocks[0].Elements[0].(*ast.TableElement); !reflect.DeepEqual(table.Rows, [][]string{{"a", "b"}}) {
		t.Errorf("rows %q", table.Rows)
	}
}

// tablesIn parsea src en el dialecto que diga su frontmatter y devuelve sus
// tablas y los TABLE007.
func tablesIn(t *testing.T, src string, document bool) ([]*ast.TableElement, []diagnostics.Diagnostic) {
	t.Helper()
	p := New(util.NewNoop())
	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if document {
		doc, diags = p.ParseDocument(src, "t.doclang")
	} else {
		doc, diags = p.Parse(src, "t.slidelang")
	}
	var tables []*ast.TableElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if tb, ok := n.(*ast.TableElement); ok {
			tables = append(tables, tb)
		}
		return nil
	})
	var t7 []diagnostics.Diagnostic
	for _, d := range diags {
		if d.RuleID == "TABLE007" {
			t7 = append(t7, d)
		}
	}
	return tables, t7
}

// Una fila con "|" cuya primera celda empieza con "[" (un link, una casilla,
// una cita) es una fila Markdown, no una fila de corchetes: no da TABLE007 y
// no se pierde. Igual en los tres dialectos y también después de `rows:`.
func TestTableRows_PipeRowsStartingWithABracket(t *testing.T) {
	bodies := map[string]struct {
		body    string
		headers []string
		rows    [][]string
	}{
		"link and checkbox": {"Nombre | Valor\n[Docs](https://x.y) | 3\n[x] listo | 4\n[ ] tarea | 5\n", []string{"Nombre", "Valor"}, [][]string{{"[Docs](https://x.y)", "3"}, {"[x] listo", "4"}, {"[ ] tarea", "5"}}},
		"bracket header":    {"[Nombre] | Valor\na | b\n", []string{"[Nombre]", "Valor"}, [][]string{{"a", "b"}}},
		"citation":          {"Fuente | Dato\n[1] | 3\n", []string{"Fuente", "Dato"}, [][]string{{"[1]", "3"}}},
		"after rows:":       {"headers: [\"x\", \"y\"]\nrows:\n  [\"a\", \"b\"]\n[x] | y\n", []string{"x", "y"}, [][]string{{"a", "b"}, {"[x]", "y"}}},
		"inline then pipe":  {"headers: [\"x\", \"y\"]\nrows: [[\"a\", \"b\"]]\n[x] listo | y\n", []string{"x", "y"}, [][]string{{"a", "b"}, {"[x] listo", "y"}}},
	}
	indent := func(body, pad string) string {
		var b strings.Builder
		for _, l := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
			b.WriteString(pad + l + "\n")
		}
		return b.String()
	}
	for name, tc := range bodies {
		for _, dialect := range []struct {
			name     string
			src      string
			document bool
		}{
			{"document strict", "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TABLE\n" + indent(tc.body, "    "), true},
			{"slide strict", "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TABLE\n" + indent(tc.body, "    "), false},
			{"document flex", "# A\n\nTABLE\n" + indent(tc.body, "  "), true},
		} {
			t.Run(name+"/"+dialect.name, func(t *testing.T) {
				tables, t7 := tablesIn(t, dialect.src, dialect.document)
				if len(t7) != 0 {
					t.Fatalf("no TABLE007 expected, got %v", t7)
				}
				if len(tables) != 1 || !reflect.DeepEqual(tables[0].Headers, tc.headers) || !reflect.DeepEqual(tables[0].Rows, tc.rows) {
					var got any = tables
					if len(tables) == 1 {
						got = []any{tables[0].Headers, tables[0].Rows}
					}
					t.Errorf("want headers %q rows %q, got %q", tc.headers, tc.rows, got)
				}
			})
		}
	}
}

// En línea y en varias líneas, las mismas filas dan las mismas celdas también
// cuando una fila tiene comillas pares con un corchete adentro y otra impares.
func TestTableInlineRows_MixedQuotesReadLikeMultiLine(t *testing.T) {
	inline, diags := strictDocTable(t, `    rows: [["a]", "b"], ["c\"d", "e"]]`+"\n")
	if len(diags) != 0 {
		t.Fatalf("inline: want no diagnostics, got %v", diags)
	}
	multi, diags := strictDocTable(t, "    rows:\n      [\"a]\", \"b\"]\n      [\"c\\\"d\", \"e\"]\n")
	if len(diags) != 0 {
		t.Fatalf("multi-line: want no diagnostics, got %v", diags)
	}
	want := [][]string{{"a]", "b"}, {`c\"d`, "e"}}
	if !reflect.DeepEqual(inline.Rows, want) || !reflect.DeepEqual(multi.Rows, want) {
		t.Errorf("inline %q, multi-line %q, want %q", inline.Rows, multi.Rows, want)
	}
}

// `headers:` sin valor con el arreglo en la línea de abajo dice que el
// arreglo va en la misma línea.
func TestTable_HeadersArrayOnTheNextLine(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TABLE\n    headers:\n      [\"a\", \"b\"]\n    rows: [[\"1\", \"2\"]]\n"
	_, t7 := tablesIn(t, src, true)
	if len(t7) != 1 || !strings.Contains(t7[0].Message, "headers: takes its array on the same line") || t7[0].Position.Line != 9 {
		t.Errorf("want the headers message at line 9, got %v", t7)
	}
}
