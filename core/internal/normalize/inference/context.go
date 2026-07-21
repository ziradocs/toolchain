// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package inference

import (
	"regexp"
	"strings"
)

// DocumentContext contiene información contextual sobre todo el documento
type DocumentContext struct {
	Title      string            // Título principal del documento
	Theme      string            // Tema detectado (técnico, empresarial, educativo, etc.)
	SlideCount int               // Número total de slides
	Keywords   []string          // Keywords principales encontradas
	Sections   []SectionContext  // Contexto de cada sección
	Metadata   map[string]string // Metadatos adicionales
}

// SectionContext contiene contexto de una sección específica
type SectionContext struct {
	Title       string   // Título de la sección
	SlideIndex  int      // Índice del slide
	Content     string   // Contenido completo
	Keywords    []string // Keywords de esta sección
	ElementHint string   // Sugerencia de tipo de elemento dominante
}

// ContextAnalyzer analiza el contexto completo del documento
type ContextAnalyzer struct {
	// Patrones regex para análisis
	technicalPatterns   *regexp.Regexp
	businessPatterns    *regexp.Regexp
	educationalPatterns *regexp.Regexp
	dataPatterns        *regexp.Regexp
}

// NewContextAnalyzer crea un nuevo analizador de contexto
func NewContextAnalyzer() *ContextAnalyzer {
	return &ContextAnalyzer{
		technicalPatterns:   regexp.MustCompile(`(?i)(código|programming|development|API|framework|database|algoritmo|función|variable|class|method)`),
		businessPatterns:    regexp.MustCompile(`(?i)(business|negocio|ventas|marketing|ROI|revenue|profit|strategy|market|customer|cliente)`),
		educationalPatterns: regexp.MustCompile(`(?i)(curso|lesson|aprend|estud|exam|test|homework|tarea|universidad|college|school)`),
		dataPatterns:        regexp.MustCompile(`(?i)(datos|data|estadística|análisis|gráfico|chart|tabla|métrica|KPI|dashboard|report)`),
	}
}

// AnalyzeDocument analiza el contexto completo del documento
func (ca *ContextAnalyzer) AnalyzeDocument(content string) DocumentContext {
	lines := strings.Split(content, "\n")

	context := DocumentContext{
		Keywords: make([]string, 0),
		Sections: make([]SectionContext, 0),
		Metadata: make(map[string]string),
	}

	// 1. Extraer metadatos del frontmatter
	context = ca.extractFrontmatterContext(lines, context)

	// 2. Analizar estructura de slides
	context = ca.analyzeSlidesStructure(lines, context)

	// 3. Detectar tema principal
	context.Theme = ca.detectMainTheme(content)

	// 4. Extraer keywords principales
	context.Keywords = ca.extractMainKeywords(content)

	return context
}

// extractFrontmatterContext extrae contexto del frontmatter
func (ca *ContextAnalyzer) extractFrontmatterContext(lines []string, context DocumentContext) DocumentContext {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return context
	}

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}

		// Extraer título
		if strings.HasPrefix(line, "title:") {
			context.Title = ca.extractValue(line)
		}

		// Extraer metadatos adicionales
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				context.Metadata[key] = value
			}
		}
	}

	return context
}

// analyzeSlidesStructure analiza la estructura de slides
func (ca *ContextAnalyzer) analyzeSlidesStructure(lines []string, context DocumentContext) DocumentContext {
	inFrontmatter := false
	skipFrontmatter := false
	slideIndex := 0

	var currentSection *SectionContext
	var contentBuilder strings.Builder

	for _, line := range lines {
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

		// Detectar nuevo slide (header)
		if strings.HasPrefix(trimmed, "#") {
			// Guardar sección anterior si existe
			if currentSection != nil {
				currentSection.Content = contentBuilder.String()
				currentSection.Keywords = ca.extractSectionKeywords(currentSection.Content)
				currentSection.ElementHint = ca.suggestDominantElement(currentSection.Content)
				context.Sections = append(context.Sections, *currentSection)
			}

			// Crear nueva sección
			title := ca.extractHeaderTitle(trimmed)
			currentSection = &SectionContext{
				Title:      title,
				SlideIndex: slideIndex,
				Content:    "",
				Keywords:   make([]string, 0),
			}
			slideIndex++
			contentBuilder.Reset()
		}

		// Acumular contenido de la sección actual
		if currentSection != nil {
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		}
	}

	// Guardar última sección
	if currentSection != nil {
		currentSection.Content = contentBuilder.String()
		currentSection.Keywords = ca.extractSectionKeywords(currentSection.Content)
		currentSection.ElementHint = ca.suggestDominantElement(currentSection.Content)
		context.Sections = append(context.Sections, *currentSection)
	}

	context.SlideCount = slideIndex
	return context
}

