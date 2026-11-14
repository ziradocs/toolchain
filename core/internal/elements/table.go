// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/codespan"
)

// TableParser maneja elementos de tabla (YAML y Markdown)
type TableParser struct{}

// CanParse determina si una línea es el inicio de una tabla
func (p *TableParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// TABLE keyword: recognized in both strict and flex. doclang ONLY parses
	// flex (DocumentFlexParser.parseSectionContent sets ctx.Mode = "flex"),
	// so a TABLE block limited to strict was unauthorable in doclang —
	// needed so the explicit merged-cell `cells:` syntax (issue #20) exists
	// in the CLI the enterprise/A11Y rulepack actually consumes. Before this
	// change, a TABLE block in flex fell through to the TextParser fallback
	// and rendered as literal text.
	//
	// Exact-token match (trimmed == "TABLE", not HasPrefix): widening this to
	// flex means it now runs over ordinary doclang prose too, where a line
	// can legitimately start with the word "TABLE" without being the block
	// keyword (e.g. "TABLE 3.1: Resultados" or "TABLE OF CONTENTS" as a
	// heading) — HasPrefix would swallow that line as a table start and
	// drop it as content. Every real TABLE block (tests, examples) is the
	// bare token with nothing else on the line, so exact match is safe.
	if (mode == "strict" || mode == "flex") && trimmed == "TABLE" {
		return true
	}

	// Both modes: Markdown table format. Requires a leading "|" (not just
	// 2+ pipes anywhere) — without this, TableParser runs before
	// Quote/Checklist/Points/Text in the registry (GetDefaultRegistry) and
	// steals legitimate lines with 2+ pipes that aren't tables, e.g.
	// "- Compara pandas | numpy | scipy" (issue #245). Every real markdown
	// table in the corpus starts each row with "|", and strict already
	// requires the same (parser/strict.go). CanParse only sees one line (it
	// can't check whether the next one is a separator row "|---|"), so a
	// leading "|" is the only viable discriminator here.
	if strings.HasPrefix(trimmed, "|") && strings.Count(trimmed, "|") >= 2 {
		return true
	}

	return false
}

// Parse parsea una tabla desde las líneas proporcionadas
func (p *TableParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := ctx.Position(startIndex)
	table := ast.NewTableElement(pos)
	consumed := 0
	line := strings.TrimSpace(ctx.Lines[startIndex])
	var diags []diagnostics.Diagnostic

	// The YAML "TABLE" block is recognized the same way in strict and flex
	// (see CanParse); mode no longer gates this branch, only the line's syntax.
	if strings.HasPrefix(line, "TABLE") {
		// El texto de la propia línea TABLE no se lee (STRICT004). En flex
		// CanParse ya exige la palabra sola, así que esto solo dispara en
		// strict.
		if d := keywordLineRestWarning(ctx, startIndex, "TABLE", "table-parser"); d != nil {
			diags = append(diags, *d)
		}
		consumed++
		startIndex++

		// Parse YAML-style table
		headers, rows, caption, label, cellsExplicit, tableRows, rowPositions, yamlDiags, yamlConsumed := p.parseYAMLTable(ctx, startIndex, pos)
		table.Caption = caption
		table.Label = label
		consumed += yamlConsumed
		diags = append(diags, yamlDiags...)

		if tableRows != nil {
			table.TableRows = tableRows
			width := 0
			for _, cell := range tableRows[0].Cells {
				span := cell.ColSpan
				if span < 1 {
					span = 1
				}
				if span > ast.MaxCellSpan-width {
					width = ast.MaxCellSpan + 1
					break
				}
				width += span
			}
			if tableRows[0].Cells == nil {
				diags = append(diags, diagnostics.NewError("table row must declare cells", pos, "table-parser"))
			} else if width == 0 || width > ast.MaxCellSpan || len(tableRows) > 1_000_000/width {
				diags = append(diags, diagnostics.NewError("tableRows grid exceeds the supported size or has no columns", pos, "table-parser"))
			} else {
				table.SyncTableViews()
				if err := ast.ValidateTableRows(table); err != nil {
					diags = append(diags, diagnostics.NewError(err.Error(), pos, "table-parser"))
				}
			}
		} else if len(cellsExplicit) > 0 {
			// Explicit merged cells (issue #20): Cells is the source of
			// truth; Headers/Rows are DERIVED from Cells (rectangular grid)
			// so linter.ElementStructureRule (TABLE003) doesn't report a
			// false positive of "inconsistent columns" over a table with
			// colspan/rowspan. No hay una fila-fuente 1:1 que le corresponda
			// a cada fila derivada, así que RowPositions se deja vacío para
			// esta forma — TABLE003 no debería dispararse acá de todos modos.
			table.Cells = cellsExplicit
			table.Headers, table.Rows = ast.FlattenCellsToRows(cellsExplicit)
		} else {
			table.Headers = headers
			table.Rows = rows
			table.RowPositions = rowPositions
			table.Cells = ast.DeriveCellsFromFlat(headers, rows)
		}
	} else {
		// Parse Markdown-style table
		headers, rows, rowPositions, columnAlign, markdownConsumed, markdownDiags := p.parseMarkdownTable(ctx, startIndex)
		diags = append(diags, markdownDiags...)
		table.ColumnAlign = columnAlign
		table.Headers = headers
		table.Rows = rows
		table.RowPositions = rowPositions
		table.Cells = ast.DeriveCellsFromFlat(headers, rows)
		consumed = markdownConsumed
	}

	return &ParseResult{
		Element:       table,
		ConsumedLines: consumed,
		Error:         nil,
		Diagnostics:   diags,
	}
}

