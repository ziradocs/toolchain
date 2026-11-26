// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Una fila con menos celdas que el encabezado se completa con celdas vacías,
// como en GFM, y da TABLE010 (un aviso) en su línea, en vez de TABLE003 (un
// error que hacía fallar el documento entero). Una fila con más celdas sigue
// siendo TABLE003. Vale para todas las formas de tabla, en los dos dialectos
// de los dos lenguajes y dentro de una columna tipada de un grid.
func TestShortTableRowIsPaddedWithAWarning(t *testing.T) {
	pipe := "| A | B | C |\n|---|---|---|\n| 1 | 2 | 3 |\n| 4 | 5 |\n| 6 |\n"
	for _, tc := range []struct {
		name     string
		src      string
		document bool
	}{
		{"doclang without frontmatter", "# Doc\n\n" + pipe, true},
		{"doclang flex", "---\nmode: flex\ntitle: \"T\"\n---\n\n# Doc\n\n" + pipe, true},
		{"doclang strict", "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n" + indentLines(pipe, 2), true},
		{"doclang strict typed column", "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n  <<grid>>\n  <<column typed>>\n" + indentLines(pipe, 4) + "  <<end>>\n", true},
		{"doclang :::note", "---\nmode: flex\ntitle: \"T\"\n---\n\n# Doc\n\n:::note\n" + pipe + ":::\n", true},
		{"doclang TABLE rows:", "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n  TABLE\n    headers: [\"A\", \"B\", \"C\"]\n    rows:\n      [\"1\", \"2\", \"3\"]\n      [\"4\", \"5\"]\n      [\"6\"]\n", true},
		{"slidelang flex", "---\nmode: flex\ntitle: \"T\"\n---\n\n# Deck\n\n## S\n\n" + pipe, false},
		{"slidelang strict", "---\nmode: strict\ntitle: \"T\"\n---\n\nSLIDE content\n  title: \"T\"\n" + indentLines(pipe, 2), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if errs := buildErrors(tc.src, tc.document); len(errs) > 0 {
				t.Fatalf("build errors: %v\n%s", errs, tc.src)
			}
			p := parser.New(util.NewNoop())
			var doc *ast.AST
			var diags []diagnostics.Diagnostic
			if tc.document {
				doc, diags = p.ParseDocument(tc.src, "t.doclang")
			} else {
				doc, diags = p.Parse(tc.src, "t.slidelang")
			}
			table := findTable(doc)
			if table == nil {
				t.Fatalf("no table\n%s", tc.src)
			}
			wantRows := [][]string{{"1", "2", "3"}, {"4", "5", ""}, {"6", "", ""}}
			if !reflect.DeepEqual(table.Rows, wantRows) {
				t.Errorf("rows %q, want %q", table.Rows, wantRows)
			}
			if len(table.Cells) != 4 || len(table.Cells[2]) != 3 || len(table.Cells[3]) != 3 || table.Cells[3][2] != (ast.TableCell{}) {
				t.Errorf("cells do not follow the padded rows: %+v", table.Cells)
			}
			// TABLE010 en la línea de cada fila corta; el contenido de la
			// línea dice cuál es.
			lines := strings.Split(tc.src, "\n")
			var got []string
			for _, d := range diags {
				if d.RuleID == "TABLE010" {
					if !d.IsWarning() {
						t.Errorf("TABLE010 is not a warning: %v", d)
					}
					got = append(got, strings.TrimSpace(lines[d.Position.Line-1]))
				}
			}
			want := []string{"| 4 | 5 |", "| 6 |"}
			if strings.Contains(tc.name, "rows:") {
				want = []string{`["4", "5"]`, `["6"]`}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("TABLE010 on lines %q, want %q", got, want)
			}
			for _, d := range lintDiags(t, tc.src, tc.document) {
				if d.RuleID == "TABLE003" || d.RuleID == "TABLE009" || d.RuleID == "TABLE010" {
					t.Errorf("the linter still sees a short row: %v", d)
				}
			}
		})
	}
}

// Una fila con más celdas que el encabezado sigue siendo TABLE003, un error,
// en su línea: GFM descarta el exceso, y eso perdería texto en silencio. La
// tabla no se toca.
func TestLongTableRowIsStillTABLE003(t *testing.T) {
	src := "# Doc\n\n| A | B |\n|---|---|\n| 1 | 2 |\n| 3 | 4 | 5 |\n"
	errs := buildErrors(src, true)
	if len(errs) != 1 || !strings.HasPrefix(errs[0], "TABLE003: ") {
		t.Fatalf("build errors: %v", errs)
	}
	for _, d := range lintDiags(t, src, true) {
		if d.RuleID == "TABLE003" && d.Position.Line != 6 {
			t.Errorf("TABLE003 on line %d, want 6", d.Position.Line)
		}
	}
}

// Una política sube o apaga TABLE010 por su ID; TABLE003 ya no lo cubre.
func TestTABLE010Policy(t *testing.T) {
	base := "---\nmode: flex\ntitle: \"T\"\n%s---\n\n# Doc\n\n| A | B |\n|---|---|\n| 1 |\n"
	with := func(rules string) string {
		return strings.Replace(base, "%s", "lint_policy:\n  rules:\n"+rules, 1)
	}
	if errs := buildErrors(strings.Replace(base, "%s", "", 1), true); len(errs) > 0 {
		t.Fatalf("build errors: %v", errs)
	}
	if errs := buildErrors(with("    TABLE003:\n      severity: error\n"), true); len(errs) > 0 {
		t.Fatalf("TABLE003 raised to error fails a short row: %v", errs)
	}
	if errs := buildErrors(with("    TABLE010:\n      severity: error\n"), true); len(errs) != 1 || !strings.HasPrefix(errs[0], "TABLE010: ") {
		t.Fatalf("with TABLE010 raised to error: %v", errs)
	}
}

