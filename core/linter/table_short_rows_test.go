// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
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

// Un AST que no pasó por el parser (un --filter, uno leído de JSON) puede
// traer una fila corta: el linter da TABLE010, no TABLE003, también en una
// columna de grid.
func TestLinterShortRowFromAnASTThatSkippedTheParser(t *testing.T) {
	table := func() *ast.TableElement {
		tbl := ast.NewTableElement(diagnostics.NewPosition(3, 1))
		tbl.Headers = []string{"A", "B"}
		tbl.Rows = [][]string{{"1", "2"}, {"3"}}
		tbl.RowPositions = []diagnostics.Position{diagnostics.NewPosition(5, 1), diagnostics.NewPosition(6, 1)}
		return tbl
	}
	grid := &ast.GridElement{Columns: []ast.ColumnElement{{Elements: []ast.Element{table()}}}}
	for name, el := range map[string]ast.Element{"block": table(), "grid column": grid} {
		t.Run(name, func(t *testing.T) {
			block := &ast.ContentBlock{Elements: []ast.Element{el}}
			var got []diagnostics.Diagnostic
			for _, d := range (&ElementStructureRule{}).Check(block) {
				if strings.HasPrefix(d.RuleID, "TABLE") {
					got = append(got, d)
				}
			}
			if len(got) != 1 || got[0].RuleID != "TABLE010" || !got[0].IsWarning() || got[0].Position.Line != 6 {
				t.Fatalf("want one TABLE010 warning on line 6, got %v", got)
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
