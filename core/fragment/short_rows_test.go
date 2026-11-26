// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package fragment

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/formatter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Una fila con menos celdas que el encabezado se completa también en un
// fragmento de contenido, como en Parse y ParseDocument (TABLE010, aviso).
// En v2.60.0 el codec rechazaba el fragmento: ParseMarkdownContent dejaba
// la fila corta y el formatter, que relee cada forma de tabla con un parser
// que sí completa, no encontraba ninguna que se releyera igual.

var shortRowWant = [][]string{{"x", "y", "z"}, {"p", "q", ""}}

func TestContentFragmentPadsShortTableRows(t *testing.T) {
	cases := []struct {
		name     string
		markdown string
		line     int // línea de la fila corta en el Markdown original
		// rawBody: el formatter escribe el cuerpo de un ::: tal como está
		// (también en `fmt` de un archivo completo), así que la fila sale
		// corta en la fuente y se vuelve a completar al releerla.
		rawBody bool
		rows    [][]string // filas esperadas; nil es shortRowWant
	}{
		{name: "pipe", markdown: "| A | B | C |\n| --- | --- | --- |\n| x | y | z |\n| p | q |\n", line: 4},
		{name: "pipe con alineación", markdown: "| A | B | C |\n| :-- | :-: | --: |\n| x | y | z |\n| p | q |\n", line: 4},
		{name: "TABLE con filas |", markdown: "TABLE\n  | A | B | C |\n  | x | y | z |\n  | p | q |\n", line: 4},
		{name: "TABLE con caption y filas |", markdown: "TABLE\n  caption: \"Cap\"\n  | A | B | C |\n  | x | y | z |\n  | p | q |\n", line: 5},
		{name: "headers y rows", markdown: "TABLE\n  headers: [\"A\", \"B\", \"C\"]\n  rows:\n      [\"x\", \"y\", \"z\"]\n      [\"p\", \"q\"]\n", line: 5},
		// Una fila de cells: que cubre menos columnas por sus spans: se le
		// agrega solo la que falta. El aviso va en la línea del TABLE, como
		// en un archivo completo (cells: no lleva la posición de cada fila).
		{name: "cells con colspan", markdown: "TABLE\n  cells:\n    - [{content: \"A\", header: true}, {content: \"B\", header: true}, {content: \"C\", header: true}]\n    - [{content: \"x\"}, {content: \"y\"}, {content: \"z\"}]\n    - [{content: \"p\", colspan: 2}]\n", line: 1, rows: [][]string{{"x", "y", "z"}, {"p", "p", ""}}},
		{name: "columna tipada de grid", markdown: "::: grid\n::: column typed\n| A | B | C |\n| --- | --- | --- |\n| x | y | z |\n| p | q |\n:::\n::: column\nOther\n:::\n", line: 6},
		{name: "bloque :::", markdown: "::: note\n| A | B | C |\n| --- | --- | --- |\n| x | y | z |\n| p | q |\n:::\n", line: 5, rawBody: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NormalizeMarkdownContent(c.markdown, parser.ContentOptions{AnchorNamespace: "short-rows"})
			if !r.Receipt.Accepted {
				t.Fatalf("fragment rejected: %+v", r.Receipt.Diagnostics)
			}
			warnings := shortRowDiagnostics(r.Receipt.Diagnostics)
			if len(warnings) != 1 || warnings[0].Severity != diagnostics.Warning || warnings[0].Position.Line != c.line {
				t.Fatalf("want one TABLE010 warning on line %d, got %+v", c.line, r.Receipt.Diagnostics)
			}
			for _, d := range r.Receipt.Diagnostics {
				if d.IsError() {
					t.Fatalf("unexpected error: %+v", d)
				}
			}
			want := c.rows
			if want == nil {
				want = shortRowWant
			}
			if got := onlyTable(t, r.Elements).Rows; !reflect.DeepEqual(got, want) {
				t.Fatalf("rows=%q, want %q", got, want)
			}

			// Ida y vuelta: la fuente escrita, armada como la arma quien la
			// usa (con sus sourceCapabilities), da la misma tabla.
			doc, diags := parser.New(util.NewNoop()).Parse(assembleFragment(r), "")
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("written fragment does not build: %+v\n%s", d, r.Source)
				}
			}
			if doc == nil || len(doc.ContentBlocks) != 1 {
				t.Fatalf("written fragment did not read back as one slide:\n%s", r.Source)
			}
			if got := onlyTable(t, doc.ContentBlocks[0].Elements).Rows; !reflect.DeepEqual(got, want) {
				t.Fatalf("read back rows=%q, want %q\n%s", got, want, r.Source)
			}
			if !reflect.DeepEqual(fragmentPayload(r.Elements), fragmentPayload(doc.ContentBlocks[0].Elements)) {
				t.Fatalf("written fragment reads back with another AST:\n%s", r.Source)
			}
			if c.rawBody {
				// Hoy el cuerpo de un ::: sale como está y la fila se vuelve
				// a completar al leerlo; no se exige que salga sin TABLE010.
				return
			}
			if again := shortRowDiagnostics(diags); len(again) != 0 {
				t.Fatalf("written fragment still has a short row: %+v\n%s", again, r.Source)
			}
			if strings.Contains(r.Source, "| p | q |\n") || strings.Contains(r.Source, `["p", "q"]`) {
				t.Fatalf("written fragment keeps the short row:\n%s", r.Source)
			}
		})
	}
}

