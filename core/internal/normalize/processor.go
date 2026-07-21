// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalize

import (
	"fmt"
	"time"

	"go.ziradocs.com/core/internal/normalize/inference"
	"go.ziradocs.com/core/internal/normalize/normalizer"
	"go.ziradocs.com/core/util"
)

// ProcessorConfig contiene la configuración para el procesador AI
type ProcessorConfig struct {
	// Configuración de normalización
	EnableNormalization bool              // Habilitar normalización
	NormalizationConfig normalizer.Config // Configuración específica de normalización

	// Configuración de inferencia
	EnableInference bool                      // Habilitar inferencia
	InferenceConfig inference.InferenceConfig // Configuración específica de inferencia

	// Configuración general
	InferenceThreshold float64       // Umbral mínimo para aplicar inferencias
	MaxProcessingTime  time.Duration // Tiempo máximo de procesamiento
}

// ProcessingReport contiene el reporte completo del procesamiento AI
type ProcessingReport struct {
	// Información general
	WasModified    bool          // True si el contenido fue modificado
	ProcessingTime time.Duration // Tiempo total de procesamiento
	OriginalSize   int           // Tamaño original en bytes
	ProcessedSize  int           // Tamaño después del procesamiento

	// Reportes específicos
	NormalizationReport *normalizer.NormalizationReport // Reporte de normalización (si se aplicó)
	InferenceReport     *inference.InferenceResult      // Reporte de inferencia (si se aplicó)

	// Sugerencias aplicables
	Suggestions []inference.ApplicableSuggestion // Sugerencias generadas

	// Métricas
	ConfidenceScore float64 // Puntuación de confianza general
	QualityScore    float64 // Puntuación de calidad del resultado
}

// Processor es el orquestador principal del sistema AI
type Processor struct {
	normalizer      *normalizer.Normalizer
	inferenceEngine *inference.InferenceEngine
	suggester       *inference.Suggester
	config          ProcessorConfig
	logger          util.Logger
}

// NewProcessor crea una nueva instancia del procesador AI
func NewProcessor(config ProcessorConfig, log util.Logger) *Processor {
	var norm *normalizer.Normalizer
	var infEngine *inference.InferenceEngine
	var suggester *inference.Suggester
	// Inicializar normalizer si está habilitado
	if config.EnableNormalization {
		norm = normalizer.NewNormalizer(config.NormalizationConfig, log)
	}
	// Inicializar inference engine si está habilitado
	if config.EnableInference {
		infEngine = inference.NewInferenceEngine(config.InferenceConfig)
		suggester = inference.NewSuggester(false) // No usar verbose, usar logger
	}
	return &Processor{
		normalizer:      norm,
		inferenceEngine: infEngine,
		suggester:       suggester,
		config:          config,
		logger:          log,
	}
}

// Process ejecuta el pipeline completo de procesamiento AI
func (p *Processor) Process(content string) (string, ProcessingReport) {
	startTime := time.Now()

	report := ProcessingReport{
		OriginalSize: len(content),
		WasModified:  false,
		Suggestions:  make([]inference.ApplicableSuggestion, 0),
	}

	processedContent := content

	// Fase 1: Normalización (si está habilitada)
	if p.config.EnableNormalization && p.normalizer != nil {
		normalized, normReport := p.normalizer.Normalize(processedContent)
		report.NormalizationReport = &normReport
		if normReport.WasModified {
			processedContent = normalized
			report.WasModified = true
			report.NormalizationReport = &normReport
		}
	}

	// Fase 2: Inferencia (si está habilitada)
	if p.config.EnableInference && p.inferenceEngine != nil {
		inferenceResult := p.inferenceEngine.AnalyzeDocument(processedContent)
		report.InferenceReport = &inferenceResult

		// Generar sugerencias aplicables
		if p.suggester != nil {
			suggestions := p.suggester.GenerateApplicableSuggestions(inferenceResult) // Filtrar sugerencias por umbral de confianza
			highConfidenceSuggestions := inference.GetHighConfidenceSuggestions(
				suggestions, p.config.InferenceThreshold)

			report.Suggestions = highConfidenceSuggestions
		}
	}

	// Fase 3: Calcular métricas finales
	p.calculateMetrics(&report, processedContent)
	// Verificar tiempo límite
	report.ProcessingTime = time.Since(startTime)
	if report.ProcessingTime > p.config.MaxProcessingTime {
		p.logger.Warn("Procesamiento excedió tiempo límite: %v > %v",
			report.ProcessingTime, p.config.MaxProcessingTime)
	}

	report.ProcessedSize = len(processedContent)

	p.logProcessingSummary(report)

	return processedContent, report
}

