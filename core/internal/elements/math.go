// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
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
		return strings.HasPrefix(trimmed, "<<math>>")
	case "flex":
		if strings.HasPrefix(trimmed, "<<math>>") {
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

	pos := diagnostics.NewPosition(startIndex+1, 1)
	openingLine := strings.TrimSpace(ctx.Lines[startIndex])

	isDollarFormat := strings.HasPrefix(openingLine, "$$") && !strings.HasPrefix(openingLine, "<<math>>")

	var content strings.Builder
	var label string
	consumedLines := 1

	if isDollarFormat {
		consumedLines = p.parseDollarForm(ctx.Lines, startIndex, openingLine, &content)
	} else {
		consumedLines = p.parseAngleForm(ctx.Lines, startIndex, &content, &label)
	}

	math := ast.NewMathElement(pos, content.String())
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
// tolerando una línea `label:` opcional dentro. NO usa detección de
// indentación (a diferencia de mermaid.go): mermaid.go asume contexto SLIDE
// indentado 2 espacios, válido en modo strict — pero `<<math>>` también se
// soporta en modo flex (doclang), donde el contenido va a columna 0, SIN
// indentación relativa a `<<math>>` (bug real encontrado vía smoke-test
// E2E: con detección de indentación, el bloque se cerraba en la primera
// línea de contenido no-indentada, dejando Content vacío). Termina por
// `<<end>>` (obligatorio para contenido correcto) o por `---` (límite de
// slide); sin ninguno de los dos, consume hasta EOF — mismo patrón ya
// aceptado en el codebase para otros elementos delimitados explícitamente
// (plantuml.go sin @enduml, chart.go con JSON sin cerrar).
func (p *MathParser) parseAngleForm(lines []string, startIndex int, content *strings.Builder, label *string) int {
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
		if trimmedLine == "" {
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
