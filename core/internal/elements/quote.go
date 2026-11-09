// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
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
	if mode == "strict" && HasStrictKeyword(trimmed, "QUOTE") {
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

	pos := ctx.Position(startIndex)

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
	// La sangría de la propia línea QUOTE: una línea en blanco solo sigue
	// dentro de la cita si lo que viene después está MÁS sangrado que ella.
	quoteIndent := CalculateIndentLevel(ctx.Lines[startIndex])

	// Saltar línea QUOTE
	if HasStrictKeyword(line, "QUOTE") {
		consumed++
		startIndex++
	}

	var contentBuilder strings.Builder
	var author, source string
	// Sangría base del cuerpo: la de su primera línea. Un item de lista más
	// sangrado que ella guarda la diferencia (keepListIndent).
	bodyIndent := firstBodyIndent(ctx.Lines, startIndex, nil)

	// Procesar líneas hasta encontrar separador o final
	for i := startIndex; i < len(ctx.Lines); i++ {
		line := strings.TrimSpace(ctx.Lines[i])

		// Una línea en blanco corta la cita, salvo que ya haya texto y la
		// siguiente línea con contenido siga más sangrada que QUOTE: entonces
		// es una línea vacía DENTRO de la cita (un párrafo aparte, como el
		// `>` vacío de flex) y cada línea en blanco cuenta. Lo que ya se leía
		// antes —una línea en blanco seguida de algo sin esa sangría— no
		// cambia.
		if line == "" {
			next := i + 1
			for next < len(ctx.Lines) && strings.TrimSpace(ctx.Lines[next]) == "" {
				next++
			}
			if contentBuilder.Len() > 0 && next < len(ctx.Lines) && p.continuesQuote(ctx, next, quoteIndent) {
				for k := i; k < next; k++ {
					contentBuilder.WriteString("\n")
					consumed++
				}
				i = next - 1
				continue
			}
			break
		}

		// Parar en separador de slide o en el inicio de otro elemento
		if line == "---" || IsNewElement(line, ctx.Mode) {
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
		contentBuilder.WriteString(keepListIndent(ctx.Lines[i], bodyIndent))
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

// continuesQuote dice si la línea con contenido que sigue a una línea en blanco
// pertenece todavía a la cita: está más sangrada que QUOTE y no abre otro
// elemento ni es un separador de slide.
func (p *QuoteParser) continuesQuote(ctx *ParseContext, index, quoteIndent int) bool {
	if CalculateIndentLevel(ctx.Lines[index]) <= quoteIndent {
		return false
	}
	trimmed := strings.TrimSpace(ctx.Lines[index])
	return trimmed != "---" && !IsNewElement(trimmed, ctx.Mode)
}

// parseFlex parsea citas en modo flex (Markdown)
func (p *QuoteParser) parseFlex(ctx *ParseContext, startIndex int, pos diagnostics.Position) *ParseResult {
	consumed := 0
	var contentBuilder strings.Builder

	// Procesar líneas que empiecen con >. Se quita el ">" y UN espacio
	// opcional, como en Markdown; lo que queda de sangría solo se conserva en
	// los items de lista (keepListIndent), que es lo que dice qué item
	// cuelga de cuál.
	var body []string
	for i := startIndex; i < len(ctx.Lines); i++ {
		line := strings.TrimSpace(ctx.Lines[i])

		// Si no empieza con >, terminar la cita
		if !strings.HasPrefix(line, ">") {
			break
		}

		rest := strings.TrimPrefix(line, ">")
		rest = strings.TrimPrefix(rest, " ")
		body = append(body, rest)
		consumed++
	}
	// La base es la menor sangría del cuerpo: una primera línea de prosa más
	// sangrada que la lista no debe aplanarla.
	base := -1
	for _, rest := range body {
		if strings.TrimSpace(rest) == "" {
			continue
		}
		if n := CalculateIndentLevel(rest); base < 0 || n < base {
			base = n
		}
	}
	for _, rest := range body {
		if contentBuilder.Len() > 0 {
			contentBuilder.WriteString("\n")
		}
		contentBuilder.WriteString(keepListIndent(rest, base))
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
			// Content context distinguishes one attribution separator from authored
			// body lines. Multiple terminal separators are ambiguous; never trim.
			bodyEnd := len(lines) - 1
			if ctx.ContentMarkdown && author != "" {
				blankCount := 0
				for k := bodyEnd - 1; k >= 0 && lines[k] == ""; k-- {
					blankCount++
				}
				if blankCount > 1 {
					return &ParseResult{ConsumedLines: consumed, Error: fmt.Errorf("ambiguous multiple blank quote lines before attribution in content Markdown")}
				}
				if blankCount == 1 {
					bodyEnd--
				}
			}
			// Remove the author line and only its explicit content separator.
			if bodyEnd > 0 {
				content = strings.Join(lines[:bodyEnd], "\n")
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
