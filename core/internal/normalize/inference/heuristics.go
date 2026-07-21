// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package inference

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Heuristic representa una heurística específica para inferencia
type Heuristic interface {
	// Analyze analiza el contenido y retorna una sugerencia
	Analyze(content string, context DocumentContext) HeuristicResult

	// Name retorna el nombre de la heurística
	Name() string

	// Priority retorna la prioridad (menor número = mayor prioridad)
	Priority() int
}

// HeuristicResult contiene el resultado de una heurística
type HeuristicResult struct {
	Suggestion   ElementSuggestion   // Sugerencia principal
	Confidence   float64             // Confianza en la sugerencia (0.0-1.0)
	Reasoning    string              // Explicación del razonamiento
	Alternatives []ElementSuggestion // Sugerencias alternativas
}

// ElementSuggestion representa una sugerencia de elemento
type ElementSuggestion struct {
	Type       string            // Tipo de elemento (chart, table, image, etc.)
	Subtype    string            // Subtipo específico (bar, line, pie, etc.)
	Properties map[string]string // Propiedades sugeridas
	Content    string            // Contenido sugerido
}

// ChartDetectionHeuristic detecta referencias implícitas a gráficos
type ChartDetectionHeuristic struct {
	patterns map[string]*regexp.Regexp
}

func NewChartDetectionHeuristic() *ChartDetectionHeuristic {
	return &ChartDetectionHeuristic{
		patterns: map[string]*regexp.Regexp{
			"bar":     regexp.MustCompile(`(?i)(bar|barra|columna|comparar|vs|versus|diferencia)`),
			"line":    regexp.MustCompile(`(?i)(line|línea|tendencia|evolución|tiempo|trend|over time|año|mes)`),
			"pie":     regexp.MustCompile(`(?i)(pie|circular|pastel|porcentaje|distribución|participación|share)`),
			"area":    regexp.MustCompile(`(?i)(área|area|filled|relleno|acumulado|stacked)`),
			"scatter": regexp.MustCompile(`(?i)(scatter|dispersión|correlation|correlación|relación)`),
		},
	}
}

func (h *ChartDetectionHeuristic) Analyze(content string, context DocumentContext) HeuristicResult {
	content = strings.ToLower(content)

	// Buscar keywords de datos y números
	dataKeywords := []string{"datos", "estadística", "porcentaje", "%", "número", "cifra", "métrica"}
	dataScore := 0
	for _, keyword := range dataKeywords {
		if strings.Contains(content, keyword) {
			dataScore++
		}
	}

	// Si no hay keywords de datos, baja confianza
	if dataScore == 0 {
		return HeuristicResult{
			Confidence: 0.1,
			Reasoning:  "No se detectaron keywords de datos",
		}
	}

	// Determinar tipo de gráfico basado en patrones
	bestType := "bar" // default
	maxMatches := 0

	for chartType, pattern := range h.patterns {
		matches := pattern.FindAllString(content, -1)
		if len(matches) > maxMatches {
			maxMatches = len(matches)
			bestType = chartType
		}
	}

	// Calcular confianza
	confidence := float64(dataScore)*0.2 + float64(maxMatches)*0.3
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Ajustar por contexto del documento
	if context.HasTheme("data") || context.HasTheme("business") {
		confidence += 0.1
	}

	suggestion := ElementSuggestion{
		Type:    "chart",
		Subtype: bestType,
		Properties: map[string]string{
			"title": h.extractChartTitle(content),
			"data":  "placeholder",
		},
	}

	return HeuristicResult{
		Suggestion: suggestion,
		Confidence: confidence,
		Reasoning:  "Detectados keywords de datos y patrones de " + bestType,
	}
}

func (h *ChartDetectionHeuristic) extractChartTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(trimmed), "gráfico") ||
			strings.Contains(strings.ToLower(trimmed), "chart") {
			// Extraer descripción después de ":"
			if strings.Contains(trimmed, ":") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	return "Gráfico generado automáticamente"
}

func (h *ChartDetectionHeuristic) Name() string {
	return "ChartDetection"
}

func (h *ChartDetectionHeuristic) Priority() int {
	return 1
}

// TableDetectionHeuristic detecta contenido que debería ser tabla
type TableDetectionHeuristic struct{}

func NewTableDetectionHeuristic() *TableDetectionHeuristic {
	return &TableDetectionHeuristic{}
}

