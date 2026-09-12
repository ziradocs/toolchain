// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// SpecialBlockParser maneja bloques especiales como :::info, :::warning, etc.
type SpecialBlockParser struct{}

// CanParse determina si una línea es el inicio de un bloque especial
func (p *SpecialBlockParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, ":::") && len(trimmed) > 3
}

// Parse parsea un bloque especial desde las líneas proporcionadas
func (p *SpecialBlockParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := ctx.Position(startIndex)
	line := strings.TrimSpace(ctx.Lines[startIndex])
	consumed := 1

	// Verify it starts with :::
	if !strings.HasPrefix(line, ":::") {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	// Extract block type: can be ":::info" or "::: info"
	blockContent := strings.TrimSpace(line[3:]) // Remove :::
	if blockContent == "" {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 1,
			Error:         nil,
		}
	}

	parts := strings.Fields(blockContent)
	blockType := parts[0]
	title := ""
	if len(parts) > 1 {
		title = strings.Join(parts[1:], " ")
	}

	// Collect content until ::: or another special block
	var content strings.Builder
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == ":::" {
			// Found closing :::
			consumed++
			break
		}

		// If we find another special block (:::type), close current block
		if strings.HasPrefix(trimmedLine, ":::") && len(trimmedLine) > 3 {
			// Don't consume this line, let the next parser handle it
			break
		}

		// If we find a new SLIDE, close the special block
		if strings.HasPrefix(trimmedLine, "SLIDE ") {
			// Don't consume this line, let the main parser handle it
			break
		}

		if content.Len() > 0 {
			content.WriteString("\n")
		}
		content.WriteString(trimmedLine)
		consumed++
	}

	block := ast.NewSpecialBlockElement(pos, blockType, content.String())
	block.Title = title
	block.Icon = KnownSpecialBlockTypes()[blockType]

	return &ParseResult{
		Element:       block,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// KnownSpecialBlockTypes es la lista única de tipos de special block con
// soporte de primera clase, mapeados a su ícono por defecto ("" si no lleva
// uno). Antes de esto había TRES listas a mano que ya se contradecían
// (issue del audit 2026-09-11, C12): el switch de arriba (7 tipos, sin
// "details"/"left"/"right"/"highlight"), la whitelist del linter
// (rules.go — 6 tipos, sin "note"/"example" que este parser SÍ reconoce), y
// spec/language-specification.md:328-336, que documenta "left, right,
// highlight" como válidos sin que ningún código los reconociera como tales.
// Esta función es la fuente única que las tres deberían consultar — el
// linter (SPECIAL001) ya lo hace; slidelang (template/base.go) queda para
// el PR que consume el bump de core.
func KnownSpecialBlockTypes() map[string]string {
	return map[string]string{
		"info":      "💡",
		"warning":   "⚠️",
		"danger":    "🚨",
		"success":   "✅",
		"tip":       "💡",
		"note":      "📝",
		"example":   "💻",
		"details":   "",
		"left":      "",
		"right":     "",
		"highlight": "",
	}
}
