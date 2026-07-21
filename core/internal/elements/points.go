// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// PointsParser maneja elementos de listas/puntos
type PointsParser struct{}

// CanParse determina si una línea es el inicio de una lista de puntos
func (p *PointsParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// Strict mode: POINTS keyword
	if mode == "strict" && strings.HasPrefix(trimmed, "POINTS") {
		return true
	}

	// Both modes: Markdown list syntax
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return true
	}

	// Numbered lists
	if len(trimmed) > 2 && (trimmed[1] == '.' || trimmed[2] == '.') {
		for i, char := range trimmed {
			if char == '.' {
				if i > 0 && i+1 < len(trimmed) && trimmed[i+1] == ' ' {
					return true
				}
				break
			}
			if char < '0' || char > '9' {
				if i == 0 {
					return false
				}
				break
			}
		}
	}

	return false
}

// Parse parsea una lista de puntos desde las líneas proporcionadas
func (p *PointsParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}
	pos := diagnostics.NewPosition(startIndex+1, 1)
	element := ast.NewPointsElement(pos)
	consumed := 0
	line := strings.TrimSpace(ctx.Lines[startIndex])

	if ctx.Mode == "strict" && strings.HasPrefix(line, "POINTS") {
		// Skip POINTS line
		consumed++
		startIndex++

		// Detectar tipo de lista basado en el primer elemento
		firstItemProcessed := false
		var currentItem *ast.PointItem
		expectedIndent := -1 // Auto-detect indentation level

		// Collect indented lines
		for i := startIndex; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmed := strings.TrimSpace(line)

			if ctx.Logger != nil {
				ctx.Logger.Debug("POINTS-PARSER", " Line %d: processing line: '%s'", i+1, line)
			}
			// Calculate current line indentation
			currentIndent := CalculateIndentLevel(line)
			// Skip empty lines
			if trimmed == "" {
				consumed++
				continue
			}

			// Auto-detect expected indentation from first non-empty line
			if expectedIndent == -1 && currentIndent > 0 {
				expectedIndent = currentIndent
				if ctx.Logger != nil {
					ctx.Logger.Debug("POINTS-PARSER", " Line %d: auto-detected indentation level: %d", i+1, expectedIndent)
				}
			}

			// Check if this line should be part of the points block
			if expectedIndent > 0 && currentIndent < expectedIndent && trimmed != "" {
				if ctx.Logger != nil {
					ctx.Logger.Debug("POINTS-PARSER", " Line %d: end of indented block (expected: %d, got: %d), breaking", i+1, expectedIndent, currentIndent)
				}
				break
			}

			// If we haven't detected indentation yet and line has no indentation, break
			if expectedIndent == -1 && currentIndent == 0 && trimmed != "" {
				if ctx.Logger != nil {
					ctx.Logger.Debug("POINTS-PARSER", " Line %d: non-indented content found, breaking", i+1)
				}
				break
			}

			// Check if this is any type of list item (-, numbers, letters)
			if p.isListItem(trimmed) {
				indent := CalculateIndentLevel(line) // Detectar tipo de lista en el primer elemento principal
				if !firstItemProcessed && indent == expectedIndent {
					element.ListType = p.detectListType(trimmed)
					firstItemProcessed = true
				}

				content := p.extractListContent(trimmed)

				if content != "" {
					itemPos := diagnostics.NewPosition(i+1, 1)
					item := ast.NewPointItem(itemPos, content)

					// Si es un elemento principal (nivel base)
					if indent == expectedIndent {
						element.Items = append(element.Items, *item)
						currentItem = &element.Items[len(element.Items)-1]
					} else if indent > expectedIndent && currentItem != nil {
						// Es un sub-elemento
						currentItem.SubPoints = append(currentItem.SubPoints, *item)
					}
				}
			}

			consumed++
		}
	} else {
		// Parse Markdown-style list (flex mode or compatibility)
		consumed = p.parseMarkdownList(ctx.Lines, startIndex, element)
	}

	return &ParseResult{
		Element:       element,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// parseMarkdownList parsea una lista en formato Markdown
func (p *PointsParser) parseMarkdownList(lines []string, startIndex int, element *ast.PointsElement) int {
	consumed := 0
	baseIndent := -1
	firstItemProcessed := false
	var currentItem *ast.PointItem

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Stop at empty lines
		if trimmed == "" {
			// Allow one empty line within list
			if i+1 < len(lines) && p.isListItem(strings.TrimSpace(lines[i+1])) {
				consumed++
				continue
			}
			break
		}

		// Check if this is a list item
		if !p.isListItem(trimmed) {
			break
		}

		// Calculate indentation
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if baseIndent == -1 {
			baseIndent = indent
		}

		// Detectar tipo de lista en el primer elemento principal
		if !firstItemProcessed && indent == baseIndent {
			element.ListType = p.detectListType(trimmed)
			firstItemProcessed = true
		}

		// Extract content
		content := p.extractListContent(trimmed)
		if content != "" {
			itemPos := diagnostics.NewPosition(i+1, 1)
			item := ast.NewPointItem(itemPos, content)

			// Si es un elemento principal (nivel base)
			if indent == baseIndent {
				element.Items = append(element.Items, *item)
				currentItem = &element.Items[len(element.Items)-1]
			} else if indent > baseIndent && currentItem != nil {
				// Es un sub-elemento
				currentItem.SubPoints = append(currentItem.SubPoints, *item)
			} else if indent < baseIndent {
				// Indentación menor que la base, terminar parsing
				break
			}
		}

		consumed++
	}

	return consumed
}

