// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/internal/normalize/normalizer/base"
)

// chartTagPattern se compila una sola vez a nivel de paquete en vez de
// dentro de extractChartType (ver docs/SECURITY_AUDIT_2026-07.md, BA-9).
var chartTagPattern = regexp.MustCompile(`<<chart:(\w+)`)

// ChartFormatterRule formatea bloques de datos de gráficos sin indentación adecuada
type ChartFormatterRule struct{}

// NewChartFormatterRule crea una nueva instancia de ChartFormatterRule
func NewChartFormatterRule() *ChartFormatterRule {
	return &ChartFormatterRule{}
}

func (r *ChartFormatterRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	var result []string
	inChartBlock := false
	blockStart := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detectar inicio de bloque de gráfico
		if strings.HasPrefix(trimmed, "<<chart:") {
			inChartBlock = true
			blockStart = i
			result = append(result, line)
			continue
		}

		// Si estamos en un bloque de gráfico
		if inChartBlock {
			// Detectar fin del bloque de gráfico - mejorado para detectar >> correctamente
			if trimmed == ">>" || (r.isEndOfChartBlock(trimmed, lines, i) && !r.isChartDataLine(trimmed)) {
				inChartBlock = false
				// Procesar el bloque completo
				chartLines := lines[blockStart+1 : i]
				// Solo formatear si no está ya formateado
				if r.needsFormatting(chartLines) {
					formattedLines := r.formatChartData(chartLines)
					result = append(result, formattedLines...)
				} else {
					// Mantener el formato original
					result = append(result, chartLines...)
				}
				result = append(result, line)
				continue
			}
			// No agregar líneas aquí, se procesarán al final del bloque
			continue
		}

		// Líneas normales fuera de bloques de gráfico
		result = append(result, line)
	}

	// Si terminamos y todavía estamos en un bloque de gráfico
	if inChartBlock && blockStart != -1 {
		chartLines := lines[blockStart+1:]
		if r.needsFormatting(chartLines) {
			formattedLines := r.formatChartData(chartLines)
			result = append(result, formattedLines...)
		} else {
			result = append(result, chartLines...)
		}
	}

	return strings.Join(result, "\n"), nil
}

// needsFormatting determina si un bloque de gráfico necesita formateo
func (r *ChartFormatterRule) needsFormatting(lines []string) bool {
	// Verificar si el content ya tiene el formato correcto esperado por SlideLang
	// Un chart bien formateado debería tener:
	// - Propiedades principales con 2 espacios de indentación
	// - Propiedades dentro de datasets: con 4 espacios de indentación

	// Contar cuántas líneas siguen el patrón esperado
	correctlyFormattedLines := 0
	totalNonEmptyLines := 0
	inDatasetsContext := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		totalNonEmptyLines++

		// Detectar cuando entramos o salimos del contexto datasets
		if trimmed == "datasets:" {
			inDatasetsContext = true
			// Check if datasets: has correct indentation (2 spaces)
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
				correctlyFormattedLines++
			}
			continue
		}

		// Si encontramos otra propiedad principal, salimos del contexto datasets
		if inDatasetsContext && r.isMainProperty(trimmed) && trimmed != "data:" {
			inDatasetsContext = false
		}

		// Propiedades principales deberían empezar con 2 espacios (excepto si están dentro de datasets)
		if !inDatasetsContext && r.isMainProperty(trimmed) {
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
				correctlyFormattedLines++
			}
			continue
		}

		// Propiedades dentro de datasets deberían tener 4 espacios
		if inDatasetsContext {
			if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") {
				correctlyFormattedLines++
			}
			continue
		}

		// Sub-propiedades deberían tener más indentación
		if r.isSubProperty(trimmed) && strings.HasPrefix(line, "    ") {
			correctlyFormattedLines++
			continue
		}

		// Arrays y elementos de datos pueden tener indentación variable pero consistente
		if (strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "]") ||
			strings.HasPrefix(trimmed, "\"") || strings.Contains(trimmed, ",")) &&
			strings.HasPrefix(line, "  ") {
			correctlyFormattedLines++
			continue
		}
	}

	// Si menos del 70% de las líneas están correctamente formateadas, necesita formateo
	if totalNonEmptyLines == 0 {
		return false
	}

	ratio := float64(correctlyFormattedLines) / float64(totalNonEmptyLines)
	return ratio < 0.7
}