// parseYAMLTable parsea una tabla en formato YAML (bloque "TABLE"). El
// quinto valor de retorno es no-nil solo si el bloque declaró "cells:"
// explícito (celdas fusionadas, issue #20) — en ese caso el llamador debe
// tratar Cells como fuente de verdad y derivar Headers/Rows de él (ver
// ast.FlattenCellsToRows), ignorando los headers/rows acumulados acá (que
// para un bloque "cells:" quedan vacíos, ya que esa sintaxis no declara
// headers:/rows: por separado).
func (p *TableParser) parseYAMLTable(ctx *ParseContext, startIndex int, pos diagnostics.Position) ([]string, [][]string, string, string, [][]ast.TableCell, []ast.TableRow, []diagnostics.Position, []diagnostics.Diagnostic, int) {
	// Initialized as empty slices (not nil): Headers/Rows have no omitempty
	// in the AST, so a nil value would serialize as JSON null instead of []
	// (issue #8 - violates the contract's JSON Schema).
	headers := []string{}
	rows := [][]string{}
	var rowPositions []diagnostics.Position
	var caption string
	var label string
	// Named differently from "cells" on purpose: the "|" fallback branch
	// below already declares a local variable "cells" ([]string, one row
	// split by pipes) unrelated to this one — same name in nested scopes
	// would still compile (shadowing), but would confuse a reader.
	var explicitCells [][]ast.TableCell
	var tableRows []ast.TableRow
	var legacyDeclared, newDeclared bool
	// delimiterSeen: the markdown delimiter row (|---|---|) after a pipe
	// header has already been skipped. Only that one row is syntax.
	var delimiterSeen bool
	var diags []diagnostics.Diagnostic
	var inlineGroups []inlineRowsGroup
	var gated []gatedRow
	consumed := 0
	expectedIndent := -1 // Auto-detect indentation level
	lines := ctx.Lines
	// Un caption: o label: repetido pisa al anterior; el anterior se avisa
	// (TABLE008).
	repeated := newRepeatedKeys(ctx, TableLineRuleID, "table-parser", "TABLE")

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		currentIndent := CalculateIndentLevel(line)
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines
		if trimmedLine == "" {
			consumed++
			continue
		}

		// Auto-detect expected indentation from first non-empty line
		if expectedIndent == -1 && currentIndent > 0 {
			expectedIndent = currentIndent
		}

		// If we haven't detected indentation yet and line has no indentation, break
		if expectedIndent == -1 && currentIndent == 0 {
			break
		}

		// Check if this line should be part of the table block
		if expectedIndent > 0 && currentIndent < expectedIndent {
			break
		}

		// Parse YAML table properties
		if strings.HasPrefix(trimmedLine, "headers:") {
			legacyDeclared = true
			headersStr := strings.TrimPrefix(trimmedLine, "headers:")
			headersStr = strings.TrimSpace(headersStr)

			// Remove brackets and parse array
			if strings.HasPrefix(headersStr, "[") && strings.HasSuffix(headersStr, "]") {
				headersStr = trimInlineArrayBrackets(headersStr)
				headers = append(headers, splitInlineArray(headersStr)...)
			} else if headersStr != "" {
				diags = append(diags, ignoredLineWarning(ctx, i, TableLineRuleID, "table-parser",
					"The headers: value on this line is not a [\"…\", \"…\"] array and was ignored: %q. Write the headers as an array, as in headers: [\"Name\", \"Role\"].",
					trimmedLine))
			}
		} else if strings.HasPrefix(trimmedLine, "rows:") {
			legacyDeclared = true
			rowsKeyIndent := currentIndent
			inline := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "rows:"))
			// multiLine: el valor es un arreglo en línea partido en varias
			// líneas ("rows: [" sola, o "rows: [[…],"), cerrado por un "]"
			// en su propia línea o al final de la última fila.
			multiLine := inline == "[" || strings.HasPrefix(inline, "[") && strings.HasPrefix(strings.TrimSpace(inline[1:]), "#")
			// `rows: [[…], […]]` en la misma línea (issue #488): antes el
			// valor se descartaba sin aviso. Solo un arreglo anidado se lee
			// acá; cualquier otro valor (`rows: []`, `rows: # …`) sigue el
			// camino de siempre, y las filas de las líneas de abajo se leen
			// igual que antes.
			if isNestedInlineArray(inline) {
				value := inline
				if inlineArrayDepth(inline) == 1 {
					// "rows: [[…], […]," con más filas abajo: se leen las de
					// esta línea como si el arreglo cerrara acá.
					multiLine = true
					value = strings.TrimSuffix(strings.TrimSpace(inline), ",") + "]"
				}
				parsed, quote, ok := parseInlineRows(value)
				switch {
				case ok:
					inlineGroups = append(inlineGroups, inlineRowsGroup{start: len(rows), n: len(parsed), pos: ctx.Position(i)})
					for _, r := range parsed {
						rows = append(rows, r)
						rowPositions = append(rowPositions, ctx.Position(i))
					}
				case quote == inlineSingleQuotes:
					diags = append(diags, diagnostics.NewWarning(
						"The rows: value on this line uses single quotes and was ignored: strict cells are quoted with double quotes, as in [[\"a\", \"b\"], [\"c\", \"d\"]]",
						ctx.Position(i), "table-parser").WithRuleID(TableRowsRuleID))
				case quote == inlineApostrophe:
					diags = append(diags, diagnostics.NewWarning(
						"The rows: value on this line was ignored: a cell starts or ends with an apostrophe, and without double quotes it is not clear where that cell ends. Write that cell in double quotes, as in [\"'90s\", \"x\"]",
						ctx.Position(i), "table-parser").WithRuleID(TableRowsRuleID))
				default:
					diags = append(diags, diagnostics.NewWarning(
						"The rows: value on this line could not be read as rows and was ignored: write each row as a [\"…\", \"…\"] array with every cell in double quotes or without quotes, or put each row on its own line below rows:",
						ctx.Position(i), "table-parser").WithRuleID(TableRowsRuleID))
				}
			}
			// Las filas de abajo. Una fila es una línea que empieza con "[",
			// con más sangría que rows: y con la misma que la primera fila.
			// Hasta v2.58.0 solo se leía una fila que empezara exactamente
			// a seis espacios del comienzo de la línea, y la primera que no
			// lo hiciera cortaba el bloque; esas filas se siguen leyendo
			// igual (también con rows: a seis espacios o más). Una fila que
			// antes no se leía entra solo si cabe en la tabla (ver
			// gatedRow): si no, el lint (TABLE003, un error) haría fallar un
			// documento que compilaba.
			consumed++
			i++
			blockIndent := -1
			readBefore := true // la fila la leía v2.58.0
			for i < len(lines) {
				rowLine := lines[i]
				rowTrimmed := strings.TrimSpace(rowLine)
				rowIndent := CalculateIndentLevel(rowLine)
				legacyRow := strings.HasPrefix(rowLine, "      [")
				if !strings.HasPrefix(rowTrimmed, "[") || (!legacyRow && rowIndent <= rowsKeyIndent) {
					i-- // Back up one line
					break
				}
				if !legacyRow {
					readBefore = false
				}
				if blockIndent == -1 {
					blockIndent = rowIndent
				} else if rowIndent != blockIndent {
					diags = append(diags, ignoredLineWarning(ctx, i, TableLineRuleID, "table-parser",
						"This row under rows: is indented differently from the first row, so it was ignored: %q. Indent every row under rows: like the first one.",
						rowTrimmed))
					readBefore = false
					consumed++
					i++
					continue
				}
				closes := false
				if multiLine {
					// La última fila de un "rows: [" de varias líneas puede
					// cerrar también el arreglo: "[…]]". Ese "]" de más es
					// sintaxis, no parte de la última celda.
					body := strings.TrimSuffix(rowTrimmed, ",")
					if inlineArrayDepth(body) == -1 && strings.HasSuffix(body, "]") {
						rowTrimmed = strings.TrimSuffix(body, "]")
						closes = true
					}
				}
				if strings.HasPrefix(rowTrimmed, "[") && strings.HasSuffix(rowTrimmed, "],") {
					rowTrimmed = strings.TrimSuffix(rowTrimmed, ",")
				}
				if strings.HasPrefix(rowTrimmed, "[") && strings.HasSuffix(rowTrimmed, "]") {
					rowStr := trimInlineArrayBrackets(rowTrimmed)
					if !readBefore {
						gated = append(gated, gatedRow{index: len(rows), line: i})
					}
					rows = append(rows, splitInlineArray(rowStr))
					rowPositions = append(rowPositions, ctx.Position(i))
				} else {
					diags = append(diags, ignoredLineWarning(ctx, i, TableLineRuleID, "table-parser",
						"This row of the TABLE does not end with \"]\" and was ignored: %q. Write each row under rows: as a complete [\"…\", \"…\"] array on one line.",
						rowTrimmed))
				}
				consumed++
				i++
				if closes {
					i-- // el for de afuera avanza a la línea siguiente
					break
				}
			}
			continue
		} else if strings.HasPrefix(trimmedLine, "cells:") {
			legacyDeclared = true
			// Explicit merged cells (issue #20): the value is a YAML
			// sequence of cell rows, indented under "cells:". Collect the
			// lines that belong to it, then parse them as full YAML — more
			// robust than the hand-rolled string splitting headers:/rows:
			// use above, needed because here the value is a nested
			// structure (a list of lists of maps), not a flat array.
			//
			// A sequence item is still part of this value either when it's
			// indented past "cells:" itself, OR when it sits at the exact
			// same indentation and starts with "-" — the idiomatic YAML
			// style for a block sequence under a mapping key
			// (`cells:\n- [...]`). Treating same-indent-without-"-" as the
			// end of the block (the pre-fix behavior) silently produced an
			// empty cellsBlockLines for that idiomatic style, and thus a
			// silently empty table.
			cellsIndent := currentIndent
			consumed++
			i++
			cellsStart := i
			var cellsBlockLines []string
			for i < len(lines) {
				rl := lines[i]
				rlTrimmed := strings.TrimSpace(rl)
				if rlTrimmed == "" {
					cellsBlockLines = append(cellsBlockLines, rl)
					consumed++
					i++
					continue
				}
				rlIndent := CalculateIndentLevel(rl)
				if rlIndent < cellsIndent || (rlIndent == cellsIndent && !strings.HasPrefix(rlTrimmed, "-")) {
					i-- // Let the outer loop reprocess this line
					break
				}
				cellsBlockLines = append(cellsBlockLines, rl)
				consumed++
				i++
			}
			if parsedCells, cellDiags, root, ok := parseCellsYAML(cellsBlockLines, pos); ok {
				explicitCells = parsedCells
				diags = append(diags, cellDiags...)
				diags = append(diags, yamlTailWarning(ctx, cellsBlockLines, root, cellsStart, "cells:")...)
			} else if len(cellsBlockLines) > 0 {
				// El aviso queda en la línea TABLE, como siempre, y cubre hasta
				// la última línea del bloque: así nombra todas las que se
				// ignoraron.
				diags = append(diags, withLineRange(ctx, diagnostics.NewWarning(
					`The "cells:" block is not valid YAML and was ignored; the table will be empty`,
					pos, "table-parser").WithRuleID("TABLE004"), cellsStart+len(cellsBlockLines)-1))
			}
			continue
		} else if strings.HasPrefix(trimmedLine, "tableRows:") {
			if newDeclared {
				diags = append(diags, diagnostics.NewError("tableRows may be declared only once", pos, "table-parser"))
			}
			newDeclared = true
			blockIndent := currentIndent
			consumed++
			i++
			rowsStart := i
			var blockLines []string
			for i < len(lines) {
				rl := lines[i]
				trim := strings.TrimSpace(rl)
				indent := CalculateIndentLevel(rl)
				if trim != "" && (indent < blockIndent || indent == blockIndent && !strings.HasPrefix(trim, "-")) {
					i--
					break
				}
				blockLines = append(blockLines, rl)
				consumed++
				i++
			}
			parsed, root, err := decodeTableRows(blockLines)
			if err != nil || len(parsed) == 0 {
				// El error se queda en la línea TABLE, como siempre, y cubre el
				// bloque entero: ninguna de sus líneas se leyó.
				diags = append(diags, withLineRange(ctx, diagnostics.NewError("invalid tableRows YAML: expected nonempty rows with known fields", pos, "table-parser"), rowsStart+len(blockLines)-1))
			} else {
				diags = append(diags, yamlTailWarning(ctx, blockLines, root, rowsStart, "tableRows:")...)
				tableRows = make([]ast.TableRow, len(parsed))
				for n, row := range parsed {
					var cells []ast.TableRowCell
					if row.Cells != nil {
						cells = make([]ast.TableRowCell, len(row.Cells))
					}
					tableRows[n] = ast.TableRow{NodeID: row.NodeID, Section: row.Section, Cells: cells}
					for j, cell := range row.Cells {
						tableRows[n].Cells[j] = ast.TableRowCell{NodeID: cell.NodeID, Content: cell.Content, IsHeader: cell.Header,
							Scope: cell.Scope, ColSpan: cell.Colspan, RowSpan: cell.Rowspan}
					}
				}
			}
			continue
		} else if strings.HasPrefix(trimmedLine, "caption:") {
			captionStr := strings.TrimPrefix(trimmedLine, "caption:")
			caption = strings.Trim(strings.TrimSpace(captionStr), "\"")
			repeated.read("caption", i, i)
		} else if strings.HasPrefix(trimmedLine, "label:") {
			// issue #239: identificador de referencia cruzada (p. ej. "tbl:ventas").
			labelStr := strings.TrimPrefix(trimmedLine, "label:")
			label = strings.Trim(strings.TrimSpace(labelStr), "\"")
			repeated.read("label", i, i)
		} else if strings.Contains(trimmedLine, "|") {
			legacyDeclared = true
			// Fallback: Parse table row (separated by |) for compatibility
			cells := TrimMarkdownRowEdges(trimmedLine, SplitMarkdownTableRow(trimmedLine))
			for j := range cells {
				cells[j] = strings.TrimSpace(cells[j])
			}

			// The delimiter row of a markdown table (|---|:-:|) is syntax,
			// not data; without this check it became a row of "---" cells.
			// As in GFM, it only counts as the delimiter when it comes
			// immediately after the header row and has one cell per header
			// column, so a later row of dashes (an "n/a" marker such as
			// "| - | - |") stays data, and the first row is always the
			// header, as before.
			if !delimiterSeen && len(headers) > 0 && len(rows) == 0 &&
				len(cells) == len(headers) && isMarkdownSeparatorRow(cells) {
				delimiterSeen = true
				consumed++
				continue
			}

			if len(headers) == 0 {
				headers = cells
			} else {
				rows = append(rows, cells)
				rowPositions = append(rowPositions, ctx.Position(i))
			}
		} else if trimmedLine != "]" && trimmedLine != "]," {
			// Ninguna rama la leyó: no es fila ni propiedad de la tabla. Se
			// consume como siempre (cortar acá cambiaría qué líneas toma la
			// tabla), pero con aviso (TABLE008). El "]" que cierra un
			// `rows: [` de varias líneas es sintaxis, no texto perdido.
			diags = append(diags, unreadTableLine(ctx, i, trimmedLine))
		}

		consumed++
	}

	if newDeclared && legacyDeclared {
		diags = append(diags, diagnostics.NewError("tableRows cannot be combined with authored headers/rows/cells", pos, "table-parser"))
	}
	diags = append(diags, repeated.diags...)
	rows, rowPositions, diags = dropMiscountedRows(ctx, headers, rows, rowPositions, inlineGroups, gated, diags)
	return headers, rows, caption, label, explicitCells, tableRows, rowPositions, diags, consumed
}

