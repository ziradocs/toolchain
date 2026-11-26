// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

// PadShortTableRows completa con celdas vacías cada fila de tabla de doc que
// tiene menos celdas que el encabezado, como hace GFM, y llama a padded con
// la tabla, el índice de la fila en Rows y cuántas celdas tenía, en orden
// (padded puede ser nil). Rows y, cuando deriva de ellas, Cells quedan con
// una celda por columna, así que todo generador (HTML, PDF, DOCX, Markdown,
// el JSON del AST) escribe la fila entera. Una fila sin celdas (la fila "|")
// queda como está, y una tabla con tableRows no se toca: su contrato de
// identidades ya la hace rectangular. Una fila con MÁS celdas que el
// encabezado tampoco se toca: es TABLE003.
//
// La corren los dos caminos por los que un AST llega a los generadores: los
// parsers, sobre lo que leyeron (y avisan cada fila con TABLE010), y
// DecodeAST, sobre el AST que entrega un filtro externo o que se lee de
// JSON; ahí la fila queda anotada para que el linter dé TABLE010 (ver
// TableElement.DecodePaddedRows).
func PadShortTableRows(doc *AST, padded func(table *TableElement, row, have int)) {
	_ = Walk(doc, func(n Node) error {
		table, ok := n.(*TableElement)
		if !ok || table.TableRows != nil {
			return nil
		}
		want := len(table.Headers)
		// Cells deriva de Headers/Rows (DeriveCellsFromFlat): una fila de
		// encabezado y una por fila, en el mismo orden.
		cellsFollowRows := want > 0 && len(table.Cells) == len(table.Rows)+1
		for i, row := range table.Rows {
			have := len(row)
			if have == 0 || have >= want {
				continue
			}
			full := make([]string, want)
			copy(full, row)
			table.Rows[i] = full
			if cellsFollowRows && len(table.Cells[i+1]) == have {
				table.Cells[i+1] = append(table.Cells[i+1], make([]TableCell, want-have)...)
			}
			if padded != nil {
				padded(table, i, have)
			}
		}
		return nil
	})
}

// DecodePaddedRows son las filas de t que DecodeAST completó con celdas
// vacías: índice en Rows → celdas que tenía. No viajan en el JSON: solo
// sirven para que el linter avise (TABLE010) las filas cortas de un AST que
// llegó de un filtro externo o de JSON, que ya no se ven cortas. Las que
// completa un parser ya las avisa el parser.
func (t *TableElement) DecodePaddedRows() map[int]int {
	return t.decodePadded
}
