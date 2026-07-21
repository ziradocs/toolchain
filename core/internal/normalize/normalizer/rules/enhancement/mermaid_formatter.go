// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"

	"go.ziradocs.com/core/internal/normalize/normalizer/base"
)

// MermaidFormatterRule formatea correctamente los bloques <<mermaid>> para que sean compatibles con el parser flex
// Esta regla se encarga de agregar la indentación requerida al contenido de los diagramas Mermaid
type MermaidFormatterRule struct{}

// NewMermaidFormatterRule crea una nueva instancia de la regla
func NewMermaidFormatterRule() *MermaidFormatterRule {
	return &MermaidFormatterRule{}
}

func (r *MermaidFormatterRule) Apply(content string) (string, error) {
	// Enfoque más simple: dividir el contenido por líneas y procesar bloque por bloque
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	i := 0
	inCodeBlock := false
	codeBlockDelimiter := ""

	for i < len(lines) {
		line := lines[i]

		// Detectar inicio/fin de bloques de código markdown
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "````") {
			if !inCodeBlock {
				inCodeBlock = true
				codeBlockDelimiter = strings.TrimSpace(line)
			} else if strings.TrimSpace(line) == strings.Split(codeBlockDelimiter, ":")[0] ||
				strings.TrimSpace(line) == "```" ||
				strings.TrimSpace(line) == "````" {
				inCodeBlock = false
				codeBlockDelimiter = ""
			}
			result = append(result, line)
			i++
			continue
		}

		// Si estamos dentro de un bloque de código markdown, no procesar
		if inCodeBlock {
			result = append(result, line)
			i++
			continue
		}

		// Si encontramos un bloque <<mermaid>>
		if strings.TrimSpace(line) == "<<mermaid>>" {
			result = append(result, line)
			i++ // Capturar contenido del bloque mermaid
			var mermaidContent []string
			for i < len(lines) {
				currentLine := lines[i]

				// Si es el fin del bloque mermaid, no incluir esta línea y salir
				if r.isEndOfMermaidBlock(currentLine, lines, i) {
					break
				}

				// Incluir la línea (puede ser vacía o contenido)
				mermaidContent = append(mermaidContent, currentLine)
				i++
			}

			// Formatear el contenido capturado
			formattedLines := r.formatMermaidLines(mermaidContent)
			result = append(result, formattedLines...)
		} else {
			result = append(result, line)
			i++
		}
	}

	return strings.Join(result, "\n"), nil
}

// isAlreadyFormatted verifica si el contenido mermaid ya está correctamente formateado
func (r *MermaidFormatterRule) isAlreadyFormatted(lines []string, startIndex int) bool {
	// Contar líneas con contenido válido de Mermaid y su indentación
	mermaidLinesFound := 0
	correctlyIndentedLines := 0

	// Mirar las próximas líneas para ver si ya tienen la indentación correcta
	for i := startIndex; i < len(lines) && i < startIndex+10; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Líneas vacías no cuentan
		if trimmed == "" {
			continue
		}

		// Verificar si es el fin del bloque mermaid
		if r.isEndOfMermaidBlock(line, lines, i) {
			break
		}

		// Si encontramos contenido válido de mermaid
		if r.containsMermaidSyntax(trimmed) || r.isMermaidKeyword(trimmed) {
			mermaidLinesFound++

			// Verificar si tiene la indentación correcta (exactamente 2 espacios)
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
				correctlyIndentedLines++
			}
		}
	}

	// Solo considerar que está formateado si TODAS las líneas de mermaid tienen indentación correcta
	// y encontramos al menos una línea de contenido mermaid
	return mermaidLinesFound > 0 && correctlyIndentedLines == mermaidLinesFound
}

// isEndOfMermaidBlock determina si una línea marca el final de un bloque mermaid
func (r *MermaidFormatterRule) isEndOfMermaidBlock(line string, lines []string, currentIndex int) bool {
	trimmed := strings.TrimSpace(line)

	// Si la línea está vacía, no es fin del bloque
	if trimmed == "" {
		return false
	}
	// Si la línea comienza en la primera columna (no indentada)
	if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
		// Verificar PRIMERO los separadores específicos de SlideLang antes de verificar sintaxis Mermaid

		// Separadores - definitivamente fin del bloque
		if trimmed == "---" {
			return true
		}

		// Headers - definitivamente fin del bloque
		if strings.HasPrefix(trimmed, "#") {
			return true
		}

		// Special blocks - definitivamente fin del bloque
		if strings.HasPrefix(trimmed, ":::") {
			return true
		}

		// Otros elementos SlideLang - definitivamente fin del bloque
		if strings.HasPrefix(trimmed, "<<") && trimmed != "<<mermaid>>" {
			return true
		}

		// Bloques de código - definitivamente fin del bloque
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "````") {
			return true
		}

		// DESPUÉS verificar si es contenido válido de Mermaid
		if r.isMermaidKeyword(trimmed) || r.containsMermaidSyntax(trimmed) {
			return false
		}

		// Listas que no son sintaxis de mermaid
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			// Pero no si es parte de la sintaxis de mermaid (como ->>)
			if !r.containsMermaidSyntax(trimmed) {
				return true
			}
		}

		// Si no es una palabra clave de Mermaid y no está indentada, probablemente es el fin del bloque
		return true
	}

	return false
}

