// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer_test

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// TestTableColumnAlign_HTML: la alineación por columna de una tabla
// Markdown (table-align-v1) llega al HTML en th y td, en los dos dialectos
// de documentos. La columna sin alineación declarada no lleva atributo. El
// HTML de documento es también el que imprime el PDF.
func TestTableColumnAlign_HTML(t *testing.T) {
	table := "| Concepto | Cantidad | Estado | Nota |\n" +
		"|:---|---:|:---:|---|\n" +
		"| Licencias | 1,200 | OK | x |\n"
	sources := map[string]string{
		"flex":   "---\ntitle: T\n---\n\n# Tabla\n\n" + table,
		"strict": "---\ntitle: T\nmode: strict\n---\n\nSECTION \"Tabla\"\n  " + strings.ReplaceAll(strings.TrimSuffix(table, "\n"), "\n", "\n  ") + "\n",
	}
	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "t.doclang")
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("parse error: %s", d.Message)
				}
			}
			html := renderer.GenerateDocumentHTML(doc, renderer.DocumentHTMLOptions{}, nil)
			for _, want := range []string{
				`<th scope="col" style="text-align: left">Concepto</th>`,
				`<th scope="col" style="text-align: right">Cantidad</th>`,
				`<th scope="col" style="text-align: center">Estado</th>`,
				`<th scope="col">Nota</th>`,
				`<td style="text-align: left">Licencias</td>`,
				`<td style="text-align: right">1,200</td>`,
				`<td style="text-align: center">OK</td>`,
				`<td>x</td>`,
			} {
				if !strings.Contains(html, want) {
					t.Errorf("falta %s en el HTML", want)
				}
			}
		})
	}
}

// TestTableColumnAlign_OnlyAllowlistedValues: un ColumnAlign que llega de
// un AST externo sin validar no se interpola tal cual en el atributo.
func TestTableColumnAlign_OnlyAllowlistedValues(t *testing.T) {
	table := ast.NewTableElement(diagnostics.NewPosition(1, 1))
	table.Headers = []string{"A", "B"}
	table.Rows = [][]string{{"1", "2"}}
	table.Cells = ast.DeriveCellsFromFlat(table.Headers, table.Rows)
	table.ColumnAlign = []string{`center" onmouseover="x`, "right"}

	html := renderer.RenderElementToHTML(table, nil, nil)
	if strings.Contains(html, "onmouseover") {
		t.Fatalf("un valor fuera de la lista llegó al HTML: %s", html)
	}
	if !strings.Contains(html, `<td style="text-align: right">2</td>`) || !strings.Contains(html, "<td>1</td>") {
		t.Errorf("HTML inesperado: %s", html)
	}
}
