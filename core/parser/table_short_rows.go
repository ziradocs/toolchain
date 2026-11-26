// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/elements"
)

// padShortTableRows completa con celdas vacías cada fila de tabla que tiene
// menos celdas que el encabezado, y avisa cada una (TABLE010, warning).
//
// Es lo que hace GFM con una fila corta. Antes la fila llegaba así al
// linter, que daba TABLE003 (error), y una sola fila corta hacía fallar el
// documento entero; en HTML, PDF, DOCX y Markdown además salía con menos
// celdas que el encabezado. Una fila con MÁS celdas que el encabezado sigue
// siendo TABLE003: GFM descarta el exceso, y eso perdería texto sin que el
// autor lo vea.
//
// Corre sobre el AST ya leído, después de las decisiones del lector de
// tablas (qué línea es el separador, qué filas entran, cuáles se descartan
// con TABLE007/TABLE008), que siguen tomándose con el número de celdas que
// escribió el autor: completar antes cambiaría qué líneas se lleva una
// tabla. Cubre todas las tablas que lee el parser, en los dos dialectos y en
// slides y documentos: Markdown con pipes, TABLE con headers:/rows:, y las
// que están dentro de una columna de grid o de un bloque especial. Una fila
// sin celdas (la fila "|") queda como está, igual que antes, y una tabla de
// celdas explícitas (cells: o tableRows:) ya sale rectangular de su lector.
func padShortTableRows(doc *ast.AST) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	_ = ast.Walk(doc, func(n ast.Node) error {
		table, ok := n.(*ast.TableElement)
		if !ok || table.TableRows != nil {
			return nil
		}
		want := len(table.Headers)
		// Cells deriva de Headers/Rows (ast.DeriveCellsFromFlat): una fila de
		// encabezado y una por fila, en el mismo orden.
		cellsFollowRows := want > 0 && len(table.Cells) == len(table.Rows)+1
		for i, row := range table.Rows {
			have := len(row)
			if have == 0 || have >= want {
				continue
			}
			padded := make([]string, want)
			copy(padded, row)
			table.Rows[i] = padded
			if cellsFollowRows && len(table.Cells[i+1]) == have {
				table.Cells[i+1] = append(table.Cells[i+1], make([]ast.TableCell, want-have)...)
			}
			pos := table.GetPosition()
			if i < len(table.RowPositions) {
				pos = table.RowPositions[i]
			}
			diags = append(diags, elements.ShortTableRowWarning(pos, have, want, "table-parser"))
		}
		return nil
	})
	return diags
}
