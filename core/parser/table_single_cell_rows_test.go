// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/util"
)

// Filas de una sola celda con pipe de cierre ("| r0 |") bajo un encabezado
// más ancho. Cuando corre el normalizador completo (una presentación en modo
// auto cuyo cuerpo empieza con "# ", un documento sin frontmatter que empieza
// con "# ", o cualquier fuente que no sea strict en la que el detector
// puntúa, como una con "Action Items"), TablesRule tomaba tres o más de esas
// filas por una tabla nueva: con menos de dos columnas las reemplazaba por una
// línea vacía (las filas se perdían sin diagnóstico) y, si la racha llegaba a
// una fila de tres celdas, les ponía un encabezado y un separador propios a
// media tabla (una fila de guiones en el AST). Cuando el normalizador no corre
// entero, cada una ya era una fila corta de la misma tabla, completada con
// TABLE010.

const singleCellHead = "| A | B | C |\n|---|---|---|\n"

// singleCellContexts son las entradas donde corre el normalizador completo.
// start es la línea del archivo en la que empieza body.
type singleCellContext struct {
	name  string
	start int
	parse func(body string) (*ast.AST, []diagnostics.Diagnostic)
}

func singleCellContexts() []singleCellContext {
	lint := func(doc *ast.AST, diags []diagnostics.Diagnostic) (*ast.AST, []diagnostics.Diagnostic) {
		if doc != nil {
			diags = append(diags, linter.New().Lint(doc)...)
		}
		return doc, diags
	}
	return []singleCellContext{
		{"slides auto", 7, func(body string) (*ast.AST, []diagnostics.Diagnostic) {
			return lint(New(util.NewNoop()).Parse("---\ntitle: T\n---\n\n# S\n\n"+body, "s.slidelang"))
		}},
		// "Action Items" hace puntuar al detector también con mode: flex.
		{"slides flex scored by the detector", 7, func(body string) (*ast.AST, []diagnostics.Diagnostic) {
			return lint(New(util.NewNoop()).Parse("---\ntitle: T\nmode: flex\n---\n# Action Items\n\n"+body, "s.slidelang"))
		}},
		{"document without frontmatter", 3, func(body string) (*ast.AST, []diagnostics.Diagnostic) {
			return lint(New(util.NewNoop()).ParseDocument("# S\n\n"+body, "d.md"))
		}},
		{"document with frontmatter scored by the detector", 7, func(body string) (*ast.AST, []diagnostics.Diagnostic) {
			return lint(New(util.NewNoop()).ParseDocument("---\ntitle: T\n---\n\n# Action Items\n\n"+body, "d.doclang"))
		}},
	}
}

// singleCellReference es la misma entrada sin el normalizador completo: el
// mismo número de líneas antes de body, así que las posiciones se comparan
// tal cual.
func singleCellReference(ctx string) func(body string) (*ast.AST, []diagnostics.Diagnostic) {
	lint := func(doc *ast.AST, diags []diagnostics.Diagnostic) (*ast.AST, []diagnostics.Diagnostic) {
		if doc != nil {
			diags = append(diags, linter.New().Lint(doc)...)
		}
		return doc, diags
	}
	if strings.HasPrefix(ctx, "slides") {
		return func(body string) (*ast.AST, []diagnostics.Diagnostic) {
			return lint(New(util.NewNoop()).Parse("---\ntitle: T\nmode: flex\n---\n# S\n\n"+body, "s.slidelang"))
		}
	}
	if ctx == "document without frontmatter" {
		return nil // no hay un documento con las mismas líneas que no puntúe
	}
	return func(body string) (*ast.AST, []diagnostics.Diagnostic) {
		return lint(New(util.NewNoop()).ParseDocument("---\ntitle: T\n---\n\n# Sin puntaje\n\n"+body, "d.doclang"))
	}
}

