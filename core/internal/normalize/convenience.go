// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalize

import (
	"go.ziradocs.com/core/v2/internal/normalize/inference"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
	"go.ziradocs.com/core/v2/util"
)

// ProcessContent es la API unificada para procesar contenido con un modo
// específico. dialect declara cuál de los dos DSLs se está parseando; ver
// base.Dialect y el doc comment de ProcessWithDetection.
func ProcessContent(content string, mode ContentMode, dialect base.Dialect, log util.Logger) (string, ProcessingReport) {
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

	return processor.setDialect(dialect).Process(content)
}

// ProcessContentByString procesa contenido usando un string de modo.
//
// SIN LLAMADORES hoy (verificado en core/, doclang/ y slidelang/). Se anota
// porque es una trampa para quien venga a cablear algo por acá: no recibe
// base.Dialect, así que si algún día se usa desde una ruta de doclang hay que
// agregárselo igual que a ProcessContent, o el normalizador vuelve a correr
// reglas de slides sobre documentos sin que ningún test lo vea (DialectAny
// preserva el comportamiento viejo en silencio).
func ProcessContentByString(content string, mode string, log util.Logger) (string, ProcessingReport, error) {
	factory := NewFactory(log)
	processor, err := factory.ForMode(mode)
	if err != nil {
		return "", ProcessingReport{}, err
	}

	processed, report := processor.Process(content)
	return processed, report, nil
}

// ProcessWithDetection procesa contenido usando un resultado de detección
// previo. dialect declara cuál de los dos DSLs se está parseando: quien llama
// siempre lo sabe, y sin ese dato el normalizador corre reglas cuya premisa es
// el modelo de slides sobre documentos (ver base.Dialect). Pasar
// base.DialectAny mantiene el comportamiento de aplicar todas las reglas.
func ProcessWithDetection(content string, detectionResult normalizer.DetectionResult, dialect base.Dialect, log util.Logger) (string, ProcessingReport) {
	factory := NewFactory(log)
	processor := factory.ForBodyContentInDialect(detectionResult.Detected, dialect)
	return processor.ProcessWithDetection(content, detectionResult)
}

// GetSuggestions obtiene sugerencias de mejora para el contenido.
//
// SIN LLAMADORES hoy, misma advertencia que ProcessContentByString arriba.
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
