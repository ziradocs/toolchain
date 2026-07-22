// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// DirectiveParser maneja elementos de directivas (@directivas)
type DirectiveParser struct{}

// CanParse determina si una línea es una directiva
func (p *DirectiveParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "@")
}

// Parse parsea una directiva desde las líneas proporcionadas
func (p *DirectiveParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	if !strings.HasPrefix(line, "@") {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	// Extract directive content
	directivePart := strings.TrimSpace(line[1:]) // Remove @
	if directivePart == "" {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 1,
			Error:         nil,
		}
	}

	// Special handling for multi-line notes directives
	if directivePart == "notes:" || directivePart == "notes" {
		return p.parseNotesDirective(ctx, startIndex, pos)
	}

	// Parse name and parameters for other directives
	name, parameters := p.parseDirectiveNameAndParams(directivePart)

	directive := ast.NewDirectiveNode(pos, name)
	directive.Parameters = parameters

	return &ParseResult{
		Element:       directive,
		ConsumedLines: 1,
		Error:         nil,
	}
}

// parseDirectiveNameAndParams parses directive name and parameters
// Supports:
// @notes "Remember to mention budget impact"
// @timer 300
// @transition type="fade" duration="1000ms"
// @highlight color="yellow"
// @center
func (p *DirectiveParser) parseDirectiveNameAndParams(content string) (string, map[string]interface{}) {
	parameters := make(map[string]interface{})

	// Find first space to separate name from parameters. Both
	// "@directiva parametros" and "@directiva: parametros" are documented,
	// supported syntax — strip trailing ":" (TrimRight, not TrimSuffix,
	// so a typo'd double colon like "@highlight::" doesn't leave one
	// colon behind and silently miss the switch-case/CSS-class match
	// below) so the name used downstream never carries it.
	parts := strings.SplitN(content, " ", 2)
	name := strings.TrimRight(parts[0], ":")

	if len(parts) == 1 {
		// No parameters
		return name, parameters
	}

	paramString := strings.TrimSpace(parts[1])
	if paramString == "" {
		return name, parameters
	}

	// Special cases for specific directives
	switch name {
	case "notes":
		// @notes "Remember to mention budget impact"
		if strings.HasPrefix(paramString, "\"") && strings.HasSuffix(paramString, "\"") {
			parameters["content"] = strings.Trim(paramString, "\"")
		} else {
			parameters["content"] = paramString
		}
	case "timer":
		// @timer 300
		if strings.Contains(paramString, "=") {
			// @timer duration=300 warning=60
			p.parseKeyValueParams(paramString, parameters)
		} else {
			// @timer 300
			parameters["duration"] = paramString
		}
	case "transition":
		// @transition type="fade" duration="1000ms"
		p.parseKeyValueParams(paramString, parameters)
	case "highlight":
		// @highlight color="yellow"
		if strings.Contains(paramString, "=") {
			p.parseKeyValueParams(paramString, parameters)
		} else {
			// @highlight yellow
			parameters["color"] = paramString
		}
	case "delay":
		// @delay 2000
		parameters["ms"] = paramString
	case "include":
		// @include ruta.doclang (issue #238) — SIEMPRE valor crudo, nunca
		// key=value, igual que "delay": la ruta es lo único que
		// core/include.Expand reconoce en build-time (corre ANTES
		// del parser, sobre el texto crudo). Si esto ramificara por "=" como
		// timer/highlight, una ruta con "=" literal (rara, pero legal en un
		// nombre de archivo) rompería el round-trip.
		parameters["path"] = paramString
	case "auto-play":
		// @auto-play interval=5000
		if strings.Contains(paramString, "=") {
			p.parseKeyValueParams(paramString, parameters)
		} else {
			// @auto-play 5000
			parameters["interval"] = paramString
		}
	default:
		// Generic key=value parameters
		if strings.Contains(paramString, "=") {
			p.parseKeyValueParams(paramString, parameters)
		} else {
			// Single value parameter
			parameters["value"] = paramString
		}
	}

	return name, parameters
}

