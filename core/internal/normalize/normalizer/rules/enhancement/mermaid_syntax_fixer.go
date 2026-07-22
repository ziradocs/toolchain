// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// ganttDatePattern se compila una sola vez a nivel de paquete en vez de
// dentro de fixGanttSyntax, que corre por línea (ver
// docs/SECURITY_AUDIT_2026-07.md, BA-9).
var ganttDatePattern = regexp.MustCompile(`(\s+:\w*,?\s*)(\d{4}-\d{2}),\s*(\d{4}-\d{2})(\s*)$`)

// MermaidSyntaxFixerRule corrige errores de sintaxis específicos en diagramas Mermaid
// Esta regla se ejecuta después del MermaidFormatterRule para corregir problemas de sintaxis
type MermaidSyntaxFixerRule struct{}

// NewMermaidSyntaxFixerRule crea una nueva instancia de la regla
func NewMermaidSyntaxFixerRule() *MermaidSyntaxFixerRule {
	return &MermaidSyntaxFixerRule{}
}

func (r *MermaidSyntaxFixerRule) Apply(content string) (string, error) {
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
			i++

			// Capturar todas las líneas del bloque mermaid
			var mermaidLines []string
			for i < len(lines) {
				currentLine := lines[i]
				trimmed := strings.TrimSpace(currentLine)

				// Si es una línea vacía, la incluimos
				if trimmed == "" {
					mermaidLines = append(mermaidLines, currentLine)
					i++
					continue
				}

				// Detectar si es el fin del bloque mermaid
				if r.isEndOfMermaidBlock(currentLine, lines, i) {
					break
				}

				// Cualquier otra línea es parte del bloque mermaid
				mermaidLines = append(mermaidLines, currentLine)
				i++
			}

			// Aplicar correcciones de sintaxis
			if len(mermaidLines) > 0 {
				correctedLines := r.fixMermaidSyntax(mermaidLines)
				result = append(result, correctedLines...)
			}
		} else {
			result = append(result, line)
			i++
		}
	}

	return strings.Join(result, "\n"), nil
}

// fixMermaidSyntax aplica correcciones de sintaxis específicas a un bloque mermaid
func (r *MermaidSyntaxFixerRule) fixMermaidSyntax(lines []string) []string {
	result := make([]string, 0, len(lines))
	isGanttDiagram := false

	// Detectar si es un diagrama gantt
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "gantt" || strings.HasPrefix(trimmed, "gantt ") {
			isGanttDiagram = true
			break
		}
	}

	for _, line := range lines {
		correctedLine := line

		if isGanttDiagram {
			correctedLine = r.fixGanttSyntax(line)
		}

		// Aplicar otras correcciones generales de Mermaid aquí si es necesario
		// correctedLine = r.fixFlowchartSyntax(correctedLine)
		// correctedLine = r.fixSequenceDiagramSyntax(correctedLine)

		result = append(result, correctedLine)
	}

	return result
}

// fixGanttSyntax corrige problemas específicos de sintaxis en diagramas gantt
func (r *MermaidSyntaxFixerRule) fixGanttSyntax(line string) string {
	// Corregir dateFormat de YYYY-MM a YYYY-MM-DD
	if strings.Contains(line, "dateFormat") && strings.Contains(line, "YYYY-MM") && !strings.Contains(line, "YYYY-MM-DD") {
		return strings.ReplaceAll(line, "YYYY-MM", "YYYY-MM-DD")
	}

	// Corregir fechas en formato YYYY-MM a YYYY-MM-DD (para las tareas)
	// Patrón: cualquier cosa que termine con :algo, YYYY-MM, YYYY-MM
	if ganttDatePattern.MatchString(line) {
		return ganttDatePattern.ReplaceAllStringFunc(line, func(match string) string {
			parts := ganttDatePattern.FindStringSubmatch(match)
			if len(parts) >= 4 {
				prefix := parts[1]
				startDate := parts[2]
				endDate := parts[3]
				suffix := parts[4]

				// Convertir YYYY-MM a YYYY-MM-DD (primer día del mes para inicio, último día para fin)
				startDateFixed := r.convertToFullDate(startDate, true)
				endDateFixed := r.convertToFullDate(endDate, false)

				return prefix + startDateFixed + ", " + endDateFixed + suffix
			}
			return match
		})
	}

	return line
}

// convertToFullDate convierte una fecha YYYY-MM a YYYY-MM-DD
// Si isStart es true, usa el día 01, si es false, usa el último día del mes
func (r *MermaidSyntaxFixerRule) convertToFullDate(dateStr string, isStart bool) string {
	if !strings.Contains(dateStr, "-") {
		return dateStr
	}

	parts := strings.Split(dateStr, "-")
	if len(parts) != 2 {
		return dateStr
	}

	year := parts[0]
	month := parts[1]

	if isStart {
		return year + "-" + month + "-01"
	}

	// Para la fecha de fin, usar el último día del mes
	switch month {
	case "01", "03", "05", "07", "08", "10", "12":
		return year + "-" + month + "-31"
	case "04", "06", "09", "11":
		return year + "-" + month + "-30"
	case "02":
		// Febrero - simplificado (no consideramos años bisiestos)
		return year + "-" + month + "-28"
	default:
		return year + "-" + month + "-01"
	}
}

// isEndOfMermaidBlock determina si una línea marca el final de un bloque mermaid
func (r *MermaidSyntaxFixerRule) isEndOfMermaidBlock(line string, lines []string, currentIndex int) bool {
	trimmed := strings.TrimSpace(line)

	// Si la línea está vacía, no es fin del bloque
	if trimmed == "" {
		return false
	}

	// Si la línea comienza en la primera columna (no indentada)
	if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
		// Verificar si es contenido válido de Mermaid
		if r.isMermaidKeyword(trimmed) || r.containsMermaidSyntax(trimmed) {
			return false
		}

		// Headers
		if strings.HasPrefix(trimmed, "#") {
			return true
		}

		// Listas
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
			// Pero no si es parte de la sintaxis de mermaid (como -->)
			if !r.containsMermaidSyntax(trimmed) {
				return true
			}
		}

		// Special blocks
		if strings.HasPrefix(trimmed, ":::") {
			return true
		}

		// Separadores
		if trimmed == "---" {
			return true
		}

		// Otros elementos
		if strings.HasPrefix(trimmed, "<<") && trimmed != "<<mermaid>>" {
			return true
		}

		// Bloques de código
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "````") {
			return true
		}

		// Si no es una palabra clave de Mermaid y no está indentada, probablemente es el fin del bloque
		return true
	}

	return false
}

// isMermaidKeyword verifica si el texto comienza con una palabra clave de Mermaid
func (r *MermaidSyntaxFixerRule) isMermaidKeyword(text string) bool {
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
func (r *MermaidSyntaxFixerRule) containsMermaidSyntax(line string) bool {
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
	if len(line) > 0 {
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

	// Palabras clave de secciones
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

func (r *MermaidSyntaxFixerRule) Description() string {
	return "Corrige errores de sintaxis específicos en diagramas Mermaid (fechas en gantt, etc.)"
}

func (r *MermaidSyntaxFixerRule) Priority() int {
	return 6 // Prioridad 6 - Después del MermaidFormatterRule (que tiene prioridad 5)
}

func (r *MermaidSyntaxFixerRule) Category() base.RuleCategory {
	return base.CategoryEnhancement
}
