// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// MermaidParser maneja el parsing de diagramas Mermaid
type MermaidParser struct{}

// CanParse determina si puede parsear una línea como Mermaid
func (p *MermaidParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	switch mode {
	case "strict":
		return strings.HasPrefix(trimmed, "<<mermaid>>")
	case "flex":
		// En flex mode, soportar tanto <<mermaid>> como ```mermaid
		if strings.HasPrefix(trimmed, "<<mermaid>>") {
			return true
		}
		// Detectar code blocks de Markdown con lenguaje "mermaid"
		if strings.HasPrefix(trimmed, "```mermaid") || strings.HasPrefix(trimmed, "````mermaid") {
			return true
		}
	}

	return false
}

// Parse parsea un elemento Mermaid
func (p *MermaidParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{Error: nil}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	openingLine := strings.TrimSpace(ctx.Lines[startIndex])

	// Detectar formato: <<mermaid>> o ```mermaid
	isMarkdownFormat := strings.HasPrefix(openingLine, "```mermaid") || strings.HasPrefix(openingLine, "````mermaid")

	consumedLines := 1 // skip opening line (<<mermaid>> or ```mermaid)
	var content strings.Builder

	if isMarkdownFormat {
		// Formato Markdown: ```mermaid ... ```
		// Recoger contenido hasta encontrar ``` o ````
		for i := startIndex + 1; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmed := strings.TrimSpace(line)

			// Terminar al encontrar closing backticks
			if trimmed == "```" || trimmed == "````" {
				consumedLines++
				break
			}

			if content.Len() > 0 {
				content.WriteString("\n")
			}
			content.WriteString(trimmed)
			consumedLines++
		}
	} else {
		// Formato <<mermaid>> - usar indentación o tag de cierre <<end>>
		indentDetector := NewAutoDetectIndentation()

		// Recoger contenido del diagrama
		for i := startIndex + 1; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmedLine := strings.TrimSpace(line)

			// Check for closing tag FIRST
			if trimmedLine == "<<end>>" {
				consumedLines++
				break
			}

			// Check for slide separator (---) - STOP here for SlideLang
			if trimmedLine == "---" {
				break
			}

			// Check if this line should be processed based on indentation
			// ALWAYS check ShouldProcessLine to detect when indentation changes or stops
			if !indentDetector.ShouldProcessLine(line, false, i+1, "MERMAID-PARSER") {
				break
			}

			// Skip empty lines
			if trimmedLine == "" {
				consumedLines++
				continue
			}

			if content.Len() > 0 {
				content.WriteString("\n")
			}
			content.WriteString(trimmedLine)

			consumedLines++
		}
	}

	// Detectar tipo de diagrama de forma más robusta
	diagramType := "graph"
	contentStr := content.String()

	// Clean and normalize the content for Mermaid standard
	contentStr = p.cleanAndNormalizeMermaidContent(contentStr)

	firstLine := strings.ToLower(strings.TrimSpace(strings.Split(contentStr, "\n")[0]))

	if strings.HasPrefix(firstLine, "flowchart") {
		diagramType = "flowchart"
	} else if strings.HasPrefix(firstLine, "graph ") {
		diagramType = "flowchart"
	} else if strings.HasPrefix(firstLine, "gantt") {
		diagramType = "gantt"
	} else if strings.Contains(contentStr, "sequenceDiagram") {
		diagramType = "sequence"
	} else if strings.Contains(contentStr, "classDiagram") {
		diagramType = "class"
	} else if strings.Contains(contentStr, "stateDiagram") {
		diagramType = "state"
	} else if strings.Contains(contentStr, "gitgraph") {
		diagramType = "git"
	} else if strings.Contains(contentStr, "pie title") {
		diagramType = "pie"
	} else if strings.Contains(contentStr, "journey") {
		diagramType = "journey"
	} else {
		// Auto-detect based on content patterns
		if strings.Contains(contentStr, "-->") || strings.Contains(contentStr, "---") {
			diagramType = "flowchart"
		} else if strings.Contains(contentStr, "section ") &&
			(strings.Contains(contentStr, ":active") || strings.Contains(contentStr, "dateFormat")) {
			diagramType = "gantt"
		}
	}

	// Crear elemento Mermaid usando el constructor existente
	mermaid := ast.NewMermaidElement(pos, diagramType, contentStr)

	return &ParseResult{
		Element:       mermaid,
		ConsumedLines: consumedLines,
		Error:         nil,
	}
}

// cleanAndNormalizeMermaidContent normalizes Mermaid content to be ready for direct JSON use
func (p *MermaidParser) cleanAndNormalizeMermaidContent(content string) string {
	if content == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	normalizedLines := make([]string, 0, len(lines))

	for _, line := range lines {
		// Remove excessive whitespace but preserve structure
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Add line to normalized content
		normalizedLines = append(normalizedLines, trimmed)
	}

	// Join with clean newlines
	result := strings.Join(normalizedLines, "\n")

	// Ensure content is properly formatted for Mermaid
	result = p.ensureMermaidHeader(result)

	// Apply automatic syntax fixes for common issues
	result = p.applySyntaxFixes(result)

	return result
}

