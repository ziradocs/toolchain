// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalizer

import (
	"regexp"
	"strings"
)

// NormalizationPattern representa un patrón específico detectado en contenido generado por AI
type NormalizationPattern struct {
	Type        string  // Tipo de patrón (gráfico, imagen, estructura)
	Description string  // Descripción del patrón encontrado
	Line        int     // Línea donde se encontró
	Confidence  float64 // Confianza en la detección (0.0-1.0)
}

// DetectionResult contiene el resultado de la detección de patrones AI
type DetectionResult struct {
	Detected bool                   // True si se detectaron patrones AI
	Patterns []NormalizationPattern // Lista de patrones encontrados
	Score    float64                // Puntuación general de AI (0.0-1.0)
}

// Detector se encarga de identificar si el contenido fue generado por AI
type Detector struct {
	// Patrones regex compilados para mejor performance
	graphicPattern           *regexp.Regexp
	imagePattern             *regexp.Regexp
	placeholderPattern       *regexp.Regexp
	markdownBackticksPattern *regexp.Regexp
}

// NewDetector crea una nueva instancia del detector
func NewDetector() *Detector {
	return &Detector{
		// Patrón multiidioma para detectar placeholders de gráficos/charts de AI
		// Español: GRÁFICO, DIAGRAMA, TABLA
		// Inglés: CHART, GRAPH, DIAGRAM, TABLE
		// Francés: GRAPHIQUE, DIAGRAMME, TABLEAU
		// Alemán: DIAGRAMM, GRAFIK, TABELLE
		graphicPattern: regexp.MustCompile(`(?i)(GRÁFICO|CHART|GRAPH|DIAGRAMA|DIAGRAM|TABLA|TABLE|GRAPHIQUE|DIAGRAMME|TABLEAU|DIAGRAMM|GRAFIK|TABELLE):\s*(.+)`),

		// Patrón para imágenes con rutas genéricas
		imagePattern: regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+\.(jpg|jpeg|png|gif|svg))\)`),
		// Patrón para placeholders genéricos
		placeholderPattern: regexp.MustCompile(`(?i)(ruta/|path/|imagen|image|placeholder|ejemplo|sample|example)`),

		// Patrón para detectar backticks de markdown que envuelven frontmatter o todo el archivo (error típico de AI)
		markdownBackticksPattern: regexp.MustCompile(`(?s)^\s*` + "`" + `{3}(ya?ml|markdown)\s*\n`),
	}
}

// Detect analiza el contenido y detecta si fue generado por AI
func (d *Detector) Detect(content string) DetectionResult {
	lines := strings.Split(content, "\n")
	var patterns []NormalizationPattern
	totalScore := 0.0

	// 1. Detectar patrones de gráficos implícitos
	graphicPatterns := d.detectGraphicPatterns(lines)
	patterns = append(patterns, graphicPatterns...)
	// 2. Detectar imágenes con rutas placeholder
	imagePatterns := d.detectImagePlaceholders(lines)
	patterns = append(patterns, imagePatterns...)
	// 3. Detectar backticks de markdown envolviendo frontmatter
	markdownBackticksPattern := d.detectMarkdownBackticks(content)
	if markdownBackticksPattern.Type != "" {
		patterns = append(patterns, markdownBackticksPattern)
	}

	// 4. Detectar missing frontmatter (contenido sin frontmatter)
	frontmatterPattern := d.detectMissingFrontmatter(content)
	if frontmatterPattern.Type != "" {
		patterns = append(patterns, frontmatterPattern)
	}

	// 5. Detectar patrones de texto específicos de AI
	aiTextPatterns := d.detectGeneratedTextPatterns(lines)
	patterns = append(patterns, aiTextPatterns...)
	// 6. Detectar diagramas Mermaid mal formateados (típico de AI)
	mermaidPatterns := d.detectMalformedMermaidDiagrams(lines)
	patterns = append(patterns, mermaidPatterns...)

	// 7. Detectar gráficos mal formateados (típico de AI)
	chartPatterns := d.detectMalformedCharts(lines)
	patterns = append(patterns, chartPatterns...)

	// Calcular puntuación total basada en patrones encontrados
	for _, pattern := range patterns {
		totalScore += pattern.Confidence
	}

	// Normalizar score (máximo 1.0)
	if totalScore > 1.0 {
		totalScore = 1.0
	}

	// Considerar AI si encontramos patrones significativos
	detected := len(patterns) > 0 && totalScore > 0.3

	return DetectionResult{
		Detected: detected,
		Patterns: patterns,
		Score:    totalScore,
	}
}