// isMainProperty verifica si una línea es una propiedad principal del chart
// Note: This is used by needsFormatting, separate from isMainChartProperty used for formatting
func (r *ChartFormatterRule) isMainProperty(line string) bool {
	mainProperties := []string{"series:", "options:", "type:", "title:", "labels:", "datasets:"}
	for _, prop := range mainProperties {
		if strings.HasPrefix(line, prop) {
			return true
		}
	}
	return false
}

// isSubProperty verifica si una línea es una sub-propiedad
func (r *ChartFormatterRule) isSubProperty(line string) bool {
	subProperties := []string{"plugins:", "title:", "display:", "text:", "legend:", "responsive:"}
	for _, prop := range subProperties {
		if strings.HasPrefix(line, prop) {
			return true
		}
	}
	return false
}

// extractChartType extrae el tipo de gráfico del tag de apertura
func (r *ChartFormatterRule) extractChartType(line string) string {
	matches := chartTagPattern.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// isEndOfChartBlock determina si una línea marca el final de un bloque de gráfico
func (r *ChartFormatterRule) isEndOfChartBlock(trimmed string, lines []string, index int) bool {
	// Final explícito con >>
	if trimmed == ">>" {
		return true
	}

	// Línea vacía seguida de contenido que no es de gráfico
	if trimmed == "" && index+1 < len(lines) {
		nextLine := strings.TrimSpace(lines[index+1])
		return !r.isChartDataLine(nextLine) && nextLine != ""
	}

	// Nueva sección/slide
	if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "##") {
		return true
	}

	// Otro bloque SlideLang
	if strings.HasPrefix(trimmed, "<<") {
		return true
	}

	return false
}

// isChartDataLine determina si una línea contiene datos de gráfico
func (r *ChartFormatterRule) isChartDataLine(line string) bool {
	chartDataPrefixes := []string{
		"title:",
		"labels:",
		"datasets:",
		"data:",
		"backgroundColor:",
		"borderColor:",
		"borderWidth:",
		"type:",
		"label:",
		"fill:",
		"tension:",
		"pointRadius:",
		"pointHoverRadius:",
		"xAxisID:",
		"yAxisID:",
		"series:",
		"options:",
		"plugins:",
		"legend:",
		"responsive:",
		"text:",
		"display:",
		"position:",
	}

	trimmed := strings.TrimSpace(line)

	// Verificar prefijos específicos de chart
	for _, prefix := range chartDataPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	// Arrays with brackets
	if strings.HasPrefix(trimmed, "[") {
		return true
	}

	// List items (YAML array style)
	if strings.HasPrefix(trimmed, "-") && strings.Contains(trimmed, ":") {
		return true
	}

	// List items without properties (simple list)
	if strings.HasPrefix(trimmed, "- ") {
		return true
	}

	// String values in quotes
	if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
		return true
	}

	return false
}

// formatChartData formatea las líneas de datos del gráfico con indentación apropiada
func (r *ChartFormatterRule) formatChartData(lines []string) []string {
	var result []string
	baseIndent := "  " // 2 espacios de indentación base
	inArrayContext := false
	inOptionsContext := false
	inDatasetsContext := false
	arrayDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Saltar líneas vacías
		if trimmed == "" {
			result = append(result, "")
			continue
		}

		// Determinar el contexto y nivel de indentación basado en el contenido
		indentLevel := r.calculateIndentLevelSemantic(trimmed, &inArrayContext, &inOptionsContext, &inDatasetsContext, &arrayDepth)

		// Formatear la línea con la indentación apropiada
		formattedLine := r.buildIndentedLine(trimmed, indentLevel, baseIndent)
		result = append(result, formattedLine)
	}

	return result
}