// yamlTailWarning es el TABLE008 de las líneas de un bloque cells: o
// tableRows: que el lector de YAML dejó afuera (ver yamlUnderIndentedTail).
// block son las líneas del bloque y first el índice de block[0] en ctx.Lines.
func yamlTailWarning(ctx *ParseContext, block []string, root *yaml.Node, first int, key string) []diagnostics.Diagnostic {
	if root == nil || !hasUnderIndentedLine(block) {
		return nil
	}
	start, end, ok := yamlDiscardedTail(block, root)
	if !ok {
		return nil
	}
	d := ignoredLineWarning(ctx, first+start, TableLineRuleID, "table-parser",
		"This line of the %s block is indented less than the rows above it, so the YAML reader stopped before it and it was ignored: %q. Indent it like the rows above.",
		key, strings.TrimSpace(block[start]))
	if end > start {
		d = withLineRange(ctx, d, first+end)
		d.Message = fmt.Sprintf("This line of the %s block is indented less than the rows above it, so the YAML reader stopped before it and it was ignored, with the lines after it: %q. Indent them like the rows above.",
			key, strings.TrimSpace(block[start]))
	}
	return []diagnostics.Diagnostic{d}
}

// unreadTableLine es el TABLE008 de una línea del cuerpo de un TABLE que
// ninguna rama lee. Una fila entre corchetes fuera de su lugar tiene su propio
// texto: es casi siempre una fila de rows: con menos sangría que rows: o
// separada de las de arriba por una línea en blanco.
func unreadTableLine(ctx *ParseContext, i int, trimmed string) diagnostics.Diagnostic {
	if strings.HasPrefix(trimmed, "[") {
		return ignoredLineWarning(ctx, i, TableLineRuleID, "table-parser",
			"This row of the TABLE was ignored: %q. A [\"…\"] row is read only right under rows:, indented more than rows:, with no blank line between the rows; or write it as a \"| a | b |\" row.",
			trimmed)
	}
	return ignoredLineWarning(ctx, i, TableLineRuleID, "table-parser",
		"This line of the TABLE is not a row or a property it reads, and was ignored: %q. Write rows as \"| a | b |\" lines or under rows:; TABLE reads headers:, rows:, cells:, tableRows:, caption: and label:.",
		trimmed)
}

// inlineRowsGroup son las filas que un `rows:` en línea agregó a rows:
// desde rows[start], n filas, leídas en la línea de pos.
type inlineRowsGroup struct {
	start, n int
	pos      diagnostics.Position
}

// gatedRow es una fila bajo rows: que v2.58.0 no leía (otra sangría que seis
// espacios, o después de una fila que cortaba el bloque): index en rows y
// line en ctx.Lines.
type gatedRow struct {
	index, line int
}

