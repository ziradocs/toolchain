// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package structure

import (
	"strings"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// MarkdownSlideStructureRule corrige documentos markdown con un # y múltiples ##
// Patrón problemático: Un solo título principal (#) seguido de múltiples subtítulos (##)
// que deberían ser slides independientes
type MarkdownSlideStructureRule struct {
	analyzer *base.DocumentAnalyzer
}

// NewMarkdownSlideStructureRule crea una nueva instancia de la regla
func NewMarkdownSlideStructureRule() *MarkdownSlideStructureRule {
	return &MarkdownSlideStructureRule{
		analyzer: base.NewDocumentAnalyzer(),
	}
}

func (r *MarkdownSlideStructureRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")

	// NO aplicar si es DocLang (modo flex) - en DocLang, ## son subsecciones bajo #, no slides separados
	isDocLang := r.isDocLangDocument(lines)

	if isDocLang {
		return content, nil
	}

	// Si el documento ya tiene separadores de slides, no aplicar la regla
	if r.hasExistingSeparators(lines) {
		return content, nil
	}

	// Detectar si el documento tiene el patrón problemático
	if !r.hasProblematicPattern(lines) {
		return content, nil
	}

	startLine := r.analyzer.SkipFrontmatter(lines)
	h2Indices := []int{}

	// Buscar todas las secciones ##
	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "## ") {
			h2Indices = append(h2Indices, i)
		}
	}
	// Insertar separadores antes de cada ## (excepto el primero si está justo después del #)
	if len(h2Indices) >= 2 {
		newLines := make([]string, 0, len(lines)+len(h2Indices))
		lastIndex := 0

		// Buscar si hay un # principal antes del primer ##
		firstH2NeedsSeparator := true
		for j := h2Indices[0] - 1; j >= startLine; j-- {
			if strings.HasPrefix(strings.TrimSpace(lines[j]), "# ") {
				firstH2NeedsSeparator = false
				break
			}
		}

		for i, h2Index := range h2Indices {
			// Para el primer ##, solo agregar separador si no hay # antes
			if i == 0 && !firstH2NeedsSeparator {
				continue
			} // Agregar contenido hasta este ##
			contentLines := lines[lastIndex:h2Index]
			newLines = append(newLines, contentLines...)

			// Verificar si la última línea del contenido está vacía
			lastContentLine := ""
			if len(contentLines) > 0 {
				lastContentLine = strings.TrimSpace(contentLines[len(contentLines)-1])
			}

			// Agregar separador con líneas vacías apropiadas
			if lastContentLine == "" {
				// Si ya hay línea vacía, solo agregar separador y línea vacía después
				newLines = append(newLines, "---", "")
			} else {
				// Si no hay línea vacía, agregar línea vacía, separador y línea vacía
				newLines = append(newLines, "", "---", "")
			}

			lastIndex = h2Index
		}

		// Agregar el resto del contenido
		newLines = append(newLines, lines[lastIndex:]...)

		return strings.Join(newLines, "\n"), nil
	}
	return content, nil
}

// hasExistingSeparators verifica si el documento ya tiene separadores de slides
func (r *MarkdownSlideStructureRule) hasExistingSeparators(lines []string) bool {
	startLine := r.analyzer.SkipFrontmatter(lines)

	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" || line == "***" || line == "___" {
			return true
		}
	}
	return false
}

