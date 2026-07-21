// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalizer

import (
	"fmt"
	"strings"
)

// ValidationResult contiene el resultado de la validación
type ValidationResult struct {
	IsValid  bool     // True si el contenido es válido
	Errors   []string // Lista de errores encontrados
	Warnings []string // Lista de warnings
	Score    float64  // Puntuación de calidad (0.0-1.0)
}

// Validator se encarga de validar que el contenido normalizado sea válido para el FlexParser
type Validator struct {
	verbose bool
}

// NewValidator crea una nueva instancia del validador
func NewValidator(verbose bool) *Validator {
	return &Validator{
		verbose: verbose,
	}
}

// Validate verifica que el contenido normalizado sea válido
func (v *Validator) Validate(content string) ValidationResult {
	result := ValidationResult{
		IsValid:  true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
		Score:    1.0,
	}

	// 1. Validar estructura básica
	v.validateBasicStructure(content, &result)

	// 2. Validar frontmatter
	v.validateFrontmatter(content, &result)

	// 3. Validar slides
	v.validateSlides(content, &result)

	// 4. Validar elementos específicos
	v.validateElements(content, &result)

	// 5. Calcular puntuación final
	v.calculateScore(&result)

	// Determinar si es válido
	result.IsValid = len(result.Errors) == 0

	if v.verbose {
		v.logValidationResult(result)
	}

	return result
}

// validateBasicStructure valida la estructura básica del documento
func (v *Validator) validateBasicStructure(content string, result *ValidationResult) {
	if strings.TrimSpace(content) == "" {
		result.Errors = append(result.Errors, "Contenido vacío")
		return
	}

	lines := strings.Split(content, "\n")

	// Debe tener frontmatter
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		result.Errors = append(result.Errors, "Documento debe comenzar con frontmatter (---)")
		return
	}

	// Debe tener al menos un header (slide)
	hasHeader := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			hasHeader = true
			break
		}
	}

	if !hasHeader {
		result.Errors = append(result.Errors, "Documento debe tener al menos un slide (header con #)")
	}
}

// validateFrontmatter valida el frontmatter
func (v *Validator) validateFrontmatter(content string, result *ValidationResult) {
	lines := strings.Split(content, "\n")

	if len(lines) == 0 {
		return
	}
	// Encontrar frontmatter
	if strings.TrimSpace(lines[0]) != "---" {
		return // Ya se reportó error en validateBasicStructure
	}

	frontmatterLines := make([]string, 0)
	endFound := false

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			endFound = true
			break
		}
		frontmatterLines = append(frontmatterLines, lines[i])
	}

	if !endFound {
		result.Errors = append(result.Errors, "Frontmatter no está cerrado correctamente (falta --- final)")
		return
	}

	// Validar campos requeridos
	frontmatterContent := strings.Join(frontmatterLines, "\n")

	requiredFields := []string{"mode", "title"}
	for _, field := range requiredFields {
		if !strings.Contains(frontmatterContent, field+":") {
			result.Errors = append(result.Errors, fmt.Sprintf("Frontmatter debe incluir campo '%s'", field))
		}
	}

	// Validar que mode sea válido
	if strings.Contains(frontmatterContent, "mode:") {
		if !strings.Contains(frontmatterContent, "mode: flex") &&
			!strings.Contains(frontmatterContent, "mode: strict") {
			result.Warnings = append(result.Warnings, "Campo 'mode' debe ser 'flex' o 'strict'")
		}
	}

	// Verificar si hay campos de AI processing
	if strings.Contains(frontmatterContent, "ai_generated: true") ||
		strings.Contains(frontmatterContent, "ai_processed: true") {
		result.Warnings = append(result.Warnings, "Contenido marcado como generado/procesado por AI")
	}
}

// validateSlides valida la estructura de slides
func (v *Validator) validateSlides(content string, result *ValidationResult) {
	lines := strings.Split(content, "\n")

	slideCount := 0
	inFrontmatter := false
	skipFrontmatter := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Saltar frontmatter
		if !skipFrontmatter {
			if trimmed == "---" {
				if inFrontmatter {
					skipFrontmatter = true
					inFrontmatter = false
				} else {
					inFrontmatter = true
				}
				continue
			}
			if inFrontmatter {
				continue
			}
		}

		// Contar slides
		if strings.HasPrefix(trimmed, "#") {
			slideCount++

			// Validar que no sea solo #
			if trimmed == "#" {
				result.Errors = append(result.Errors, fmt.Sprintf("Header vacío en línea %d", i+1))
			}

			// Validar estructura de headers
			v.validateHeaderStructure(trimmed, i+1, result)
		}
	}

	if slideCount == 0 {
		result.Errors = append(result.Errors, "No se encontraron slides (headers)")
	} else if slideCount < 2 {
		result.Warnings = append(result.Warnings, "Presentación tiene menos de 2 slides")
	}
}