// dropMiscountedRows saca de rows las filas que no se pueden leer sin
// romper un build que antes pasaba, con su aviso:
//
//   - las de un `rows:` en línea cuando alguna no tiene tantas celdas como
//     los encabezados (o, sin encabezados, como la primera fila del
//     arreglo), con TABLE007. Main descartaba ese valor sin aviso; leerlo y
//     dejar que el linter diera TABLE003 (error) rompería un build que main
//     dejaba pasar.
//   - una fila de gated (ver gatedRow) con encabezados y otro número de
//     celdas, con TABLE008: antes se perdía con un TABLE008, y leída daría
//     TABLE003. Sin encabezados TABLE003 no revisa la tabla, y una fila sin
//     celdas tampoco cuenta.
//
// Las demás filas no se tocan.
func dropMiscountedRows(ctx *ParseContext, headers []string, rows [][]string, rowPositions []diagnostics.Position, groups []inlineRowsGroup, gated []gatedRow, diags []diagnostics.Diagnostic) ([][]string, []diagnostics.Position, []diagnostics.Diagnostic) {
	drop := map[int]bool{}
	for _, group := range groups {
		if group.n == 0 {
			continue
		}
		want := len(headers)
		what := "the headers have"
		if want == 0 {
			want = len(rows[group.start])
			what = "the first row has"
		}
		bad := -1
		for k := 0; k < group.n; k++ {
			if len(rows[group.start+k]) != want {
				bad = k
				break
			}
		}
		if bad < 0 {
			continue
		}
		diags = append(diags, diagnostics.NewWarning(
			fmt.Sprintf("The rows: value on this line was ignored: row %d has %d cells and %s %d. Fix that row, or put each row on its own line below rows:", bad+1, len(rows[group.start+bad]), what, want),
			group.pos, "table-parser").WithRuleID(TableRowsRuleID))
		for k := 0; k < group.n; k++ {
			drop[group.start+k] = true
		}
	}
	for _, g := range gated {
		row := rows[g.index]
		if len(headers) == 0 || len(row) == 0 || len(row) == len(headers) {
			continue
		}
		diags = append(diags, ignoredLineWarning(ctx, g.line, TableLineRuleID, "table-parser",
			"This row under rows: has %d cells where the headers have %d, so it was ignored: %q. Give it one cell per header.",
			len(row), len(headers), strings.TrimSpace(ctx.Lines[g.line])))
		drop[g.index] = true
	}
	if len(drop) == 0 {
		return rows, rowPositions, diags
	}
	keptRows := make([][]string, 0, len(rows)-len(drop))
	keptPositions := make([]diagnostics.Position, 0, len(rowPositions))
	for k := range rows {
		if drop[k] {
			continue
		}
		keptRows = append(keptRows, rows[k])
		if k < len(rowPositions) {
			keptPositions = append(keptPositions, rowPositions[k])
		}
	}
	return keptRows, keptPositions, diags
}

// inlineArrayDepth cuenta los "[" menos los "]" de v, fuera de comillas
// dobles: 1 para "[[…]," (un arreglo de filas que sigue en la línea de
// abajo), -1 para "[…]]" (la última fila que además lo cierra).
func inlineArrayDepth(v string) int {
	depth := 0
	inQuotes := false
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '"':
			inQuotes = !inQuotes
		case '[':
			if !inQuotes {
				depth++
			}
		case ']':
			if !inQuotes {
				depth--
			}
		}
	}
	return depth
}

type yamlTableRowEntry struct {
	NodeID  string                  `yaml:"nodeId"`
	Section string                  `yaml:"section"`
	Cells   []yamlTableRowCellEntry `yaml:"cells"`
}

type yamlTableRowCellEntry struct {
	NodeID  string `yaml:"nodeId"`
	Content string `yaml:"content"`
	Header  bool   `yaml:"header"`
	Scope   string `yaml:"scope"`
	Colspan int    `yaml:"colspan"`
	Rowspan int    `yaml:"rowspan"`
}

// trimInlineArrayBrackets quita UN "[" inicial y UN "]" final del valor de
// headers:/rows:, en vez del strings.Trim(s, "[]") anterior, que quitaba
// TODOS los corchetes de los extremos y por lo tanto se comía el corchete
// que fuera parte del contenido de la primera/última celda (p. ej.
// `["[borrador]", "B"]` perdía el "[" de la celda). El llamador ya verificó
// que el valor empieza en "[" y termina en "]".
func trimInlineArrayBrackets(s string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
}

// splitInlineArray parte el contenido (ya sin corchetes) de un array inline
// del bloque TABLE — headers: o una fila de rows: — en sus celdas,
// respetando las comillas dobles: una coma DENTRO de una celda entrecomillada
// ("Manual, dual-approval", "1,000", "Berlin, Germany") ya no parte la celda
// en dos. Antes se hacía strings.Split(s, ",") a secas, así que ese contenido
// —muy común— generaba una celda de más y disparaba el diagnóstico del linter
// sobre el número de columnas (TABLE003, "Table row has incorrect number of
// columns"), que culpaba a la fila cuando el problema real era el contenido.
//
// Reglas, en orden:
//
//   - El BACKSLASH NO ES ESCAPE: toda comilla doble abre o cierra, y `\` es
//     contenido literal como cualquier otro carácter. Es la misma política
//     que el resto del dialecto strict, documentada en quote()
//     (formatter/util.go): ahí se explica por qué escapar `\` corrompía el
//     round-trip de valores con backslash literal. Tratar `\"` como comilla
//     escapada —como hacía la primera versión de este splitter— rompía
//     justamente el caso que la política protege: una celda terminada en
//     backslash (`["Berlin, Germany", "C:\"]`) se comía su comilla de cierre,
//     dejaba las comillas desbalanceadas y caía al fallback de abajo,
//     reproduciendo el bug de las tres celdas. La contrapartida es que una
//     comilla doble LITERAL dentro de una celda sigue sin ser representable
//     en esta forma — limitación pre-existente del dialecto, no de este fix:
//     para eso está la forma "cells:" (YAML real, que se entrecomilla sola),
//     y checkQuotable() ya rechaza ese valor en el formatter.
//   - Si las comillas quedan desbalanceadas (conteo IMPAR, p. ej. `A", B`) se
//     cae al split ingenuo por comas de siempre, así que ese caso conserva
//     exactamente el comportamiento previo. Ojo: es una red de seguridad por
//     conteo, no una validación; entrada malformada con un número PAR de
//     comillas mal puestas (`A", B", C`) sí puede diferir del split ingenuo.
//     Entrada bien formada —con o sin comillas— es el caso que este splitter
//     garantiza.
//   - Un array vacío (`headers: []`) produce cero celdas, no una celda vacía
//     como antes (strings.Split("", ",") devuelve un elemento vacío). Ese
//     header fantasma falseaba el conteo de columnas; ahora un `headers: []`
//     reporta el TABLE001 que corresponde ("should have headers defined").
//
// Comillas simples ('A, B') NO se reconocen como delimitador: el dialecto
// strict entrecomilla con comillas dobles en todas partes (quote()), así que
// una comilla simple es contenido literal, no sintaxis.
func splitInlineArray(s string) []string {
	raw, ok := splitTopLevelCommas(s)
	if !ok {
		// Comillas desbalanceadas: comportamiento legacy (ver doc arriba).
		legacy := []string{}
		for _, part := range strings.Split(s, ",") {
			legacy = append(legacy, strings.Trim(strings.TrimSpace(part), "\""))
		}
		return legacy
	}

	// El último item se descarta si TODO el array venía vacío ("[]"): ahí
	// splitTopLevelCommas devuelve un único item vacío, que como celda sería
	// el header fantasma descrito arriba. Con una coma de por medio ("A,") el
	// item vacío sí es una celda y se conserva, igual que antes.
	if len(raw) == 1 && strings.TrimSpace(raw[0]) == "" {
		return []string{}
	}

	items := make([]string, 0, len(raw))
	for _, item := range raw {
		items = append(items, unquoteInlineArrayItem(item))
	}
	return items
}

// unquoteInlineArrayItem normaliza una celda ya separada: recorta espacios y,
// si queda envuelta en un par de comillas dobles, quita ESE par y nada más —
// el contenido interior (comas, backslashes, corchetes) pasa verbatim, sin
// desescapes, por la política de splitInlineArray. Si no forma un par (`A"`,
// `"`, sin comillas), se conserva el strings.Trim(item, `"`) histórico para no
// cambiar el comportamiento de entrada malformada.
func unquoteInlineArrayItem(item string) string {
	item = strings.TrimSpace(item)
	if len(item) >= 2 && item[0] == '"' && item[len(item)-1] == '"' {
		return item[1 : len(item)-1]
	}
	return strings.Trim(item, "\"")
}

