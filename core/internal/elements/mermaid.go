// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// MermaidParser maneja el parsing de diagramas Mermaid
type MermaidParser struct{}

// CanParse determina si puede parsear una línea como Mermaid
func (p *MermaidParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	switch mode {
	case "strict":
		return trimmed == "<<mermaid>>"
	case "flex":
		// En flex mode, soportar tanto <<mermaid>> como ```mermaid
		if trimmed == "<<mermaid>>" {
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

	pos := ctx.Position(startIndex)
	openingLine := strings.TrimSpace(ctx.Lines[startIndex])

	// Detectar formato: <<mermaid>> o ```mermaid
	isMarkdownFormat := strings.HasPrefix(openingLine, "```mermaid") || strings.HasPrefix(openingLine, "````mermaid")

	consumedLines := 1 // skip opening line (<<mermaid>> or ```mermaid)
	var contentLines []string

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

			contentLines = append(contentLines, line)
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

			// Check if this line should be processed based on indentation
			// ALWAYS check ShouldProcessLine to detect when indentation changes or stops
			if !indentDetector.ShouldProcessLine(line, false, i+1, "MERMAID-PARSER") {
				break
			}

			// Mermaid es un bloque opaco. El separador `---` es válido en su
			// frontmatter y no puede terminarlo antes de <<end>>. Conservamos la
			// línea cruda: la dedent común se aplica una sola vez al final.
			contentLines = append(contentLines, line)
			consumedLines++
		}
	}

	contentStr := dedentMermaidContent(contentLines)
	diagramType := detectMermaidType(contentStr)

	// Crear elemento Mermaid usando el constructor existente
	mermaid := ast.NewMermaidElement(pos, diagramType, contentStr)

	return &ParseResult{
		Element:       mermaid,
		ConsumedLines: consumedLines,
		Error:         nil,
	}
}

// dedentMermaidContent aplica solo el dedent común del bloque. Mermaid usa
// sangría como sintaxis (mindmap y frontmatter YAML), así que TrimSpace por
// línea o descartar blancos internos corrompe el diagrama y hace que fmt
// persista esa corrupción en el fuente.
func dedentMermaidContent(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}

	common := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := CalculateIndentLevel(line)
		if common == -1 || indent < common {
			common = indent
		}
	}
	if common <= 0 {
		return strings.Join(lines, "\n")
	}
	for i, line := range lines {
		cut := 0
		for cut < len(line) && cut < common && (line[cut] == ' ' || line[cut] == '\t') {
			cut++
		}
		lines[i] = line[cut:]
	}
	return strings.Join(lines, "\n")
}

// detectMermaidType mira solo la primera instrucción real. Un frontmatter
// YAML opcional precedido por --- no es una instrucción Mermaid.
func detectMermaidType(content string) string {
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter {
			continue
		}
		keyword := strings.Fields(strings.ToLower(trimmed))
		if len(keyword) == 0 {
			return "unknown"
		}
		switch keyword[0] {
		case "flowchart", "graph":
			return "flowchart"
		case "sequencediagram":
			return "sequence"
		case "classdiagram":
			return "class"
		case "statediagram", "statediagram-v2":
			return "state"
		case "gitgraph":
			return "git"
		case "mindmap", "timeline", "erdiagram", "quadrantchart", "xychart-beta", "sankey-beta", "c4context", "c4container", "c4component", "c4dynamic", "gantt", "pie", "journey":
			return keyword[0]
		default:
			return "unknown"
		}
	}
	return "unknown"
}
