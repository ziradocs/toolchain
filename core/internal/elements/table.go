// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TableParser maneja elementos de tabla (YAML y Markdown)
type TableParser struct{}

// CanParse determina si una línea es el inicio de una tabla
func (p *TableParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// TABLE keyword: reconocido tanto en strict como en flex. doclang SOLO
	// parsea flex (DocumentFlexParser.parseSectionContent fija ctx.Mode =
	// "flex"), así que un bloque TABLE limitado a strict era inautorable en
	// doclang — necesario para que la sintaxis `cells:` explícita de celdas
	// fusionadas (issue #20) exista en el CLI que la enterprise CLI/rulepack
	// A11Y realmente consume. Antes de este cambio, un bloque TABLE en flex
	// caía al fallback de TextParser y se renderizaba como texto literal.
	if (mode == "strict" || mode == "flex") && strings.HasPrefix(trimmed, "TABLE") {
		return true
	}

	// Both modes: Markdown table format. Exige "|" inicial (no solo 2+
	// pipes en cualquier posición) — sin esto, TableParser va antes de
	// Quote/Checklist/Points/Text en el registry (GetDefaultRegistry) y le
	// roba líneas legítimas con 2+ pipes que no son tablas, p. ej.
	// "- Compara pandas | numpy | scipy" (issue #245). El 100% de las
	// tablas markdown reales del corpus empiezan cada fila con "|", y
	// strict ya exige lo mismo (parser/strict.go). CanParse solo ve una
	// línea (no puede mirar si la siguiente es una fila separadora
	// "|---|") así que "|" inicial es el único discriminador viable acá.
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

	pos := diagnostics.NewPosition(startIndex+1, 1)
	table := ast.NewTableElement(pos)
	consumed := 0
	line := strings.TrimSpace(ctx.Lines[startIndex])

	// El bloque YAML "TABLE" se reconoce igual en strict y flex (ver
	// CanParse); el modo ya no gatea esta rama, solo la sintaxis de la línea.
	if strings.HasPrefix(line, "TABLE") {
		consumed++
		startIndex++

		// Parse YAML-style table
		headers, rows, caption, label, cellsExplicit, yamlConsumed := p.parseYAMLTable(ctx.Lines, startIndex)
		table.Caption = caption
		table.Label = label
		consumed += yamlConsumed

		if len(cellsExplicit) > 0 {
			// Celdas fusionadas explícitas (issue #20): Cells es la fuente de
			// verdad; Headers/Rows se DERIVAN de Cells (grilla rectangular)
			// para que linter.ElementStructureRule (TABLE003) no reporte un
			// falso positivo de "columnas inconsistentes" sobre una tabla con
			// colspan/rowspan.
			table.Cells = cellsExplicit
			table.Headers, table.Rows = ast.FlattenCellsToRows(cellsExplicit)
		} else {
			table.Headers = headers
			table.Rows = rows
			table.Cells = ast.DeriveCellsFromFlat(headers, rows)
		}
	} else {
		// Parse Markdown-style table
		headers, rows, markdownConsumed := p.parseMarkdownTable(ctx.Lines, startIndex)
		table.Headers = headers
		table.Rows = rows
		table.Cells = ast.DeriveCellsFromFlat(headers, rows)
		consumed = markdownConsumed
	}

	return &ParseResult{
		Element:       table,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// parseYAMLTable parsea una tabla en formato YAML (bloque "TABLE").
// El quinto valor de retorno es no-nil solo si el bloque declaró "cells:"
// explícito (celdas fusionadas, issue #20) — en ese caso el llamador debe
// tratar Cells como fuente de verdad y derivar Headers/Rows de él (ver
// ast.FlattenCellsToRows), ignorando los headers/rows acumulados acá (que
// para un bloque "cells:" quedan vacíos, ya que esa sintaxis no declara
// headers:/rows: por separado).
func (p *TableParser) parseYAMLTable(lines []string, startIndex int) ([]string, [][]string, string, string, [][]ast.TableCell, int) {
	// Inicializados como slices vacíos (no nil): Headers/Rows no llevan
	// omitempty en el AST, así que un valor nil se serializaría como
	// JSON null en vez de [] (issue #8 - viola el JSON Schema del contrato).
	headers := []string{}
	rows := [][]string{}
	var caption string
	var label string
	// Nombre distinto de "cells" a propósito: el branch de fallback "|" más
	// abajo ya declara una variable local "cells" ([]string, una fila
	// partida por pipes) sin relación con esta — un mismo nombre en scopes
	// anidados compilaría igual (shadowing), pero confundiría al lector.
	var explicitCells [][]ast.TableCell
	consumed := 0
	expectedIndent := -1 // Auto-detect indentation level

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
			headersStr := strings.TrimPrefix(trimmedLine, "headers:")
			headersStr = strings.TrimSpace(headersStr)

			// Remove brackets and parse array
			if strings.HasPrefix(headersStr, "[") && strings.HasSuffix(headersStr, "]") {
				headersStr = strings.Trim(headersStr, "[]")
				parts := strings.Split(headersStr, ",")
				for _, part := range parts {
					header := strings.TrimSpace(part)
					header = strings.Trim(header, "\"")
					headers = append(headers, header)
				}
			}
		} else if strings.HasPrefix(trimmedLine, "rows:") {
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
					rowStr := strings.Trim(rowTrimmed, "[]")
					parts := strings.Split(rowStr, ",")
					var row []string
					for _, part := range parts {
						cell := strings.TrimSpace(part)
						cell = strings.Trim(cell, "\"")
						row = append(row, cell)
					}
					rows = append(rows, row)
				}
				consumed++
				i++
			}
			continue
		} else if strings.HasPrefix(trimmedLine, "cells:") {
			// Celdas fusionadas explícitas (issue #20): el valor es una
			// secuencia YAML de filas de celdas, indentada bajo "cells:".
			// Recolectar las líneas mientras excedan la indentación de
			// "cells:" en sí (mismo criterio que usa el loop exterior para
			// delimitar el bloque TABLE completo), luego parsearlas como YAML
			// completo — más robusto que el partido de strings hand-rolled
			// que usan headers:/rows: arriba, necesario porque acá el valor
			// es una estructura anidada (lista de listas de mapas), no un
			// array plano.
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
				if CalculateIndentLevel(rl) <= cellsIndent {
					i-- // Dejar que el loop exterior reprocese esta línea
					break
				}
				cellsBlockLines = append(cellsBlockLines, rl)
				consumed++
				i++
			}
			if parsedCells, ok := parseCellsYAML(cellsBlockLines); ok {
				explicitCells = parsedCells
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
			// Fallback: Parse table row (separated by |) for compatibility
			cells := strings.Split(trimmedLine, "|")
			for j := range cells {
				cells[j] = strings.TrimSpace(cells[j])
			}

			if len(headers) == 0 {
				headers = cells
			} else {
				rows = append(rows, cells)
			}
		}

		consumed++
	}

	return headers, rows, caption, label, explicitCells, consumed
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

