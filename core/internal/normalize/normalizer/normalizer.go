// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalizer

import (
	"sort"
	"strings"
	"time"

	"go.ziradocs.com/core/internal/normalize/normalizer/base"
	"go.ziradocs.com/core/util"
)

// NormalizationReport contiene información sobre las transformaciones aplicadas
type NormalizationReport struct {
	Applied         []string        // Lista de reglas aplicadas
	Errors          []string        // Errores encontrados
	ProcessingTime  time.Duration   // Tiempo de procesamiento
	OriginalSize    int             // Tamaño original en bytes
	NormalizedSize  int             // Tamaño después de normalización
	DetectionResult DetectionResult // Resultado de la detección de AI
	WasModified     bool            // True si el contenido fue modificado
}

// Config contiene la configuración para el normalizador
type Config struct {
	EnableDetection  bool     // Habilitar detección de patrones AI
	EnableTransforms bool     // Habilitar transformaciones
	SkipRules        []string // Reglas a saltar
	BodyOnly         bool     // True si estamos procesando solo el cuerpo (sin frontmatter)
}

// Normalizer es el pipeline principal de normalización
type Normalizer struct {
	detector *Detector
	rules    []base.TransformRule
	config   Config
	logger   util.Logger
}

// NewNormalizer crea una nueva instancia del normalizador
func NewNormalizer(config Config, log util.Logger) *Normalizer {
	// Usar reglas estándar
	rules := GetTransformRules(log)
	normalizer := &Normalizer{
		detector: NewDetector(),
		rules:    rules,
		config:   config,
		logger:   log,
	}

	// Filtrar reglas si hay alguna en skip list
	if len(config.SkipRules) > 0 {
		normalizer.filterRules(config.SkipRules)
	}

	// Ordenar reglas por prioridad
	sort.Slice(normalizer.rules, func(i, j int) bool {
		return normalizer.rules[i].Priority() < normalizer.rules[j].Priority()
	})

	return normalizer
}

// Normalize ejecuta el pipeline completo de normalización
func (n *Normalizer) Normalize(content string) (string, NormalizationReport) {
	startTime := time.Now()

	report := NormalizationReport{
		Applied:      make([]string, 0),
		Errors:       make([]string, 0),
		OriginalSize: len(content),
		WasModified:  false,
	} // Fase 1: Detección de patrones AI
	if n.config.EnableDetection {
		report.DetectionResult = n.detector.Detect(content)
		n.logDetectionResults(report.DetectionResult)
	}
	// Si no se detectaron patrones AI y la detección está habilitada,
	// aplicar solo reglas de formateo básico (como MermaidFormatter)
	if n.config.EnableDetection && !report.DetectionResult.Detected {
		// Apply basic formatting only
		normalizedContent := n.applyBasicFormatting(content, &report)
		report.ProcessingTime = time.Since(startTime)
		report.NormalizedSize = len(normalizedContent)
		return normalizedContent, report
	}

	// Fase 2: Aplicar transformaciones
	normalizedContent := content
	if n.config.EnableTransforms {
		normalizedContent = n.applyTransformations(content, &report)
	}

	// Fase 3: Validación del resultado
	validationErrors := n.validateResult(normalizedContent)
	report.Errors = append(report.Errors, validationErrors...)
	// Finalizar reporte
	report.ProcessingTime = time.Since(startTime)
	report.NormalizedSize = len(normalizedContent)
	report.WasModified = normalizedContent != content

	n.logNormalizationSummary(report)

	return normalizedContent, report
}

// NormalizeWithDetection ejecuta el pipeline de normalización con un resultado de detección previamente calculado
// Esto evita duplicar la detección de AI cuando ya se ejecutó en una fase anterior
func (n *Normalizer) NormalizeWithDetection(content string, detectionResult DetectionResult) (string, NormalizationReport) {
	startTime := time.Now()

	report := NormalizationReport{
		Applied:         make([]string, 0),
		Errors:          make([]string, 0),
		OriginalSize:    len(content),
		WasModified:     false,
		DetectionResult: detectionResult, // Usar resultado ya calculado
	}
	// No ejecutar detección nuevamente, usar el resultado proporcionado
	if n.config.EnableDetection && len(detectionResult.Patterns) > 0 {
		// Using previous AI detection result
	}

	// Si no se detectaron patrones AI y la detección está habilitada,
	// aplicar solo reglas de formateo básico (como MermaidFormatter)
	if n.config.EnableDetection && !report.DetectionResult.Detected {
		normalizedContent := n.applyBasicFormatting(content, &report)
		report.ProcessingTime = time.Since(startTime)
		report.NormalizedSize = len(normalizedContent)
		return normalizedContent, report
	}

	// Fase 2: Aplicar transformaciones
	normalizedContent := content
	if n.config.EnableTransforms {
		normalizedContent = n.applyTransformations(content, &report)
	}

	// Fase 3: Validación del resultado
	validationErrors := n.validateResult(normalizedContent)
	report.Errors = append(report.Errors, validationErrors...)
	// Finalizar reporte
	report.ProcessingTime = time.Since(startTime)
	report.NormalizedSize = len(normalizedContent)
	report.WasModified = normalizedContent != content

	n.logNormalizationSummary(report)

	return normalizedContent, report
}

