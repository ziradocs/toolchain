// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
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

	pos := ctx.Position(startIndex)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	if ctx.Mode == "strict" {
		element, consumed, err := p.parseStrictCode(ctx, startIndex, pos, line)
		return &ParseResult{
			Element:       element,
			ConsumedLines: consumed,
			Error:         err,
		}
	} else {
		element, consumed, err := p.parseFlexCode(ctx, startIndex, pos, line)
		return &ParseResult{
			Element:       element,
			ConsumedLines: consumed,
			Error:         err,
		}
	}
}

// parseStrictCode handles strict mode code parsing: CODE language
func (p *CodeParser) parseStrictCode(ctx *ParseContext, startIndex int, pos diagnostics.Position, line string) (ast.Element, int, error) {
	parts := strings.Fields(line)
	consumedLines := 1 // skip CODE line

	// `CODE <lenguaje> <archivo>`: el tercer token es el nombre de archivo
	// (code-filename-v1). Antes se descartaba en silencio, igual que
	// cualquier token de más; ahora un cuarto token es un error.
	language, filename := "", ""
	var headerErr error
	if len(parts) > 1 {
		language = parts[1]
	}
	if len(parts) > 2 {
		filename = parts[2]
	}
	if len(parts) > 3 {
		headerErr = fmt.Errorf("CODE takes at most a language and a filename; unexpected %q", strings.Join(parts[3:], " "))
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
	codeElement.Filename = filename
	return codeElement, consumedLines, headerErr
}

// parseFlexCode handles flex mode code parsing: ```language
func (p *CodeParser) parseFlexCode(ctx *ParseContext, startIndex int, pos diagnostics.Position, line string) (ast.Element, int, error) {
	// ```lenguaje [archivo] o ```lenguaje title="archivo"
	language, filename, headerErr := parseFenceInfo(strings.TrimSpace(line[3:]))
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
	codeElement.Filename = filename
	return codeElement, consumedLines, headerErr
}

// parseFenceInfo separa la línea de info de una fence (```ts renewals.ts o
// ```ts title="renewals.ts") en lenguaje y nombre de archivo. Antes todo el
// resto de la línea terminaba en Language ("ts renewals.ts"), así que el
// lenguaje quedaba contaminado y el nombre sin campo propio.
func parseFenceInfo(info string) (language, filename string, err error) {
	if info == "" {
		return "", "", nil
	}
	language, rest, _ := strings.Cut(info, " ")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return language, "", nil
	}
	// `[etiqueta]` (la sintaxis de code-group) y `{1,3-5}` (líneas
	// resaltadas) no son nombres de archivo: conservan el comportamiento
	// anterior, en el que la línea completa queda como Language.
	if strings.HasPrefix(rest, "[") || strings.HasPrefix(rest, "{") {
		return info, "", nil
	}
	if value, ok := strings.CutPrefix(rest, "title="); ok {
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, " \t") {
			return language, "", fmt.Errorf("code fence title %q must be a filename without spaces", value)
		}
		return language, value, nil
	}
	if strings.ContainsAny(rest, " \t") {
		return language, "", fmt.Errorf("code fence info %q takes at most a language and a filename", info)
	}
	return language, rest, nil
}
