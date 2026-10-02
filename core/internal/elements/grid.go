// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
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

	pos := ctx.Position(startIndex)
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
	var typedDiagnostics []diagnostics.Diagnostic
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
			var columnResult *ParseResult
			if isFlexTypedColumnMarker(trimmedLine) {
				columnResult = p.parseTypedColumnFlex(ctx, i)
				typedDiagnostics = append(typedDiagnostics, columnResult.Diagnostics...)
			} else {
				columnResult = p.parseColumn(ctx, i)
			}
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
		Diagnostics:   typedDiagnostics,
	}
}

// isFlexTypedColumnMarker reporta si trimmedLine abre una columna flex
// tipada (`::: column typed`, issue #373). Es una coincidencia exacta de tres
// palabras: `::: column` con cualquier otro sufijo sigue siendo una columna
// cruda, como siempre (CanParse y el despacho de columnas son de prefijo).
func isFlexTypedColumnMarker(trimmedLine string) bool {
	fields := strings.Fields(trimmedLine)
	return len(fields) == 3 && fields[0] == ":::" && fields[1] == "column" && fields[2] == "typed"
}

// isStrictTypedColumnMarker es el equivalente strict: la línea completa,
// recortada, es `<<column typed>>`.
func isStrictTypedColumnMarker(trimmedLine string) bool {
	return trimmedLine == "<<column typed>>"
}