// yamlTableCellEntry es la forma YAML de una celda dentro del bloque
// "cells:" (issue #20) — nombres de campo en minúscula sin prefijo, para que
// la sintaxis de autoría sea la aprobada:
//
//	cells:
//	  - [{content: A, header: true, colspan: 2}, {content: B, header: true}]
//	  - [{content: 1}, {content: 2}, {content: 3}]
type yamlTableCellEntry struct {
	Content string `yaml:"content"`
	Header  bool   `yaml:"header"`
	Scope   string `yaml:"scope"`
	Colspan int    `yaml:"colspan"`
	Rowspan int    `yaml:"rowspan"`
}

// parseCellsYAML parses the block of lines collected under "cells:" (a YAML
// sequence of sequences of maps, flow style) into [][]ast.TableCell.
// Returns ok=false if the block is empty or not valid YAML — in that case
// the caller keeps the previous behavior (Headers/Rows derived from other
// keys in the block, typically empty if only a malformed "cells:" was
// declared) and is responsible for surfacing a diagnostic, since only the
// caller has the table's position.
//
// A declared colspan/rowspan above ast.MaxCellSpan is clamped rather than
// rejected outright, and reported via a single aggregated warning — see
// ast.MaxCellSpan's doc comment for why the cap exists. An invalid scope
// (anything other than "", "row", "col") is cleared the same way — see the
// TABLE006 comment below.
//
// Decodifica una sola vez, a un yaml.Node, y de ahí a las celdas: el nodo
// raíz se devuelve para calcular las líneas que yaml.v3 descartó
// (yamlDiscardedTail) sin volver a leer el bloque.
func parseCellsYAML(blockLines []string, pos diagnostics.Position) ([][]ast.TableCell, []diagnostics.Diagnostic, *yaml.Node, bool) {
	if len(blockLines) == 0 {
		return nil, nil, nil, false
	}

	raw := strings.Join(blockLines, "\n")
	var doc yaml.Node
	var parsed [][]yamlTableCellEntry
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil || len(doc.Content) == 0 {
		return nil, nil, nil, false
	}
	if err := doc.Decode(&parsed); err != nil || len(parsed) == 0 {
		return nil, nil, nil, false
	}

	clamped := false
	invalidScope := false
	cells := make([][]ast.TableCell, len(parsed))
	for i, row := range parsed {
		cells[i] = make([]ast.TableCell, len(row))
		for j, c := range row {
			colspan := c.Colspan
			if colspan > ast.MaxCellSpan {
				colspan = ast.MaxCellSpan
				clamped = true
			}
			rowspan := c.Rowspan
			if rowspan > ast.MaxCellSpan {
				rowspan = ast.MaxCellSpan
				clamped = true
			}
			scope := c.Scope
			if scope != "" && scope != "row" && scope != "col" {
				// Cleared, not just left as-is: renderTableCells'
				// writeTableCellRow only ever emits scope="row"/"col" (a
				// fixed allowlist, defensive against interpolating an
				// arbitrary author string into an HTML attribute), so an
				// invalid value like "column" would otherwise round-trip
				// into the JSON contract (elem.Cells) while silently never
				// appearing in the rendered HTML — no error, no diagnostic,
				// just a value that quietly does nothing. Precisely the
				// kind of silent failure this a11y feature can't afford:
				// a typo'd scope looks "saved" in the JSON but the
				// accessibility structure it was meant to declare never
				// reaches the DOM.
				invalidScope = true
				scope = ""
			}
			cells[i][j] = ast.TableCell{
				Content:  c.Content,
				IsHeader: c.Header,
				Scope:    scope,
				ColSpan:  colspan,
				RowSpan:  rowspan,
			}
		}
	}

	var diags []diagnostics.Diagnostic
	if clamped {
		diags = append(diags, diagnostics.NewWarning(
			"A cell declared colspan/rowspan above the supported maximum; it was clamped",
			pos, "table-parser").WithRuleID("TABLE005"))
	}
	if invalidScope {
		diags = append(diags, diagnostics.NewWarning(
			`A cell declared a scope other than "row"/"col"; it was cleared`,
			pos, "table-parser").WithRuleID("TABLE006"))
	}
	return cells, diags, doc.Content[0], true
}

// SplitMarkdownTableRow parte una fila de tabla markdown por "|", respetando
// code spans (con backticks) y pipes escapados (\|) — ninguno de los dos es
// un separador de celda de verdad. strings.Split(line, "|") partía
// ciegamente por CADA "|", así que una celda con código (p. ej.
// "`User | null`") o un pipe escapado a propósito se fragmentaba en celdas
// de más, disparando TABLE003 ("número incorrecto de columnas") sobre una
// fila perfectamente válida (F10, audit 2026-09-11). Exportada porque
// parser/strict.go (la tabla markdown de SlideLang strict, un parser
// separado del de este paquete) tiene el mismo bug con el mismo split
// ciego y necesita el mismo fix.
//
// codeSpanRanges (abajo) implementa la regla real de CommonMark para code
// spans: el delimitador no es "un backtick", es una CORRIDA de N backticks
// consecutivos, y solo otra corrida de EXACTAMENTE N backticks la cierra —
// un span delimitado por dos backticks seguidos, con un backtick suelto en
// el medio, tiene contenido "a`b" (el backtick suelto del medio no
// cierra nada, porque su corrida mide 1 y el delimitador mide 2). Una
// corrida que abre pero no encuentra un cierre de su mismo largo en el
// resto de la línea NO es un delimitador — son backticks literales, y el
// escaneo sigue de largo (hallazgo de revisión: la versión anterior
// alternaba inCode por cada CARÁCTER "`" suelto, así que una corrida de 2
// backticks se leía como "abre, cierra" en vez de "abre un span de largo
// 2" — cualquier "|" adentro de un span delimitado por 2+ backticks se
// interpretaba mal como separador). Esto también reemplaza sin pérdida al
// chequeo anterior de "cantidad de backticks impar en toda la línea": un
// backtick suelto sin pareja de su mismo largo simplemente no matchea
// nada aquí, y punto — no hace falta una heurística aparte para ese caso.
//
// Una corrida de backticks escapada (precedida por un número IMPAR de
// backslashes, p. ej. "\`") no puede abrir un span (hallazgo de tercera
// ronda de revisión, con precedente exacto en el ejemplo de la sección
// "Backslash escapes" del spec de CommonMark: "\`not code`" se renderiza
// como el texto literal "`not code`", nunca como <code>). backslashEscaped
// (abajo) da esa paridad; un backtick escapado se salta sin abrir ni
// cerrar nada, como si fuera texto común.
func codeSpanRanges(runes []rune) []bool {
	inSpan := make([]bool, len(runes))
	escaped := backslashEscaped(runes)
	for _, sp := range codespan.Find(runes, func(i int) bool { return escaped[i] }) {
		for k := sp.OpenStart; k < sp.CloseEnd; k++ {
			inSpan[k] = true
		}
	}
	return inSpan
}

// backslashEscaped marca, por índice de rune, qué posiciones quedan
// escapadas por un backslash que las precede — es decir, escaped[i] es
// true cuando el número de backslashes consecutivos justo antes de la
// posición i es IMPAR (hallazgo de tercera ronda de revisión: la versión
// anterior sólo miraba UN backslash hacia atrás, así que "\\|" —un
// backslash escapado seguido de un pipe real y sin escapar— se leía igual
// que "\|" —un backslash escapando al pipe—, porque el segundo backslash
// de la corrida, tomado aislado, "veía" el pipe siguiente sin saber que él
// mismo ya estaba consumido por el backslash anterior. Una corrida de N
// backslashes consecutivos se empareja de a pares —cada par es un
// backslash literal— y sólo el ÚLTIMO de una corrida IMPAR escapa al
// carácter que sigue.
func backslashEscaped(runes []rune) []bool {
	n := len(runes)
	escaped := make([]bool, n)
	i := 0
	for i < n {
		if runes[i] != '\\' {
			i++
			continue
		}
		start := i
		for i < n && runes[i] == '\\' {
			i++
		}
		runLen := i - start
		if runLen%2 == 1 && i < n {
			escaped[i] = true
		}
	}
	return escaped
}