// tableShape es lo que se compara de un AST: cada tabla, cada texto y cada
// columna de grid con su línea, y los diagnósticos TABLE* con la suya.
func tableShape(t *testing.T, doc *ast.AST, diags []diagnostics.Diagnostic) []string {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	var out []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			line := 0
			if p, ok := x["position"].(map[string]any); ok {
				if l, ok := p["line"].(float64); ok {
					line = int(l)
				}
			}
			switch x["type"] {
			case "table":
				h, _ := json.Marshal(x["headers"])
				r, _ := json.Marshal(x["rows"])
				c, _ := json.Marshal(x["cells"])
				out = append(out, fmt.Sprintf("table %d %s %s %s", line, h, r, c))
			case "text", "column":
				out = append(out, fmt.Sprintf("%s %d %q", x["type"], line, fmt.Sprint(x["content"])))
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(x[k])
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(tree)
	var ds []string
	for _, d := range diags {
		if strings.HasPrefix(d.RuleID, "TABLE") {
			ds = append(ds, fmt.Sprintf("%s %d %s", d.RuleID, d.Position.Line, d.Message))
		}
	}
	sort.Strings(ds)
	return append(out, ds...)
}

func tablesOf(doc *ast.AST) []*ast.TableElement {
	var tables []*ast.TableElement
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if table, ok := el.(*ast.TableElement); ok {
				tables = append(tables, table)
			}
		}
	}
	return tables
}

func textsOf(doc *ast.AST) map[string]int {
	texts := map[string]int{}
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if text, ok := el.(*ast.TextElement); ok {
				texts[text.Content] = text.Position.Line
			}
		}
	}
	return texts
}

// Los cinco casos del reporte: cada fila corta es una fila de la misma tabla,
// completada con celdas vacías y con TABLE010 en su línea; ninguna fila de
// guiones y el texto de después en su línea.
func TestSingleCellRowsStayInTheirTable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		rows  [][]string
		short []int // líneas de body (desde 1) con TABLE010
		after int   // línea de body de "Despues.", 0 si no hay
	}{
		{"two-cell rows", singleCellHead + "| 1 | x |\n| 1 | x |\n| 1 | x |\n",
			[][]string{{"1", "x", ""}, {"1", "x", ""}, {"1", "x", ""}}, []int{3, 4, 5}, 0},
		{"two single-cell rows", singleCellHead + "| r0 |\n| r1 |\n",
			[][]string{{"r0", "", ""}, {"r1", "", ""}}, []int{3, 4}, 0},
		{"three single-cell rows", singleCellHead + "| r0 |\n| r1 |\n| r2 |\n",
			[][]string{{"r0", "", ""}, {"r1", "", ""}, {"r2", "", ""}}, []int{3, 4, 5}, 0},
		{"full row, then three single-cell rows and text", singleCellHead + "| 1 | 2 | 3 |\n| r0 |\n| r1 |\n| r2 |\n\nDespues.\n",
			[][]string{{"1", "2", "3"}, {"r0", "", ""}, {"r1", "", ""}, {"r2", "", ""}}, []int{4, 5, 6}, 8},
		{"three single-cell rows, then a full row", singleCellHead + "| r0 |\n| r1 |\n| r2 |\n| 1 | 2 | 3 |\n",
			[][]string{{"r0", "", ""}, {"r1", "", ""}, {"r2", "", ""}, {"1", "2", "3"}}, []int{3, 4, 5}, 0},
		// Más filas de las diez que la regla junta de una vez: antes se
		// perdían diez y las dos últimas eran otra tabla.
		{"twelve single-cell rows", singleCellHead + strings.Repeat("| r |\n", 12) + "\nDespues.\n",
			[][]string{{"r", "", ""}, {"r", "", ""}, {"r", "", ""}, {"r", "", ""}, {"r", "", ""}, {"r", "", ""},
				{"r", "", ""}, {"r", "", ""}, {"r", "", ""}, {"r", "", ""}, {"r", "", ""}, {"r", "", ""}},
			[]int{3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}, 16},
	} {
		for _, c := range singleCellContexts() {
			t.Run(tc.name+"/"+c.name, func(t *testing.T) {
				doc, diags := c.parse(tc.body)
				if doc == nil {
					t.Fatalf("no AST: %v", diags)
				}
				tables := tablesOf(doc)
				if len(tables) != 1 {
					t.Fatalf("got %d tables, want 1", len(tables))
				}
				table := tables[0]
				if want := []string{"A", "B", "C"}; !reflect.DeepEqual(table.Headers, want) {
					t.Errorf("Headers = %q, want %q", table.Headers, want)
				}
				if !reflect.DeepEqual(table.Rows, tc.rows) {
					t.Errorf("Rows = %q, want %q", table.Rows, tc.rows)
				}
				if len(table.Cells) != len(tc.rows)+1 {
					t.Errorf("Cells has %d rows, want %d", len(table.Cells), len(tc.rows)+1)
				}
				var short []int
				for _, d := range diags {
					switch d.RuleID {
					case "TABLE010":
						short = append(short, d.Position.Line-c.start+1)
					case "TABLE002", "TABLE003", "TABLE008":
						t.Errorf("unexpected %s at line %d: %s", d.RuleID, d.Position.Line, d.Message)
					}
				}
				sort.Ints(short)
				if !reflect.DeepEqual(short, tc.short) {
					t.Errorf("TABLE010 on body lines %v, want %v", short, tc.short)
				}
				if tc.after > 0 {
					line, ok := textsOf(doc)["Despues."]
					if !ok {
						t.Fatalf("the text after the table is gone")
					}
					if want := c.start + tc.after - 1; line != want {
						t.Errorf("text after the table on line %d, want %d", line, want)
					}
				}
			})
		}
	}
}

