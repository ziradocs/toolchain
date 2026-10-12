// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// MathParser maneja el parsing de ecuaciones/fórmulas LaTeX en bloque (issue
// #239-B, decisión B del plan OSS). Sintaxis: strict `<<math>> ... <<end>>`
// (espeja mermaid.go), flex `$$ ... $$` (una sola línea o multi-línea).
// Alcance deliberado: solo bloque/display — math INLINE ($...$ a mitad de
// una línea de prosa) queda fuera, toca el pipeline inline
// (renderer/sanitizer.go) y es más invasivo; ver issue de seguimiento.
type MathParser struct{}

// CanParse determina si puede parsear una línea como Math.
func (p *MathParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	switch mode {
	case "strict":
		return trimmed == "<<math>>"
	case "flex":
		if trimmed == "<<math>>" {
			return true
		}
		// $$ debe ser el inicio de la línea (no "el valor es $$x$$ acá") —
		// eso es lo que mantiene esto en block/display, no inline.
		return strings.HasPrefix(trimmed, "$$")
	}

	return false
}

// Parse parsea un elemento Math.
func (p *MathParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{Error: nil}
	}

	pos := ctx.Position(startIndex)
	openingLine := strings.TrimSpace(ctx.Lines[startIndex])

	isDollarFormat := strings.HasPrefix(openingLine, "$$") && !strings.HasPrefix(openingLine, "<<math>>")

	var content strings.Builder
	var caption, label string
	consumedLines := 1

	if ctx.MathSource {
		return p.parseLiteral(ctx, startIndex, isDollarFormat)
	}
	if isDollarFormat {
		consumedLines = p.parseDollarForm(ctx.Lines, startIndex, openingLine, &content)
	} else {
		consumedLines = p.parseAngleForm(ctx.Lines, startIndex, ctx.Mode, &content, &caption, &label)
	}

	math := ast.NewMathElement(pos, content.String())
	math.Caption = caption
	math.Label = label

	return &ParseResult{
		Element:       math,
		ConsumedLines: consumedLines,
		Error:         nil,
	}
}

// parseDollarForm recoge el contenido de un bloque `$$ ... $$` (flex). Sin
// metadata (label/caption): eso es un concepto strict-only, mismo alcance
// que caption/label en TABLE/IMAGE (internal/elements/table.go, image.go).
// Soporta cierre en la misma línea ($$formula$$) o en una línea posterior.
func (p *MathParser) parseDollarForm(lines []string, startIndex int, openingLine string, content *strings.Builder) int {
	consumed := 1
	rest := strings.TrimSpace(strings.TrimPrefix(openingLine, "$$"))

	if strings.HasSuffix(rest, "$$") && rest != "$$" {
		// Cierre en la misma línea: $$formula$$
		content.WriteString(strings.TrimSpace(strings.TrimSuffix(rest, "$$")))
		return consumed
	}

	if rest != "" {
		content.WriteString(rest)
	}

	for i := startIndex + 1; i < len(lines); i++ {
		trimmedLine := strings.TrimSpace(lines[i])

		if strings.HasSuffix(trimmedLine, "$$") {
			inner := strings.TrimSpace(strings.TrimSuffix(trimmedLine, "$$"))
			if inner != "" {
				if content.Len() > 0 {
					content.WriteString("\n")
				}
				content.WriteString(inner)
			}
			consumed++
			break
		}
		if trimmedLine == "---" {
			break
		}

		if content.Len() > 0 {
			content.WriteString("\n")
		}
		content.WriteString(trimmedLine)
		consumed++
	}

	return consumed
}

