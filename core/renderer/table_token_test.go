// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestTableTokenCellClass: solo una celda que es un ID corto (mayúsculas y
// dígitos con guion, hasta 12 caracteres) lleva la clase nowrap. Fechas,
// palabras con guion y URLs siguen partiéndose: con la regla anterior una
// tabla de ocho columnas de fechas se salía de la página. Los mismos casos
// están en doclang (TestDocxIsShortToken), que espeja esta regla.
func TestTableTokenCellClass(t *testing.T) {
	for cell, want := range map[string]bool{
		"OB-1":               true,
		" RF-12 ":            true,
		"NFR-SEG-3":          true,
		"NFR-SEG-12":         true,
		"**RF-12**":          true,
		"`US-A01`":           true,
		"A-B-C-D-E":          false,
		"ABCDEFGH-123":       true,
		"ABCDEFGHI-123":      false,
		"2026-11-02":         false,
		"2026-11-02/2026-12": false,
		"co-op":              false,
		"well-known":         false,
		"I/O":                false,
		"https://x.com/a-b":  false,
		"Rf-12":              false,
		"Alta":               false,
		"12":                 false,
		"":                   false,
		"RF-12 y RF-13":      false,
	} {
		if got := tableTokenCellClass(cell) != ""; got != want {
			t.Errorf("tableTokenCellClass(%q) = %v, want %v", cell, got, want)
		}
	}
}

// TestRenderTable_IDColumnIsNowrap: en la tabla del corpus (IDs cortos en la
// primera columna, texto largo en la segunda) los IDs salen con la clase y
// la descripción no; la hoja de estilos del documento tiene la regla.
func TestRenderTable_IDColumnIsNowrap(t *testing.T) {
	table := ast.NewTableElement(diagnostics.NewPosition(1, 1))
	table.Headers = []string{"ID", "Descripción"}
	table.Rows = [][]string{
		{"OB-1", "Reducir el tiempo de atención en ventanilla mediante la automatización del registro"},
		{"NFR-SEG-3", "Todas las comunicaciones deben ir cifradas con TLS 1.2 o superior"},
	}
	html := renderTableElement(table, nil, nil)
	for _, want := range []string{`<td class="cell-token">OB-1</td>`, `<td class="cell-token">NFR-SEG-3</td>`, `<td>Reducir`, `<th scope="col">ID</th>`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in %s", want, html)
		}
	}
	if !strings.Contains(generateDocumentStyles(DocumentHTMLOptions{}, nil), "td.cell-token") {
		t.Error("the document stylesheet has no rule for td.cell-token")
	}
}
