// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package inference

import (
	"fmt"
	"strings"
)

// SuggestionType define los tipos de sugerencias que se pueden generar
type SuggestionType string

const (
	SuggestionTypeChart   SuggestionType = "chart"
	SuggestionTypeTable   SuggestionType = "table"
	SuggestionTypeDiagram SuggestionType = "diagram"
	SuggestionTypeImage   SuggestionType = "image"
	SuggestionTypeCode    SuggestionType = "code"
	SuggestionTypeList    SuggestionType = "list"
	SuggestionTypeText    SuggestionType = "text"
)

// ApplicableAction define las acciones que se pueden tomar con una sugerencia
type ApplicableAction string

const (
	ActionTransform ApplicableAction = "transform" // Transformar contenido existente
	ActionInsert    ApplicableAction = "insert"    // Insertar nuevo elemento
	ActionReplace   ApplicableAction = "replace"   // Reemplazar elemento existente
	ActionEnhance   ApplicableAction = "enhance"   // Mejorar elemento existente
)

// Suggester se encarga de generar sugerencias aplicables basadas en inferencias
type Suggester struct {
	verbose bool
}

// NewSuggester crea una nueva instancia del sugeridor
func NewSuggester(verbose bool) *Suggester {
	return &Suggester{
		verbose: verbose,
	}
}

// GenerateApplicableSuggestions convierte resultados de inferencia en sugerencias aplicables
func (s *Suggester) GenerateApplicableSuggestions(result InferenceResult) []ApplicableSuggestion {
	suggestions := make([]ApplicableSuggestion, 0)

	for _, rankedSuggestion := range result.Suggestions {
		applicable := s.convertToApplicable(rankedSuggestion, result.Context)
		if applicable != nil {
			suggestions = append(suggestions, *applicable)
		}
	}

	return suggestions
}

// ApplicableSuggestion representa una sugerencia que puede ser aplicada al contenido
type ApplicableSuggestion struct {
	Type           SuggestionType   // Tipo de sugerencia
	Action         ApplicableAction // Acción a realizar
	TargetSection  int              // Sección objetivo (índice)
	Confidence     float64          // Confianza en la sugerencia
	Title          string           // Título descriptivo
	Description    string           // Descripción detallada
	Preview        string           // Vista previa del resultado
	Implementation string           // Código/markup a aplicar
	Source         string           // Fuente de la sugerencia
	Benefits       []string         // Beneficios de aplicar la sugerencia
}

// convertToApplicable convierte una sugerencia rankeada en una sugerencia aplicable
func (s *Suggester) convertToApplicable(ranked RankedSuggestion, context DocumentContext) *ApplicableSuggestion {
	switch ranked.Suggestion.Type {
	case "chart":
		return s.generateChartSuggestion(ranked, context)
	case "table":
		return s.generateTableSuggestion(ranked, context)
	case "diagram":
		return s.generateDiagramSuggestion(ranked, context)
	case "image":
		return s.generateImageSuggestion(ranked, context)
	case "code":
		return s.generateCodeSuggestion(ranked, context)
	default:
		return nil
	}
}

// generateChartSuggestion genera una sugerencia aplicable para gráficos
func (s *Suggester) generateChartSuggestion(ranked RankedSuggestion, context DocumentContext) *ApplicableSuggestion {
	chartType := ranked.Suggestion.Subtype
	if chartType == "" {
		chartType = "bar"
	}

	title := ranked.Suggestion.Properties["title"]
	if title == "" {
		title = "Gráfico " + chartType
	}

	chartContent := fmt.Sprintf("type: %s\ntitle: %s\ndata: placeholder", chartType, title)
	implementation := fmt.Sprintf("```chart\n\n%s\n\n```", chartContent)

	return &ApplicableSuggestion{
		Type:           SuggestionTypeChart,
		Action:         ActionTransform,
		TargetSection:  ranked.SectionIndex,
		Confidence:     ranked.Confidence,
		Title:          "Convertir a gráfico " + chartType,
		Description:    "Transformar referencias implícitas a gráficos en elementos chart visuales",
		Preview:        fmt.Sprintf("Gráfico de tipo %s: %s", chartType, title),
		Implementation: implementation,
		Source:         ranked.Source,
		Benefits: []string{
			"Visualización clara de datos",
			"Mejor comprensión de la información",
			"Presentación más profesional",
		},
	}
}

