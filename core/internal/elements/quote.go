// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// QuoteParser maneja citas en bloque usando sintaxis Markdown (>)
type QuoteParser struct{}

// CanParse verifica si la línea inicia una cita en bloque
func (p *QuoteParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// En modo strict, buscar palabra clave QUOTE
	if mode == "strict" && strings.HasPrefix(trimmed, "QUOTE") {
		return true
	}

	// En modo flex, buscar sintaxis Markdown (>)
	if mode == "flex" && strings.HasPrefix(trimmed, ">") {
		return true
	}

	return false
}

// Parse parsea una cita en bloque desde las líneas proporcionadas
func (p *QuoteParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)

	// Procesar modo strict
	if ctx.Mode == "strict" {
		return p.parseStrict(ctx, startIndex, pos)
	}

	// Procesar modo flex (Markdown)
	return p.parseFlex(ctx, startIndex, pos)
}

// parseStrict parsea citas en modo strict
func (p *QuoteParser) parseStrict(ctx *ParseContext, startIndex int, pos diagnostics.Position) *ParseResult {
	consumed := 0
	line := strings.TrimSpace(ctx.Lines[startIndex])

	// Saltar línea QUOTE
	if strings.HasPrefix(line, "QUOTE") {
		consumed++
		startIndex++
	}

	var contentBuilder strings.Builder
	var author, source string

	// Procesar líneas hasta encontrar separador o final
	for i := startIndex; i < len(ctx.Lines); i++ {
		line := strings.TrimSpace(ctx.Lines[i])

		// Parar en línea vacía o separador de slide
		if line == "" || line == "---" || IsNewElement(line, ctx.Mode) {
			break
		}

		// Buscar metadatos especiales
		if strings.HasPrefix(line, "AUTHOR:") {
			author = strings.TrimSpace(strings.TrimPrefix(line, "AUTHOR:"))
			consumed++
			continue
		}

		if strings.HasPrefix(line, "SOURCE:") {
			source = strings.TrimSpace(strings.TrimPrefix(line, "SOURCE:"))
			consumed++
			continue
		}

		// Agregar contenido
		if contentBuilder.Len() > 0 {
			contentBuilder.WriteString("\n")
		}
		contentBuilder.WriteString(line)
		consumed++
	}

	quote := ast.NewQuoteElement(pos, contentBuilder.String())
	quote.Author = author
	quote.Source = source

	return &ParseResult{
		Element:       quote,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// parseFlex parsea citas en modo flex (Markdown)
func (p *QuoteParser) parseFlex(ctx *ParseContext, startIndex int, pos diagnostics.Position) *ParseResult {
	consumed := 0
	var contentBuilder strings.Builder

	// Procesar líneas que empiecen con >
	for i := startIndex; i < len(ctx.Lines); i++ {
		line := strings.TrimSpace(ctx.Lines[i])

		// Si no empieza con >, terminar la cita
		if !strings.HasPrefix(line, ">") {
			break
		}

		// Remover el > y espacios iniciales
		content := strings.TrimSpace(strings.TrimPrefix(line, ">"))

		if contentBuilder.Len() > 0 {
			contentBuilder.WriteString("\n")
		}
		contentBuilder.WriteString(content)
		consumed++
	}

	// Extraer autor si está en formato "-- Autor" al final
	content := contentBuilder.String()
	author := ""

	// Buscar patrón "-- Autor" al final
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		lastLine := strings.TrimSpace(lines[len(lines)-1])
		if strings.HasPrefix(lastLine, "--") || strings.HasPrefix(lastLine, "—") {
			author = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(lastLine, "--"), "—"))
			// Remover la línea del autor del contenido
			if len(lines) > 1 {
				content = strings.Join(lines[:len(lines)-1], "\n")
			} else {
				content = ""
			}
		}
	}

	quote := ast.NewQuoteElement(pos, content)
	if author != "" {
		quote.Author = author
	}

	return &ParseResult{
		Element:       quote,
		ConsumedLines: consumed,
		Error:         nil,
	}
}