// Un AST que llega de un --filter o de JSON pasa por ast.DecodeAST, que
// completa la fila corta: el linter la avisa con TABLE010 en la línea de la
// fila (también en una columna de grid), y la fila llega entera a los
// generadores. Un AST armado en Go que no pasó por el parser ni por
// DecodeAST trae la fila corta y así saldría en los generadores: sigue
// siendo TABLE003 (TABLE009 en una columna de grid), como antes.
func TestLinterShortRowFromAnASTThatSkippedTheParser(t *testing.T) {
	table := func() *ast.TableElement {
		tbl := ast.NewTableElement(diagnostics.NewPosition(3, 1))
		tbl.Headers = []string{"A", "B"}
		tbl.Rows = [][]string{{"1", "2"}, {"3"}}
		tbl.Cells = ast.DeriveCellsFromFlat(tbl.Headers, tbl.Rows)
		tbl.RowPositions = []diagnostics.Position{diagnostics.NewPosition(5, 1), diagnostics.NewPosition(6, 1)}
		return tbl
	}
	wrap := map[string]func(*ast.TableElement) ast.Element{
		"block": func(tbl *ast.TableElement) ast.Element { return tbl },
		"grid column": func(tbl *ast.TableElement) ast.Element {
			pos := diagnostics.NewPosition(2, 1)
			grid, col := ast.NewGridElement(pos), ast.NewColumnElement(pos, "")
			col.Elements = []ast.Element{tbl}
			grid.Columns = []ast.ColumnElement{*col}
			return grid
		},
	}
	tableDiags := func(doc *ast.AST) []diagnostics.Diagnostic {
		var got []diagnostics.Diagnostic
		for _, d := range (&ElementStructureRule{}).Check(&doc.ContentBlocks[0]) {
			if strings.HasPrefix(d.RuleID, "TABLE") {
				got = append(got, d)
			}
		}
		return got
	}
	for name, w := range wrap {
		t.Run(name+" through DecodeAST", func(t *testing.T) {
			doc := ast.NewAST(diagnostics.NewPosition(1, 1))
			doc.ContentBlocks = []ast.ContentBlock{{Elements: []ast.Element{w(table())}}}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := ast.DecodeAST(raw)
			if err != nil {
				t.Fatal(err)
			}
			tbl := findTable(decoded)
			if !reflect.DeepEqual(tbl.Rows, [][]string{{"1", "2"}, {"3", ""}}) || len(tbl.Cells[2]) != 2 {
				t.Fatalf("DecodeAST did not complete the row: rows %q cells %+v", tbl.Rows, tbl.Cells)
			}
			got := tableDiags(decoded)
			if len(got) != 1 || got[0].RuleID != "TABLE010" || !got[0].IsWarning() || got[0].Position.Line != 6 || !strings.Contains(got[0].Message, "has 1 cell where") {
				t.Fatalf("want one TABLE010 warning on line 6, got %v", got)
			}
		})
		t.Run(name+" built in Go", func(t *testing.T) {
			doc := ast.NewAST(diagnostics.NewPosition(1, 1))
			doc.ContentBlocks = []ast.ContentBlock{{Elements: []ast.Element{w(table())}}}
			want := "TABLE003"
			if name == "grid column" {
				want = "TABLE009"
			}
			got := tableDiags(doc)
			if len(got) != 1 || got[0].RuleID != want || got[0].Position.Line != 6 {
				t.Fatalf("want one %s on line 6, got %v", want, got)
			}
		})
	}
}

// Las entradas públicas de cada parser de cuerpo completan las filas cortas
// y las avisan, igual que ParseDocument y Parse.
func TestBodyParsersPadShortRows(t *testing.T) {
	pipe := "| A | B |\n|---|---|\n| 1 |\n"
	for name, parse := range map[string]func() (*ast.AST, []diagnostics.Diagnostic){
		"NewDocumentFlexParser": func() (*ast.AST, []diagnostics.Diagnostic) {
			return parser.NewDocumentFlexParser("# D\n\n"+pipe, util.NewNoop()).Parse()
		},
		"NewDocumentFlexParserWithNormalization": func() (*ast.AST, []diagnostics.Diagnostic) {
			return parser.NewDocumentFlexParserWithNormalization("# D\n\n"+pipe, util.NewNoop()).Parse()
		},
		"NewDocumentStrictParser": func() (*ast.AST, []diagnostics.Diagnostic) {
			return parser.NewDocumentStrictParser("---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n"+indentLines(pipe, 2), util.NewNoop()).Parse()
		},
		"NewFlexParser": func() (*ast.AST, []diagnostics.Diagnostic) {
			return parser.NewFlexParser("# S\n\n"+pipe, util.NewNoop()).Parse()
		},
		"NewStrictParser": func() (*ast.AST, []diagnostics.Diagnostic) {
			return parser.NewStrictParser("SLIDE content\n  title: \"T\"\n"+indentLines(pipe, 2), util.NewNoop()).Parse()
		},
	} {
		t.Run(name, func(t *testing.T) {
			doc, diags := parse()
			tbl := findTable(doc)
			if tbl == nil || !reflect.DeepEqual(tbl.Rows, [][]string{{"1", ""}}) {
				t.Fatalf("rows not completed: %+v", tbl)
			}
			n := 0
			for _, d := range diags {
				if d.RuleID == "TABLE010" {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("%d TABLE010, want 1: %v", n, diags)
			}
		})
	}
}

func findTable(doc *ast.AST) *ast.TableElement {
	var found *ast.TableElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if t, ok := n.(*ast.TableElement); ok && found == nil {
			found = t
		}
		return nil
	})
	return found
}
