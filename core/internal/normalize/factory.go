// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalize

import (
	"errors"
	"time"

	"go.ziradocs.com/core/internal/normalize/normalizer"
	"go.ziradocs.com/core/util"
)

// ContentMode define los diferentes modos de procesamiento de contenido
type ContentMode int

const (
	ContentModeStrict   ContentMode = iota // Sin procesamiento AI (modo strict)
	ContentModeFlex                        // Normalización básica preventiva (modo flex)
	ContentModeFull                        // Procesamiento completo AI (modo flex-full, alias deprecado flex-ai)
	ContentModeBodyOnly                    // Solo cuerpo, normalización básica
	ContentModeBodyFull                    // Solo cuerpo, procesamiento AI completo
)

// String implementa fmt.Stringer para ContentMode
func (c ContentMode) String() string {
	switch c {
	case ContentModeStrict:
		return "strict"
	case ContentModeFlex:
		return "flex"
	case ContentModeFull:
		return "full"
	case ContentModeBodyOnly:
		return "body-only"
	case ContentModeBodyFull:
		return "body-full"
	default:
		return "unknown"
	}
}

// Factory es el factory centralizado para crear procesadores de normalización
// con configuraciones semánticas predefinidas
type Factory struct {
	logger util.Logger
}

// NewFactory crea una nueva instancia del factory de normalización
func NewFactory(log util.Logger) *Factory {
	return &Factory{
		logger: log,
	}
}

// ForStrictMode crea un procesador para modo strict
// - Sin normalización (strict no necesita AI)
// - Sin inferencia
func (f *Factory) ForStrictMode() *Processor {
	config := ProcessorConfig{
		EnableNormalization: false,
		EnableInference:     false,
		MaxProcessingTime:   time.Second * 5, // Timeout mínimo
	}
	return f.createProcessor(config)
}

// ForFlexMode crea un procesador para modo flex
// - Normalización básica preventiva
// - Sin inferencia (solo corrección)
func (f *Factory) ForFlexMode() *Processor {
	normConfig := normalizer.Config{
		EnableDetection:  true,       // Detectar contenido AI
		EnableTransforms: true,       // Aplicar transformaciones básicas
		SkipRules:        []string{}, // No saltar reglas por defecto
		BodyOnly:         false,      // Contenido completo
	}

	config := ProcessorConfig{
		EnableNormalization: true,
		NormalizationConfig: normConfig,
		EnableInference:     false, // Sin inferencia para flex normal
		MaxProcessingTime:   time.Second * 10,
	}
	return f.createProcessor(config)
}

// ForFullContent crea un procesador para normalización completa (modo flex-full, alias deprecado flex-ai)
// - Normalización completa
// - Inferencia habilitada
func (f *Factory) ForFullContent() *Processor {
	normConfig := normalizer.Config{
		EnableDetection:  true,       // Detectar patrones AI
		EnableTransforms: true,       // Aplicar todas las transformaciones
		SkipRules:        []string{}, // No saltar reglas
		BodyOnly:         false,      // Contenido completo
	}

	config := ProcessorConfig{
		EnableNormalization: true,
		NormalizationConfig: normConfig,
		EnableInference:     true,             // Habilitar inferencia para AI
		InferenceThreshold:  0.6,              // Umbral de confianza
		MaxProcessingTime:   time.Second * 30, // Más tiempo para procesamiento completo
	}
	return f.createProcessor(config)
}

// ForBodyContent crea un procesador para contenido de cuerpo (sin frontmatter)
// - Para usar en parser cuando ya se extrajo el frontmatter
// - aiContent determina el nivel de procesamiento
func (f *Factory) ForBodyContent(aiContent bool) *Processor {
	normConfig := normalizer.Config{
		EnableDetection:  true,                    // Detectar patrones AI
		EnableTransforms: true,                    // Aplicar transformaciones
		SkipRules:        []string{"frontmatter"}, // Saltar reglas de frontmatter
		BodyOnly:         true,                    // Solo cuerpo de contenido
	}

	// Configuración base
	config := ProcessorConfig{
		EnableNormalization: true,
		NormalizationConfig: normConfig,
		EnableInference:     aiContent, // Inferencia solo si es contenido AI
		MaxProcessingTime:   time.Second * 15,
	}

	// Ajustar umbral según tipo de contenido
	if aiContent {
		config.InferenceThreshold = 0.6
	}

	return f.createProcessor(config)
}

// ForMode crea un procesador basado en el modo de parsing detectado
func (f *Factory) ForMode(mode string) (*Processor, error) {
	switch mode {
	case "strict":
		return f.ForStrictMode(), nil
	case "flex":
		return f.ForFlexMode(), nil
	case "flex-full", "flex-ai", "auto": // "flex-ai" es alias deprecado de "flex-full"
		return f.ForFullContent(), nil
	default:
		f.logger.Warn("Modo desconocido '%s', modos válidos: strict, flex, flex-full, flex-ai, auto", mode)
		return nil, errors.New("modo desconocido: " + mode)
	}
}

// createProcessor es el método interno para crear procesadores
func (f *Factory) createProcessor(config ProcessorConfig) *Processor {
	return NewProcessor(config, f.logger)
}
