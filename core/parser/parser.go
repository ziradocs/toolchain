// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/normalize"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
	"go.ziradocs.com/core/v2/util"
)

type Parser struct {
	frontMatterParser   *FrontMatterParser
	enableNormalize     bool
	lastDetectionResult *normalizer.DetectionResult // Almacena el último resultado de detección
	logger              util.Logger
}

// New crea un Parser. log nunca queda nil: un caller que pase nil recibe un
// Noop en su lugar, en vez de propagar nil hacia p.logger.Info/Debug (acá
// mismo y en NewStrictParser/NewFlexParser, issue #134/G1c) y panicar en el
// primer log del parse.
func New(log util.Logger) *Parser {
	if log == nil {
		log = util.NewNoop()
	}
	return &Parser{
		frontMatterParser: &FrontMatterParser{},
		enableNormalize:   true, // Habilitado por defecto: normalización determinística sin red
		logger:            log,
	}
}

// SetNormalization habilita/deshabilita el pre-procesamiento de normalización
// (detección de patrones + transformaciones determinísticas, sin red — ver
// internal/normalize). Nombre canónico; SetAIProcessing es un alias
// deprecado que delega acá (decisión 2 del plan OSS: el normalizador base
// es determinista, "AI" se reserva para el llm-kit).
func (p *Parser) SetNormalization(enabled bool) {
	p.enableNormalize = enabled
}

// EnableNormalization es un método de conveniencia para habilitar
// normalización con configuración por defecto. Nombre canónico de
// EnableAIProcessing (deprecado).
func (p *Parser) EnableNormalization() {
	p.SetNormalization(true)
}

// SetAIProcessing es un alias deprecado de SetNormalization.
//
// Deprecated: usa SetNormalization en su lugar.
func (p *Parser) SetAIProcessing(enabled bool) {
	p.SetNormalization(enabled)
}

// EnableAIProcessing es un alias deprecado de EnableNormalization.
//
// Deprecated: usa EnableNormalization en su lugar.
func (p *Parser) EnableAIProcessing() {
	p.SetNormalization(true)
}

// ParseDocument parsea un DOCUMENTO (`.doclang`) despachando por el `mode:`
// del frontmatter, y es el espejo documental de Parse: mismas tres fases
// —leer el modo, normalizar salvo en strict, elegir parser— pero sobre la
// familia de parsers cuya unidad es la SECCIÓN, no la diapositiva.
//
// Modos:
//   - "strict"                     → DocumentStrictParser, sin normalizar nunca.
//   - "flex", "flex-full", "auto"  → DocumentFlexParser. En documentos son
//     sinónimos: hay una sola gramática flex, y "auto" no tiene nada que
//     autodetectar (a diferencia de las presentaciones, donde elige entre
//     dos parsers distintos).
//   - sin frontmatter, o frontmatter ilegible → flex, que es lo que un
//     `.doclang` sin declarar siempre significó.
//
// El modo se lee del contenido ORIGINAL y solo se usa para despachar: es el
// parser elegido quien vuelve a parsear el frontmatter y lo cuelga del AST.
// Esa duplicación es deliberada — evita emitir dos veces los diagnósticos
// del frontmatter, y deja intacto que un documento SIN frontmatter reciba
// el que le inyecta el normalizador (regla InjectionRule, `mode: flex-full`),
// como venía pasando.
//
// La normalización queda gobernada por SetNormalization (activa por defecto,
// igual que en Parse), salvo en strict, donde no corre en ningún caso: un
// documento que declara el dialecto auditable no puede ser reescrito antes
// de leerse.
func (p *Parser) ParseDocument(content string, filePath string) (*ast.AST, []diagnostics.Diagnostic) {
	var allDiagnostics []diagnostics.Diagnostic

	// FASE 1: leer el modo (solo para despachar; ver el doc comment).
	mode := peekDocumentMode(content)

	// FASE 2: normalización, nunca en strict.
	processedContent := content
	if p.enableNormalize && mode != "strict" {
		detector := normalizer.NewDetector()
		detectionResult := detector.Detect(content)
		p.lastDetectionResult = &detectionResult

		if detectionResult.Detected {
			p.logger.Info("NORMALIZE", "🔍 Detectado contenido AI (score: %.2f, %d patrones)",
				detectionResult.Score, len(detectionResult.Patterns))
			for i, pattern := range detectionResult.Patterns {
				p.logger.Debug("NORMALIZE", "  [%d] %s (confianza: %.2f, línea: %d): %s",
					i+1, pattern.Type, pattern.Confidence, pattern.Line, pattern.Description)
			}
		}

		processed, report := normalize.ProcessWithDetection(content, detectionResult, base.DialectDocuments, p.logger)
		if report.WasModified {
			rulesApplied := len(report.GetTransformationsApplied())
			changeBytes := len(processed) - len(content)
			p.logger.Info("NORMALIZE", "Normalización aplicada → %d reglas, %+d bytes", rulesApplied, changeBytes)
			for i, rule := range report.GetTransformationsApplied() {
				p.logger.Debug("NORMALIZE", "  [%d] %s", i+1, rule)
			}
			processedContent = processed
		}

		if report.HasErrors() {
			for _, errMsg := range report.GetErrors() {
				allDiagnostics = append(allDiagnostics,
					diagnostics.NewError(errMsg, diagnostics.NewPosition(1, 1), "preprocessor"))
			}
		}
	}

	// FASE 3: despachar.
	var astNode *ast.AST
	var bodyDiagnostics []diagnostics.Diagnostic

	if mode == "strict" {
		astNode, bodyDiagnostics = NewDocumentStrictParser(processedContent, p.logger).Parse()
	} else {
		astNode, bodyDiagnostics = NewDocumentFlexParser(processedContent, p.logger).Parse()
	}

	if astNode != nil && filePath != "" {
		astNode.FilePath = filePath
	}

	allDiagnostics = append(allDiagnostics, bodyDiagnostics...)
	return astNode, allDiagnostics
}