// detectMainTheme detecta el tema principal del documento
func (ca *ContextAnalyzer) detectMainTheme(content string) string {
	content = strings.ToLower(content)

	scores := map[string]int{
		"technical":   len(ca.technicalPatterns.FindAllString(content, -1)),
		"business":    len(ca.businessPatterns.FindAllString(content, -1)),
		"educational": len(ca.educationalPatterns.FindAllString(content, -1)),
		"data":        len(ca.dataPatterns.FindAllString(content, -1)),
	}

	maxScore := 0
	theme := "general"
	for t, score := range scores {
		if score > maxScore {
			maxScore = score
			theme = t
		}
	}

	return theme
}

// extractMainKeywords extrae las keywords principales del documento
func (ca *ContextAnalyzer) extractMainKeywords(content string) []string {
	// Lista de keywords relevantes a buscar
	keywordPatterns := []string{
		"python", "javascript", "go", "java", "programming",
		"business", "marketing", "sales", "strategy",
		"data", "análisis", "gráfico", "estadística",
		"education", "curso", "learning", "tutorial",
		"presentation", "slide", "demo", "pitch",
	}

	var found []string
	content = strings.ToLower(content)

	for _, keyword := range keywordPatterns {
		if strings.Contains(content, keyword) {
			found = append(found, keyword)
		}
	}

	return found
}

// extractSectionKeywords extrae keywords específicas de una sección
func (ca *ContextAnalyzer) extractSectionKeywords(content string) []string {
	content = strings.ToLower(content)

	// Patrones específicos para elementos visuales
	visualKeywords := []string{
		"gráfico", "chart", "tabla", "table", "diagrama", "diagram",
		"imagen", "image", "foto", "picture", "video",
		"lista", "list", "puntos", "bullets",
		"código", "code", "ejemplo", "example",
	}

	var found []string
	for _, keyword := range visualKeywords {
		if strings.Contains(content, keyword) {
			found = append(found, keyword)
		}
	}

	return found
}

// suggestDominantElement sugiere el tipo de elemento dominante en una sección
func (ca *ContextAnalyzer) suggestDominantElement(content string) string {
	content = strings.ToLower(content)

	// Patrones para diferentes tipos de elementos
	patterns := map[string]*regexp.Regexp{
		"chart":   regexp.MustCompile(`(?i)(gráfico|chart|graph|estadística|datos|porcentaje|%|comparar|tendencia)`),
		"table":   regexp.MustCompile(`(?i)(tabla|table|comparación|vs|versus|\||filas|columnas)`),
		"image":   regexp.MustCompile(`(?i)(imagen|image|foto|picture|visual|!\[)`),
		"code":    regexp.MustCompile(`(?i)(código|code|function|var|class|def|` + "```" + `)`),
		"list":    regexp.MustCompile(`(?i)(lista|list|puntos|bullets|pasos|steps|\d+\.|-\s)`),
		"diagram": regexp.MustCompile(`(?i)(diagrama|diagram|flujo|flow|proceso|process|esquema)`),
	}

	maxMatches := 0
	dominantType := "text"

	for elementType, pattern := range patterns {
		matches := pattern.FindAllString(content, -1)
		if len(matches) > maxMatches {
			maxMatches = len(matches)
			dominantType = elementType
		}
	}

	return dominantType
}

// extractValue extrae el valor de una línea key: value
func (ca *ContextAnalyzer) extractValue(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) == 2 {
		value := strings.TrimSpace(parts[1])
		// Remover comillas si las tiene
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}
		return value
	}
	return ""
}

// extractHeaderTitle extrae el título de un header
func (ca *ContextAnalyzer) extractHeaderTitle(header string) string {
	// Remover # al inicio
	title := header
	for strings.HasPrefix(title, "#") {
		title = title[1:]
	}
	return strings.TrimSpace(title)
}

// GetSectionByIndex retorna una sección por su índice
func (dc *DocumentContext) GetSectionByIndex(index int) *SectionContext {
	if index >= 0 && index < len(dc.Sections) {
		return &dc.Sections[index]
	}
	return nil
}

// GetSectionsByKeyword retorna secciones que contienen una keyword específica
func (dc *DocumentContext) GetSectionsByKeyword(keyword string) []SectionContext {
	var matching []SectionContext
	keyword = strings.ToLower(keyword)

	for _, section := range dc.Sections {
		for _, sectionKeyword := range section.Keywords {
			if strings.Contains(strings.ToLower(sectionKeyword), keyword) {
				matching = append(matching, section)
				break
			}
		}
	}

	return matching
}

// HasTheme verifica si el documento tiene un tema específico
func (dc *DocumentContext) HasTheme(theme string) bool {
	return strings.EqualFold(dc.Theme, theme)
}

// GetMetadata retorna un metadato específico
func (dc *DocumentContext) GetMetadata(key string) (string, bool) {
	value, exists := dc.Metadata[key]
	return value, exists
}
