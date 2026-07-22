// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package structure

import (
	"strings"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// TitleSlideRule inyecta un slide de título cuando falta
type TitleSlideRule struct {
	analyzer *base.DocumentAnalyzer
}

// NewTitleSlideRule crea una nueva instancia de la regla
func NewTitleSlideRule() *TitleSlideRule {
	return &TitleSlideRule{
		analyzer: base.NewDocumentAnalyzer(),
	}
}

func (r *TitleSlideRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 {
		return content, nil
	}

	// Extraer título del frontmatter
	frontmatterTitle := r.analyzer.ExtractFrontmatterTitle(content)
	if frontmatterTitle == "" {
		return content, nil // Sin título en frontmatter, no podemos crear slide
	}

	// Extraer body content (después del frontmatter)
	bodyContent := r.extractBodyContent(content)
	bodyLines := strings.Split(bodyContent, "\n")

	// Buscar el primer header (slide)
	firstHeaderText, firstHeaderLine := r.analyzer.FindFirstHeader(bodyLines)
	if firstHeaderLine == -1 {
		return content, nil // No hay headers, no necesitamos slide de título
	}

	// Verificar si el primer slide ya es exactamente el título del frontmatter
	if r.analyzer.TitlesAreIdentical(firstHeaderText, frontmatterTitle) {
		return content, nil // Ya tiene slide de título
	}

	// Crear slide de título
	titleSlide := r.createTitleSlide(frontmatterTitle)

	// Inyectar el slide de título antes del primer header
	newBodyLines := make([]string, 0, len(bodyLines)+5)

	// Agregar líneas antes del primer header
	newBodyLines = append(newBodyLines, bodyLines[:firstHeaderLine]...)

	// Agregar slide de título
	newBodyLines = append(newBodyLines, titleSlide...)
	newBodyLines = append(newBodyLines, "", "---", "")

	// Agregar el resto del contenido
	newBodyLines = append(newBodyLines, bodyLines[firstHeaderLine:]...)

	// Reconstruir contenido completo
	frontmatterPart := r.extractFrontmatterPart(content)
	newBodyContent := strings.Join(newBodyLines, "\n")

	return frontmatterPart + newBodyContent, nil
}

// extractBodyContent extrae el contenido después del frontmatter
func (r *TitleSlideRule) extractBodyContent(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return content
	}

	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			if i+1 < len(lines) {
				return strings.Join(lines[i+1:], "\n")
			}
			return ""
		}
	}
	return content
}

// extractFrontmatterPart extrae solo la parte del frontmatter
func (r *TitleSlideRule) extractFrontmatterPart(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return ""
	}

	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return strings.Join(lines[:i+1], "\n") + "\n"
		}
	}
	return ""
}

// createTitleSlide crea las líneas del slide de título
func (r *TitleSlideRule) createTitleSlide(title string) []string {
	return []string{
		"# " + title,
		"",
	}
}

func (r *TitleSlideRule) Description() string {
	return "Inyecta slide de título cuando el contenido va directo al tema sin presentación"
}

func (r *TitleSlideRule) Priority() int {
	return 3 // Después de frontmatter cleanup pero antes de otras transformaciones de contenido
}

func (r *TitleSlideRule) Category() base.RuleCategory {
	return base.CategoryStructure
}