func (h *TableDetectionHeuristic) Analyze(content string, context DocumentContext) HeuristicResult {
	lines := strings.Split(content, "\n")

	// Buscar patrones de estructura tabular
	structuredLines := 0
	totalLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		totalLines++

		// Buscar separadores comunes de tabla
		if strings.Contains(trimmed, "|") ||
			strings.Contains(trimmed, ":") ||
			strings.Contains(trimmed, "\t") {
			structuredLines++
		}
	}

	if totalLines == 0 {
		return HeuristicResult{Confidence: 0.0}
	}

	structureRatio := float64(structuredLines) / float64(totalLines)

	// Si menos del 60% de líneas tienen estructura, baja confianza
	if structureRatio < 0.6 {
		return HeuristicResult{
			Confidence: structureRatio * 0.5,
			Reasoning:  "Pocas líneas con estructura tabular",
		}
	}

	// Buscar keywords de comparación
	comparisonKeywords := []string{"vs", "versus", "comparación", "diferencia", "tipo", "categoria"}
	comparisonScore := 0
	contentLower := strings.ToLower(content)

	for _, keyword := range comparisonKeywords {
		if strings.Contains(contentLower, keyword) {
			comparisonScore++
		}
	}

	confidence := structureRatio*0.7 + float64(comparisonScore)*0.1
	if confidence > 1.0 {
		confidence = 1.0
	}

	suggestion := ElementSuggestion{
		Type:    "table",
		Subtype: "markdown",
		Properties: map[string]string{
			"headers": "auto-detect",
			"style":   "default",
		},
	}

	return HeuristicResult{
		Suggestion: suggestion,
		Confidence: confidence,
		Reasoning:  "Estructura tabular detectada con ratio: " + formatFloat(structureRatio),
	}
}

func (h *TableDetectionHeuristic) Name() string {
	return "TableDetection"
}

func (h *TableDetectionHeuristic) Priority() int {
	return 2
}

// DiagramDetectionHeuristic detecta contenido que debería ser diagrama
type DiagramDetectionHeuristic struct{}

func NewDiagramDetectionHeuristic() *DiagramDetectionHeuristic {
	return &DiagramDetectionHeuristic{}
}

func (h *DiagramDetectionHeuristic) Analyze(content string, context DocumentContext) HeuristicResult {
	contentLower := strings.ToLower(content)

	// Patrones específicos de diagramas
	patterns := map[string]*regexp.Regexp{
		"flow":     regexp.MustCompile(`(?i)(flujo|flow|proceso|process|pasos|steps|workflow|pipeline)`),
		"org":      regexp.MustCompile(`(?i)(organigrama|organization|jerarquía|hierarchy|estructura|team)`),
		"network":  regexp.MustCompile(`(?i)(red|network|conexión|connection|nodos|nodes|relación)`),
		"timeline": regexp.MustCompile(`(?i)(timeline|línea de tiempo|cronograma|schedule|etapa|phase)`),
	}

	bestType := ""
	maxMatches := 0

	for diagramType, pattern := range patterns {
		matches := pattern.FindAllString(contentLower, -1)
		if len(matches) > maxMatches {
			maxMatches = len(matches)
			bestType = diagramType
		}
	}

	if maxMatches == 0 {
		return HeuristicResult{
			Confidence: 0.0,
			Reasoning:  "No se detectaron patrones de diagrama",
		}
	}

	// Buscar indicadores adicionales
	sequenceIndicators := []string{"1.", "2.", "3.", "primero", "segundo", "luego", "después", "finally"}
	sequenceScore := 0

	for _, indicator := range sequenceIndicators {
		if strings.Contains(contentLower, indicator) {
			sequenceScore++
		}
	}

	confidence := float64(maxMatches)*0.3 + float64(sequenceScore)*0.2
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Ajustar por contexto
	if context.HasTheme("technical") || context.HasTheme("business") {
		confidence += 0.1
	}

	suggestion := ElementSuggestion{
		Type:    "diagram",
		Subtype: bestType,
		Properties: map[string]string{
			"direction": "top-down",
			"style":     "default",
		},
	}

	return HeuristicResult{
		Suggestion: suggestion,
		Confidence: confidence,
		Reasoning:  "Detectados patrones de " + bestType + " con " + strconv.Itoa(maxMatches) + " coincidencias",
	}
}

func (h *DiagramDetectionHeuristic) Name() string {
	return "DiagramDetection"
}

func (h *DiagramDetectionHeuristic) Priority() int {
	return 3
}

// ImagePlaceholderHeuristic detecta referencias implícitas a imágenes
type ImagePlaceholderHeuristic struct{}

func NewImagePlaceholderHeuristic() *ImagePlaceholderHeuristic {
	return &ImagePlaceholderHeuristic{}
}

