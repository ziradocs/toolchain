// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TableParser maneja elementos de tabla (YAML y Markdown)
type TableParser struct{}

// CanParse determina si una línea es el inicio de una tabla
func (p *TableParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// Strict mode: TABLE keyword
	if mode == "strict" && strings.HasPrefix(trimmed, "TABLE") {
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

	// Check if it's a strict mode TABLE declaration
	if ctx.Mode == "strict" && strings.HasPrefix(line, "TABLE") {
		consumed++
		startIndex++

		// Parse YAML-style table
		headers, rows, caption, label, yamlConsumed := p.parseYAMLTable(ctx.Lines, startIndex)
		table.Headers = headers
		table.Rows = rows
		table.Caption = caption
		table.Label = label
		consumed += yamlConsumed
	} else {
		// Parse Markdown-style table
		headers, rows, markdownConsumed := p.parseMarkdownTable(ctx.Lines, startIndex)
		table.Headers = headers
		table.Rows = rows
		consumed = markdownConsumed
	}

	return &ParseResult{
		Element:       table,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// parseYAMLTable parsea una tabla en formato YAML (strict mode)
func (p *TableParser) parseYAMLTable(lines []string, startIndex int) ([]string, [][]string, string, string, int) {
	// Inicializados como slices vacíos (no nil): Headers/Rows no llevan
	// omitempty en el AST, así que un valor nil se serializaría como
	// JSON null en vez de [] (issue #8 - viola el JSON Schema del contrato).
	headers := []string{}
	rows := [][]string{}
	var caption string
	var label string
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

	return headers, rows, caption, label, consumed
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