// isMermaidKeyword verifica si el texto comienza con una palabra clave de Mermaid
func (r *MermaidFormatterRule) isMermaidKeyword(text string) bool {
	mermaidKeywords := []string{
		"graph", "flowchart", "sequenceDiagram", "classDiagram",
		"stateDiagram", "gantt", "pie", "gitgraph", "erDiagram",
		"journey", "quadrantChart", "requirement", "C4Context",
		"mindmap", "timeline", "sankey", "block",
	}

	fields := strings.Fields(text)
	if len(fields) > 0 {
		firstWord := fields[0]
		for _, keyword := range mermaidKeywords {
			if strings.HasPrefix(firstWord, keyword) {
				return true
			}
		}
	}

	return false
}

// containsMermaidSyntax verifica si una línea contiene sintaxis típica de Mermaid
func (r *MermaidFormatterRule) containsMermaidSyntax(line string) bool {
	// Flechas y conectores de Mermaid
	mermaidPatterns := []string{
		"-->", "-.->", "==>", "~~>", "->>", "<<-", "<<-->>",
		"<->", "<-->", "---", "---|", "|---",
	}

	for _, pattern := range mermaidPatterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}

	// Patrones de nodos con corchetes, paréntesis, etc.
	// A[texto], B(texto), C{texto}, D((texto)), etc.
	// Pero solo si parece ser un nodo válido (letra/número seguido de bracket)
	if len(line) > 0 {
		// Buscar patrones como A[...], B(...), C{...}
		for i, char := range line {
			if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
				if i+1 < len(line) {
					nextChar := line[i+1]
					if nextChar == '[' || nextChar == '(' || nextChar == '{' {
						return true
					}
				}
			}
		}
	}

	// Palabras clave de secciones en gantt, journey, etc.
	sectionKeywords := []string{
		"section", "participant", "title", "dateFormat",
		"class", "style", "click", "fill", "stroke",
	}

	lineLower := strings.ToLower(line)
	for _, keyword := range sectionKeywords {
		if strings.HasPrefix(lineLower, keyword+" ") || strings.HasPrefix(lineLower, keyword+":") {
			return true
		}
	}

	return false
}

// formatMermaidContent formatea correctamente el contenido mermaid para el parser flex
func (r *MermaidFormatterRule) formatMermaidContent(content string) string {
	lines := strings.Split(content, "\n")
	var formattedLines []string

	// Verificar si hay contenido real (no solo líneas vacías)
	hasRealContent := false
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			hasRealContent = true
			break
		}
	}

	// Si no hay contenido real, solo preservar una línea vacía si el input tenía líneas
	if !hasRealContent {
		if len(lines) > 0 && len(lines[0]) == 0 {
			return ""
		}
		return ""
	}

	// Contar líneas vacías al final del contenido original
	trailingEmptyLines := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			trailingEmptyLines++
		} else {
			break
		}
	}

	// Procesar cada línea
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Si la línea no está vacía, agregar indentación de 2 espacios
		if trimmed != "" {
			// Verificar si ya tiene indentación correcta (2 espacios exactos)
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
				// Ya tiene la indentación correcta
				formattedLines = append(formattedLines, line)
			} else {
				// Agregar indentación de 2 espacios
				formattedLines = append(formattedLines, "  "+trimmed)
			}
		} else if len(formattedLines) > 0 {
			// Preservar líneas vacías entre secciones del diagrama
			formattedLines = append(formattedLines, "")
		}
	}

	// Eliminar TODAS las líneas vacías al final
	for len(formattedLines) > 0 && strings.TrimSpace(formattedLines[len(formattedLines)-1]) == "" {
		formattedLines = formattedLines[:len(formattedLines)-1]
	}

	// Solo restaurar líneas vacías al final si había exactamente 1 línea vacía
	// (no múltiples líneas vacías que son solo whitespace extra)
	if trailingEmptyLines == 1 {
		formattedLines = append(formattedLines, "")
	}

	return strings.Join(formattedLines, "\n")
}

// formatMermaidLines formatea una lista de líneas de contenido mermaid
func (r *MermaidFormatterRule) formatMermaidLines(lines []string) []string {
	var result []string

	// Si no hay líneas, devolver vacío
	if len(lines) == 0 {
		return result
	}

	// Procesar cada línea
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Si la línea está vacía, mantenerla
		if trimmed == "" {
			result = append(result, line)
		} else {
			// Si ya tiene la indentación correcta (exactamente 2 espacios), mantenerla
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
				result = append(result, line)
			} else {
				// Agregar indentación de 2 espacios
				result = append(result, "  "+trimmed)
			}
		}
	}

	return result
}

// needsFormatting verifica si el contenido mermaid necesita ser formateado
func (r *MermaidFormatterRule) needsFormatting(content string) bool {
	// Si está vacío o solo espacios, no necesita formateo
	if strings.TrimSpace(content) == "" {
		return false
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Líneas vacías no cuentan
		if trimmed == "" {
			continue
		}

		// Si la línea no tiene exactamente 2 espacios de indentación, necesita formateo
		if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
			return true
		}
	}

	return false
}

func (r *MermaidFormatterRule) Description() string {
	return "Formatea bloques <<mermaid>> agregando la indentación requerida (2 espacios) para compatibilidad con el parser flex"
}

func (r *MermaidFormatterRule) Priority() int {
	return 5 // Prioridad 5 - Después de la conversión de markdown a slidelang, antes del parsing
}

func (r *MermaidFormatterRule) Category() base.RuleCategory {
	return base.CategoryEnhancement
}