// La API de dos pasos que documenta docs/content-markdown-fragments.md
// (ParseMarkdownContent y luego FormatContentFragment) completa igual que el
// codec, sin pasar por NormalizeMarkdownContent.
func TestParseMarkdownContentPadsShortRowsForFormatter(t *testing.T) {
	nodes, diags := parser.ParseMarkdownContent("| A | B | C |\n| --- | --- | --- |\n| x | y | z |\n| p | q |\n", parser.ContentOptions{AnchorNamespace: "short-rows-api"})
	if w := shortRowDiagnostics(diags); len(diags) != 1 || len(w) != 1 || w[0].Position.Line != 4 {
		t.Fatalf("want only TABLE010 on line 4, got %+v", diags)
	}
	if got := onlyTable(t, nodes).Rows; !reflect.DeepEqual(got, shortRowWant) {
		t.Fatalf("rows=%q, want %q", got, shortRowWant)
	}
	source, err := formatter.FormatContentFragment(nodes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, "| p | q |  |\n") {
		t.Fatalf("short row not written complete:\n%s", source)
	}
}

// Una fila con MÁS celdas que el encabezado no se completa ni se recorta: el
// codec la escribe tal cual, como antes de completar las cortas. El TABLE003
// de esa fila lo da el linter cuando se compila el documento armado.
func TestContentFragmentKeepsLongTableRows(t *testing.T) {
	r := NormalizeMarkdownContent("| A | B | C |\n| --- | --- | --- |\n| x | y | z |\n| p | q | r | s |\n", parser.ContentOptions{AnchorNamespace: "long-rows"})
	if !r.Receipt.Accepted {
		t.Fatalf("%+v", r.Receipt.Diagnostics)
	}
	if w := shortRowDiagnostics(r.Receipt.Diagnostics); len(w) != 0 {
		t.Fatalf("long row reported as short: %+v", w)
	}
	want := [][]string{{"x", "y", "z"}, {"p", "q", "r", "s"}}
	if got := onlyTable(t, r.Elements).Rows; !reflect.DeepEqual(got, want) {
		t.Fatalf("rows=%q, want %q", got, want)
	}
	if !strings.Contains(r.Source, "| p | q | r | s |\n") {
		t.Fatalf("long row changed:\n%s", r.Source)
	}
}

func shortRowDiagnostics(diags []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	for _, d := range diags {
		if d.RuleID == "TABLE010" {
			out = append(out, d)
		}
	}
	return out
}

func onlyTable(t *testing.T, nodes []ast.Element) *ast.TableElement {
	t.Helper()
	var tables []*ast.TableElement
	_ = ast.Walk(fragmentDocument(nodes), func(n ast.Node) error {
		if table, ok := n.(*ast.TableElement); ok {
			tables = append(tables, table)
		}
		return nil
	})
	if len(tables) != 1 {
		t.Fatalf("want one table, got %d", len(tables))
	}
	return tables[0]
}

func assembleFragment(r ContentFragmentResult) string {
	wrapper := "---\nmode: strict\n"
	if len(r.Receipt.SourceCapabilities) > 0 {
		wrapper += "ast_capabilities: [" + strings.Join(r.Receipt.SourceCapabilities, ", ") + "]\n"
	}
	return wrapper + "---\nSLIDE content\n" + r.Source
}