// applyTransformations aplica todas las reglas de transformación en orden de prioridad
func (n *Normalizer) applyTransformations(content string, report *NormalizationReport) string {
	currentContent := content
	var frontmatter, bodyContent string
	var hasFrontmatter bool
	// Solo intentar separar frontmatter si NO estamos procesando solo el cuerpo
	if !n.config.BodyOnly {
		// PASO 1: Aplicar reglas de limpieza inicial (prioridad -1 y 0) al contenido completo
		// Esto incluye BackticksCleanupRule y YamlEscapingRule que necesitan limpiar antes de detectar frontmatter
		for _, rule := range n.rules {
			if rule.Priority() == -1 || (rule.Priority() == 0 && n.shouldApplyCleanupRuleEarly(rule)) {
				transformed, err := rule.Apply(currentContent)
				if err != nil {
					n.logger.Warn("Error en regla %s: %v", rule.Description(), err)
					report.Errors = append(report.Errors, err.Error())
					continue
				}

				if transformed != currentContent {
					currentContent = transformed
					report.Applied = append(report.Applied, rule.Description())
					report.WasModified = true
				}
			}
		}
		// PASO 2: Separar frontmatter del contenido ya limpiado
		frontmatter, bodyContent, hasFrontmatter = n.extractFrontmatter(currentContent)

		// PASO 3: Aplicar el resto de transformaciones SOLO al body content
		if hasFrontmatter {
			currentContent = bodyContent
		}
	}
	// Si estamos procesando solo el cuerpo, trabajar directamente con el contenido

	// Aplicar transformaciones al contenido
	for _, rule := range n.rules {
		// Verificar si la regla debe ser saltada
		if n.shouldSkipRule(rule) {
			continue
		}

		// Skip early cleanup rules as they were already applied
		if rule.Priority() == -1 || (rule.Priority() == 0 && n.shouldApplyCleanupRuleEarly(rule)) {
			continue
		}
		// IMPORTANTE: Saltar la regla de inyección de frontmatter si ya existe o si estamos procesando solo el cuerpo
		if (hasFrontmatter && strings.Contains(rule.Description(), "frontmatter")) ||
			(n.config.BodyOnly && strings.Contains(rule.Description(), "frontmatter")) {
			continue
		}

		// Aplicar la transformación
		transformed, err := rule.Apply(currentContent)
		if err != nil {
			n.logger.Warn("Error en regla %s: %v", rule.Description(), err)
			report.Errors = append(report.Errors, err.Error())
			continue
		}

		// Si hubo cambios, registrar la regla como aplicada
		if transformed != currentContent {
			report.Applied = append(report.Applied, rule.Description())
			currentContent = transformed
		}
	}
	// Recombinar frontmatter con contenido procesado solo si NO estamos procesando solo el cuerpo
	if !n.config.BodyOnly && hasFrontmatter {
		finalContent := frontmatter + "\n" + currentContent
		return finalContent
	} else {
		return currentContent
	}
}

// applyBasicFormatting aplica solo reglas básicas de formateo (para archivos no detectados como AI)
func (n *Normalizer) applyBasicFormatting(content string, report *NormalizationReport) string {
	// Reglas "esenciales" que corren SIEMPRE, aunque el contenido no se haya
	// detectado como generado por IA. Originalmente sólo cubría formateo
	// puramente cosmético (indentación de mermaid); CodeGroupFormatter se
	// sumó en el fix del issue #174 porque, sin ella, contenido escrito a
	// mano con el wrapper ":::code-item{title=}" llega al parser sin
	// reescribir y sólo el primer tab del code-group sobrevive — el parser
	// real (elements/code_group.go) nunca entendió ese wrapper, así que
	// reescribirlo aquí no es opcional para que el documento sea válido.
	// Si se agrega algo más a esta lista, confirmar que sea igual de
	// necesario (no sólo "vendría bien") — cada regla nueva corre sobre
	// TODO contenido flex, incluso el que ya está en sintaxis canónica.
	basicRules := []string{
		"MermaidFormatter",   // Para formatear bloques mermaid con indentación
		"CodeGroupFormatter", // Para normalizar sintaxis de code-groups (::::code-group / :::code-item{})
	}

	currentContent := content
	for _, rule := range n.rules {
		// Solo aplicar reglas de formateo básico
		shouldApply := false
		for _, basicRule := range basicRules {
			if strings.Contains(rule.Description(), basicRule) {
				shouldApply = true
				break
			}
		}
		if !shouldApply {
			continue
		}
		n.logger.Debug("NORMALIZER", "Aplicando regla básica: %s", rule.Description())
		transformed, err := rule.Apply(currentContent)
		if err != nil {
			n.logger.Warn("Error en regla básica %s: %v", rule.Description(), err)
			report.Errors = append(report.Errors, err.Error())
			continue
		}

		if transformed != currentContent {
			currentContent = transformed
			report.Applied = append(report.Applied, rule.Description())
			report.WasModified = true
		}
	}

	return currentContent
}