func (h *ImagePlaceholderHeuristic) Analyze(content string, context DocumentContext) HeuristicResult {
	contentLower := strings.ToLower(content)

	// Patrones que sugieren imágenes
	imageKeywords := []string{
		"imagen", "image", "foto", "picture", "visual", "screenshot",
		"captura", "ejemplo visual", "mostrar", "ver", "observar",
	}

	keywordCount := 0
	for _, keyword := range imageKeywords {
		if strings.Contains(contentLower, keyword) {
			keywordCount++
		}
	}

	if keywordCount == 0 {
		return HeuristicResult{
			Confidence: 0.0,
			Reasoning:  "No se detectaron keywords de imagen",
		}
	}

	// Detectar tipo de imagen basado en contexto
	imageType := "illustration"
	if strings.Contains(contentLower, "screenshot") || strings.Contains(contentLower, "captura") {
		imageType = "screenshot"
	} else if strings.Contains(contentLower, "logo") || strings.Contains(contentLower, "icon") {
		imageType = "icon"
	} else if strings.Contains(contentLower, "diagram") || strings.Contains(contentLower, "esquema") {
		imageType = "diagram"
	}

	confidence := float64(keywordCount) * 0.3
	if confidence > 1.0 {
		confidence = 1.0
	}

	suggestion := ElementSuggestion{
		Type:    "image",
		Subtype: imageType,
		Properties: map[string]string{
			"alt":  h.generateAltText(content),
			"path": "assets/images/placeholder.png",
		},
	}

	return HeuristicResult{
		Suggestion: suggestion,
		Confidence: confidence,
		Reasoning:  "Detectadas " + strconv.Itoa(keywordCount) + " referencias a imágenes",
	}
}

func (h *ImagePlaceholderHeuristic) generateAltText(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 10 && len(trimmed) < 100 {
			// Usar la primera línea descriptiva como alt text
			return trimmed
		}
	}
	return "Imagen descriptiva"
}

func (h *ImagePlaceholderHeuristic) Name() string {
	return "ImagePlaceholder"
}

func (h *ImagePlaceholderHeuristic) Priority() int {
	return 4
}

// CodeDetectionHeuristic detecta referencias a código
type CodeDetectionHeuristic struct{}

func NewCodeDetectionHeuristic() *CodeDetectionHeuristic {
	return &CodeDetectionHeuristic{}
}

func (h *CodeDetectionHeuristic) Analyze(content string, context DocumentContext) HeuristicResult {
	contentLower := strings.ToLower(content)

	// Buscar keywords de código
	codeKeywords := []string{
		"código", "code", "function", "función", "variable", "class", "método",
		"algorithm", "algoritmo", "script", "programa", "implementación",
	}

	keywordCount := 0
	for _, keyword := range codeKeywords {
		if strings.Contains(contentLower, keyword) {
			keywordCount++
		}
	}

	// Buscar sintaxis específica de código
	syntaxPatterns := []string{
		"def ", "function ", "class ", "var ", "let ", "const ",
		"if (", "for (", "while (", "return ", "import ", "from ",
	}

	syntaxCount := 0
	for _, pattern := range syntaxPatterns {
		if strings.Contains(contentLower, pattern) {
			syntaxCount++
		}
	}

	if keywordCount == 0 && syntaxCount == 0 {
		return HeuristicResult{
			Confidence: 0.0,
			Reasoning:  "No se detectaron patrones de código",
		}
	}

	// Detectar lenguaje basado en contexto
	language := h.detectLanguage(content, context)

	confidence := float64(keywordCount)*0.2 + float64(syntaxCount)*0.4
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Bonificación si el contexto es técnico
	if context.HasTheme("technical") {
		confidence += 0.2
	}

	suggestion := ElementSuggestion{
		Type:    "code",
		Subtype: language,
		Properties: map[string]string{
			"language": language,
			"theme":    "default",
		},
	}

	return HeuristicResult{
		Suggestion: suggestion,
		Confidence: confidence,
		Reasoning:  "Detectados patrones de código en " + language,
	}
}

func (h *CodeDetectionHeuristic) detectLanguage(content string, context DocumentContext) string {
	contentLower := strings.ToLower(content)

	// Buscar indicadores específicos de lenguaje
	languages := map[string][]string{
		"python":     {"python", "def ", "import ", "from ", "__init__"},
		"javascript": {"javascript", "function ", "var ", "let ", "const ", "=>"},
		"java":       {"java", "public class", "private ", "static "},
		"go":         {"go", "func ", "package ", "import ("},
		"sql":        {"sql", "select ", "from ", "where ", "insert "},
	}

	for language, patterns := range languages {
		for _, pattern := range patterns {
			if strings.Contains(contentLower, pattern) {
				return language
			}
		}
	}

	// Si está en contexto técnico y tiene keywords generales, asumir python
	if context.HasTheme("technical") {
		return "python"
	}

	return "text"
}

func (h *CodeDetectionHeuristic) Name() string {
	return "CodeDetection"
}

func (h *CodeDetectionHeuristic) Priority() int {
	return 5
}

// GetDefaultHeuristics retorna el conjunto predeterminado de heurísticas
func GetDefaultHeuristics() []Heuristic {
	return []Heuristic{
		NewChartDetectionHeuristic(),
		NewTableDetectionHeuristic(),
		NewDiagramDetectionHeuristic(),
		NewImagePlaceholderHeuristic(),
		NewCodeDetectionHeuristic(),
	}
}

// formatFloat formatea una fracción (0.0-1.0) como porcentaje redondeado.
func formatFloat(f float64) string {
	return strconv.Itoa(int(math.Round(f*100))) + "%"
}
