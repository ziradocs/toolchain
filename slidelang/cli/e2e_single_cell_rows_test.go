// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/report"
)

// coreKeepsSingleCellRows dice si el core con el que se compila deja en su
// tabla tres filas de una sola celda en una presentación en modo auto cuyo
// cuerpo empieza con "# " (ahí corre el normalizador completo). Contra un
// core publicado que todavía las borra, la prueba de abajo se salta. La
// guarda pregunta lo mismo que la prueba afirma, así que con GOWORK=off no
// prueba nada: la cubre el job que compila contra el core del árbol. Se
// quita cuando slidelang requiera un core publicado con el arreglo.
func coreKeepsSingleCellRows() bool {
	doc, _ := parser.New(nil).Parse("---\ntitle: T\n---\n\n# S\n\n| A | B | C |\n|---|---|---|\n| r0 |\n| r1 |\n| r2 |\n", "s.slidelang")
	if doc == nil {
		return false
	}
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if table, ok := el.(*ast.TableElement); ok {
				return len(table.Rows) == 3
			}
		}
	}
	return false
}

// Filas de una sola celda con pipe de cierre bajo un encabezado de tres
// columnas, en una presentación sin `mode:` (modo auto): cada una es una fila
// de la misma tabla en el AST de `build --format json`, completada con celdas
// vacías, con su TABLE010 en el reporte en su línea, sin una fila de guiones,
// y el texto de después sigue en su línea.
func TestE2E_SingleCellTableRows(t *testing.T) {
	if !coreKeepsSingleCellRows() {
		t.Skip("this core drops single-cell table rows when the full normalizer runs")
	}
	const head = "---\ntitle: \"D\"\n---\n\n# Resultados\n\n| A | B | C |\n|---|---|---|\n"
	for _, tc := range []struct {
		name, body string
		rows       [][]string
		short      []int
	}{
		{"full row first", "| 1 | 2 | 3 |\n| r0 |\n| r1 |\n| r2 |\n\nDespues.\n",
			[][]string{{"1", "2", "3"}, {"r0", "", ""}, {"r1", "", ""}, {"r2", "", ""}}, []int{10, 11, 12}},
		{"full row last", "| r0 |\n| r1 |\n| r2 |\n| 1 | 2 | 3 |\n\nDespues.\n",
			[][]string{{"r0", "", ""}, {"r1", "", ""}, {"r2", "", ""}, {"1", "2", "3"}}, []int{9, 10, 11}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			in := filepath.Join(dir, "d.slidelang")
			if err := os.WriteFile(in, []byte(head+tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(dir, "out")
			rep := filepath.Join(dir, "report.json")
			if err := runCLI(t, Options{}, []string{"build", in, "--format", "json", "--output", out, "--report", "json", "--report-out", rep}); err != nil {
				t.Fatalf("build: %v", err)
			}
			raw, err := os.ReadFile(filepath.Join(out, "d.json"))
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				ContentBlocks []struct {
					Elements []struct {
						Type     string     `json:"type"`
						Content  string     `json:"content"`
						Headers  []string   `json:"headers"`
						Rows     [][]string `json:"rows"`
						Position struct {
							Line int `json:"line"`
						} `json:"position"`
					} `json:"elements"`
				} `json:"contentBlocks"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("AST: %v", err)
			}
			tables, after := 0, 0
			for _, block := range doc.ContentBlocks {
				for _, el := range block.Elements {
					switch {
					case el.Type == "table":
						tables++
						if !reflect.DeepEqual(el.Headers, []string{"A", "B", "C"}) || !reflect.DeepEqual(el.Rows, tc.rows) {
							t.Errorf("table = %q %q, want [A B C] %q", el.Headers, el.Rows, tc.rows)
						}
					case el.Type == "text" && el.Content == "Despues.":
						after = el.Position.Line
					}
				}
			}
			if tables != 1 {
				t.Errorf("got %d tables, want 1", tables)
			}
			if after != 14 {
				t.Errorf("text after the table on line %d, want 14", after)
			}
			data, err := os.ReadFile(rep)
			if err != nil {
				t.Fatal(err)
			}
			var r report.ReportJSON
			if err := json.Unmarshal(data, &r); err != nil {
				t.Fatalf("report: %v", err)
			}
			var short []int
			for _, f := range r.Findings {
				switch f.RuleID {
				case "TABLE010":
					short = append(short, f.Position.Line)
				case "TABLE002", "TABLE003", "TABLE008":
					t.Errorf("unexpected %s at line %d: %s", f.RuleID, f.Position.Line, f.Message)
				}
			}
			sort.Ints(short)
			if !reflect.DeepEqual(short, tc.short) {
				t.Errorf("TABLE010 on lines %v, want %v", short, tc.short)
			}
		})
	}
}