func SplitMarkdownTableRow(line string) []string {
	runes := []rune(line)
	inSpan := codeSpanRanges(runes)

	var cells []string
	var current strings.Builder
	i := 0
	for i < len(runes) {
		r := runes[i]
		switch {
		case r == '\\':
			start := i
			for i < len(runes) && runes[i] == '\\' {
				i++
			}
			runLen := i - start
			if inSpan[start] {
				// Backslashes DENTRO de un code span son literales —
				// CommonMark §6.1: "backslash escapes do not work in code
				// spans" —, con UNA excepción que pone la extensión de
				// tablas de GFM: el "\|" se procesa al partir la fila,
				// ANTES del parseo inline, así que también vale dentro de
				// un code span (ejemplo 200 del spec de GFM: la celda
				// "`\|`" se renderiza <code>|</code>). Una versión
				// anterior preservaba la corrida byte a byte también
				// delante del "|", y "`a \| b`" salía con el backslash
				// visible en HTML/PDF/DOCX. Ahora la corrida que precede a
				// un "|" pierde exactamente un backslash (el que escapa al
				// pipe, como hace cmark-gfm al desescapar la celda); el
				// resto de la corrida, y cualquier corrida que no preceda
				// a un "|", queda literal. El "|" sin backslash dentro de
				// un span sigue protegido (caso de abajo), así que el
				// formatter solo tiene que agregar un backslash cuando el
				// contenido ya trae alguno delante del pipe.
				keep := runLen
				if i < len(runes) && runes[i] == '|' {
					keep = runLen - 1
				}
				for k := 0; k < keep; k++ {
					current.WriteRune('\\')
				}
			} else if i < len(runes) && runes[i] == '|' {
				// Corrida seguida de un pipe: CommonMark empareja los
				// backslashes de a DOS — cada par escapa al otro y
				// colapsa a UN backslash literal (hallazgo de cuarta
				// ronda de revisión: la versión anterior escribía los
				// runLen-1 backslashes de una corrida impar uno por uno,
				// sin aparear — "\\\|" reconstruía 2 backslashes en vez
				// de 1). Si sobra uno (corrida impar), ese último escapa
				// al pipe, que queda literal; si no sobra ninguno
				// (corrida par), el pipe sigue sin escapar y es
				// separador real.
				for k := 0; k < runLen/2; k++ {
					current.WriteRune('\\')
				}
				if runLen%2 == 1 {
					current.WriteRune('|')
					i++
				}
			} else {
				// Fuera de esa posición, un backslash no es un escape
				// (ver splitInlineArray/C8): se escribe literal, corrida
				// completa, sin aparear.
				for k := 0; k < runLen; k++ {
					current.WriteRune('\\')
				}
			}
			continue
		case r == '|' && inSpan[i]:
			current.WriteRune(r)
		case r == '|':
			cells = append(cells, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
		i++
	}
	cells = append(cells, current.String())
	return cells
}

// TrimMarkdownRowEdges quita las celdas vacías que producen los pipes de
// borde de una fila markdown ("| a | b |" se parte, con
// SplitMarkdownTableRow, en ["", " a ", " b ", ""]). Recibe la línea original
// además de las celdas porque la regla depende de ella: se descarta a lo más
// UNA celda vacía al principio, y sólo si la línea (sin espacios en los
// extremos) empieza con "|", y a lo más UNA al final, y sólo si termina con
// "|". Una fila sin pipes de borde ("a | b") no pierde nada, una celda vacía
// legítima en medio ("| a || c |") se conserva, y una celda vacía legítima
// pegada al borde ("| | b |") también, porque ese caso trae dos pipes
// seguidos y sólo el primero es el de borde.
//
// Antes cada llamador hacía este recorte por su cuenta, y el fallback de
// pipes dentro de un bloque TABLE no lo hacía: una fila "| Métrica | Q2 |"
// salía con cinco columnas en vez de tres.
func TrimMarkdownRowEdges(line string, cells []string) []string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "|") && len(cells) > 0 && strings.TrimSpace(cells[0]) == "" {
		cells = cells[1:]
	}
	if strings.HasSuffix(line, "|") && len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// MarkdownColumnAlign lee la alineación de cada columna de una fila
// delimitadora Markdown: ":---" left, ":---:" center, "---:" right y "---"
// sin declarar (""). Devuelve nil si la fila no es delimitadora, si no
// tiene una celda por columna, o si ninguna columna declara alineación.
func MarkdownColumnAlign(cells []string, numCols int) []string {
	if len(cells) != numCols || !isMarkdownSeparatorRow(cells) {
		return nil
	}
	align := make([]string, numCols)
	declared := false
	for i, c := range cells {
		c = strings.TrimSpace(c)
		left, right := strings.HasPrefix(c, ":"), strings.HasSuffix(c, ":")
		switch {
		case left && right:
			align[i] = "center"
		case right:
			align[i] = "right"
		case left:
			align[i] = "left"
		}
		if align[i] != "" {
			declared = true
		}
	}
	if !declared {
		return nil
	}
	return align
}

// IsMarkdownSeparatorLine dice si una línea de una tabla Markdown con pipes
// es su fila separadora. Es la regla de los dos dialectos: la usan la tabla
// Markdown de flex y de un bloque ":::" (TableParser.parseMarkdownTable) y la
// de strict (parser/strict.go), cada una en las líneas que ya revisaba.
//
// Una línea es separadora si no tiene nada más que guiones, dos puntos,
// espacios y "|", y además tiene "---", o empieza con "|" y tiene ":--",
// "--:" o ":-:": "|---|", "|:--|--:|", "| :-: | --- |", "| --- | ---" sin el
// "|" final, "--- | ---" sin pipes de borde. Una
// línea con cualquier otro texto no es separadora: "| 2019 | --- |" tiene
// una celda "---" y "| a --- b | c |" una celda "a --- b", y son filas de
// datos donde caben (ver parseMarkdownTable). Antes la tabla de
// flex y de ":::" saltaba toda línea con esas subcadenas, y sus celdas no
// llegaban al AST; strict las leía como datos salvo justo después del
// encabezado, y solo reconocía el separador por "---".
//
// Lo que no cambia: una fila de guiones cortos ("| - | - |", "|--|--|") no es
// separadora, porque no tiene ninguna de las cuatro subcadenas, y en flex
// esta regla se aplica a cualquier fila de la tabla, donde un "-" suele
// marcar una celda vacía. Una línea que no empieza con "|" (":-- | --:",
// ":---:") solo es separadora con "---", como en strict hasta ahora; flex no
// la ve como fila de la tabla. Tampoco cambia qué acepta
// el bloque TABLE (isMarkdownSeparatorRow) ni en qué filas mira cada parser.
func IsMarkdownSeparatorLine(line string) bool {
	line = strings.TrimSpace(line)
	// Espacios en el sentido de unicode.IsSpace, como el TrimSpace de cada
	// celda: un "---\u00a0" sigue siendo una celda de guiones.
	for _, r := range line {
		if r != '|' && r != '-' && r != ':' && !unicode.IsSpace(r) {
			return false
		}
	}
	if strings.Contains(line, "---") {
		return true
	}
	return strings.HasPrefix(line, "|") &&
		(strings.Contains(line, ":-:") || strings.Contains(line, ":--") || strings.Contains(line, "--:"))
}

// isMarkdownSeparatorRow reporta si una fila ya partida es la fila
// delimitadora de una tabla markdown: al menos una celda y todas del estilo
// "---", ":--", "--:" o ":-:" (guiones con dos puntos opcionales en los
// extremos). Es más estricta que buscar "---" en la línea, que también
// atraparía una celda de datos como "a---b".
func isMarkdownSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		c = strings.TrimSpace(c)
		c = strings.TrimPrefix(c, ":")
		c = strings.TrimSuffix(c, ":")
		if c == "" || strings.Trim(c, "-") != "" {
			return false
		}
	}
	return true
}

