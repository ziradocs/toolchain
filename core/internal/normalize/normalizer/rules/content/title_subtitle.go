// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"strings"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// TitleSubtitleRule corrige la estructura problemática de título-subtítulo
type TitleSubtitleRule struct {
	analyzer *base.DocumentAnalyzer
}

// NewTitleSubtitleRule crea una nueva instancia de la regla
func NewTitleSubtitleRule() *TitleSubtitleRule {
	return &TitleSubtitleRule{
		analyzer: base.NewDocumentAnalyzer(),
	}
}

func (r *TitleSubtitleRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	modified := false

	// NO aplicar si es DocLang (archivos con frontmatter + estructura #/##)
	// En DocLang, # → ## es la estructura jerárquica correcta, NO un problema
	if r.isDocLangDocument(lines) {
		return content, nil
	}

	// Buscar el PRIMER título # en todo el documento (excluyendo frontmatter)
	firstTitleFound := false
	startLine := r.analyzer.SkipFrontmatter(lines)

	for i := startLine; i < len(lines); i++ {
		currentLine := strings.TrimSpace(lines[i])

		// Detectar títulos con #
		if strings.HasPrefix(currentLine, "# ") && !firstTitleFound {
			firstTitleFound = true

			// Verificar si la siguiente línea no vacía es un ## (problema común)
			nextContentLine := r.analyzer.FindNextNonEmptyLine(lines, i)
			if nextContentLine != -1 && strings.HasPrefix(strings.TrimSpace(lines[nextContentLine]), "## ") {
				// Problema detectado: # seguido inmediatamente por ##
				// Convertir el ## en contenido normal o mantener solo el #
				lines[nextContentLine] = strings.TrimSpace(lines[nextContentLine][3:]) // Quitar "## "
				modified = true
			}
		}

		// Detectar subtítulos (###) - convertir a ## SOLO inmediatamente después de un #
		if strings.HasPrefix(currentLine, "### ") && firstTitleFound {
			// Solo convertir ### a ## si está inmediatamente después del primer título #
			prevContentLine := r.analyzer.FindPrevNonEmptyLine(lines, i)
			if prevContentLine != -1 && strings.HasPrefix(strings.TrimSpace(lines[prevContentLine]), "# ") {
				lines[i] = "## " + strings.TrimSpace(currentLine[4:])
				modified = true
			}
			// Si no está después de un #, dejarlo como ### (header H3 legítimo)
		}
	}

	if modified {
		return strings.Join(lines, "\n"), nil
	}
	return content, nil
}

func (r *TitleSubtitleRule) Description() string {
	return "Corrige estructura problemática de título (# ) seguido inmediatamente por subtítulo (## )"
}

func (r *TitleSubtitleRule) Priority() int {
	return 2 // Alta prioridad, afecta estructura básica
}

func (r *TitleSubtitleRule) Category() base.RuleCategory {
	return base.CategoryContent
}

// isDocLangDocument verifica si el documento es DocLang (estructura jerárquica)
// En DocLang, la estructura # → ## → ### es correcta y NO debe modificarse
func (r *TitleSubtitleRule) isDocLangDocument(lines []string) bool {
	// Estrategia 1: Si tiene frontmatter, es muy probable que sea DocLang
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		return true
	}

	// Estrategia 2: Detectar patrón jerárquico de DocLang sin frontmatter
	// DocLang tiende a tener múltiples secciones ## bajo un # principal
	// y contiene elementos complejos como mapas, charts, código, etc.

	startLine := r.analyzer.SkipFrontmatter(lines)

	h1Count := 0
	h2Count := 0
	h3Count := 0
	hasComplexElements := false

	// Analizar estructura del documento
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

	// Heurística: Si tiene 1 H1 con múltiples H2/H3 Y elementos complejos,
	// probablemente es DocLang con estructura jerárquica
	if h1Count >= 1 && (h2Count >= 2 || h3Count >= 2) && hasComplexElements {
		return true
	}

	// Si tiene varios H1 con estructura jerárquica debajo, también es DocLang
	if h1Count >= 2 && h2Count >= h1Count && hasComplexElements {
		return true
	}

	return false
}