// calculateIndentLevelSemantic calcula el nivel de indentación basado en el contexto semántico
func (r *ChartFormatterRule) calculateIndentLevelSemantic(line string, inArray *bool, inOptions *bool, inDatasets *bool, arrayDepth *int) int {
	// Handle data property specially so multi-line arrays keep their context
	if strings.HasPrefix(line, "data:") {
		trimmedData := strings.TrimSpace(line)

		if strings.HasSuffix(trimmedData, "[") && !strings.Contains(trimmedData, "]") {
			// data: [  -> start of multi-line array
			*inArray = true
			*arrayDepth = 1
		} else {
			// Inline array or scalar value
			*inArray = false
			*arrayDepth = 0
		}

		if *inDatasets {
			return 2
		}
		return 1
	}

	// First check if we're in datasets context and this is a child property
	if *inDatasets && !r.isMainChartProperty(line) {
		// Properties within datasets go to level 2
		return 2
	}

	// Propiedades principales del chart (nivel 1) - solo si no estamos en contexto anidado
	if r.isMainChartProperty(line) && !*inArray && !*inOptions {
		// Reset contexts when we hit a main property
		*inArray = false

		// Check if this is datasets: to set the context
		switch line {
		case "datasets:":
			*inDatasets = true
			*inOptions = false
		case "options:":
			*inOptions = true
			*inDatasets = false
		default:
			// Any other main property ends the datasets context
			*inDatasets = false
			*inOptions = false
		}

		*arrayDepth = 0

		// Caso especial: "data: [" debe activar inArray
		if strings.HasPrefix(line, "data:") && strings.HasSuffix(line, "[") {
			*inArray = true
			*arrayDepth = 1
		}

		return 1
	}

	// Manejo especial para arrays
	if strings.HasPrefix(line, "[") && !strings.Contains(line, ",") {
		// Es el inicio del array (solo "[")
		*inArray = true
		*arrayDepth = 1
		return 1
	}

	if *inArray {
		// Elementos dentro del array
		if strings.HasPrefix(line, "[") && strings.Contains(line, ",") {
			return 2 // Elementos del array van en nivel 2
		}
		if strings.HasPrefix(line, "]") && !strings.Contains(line, ",") {
			// Es el cierre del array (solo "]")
			*inArray = false
			*arrayDepth = 0
			return 1
		}
		// Líneas dentro del array (elementos con formato ["item", value])
		return 2
	}

	// Manejo especial para options context
	if *inOptions {
		if line == "plugins:" {
			return 2 // plugins va en nivel 2 dentro de options
		}
		if line == "title:" {
			return 3 // title dentro de plugins va en nivel 3
		}
		if strings.HasPrefix(line, "display:") || strings.HasPrefix(line, "text:") {
			return 4 // propiedades de title van en nivel 4
		}
		// Otras propiedades dentro de options
		return 2
	}

	// Properties directas del chart que no son principales (como series cuando no estamos en array)
	if strings.Contains(line, ":") && !*inArray {
		return 1
	}

	// Default para líneas que no tienen contexto específico
	return 1
}

// isMainChartProperty verifica si es una propiedad principal del chart (top-level)
// These properties should be at indent level 1 when not in a nested context
func (r *ChartFormatterRule) isMainChartProperty(line string) bool {
	mainProps := []string{"series:", "options:", "type:", "title:", "labels:", "datasets:"}
	for _, prop := range mainProps {
		if strings.HasPrefix(line, prop) {
			return true
		}
	}
	return false
}

// buildIndentedLine construye una línea con la indentación apropiada
func (r *ChartFormatterRule) buildIndentedLine(line string, level int, baseIndent string) string {
	indent := strings.Repeat(baseIndent, level)
	return indent + line
}

// CalculateIndentLevelSemanticPublic expone calculateIndentLevelSemantic para testing
func (r *ChartFormatterRule) CalculateIndentLevelSemanticPublic(line string, inArray *bool, inOptions *bool, inDatasets *bool, arrayDepth *int) int {
	return r.calculateIndentLevelSemantic(line, inArray, inOptions, inDatasets, arrayDepth)
}

// NeedsFormattingPublic expone needsFormatting para testing
func (r *ChartFormatterRule) NeedsFormattingPublic(lines []string) bool {
	return r.needsFormatting(lines)
}

func (r *ChartFormatterRule) Description() string {
	return "Formatea bloques de datos de gráficos con indentación apropiada para AI-generated content"
}

func (r *ChartFormatterRule) Priority() int {
	return 5 // Prioridad alta para ejecutar antes de otros formatters
}

func (r *ChartFormatterRule) Category() base.RuleCategory {
	return base.CategoryEnhancement
}
