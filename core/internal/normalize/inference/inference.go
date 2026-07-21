// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package inference

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// InferenceConfig contiene la configuración para el motor de inferencia
type InferenceConfig struct {
	MinConfidence    float64  // Confianza mínima para aplicar sugerencias (0.0-1.0)
	MaxSuggestions   int      // Número máximo de sugerencias por análisis
	EnableHeuristics []string // Lista de heurísticas habilitadas (vacío = todas)
	Verbose          bool     // Logging detallado
}

// InferenceResult contiene el resultado completo del análisis de inferencia
type InferenceResult struct {
	Suggestions       []RankedSuggestion // Sugerencias ordenadas por confianza
	ProcessingTime    time.Duration      // Tiempo de procesamiento
	Context           DocumentContext    // Contexto analizado del documento
	HeuristicsUsed    []string           // Lista de heurísticas utilizadas
	OverallConfidence float64            // Confianza promedio de las sugerencias
}

// RankedSuggestion es una sugerencia con información de ranking
type RankedSuggestion struct {
	Suggestion   ElementSuggestion // La sugerencia en sí
	Confidence   float64           // Confianza de la sugerencia
	Source       string            // Heurística que generó la sugerencia
	Reasoning    string            // Explicación del razonamiento
	SectionIndex int               // Índice de la sección donde aplica
}

// InferenceEngine es el motor principal de inferencia
type InferenceEngine struct {
	heuristics      []Heuristic
	contextAnalyzer *ContextAnalyzer
	config          InferenceConfig
}

// NewInferenceEngine crea una nueva instancia del motor de inferencia
func NewInferenceEngine(config InferenceConfig) *InferenceEngine {
	engine := &InferenceEngine{
		heuristics:      GetDefaultHeuristics(),
		contextAnalyzer: NewContextAnalyzer(),
		config:          config,
	}

	// Filtrar heurísticas si hay lista específica
	if len(config.EnableHeuristics) > 0 {
		engine.filterHeuristics(config.EnableHeuristics)
	}

	// Ordenar por prioridad
	sort.Slice(engine.heuristics, func(i, j int) bool {
		return engine.heuristics[i].Priority() < engine.heuristics[j].Priority()
	})

	return engine
}

// AnalyzeDocument analiza un documento completo y genera sugerencias
func (ie *InferenceEngine) AnalyzeDocument(content string) InferenceResult {
	startTime := time.Now()

	if ie.config.Verbose {
		fmt.Println("=== Iniciando Análisis de Inferencia ===")
	}

	// 1. Analizar contexto del documento
	context := ie.contextAnalyzer.AnalyzeDocument(content)

	if ie.config.Verbose {
		ie.logContext(context)
	}

	// 2. Analizar cada sección individualmente
	suggestions := make([]RankedSuggestion, 0)
	heuristicsUsed := make(map[string]bool)

	for sectionIndex, section := range context.Sections {
		sectionSuggestions := ie.analyzeSection(section.Content, context, sectionIndex)
		suggestions = append(suggestions, sectionSuggestions...)

		// Registrar heurísticas usadas
		for _, suggestion := range sectionSuggestions {
			heuristicsUsed[suggestion.Source] = true
		}
	}

	// 3. Filtrar por confianza mínima
	filteredSuggestions := ie.filterByConfidence(suggestions)

	// 4. Limitar número de sugerencias
	if len(filteredSuggestions) > ie.config.MaxSuggestions {
		filteredSuggestions = filteredSuggestions[:ie.config.MaxSuggestions]
	}

	// 5. Calcular confianza general
	overallConfidence := ie.calculateOverallConfidence(filteredSuggestions)

	// 6. Preparar lista de heurísticas utilizadas
	usedHeuristics := make([]string, 0, len(heuristicsUsed))
	for heuristic := range heuristicsUsed {
		usedHeuristics = append(usedHeuristics, heuristic)
	}

	result := InferenceResult{
		Suggestions:       filteredSuggestions,
		ProcessingTime:    time.Since(startTime),
		Context:           context,
		HeuristicsUsed:    usedHeuristics,
		OverallConfidence: overallConfidence,
	}

	if ie.config.Verbose {
		ie.logResult(result)
	}

	return result
}

// analyzeSection analiza una sección específica del documento
func (ie *InferenceEngine) analyzeSection(sectionContent string, context DocumentContext, sectionIndex int) []RankedSuggestion {
	suggestions := make([]RankedSuggestion, 0)

	for _, heuristic := range ie.heuristics {
		result := heuristic.Analyze(sectionContent, context)

		if result.Confidence >= ie.config.MinConfidence {
			ranked := RankedSuggestion{
				Suggestion:   result.Suggestion,
				Confidence:   result.Confidence,
				Source:       heuristic.Name(),
				Reasoning:    result.Reasoning,
				SectionIndex: sectionIndex,
			}
			suggestions = append(suggestions, ranked)

			// Agregar alternativas si las hay
			for _, alt := range result.Alternatives {
				altRanked := RankedSuggestion{
					Suggestion:   alt,
					Confidence:   result.Confidence * 0.8, // Reducir confianza para alternativas
					Source:       heuristic.Name() + " (alt)",
					Reasoning:    result.Reasoning + " (alternativa)",
					SectionIndex: sectionIndex,
				}
				suggestions = append(suggestions, altRanked)
			}
		}
	}

	// Ordenar por confianza descendente
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Confidence > suggestions[j].Confidence
	})

	return suggestions
}