// peekDocumentMode devuelve el `mode:` declarado en content, o "" si no hay
// frontmatter o no se puede leer. DESCARTA los diagnósticos a propósito: el
// parser que se elija va a re-parsear el mismo frontmatter y emitirlos él,
// así que quedárselos acá los duplicaría. Un frontmatter roto cae a "" y de
// ahí al parser flex, que es quien reporta el error — el mismo que se
// reportaba antes de que existiera este despacho.
func peekDocumentMode(content string) string {
	fm, _, _ := (&FrontMatterParser{}).Parse(content)
	if fm == nil {
		return ""
	}
	return fm.Mode
}

func (p *Parser) Parse(content string, filePath string) (*ast.AST, []diagnostics.Diagnostic) {
	var allDiagnostics []diagnostics.Diagnostic

	// FASE 1: Parsear FrontMatter primero para determinar el modo
	frontMatter, bodyContent, fmDiagnostics := p.frontMatterParser.Parse(content)
	allDiagnostics = append(allDiagnostics, fmDiagnostics...)

	if frontMatter == nil {
		return nil, allDiagnostics
	}

	// FASE 2: Pre-procesamiento AI solo si no es modo strict
	processedContent := content
	var preProcessReport *normalize.ProcessingReport
	if p.enableNormalize && frontMatter.Mode != "strict" {
		detector := normalizer.NewDetector()
		detectionResult := detector.Detect(content)

		// Almacenar el resultado para reutilizarlo en otras fases
		p.lastDetectionResult = &detectionResult // Logging detallado de la detección AI
		if detectionResult.Detected {
			p.logger.Info("NORMALIZE", "🔍 Detectado contenido AI (score: %.2f, %d patrones)",
				detectionResult.Score, len(detectionResult.Patterns))

			// Detalles de los patrones solo en modo detailed
			for i, pattern := range detectionResult.Patterns {
				p.logger.Debug("NORMALIZE", "  [%d] %s (confianza: %.2f, línea: %d): %s",
					i+1, pattern.Type, pattern.Confidence, pattern.Line, pattern.Description)
			}
		}

		// Usar la nueva API del factory para el pre-procesamiento
		processed, report := normalize.ProcessWithDetection(content, detectionResult, base.DialectSlides, p.logger)
		preProcessReport = &report
		if report.WasModified {
			processedContent = processed

			// Información esencial - siempre visible
			rulesApplied := len(report.GetTransformationsApplied())
			changeBytes := len(processedContent) - len(content)
			p.logger.Info("NORMALIZE", "Normalización aplicada → %d reglas, %+d bytes", rulesApplied, changeBytes)

			// Detalles - solo en modo detailed/debug
			for i, rule := range report.GetTransformationsApplied() {
				p.logger.Debug("NORMALIZE", "  [%d] %s", i+1, rule)
			}

			// Agregar diagnóstico simple para el linter
			pos := diagnostics.NewPosition(1, 1)
			aiDiag := diagnostics.Diagnostic{
				Severity: diagnostics.Info,
				Message:  fmt.Sprintf("Normalización aplicada: %d reglas, %+d bytes", rulesApplied, changeBytes),
				Position: pos,
				Source:   "normalizer",
			}
			allDiagnostics = append(allDiagnostics, aiDiag)
		}
		// Información sobre detección AI
		if detectionResult.Detected {
			p.logger.Info("NORMALIZE", "Contenido AI detectado → puntuación: %.2f, patrones: %d",
				detectionResult.Score, len(detectionResult.Patterns))

			// Diagnóstico simple para el linter
			pos := diagnostics.NewPosition(1, 1)
			aiDetectedDiag := diagnostics.Diagnostic{
				Severity: diagnostics.Info,
				Message:  fmt.Sprintf("Contenido AI detectado: %d patrones", len(detectionResult.Patterns)),
				Position: pos,
				Source:   "detector",
			}
			allDiagnostics = append(allDiagnostics, aiDetectedDiag)
		}

		// Agregar errores de procesamiento AI como diagnósticos
		if report.HasErrors() {
			for _, errMsg := range report.GetErrors() {
				pos := diagnostics.NewPosition(1, 1)
				aiErrorDiag := diagnostics.NewError(errMsg, pos, "preprocessor")
				allDiagnostics = append(allDiagnostics, aiErrorDiag)
			}
		}

		// Re-parsear el frontmatter del contenido procesado para obtener el bodyContent actualizado
		_, bodyContent, _ = p.frontMatterParser.Parse(processedContent)
	}

	// FASE 3: Determinar modo y parsear cuerpo
	var astNode *ast.AST
	var bodyDiagnostics []diagnostics.Diagnostic

	switch frontMatter.Mode {
	case "strict":
		strictParser := NewStrictParser(bodyContent, p.logger)
		astNode, bodyDiagnostics = strictParser.Parse()
		astNode.FrontMatter = frontMatter
		astNode.FilePath = filePath
	case "flex": // Si el AI está habilitado, el contenido ya fue pre-procesado, usar tal como está
		var processedBodyContent string
		if p.enableNormalize {
			// El bodyContent ya proviene del processedContent que fue normalizado por AI
			processedBodyContent = bodyContent
		} else {
			// AI deshabilitado, aplicar normalización básica manual
			processedBodyContent = p.applyBasicNormalization(bodyContent)
		}
		flexParser := NewFlexParser(processedBodyContent, p.logger)
		astNode, bodyDiagnostics = flexParser.Parse()
		astNode.FrontMatter = frontMatter
		astNode.FilePath = filePath
	case "flex-ai", "flex-full": // "flex-ai" es un alias deprecado de "flex-full" (mismo comportamiento)
		// Si el AI está habilitado, el contenido ya fue pre-procesado, usar tal como está
		var processedBodyContent string
		if p.enableNormalize {
			// El bodyContent ya proviene del processedContent que fue normalizado por AI
			processedBodyContent = bodyContent
			p.logger.Debug("PARSE", "=== MODO FLEX-FULL: USANDO CONTENIDO PRE-PROCESADO POR AI ===")
			p.logger.Debug("PARSE", "Saltando normalización completa (ya aplicada en pre-procesamiento)")
		} else {
			// AI deshabilitado, aplicar normalización completa manual
			processedBodyContent = p.applyFullNormalization(bodyContent)
		}
		flexParser := NewFlexParser(processedBodyContent, p.logger)
		astNode, bodyDiagnostics = flexParser.Parse()
		astNode.FrontMatter = frontMatter
		astNode.FilePath = filePath
	case "auto":
		// Modo automático: detecta si es AI y aplica el procesamiento apropiado
		// Si ya hubo pre-procesamiento, usamos ese resultado
		autoParser := p.createAutoParser(bodyContent, preProcessReport)
		astNode, bodyDiagnostics = autoParser.Parse()
		astNode.FrontMatter = frontMatter
		astNode.FilePath = filePath
	default: // Para modos desconocidos, intentar auto-detección AI si está habilitado
		if p.enableNormalize {
			pos := diagnostics.NewPosition(1, 1)
			aiDiag := diagnostics.Diagnostic{
				Severity: diagnostics.Warning,
				Message:  fmt.Sprintf("Modo desconocido '%s', aplicando auto-detección AI", frontMatter.Mode),
				Position: pos,
				Source:   "parser",
			}
			allDiagnostics = append(allDiagnostics, aiDiag)
			autoParser := p.createAutoParser(bodyContent, preProcessReport)
			astNode, bodyDiagnostics = autoParser.Parse()
			astNode.FrontMatter = frontMatter
			astNode.FilePath = filePath
		} else {
			allDiagnostics = append(allDiagnostics,
				diagnostics.NewError("Unknown mode: "+frontMatter.Mode,
					diagnostics.NewPosition(1, 1), "parser"))
			return nil, allDiagnostics
		}
	}

	allDiagnostics = append(allDiagnostics, bodyDiagnostics...)
	return astNode, allDiagnostics
}