// shouldSkipRule verifica si una regla debe ser saltada
func (n *Normalizer) shouldSkipRule(rule base.TransformRule) bool {
	ruleDesc := rule.Description()
	for _, skipRule := range n.config.SkipRules {
		if strings.Contains(ruleDesc, skipRule) {
			return true
		}
	}
	return false
}

// filterRules filtra las reglas basándose en la configuración
func (n *Normalizer) filterRules(skipRules []string) {
	var filteredRules []base.TransformRule

	for _, rule := range n.rules {
		shouldSkip := false
		for _, skipRule := range skipRules {
			if strings.Contains(rule.Description(), skipRule) {
				shouldSkip = true
				break
			}
		}

		if !shouldSkip {
			filteredRules = append(filteredRules, rule)
		}
	}

	n.rules = filteredRules
}

// validateResult valida que el contenido normalizado sea válido
func (n *Normalizer) validateResult(content string) []string {
	var errors []string

	// Validación básica: solo requerir frontmatter si parece ser un documento completo
	// Si el contenido empieza con ## o tiene headers, probablemente es solo el body
	contentTrimmed := strings.TrimSpace(content)

	// Si empieza con header markdown (## ), es probable que sea solo el cuerpo
	// En este caso, no requerimos frontmatter
	if !strings.HasPrefix(contentTrimmed, "##") && !strings.HasPrefix(contentTrimmed, "#") {
		if !strings.HasPrefix(contentTrimmed, "---") {
			errors = append(errors, "Contenido normalizado no tiene frontmatter válido")
		}
	}

	// Validación: debe tener al menos un slide
	if !strings.Contains(content, "#") {
		errors = append(errors, "Contenido normalizado no tiene slides (headers)")
	}
	// Validación: frontmatter debe tener slides
	lines := strings.Split(content, "\n")
	inFrontmatter := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if inFrontmatter {
				break // Final del frontmatter
			}
			inFrontmatter = true
			continue
		}
	}
	// Nota: No validamos el campo 'mode' aquí porque el parser
	// tiene un fallback robusto que asigna 'auto' cuando falta
	// Esta validación prematura causaba errores falsos

	return errors
}

// logDetectionResults registra los resultados de la detección
func (n *Normalizer) logDetectionResults(result DetectionResult) {
	if result.Detected {
		n.logger.Info("NORMALIZE", "Contenido AI detectado (score: %.2f, %d patrones)", result.Score, len(result.Patterns))
	}
}

// logNormalizationSummary registra un resumen de la normalización
func (n *Normalizer) logNormalizationSummary(report NormalizationReport) {
	if report.WasModified {
		sizeDiff := report.NormalizedSize - report.OriginalSize
		n.logger.Info("NORMALIZE", "Normalización completada → %+d bytes (%d reglas, %v)",
			sizeDiff, len(report.Applied), report.ProcessingTime.Truncate(time.Millisecond))

		if len(report.Errors) > 0 {
			n.logger.Warn("%d errores durante normalización", len(report.Errors))
		}
	}
}

// GetDefaultConfig retorna la configuración predeterminada
func GetDefaultConfig() Config {
	return Config{
		EnableDetection:  true,
		EnableTransforms: true,
		SkipRules:        make([]string, 0),
		BodyOnly:         false,
	}
}

// extractFrontmatter separa el frontmatter del contenido
func (n *Normalizer) extractFrontmatter(content string) (frontmatter, body string, hasFrontmatter bool) {
	// Normalizar las terminaciones de línea a solo \n
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	lines := strings.Split(content, "\n")

	// Buscar la primera línea que no esté vacía para verificar frontmatter
	startIndex := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			if trimmed == "---" {
				startIndex = i
			}
			break
		}
	}

	// Verificar si empieza con frontmatter
	if startIndex == -1 {
		return "", content, false
	}
	// Buscar el cierre del frontmatter
	endIndex := -1
	for i := startIndex + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		// No se encontró cierre, tratar como contenido normal
		return "", content, false
	}

	// Separar frontmatter y body
	// Incluir líneas vacías antes del frontmatter en el frontmatter
	frontmatterLines := lines[0 : endIndex+1]
	bodyLines := lines[endIndex+1:]

	frontmatter = strings.Join(frontmatterLines, "\n")
	body = strings.Join(bodyLines, "\n")

	return frontmatter, body, true
}

// shouldApplyCleanupRuleEarly determina si una regla debe aplicarse antes de separar frontmatter
func (n *Normalizer) shouldApplyCleanupRuleEarly(rule base.TransformRule) bool {
	// Reglas que necesitan aplicarse temprano para preservar la estructura
	return strings.Contains(rule.Description(), "backticks") ||
		strings.Contains(rule.Description(), "Escapa valores YAML")
}