// parseKeyValueParams parses parameters in key=value format
func (p *DirectiveParser) parseKeyValueParams(paramString string, parameters map[string]interface{}) {
	// Simple parser for key=value pairs
	// Handles: key=value key2="value with spaces" key3='single quotes'

	i := 0
	for i < len(paramString) {
		// Skip whitespace
		for i < len(paramString) && (paramString[i] == ' ' || paramString[i] == '\t') {
			i++
		}
		if i >= len(paramString) {
			break
		}

		// Parse key
		keyStart := i
		for i < len(paramString) && paramString[i] != '=' {
			i++
		}
		if i >= len(paramString) {
			break
		}
		key := strings.TrimSpace(paramString[keyStart:i])
		i++ // skip '='

		// Parse value
		if i >= len(paramString) {
			break
		}

		var value string
		switch paramString[i] {
		case '"':
			// Quoted value with double quotes
			i++ // skip opening quote
			valueStart := i
			for i < len(paramString) && paramString[i] != '"' {
				i++
			}
			value = paramString[valueStart:i]
			if i < len(paramString) {
				i++ // skip closing quote
			}
		case '\'':
			// Quoted value with single quotes
			i++ // skip opening quote
			valueStart := i
			for i < len(paramString) && paramString[i] != '\'' {
				i++
			}
			value = paramString[valueStart:i]
			if i < len(paramString) {
				i++ // skip closing quote
			}
		default:
			// Unquoted value
			valueStart := i
			for i < len(paramString) && paramString[i] != ' ' && paramString[i] != '\t' {
				i++
			}
			value = paramString[valueStart:i]
		}

		parameters[key] = value
	}
}

// parseNotesDirective parses multi-line notes directives
// Supports both @notes: and @notes formats with multi-line content
func (p *DirectiveParser) parseNotesDirective(ctx *ParseContext, startIndex int, pos diagnostics.Position) *ParseResult {
	consumedLines := 1
	var notesContent []string

	// Check if the notes directive has content on the same line
	line := strings.TrimSpace(ctx.Lines[startIndex])
	directivePart := strings.TrimSpace(line[1:]) // Remove @

	// If it's @notes with content on the same line like @notes "content"
	if directivePart != "notes:" && directivePart != "notes" {
		name, parameters := p.parseDirectiveNameAndParams(directivePart)
		directive := ast.NewDirectiveNode(pos, name)
		directive.Parameters = parameters
		return &ParseResult{
			Element:       directive,
			ConsumedLines: 1,
			Error:         nil,
		}
	}

	// Collect multi-line content after @notes: or @notes
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		nextLine := strings.TrimSpace(ctx.Lines[i])

		// Stop if we hit another directive, slide break, or empty line that indicates end of notes
		if strings.HasPrefix(nextLine, "@") ||
			strings.HasPrefix(nextLine, "---") ||
			strings.HasPrefix(nextLine, "#") ||
			(nextLine == "" && i > startIndex+1 && len(notesContent) > 0) {
			break
		}

		// Add non-empty lines to notes content
		if nextLine != "" {
			notesContent = append(notesContent, nextLine)
			consumedLines++
		} else if len(notesContent) > 0 {
			// Empty line in the middle of notes content - add it to preserve formatting
			notesContent = append(notesContent, "")
			consumedLines++
		}
	}

	// Create the notes directive
	directive := ast.NewDirectiveNode(pos, "notes")
	directive.Parameters = make(map[string]interface{})
	if len(notesContent) > 0 {
		directive.Parameters["content"] = strings.Join(notesContent, "\n")
	} else {
		directive.Parameters["content"] = ""
	}

	return &ParseResult{
		Element:       directive,
		ConsumedLines: consumedLines,
		Error:         nil,
	}
}
