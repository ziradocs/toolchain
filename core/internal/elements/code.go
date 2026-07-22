// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// CodeParser handles parsing of code blocks
type CodeParser struct{}

// CanParse determines if the current line starts a code block
func (p *CodeParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// Strict mode: CODE keyword
	if mode == "strict" {
		return strings.HasPrefix(trimmed, "CODE")
	}

	// Flex mode: ```language fenced code blocks
	if mode == "flex" {
		return strings.HasPrefix(trimmed, "```")
	}

	return false
}

// Parse extracts a code block element
func (p *CodeParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	if ctx.Mode == "strict" {
		element, consumed := p.parseStrictCode(ctx, startIndex, pos, line)
		return &ParseResult{
			Element:       element,
			ConsumedLines: consumed,
			Error:         nil,
		}
	} else {
		element, consumed := p.parseFlexCode(ctx, startIndex, pos, line)
		return &ParseResult{
			Element:       element,
			ConsumedLines: consumed,
			Error:         nil,
		}
	}
}

// parseStrictCode handles strict mode code parsing: CODE language
func (p *CodeParser) parseStrictCode(ctx *ParseContext, startIndex int, pos diagnostics.Position, line string) (ast.Element, int) {
	parts := strings.Fields(line)
	consumedLines := 1 // skip CODE line

	// Extract language if specified
	language := ""
	if len(parts) > 1 {
		language = parts[1]
	}
	var content strings.Builder
	expectedIndent := -1 // Auto-detect indentation level

	// Collect indented lines as code content
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]
		currentIndent := CalculateIndentLevel(line)
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines
		if trimmedLine == "" {
			content.WriteString("\n")
			consumedLines++
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

		// Check if this line should be part of the code block
		if expectedIndent > 0 && currentIndent < expectedIndent {
			break
		}

		// Remove the expected indentation from the line
		if expectedIndent > 0 && currentIndent >= expectedIndent {
			// Remove the base indentation but preserve any extra indentation
			lineWithoutBaseIndent := line
			if strings.HasPrefix(line, strings.Repeat(" ", expectedIndent)) {
				lineWithoutBaseIndent = line[expectedIndent:]
			} else if strings.HasPrefix(line, "\t") && expectedIndent == 4 {
				lineWithoutBaseIndent = line[1:]
			}
			content.WriteString(lineWithoutBaseIndent)
		} else {
			content.WriteString(trimmedLine)
		}
		content.WriteString("\n")
		consumedLines++
	}

	codeElement := ast.NewCodeElement(pos, language, strings.TrimSuffix(content.String(), "\n"))
	return codeElement, consumedLines
}

// parseFlexCode handles flex mode code parsing: ```language
func (p *CodeParser) parseFlexCode(ctx *ParseContext, startIndex int, pos diagnostics.Position, line string) (ast.Element, int) {
	// Extract language from ```language
	language := strings.TrimSpace(line[3:])
	consumedLines := 1 // skip opening ``` line

	var content strings.Builder

	// Collect content until closing ```
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]

		if strings.TrimSpace(line) == "```" {
			consumedLines++ // count closing ```
			break
		}

		if content.Len() > 0 {
			content.WriteString("\n")
		}
		content.WriteString(line)
		consumedLines++
	}

	codeElement := ast.NewCodeElement(pos, language, content.String())
	return codeElement, consumedLines
}
