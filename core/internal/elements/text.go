// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TextParser maneja elementos de texto (fallback parser)
type TextParser struct{}

// CanParse siempre retorna true ya que es el parser de fallback
func (p *TextParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// In strict mode, handle TEXT keyword
	if mode == "strict" && strings.HasPrefix(trimmed, "TEXT") {
		return true
	}

	// In flex mode, any non-empty line that's not handled by other parsers
	// becomes text. This is the fallback parser.
	if mode == "flex" && trimmed != "" {
		return true
	}

	return false
}

// Parse parsea texto desde las líneas proporcionadas
func (p *TextParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])
	consumed := 0

	var content strings.Builder

	if ctx.Mode == "strict" && strings.HasPrefix(line, "TEXT") { // Skip TEXT line
		consumed++
		startIndex++

		// Collect indented lines as content
		expectedIndent := -1 // Auto-detect indentation level
		for i := startIndex; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
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

			// Check if this line should be part of the text block
			if expectedIndent > 0 && currentIndent < expectedIndent {
				break
			}

			// Add the content with space separator
			if trimmedLine != "" {
				content.WriteString(trimmedLine)
				content.WriteString(" ")
			}
			consumed++
		}
	} else {
		// Flex mode: collect consecutive text lines but be smart about lists
		for i := startIndex; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmed := strings.TrimSpace(line)

			// Stop at empty lines
			if trimmed == "" {
				break
			}

			// Stop if another element type is detected
			if p.isOtherElementType(trimmed, ctx.Mode) {
				break
			}

			// In flex mode, stop if we detect a numbered list that should be separate
			if i > startIndex && p.isStartOfNumberedList(trimmed) {
				break
			}

			// Process line as normal text content
			if content.Len() > 0 {
				content.WriteString(" ")
			}
			content.WriteString(trimmed)
			consumed++
		}
	}

	// Don't create empty text elements
	text := strings.TrimSpace(content.String())
	if text == "" {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: consumed,
			Error:         nil,
		}
	}

	return &ParseResult{
		Element:       ast.NewTextElement(pos, text),
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// isOtherElementType checks if a line indicates another element type
func (p *TextParser) isOtherElementType(line string, mode string) bool {
	// Check for various element indicators

	// Special element tags (<<chart>>, <<map>>, <<mermaid>>, etc.)
	if strings.HasPrefix(line, "<<") && strings.Contains(line, ">>") {
		return true
	}

	// Subsection headers in DocLang (##, ###, ####, etc.) - but NOT # (H1)
	// H1 is handled separately and creates new sections
	if strings.HasPrefix(line, "##") {
		return true
	}

	// Special blocks
	if strings.HasPrefix(line, ":::") {
		return true
	}

	// Directives
	if strings.HasPrefix(line, "@") {
		return true
	}

	// Lists
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}

	// Code blocks (markdown style)
	if strings.HasPrefix(line, "```") {
		return true
	}

	// Images (markdown style)
	if strings.HasPrefix(line, "![") && strings.Contains(line, "](") {
		return true
	}

	// Tables (markdown style)
	if strings.Contains(line, "|") && strings.Count(line, "|") >= 2 {
		return true
	}

	// Strict mode keywords
	if mode == "strict" {
		keywords := []string{"TEXT", "POINTS", "CODE", "IMAGE", "TABLE", "MERMAID", "CHART", "MAP"}
		for _, keyword := range keywords {
			if strings.HasPrefix(line, keyword+" ") || line == keyword {
				return true
			}
		}
	}

	return false
}

// isStartOfNumberedList checks if a line starts a numbered list (1. 2. etc.)
func (p *TextParser) isStartOfNumberedList(line string) bool {
	if len(line) < 3 {
		return false
	}

	// Check for numbered lists (1. 2. 3. etc.)
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

	return false
}
