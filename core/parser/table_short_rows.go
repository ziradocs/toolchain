// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/elements"
)

// padShortTableRows completa con celdas vacías cada fila de tabla que tiene
// menos celdas que el encabezado (ast.PadShortTableRows) y avisa cada una
// (TABLE010, warning).
//
// Es lo que hace GFM con una fila corta. Antes la fila llegaba así al
// linter, que daba TABLE003 (error), y una sola fila corta hacía fallar el
// documento entero; en HTML, PDF, DOCX y Markdown además salía con menos
// celdas que el encabezado. Una fila con MÁS celdas que el encabezado sigue
// siendo TABLE003: GFM descarta el exceso, y eso perdería texto sin que el
// autor lo vea.
//
// La corre cada parser de cuerpo al terminar (FlexParser, StrictParser,
// DocumentFlexParser, DocumentStrictParser), así que también las entradas
// públicas NewFlexParser, NewStrictParser y NewDocument*Parser completan.
// ParseMarkdownContent, que lee un fragmento con el registry sin abrir un
// parser de cuerpo, la corre sobre sus elementos al terminar.
// Es idempotente: una segunda pasada no encuentra filas cortas. Corre sobre
// el AST ya leído, después de las decisiones del lector de tablas (qué
// línea es el separador, qué filas entran, cuáles se descartan con
// TABLE007/TABLE008), que siguen tomándose con el número de celdas que
// escribió el autor: completar antes cambiaría qué líneas se lleva una
// tabla.
func padShortTableRows(doc *ast.AST) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	ast.PadShortTableRows(doc, func(table *ast.TableElement, row, have int) {
		pos := table.GetPosition()
		if row < len(table.RowPositions) {
			pos = table.RowPositions[row]
		}
		diags = append(diags, elements.ShortTableRowWarning(pos, have, len(table.Headers), "table-parser"))
	})
	return diags
}
