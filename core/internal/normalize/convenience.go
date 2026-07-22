// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalize

import (
	"go.ziradocs.com/core/v2/internal/normalize/inference"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer"
	"go.ziradocs.com/core/v2/util"
)

// ProcessContent es la API unificada para procesar contenido con un modo específico
func ProcessContent(content string, mode ContentMode, log util.Logger) (string, ProcessingReport) {
	factory := NewFactory(log)

	var processor *Processor
	switch mode {
	case ContentModeStrict:
		processor = factory.ForStrictMode()
	case ContentModeFlex:
		processor = factory.ForFlexMode()
	case ContentModeFull:
		processor = factory.ForFullContent()
	case ContentModeBodyOnly:
		processor = factory.ForBodyContent(false)
	case ContentModeBodyFull:
		processor = factory.ForBodyContent(true)
	default:
		processor = factory.ForFlexMode()
	}

	return processor.Process(content)
}

// ProcessContentByString procesa contenido usando un string de modo
func ProcessContentByString(content string, mode string, log util.Logger) (string, ProcessingReport, error) {
	factory := NewFactory(log)
	processor, err := factory.ForMode(mode)
	if err != nil {
		return "", ProcessingReport{}, err
	}

	processed, report := processor.Process(content)
	return processed, report, nil
}

// ProcessWithDetection procesa contenido usando un resultado de detección previo
func ProcessWithDetection(content string, detectionResult normalizer.DetectionResult, log util.Logger) (string, ProcessingReport) {
	factory := NewFactory(log)
	processor := factory.ForBodyContent(detectionResult.Detected)
	return processor.ProcessWithDetection(content, detectionResult)
}

// GetSuggestions obtiene sugerencias de mejora para el contenido
func GetSuggestions(content string, minConfidence float64, log util.Logger) []inference.ApplicableSuggestion {
	factory := NewFactory(log)
	processor := factory.ForFullContent()

	// Deshabilitar normalización para solo obtener sugerencias
	processor.config.EnableNormalization = false

	_, report := processor.Process(content)
	return inference.GetHighConfidenceSuggestions(report.Suggestions, minConfidence)
}

// ContentModeFromString convierte un string de modo a ContentMode
func ContentModeFromString(mode string) ContentMode {
	switch mode {
	case "strict":
		return ContentModeStrict
	case "flex":
		return ContentModeFlex
	case "flex-full", "flex-ai", "auto": // "flex-ai" es alias deprecado de "flex-full"
		return ContentModeFull
	default:
		return ContentModeFlex
	}
}