// detectGraphicPatterns busca placeholders de gráficos generados por AI
func (d *Detector) detectGraphicPatterns(lines []string) []NormalizationPattern {
	var patterns []NormalizationPattern

	for i, line := range lines {
		// Excluir sintaxis válida de SlideLang: <<chart: type>>, <<mermaid>>, etc.
		if strings.Contains(line, "<<") && strings.Contains(line, ">>") {
			continue // Saltar líneas que contienen sintaxis válida de SlideLang
		}

		matches := d.graphicPattern.FindStringSubmatch(line)
		if len(matches) > 2 {
			// Estas son leyendas/placeholders de AI, no instrucciones reales para crear charts
			patterns = append(patterns, NormalizationPattern{
				Type:        "graphic_placeholder", // Cambiado de "graphic_implicit"
				Description: "Placeholder de gráfico de AI: " + matches[2],
				Line:        i + 1,
				Confidence:  0.8, // Alto porque es muy específico de AI
			})
		}
	}

	return patterns
}

// detectImagePlaceholders busca imágenes con rutas placeholder
func (d *Detector) detectImagePlaceholders(lines []string) []NormalizationPattern {
	var patterns []NormalizationPattern

	for i, line := range lines {
		matches := d.imagePattern.FindStringSubmatch(line)
		if len(matches) > 2 {
			imagePath := matches[2]
			if d.placeholderPattern.MatchString(imagePath) {
				patterns = append(patterns, NormalizationPattern{
					Type:        "image_placeholder",
					Description: "Imagen con ruta placeholder: " + imagePath,
					Line:        i + 1,
					Confidence:  0.7,
				})
			}
		}
	}

	return patterns
}

// detectMissingFrontmatter detecta si falta frontmatter
func (d *Detector) detectMissingFrontmatter(content string) NormalizationPattern {
	lines := strings.Split(content, "\n")

	// Si no comienza con ---, probablemente le falta frontmatter
	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "---" {
		// Verificar si tiene estructura de slide (empieza con #)
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if strings.HasPrefix(trimmed, "# ") {
					return NormalizationPattern{
						Type:        "missing_frontmatter",
						Description: "Contenido de slides sin frontmatter",
						Line:        1,
						Confidence:  0.6,
					}
				}
				break
			}
		}
	}

	return NormalizationPattern{}
}

// detectMarkdownBackticks detecta backticks de markdown que envuelven incorrectamente el frontmatter o todo el archivo
func (d *Detector) detectMarkdownBackticks(content string) NormalizationPattern {
	if d.markdownBackticksPattern.MatchString(content) {
		return NormalizationPattern{
			Type:        "markdown_backticks",
			Description: "Contenido envuelto incorrectamente con backticks de markdown (```yaml, ```markdown)",
			Line:        1,   // Usualmente está al inicio
			Confidence:  0.9, // Alta confianza, es un patrón muy específico de AI
		}
	}
	return NormalizationPattern{} // Patrón vacío si no se detecta
}

// detectGeneratedTextPatterns detecta patrones de texto típicos de AI
func (d *Detector) detectGeneratedTextPatterns(lines []string) []NormalizationPattern {
	var patterns []NormalizationPattern
	// Patrones de texto que indican generación por AI
	aiPhrases := []struct {
		pattern    string
		confidence float64
	}{
		{"Action Items", 0.5},
		{"Tips para", 0.4},
		{"Consejos para", 0.4},
		// {"Datos y estadísticas clave", 0.6}, // REMOVIDO: frase común en español
		{"Beneficios principales", 0.4},
		{"Características principales", 0.4},
		{"Resumen y Action Items", 0.7},
	}

	for i, line := range lines {
		for _, phrase := range aiPhrases {
			if strings.Contains(line, phrase.pattern) {
				patterns = append(patterns, NormalizationPattern{
					Type:        "ai_text_pattern",
					Description: "Frase típica de AI: " + phrase.pattern,
					Line:        i + 1,
					Confidence:  phrase.confidence,
				})
			}
		}
	}

	return patterns
}