// createAutoParser crea un parser automático que detecta si es contenido generado por IA
func (p *Parser) createAutoParser(bodyContent string, preProcessReport *normalize.ProcessingReport) *FlexParser {
	// Si ya hubo pre-procesamiento AI, usamos ese resultado
	if preProcessReport != nil && preProcessReport.WasModified {
		p.logger.Debug("PARSE", "Auto-detección: usando contenido ya pre-procesado - %s", preProcessReport.Summary())
	} else {
		// Solo detectamos y procesamos si no hubo pre-procesamiento
		if p.enableNormalize {
			detector := normalizer.NewDetector()
			detectionResult := detector.Detect(bodyContent)

			if detectionResult.Detected {
				// Si es AI, aplicar procesamiento completo usando la nueva API
				processed, report := normalize.ProcessContent(bodyContent, normalize.ContentModeFull, base.DialectSlides, p.logger)

				if report.WasModified {
					bodyContent = processed
					p.logger.Debug("PARSE", "Auto-detección: contenido AI procesado - %s", report.Summary())
				}
			} else {
				p.logger.Debug("PARSE", "Auto-detección: contenido normal, usando flex estándar")
			}
		}
	}

	// Crear parser flex con el contenido (procesado o no)
	flexParser := NewFlexParser(bodyContent, p.logger)

	return flexParser
}