// generateTableSuggestion genera una sugerencia aplicable para tablas
func (s *Suggester) generateTableSuggestion(ranked RankedSuggestion, context DocumentContext) *ApplicableSuggestion {
	return &ApplicableSuggestion{
		Type:           SuggestionTypeTable,
		Action:         ActionTransform,
		TargetSection:  ranked.SectionIndex,
		Confidence:     ranked.Confidence,
		Title:          "Convertir a tabla",
		Description:    "Transformar datos estructurados en tabla Markdown",
		Preview:        "Tabla con headers y datos organizados",
		Implementation: s.generateTableMarkdown(),
		Source:         ranked.Source,
		Benefits: []string{
			"Datos organizados y fáciles de leer",
			"Comparación clara entre elementos",
			"Formato estándar y compatible",
		},
	}
}

// generateDiagramSuggestion genera una sugerencia aplicable para diagramas
func (s *Suggester) generateDiagramSuggestion(ranked RankedSuggestion, context DocumentContext) *ApplicableSuggestion {
	diagramType := ranked.Suggestion.Subtype
	if diagramType == "" {
		diagramType = "flow"
	}

	diagramContent := s.generateMermaidDiagram(diagramType)
	implementation := fmt.Sprintf("```mermaid\n\n%s\n\n```", diagramContent)

	return &ApplicableSuggestion{
		Type:           SuggestionTypeDiagram,
		Action:         ActionTransform,
		TargetSection:  ranked.SectionIndex,
		Confidence:     ranked.Confidence,
		Title:          "Convertir a diagrama " + diagramType,
		Description:    "Crear diagrama visual para representar el proceso o estructura",
		Preview:        fmt.Sprintf("Diagrama de tipo %s con elementos conectados", diagramType),
		Implementation: implementation,
		Source:         ranked.Source,
		Benefits: []string{
			"Visualización clara de procesos",
			"Mejor comprensión de relaciones",
			"Comunicación más efectiva",
		},
	}
}

// generateImageSuggestion genera una sugerencia aplicable para imágenes
func (s *Suggester) generateImageSuggestion(ranked RankedSuggestion, context DocumentContext) *ApplicableSuggestion {
	altText := ranked.Suggestion.Properties["alt"]
	if altText == "" {
		altText = "Imagen descriptiva"
	}

	path := ranked.Suggestion.Properties["path"]
	if path == "" {
		path = "assets/images/placeholder.png"
	}

	implementation := fmt.Sprintf("![%s](%s)", altText, path)

	return &ApplicableSuggestion{
		Type:           SuggestionTypeImage,
		Action:         ActionInsert,
		TargetSection:  ranked.SectionIndex,
		Confidence:     ranked.Confidence,
		Title:          "Insertar imagen",
		Description:    "Agregar imagen para complementar el contenido textual",
		Preview:        fmt.Sprintf("Imagen: %s", altText),
		Implementation: implementation,
		Source:         ranked.Source,
		Benefits: []string{
			"Apoyo visual al contenido",
			"Mayor engagement de la audiencia",
			"Comunicación más efectiva",
		},
	}
}

// generateCodeSuggestion genera una sugerencia aplicable para código
func (s *Suggester) generateCodeSuggestion(ranked RankedSuggestion, context DocumentContext) *ApplicableSuggestion {
	language := ranked.Suggestion.Subtype
	if language == "" {
		language = "text"
	}

	implementation := fmt.Sprintf("```%s\n// Código de ejemplo\n// TODO: Implementar funcionalidad\n```", language)

	return &ApplicableSuggestion{
		Type:           SuggestionTypeCode,
		Action:         ActionTransform,
		TargetSection:  ranked.SectionIndex,
		Confidence:     ranked.Confidence,
		Title:          "Formatear como código " + language,
		Description:    "Aplicar formato de código con highlighting de sintaxis",
		Preview:        fmt.Sprintf("Bloque de código en %s con highlighting", language),
		Implementation: implementation,
		Source:         ranked.Source,
		Benefits: []string{
			"Código legible con syntax highlighting",
			"Separación clara del texto normal",
			"Mejor experiencia para desarrolladores",
		},
	}
}

