// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

// DeriveCellsFromFlat construye TableElement.Cells a partir de la vista plana
// Headers/Rows que todo parser de tablas ya produce hoy (markdown pipe, YAML
// simple, strict). El primer row (si hay headers) se marca IsHeader+Scope="col"
// (mismo criterio que ya aplica renderTableElement al emitir `<th scope="col">`
// para cada header); las filas de cuerpo quedan como celdas simples sin span.
// Es la mitad "simple" del seam de issue #20 — la mitad "fusionada" es
// FlattenCellsToRows, usada cuando el autor declara `cells:` explícito.
func DeriveCellsFromFlat(headers []string, rows [][]string) [][]TableCell {
	if len(headers) == 0 && len(rows) == 0 {
		return [][]TableCell{}
	}

	cells := make([][]TableCell, 0, len(rows)+1)

	if len(headers) > 0 {
		headerRow := make([]TableCell, len(headers))
		for i, h := range headers {
			headerRow[i] = TableCell{Content: h, IsHeader: true, Scope: "col"}
		}
		cells = append(cells, headerRow)
	}

	for _, row := range rows {
		bodyRow := make([]TableCell, len(row))
		for i, c := range row {
			bodyRow[i] = TableCell{Content: c}
		}
		cells = append(cells, bodyRow)
	}

	return cells
}

// cellCarry rastrea una celda con RowSpan>1 que sigue ocupando una columna en
// las filas siguientes a la que la declaró.
type cellCarry struct {
	content  string
	isHeader bool
	left     int // filas restantes (incluida la actual) que esta celda ocupa
}

// FlattenCellsToRows deriva la vista plana Headers/Rows a partir de Cells
// cuando el autor declaró celdas fusionadas explícitas (bloque YAML `cells:`,
// issue #20). La derivación expande cada colspan/rowspan repitiendo el
// contenido de la celda en cada slot que cubre, produciendo una grilla
// RECTANGULAR (todas las filas con el mismo ancho) — esto es deliberado: es
// lo que evita que linter.ElementStructureRule (TABLE003, severidad Error)
// reporte un falso positivo por "número de columnas inconsistente" en una
// tabla con celdas fusionadas legítima, ya que TABLE003 compara
// len(row) contra len(Headers) sin ningún concepto de span.
//
// Limitación conocida y aceptada: si una especificación de celdas se solapa
// (dos celdas reclamando el mismo slot vía colspan/rowspan cruzados) el
// resultado de esta función puede no reflejar un layout HTML válido — Cells
// en sí (la fuente de verdad) preserva los valores declarados sin corromper
// nada; solo la vista plana derivada puede quedar imprecisa en ese caso límite.
func FlattenCellsToRows(cells [][]TableCell) ([]string, [][]string) {
	if len(cells) == 0 {
		return []string{}, [][]string{}
	}

	carries := map[int]*cellCarry{}
	var grid [][]string
	maxWidth := 0

	for _, srcRow := range cells {
		var outRow []string
		col := 0
		srcIdx := 0

		for srcIdx < len(srcRow) || carryExistsAtOrBeyond(carries, col) {
			if c, ok := carries[col]; ok && c.left > 0 {
				outRow = append(outRow, c.content)
				col++
				continue
			}
			if srcIdx >= len(srcRow) {
				break
			}

			cell := srcRow[srcIdx]
			span := cell.ColSpan
			if span < 1 {
				span = 1
			}
			rspan := cell.RowSpan
			if rspan < 1 {
				rspan = 1
			}
			for k := 0; k < span; k++ {
				outRow = append(outRow, cell.Content)
				if rspan > 1 {
					carries[col+k] = &cellCarry{content: cell.Content, isHeader: cell.IsHeader, left: rspan}
				}
			}
			col += span
			srcIdx++
		}

		// Esta fila "consume" un turno de cada celda con rowspan activo,
		// incluidas las recién creadas arriba (left arrancó en rspan, que
		// cuenta la fila actual).
		for c, oc := range carries {
			oc.left--
			if oc.left <= 0 {
				delete(carries, c)
			}
		}

		if len(outRow) > maxWidth {
			maxWidth = len(outRow)
		}
		grid = append(grid, outRow)
	}

	for i := range grid {
		for len(grid[i]) < maxWidth {
			grid[i] = append(grid[i], "")
		}
	}

	firstRowAllHeader := len(cells[0]) > 0
	for _, c := range cells[0] {
		if !c.IsHeader {
			firstRowAllHeader = false
			break
		}
	}

	if firstRowAllHeader {
		return grid[0], grid[1:]
	}
	return []string{}, grid
}

func carryExistsAtOrBeyond(carries map[int]*cellCarry, col int) bool {
	for c := range carries {
		if c >= col {
			return true
		}
	}
	return false
}
