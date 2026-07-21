// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// GridParser maneja bloques de grid layout con columnas anidadas
type GridParser struct{}

// CanParse determina si una línea es el inicio de un bloque grid.
//
// El dialecto strict usa la forma delimitada <<grid>> … <<end>> (con
// separadores <<column>>), consistente con los otros bloques delimitados de
// strict (<<map>>/<<chart>>/<<math>>); el dialecto flex usa la forma
// Markdown-extended "::: grid" / "::: column". Ver parseStrictGrid para la
// elección de sintaxis y su round-trip.
func (p *GridParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)
	if mode == "strict" {
		return trimmed == "<<grid>>"
	}
	return strings.HasPrefix(trimmed, "::: grid")
}

// Parse parsea un bloque grid con columnas anidadas
func (p *GridParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	if ctx.Mode == "strict" {
		return p.parseStrictGrid(ctx, startIndex)
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])
	consumed := 1

	// Verify it starts with ::: grid
	if !strings.HasPrefix(line, "::: grid") {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	// Create grid element
	gridElement := ast.NewGridElement(pos)
	var strayContent strings.Builder
	// Blank lines seen since the last stray-content line. Only flushed into
	// strayContent if another stray-content line follows (a paragraph break
	// within the loose prose); if a column/closing/SLIDE line follows instead,
	// they were just spacing around it and are discarded.
	pendingBlankLines := 0

	// Parse columns until closing ::: or end of file
	i := startIndex + 1
	for i < len(ctx.Lines) {
		line := ctx.Lines[i]
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == ":::" {
			// Check if this might be the grid closing or just column closing
			// Look ahead to see if there are more columns
			nextColumnFound := IsJustASeparator(ctx.Lines, i, "::: column", ":::", "SLIDE ")

			pendingBlankLines = 0
			if nextColumnFound {
				// This is just a column separator, continue
				consumed++
				i++
				continue
			}

			// This is the grid closing
			consumed++
			break
		}

		// Check if this is a column start
		if strings.HasPrefix(trimmedLine, "::: column") {
			pendingBlankLines = 0
			// Parse this column
			columnResult := p.parseColumn(ctx, i)
			if columnResult.Element != nil {
				if columnElement, ok := columnResult.Element.(*ast.ColumnElement); ok {
					gridElement.Columns = append(gridElement.Columns, *columnElement)
				}
			}
			consumed += columnResult.ConsumedLines
			i += columnResult.ConsumedLines // Move past this column
		} else if strings.HasPrefix(trimmedLine, "SLIDE ") {
			// Don't consume this line, let the main parser handle it
			break
		} else if trimmedLine == "" {
			pendingBlankLines++
			consumed++
			i++
		} else {
			// Content line outside any column: not structured as its own
			// element, but preserved in GridElement.Content instead of
			// silently discarded (issue #9 - lost prose in --format json).
			if strayContent.Len() > 0 {
				strayContent.WriteString(strings.Repeat("\n", pendingBlankLines+1))
			}
			pendingBlankLines = 0
			strayContent.WriteString(line)
			consumed++
			i++
		}
	}

	gridElement.Content = strayContent.String()

	return &ParseResult{
		Element:       gridElement,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// parseColumn parsea una columna individual dentro del grid
func (p *GridParser) parseColumn(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	consumed := 1

	// Collect column content until ::: or another column
	var content strings.Builder
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == ":::" ||
			strings.HasPrefix(trimmedLine, "::: column") ||
			strings.HasPrefix(trimmedLine, "SLIDE ") {
			// End of this column
			break
		}

		if content.Len() > 0 {
			content.WriteString("\n")
		}
		content.WriteString(line) // Use original line to preserve indentation
		consumed++
	}

	columnElement := ast.NewColumnElement(pos, content.String())

	return &ParseResult{
		Element:       columnElement,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// parseStrictGrid parsea la forma strict del grid:
//
//	<<grid>>
//	[prosa suelta opcional → GridElement.Content]
//	<<column>>
//	contenido de la columna 1 (crudo, dedentado)
//	<<column>>
//	contenido de la columna 2
//	<<end>>
//
// Elección de sintaxis (compromiso público, ver el PR): la forma delimitada
// por marcadores <<grid>>/<<column>>/<<end>> se eligió sobre una alternativa
// de palabra-clave-más-indentación (GRID / COLUMN, con el cuerpo indentado)
// porque las fronteras de columna y de grid quedan explícitas en vez de
// depender de la indentación — el contenido de una columna puede tener su
// propia indentación relativa (sub-puntos, código) sin que el parser tenga
// que adivinar dónde termina la columna. Es además el mismo estilo que los
// otros bloques delimitados de strict (<<map>>/<<chart>>/<<math>>).
//
// El contenido de cada columna se guarda como Content en bruto (igual que la
// forma flex "::: column"), no como Elements tipados: es lo que el renderer de
// slides consume (data.ConvertColumnsWithVariables lee column.Content) y lo que
// produce el parser flex, así que la forma strict round-trip-ea al MISMO AST
// que "::: grid" para la misma estructura lógica.
//
// Indentación: formatStrictElement indenta el elemento completo 2 espacios, así
// que las líneas de contenido llegan con esa sangría base. parseStrictGrid la
// quita (dedentByLeadingSpaces hasta baseIndent, la sangría del marcador
// <<grid>>), preservando cualquier indentación relativa más profunda. Esto es
// el inverso exacto de indent(), lo que hace el fmt idempotente (a diferencia
// del bug pre-existente de :::code-group, que guarda la sangría base verbatim y
// gana 2 espacios por pasada).
func (p *GridParser) parseStrictGrid(ctx *ParseContext, startIndex int) *ParseResult {
	pos := diagnostics.NewPosition(startIndex+1, 1)
	baseIndent := leadingSpaceCount(ctx.Lines[startIndex])

	gridElement := ast.NewGridElement(pos)
	consumed := 1 // la línea <<grid>>

	var stray []string
	var colLines []string
	var columns []ast.ColumnElement
	inColumn := false
	colPos := pos

	flushColumn := func() {
		if inColumn {
			col := ast.NewColumnElement(colPos, strings.Join(colLines, "\n"))
			columns = append(columns, *col)
			colLines = nil
			inColumn = false
		}
	}
	finish := func() *ParseResult {
		flushColumn()
		gridElement.Columns = columns
		gridElement.Content = strings.Join(stray, "\n")
		return &ParseResult{Element: gridElement, ConsumedLines: consumed}
	}

	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]
		trimmedLine := strings.TrimSpace(line)

		switch {
		case trimmedLine == "<<end>>":
			consumed++
			return finish()
		case isSlideBoundary(line):
			// Defensivo: un grid sin <<end>> (input malformado). No consumir
			// esta línea — dejar que el parser principal la maneje como el
			// inicio del siguiente slide. SOLO dispara en un marcador de slide
			// REAL (columna 0, sin sangría): una línea de CONTENIDO indentada
			// que casualmente empieza con "SLIDE " (p. ej. el texto
			// "SLIDE overview" dentro de una columna) NO es un límite — el grid
			// es <<end>>-delimitado, así que se preserva como contenido. Como
			// formatStrictElement siempre indenta el cuerpo del grid, ese texto
			// se re-emite indentado y nunca se confunde con un límite al
			// reparsear (round-trip estable).
			return finish()
		case trimmedLine == "<<column>>":
			flushColumn()
			inColumn = true
			colPos = diagnostics.NewPosition(i+1, 1)
			consumed++
		default:
			content := dedentByLeadingSpaces(line, baseIndent)
			if inColumn {
				colLines = append(colLines, content)
			} else {
				stray = append(stray, content)
			}
			consumed++
		}
	}

	// EOF sin <<end>>: cerrar con lo acumulado.
	return finish()
}

// isSlideBoundary reporta si line (SIN trim) es un marcador de slide de nivel
// superior — "SLIDE " en columna 0, sin sangría. Un slide real siempre está en
// la columna 0 (parser.StrictParser emite y despacha SLIDE ahí); una línea
// indentada que empieza con "SLIDE " es contenido de columna, no un límite. Se
// chequea sobre la línea cruda a propósito: es la sangría, no las palabras, lo
// que distingue un límite de un texto que casualmente dice "SLIDE …".
func isSlideBoundary(line string) bool {
	return strings.HasPrefix(line, "SLIDE ")
}

// leadingSpaceCount cuenta los espacios (solo ' ') al inicio de line. El
// formatter indenta con espacios (indent() en formatter/util.go usa
// strings.Repeat(" ", n)), así que contar espacios es el inverso correcto.
func leadingSpaceCount(line string) int {
	n := 0
	for n < len(line) && line[n] == ' ' {
		n++
	}
	return n
}

// dedentByLeadingSpaces quita HASTA n espacios iniciales de line (nunca más de
// los que hay, y solo espacios). "Hasta n" en vez de un corte fijo line[n:]
// para que una línea escrita a mano con menos sangría que la base no pierda un
// carácter real.
func dedentByLeadingSpaces(line string, n int) string {
	i := 0
	for i < len(line) && i < n && line[i] == ' ' {
		i++
	}
	return line[i:]
}
