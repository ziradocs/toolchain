// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"

	"go.ziradocs.com/core/internal/normalize/normalizer/base"
)

// GraphicsRule maneja placeholders de gráficos generados por AI
type GraphicsRule struct {
	chartInference *base.ChartTypeInference
	mode           string // "convert", "comment", "remove"
}

// NewGraphicsRule crea una nueva instancia de la regla
func NewGraphicsRule(mode string) *GraphicsRule {
	if mode == "" {
		mode = "comment" // Por defecto, convertir a comentarios
	}

	return &GraphicsRule{
		chartInference: base.NewChartTypeInference(),
		mode:           mode,
	}
}

func (r *GraphicsRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	modified := false

	// Patrón multiidioma para gráficos
	graphicKeywords := []string{
		"GRÁFICO", "CHART", "GRAPH", "DIAGRAMA", "DIAGRAM",
		"TABLA", "TABLE", "GRAPHIQUE", "DIAGRAMME", "TABLEAU",
		"DIAGRAMM", "GRAFIK", "TABELLE",
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// VALIDACIÓN 1: No aplicar si ya hay elementos SlideLang válidos cerca
		// EXCEPTO si el placeholder contiene palabras clave obvias de AI
		if r.isNearValidSlideLangElement(lines, i) && !r.hasObviousPlaceholderKeywords(trimmed) {
			continue
		}

		// VALIDACIÓN 2: No aplicar si la línea actual es un elemento SlideLang válido
		if r.isSlideLangElement(trimmed) {
			continue
		}

		// Buscar patrones de gráficos (solo placeholders textuales)
		for _, keyword := range graphicKeywords {
			if r.isGraphicPlaceholder(trimmed, keyword) {
				// Extraer descripción después del ":"
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					description := strings.TrimSpace(parts[1])

					switch r.mode {
					case "convert":
						chartType := r.chartInference.InferChartType(description)
						lines[i] = r.createChartBlock(chartType, description)
						modified = true
					case "comment":
						lines[i] = "<!-- " + trimmed + " -->"
						modified = true
					case "remove":
						lines[i] = ""
						modified = true
					}
					break
				}
			}
		}
	}

	if modified {
		return strings.Join(lines, "\n"), nil
	}
	return content, nil
}

func (r *GraphicsRule) createChartBlock(chartType, description string) string {
	return `<chart type="` + chartType + `" title="` + description + `">
<!-- Datos del gráfico aquí -->
</chart>`
}

// isNearValidSlideLangElement verifica si hay elementos SlideLang válidos cerca de la línea actual
func (r *GraphicsRule) isNearValidSlideLangElement(lines []string, currentIndex int) bool {
	// Verificar líneas anteriores y posteriores (rango de ±3 líneas)
	start := currentIndex - 3
	if start < 0 {
		start = 0
	}
	end := currentIndex + 3
	if end >= len(lines) {
		end = len(lines) - 1
	}

	for i := start; i <= end; i++ {
		if i == currentIndex {
			continue // No verificar la línea actual
		}

		line := strings.TrimSpace(lines[i])
		// Detectar elementos SlideLang válidos
		if strings.HasPrefix(line, "<<chart:") ||
			strings.HasPrefix(line, "<<mermaid>>") ||
			strings.HasPrefix(line, "<<image:") ||
			strings.HasPrefix(line, "<<map:") {
			return true
		}

		// Detectar propiedades de configuración de charts válidos (solo si están claramente estructurados)
		if (strings.HasPrefix(line, "title:") ||
			strings.HasPrefix(line, "labels:") ||
			strings.HasPrefix(line, "datasets:") ||
			strings.HasPrefix(line, "data:") ||
			strings.HasPrefix(line, "color:")) &&
			!strings.Contains(line, `"`) { // No debe ser parte de JSON
			return true
		}
	}

	return false
}