// filterByConfidence filtra sugerencias por confianza mínima
func (ie *InferenceEngine) filterByConfidence(suggestions []RankedSuggestion) []RankedSuggestion {
	filtered := make([]RankedSuggestion, 0)

	for _, suggestion := range suggestions {
		if suggestion.Confidence >= ie.config.MinConfidence {
			filtered = append(filtered, suggestion)
		}
	}

	return filtered
}

// calculateOverallConfidence calcula la confianza promedio ponderada
func (ie *InferenceEngine) calculateOverallConfidence(suggestions []RankedSuggestion) float64 {
	if len(suggestions) == 0 {
		return 0.0
	}

	totalConfidence := 0.0
	totalWeight := 0.0

	for i, suggestion := range suggestions {
		// Dar más peso a las primeras sugerencias (mejor ranking)
		weight := 1.0 / float64(i+1)
		totalConfidence += suggestion.Confidence * weight
		totalWeight += weight
	}

	return totalConfidence / totalWeight
}

// filterHeuristics filtra las heurísticas basándose en la configuración
func (ie *InferenceEngine) filterHeuristics(enabledHeuristics []string) {
	filtered := make([]Heuristic, 0)

	for _, heuristic := range ie.heuristics {
		for _, enabled := range enabledHeuristics {
			if strings.Contains(heuristic.Name(), enabled) {
				filtered = append(filtered, heuristic)
				break
			}
		}
	}

	ie.heuristics = filtered
}

// logContext registra información del contexto analizado
func (ie *InferenceEngine) logContext(context DocumentContext) {
	fmt.Printf("Contexto del documento:\n")
	fmt.Printf("  Título: %s\n", context.Title)
	fmt.Printf("  Tema: %s\n", context.Theme)
	fmt.Printf("  Slides: %d\n", context.SlideCount)
	fmt.Printf("  Keywords: %v\n", context.Keywords)
	fmt.Printf("  Secciones: %d\n", len(context.Sections))

	for i, section := range context.Sections {
		fmt.Printf("    %d. %s (hint: %s, keywords: %v)\n",
			i+1, section.Title, section.ElementHint, section.Keywords)
	}
	fmt.Println()
}

// logResult registra el resultado del análisis
func (ie *InferenceEngine) logResult(result InferenceResult) {
	fmt.Printf("=== Resultado de Inferencia ===\n")
	fmt.Printf("Tiempo de procesamiento: %v\n", result.ProcessingTime)
	fmt.Printf("Confianza general: %.2f\n", result.OverallConfidence)
	fmt.Printf("Heurísticas utilizadas: %v\n", result.HeuristicsUsed)
	fmt.Printf("Sugerencias generadas: %d\n", len(result.Suggestions))

	for i, suggestion := range result.Suggestions {
		fmt.Printf("  %d. %s/%s (confianza: %.2f, sección: %d)\n",
			i+1, suggestion.Suggestion.Type, suggestion.Suggestion.Subtype,
			suggestion.Confidence, suggestion.SectionIndex)
		fmt.Printf("     Fuente: %s\n", suggestion.Source)
		fmt.Printf("     Razón: %s\n", suggestion.Reasoning)
		if len(suggestion.Suggestion.Properties) > 0 {
			fmt.Printf("     Propiedades: %v\n", suggestion.Suggestion.Properties)
		}
		fmt.Println()
	}
}

// QuickInfer es un método de conveniencia para inferencia básica
func QuickInfer(content string, minConfidence float64) ([]RankedSuggestion, bool) {
	config := InferenceConfig{
		MinConfidence:  minConfidence,
		MaxSuggestions: 10,
		Verbose:        false,
	}

	engine := NewInferenceEngine(config)
	result := engine.AnalyzeDocument(content)

	return result.Suggestions, len(result.Suggestions) > 0
}

// GetDefaultConfig retorna la configuración predeterminada
func GetDefaultConfig() InferenceConfig {
	return InferenceConfig{
		MinConfidence:    0.5,        // Confianza mínima del 50%
		MaxSuggestions:   20,         // Máximo 20 sugerencias
		EnableHeuristics: []string{}, // Todas las heurísticas habilitadas
		Verbose:          false,
	}
}

// AnalyzeSingleSection analiza una sección específica en lugar del documento completo
func (ie *InferenceEngine) AnalyzeSingleSection(content string, documentContext DocumentContext) []RankedSuggestion {
	return ie.analyzeSection(content, documentContext, 0)
}

// GetSuggestionsByType filtra sugerencias por tipo específico
func (result *InferenceResult) GetSuggestionsByType(elementType string) []RankedSuggestion {
	filtered := make([]RankedSuggestion, 0)

	for _, suggestion := range result.Suggestions {
		if suggestion.Suggestion.Type == elementType {
			filtered = append(filtered, suggestion)
		}
	}

	return filtered
}

// GetTopSuggestion retorna la sugerencia con mayor confianza
func (result *InferenceResult) GetTopSuggestion() *RankedSuggestion {
	if len(result.Suggestions) == 0 {
		return nil
	}
	return &result.Suggestions[0]
}

// HasSuggestions verifica si hay sugerencias disponibles
func (result *InferenceResult) HasSuggestions() bool {
	return len(result.Suggestions) > 0
}

// GetSuggestionsForSection retorna sugerencias para una sección específica
func (result *InferenceResult) GetSuggestionsForSection(sectionIndex int) []RankedSuggestion {
	filtered := make([]RankedSuggestion, 0)

	for _, suggestion := range result.Suggestions {
		if suggestion.SectionIndex == sectionIndex {
			filtered = append(filtered, suggestion)
		}
	}

	return filtered
}