// ensureMermaidHeader ensures the diagram has a proper header if needed
func (p *MermaidParser) ensureMermaidHeader(content string) string {
	if content == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	firstLine := strings.ToLower(strings.TrimSpace(lines[0]))

	// Check if it already has a proper header
	if strings.HasPrefix(firstLine, "flowchart") ||
		strings.HasPrefix(firstLine, "graph ") ||
		strings.HasPrefix(firstLine, "gantt") ||
		strings.HasPrefix(firstLine, "sequencediagram") ||
		strings.HasPrefix(firstLine, "classdiagram") ||
		strings.HasPrefix(firstLine, "statediagram") ||
		strings.HasPrefix(firstLine, "pie") ||
		strings.HasPrefix(firstLine, "journey") ||
		strings.HasPrefix(firstLine, "gitgraph") {
		return content
	}

	// Auto-detect and add header if needed
	if strings.Contains(content, "-->") || strings.Contains(content, "---") {
		// Looks like a flowchart
		return "flowchart TD\n" + content
	}

	// Return as-is if we can't determine the type
	return content
}

// applySyntaxFixes applies automatic corrections for common Mermaid syntax issues
func (p *MermaidParser) applySyntaxFixes(content string) string {
	if content == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	correctedLines := make([]string, 0, len(lines))

	// Detect diagram type for specific fixes
	isFlowchart := false
	firstLine := strings.ToLower(strings.TrimSpace(lines[0]))
	if strings.HasPrefix(firstLine, "flowchart") || strings.HasPrefix(firstLine, "graph ") {
		isFlowchart = true
	} else {
		// Check if content looks like a flowchart
		for _, line := range lines {
			if strings.Contains(line, "-->") || strings.Contains(line, "---") {
				isFlowchart = true
				break
			}
		}
	}

	for _, line := range lines {
		correctedLine := line

		if isFlowchart {
			correctedLine = p.fixFlowchartSyntax(correctedLine)
		}

		correctedLines = append(correctedLines, correctedLine)
	}

	return strings.Join(correctedLines, "\n")
}

// fixFlowchartSyntax corrects common flowchart syntax issues
func (p *MermaidParser) fixFlowchartSyntax(line string) string {
	// Fix numbered lists in node labels that cause "Unsupported markdown: list" error
	// Using single quotes to escape special characters (cleaner and more readable)
	// This preserves the original content while making it Mermaid-compatible

	// Pattern for rectangular nodes: A[1. Text] -> A['1. Text']
	numberedNodePattern := regexp.MustCompile(`([A-Z][A-Z0-9]*)\[(\d+\.\s+[^\]]+)\]`)
	if numberedNodePattern.MatchString(line) {
		correctedLine := numberedNodePattern.ReplaceAllStringFunc(line, func(match string) string {
			parts := numberedNodePattern.FindStringSubmatch(match)
			if len(parts) >= 3 {
				nodeId := parts[1]  // A, B, C, etc.
				content := parts[2] // "1. Text"
				return nodeId + "['" + content + "']"
			}
			return match
		})
		return correctedLine
	}

	// Pattern for circle nodes: A((1. Text)) -> A(('1. Text'))
	numberedCirclePattern := regexp.MustCompile(`([A-Z][A-Z0-9]*)\(\((\d+\.\s+[^\)]+)\)\)`)
	if numberedCirclePattern.MatchString(line) {
		correctedLine := numberedCirclePattern.ReplaceAllStringFunc(line, func(match string) string {
			parts := numberedCirclePattern.FindStringSubmatch(match)
			if len(parts) >= 3 {
				nodeId := parts[1]
				content := parts[2]
				return nodeId + "(('" + content + "'))"
			}
			return match
		})
		return correctedLine
	}

	// Pattern for diamond nodes: A{1. Text} -> A{'1. Text'}
	numberedDiamondPattern := regexp.MustCompile(`([A-Z][A-Z0-9]*)\{(\d+\.\s+[^\}]+)\}`)
	if numberedDiamondPattern.MatchString(line) {
		correctedLine := numberedDiamondPattern.ReplaceAllStringFunc(line, func(match string) string {
			parts := numberedDiamondPattern.FindStringSubmatch(match)
			if len(parts) >= 3 {
				nodeId := parts[1]
				content := parts[2]
				return nodeId + "{'" + content + "'}"
			}
			return match
		})
		return correctedLine
	}

	// Pattern for parentheses nodes: A(1. Text) -> A('1. Text')
	numberedParenPattern := regexp.MustCompile(`([A-Z][A-Z0-9]*)\((\d+\.\s+[^\)]+)\)`)
	if numberedParenPattern.MatchString(line) {
		correctedLine := numberedParenPattern.ReplaceAllStringFunc(line, func(match string) string {
			parts := numberedParenPattern.FindStringSubmatch(match)
			if len(parts) >= 3 {
				nodeId := parts[1]
				content := parts[2]
				return nodeId + "('" + content + "')"
			}
			return match
		})
		return correctedLine
	}

	return line
}