// validateHeaderStructure valida la estructura de headers
func (v *Validator) validateHeaderStructure(header string, lineNum int, result *ValidationResult) {
	// Contar niveles de #
	level := 0
	for _, char := range header {
		if char == '#' {
			level++
		} else {
			break
		}
	}

	// Validar niveles apropiados (1-6)
	if level > 6 {
		result.Errors = append(result.Errors, fmt.Sprintf("Header en línea %d tiene demasiados niveles (máximo 6)", lineNum))
	}

	// Validar que hay espacio después de #
	if level > 0 && len(header) > level {
		if header[level] != ' ' {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Header en línea %d debe tener espacio después de #", lineNum))
		}
	}

	// Validar que no esté vacío
	content := strings.TrimSpace(header[level:])
	if content == "" {
		result.Errors = append(result.Errors, fmt.Sprintf("Header en línea %d está vacío", lineNum))
	}
}

// validateElements valida elementos específicos normalizados
func (v *Validator) validateElements(content string, result *ValidationResult) {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Validar bloques chart
		if strings.HasPrefix(trimmed, "```chart") {
			v.validateChartBlock(lines, i, result)
		}

		// Validar imágenes
		if strings.Contains(trimmed, "![") && strings.Contains(trimmed, "](") {
			v.validateImageElement(trimmed, i+1, result)
		}

		// Validar tablas
		if strings.Contains(trimmed, "|") && strings.Contains(trimmed, "-") {
			v.validateTableElement(lines, i, result)
		}
	}
}

// validateChartBlock valida un bloque chart
func (v *Validator) validateChartBlock(lines []string, startIndex int, result *ValidationResult) {
	lineNum := startIndex + 1
	endFound := false
	hasType := false
	hasTitle := false

	for i := startIndex + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if line == "```" {
			endFound = true
			break
		}

		if strings.HasPrefix(line, "type:") {
			hasType = true
		}

		if strings.HasPrefix(line, "title:") {
			hasTitle = true
		}
	}

	if !endFound {
		result.Errors = append(result.Errors, fmt.Sprintf("Bloque chart en línea %d no está cerrado", lineNum))
	}

	if !hasType {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Bloque chart en línea %d debe incluir 'type'", lineNum))
	}

	if !hasTitle {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Bloque chart en línea %d debe incluir 'title'", lineNum))
	}
}

// validateImageElement valida un elemento imagen
func (v *Validator) validateImageElement(line string, lineNum int, result *ValidationResult) {
	// Buscar patrón ![alt](path)
	if strings.Contains(line, "![") && strings.Contains(line, "](") {
		// Verificar que la ruta no sea placeholder genérico
		if strings.Contains(line, "placeholder") || strings.Contains(line, "example") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Imagen en línea %d usa ruta placeholder", lineNum))
		}

		// Verificar estructura de assets/
		if !strings.Contains(line, "assets/") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Imagen en línea %d no usa estructura assets/", lineNum))
		}
	}
}

// validateTableElement valida un elemento tabla
func (v *Validator) validateTableElement(lines []string, startIndex int, result *ValidationResult) {
	lineNum := startIndex + 1
	line := strings.TrimSpace(lines[startIndex])

	// Si es una línea de separación (|---|---|), verificar que tenga header
	if strings.Contains(line, "---") {
		if startIndex > 0 {
			previousLine := strings.TrimSpace(lines[startIndex-1])
			if !strings.Contains(previousLine, "|") {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Tabla en línea %d no tiene header válido", lineNum))
			}
		}
	}

	// Contar columnas (número de |)
	pipeCount := strings.Count(line, "|")
	if pipeCount < 2 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Tabla en línea %d puede no tener suficientes columnas", lineNum))
	}
}

// calculateScore calcula la puntuación de calidad
func (v *Validator) calculateScore(result *ValidationResult) {
	score := 1.0

	// Restar por errores (más severo)
	score -= float64(len(result.Errors)) * 0.2

	// Restar por warnings (menos severo)
	score -= float64(len(result.Warnings)) * 0.05

	// Asegurar que no sea negativa
	if score < 0.0 {
		score = 0.0
	}

	result.Score = score
}

// logValidationResult registra el resultado de la validación
func (v *Validator) logValidationResult(result ValidationResult) {
	fmt.Printf("=== Resultado de Validación ===\n")
	fmt.Printf("¿Es válido?: %v\n", result.IsValid)
	fmt.Printf("Puntuación: %.2f\n", result.Score)

	if len(result.Errors) > 0 {
		fmt.Printf("Errores: %d\n", len(result.Errors))
		for _, err := range result.Errors {
			fmt.Printf("  ✗ %s\n", err)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("Warnings: %d\n", len(result.Warnings))
		for _, warning := range result.Warnings {
			fmt.Printf("  ⚠ %s\n", warning)
		}
	}

	if len(result.Errors) == 0 && len(result.Warnings) == 0 {
		fmt.Println("✓ Sin problemas detectados")
	}
	fmt.Println()
}