// parseMarkdownTable parsea una tabla en formato Markdown.
//
// Una fila que antes se saltaba como separador solo por tener "---", ":--",
// "--:" o ":-:" ("| 2019 | --- |") ahora es de datos, pero solo cuando cabe en
// la tabla, para que ningún documento que compilaba deje de compilar (el lint
// TABLE003 es un error): una fila de datos así entra si tiene una celda por
// columna del encabezado, y como primera línea es el encabezado si la sigue
// un separador con su mismo número de celdas (la forma de GFM) y todas las
// filas de abajo tienen ese número (o ninguna celda). Si no cabe, se salta
// como antes, con un TABLE008 que la nombra y dice por qué.
//
// Lo mismo vale para un separador sin pipe inicial justo después del
// encabezado ("--- | ---", ver isBareHeaderSeparator): antes cortaba la
// tabla, la línea quedaba como párrafo y las filas de abajo eran otra tabla
// con su propio encabezado. Ahora es el separador solo si todas las filas de
// abajo caben en el encabezado; si no, la tabla se lee como antes.
func (p *TableParser) parseMarkdownTable(ctx *ParseContext, startIndex int) ([]string, [][]string, []diagnostics.Position, []string, int, []diagnostics.Diagnostic) {
	t := p.readMarkdownTable(ctx, startIndex, true, true)
	if (t.headerRecovered || t.bareSeparator) && !rowsFitHeader(t.headers, t.rows) {
		// Las dos recuperaciones no se dan juntas: el encabezado recuperado
		// necesita un separador que empiece con "|" en la línea siguiente.
		t = p.readMarkdownTable(ctx, startIndex, !t.headerRecovered, !t.bareSeparator)
	}
	return t.headers, t.rows, t.rowPositions, t.columnAlign, t.consumed, t.diags
}

// rowsFitHeader dice si cada fila tiene una celda por columna del
// encabezado o ninguna celda: la condición con la que TABLE003 no dispara.
func rowsFitHeader(headers []string, rows [][]string) bool {
	for _, row := range rows {
		if len(row) != 0 && len(row) != len(headers) {
			return false
		}
	}
	return true
}

// IsHeaderSeparatorLine dice si line, la línea que sigue al encabezado de una
// tabla Markdown de numCols columnas, es su separador. Es
// IsMarkdownSeparatorLine más dos formas de GFM que esa regla no lee, solo con
// una celda por columna: un separador de guiones cortos ("|--|--|",
// "| :-- | --: |" ya lo era) y uno sin pipe inicial ("--- | ---",
// ":-- | --:", "--|--"). Cada celda tiene que tener al menos dos guiones: una
// fila "| - | - |" sigue siendo de datos, porque un "-" suele marcar una
// celda vacía. Solo la línea que sigue al encabezado puede ser el separador;
// en cualquier otra fila de la tabla, una fila de guiones es de datos.
//
// Una línea sin "|" ("---", ":---:") no entra por la regla de GFM: en flex
// una línea de guiones sola no es parte de la tabla, y en strict ya era
// separador por IsMarkdownSeparatorLine. La forma sin pipe inicial la usa la
// tabla de flex y de ":::" solo si las filas de abajo caben (ver
// parseMarkdownTable); strict, que no lo revisa, usa solo la de guiones
// cortos.
func IsHeaderSeparatorLine(line string, numCols int) bool {
	line = strings.TrimSpace(line)
	if IsMarkdownSeparatorLine(line) && (strings.HasPrefix(line, "|") || !strings.Contains(line, "|")) {
		return true
	}
	if !strings.Contains(line, "|") {
		return false
	}
	cells := TrimMarkdownRowEdges(line, SplitMarkdownTableRow(line))
	if len(cells) != numCols || !isMarkdownSeparatorRow(cells) {
		return false
	}
	for _, c := range cells {
		if strings.Count(c, "-") < 2 {
			return false
		}
	}
	return true
}

// isBareHeaderSeparator dice si line es un separador sin pipe inicial
// ("--- | ---") que IsHeaderSeparatorLine acepta.
func isBareHeaderSeparator(line string, numCols int) bool {
	line = strings.TrimSpace(line)
	return !strings.HasPrefix(line, "|") && strings.Contains(line, "|") && IsHeaderSeparatorLine(line, numCols)
}

type markdownTableRead struct {
	headers      []string
	rows         [][]string
	rowPositions []diagnostics.Position
	columnAlign  []string
	consumed     int
	diags        []diagnostics.Diagnostic
	// headerRecovered: el encabezado es una primera línea con guiones que
	// antes se saltaba como separador.
	headerRecovered bool
	// bareSeparator: el separador es una línea sin pipe inicial
	// ("--- | ---") que antes cortaba la tabla.
	bareSeparator bool
}

// legacyMarkdownSeparator es la regla vieja de la tabla de flex y de ":::":
// toda línea con "---", ":--", "--:" o ":-:" se saltaba como separador.
func legacyMarkdownSeparator(line string) bool {
	return strings.Contains(line, "---") || strings.Contains(line, ":-:") ||
		strings.Contains(line, ":--") || strings.Contains(line, "--:")
}

func markdownRowCells(line string) []string {
	cells := TrimMarkdownRowEdges(line, SplitMarkdownTableRow(line))
	for j := range cells {
		cells[j] = strings.TrimSpace(cells[j])
	}
	return cells
}

func (p *TableParser) readMarkdownTable(ctx *ParseContext, startIndex int, recoverHeader, recoverBareSeparator bool) markdownTableRead {
	// Inicializados como slices vacíos (no nil), ver comentario en parseYAMLTable.
	t := markdownTableRead{headers: []string{}, rows: [][]string{}}
	lines := ctx.Lines
	// separatorSeen: ya se leyó el separador que sigue al encabezado. Solo
	// ese es sintaxis; una fila de guiones más abajo es de datos si cabe.
	separatorSeen := false

	for i := startIndex; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		// El lugar del separador: hay encabezado y todavía ninguna fila.
		separatorSlot := len(t.headers) > 0 && len(t.rows) == 0 && !separatorSeen

		if recoverBareSeparator && separatorSlot && isBareHeaderSeparator(line, len(t.headers)) {
			// "--- | ---": el separador de GFM sin pipe inicial. Antes
			// cortaba la tabla (ver parseMarkdownTable).
			t.columnAlign = MarkdownColumnAlign(TrimMarkdownRowEdges(line, SplitMarkdownTableRow(line)), len(t.headers))
			t.bareSeparator = true
			separatorSeen = true
			t.consumed++
			continue
		}

		// Stop at empty lines or lines that don't open with "|". Issue #191:
		// esto era Contains("|") — un solo pipe en CUALQUIER posición bastaba
		// para seguir consumiendo la línea como fila, así que una tabla
		// seguida de prosa con un "|" suelto (p. ej. "Total anual | 2024")
		// se absorbía como fila de basura en vez de cortar la tabla. Las
		// otras tres puertas del sistema — TableParser.CanParse (:49),
		// IsNewElement (common.go:342) y la continuación de tabla en modo
		// strict (parser/strict.go:893) — ya exigían un "|" INICIAL; esta
		// era la única que no.
		if line == "" || !strings.HasPrefix(line, "|") {
			break
		}

		// El separador (|---|---|) es la línea que sigue al encabezado, ver
		// IsHeaderSeparatorLine. Una línea de guiones antes del encabezado
		// se salta como siempre.
		if (separatorSlot && IsHeaderSeparatorLine(line, len(t.headers))) || (len(t.headers) == 0 && IsMarkdownSeparatorLine(line)) {
			// La alineación solo sale de la delimitadora que sigue al header.
			if separatorSlot {
				t.columnAlign = MarkdownColumnAlign(TrimMarkdownRowEdges(line, SplitMarkdownTableRow(line)), len(t.headers))
				separatorSeen = true
			}
			t.consumed++
			continue
		}

		// Parse table row
		// Drop the empty pieces that the leading/trailing "|" leave behind.
		cells := markdownRowCells(line)

		// Una fila con guiones que antes se saltaba como separador solo
		// entra si cabe (ver parseMarkdownTable). Eso incluye una fila de
		// solo guiones que no sigue al encabezado ("| --- |" a media
		// tabla): antes se saltaba sin aviso.
		if legacyMarkdownSeparator(line) {
			var why string
			switch {
			case len(t.headers) == 0:
				next := ""
				if i+1 < len(lines) {
					next = strings.TrimSpace(lines[i+1])
				}
				if !recoverHeader || !strings.HasPrefix(next, "|") || !IsMarkdownSeparatorLine(next) ||
					len(TrimMarkdownRowEdges(next, SplitMarkdownTableRow(next))) != len(cells) {
					why = "This line opens the table and has dashes in it, so it is read as a separator row and its cells were ignored: %q. A first row with dashes in a cell is the header only when a separator row with one cell per column follows it and every row below has that many cells."
				} else {
					t.headerRecovered = true
				}
			case len(cells) != len(t.headers) && IsMarkdownSeparatorLine(line):
				why = fmt.Sprintf("This row of dashes is not the separator row, which comes right after the header, and it has %d cells where the header has %d, so it was ignored: %%q. A row of dashes below the separator stays in the table when it has one cell per column.", len(cells), len(t.headers))
			case len(cells) != len(t.headers):
				why = fmt.Sprintf("This table row has dashes in it and %d cells where the header has %d, so it is read as a separator row and its cells were ignored: %%q. A row with dashes in a cell stays in the table when it has one cell per column; a separator row has only dashes and colons, as in | --- | :-: |.", len(cells), len(t.headers))
			}
			if why != "" {
				t.diags = append(t.diags, ignoredLineWarning(ctx, i, TableLineRuleID, "table-parser", why, line))
				t.consumed++
				continue
			}
		}

		// First row becomes headers
		if len(t.headers) == 0 {
			t.headers = cells
		} else {
			t.rows = append(t.rows, cells)
			t.rowPositions = append(t.rowPositions, ctx.Position(i))
		}

		t.consumed++
	}

	return t
}