// Las tablas que la regla nunca debió tocar salen igual que sin el
// normalizador completo: con las mismas filas, las mismas posiciones y los
// mismos diagnósticos TABLE*. Varias se perdían o salían mal antes (una
// columna con tres filas o más, filas de dos celdas sin pipe de cierre, las
// de un bloque ":::", de una columna de grid o de debajo de una lista).
func TestNormalizerKeepsRowsOfExistingTables(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"one-column table", "| A |\n|---|\n| x |\n"},
		{"one-column table with three rows", "| A |\n|---|\n| x |\n| y |\n| z |\n\nDespues.\n"},
		{"two tables without a blank line", "| A | B |\n|---|---|\n| 1 | 2 |\n| C | D |\n|---|---|\n| 3 | 4 |\n"},
		{"one-column table right below a wider one", singleCellHead + "| 1 | 2 | 3 |\n| D |\n|---|\n| x |\n| y |\n| z |\n"},
		{"rows without a closing pipe", singleCellHead + "| r0\n| r1\n| r2\n"},
		{"two-cell rows without a closing pipe", singleCellHead + "| a | b\n| c | d\n| e | f\n"},
		{"header without separator, rows without a closing pipe", "| A | B |\n| c | d\n| e | f\n| g | h\n"},
		{"explicit empty cells", singleCellHead + "| r0 | | |\n| r1 | | |\n| r2 | | |\n"},
		{"table without separator", "| a | b\n| c | d\n| e | f\n"},
		{"single-cell rows without a header", "Antes.\n\n| r0 |\n| r1 |\n| r2 |\n\nDespues.\n"},
		{"note block", ":::note\n" + singleCellHead + "| r0 |\n| r1 |\n| r2 |\n:::\n\nDespues.\n"},
		{"grid column", "::: grid\n::: column\n" + singleCellHead + "| r0 |\n| r1 |\n| r2 |\n:::\n::: column\nOtro.\n:::\n:::\n\nDespues.\n"},
		{"after a list", "- uno\n- dos\n\n" + singleCellHead + "| r0 |\n| r1 |\n| r2 |\n\nDespues.\n"},
		{"indented under a list item", "- uno\n\n  | A | B | C |\n  |---|---|---|\n  | r0 |\n  | r1 |\n  | r2 |\n\nDespues.\n"},
	} {
		for _, c := range singleCellContexts() {
			ref := singleCellReference(c.name)
			if ref == nil {
				continue
			}
			t.Run(tc.name+"/"+c.name, func(t *testing.T) {
				doc, diags := c.parse(tc.body)
				refDoc, refDiags := ref(tc.body)
				if doc == nil || refDoc == nil {
					t.Fatalf("no AST: %v / %v", diags, refDiags)
				}
				got, want := tableShape(t, doc, diags), tableShape(t, refDoc, refDiags)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("with the full normalizer:\n%s\nwithout it:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
				}
			})
		}
	}
}

