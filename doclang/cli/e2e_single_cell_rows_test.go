// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/report"
)

// coreKeepsSingleCellRows dice si el core con el que se compila deja en su
// tabla tres filas de una sola celda cuando corre el normalizador completo
// (un documento sin frontmatter que empieza con "# "). Contra un core
// publicado que todavía las borra, las pruebas de abajo se saltan. La
// guarda pregunta lo mismo que la prueba afirma, así que con GOWORK=off no
// prueba nada: la cubre el job que compila contra el core del árbol. Se
// quita cuando slidelang y doclang requieran un core publicado con el
// arreglo.
func coreKeepsSingleCellRows() bool {
	doc, _ := parser.New(nil).ParseDocument("# S\n\n| A | B | C |\n|---|---|---|\n| r0 |\n| r1 |\n| r2 |\n", "d.md")
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
// columnas, en los dos documentos en los que corre el normalizador completo:
// cada una sale en el Markdown como una fila de la misma tabla, completada
// con celdas vacías, con su TABLE010 en el reporte en su línea, sin una fila
// de guiones de más, y el párrafo de después sigue ahí.
func TestE2E_SingleCellTableRows(t *testing.T) {
	if !coreKeepsSingleCellRows() {
		t.Skip("this core drops single-cell table rows when the full normalizer runs")
	}
	const table = "| A | B | C |\n|---|---|---|\n"
	for _, tc := range []struct {
		name, prefix, ext string
	}{
		// El cuerpo empieza en la línea 3.
		{"markdown without frontmatter", "# Informe\n\n", ".md"},
		// "Action Items" hace puntuar al detector aunque haya frontmatter.
		// El cuerpo empieza en la línea 7.
		{"doclang with frontmatter", "---\ntitle: \"D\"\n---\n\n# Action Items\n\n", ".doclang"},
	} {
		for _, body := range []struct {
			name, src, markdown string
			short               []int // líneas del cuerpo (desde 1) con TABLE010
		}{
			{"full row first", table + "| 1 | 2 | 3 |\n| r0 |\n| r1 |\n| r2 |\n\nDespues.\n",
				"| A | B | C |\n| --- | --- | --- |\n| 1 | 2 | 3 |\n| r0 |  |  |\n| r1 |  |  |\n| r2 |  |  |\n\nDespues.\n", []int{4, 5, 6}},
			{"full row last", table + "| r0 |\n| r1 |\n| r2 |\n| 1 | 2 | 3 |\n\nDespues.\n",
				"| A | B | C |\n| --- | --- | --- |\n| r0 |  |  |\n| r1 |  |  |\n| r2 |  |  |\n| 1 | 2 | 3 |\n\nDespues.\n", []int{3, 4, 5}},
		} {
			t.Run(tc.name+"/"+body.name, func(t *testing.T) {
				dir := t.TempDir()
				in := filepath.Join(dir, "d"+tc.ext)
				if err := os.WriteFile(in, []byte(tc.prefix+body.src), 0o600); err != nil {
					t.Fatal(err)
				}
				out := filepath.Join(dir, "out")
				rep := filepath.Join(dir, "report.json")
				if err := runCLI(t, Options{Name: "doclang", Version: "test"}, []string{"build", in, "--format", "markdown", "--output", out, "--report", "json", "--report-out", rep}); err != nil {
					t.Fatalf("build: %v", err)
				}
				md, err := os.ReadFile(filepath.Join(out, "d.md"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(md), body.markdown) {
					t.Errorf("the Markdown does not carry the table as written, followed by the paragraph:\n%s\nwant it to contain:\n%s", md, body.markdown)
				}
				dashes := 0
				for _, line := range strings.Split(string(md), "\n") {
					if strings.HasPrefix(line, "|") && strings.Contains(line, "---") {
						dashes++
					}
				}
				if dashes != 1 {
					t.Errorf("the table has %d rows with dashes, want 1 (the separator row):\n%s", dashes, md)
				}
				data, err := os.ReadFile(rep)
				if err != nil {
					t.Fatal(err)
				}
				var r report.ReportJSON
				if err := json.Unmarshal(data, &r); err != nil {
					t.Fatalf("report: %v", err)
				}
				start := strings.Count(tc.prefix, "\n") + 1
				var short []int
				for _, f := range r.Findings {
					switch f.RuleID {
					case "TABLE010":
						short = append(short, f.Position.Line-start+1)
					case "TABLE002", "TABLE003", "TABLE008":
						t.Errorf("unexpected %s at line %d: %s", f.RuleID, f.Position.Line, f.Message)
					}
				}
				sort.Ints(short)
				if !reflect.DeepEqual(short, body.short) {
					t.Errorf("TABLE010 on body lines %v, want %v", short, body.short)
				}
			})
		}
	}
}