// calculateMetrics calcula las métricas de calidad y confianza
func (p *Processor) calculateMetrics(report *ProcessingReport, content string) {
	// Calcular confidence score basado en inferencia
	if report.InferenceReport != nil {
		report.ConfidenceScore = report.InferenceReport.OverallConfidence
	}

	// Calcular quality score basado en normalización y validación
	qualityScore := 1.0

	if report.NormalizationReport != nil {
		// Penalizar por errores de normalización
		qualityScore -= float64(len(report.NormalizationReport.Errors)) * 0.1

		// Bonificar por transformaciones exitosas
		if len(report.NormalizationReport.Applied) > 0 {
			qualityScore += 0.1
		}
	}
	// Validar contenido final
	validator := normalizer.NewValidator(false) // Usar false por defecto
	validationResult := validator.Validate(content)
	qualityScore *= validationResult.Score

	// Asegurar que esté en el rango [0, 1]
	if qualityScore < 0.0 {
		qualityScore = 0.0
	} else if qualityScore > 1.0 {
		qualityScore = 1.0
	}

	report.QualityScore = qualityScore
}

// logProcessingSummary registra un resumen del procesamiento
func (p *Processor) logProcessingSummary(report ProcessingReport) {
	// Only log if there were significant changes
	if report.WasModified && (report.ProcessedSize-report.OriginalSize) != 0 {
		rulesApplied := 0
		if report.NormalizationReport != nil {
			rulesApplied = len(report.NormalizationReport.Applied)
		}

		p.logger.Debug("NORMALIZE-PROCESSOR", "Procesamiento completado → %+d bytes (%d reglas, %v)",
			report.ProcessedSize-report.OriginalSize, rulesApplied, report.ProcessingTime.Truncate(time.Millisecond))
	}
}

// ProcessWithDetection ejecuta el procesamiento usando un resultado de detección previamente calculado
func (p *Processor) ProcessWithDetection(content string, detectionResult normalizer.DetectionResult) (string, ProcessingReport) {
	startTime := time.Now()

	report := ProcessingReport{
		OriginalSize:        len(content),
		WasModified:         false,
		Suggestions:         make([]inference.ApplicableSuggestion, 0),
		NormalizationReport: nil,
		InferenceReport:     nil,
	}

	currentContent := content

	// Fase 1: Normalización (usando detección previa)
	if p.config.EnableNormalization {
		normalizedContent, normReport := p.normalizer.NormalizeWithDetection(currentContent, detectionResult)
		report.NormalizationReport = &normReport

		if normReport.WasModified {
			currentContent = normalizedContent
			report.WasModified = true
		}
	}
	// Completar métricas del reporte
	report.ProcessingTime = time.Since(startTime)
	report.ProcessedSize = len(currentContent)
	p.logProcessingSummary(report)

	return currentContent, report
}

// Summary métodos de conveniencia para el reporte

// HasErrors verifica si hubo errores durante el procesamiento
func (r *ProcessingReport) HasErrors() bool {
	if r.NormalizationReport != nil && len(r.NormalizationReport.Errors) > 0 {
		return true
	}
	return false
}

// GetErrors retorna todos los errores encontrados
func (r *ProcessingReport) GetErrors() []string {
	var errors []string

	if r.NormalizationReport != nil {
		errors = append(errors, r.NormalizationReport.Errors...)
	}

	return errors
}

// GetTransformationsApplied retorna las transformaciones aplicadas
func (r *ProcessingReport) GetTransformationsApplied() []string {
	if r.NormalizationReport != nil {
		return r.NormalizationReport.Applied
	}
	return make([]string, 0)
}

// GetHighConfidenceSuggestions retorna sugerencias con alta confianza
func (r *ProcessingReport) GetHighConfidenceSuggestions(minConfidence float64) []inference.ApplicableSuggestion {
	return inference.GetHighConfidenceSuggestions(r.Suggestions, minConfidence)
}

// Summary genera un resumen textual del procesamiento
func (r *ProcessingReport) Summary() string {
	if !r.WasModified && len(r.Suggestions) == 0 {
		return "No se aplicaron cambios ni se generaron sugerencias"
	}

	summary := fmt.Sprintf("Procesamiento completado en %v. ", r.ProcessingTime)

	if r.WasModified {
		summary += fmt.Sprintf("Contenido modificado (%d→%d bytes). ", r.OriginalSize, r.ProcessedSize)
	}

	if len(r.Suggestions) > 0 {
		summary += fmt.Sprintf("%d sugerencias generadas. ", len(r.Suggestions))
	}
	summary += fmt.Sprintf("Calidad: %.0f%%, Confianza: %.0f%%",
		r.QualityScore*100, r.ConfidenceScore*100)

	return summary
}