// applyBasicNormalization aplica normalización básica (para modo flex)
func (p *Parser) applyBasicNormalization(content string) string {
	p.logger.Debug("PARSE", "=== APLICANDO NORMALIZACIÓN BÁSICA (FLEX) ===")

	// Configuración para normalización básica
	config := normalizer.Config{
		EnableDetection:  false,                   // No necesitamos detección AI
		EnableTransforms: true,                    // Sí queremos aplicar transformaciones
		SkipRules:        []string{"frontmatter"}, // Saltar reglas de frontmatter ya que estamos procesando solo body content
		BodyOnly:         true,                    // Indicar que estamos procesando solo el cuerpo
		Dialect:          base.DialectSlides,      // este parser es el de slidelang
	}

	norm := normalizer.NewNormalizer(config, p.logger)
	normalized, report := norm.Normalize(content)
	// if p.verbose && report.WasModified {
	if report.WasModified {
		p.logger.Debug("PARSE", "Normalización básica aplicada: %d reglas, %d bytes → %d bytes",
			len(report.Applied), report.OriginalSize, report.NormalizedSize)
		for _, rule := range report.Applied {
			p.logger.Debug("PARSE", "  - %s", rule)
		}
	}

	return normalized
}

// applyFullNormalization aplica normalización completa (para modo flex-full, alias deprecado flex-ai)
func (p *Parser) applyFullNormalization(content string) string {
	p.logger.Debug("PARSE", "=== APLICANDO NORMALIZACIÓN COMPLETA (FLEX-AI) ===")

	// Configuración para normalización completa
	config := normalizer.Config{
		EnableDetection:  false,                   // No necesitamos detección AI, sabemos que es AI
		EnableTransforms: true,                    // Sí queremos aplicar todas las transformaciones
		SkipRules:        []string{"frontmatter"}, // Saltar reglas de frontmatter ya que estamos procesando solo body content
		Dialect:          base.DialectSlides,      // este parser es el de slidelang
	}
	norm := normalizer.NewNormalizer(config, p.logger)
	normalized, report := norm.Normalize(content)
	if report.WasModified {
		p.logger.Debug("PARSE", "Normalización completa aplicada: %d reglas, %d bytes → %d bytes",
			len(report.Applied), report.OriginalSize, report.NormalizedSize)
		for _, rule := range report.Applied {
			p.logger.Debug("PARSE", "  - %s", rule)
		}
	}

	return normalized
}
