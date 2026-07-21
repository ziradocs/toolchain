// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// ChecklistParser maneja elementos de listas de tareas/checklists
type ChecklistParser struct{}

// CanParse determina si una línea es el inicio de una lista de tareas
func (p *ChecklistParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// Strict mode: CHECKLIST keyword
	if mode == "strict" && strings.HasPrefix(trimmed, "CHECKLIST") {
		return true
	}

	// Both modes: Markdown checklist syntax
	return p.isChecklistItem(trimmed)
}

// Parse parsea una lista de tareas desde las líneas proporcionadas
func (p *ChecklistParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	element := ast.NewChecklistElement(pos)
	consumed := 0
	line := strings.TrimSpace(ctx.Lines[startIndex])

	if ctx.Mode == "strict" && strings.HasPrefix(line, "CHECKLIST") {
		// Skip CHECKLIST line
		consumed++
		startIndex++

		// Process indented checklist items
		consumed += p.parseStrictChecklist(ctx.Lines, startIndex, element)
	} else {
		// Markdown-style parsing
		consumed = p.parseMarkdownChecklist(ctx.Lines, startIndex, element)
	}

	// Set end position
	if consumed > 0 {
		element.EndPosition = diagnostics.NewPosition(startIndex+consumed, 1)
	}

	return &ParseResult{
		Element:       element,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// parseStrictChecklist parsea una lista de tareas en modo estricto
func (p *ChecklistParser) parseStrictChecklist(lines []string, startIndex int, element *ast.ChecklistElement) int {
	consumed := 0
	expectedIndent := -1
	var currentItem *ast.ChecklistItem

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

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
		}

		// Check if this line should be part of the checklist block
		if expectedIndent > 0 && currentIndent < expectedIndent && trimmed != "" {
			break
		}

		// Check if line starts a new element
		if IsNewElement(line, "strict") {
			break
		}

		// Parse checklist item
		if p.isStrictChecklistItem(trimmed) {
			content, checked := p.parseStrictChecklistContent(trimmed)
			if content != "" {
				itemPos := diagnostics.NewPosition(i+1, 1)
				item := ast.NewChecklistItem(itemPos, content, checked)

				if currentIndent == expectedIndent {
					// Main level item
					element.Items = append(element.Items, *item)
					currentItem = &element.Items[len(element.Items)-1]
				} else if currentIndent > expectedIndent && currentItem != nil {
					// Sub-item
					subItem := ast.NewChecklistItem(itemPos, content, checked)
					currentItem.SubItems = append(currentItem.SubItems, *subItem)
				}
			}
		} else if currentIndent >= expectedIndent && currentItem != nil {
			// Continuation line for current item
			currentItem.Content += " " + trimmed
		}

		consumed++
	}

	return consumed
}

// parseMarkdownChecklist parsea una lista de tareas en formato Markdown
func (p *ChecklistParser) parseMarkdownChecklist(lines []string, startIndex int, element *ast.ChecklistElement) int {
	consumed := 0
	baseIndent := -1
	var currentItem *ast.ChecklistItem

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Stop at empty lines
		if trimmed == "" {
			// Allow one empty line within checklist
			if i+1 < len(lines) && p.isChecklistItem(strings.TrimSpace(lines[i+1])) {
				consumed++
				continue
			}
			break
		}

		// Check if this is a checklist item
		if !p.isChecklistItem(trimmed) {
			break
		}

		// Calculate indentation
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if baseIndent == -1 {
			baseIndent = indent
		}

		// Extract content and checked status
		content, checked := p.parseChecklistContent(trimmed)
		if content != "" {
			itemPos := diagnostics.NewPosition(i+1, 1)
			item := ast.NewChecklistItem(itemPos, content, checked)

			// Si es un elemento principal (nivel base)
			if indent == baseIndent {
				element.Items = append(element.Items, *item)
				currentItem = &element.Items[len(element.Items)-1]
			} else if indent > baseIndent && currentItem != nil {
				// Sub-item
				subItem := ast.NewChecklistItem(itemPos, content, checked)
				currentItem.SubItems = append(currentItem.SubItems, *subItem)
			}
		}

		consumed++
	}

	return consumed
}

// isChecklistItem verifica si una línea es un item de checklist
func (p *ChecklistParser) isChecklistItem(line string) bool {
	// Markdown checklist format: - [ ] or - [x] or - [X]
	re := regexp.MustCompile(`^[-*+]\s*\[([xX\s])\]\s*(.*)`)
	return re.MatchString(line)
}

// parseChecklistContent extrae el contenido y estado de un item de checklist
func (p *ChecklistParser) parseChecklistContent(line string) (string, bool) {
	re := regexp.MustCompile(`^[-*+]\s*\[([xX\s])\]\s*(.*)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 3 {
		return "", false
	}

	checked := strings.ToLower(matches[1]) == "x"
	content := strings.TrimSpace(matches[2])

	return content, checked
}

// isStrictChecklistItem verifica si una línea es un item de checklist en modo estricto
func (p *ChecklistParser) isStrictChecklistItem(line string) bool {
	// Strict mode format: [x] or [ ] or [X] at the beginning
	re := regexp.MustCompile(`^\[([xX\s])\]\s*(.*)`)
	return re.MatchString(line)
}

// parseStrictChecklistContent extrae el contenido y estado de un item de checklist en modo estricto
func (p *ChecklistParser) parseStrictChecklistContent(line string) (string, bool) {
	re := regexp.MustCompile(`^\[([xX\s])\]\s*(.*)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 3 {
		return "", false
	}

	checked := strings.ToLower(matches[1]) == "x"
	content := strings.TrimSpace(matches[2])

	return content, checked
}