// generateTableMarkdown genera markdown básico para tabla
func (s *Suggester) generateTableMarkdown() string {
	return `| Columna 1 | Columna 2 | Columna 3 |
|-----------|-----------|-----------|
| Dato 1    | Dato 2    | Dato 3    |
| Dato 4    | Dato 5    | Dato 6    |`
}

// generateMermaidDiagram genera un diagrama Mermaid básico
func (s *Suggester) generateMermaidDiagram(diagramType string) string {
	switch diagramType {
	case "flow":
		return `graph TD
    A[Inicio] --> B{Decisión}
    B -->|Sí| C[Acción 1]
    B -->|No| D[Acción 2]
    C --> E[Fin]
    D --> E`
	case "org":
		return `graph TD
    A[Director] --> B[Manager 1]
    A --> C[Manager 2]
    B --> D[Empleado 1]
    B --> E[Empleado 2]
    C --> F[Empleado 3]`
	case "timeline":
		return `gantt
    title Línea de Tiempo
    dateFormat  YYYY-MM-DD
    section Fase 1
    Tarea 1           :2024-01-01, 30d
    Tarea 2           :2024-02-01, 20d`
	default:
		return `graph LR
    A[Elemento 1] --> B[Elemento 2]
    B --> C[Elemento 3]`
	}
}

// GetSuggestionsByAction filtra sugerencias por acción específica
func GetSuggestionsByAction(suggestions []ApplicableSuggestion, action ApplicableAction) []ApplicableSuggestion {
	filtered := make([]ApplicableSuggestion, 0)
	for _, suggestion := range suggestions {
		if suggestion.Action == action {
			filtered = append(filtered, suggestion)
		}
	}
	return filtered
}

// GetSuggestionsByType filtra sugerencias por tipo específico
func GetSuggestionsByType(suggestions []ApplicableSuggestion, suggestionType SuggestionType) []ApplicableSuggestion {
	filtered := make([]ApplicableSuggestion, 0)
	for _, suggestion := range suggestions {
		if suggestion.Type == suggestionType {
			filtered = append(filtered, suggestion)
		}
	}
	return filtered
}

// GetHighConfidenceSuggestions filtra sugerencias con alta confianza
func GetHighConfidenceSuggestions(suggestions []ApplicableSuggestion, minConfidence float64) []ApplicableSuggestion {
	filtered := make([]ApplicableSuggestion, 0)
	for _, suggestion := range suggestions {
		if suggestion.Confidence >= minConfidence {
			filtered = append(filtered, suggestion)
		}
	}
	return filtered
}

// FormatAsMarkdown formatea una sugerencia como markdown para documentación
func (suggestion *ApplicableSuggestion) FormatAsMarkdown() string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "## %s\n\n", suggestion.Title)
	fmt.Fprintf(&builder, "**Confianza:** %.2f\n\n", suggestion.Confidence)
	fmt.Fprintf(&builder, "**Descripción:** %s\n\n", suggestion.Description)
	fmt.Fprintf(&builder, "**Acción:** %s\n\n", suggestion.Action)
	fmt.Fprintf(&builder, "**Sección objetivo:** %d\n\n", suggestion.TargetSection)

	if len(suggestion.Benefits) > 0 {
		builder.WriteString("**Beneficios:**\n")
		for _, benefit := range suggestion.Benefits {
			fmt.Fprintf(&builder, "- %s\n", benefit)
		}
		builder.WriteString("\n")
	}

	builder.WriteString("**Implementación:**\n")
	builder.WriteString("```markdown\n")
	builder.WriteString(suggestion.Implementation)
	builder.WriteString("\n```\n\n")

	return builder.String()
}

// QuickSuggest es una función de conveniencia para generar sugerencias rápidas
func QuickSuggest(content string, minConfidence float64) []ApplicableSuggestion {
	suggestions, hasResults := QuickInfer(content, minConfidence)
	if !hasResults {
		return make([]ApplicableSuggestion, 0)
	}

	// Crear un contexto básico
	analyzer := NewContextAnalyzer()
	context := analyzer.AnalyzeDocument(content)

	// Simular resultado de inferencia
	inferenceResult := InferenceResult{
		Suggestions: suggestions,
		Context:     context,
	}

	suggester := NewSuggester(false)
	return suggester.GenerateApplicableSuggestions(inferenceResult)
}