// detectMalformedMermaidDiagrams busca diagramas Mermaid mal formateados (típico de AI)
func (d *Detector) detectMalformedMermaidDiagrams(lines []string) []NormalizationPattern {
	var patterns []NormalizationPattern

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detectar bloques <<mermaid>> seguidos de contenido sin indentación
		if trimmedLine == "<<mermaid>>" {
			// Verificar las siguientes líneas para contenido sin indentación
			for j := i + 1; j < len(lines) && j < i+10; j++ { // Revisar hasta 10 líneas siguientes
				nextLine := lines[j]
				trimmedNext := strings.TrimSpace(nextLine)

				// Si encontramos una línea vacía, continuar
				if trimmedNext == "" {
					continue
				}

				// Si encontramos el cierre del bloque, terminar
				if trimmedNext == "<</mermaid>>" || strings.HasPrefix(trimmedNext, "<<") {
					break
				}

				// Si encontramos contenido Mermaid sin indentación (típico de AI)
				if (strings.HasPrefix(trimmedNext, "flowchart") ||
					strings.HasPrefix(trimmedNext, "graph") ||
					strings.HasPrefix(trimmedNext, "gantt") ||
					strings.HasPrefix(trimmedNext, "sequenceDiagram") ||
					strings.Contains(trimmedNext, "-->") ||
					strings.Contains(trimmedNext, "dateFormat")) &&
					!strings.HasPrefix(nextLine, "  ") { // No tiene indentación de 2 espacios

					patterns = append(patterns, NormalizationPattern{
						Type:        "malformed_mermaid",
						Description: "Diagrama Mermaid sin indentación requerida (típico de AI)",
						Line:        j + 1,
						Confidence:  0.8,
					})
					break
				}
			}
		}
	}

	return patterns
}

// detectMalformedCharts detecta bloques de gráficos con datos mal formateados (típico de AI)
func (d *Detector) detectMalformedCharts(lines []string) []NormalizationPattern {
	var patterns []NormalizationPattern
	inChartBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detectar inicio de bloque de gráfico
		if strings.HasPrefix(trimmed, "<<chart:") {
			inChartBlock = true
			continue
		}
		// Si estamos en un bloque de gráfico
		if inChartBlock {
			// Detectar fin del bloque
			if trimmed == ">>" || strings.HasPrefix(trimmed, "---") ||
				strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "<<") {
				inChartBlock = false
				continue
			}

			// Detectar datos de gráfico mal formateados (sin indentación)
			// IMPORTANTE: usar 'line' (con espacios originales) no 'trimmed'
			if d.isChartDataWithoutIndentation(line) {
				patterns = append(patterns, NormalizationPattern{
					Type:        "malformed_chart",
					Description: "Datos de gráfico sin indentación apropiada (típico de AI)",
					Line:        i + 1,
					Confidence:  0.6, // Señal fuerte de AI
				})
			}
		}
	}

	return patterns
}

// isChartDataWithoutIndentation verifica si una línea es datos de gráfico sin indentación
func (d *Detector) isChartDataWithoutIndentation(line string) bool {
	// Si la línea está indentada, no es problemática
	if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
		return false
	}

	// Propiedades de gráfico que deberían estar indentadas
	chartProperties := []string{
		"title:", "labels:", "datasets:", "data:", "backgroundColor:",
		"borderColor:", "borderWidth:", "type:", "label:", "fill:",
		"tension:", "pointRadius:", "pointHoverRadius:",
	}

	for _, prop := range chartProperties {
		if strings.HasPrefix(line, prop) {
			return true // Es una propiedad de gráfico sin indentación
		}
	}

	return false
}

// GetPatternsByType filtra patrones por tipo específico
func (dr *DetectionResult) GetPatternsByType(patternType string) []NormalizationPattern {
	var filtered []NormalizationPattern
	for _, pattern := range dr.Patterns {
		if pattern.Type == patternType {
			filtered = append(filtered, pattern)
		}
	}
	return filtered
}

// HasPattern verifica si existe al menos un patrón del tipo especificado
func (dr *DetectionResult) HasPattern(patternType string) bool {
	for _, pattern := range dr.Patterns {
		if pattern.Type == patternType {
			return true
		}
	}
	return false
}
