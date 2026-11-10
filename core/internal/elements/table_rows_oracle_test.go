// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// oracleParseYAMLTable es parseYAMLTable tal como estaba en main antes de
// #495, copiado como oráculo para FuzzTableRowsMatchMain. Lo único agregado
// son las marcas de "dropped": las líneas que main consumía o ignoraba sin
// leer (un valor en línea de rows:, una línea "[" de la lista que no cierra,
// una fila de corchetes completa que el bucle de afuera ignoraba). Es lo que
// #488/#495 cambian a propósito; fuera de eso, el parser nuevo tiene que leer
// exactamente lo mismo.
func oracleParseYAMLTable(ctx *ParseContext, startIndex int, pos diagnostics.Position) (headers []string, rows [][]string, caption, label string, explicitCells [][]ast.TableCell, tableRows []ast.TableRow, rowPositions []diagnostics.Position, diags []diagnostics.Diagnostic, consumed int, dropped bool) {
	// Initialized as empty slices (not nil): Headers/Rows have no omitempty
	// in the AST, so a nil value would serialize as JSON null instead of []
	// (issue #8 - violates the contract's JSON Schema).
	headers = []string{}
	rows = [][]string{}
	// Named differently from "cells" on purpose: the "|" fallback branch
	// below already declares a local variable "cells" ([]string, one row
	// split by pipes) unrelated to this one — same name in nested scopes
	// would still compile (shadowing), but would confuse a reader.
	var legacyDeclared, newDeclared bool
	// delimiterSeen: the markdown delimiter row (|---|---|) after a pipe
	// header has already been skipped. Only that one row is syntax.
	var delimiterSeen bool
	consumed = 0
	expectedIndent := -1 // Auto-detect indentation level
	lines := ctx.Lines

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
			}
		} else if strings.HasPrefix(trimmedLine, "rows:") {
			legacyDeclared = true
			// Marca: main ignoraba un valor en línea (#488).
			if v := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "rows:")); v != "" && v != "[" {
				dropped = true
			}
			// Process rows array
			consumed++
			i++
			for i < len(lines) {
				rowLine := lines[i]
				if !strings.HasPrefix(rowLine, "      [") {
					i-- // Back up one line
					break
				}

				rowTrimmed := strings.TrimSpace(rowLine)
				if strings.HasPrefix(rowTrimmed, "[") && strings.HasSuffix(rowTrimmed, "],") {
					rowTrimmed = strings.TrimSuffix(rowTrimmed, ",")
				}
				if strings.HasPrefix(rowTrimmed, "[") && strings.HasSuffix(rowTrimmed, "]") {
					rowStr := trimInlineArrayBrackets(rowTrimmed)
					rows = append(rows, splitInlineArray(rowStr))
					rowPositions = append(rowPositions, ctx.Position(i))
				} else {
					dropped = true // Marca: main consumía la línea sin leerla.
				}
				consumed++
				i++
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
			if parsedCells, cellDiags, ok := parseCellsYAML(cellsBlockLines, pos); ok {
				explicitCells = parsedCells
				diags = append(diags, cellDiags...)
			} else if len(cellsBlockLines) > 0 {
				diags = append(diags, diagnostics.NewWarning(
					`The "cells:" block is not valid YAML and was ignored; the table will be empty`,
					pos, "table-parser").WithRuleID("TABLE004"))
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
			var parsed []yamlTableRowEntry
			decoder := yaml.NewDecoder(strings.NewReader(strings.Join(blockLines, "\n")))
			decoder.KnownFields(true)
			if err := decoder.Decode(&parsed); err != nil || len(parsed) == 0 {
				diags = append(diags, diagnostics.NewError("invalid tableRows YAML: expected nonempty rows with known fields", pos, "table-parser"))
			} else {
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
		} else if strings.HasPrefix(trimmedLine, "label:") {
			// issue #239: identificador de referencia cruzada (p. ej. "tbl:ventas").
			labelStr := strings.TrimPrefix(trimmedLine, "label:")
			label = strings.Trim(strings.TrimSpace(labelStr), "\"")
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
		} else if strings.HasPrefix(trimmedLine, "[") {
			dropped = true // Marca: main ignoraba una línea "[…" sin "|".
		}

		consumed++
	}

	if newDeclared && legacyDeclared {
		diags = append(diags, diagnostics.NewError("tableRows cannot be combined with authored headers/rows/cells", pos, "table-parser"))
	}
	return headers, rows, caption, label, explicitCells, tableRows, rowPositions, diags, consumed, dropped
}