// La regla arma una sola tabla por pasada y la primera candidata termina la
// pasada aunque ya no la convierta: una lista "clave: valor" debajo de una
// tabla de filas cortas sigue siendo texto, como en v2.61.0, y la tabla
// conserva sus filas.
func TestNormalizerLeavesTheListAfterTheTable(t *testing.T) {
	for _, c := range singleCellContexts() {
		t.Run(c.name, func(t *testing.T) {
			doc, diags := c.parse(singleCellHead + "| r0 |\n| r1 |\n| r2 |\n\nNombre: Ana\nEdad: 30\nCiudad: Lima\n")
			if doc == nil {
				t.Fatalf("no AST: %v", diags)
			}
			tables := tablesOf(doc)
			if len(tables) != 1 {
				t.Fatalf("got %d tables, want 1", len(tables))
			}
			if want := [][]string{{"r0", "", ""}, {"r1", "", ""}, {"r2", "", ""}}; !reflect.DeepEqual(tables[0].Rows, want) {
				t.Errorf("Rows = %q, want %q", tables[0].Rows, want)
			}
			if line, ok := textsOf(doc)["Nombre: Ana Edad: 30 Ciudad: Lima"]; !ok {
				t.Errorf("the key: value list is not a paragraph: %v", textsOf(doc))
			} else if want := c.start + 6; line != want {
				t.Errorf("the list is on line %d, want %d", line, want)
			}
		})
	}
}

// Una línea separadora antes del encabezado la salta el parser, así que la
// línea siguiente es el encabezado, la primera de la tabla: ahí la regla
// convierte como en v2.61.0 (encabezado completado al ancho de las filas, sin
// TABLE003), igual que sin la línea de guiones.
func TestNormalizerConvertsAtTheHeaderAfterALeadingSeparator(t *testing.T) {
	for _, lead := range []string{"|---|---|\n", "|-----|\n", ""} {
		for _, c := range singleCellContexts() {
			t.Run(fmt.Sprintf("%q/%s", lead, c.name), func(t *testing.T) {
				doc, diags := c.parse(lead + "| A | B\n| a | b | c\n| d | e | f\n")
				if doc == nil {
					t.Fatalf("no AST: %v", diags)
				}
				tables := tablesOf(doc)
				if len(tables) != 1 {
					t.Fatalf("got %d tables, want 1", len(tables))
				}
				if want := []string{"A", "B", ""}; !reflect.DeepEqual(tables[0].Headers, want) {
					t.Errorf("Headers = %q, want %q", tables[0].Headers, want)
				}
				if want := [][]string{{"a", "b", "c"}, {"d", "e", "f"}}; !reflect.DeepEqual(tables[0].Rows, want) {
					t.Errorf("Rows = %q, want %q", tables[0].Rows, want)
				}
				for _, d := range diags {
					if strings.HasPrefix(d.RuleID, "TABLE") {
						t.Errorf("unexpected %s at line %d: %s", d.RuleID, d.Position.Line, d.Message)
					}
				}
			})
		}
	}
}

// El cambio de comportamiento: una fila más ancha que el encabezado que la
// regla sacaba de su tabla ahora se lee en ella y da TABLE003. body es lo que
// va desde la línea start del contexto; line, la línea de body con TABLE003.
func TestNormalizerReadsTheWiderRowInItsTable(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		line       int
	}{
		{"ten one-cell rows before a wider row", strings.Repeat("| r |\n", 10) + "| A | B |\n| 1 | 2 |\n", 11},
		{"TABLE block with a wider row below one-cell rows", "TABLE\n  | H0 |\n  |---|\n  | a |\n  | b |\n  | v | v |\n", 6},
		// La primera línea, con guiones, sería el encabezado recuperado (abajo
		// va un separador con su número de celdas), pero las filas no caben y
		// el parser la vuelve a leer sin recuperarlo: el encabezado es
		// "| c | d", la regla no convierte en "| e | f | g" y da TABLE003.
		{"dashed first line whose header is not recovered", "| x | --- |\n|---|---|\n| c | d\n| e | f | g\n| h | i\n", 4},
	} {
		for _, c := range singleCellContexts() {
			t.Run(tc.name+"/"+c.name, func(t *testing.T) {
				doc, diags := c.parse(tc.body)
				if doc == nil {
					t.Fatalf("no AST: %v", diags)
				}
				var got []int
				for _, d := range diags {
					switch d.RuleID {
					case "TABLE003":
						got = append(got, d.Position.Line-c.start+1)
					case "TABLE002":
						t.Errorf("unexpected TABLE002 at line %d", d.Position.Line)
					}
				}
				if !reflect.DeepEqual(got, []int{tc.line}) {
					t.Errorf("TABLE003 on body lines %v, want [%d]", got, tc.line)
				}
				if n := len(tablesOf(doc)); n != 1 {
					t.Errorf("got %d tables, want 1", n)
				}
			})
		}
	}
}