// IsListItem checks if a line is a list item (public method)
func (p *PointsParser) IsListItem(line string) bool {
	return p.isListItem(line)
}

// ExtractListContent extracts the content from a list item line (public method)
func (p *PointsParser) ExtractListContent(line string) string {
	return p.extractListContent(line)
}

// isListItem checks if a line is a list item
func (p *PointsParser) isListItem(line string) bool {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}

	// Check for numbered lists (1. 2. etc.)
	if len(line) > 2 {
		for i, char := range line {
			if char == '.' && i > 0 {
				// Check if followed by space
				if i+1 < len(line) && line[i+1] == ' ' {
					return true
				}
				break
			}
			if char < '0' || char > '9' {
				break
			}
		}
	}

	// Check for alphabetic lists (a. b. c. etc.)
	if len(line) >= 3 && line[1] == '.' && line[2] == ' ' {
		char := line[0]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			return true
		}
	}

	return false
}

// extractListContent extracts the content from a list item line
func (p *PointsParser) extractListContent(line string) string {
	// Handle unordered lists
	if strings.HasPrefix(line, "- ") {
		return strings.TrimSpace(line[2:])
	}
	if strings.HasPrefix(line, "* ") {
		return strings.TrimSpace(line[2:])
	}
	if strings.HasPrefix(line, "+ ") {
		return strings.TrimSpace(line[2:])
	}

	// Handle numbered lists
	dotIndex := strings.Index(line, ". ")
	if dotIndex > 0 {
		return strings.TrimSpace(line[dotIndex+2:])
	}

	return ""
}

// detectListType detecta si una línea indica una lista ordenada o no ordenada
func (p *PointsParser) detectListType(line string) string {
	trimmed := strings.TrimSpace(line)

	// Lista no ordenada (bullets)
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return "unordered"
	}

	// Lista numerada (1. 2. 3. etc.)
	if len(trimmed) > 2 {
		for i, char := range trimmed {
			if char == '.' && i > 0 {
				// Check if followed by space
				if i+1 < len(trimmed) && trimmed[i+1] == ' ' {
					return "ordered"
				}
				break
			}
			if char < '0' || char > '9' {
				break
			}
		}
	}

	// Lista alfabética (a. b. c. etc.) - también se considera ordenada
	if len(trimmed) >= 3 && trimmed[1] == '.' && trimmed[2] == ' ' {
		char := trimmed[0]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			return "ordered"
		}
	}

	return "unordered" // default
}

// isSubItem detecta si una línea es un sub-elemento basado en la indentación
func (p *PointsParser) isSubItem(line string, baseIndent int) bool {
	trimmed := strings.TrimSpace(line)
	if !p.isListItem(trimmed) {
		return false
	}

	indent := CalculateIndentLevel(line)
	return indent > baseIndent
}