// parseTypedColumnFlex parsea una columna `::: column typed` (issue #373):
// su cuerpo se reparte entre los parsers de elemento del registry flex, el
// mismo bucle que usa el cuerpo de un slide (TextParser es el respaldo), y el
// resultado llena ColumnElement.Elements en vez de Content.
//
// El cuerpo termina en `:::`, en la siguiente `::: column` o en una frontera
// de slide strict ("SLIDE "/"SECTION " en columna 0), los mismos cortes que una
// columna cruda. A propósito NO corta en `# `, `## ` ni `---`: cortar ahí
// sacaba el resto de la columna hacia la prosa suelta del grid sin ningún
// diagnóstico, y una columna cruda sí los conserva dentro. Como cada elemento consume su propio
// bloque completo, un `:::` que le pertenece a un bloque especial o a una
// valla de código se lo traga ese elemento y nunca se lee como cierre de la
// columna. No se tipan los encabezados (`###`): el registry no tiene
// HeadingParser y quedan como TextElement, igual que un TEXT strict con esa
// línea. Un grid anidado es un error, ver parseTypedColumnBody en strict.
func (p *GridParser) parseTypedColumnFlex(ctx *ParseContext, startIndex int) *ParseResult {
	pos := ctx.Position(startIndex)
	column := ast.NewColumnElement(pos, "")
	registry := GetDefaultRegistry()
	var diags []diagnostics.Diagnostic

	consumed := 1
	i := startIndex + 1
	for i < len(ctx.Lines) {
		line := ctx.Lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == ":::" ||
			strings.HasPrefix(trimmed, "::: column") ||
			IsStrictBlockBoundary(line) {
			break
		}
		if trimmed == "" {
			consumed++
			i++
			continue
		}
		if strings.HasPrefix(trimmed, "::: grid") {
			diags = append(diags, diagnostics.NewError(
				"a ::: grid cannot be nested inside a typed column", ctx.Position(i), "parser"))
			consumed++
			i++
			continue
		}

		ctx.CurrentLine = i
		result := registry.Parse(ctx, i)
		diags = append(diags, result.Diagnostics...)
		if result.Error != nil {
			diags = append(diags, diagnostics.NewError(result.Error.Error(), ctx.Position(i), "parser"))
		}
		if result.Element != nil && result.ConsumedLines > 0 {
			column.Elements = append(column.Elements, result.Element)
			consumed += result.ConsumedLines
			i += result.ConsumedLines
			continue
		}
		if result.ConsumedLines > 0 {
			consumed += result.ConsumedLines
			i += result.ConsumedLines
			continue
		}
		// "# " y "## " no los reclama ningún parser del registry (a nivel de
		// slide los intercepta el bucle de flex, que aquí no corre): dentro de
		// una columna son prosa, igual que un TEXT strict con esa línea. Cada
		// una queda como su propio TextElement para no perderla ni
		// fusionarla con la prosa vecina.
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") {
			column.Elements = append(column.Elements, ast.NewTextElement(ctx.Position(i), trimmed))
			consumed++
			i++
			continue
		}
		// Failsafe: ningún parser reclamó la línea. Se avisa en vez de
		// descartarla en silencio (misma regla que el bucle de un slide).
		diags = append(diags, diagnostics.NewWarning(
			"Unrecognized line, content was discarded: "+`"`+trimmed+`"`+". Check DSL Flex syntax documentation.",
			ctx.Position(i), "parser").WithRuleID("FLEX001"))
		consumed++
		i++
	}

	return &ParseResult{Element: column, ConsumedLines: consumed, Diagnostics: diags}
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

	pos := ctx.Position(startIndex)
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
// Columna tipada (issue #373): `<<column typed>>` abre una columna cuyo cuerpo,
// sangrado bajo el marcador, se parsea con la gramática de un SLIDE y llena
// Elements en vez de Content. La forma cruda no cambia y las dos conviven en el
// mismo grid. El cuerpo se delimita por sangría, no por `<<end>>`; ver
// docs/portable-typed-columns.md.
//
// Indentación: formatStrictElement indenta el elemento completo 2 espacios, así
// que las líneas de contenido llegan con esa sangría base. parseStrictGrid la
// quita (dedentByLeadingSpaces hasta baseIndent, la sangría del marcador
// <<grid>>), preservando cualquier indentación relativa más profunda. Esto es
// el inverso exacto de indent(), lo que hace el fmt idempotente (a diferencia
// del bug pre-existente de :::code-group, que guarda la sangría base verbatim y
// gana 2 espacios por pasada).
func (p *GridParser) parseStrictGrid(ctx *ParseContext, startIndex int) *ParseResult {
	pos := ctx.Position(startIndex)
	baseIndent := leadingSpaceCount(ctx.Lines[startIndex])

	gridElement := ast.NewGridElement(pos)
	consumed := 1 // la línea <<grid>>

	var stray []string
	var colLines []string
	var columns []ast.ColumnElement
	var diags []diagnostics.Diagnostic
	inColumn := false
	colPos := pos
	// afterTyped es true entre el cuerpo de una columna tipada y el siguiente
	// marcador: una línea a la sangría del grid ahí no es prosa suelta (eso
	// sólo existe antes de la primera columna) sino un cuerpo mal sangrado.
	afterTyped := false

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
		return &ParseResult{Element: gridElement, ConsumedLines: consumed, Diagnostics: diags}
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
			afterTyped = false
			colPos = ctx.Position(i)
			consumed++
		case isStrictTypedColumnMarker(trimmedLine):
			flushColumn()
			afterTyped = true
			// El cuerpo es la tira de líneas más sangradas que el marcador,
			// con las en blanco finales fuera: son separación, no contenido.
			// Delimitarlo por sangría (y no por la primera `<<end>>`) es lo
			// que deja que un chart o un quiz dentro de la columna cierre
			// con su propia `<<end>>`.
			end := i + 1
			for j := i + 1; j < len(ctx.Lines); j++ {
				if strings.TrimSpace(ctx.Lines[j]) == "" {
					continue
				}
				if leadingSpaceCount(ctx.Lines[j]) <= baseIndent {
					break
				}
				end = j + 1
			}
			col := ast.NewColumnElement(ctx.Position(i), "")
			if ctx.TypedColumnBody == nil {
				diags = append(diags, diagnostics.NewError(
					"typed columns are not supported in this context", ctx.Position(i), "parser"))
			} else {
				body := make([]string, 0, end-i-1)
				for _, bl := range ctx.Lines[i+1 : end] {
					body = append(body, dedentByLeadingSpaces(bl, baseIndent))
				}
				els, bodyDiags := ctx.TypedColumnBody(body, ctx.LineOffset+i+1)
				col.Elements = els
				diags = append(diags, bodyDiags...)
			}
			columns = append(columns, *col)
			consumed += end - i
			i = end - 1
		default:
			if !inColumn && afterTyped {
				diags = append(diags, diagnostics.NewError(
					"the body of a typed column must be indented under its <<column typed>> marker",
					ctx.Position(i), "parser"))
				consumed++
				continue
			}
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

// isSlideBoundary reporta si line (SIN trim) es un marcador de bloque strict
// de nivel superior — "SLIDE " (slidelang) o "SECTION " (doclang) en columna
// 0, sin sangría. Un bloque real siempre está en la columna 0 (StrictParser y
// DocumentStrictParser emiten y despachan ahí); una línea indentada que
// casualmente empieza con esas palabras es contenido de columna, no un
// límite. Delega en IsStrictBlockBoundary — ver ahí el porqué se chequea la
// línea cruda.
func isSlideBoundary(line string) bool {
	return IsStrictBlockBoundary(line)
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