// TableRowsRuleID es el aviso de un `rows:` en línea que no se pudo leer como
// filas (issue #488). Es warning en todos los dialectos: las filas se pierden
// igual que antes, pero ahora con aviso.
const TableRowsRuleID = "TABLE007"

// isNestedInlineArray dice si v, el valor de `rows:` en su misma línea, es un
// arreglo de arreglos: empieza con "[" y, después de espacios, otro "[".
func isNestedInlineArray(v string) bool {
	rest, ok := strings.CutPrefix(v, "[")
	return ok && strings.HasPrefix(strings.TrimLeft(rest, " \t"), "[")
}

// parseInlineRows lee `[[…], […]]`, el valor de `rows:` en su misma línea,
// con las mismas reglas que una fila en su propia línea (splitInlineArray:
// la comilla doble abre o cierra, el backslash es contenido). Cada celda
// tiene que estar entre comillas dobles sin otra comilla adentro, o no tener
// comillas: así el corte entre filas no depende de adivinar dónde cierra una
// comilla. Si algo no cuadra (comillas sueltas, texto entre filas o después
// del arreglo, salvo un comentario `#`), ok es false y no se lee ninguna
// fila: nunca se devuelven filas de un corte dudoso. Una celda entre comillas
// simples ('a', el estilo de un repr de Python) tampoco se lee: en strict la
// comilla simple es contenido, así que los apóstrofos quedarían en la celda.
// Tampoco una celda sin comillas dobles que empiece o termine con un
// apóstrofo ('90s, rock'): no se sabe si es contenido o media comilla
// simple. quote dice cuál de los dos casos fue, para el aviso.
//
// Una fila vacía ([]) no cuenta: `rows: [[]]` da cero filas, como en main.
func parseInlineRows(v string) (rows [][]string, quote inlineQuoteProblem, ok bool) {
	i := 1 // después del "[" exterior
	skip := func() {
		for i < len(v) && (v[i] == ' ' || v[i] == '\t') {
			i++
		}
	}
	for {
		skip()
		if i >= len(v) || v[i] != '[' {
			return nil, inlineQuotesOK, false
		}
		start := i + 1
		inQuotes := false
		j := start
		for ; j < len(v); j++ {
			if v[j] == '"' {
				inQuotes = !inQuotes
			} else if v[j] == ']' && !inQuotes {
				break
			}
		}
		if j >= len(v) {
			return nil, inlineQuotesOK, false
		}
		content := v[start:j]
		if q := inlineRowQuoteProblem(content); q != inlineQuotesOK {
			return nil, q, false
		}
		if !wellQuotedInlineRow(content) {
			return nil, inlineQuotesOK, false
		}
		if cells := splitInlineArray(content); len(cells) > 0 {
			rows = append(rows, cells)
		}
		i = j + 1
		skip()
		if i < len(v) && v[i] == ',' {
			i++
			continue
		}
		if i < len(v) && v[i] == ']' {
			i++
			break
		}
		return nil, inlineQuotesOK, false
	}
	if rest := strings.TrimSpace(v[i:]); rest != "" && !strings.HasPrefix(rest, "#") {
		return nil, inlineQuotesOK, false
	}
	return rows, inlineQuotesOK, true
}

// wellQuotedInlineRow dice si cada celda de content (una fila sin sus
// corchetes) está entre comillas dobles sin otra comilla adentro, o no tiene
// comillas.
func wellQuotedInlineRow(content string) bool {
	items, ok := splitTopLevelCommas(content)
	if !ok {
		return false
	}
	for _, item := range items {
		item = strings.TrimSpace(item)
		n := strings.Count(item, "\"")
		if n == 0 {
			continue
		}
		if n != 2 || len(item) < 2 || item[0] != '"' || item[len(item)-1] != '"' {
			return false
		}
	}
	return true
}

// inlineQuoteProblem dice por qué un `rows:` en línea no se leyó, cuando es
// por comillas simples o apóstrofos.
type inlineQuoteProblem int

const (
	inlineQuotesOK inlineQuoteProblem = iota
	// inlineSingleQuotes: una celda está entre comillas simples ('a').
	inlineSingleQuotes
	// inlineApostrophe: una celda sin comillas dobles empieza o termina
	// con un apóstrofo ('90s), sin cerrarlo.
	inlineApostrophe
)

// inlineRowQuoteProblem revisa las celdas de content (una fila sin sus
// corchetes): una entre comillas simples ('a', 'a, b') o una sin comillas
// dobles que empieza o termina con un apóstrofo. Un apóstrofo dentro de una
// celda (O'Brien) o dentro de comillas dobles ("'90s") no cuenta.
func inlineRowQuoteProblem(content string) inlineQuoteProblem {
	items, ok := splitTopLevelCommas(content)
	if !ok {
		return inlineQuotesOK
	}
	apostrophe := false
	for _, item := range items {
		item = strings.TrimSpace(item)
		if strings.HasPrefix(item, "\"") {
			continue
		}
		if len(item) >= 2 && item[0] == '\'' && item[len(item)-1] == '\'' {
			return inlineSingleQuotes
		}
		if strings.HasPrefix(item, "'") || strings.HasSuffix(item, "'") {
			apostrophe = true
		}
	}
	if apostrophe {
		// 'a, b' se parte en "'a" y "b'": son comillas simples, no dos
		// apóstrofos sueltos.
		if strings.HasPrefix(strings.TrimSpace(items[0]), "'") && strings.HasSuffix(strings.TrimSpace(items[len(items)-1]), "'") {
			return inlineSingleQuotes
		}
		return inlineApostrophe
	}
	return inlineQuotesOK
}

// decodeTableRows lee un bloque tableRows: exactamente como siempre, con
// KnownFields(true) sobre las mismas estructuras: así las llaves de merge
// (`<<: *base`), los alias de una fila o de una celda, los tipos y los nulos se
// aceptan o se rechazan igual que en origin/main (lo fija
// TestDecodeTableRowsMatchesKnownFields). KnownFields no existe al decodificar
// desde un yaml.Node, y reimplementar la semántica de merge y alias sobre el
// nodo para tener una sola lectura no da la misma garantía.
//
// El nodo raíz solo hace falta para calcular las líneas que yaml.v3 descartó
// (yamlDiscardedTail), y esas líneas están siempre menos sangradas que la
// primera del bloque. Si no hay ninguna así, que es el caso de casi toda tabla,
// no se lee el bloque una segunda vez y el nodo es nil.
func decodeTableRows(blockLines []string) ([]yamlTableRowEntry, *yaml.Node, error) {
	text := strings.Join(blockLines, "\n")
	var parsed []yamlTableRowEntry
	decoder := yaml.NewDecoder(strings.NewReader(text))
	decoder.KnownFields(true)
	if err := decoder.Decode(&parsed); err != nil {
		return nil, nil, err
	}
	if !hasUnderIndentedLine(blockLines) {
		return parsed, nil, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil || len(doc.Content) == 0 {
		return parsed, nil, nil
	}
	return parsed, doc.Content[0], nil
}

// hasUnderIndentedLine dice si alguna línea no vacía ni de comentario está
// menos sangrada que la primera: la condición necesaria para que yaml.v3 haya
// descartado algo (ver yamlDiscardedTail).
func hasUnderIndentedLine(lines []string) bool {
	first := -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := CalculateIndentLevel(line)
		if first < 0 {
			first = indent
			continue
		}
		if indent < first {
			return true
		}
	}
	return false
}