// parseAngleForm recoge el contenido de un bloque `<<math>> ... <<end>>`,
// tolerando líneas `caption:` y `label:` opcionales dentro. NO usa detección de
// indentación (a diferencia de mermaid.go): mermaid.go asume contexto SLIDE
// indentado 2 espacios, válido en modo strict — pero `<<math>>` también se
// soporta en modo flex (doclang), donde el contenido va a columna 0, SIN
// indentación relativa a `<<math>>` (bug real encontrado vía smoke-test
// E2E: con detección de indentación, el bloque se cerraba en la primera
// línea de contenido no-indentada, dejando Content vacío). Termina por
// `<<end>>` (obligatorio para contenido correcto) o por `---` (límite de
// slide); sin ninguno de los dos, en modo STRICT se detiene en el siguiente
// límite de bloque strict ("SLIDE "/"SECTION " en columna 0, ver
// IsStrictBlockBoundary) o, a falta de eso también, en EOF — mismo patrón ya
// aceptado en el codebase para otros elementos delimitados explícitamente
// (plantuml.go sin @enduml, chart.go con JSON sin cerrar). El chequeo de
// límite strict existe porque un `<<math>>` sin `<<end>>` en modo strict
// tenía exactamente el mismo bug que #107: se tragaba todos los slides
// siguientes hasta EOF.
//
// El chequeo de límite strict está gateado a mode=="strict" a propósito: en
// flex el contenido va a columna 0 sin indentación (comentario de arriba),
// así que una fórmula LaTeX que arrancara con la palabra "SLIDE" o "SECTION"
// en su propia línea (p. ej. una matriz o vector nombrado así) dispararía
// esto por error si corriera también en flex — y flex ya termina
// correctamente en "---" o EOF.
func (p *MathParser) parseAngleForm(lines []string, startIndex int, mode string, content *strings.Builder, caption, label *string) int {
	consumed := 1

	for i := startIndex + 1; i < len(lines); i++ {
		trimmedLine := strings.TrimSpace(lines[i])

		if trimmedLine == "<<end>>" {
			consumed++
			break
		}
		if trimmedLine == "---" {
			break
		}
		if mode == "strict" && IsStrictBlockBoundary(lines[i]) {
			break
		}
		if trimmedLine == "" {
			consumed++
			continue
		}
		if strings.HasPrefix(trimmedLine, "caption:") {
			// Igual que IMAGE/TABLE, el caption es prosa asociada a la
			// ecuación y no parte del LaTeX crudo.
			captionStr := strings.TrimPrefix(trimmedLine, "caption:")
			*caption = strings.Trim(strings.TrimSpace(captionStr), "\"")
			consumed++
			continue
		}
		if strings.HasPrefix(trimmedLine, "label:") {
			// issue #239: identificador de referencia cruzada (p. ej. "eq:euler").
			labelStr := strings.TrimPrefix(trimmedLine, "label:")
			*label = strings.Trim(strings.TrimSpace(labelStr), "\"")
			consumed++
			continue
		}

		if content.Len() > 0 {
			content.WriteString("\n")
		}
		content.WriteString(trimmedLine)
		consumed++
	}

	return consumed
}

// parseLiteral preserves all payload lines. Strict removes only the exact
// header whitespace prefix plus two spaces; Flex removes no whitespace.
// Blank lines shorter than that prefix are retained, rather than dropped.
func (p *MathParser) parseLiteral(ctx *ParseContext, start int, dollar bool) *ParseResult {
	rawOpen := ctx.Lines[start]
	base := ""
	if ctx.Mode == "strict" {
		base = rawOpen[:len(rawOpen)-len(strings.TrimLeft(rawOpen, " \t"))] + "  "
	}
	var body []string
	var caption, label string
	consumed := 1
	closed := false
	if dollar {
		opening := strings.TrimLeft(rawOpen, " \t")
		rest := strings.TrimPrefix(opening, "$$")
		if strings.HasSuffix(rest, "$$") && len(rest) >= 2 {
			return &ParseResult{Element: ast.NewMathElement(ctx.Position(start), strings.TrimSuffix(rest, "$$")), ConsumedLines: 1}
		}
		if rest != "" {
			body = append(body, rest)
		}
	}
	var parseErr error
	for i := start + 1; i < len(ctx.Lines); i++ {
		raw := ctx.Lines[i]
		trimmed := strings.TrimSpace(raw)
		if !dollar && trimmed == "<<end>>" || dollar && trimmed == "$$" {
			consumed++
			closed = true
			break
		}
		if trimmed == "---" || ctx.Mode == "strict" && IsStrictBlockBoundary(raw) {
			break
		}
		line := raw
		if base != "" {
			if rest, ok := strings.CutPrefix(raw, base); ok {
				line = rest
			} else if trimmed != "" {
				parseErr = fmt.Errorf("literal math line requires structural indentation %q", base)
			}
		}
		if !dollar && (strings.HasPrefix(line, "caption:") || strings.HasPrefix(line, "label:")) {
			key, value, _ := strings.Cut(line, ":")
			value = strings.Trim(strings.TrimSpace(value), "\"")
			if key == "caption" {
				caption = value
			} else {
				label = value
			}
		} else if dollar && strings.HasSuffix(line, "$$") {
			body = append(body, strings.TrimSuffix(line, "$$"))
			consumed++
			closed = true
			break
		} else {
			body = append(body, line)
		}
		consumed++
	}
	if !closed {
		parseErr = fmt.Errorf("literal math requires a closing delimiter")
	}
	m := ast.NewMathElement(ctx.Position(start), strings.Join(body, "\n"))
	m.Caption = caption
	m.Label = label
	return &ParseResult{Element: m, ConsumedLines: consumed, Error: parseErr}
}
