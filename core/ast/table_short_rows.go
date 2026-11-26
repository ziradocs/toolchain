// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import "go.ziradocs.com/core/v2/diagnostics"

// PadShortTableRows completa con celdas vacías cada fila de tabla de doc que
// cubre menos columnas que el encabezado, como hace GFM, y llama a padded con
// la tabla, el índice de la fila en Rows y cuántas columnas cubría, en orden
// (padded puede ser nil). Rows y, cuando deriva de ellas, Cells quedan con
// una celda por columna, así que todo generador (HTML, PDF, DOCX, Markdown,
// el JSON del AST) escribe la fila entera. Una fila sin celdas (la fila "|")
// queda como está, y una tabla con tableRows no se toca: su contrato de
// identidades ya la hace rectangular. Una fila con MÁS celdas que el
// encabezado tampoco se toca: es TABLE003.
//
// Una fila de Cells con celdas fusionadas (colSpan, o rowSpan de una fila de
// arriba) cubre las columnas que suman sus spans, no tantas como celdas
// tiene: [{M, colSpan: 3}] en una tabla de tres columnas está completa, y no
// se le agrega nada ni se avisa. A una que cubre menos se le agregan solo las
// columnas que faltan, y Rows se vuelve a derivar de Cells
// (FlattenCellsToRows), como en una tabla de celdas explícitas.
//
// La corren los dos caminos por los que un AST llega a los generadores: los
// parsers, sobre lo que leyeron (y avisan cada fila con TABLE010), y
// DecodeAST, sobre el AST que entrega un filtro externo o que se lee de
// JSON; ahí la fila queda anotada para que el linter dé TABLE010 (ver
// TableElement.DecodePaddedRows y CarryDecodePaddedRows).
func PadShortTableRows(doc *AST, padded func(table *TableElement, row, have int)) {
	_ = Walk(doc, func(n Node) error {
		table, ok := n.(*TableElement)
		if !ok || table.TableRows != nil {
			return nil
		}
		want := len(table.Headers)
		// Cells deriva de Headers/Rows (DeriveCellsFromFlat), o un filtro la
		// trae con spans: una fila de encabezado y una por fila, en el mismo
		// orden.
		if want > 0 && len(table.Cells) == len(table.Rows)+1 && cellsHaveSpans(table.Cells) {
			padSpannedRows(table, want, padded)
			return nil
		}
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

func cellsHaveSpans(cells [][]TableCell) bool {
	for _, row := range cells {
		for _, c := range row {
			if c.ColSpan > 1 || c.RowSpan > 1 {
				return true
			}
		}
	}
	return false
}

// padSpannedRows es PadShortTableRows para una tabla cuyas Cells traen
// spans: mide cada fila por las columnas que cubre (sus colSpan más las
// celdas de arriba que la alcanzan con rowSpan) y completa solo lo que falta.
func padSpannedRows(table *TableElement, want int, padded func(table *TableElement, row, have int)) {
	widths := cellRowWidths(table.Cells)
	for i := range table.Rows {
		r := i + 1
		have := widths[r]
		if len(table.Cells[r]) == 0 || have >= want {
			continue
		}
		table.Cells[r] = append(table.Cells[r], make([]TableCell, want-have)...)
		if padded != nil {
			padded(table, i, have)
		}
	}
	headers, rows := FlattenCellsToRows(table.Cells)
	if len(rows) != len(table.Rows) || !sameStringSlice(headers, table.Headers) {
		// Cells no da esta vista plana: se completa Rows sin más, para que
		// ninguna fila quede más corta que el encabezado.
		for i, row := range table.Rows {
			if len(row) > 0 && len(row) < want {
				full := make([]string, want)
				copy(full, row)
				table.Rows[i] = full
			}
		}
		return
	}
	// La vista plana se rehace de Cells, con o sin filas completadas: una
	// fila como ["M"] bajo [{M, colSpan: 3}] es la vista de esa celda, que
	// FlattenCellsToRows escribe en sus tres columnas.
	table.Rows = rows
}

// cellRowWidths son las columnas que cubre cada fila de cells: la suma de
// los colSpan de sus celdas más las de celdas de filas de arriba cuyo
// rowSpan llega hasta ella.
func cellRowWidths(cells [][]TableCell) []int {
	type carry struct{ cols, left int }
	var carries []carry
	widths := make([]int, len(cells))
	for r, row := range cells {
		w := 0
		var next []carry
		for _, c := range carries {
			w += c.cols
			if c.left > 1 {
				next = append(next, carry{c.cols, c.left - 1})
			}
		}
		for _, c := range row {
			cols := c.ColSpan
			if cols < 1 {
				cols = 1
			}
			w += cols
			if c.RowSpan > 1 {
				next = append(next, carry{cols, c.RowSpan - 1})
			}
		}
		widths[r] = w
		carries = next
	}
	return widths
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// DecodePaddedRows son las filas de t que DecodeAST completó con celdas
// vacías: índice en Rows → columnas que cubría. No viajan en el JSON: solo
// sirven para que el linter avise (TABLE010) las filas cortas de un AST que
// llegó de un filtro externo o de JSON, que ya no se ven cortas. Las que
// completa un parser ya las avisa el parser.
func (t *TableElement) DecodePaddedRows() map[int]int {
	return t.decodePadded
}

// CarryDecodePaddedRows pasa a to las filas que DecodeAST había completado
// en from (ver DecodePaddedRows), para que el aviso no se pierda en una
// cadena de filtros: cada filtro recibe el AST en JSON, donde la anotación
// no viaja, y el decode de su salida ya no ve la fila corta. Una tabla de to
// se empareja con la de from que tiene su misma posición, y una fila, por su
// índice si sigue existiendo y lleva la misma posición; una fila que to ya
// tiene anotada no se duplica.
func CarryDecodePaddedRows(from, to *AST) {
	type key struct{ line, column int }
	prev := map[key]*TableElement{}
	_ = Walk(from, func(n Node) error {
		if t, ok := n.(*TableElement); ok && len(t.decodePadded) > 0 {
			prev[key{t.Position.Line, t.Position.Column}] = t
		}
		return nil
	})
	if len(prev) == 0 {
		return
	}
	_ = Walk(to, func(n Node) error {
		t, ok := n.(*TableElement)
		if !ok {
			return nil
		}
		old := prev[key{t.Position.Line, t.Position.Column}]
		if old == nil {
			return nil
		}
		for row, have := range old.decodePadded {
			if row >= len(t.Rows) || rowPosition(old, row) != rowPosition(t, row) {
				continue
			}
			if _, done := t.decodePadded[row]; done {
				continue
			}
			if t.decodePadded == nil {
				t.decodePadded = map[int]int{}
			}
			t.decodePadded[row] = have
		}
		return nil
	})
}

func rowPosition(t *TableElement, row int) diagnostics.Position {
	if row < len(t.RowPositions) {
		return t.RowPositions[row]
	}
	return t.Position
}