// parseCellsYAML parsea el bloque de líneas recolectado bajo "cells:" (una
// secuencia YAML de secuencias de mapas, en flow style) a [][]ast.TableCell.
// Retorna ok=false si el bloque está vacío o no es YAML válido — en ese caso
// el llamador conserva el comportamiento previo (Headers/Rows derivados de
// otras claves del bloque, típicamente vacíos si solo se declaró "cells:"
// malformado).
func parseCellsYAML(blockLines []string) ([][]ast.TableCell, bool) {
	if len(blockLines) == 0 {
		return nil, false
	}

	raw := strings.Join(blockLines, "\n")
	var parsed [][]yamlTableCellEntry
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil || len(parsed) == 0 {
		return nil, false
	}

	cells := make([][]ast.TableCell, len(parsed))
	for i, row := range parsed {
		cells[i] = make([]ast.TableCell, len(row))
		for j, c := range row {
			cells[i][j] = ast.TableCell{
				Content:  c.Content,
				IsHeader: c.Header,
				Scope:    c.Scope,
				ColSpan:  c.Colspan,
				RowSpan:  c.Rowspan,
			}
		}
	}
	return cells, true
}

// parseMarkdownTable parsea una tabla en formato Markdown
func (p *TableParser) parseMarkdownTable(lines []string, startIndex int) ([]string, [][]string, int) {
	// Inicializados como slices vacíos (no nil), ver comentario en parseYAMLTable.
	headers := []string{}
	rows := [][]string{}
	consumed := 0

	for i := startIndex; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Stop at empty lines or lines without |
		if line == "" || !strings.Contains(line, "|") {
			break
		}

		// Skip separator lines (e.g., |---|---|)
		if strings.Contains(line, "---") || strings.Contains(line, ":-:") || strings.Contains(line, ":--") || strings.Contains(line, "--:") {
			consumed++
			continue
		}

		// Parse table row
		cells := strings.Split(line, "|")

		// Clean up cells - remove empty first/last if they exist due to leading/trailing |
		if len(cells) > 0 && strings.TrimSpace(cells[0]) == "" {
			cells = cells[1:]
		}
		if len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
			cells = cells[:len(cells)-1]
		}

		// Trim whitespace from each cell
		for j := range cells {
			cells[j] = strings.TrimSpace(cells[j])
		}

		// First row becomes headers
		if len(headers) == 0 {
			headers = cells
		} else {
			rows = append(rows, cells)
		}

		consumed++
	}

	return headers, rows, consumed
}