// hasProblematicPattern detecta si el documento tiene el patrón problemático:
// Múltiples secciones ## que deberían ser slides independientes después de un #
func (r *MarkdownSlideStructureRule) hasProblematicPattern(lines []string) bool {
	startLine := r.analyzer.SkipFrontmatter(lines)

	titleIndex := -1
	h2Indices := []int{}

	// Buscar el # principal y todos los ##
	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "# ") && titleIndex == -1 {
			titleIndex = i
		} else if strings.HasPrefix(line, "## ") {
			h2Indices = append(h2Indices, i)
		}
	}

	// Si no hay título principal o menos de 2 subtítulos, no hay patrón problemático
	if titleIndex == -1 || len(h2Indices) < 2 {
		return false
	}

	// Verificar si ya hay separadores entre las secciones ##
	hasSeparators := false
	for i := 0; i < len(h2Indices)-1; i++ {
		// Buscar separador entre h2Indices[i] y h2Indices[i+1]
		for j := h2Indices[i] + 1; j < h2Indices[i+1]; j++ {
			line := strings.TrimSpace(lines[j])
			if line == "---" || line == "***" || line == "___" {
				hasSeparators = true
				break
			}
		}
		if hasSeparators {
			break
		}
	}

	// Patrón problemático: múltiples ## que parecen ser secciones independientes sin separadores
	// Solo aplicar si hay contenido sustancial entre las secciones ##
	if !hasSeparators && len(h2Indices) >= 2 {
		// Verificar que cada sección ## tenga contenido sustancial (más que solo texto descriptivo)
		hasSubstantialSections := false
		for i := 0; i < len(h2Indices); i++ {
			contentLines := 0
			nextH2 := len(lines)
			if i < len(h2Indices)-1 {
				nextH2 = h2Indices[i+1]
			}

			// Contar líneas de contenido real (no vacías)
			for j := h2Indices[i] + 1; j < nextH2; j++ {
				if strings.TrimSpace(lines[j]) != "" {
					contentLines++
				}
			}

			// Si una sección tiene más de 3 líneas de contenido, podría ser una sección independiente
			if contentLines > 3 {
				hasSubstantialSections = true
				break
			}
		}

		return hasSubstantialSections
	}

	return false
}

func (r *MarkdownSlideStructureRule) Description() string {
	return "Corrige estructura markdown con un # y múltiples ## convirtiéndolos en slides independientes"
}

func (r *MarkdownSlideStructureRule) Priority() int {
	return 2 // Antes que HeadersRule pero después de frontmatter
}

func (r *MarkdownSlideStructureRule) Category() base.RuleCategory {
	return base.CategoryStructure
}

// isDocLangDocument verifica si el documento es DocLang (modo flex con estructura jerárquica)
// En DocLang, ## son subsecciones bajo #, NO slides separados
func (r *MarkdownSlideStructureRule) isDocLangDocument(lines []string) bool {
	// Estrategia 1: Si tiene frontmatter con estructura # → ##, PODRÍA ser DocLang
	// pero solo si no tiene múltiples ## con contenido sustancial (que sería SlideLang)
	hasFrontmatter := len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"

	// Si tiene frontmatter, buscar estructura # → ##
	if hasFrontmatter {
		startLine := r.analyzer.SkipFrontmatter(lines)
		hasH1 := false
		h1Index := -1

		// Buscar primer #
		for i := startLine; i < len(lines) && i < startLine+50; i++ {
			line := strings.TrimSpace(lines[i])
			if strings.HasPrefix(line, "# ") {
				hasH1 = true
				h1Index = i
				break
			}
		}

		// Si encontramos #, buscar ## cerca
		if hasH1 && h1Index >= 0 {
			h2Count := 0
			for i := h1Index + 1; i < len(lines) && i < h1Index+20; i++ {
				line := strings.TrimSpace(lines[i])
				if strings.HasPrefix(line, "## ") {
					h2Count++
				}
			}

			// Si tiene frontmatter + # pero solo 1 ##, es DocLang
			// Si tiene múltiples ##, check si tienen contenido sustancial (sería SlideLang presentation)
			if h2Count == 1 {
				return true
			}
		}
	}

	// Estrategia 2: Detectar patrón jerárquico DocLang sin frontmatter
	// DocLang sin frontmatter tiene múltiples ## bajo un #, con elementos complejos
	startLine := r.analyzer.SkipFrontmatter(lines)

	h1Count := 0
	h2Count := 0
	h3Count := 0
	hasComplexElements := false

	// Analizar primeras 100 líneas
	for i := startLine; i < len(lines) && i < startLine+100; i++ {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "# ") {
			h1Count++
		} else if strings.HasPrefix(line, "## ") {
			h2Count++
		} else if strings.HasPrefix(line, "### ") {
			h3Count++
		}

		// Detectar elementos complejos típicos de DocLang
		if strings.Contains(line, "<<map") ||
			strings.Contains(line, "<<chart") ||
			strings.Contains(line, "<<mermaid") ||
			strings.Contains(line, "```") {
			hasComplexElements = true
		}
	}

	// Heurística: Si tiene estructura jerárquica profunda con elementos complejos,
	// es DocLang (no queremos insertar separadores ---)
	if h1Count >= 1 && h2Count >= 3 && hasComplexElements {
		return true
	}

	// Si tiene múltiples H1 con H2/H3 debajo y elementos complejos, también es DocLang
	if h1Count >= 2 && (h2Count >= h1Count || h3Count >= 2) && hasComplexElements {
		return true
	}

	return false
}