// isSlideLangElement verifica si una línea es un elemento SlideLang válido
func (r *GraphicsRule) isSlideLangElement(line string) bool {
	// Elementos SlideLang que usan sintaxis << >> (deben estar completos)
	if strings.HasPrefix(line, "<<") && strings.HasSuffix(line, ">>") {
		return true
	}

	// Casos especiales: elementos que pueden estar en líneas separadas pero válidos
	if strings.HasPrefix(line, "<<chart:") && strings.Contains(line, ">>") ||
		strings.HasPrefix(line, "<<mermaid>>") ||
		strings.HasPrefix(line, "<<image:") && strings.Contains(line, ">>") ||
		strings.HasPrefix(line, "<<map:") && strings.Contains(line, ">>") {
		return true
	}

	return false
}

// isGraphicPlaceholder verifica si una línea es un placeholder textual de gráfico
func (r *GraphicsRule) isGraphicPlaceholder(line, keyword string) bool {
	upperLine := strings.ToUpper(line)
	upperKeyword := strings.ToUpper(keyword)

	// Verificar que contiene la palabra clave con ":"
	if !strings.Contains(upperLine, upperKeyword+":") {
		return false
	}

	// Verificar que NO sea un elemento SlideLang
	if strings.HasPrefix(line, "<<") || strings.Contains(line, ">>") {
		return false
	}

	// Verificar que la palabra clave esté al inicio de la línea (después de espacios/markdown)
	trimmedLine := strings.TrimLeft(line, " \t*#")
	if !strings.HasPrefix(strings.ToUpper(trimmedLine), upperKeyword+":") {
		return false
	}

	// Extraer la descripción después del ":"
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return false
	}
	description := strings.TrimSpace(parts[1])
	upperDescription := strings.ToUpper(description)

	// Si contiene palabras clave de placeholder típicas de AI, es un placeholder
	placeholderWords := []string{"INSERTAR", "INSERT", "ADD", "AGREGAR", "PLACEHOLDER", "AQUÍ", "HERE", "ICI", "HIER", "DESCRIPCIÓN"}
	for _, word := range placeholderWords {
		if strings.Contains(upperDescription, word) {
			return true
		}
	}

	// Si es una descripción muy corta (menos de 30 caracteres), probablemente es un placeholder
	if len(description) < 30 {
		return true
	}

	// Rechazar descripciones largas y detalladas (más de 80 caracteres)
	if len(description) > 80 {
		return false
	}

	// Solo si no parece una descripción muy específica o técnica
	if !containsSpecificTerms(upperDescription) {
		return true
	}
	return false
}

// containsSpecificTerms verifica si la descripción contiene términos específicos que sugieren contenido real
func containsSpecificTerms(upperDescription string) bool {
	specificTerms := []string{
		"TRIMESTRAL", "QUARTERLY", "ANUAL", "ANNUAL", "COMPARACIÓN", "COMPARISON",
		"CRECIMIENTO", "GROWTH", "PORCENTAJE", "PERCENTAGE", "ANÁLISIS", "ANALYSIS",
		"DETALLADO", "DETAILED", "ESPECÍFICO", "SPECIFIC", "DATOS", "DATA",
		"VENTAS", "SALES", "MARKETING", "FINANCIERO", "FINANCIAL",
	}

	for _, term := range specificTerms {
		if strings.Contains(upperDescription, term) {
			return true
		}
	}
	return false
}

// hasObviousPlaceholderKeywords verifica si la línea contiene palabras clave obvias de placeholder
func (r *GraphicsRule) hasObviousPlaceholderKeywords(line string) bool {
	upperLine := strings.ToUpper(line)
	obviousKeywords := []string{"PLACEHOLDER", "INSERTAR", "INSERT", "AGREGAR", "ADD", "AQUÍ", "HERE", "ICI", "HIER"}

	for _, keyword := range obviousKeywords {
		if strings.Contains(upperLine, keyword) {
			return true
		}
	}

	return false
}

func (r *GraphicsRule) Description() string {
	return "Maneja placeholders de gráficos generados por AI (modo: " + r.mode + ")"
}

func (r *GraphicsRule) Priority() int {
	return 3
}

func (r *GraphicsRule) Category() base.RuleCategory {
	return base.CategoryEnhancement
}
