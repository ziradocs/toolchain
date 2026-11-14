// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
)

// Una forma que se relee con la misma tabla pero con un aviso del parser no
// sirve: el aviso dice que el parser dejó algo afuera. tableReadsBack la
// descarta en los dos dialectos.
func TestTableReadsBack_RejectsAFormWithParserWarnings(t *testing.T) {
	for _, d := range []tableDialect{tableDialects[0], tableDialects[5]} {
		t.Run(d.name, func(t *testing.T) {
			rows := strings.Repeat(" ", d.rowsIndent()-2)
			doc, _ := d.parse(d.wrap("TABLE\n  headers: [\"A\"]\n  rows:\n" + rows + "  [\"1\"]"))
			e := firstTable(doc)
			if e == nil || len(e.Rows) != 1 {
				t.Fatalf("fixture: %s", jsonOf(t, stripPositions(e)))
			}
			clean := "TABLE\n  headers: [\"A\"]\n  rows:\n" + strings.Repeat(" ", d.rowsIndent()) + "[\"1\"]"
			if !tableReadsBack(e, clean, !d.strict) {
				t.Fatalf("the clean form does not read back:\n%s", clean)
			}
			// La misma tabla, más una línea que el TABLE consume sin leer
			// (TABLE008).
			warned := clean + "\n  nota suelta"
			if tableReadsBack(e, warned, !d.strict) {
				t.Errorf("a form that reads back with a TABLE008 was accepted:\n%s", warned)
			}
		})
	}
}

// Si ninguna forma, headers:/rows: incluida, se relee igual, fmt rechaza la
// tabla en vez de escribir la última que probó. Un caption con un salto de
// línea no cabe en ninguna: la línea caption: se corta.
func TestFormat_TableWithNoFormThatReadsBackIsRefused(t *testing.T) {
	for _, d := range []tableDialect{tableDialects[0], tableDialects[5]} {
		t.Run(d.name, func(t *testing.T) {
			doc, _ := d.parse(d.wrap("TABLE\n  caption: \"Ventas\"\n  headers: [\"A\"]\n  rows:\n" + strings.Repeat(" ", d.rowsIndent()) + "[\"1\"]"))
			e := firstTable(doc)
			e.Caption = "Ventas\nde 2026"
			_, err := d.format(doc)
			var unsupported *UnsupportedElementError
			if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), "does not read back unchanged in any table form") {
				t.Fatalf("want the table refused, got %v", err)
			}
		})
	}
}

// Una cerca de cuatro acentos de un diagrama dentro de un bloque cierra en
// "````", como la cierra su parser, y vuelve con su sangría.
func TestFormat_CalloutFourBacktickDiagramFence(t *testing.T) {
	fence := "````mermaid\ngraph TD\n    A --> B\n````"
	out := calloutRoundTrip(t, fmt.Sprintf(calloutDoc, ":::note\n"+fence+"\n:::"), FormatDocument)
	if !strings.Contains(out, ":::note\n"+fence+"\n:::") {
		t.Errorf("fmt wrote\n%s", out)
	}
}

// El mensaje de un bloque que strict no puede escribir dice su línea y qué
// elemento se pierde, y una cerca de cuatro acentos no deja un acento suelto
// en el nombre del lenguaje.
func TestFormat_CalloutRefusalNamesTheElement(t *testing.T) {
	d := tableDialect{}
	doc, _ := d.parse(fmt.Sprintf(calloutDoc, ":::note\n![alt](foto.png)\n:::"))
	_, err := FormatDocumentStrict(doc)
	if err == nil || !strings.Contains(err.Error(), ":::note block (line 6)") || !strings.Contains(err.Error(), `the image "foto.png"`) {
		t.Errorf("the strict refusal does not name the line and the element: %v", err)
	}
	code := &ast.CodeElement{Language: "`python", Content: "if x:\n    y()"}
	if got := describeNestedElement(code); !strings.Contains(got, "the python code block") {
		t.Errorf("describeNestedElement = %q", got)
	}
}

// Releer cada tabla para elegir su forma cuesta lo mismo por tabla: fmt sigue
// siendo lineal en el número de tablas y en el de filas.
func TestFormat_TableRereadScalesLinearly(t *testing.T) {
	pipe := "| A%d | B |\n|---|---|\n| 1 | 2 |"
	// Las cuatro formas probadas: con una celda con espacios en los bordes
	// (pipe y TABLE con filas "|" los recortan) y una fila de otro largo
	// (cells: la rellena) solo cabe en headers:/rows:.
	worst := "TABLE\n  headers: [\"A%d\"]\n  rows:\n%[2]s[\" x \"]\n%[2]s[\"1\", \"2\"]"
	many := func(table string, strict bool) func(n int) (*ast.AST, func(*ast.AST) (string, error)) {
		return func(n int) (*ast.AST, func(*ast.AST) (string, error)) {
			d := tableDialects[5]
			format := FormatDocument
			if strict {
				d = tableDialects[3]
				format = FormatDocumentStrict
			}
			var b strings.Builder
			for i := 0; i < n; i++ {
				if i > 0 {
					b.WriteString("\n\n")
				}
				if strings.Contains(table, "%[2]s") {
					fmt.Fprintf(&b, table, i, strings.Repeat(" ", d.rowsIndent()))
				} else {
					fmt.Fprintf(&b, table, i)
				}
			}
			doc, _ := d.parse(d.wrap(b.String()))
			if out, err := format(doc); err != nil || strings.Contains(table, "TABLE") && !strings.Contains(out, "rows:") {
				t.Fatalf("fixture: %v\n%.300s", err, out)
			}
			return doc, format
		}
	}
	rows := func(n int) (*ast.AST, func(*ast.AST) (string, error)) {
		lines := []string{"| A | B |", "|---|---|"}
		for i := 0; i < n; i++ {
			lines = append(lines, fmt.Sprintf("| %d | x |", i))
		}
		doc, _ := tableDialects[5].parse(tableDialects[5].wrap(strings.Join(lines, "\n")))
		return doc, FormatDocument
	}
	for name, gen := range map[string]func(int) (*ast.AST, func(*ast.AST) (string, error)){
		"pipe tables, flex":        many(pipe, false),
		"pipe tables, strict":      many(pipe, true),
		"four-form tables, flex":   many(worst, false),
		"four-form tables, strict": many(worst, true),
		"rows of one table":        rows,
	} {
		t.Run(name, func(t *testing.T) {
			best := func(n int) time.Duration {
				doc, format := gen(n)
				d := time.Duration(1 << 62)
				for i := 0; i < 3; i++ {
					runtime.GC()
					st := time.Now()
					if _, err := format(doc); err != nil {
						t.Fatalf("fmt: %v", err)
					}
					if e := time.Since(st); e < d {
						d = e
					}
				}
				return d
			}
			n := 50
			for n < 1<<14 && best(n) < 10*time.Millisecond {
				n *= 2
			}
			var t1, t4 time.Duration
			for a := 0; a < 3; a++ {
				t1, t4 = best(n), best(4*n)
				if t4 < 10*t1 {
					return
				}
			}
			t.Errorf("%v with n=%d and %v with 4n (limit 10x)", t1, n, t4)
		})
	}
}
