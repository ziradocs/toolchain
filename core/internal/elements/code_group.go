// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// CodeGroupParser maneja grupos de código (::::code-group o :::code-group)
type CodeGroupParser struct{}

// stripBaseIndent quita hasta n espacios de indentación al inicio de line,
// preservando cualquier indentación adicional (la indentación real del
// código). Espeja la lógica de CodeParser.parseStrictCode
// (internal/elements/code.go): si el prefijo de n espacios está presente lo
// remueve; si no (línea vacía, o menos indentada que el marcador — un
// code-group está delimitado por fences, no por indentación, así que esas
// líneas se conservan tal cual), deja la línea intacta. n==0 es un no-op.
func stripBaseIndent(line string, n int) string {
	if n <= 0 {
		return line
	}
	if strings.HasPrefix(line, strings.Repeat(" ", n)) {
		return line[n:]
	}
	return line
}

// CanParse determina si una línea es el inicio de un grupo de código
func (p *CodeGroupParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)
	// Soporta tanto ::::code-group (Markdown extended) como :::code-group (legacy)
	return strings.HasPrefix(trimmed, "::::code-group") || strings.HasPrefix(trimmed, ":::code-group")
}

// Parse parsea un grupo de código desde las líneas proporcionadas
func (p *CodeGroupParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := ctx.Position(startIndex)
	codeGroup := ast.NewCodeGroupElement(pos)
	consumed := 1 // Skip :::code-group line
	var groupErr error

	// En modo strict, el formatter (formatter/strict.go: formatCodeGroup +
	// formatStrictElement con indent(body, 2)) emite el bloque completo
	// —marcador :::code-group, fences ``` y contenido— indentado al nivel
	// del elemento dentro del SLIDE. Si guardáramos el contenido verbatim,
	// cada línea de código heredaría esa indentación de elemento y crecería
	// 2 espacios por pasada de `fmt` (no idempotente). Dedentamos el
	// contenido por la indentación base del propio marcador :::code-group
	// para que el round-trip fmt→build→fmt sea estable, mismo enfoque que
	// CodeParser.parseStrictCode (internal/elements/code.go). Solo aplica a
	// strict: en flex el marcador va a columna 0 (dedent 0 = no-op) y no
	// queremos alterar el AST de doclang/flex, así que se gatea por modo
	// igual que hace CodeParser.
	baseIndent := 0
	if ctx.Mode == "strict" {
		baseIndent = CalculateIndentLevel(ctx.Lines[startIndex])
	}

	// Collect code blocks until :::
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := strings.TrimSpace(ctx.Lines[i])

		if line == ":::" {
			// Found closing :::
			consumed++
			break
		}

		// Look for start of code block ```language [label]
		if strings.HasPrefix(line, "```") {
			// ```lenguaje [etiqueta]: la etiqueta es todo lo que va entre los
			// corchetes, espacios incluidos. Antes se partía por espacios y
			// una etiqueta como [With Visualization] se perdía sin aviso.
			language, rest, _ := strings.Cut(strings.TrimSpace(line[3:]), " ")
			rest = strings.TrimSpace(rest)
			label := ""
			if rest != "" {
				if len(rest) >= 2 && strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]") {
					label = strings.TrimSpace(rest[1 : len(rest)-1])
				} else if groupErr == nil {
					groupErr = fmt.Errorf("code-group fence %q must be ```language [label]", line)
				}
			}

			consumed++ // Skip ``` line
			i++

			var content strings.Builder

			// Collect code content until ```
			for i < len(ctx.Lines) {
				line := ctx.Lines[i]
				if strings.TrimSpace(line) == "```" {
					// Found code block end
					consumed++ // Skip closing ```
					break
				}

				if content.Len() > 0 {
					content.WriteString("\n")
				}
				content.WriteString(stripBaseIndent(line, baseIndent))
				consumed++
				i++
			}

			codeBlock := ast.CodeBlock{
				Language: language,
				Label:    label,
				Content:  content.String(),
			}
			codeGroup.CodeBlocks = append(codeGroup.CodeBlocks, codeBlock)
		} else {
			// Una línea fuera de las fences no tiene lugar en el AST del
			// code-group; antes desaparecía sin diagnóstico.
			if line != "" && groupErr == nil {
				groupErr = fmt.Errorf("unexpected content inside :::code-group outside a code fence: %q", line)
			}
			// Skip non-code lines
			consumed++
		}
	}

	return &ParseResult{
		Element:       codeGroup,
		ConsumedLines: consumed,
		Error:         groupErr,
	}
}
