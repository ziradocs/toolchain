// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/internal/normalize/normalizer/base"
)

// mermaidCodeBlockPattern se compila una sola vez a nivel de paquete en vez
// de en cada llamada a Apply (ver docs/SECURITY_AUDIT_2026-07.md, BA-9).
var mermaidCodeBlockPattern = regexp.MustCompile(`(?s)` + "`" + `{3}mermaid\s*\n(.*?)\n` + "`" + `{3}`)

// MermaidRule convierte bloques de código Mermaid de formato markdown (```mermaid) al formato SlideLang (<<mermaid>>)
type MermaidRule struct{}

// NewMermaidRule crea una nueva instancia de la regla
func NewMermaidRule() *MermaidRule {
	return &MermaidRule{}
}

func (r *MermaidRule) Apply(content string) (string, error) {
	// Buscar todas las coincidencias de bloques mermaid de markdown
	// (```mermaid\n[contenido]\n```)
	matches := mermaidCodeBlockPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	result := content

	// Procesar cada bloque mermaid encontrado (en orden inverso para mantener índices)
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		if len(match) >= 2 {
			fullMatch := match[0]      // Todo el bloque ```mermaid...```
			mermaidContent := match[1] // Solo el contenido interno

			// Limpiar y formatear el contenido mermaid
			cleanContent := r.cleanMermaidContent(mermaidContent)

			// Crear el bloque en formato SlideLang
			slideLangBlock := "<<mermaid>>"
			if strings.TrimSpace(cleanContent) != "" {
				slideLangBlock += "\n" + cleanContent
			}

			// Reemplazar en el contenido
			result = strings.Replace(result, fullMatch, slideLangBlock, 1)
		}
	}

	return result, nil
}

// cleanMermaidContent limpia y formatea correctamente el contenido mermaid
func (r *MermaidRule) cleanMermaidContent(content string) string {
	// Si está vacío o solo espacios, retornar vacío
	if strings.TrimSpace(content) == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	var cleanLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			// Cada línea con contenido debe tener exactamente 2 espacios de indentación
			cleanLines = append(cleanLines, "  "+trimmed)
		} else if len(cleanLines) > 0 {
			// Preservar líneas vacías que separan secciones del diagrama
			cleanLines = append(cleanLines, "")
		}
	}

	// Eliminar líneas vacías al final
	for len(cleanLines) > 0 && strings.TrimSpace(cleanLines[len(cleanLines)-1]) == "" {
		cleanLines = cleanLines[:len(cleanLines)-1]
	}

	return strings.Join(cleanLines, "\n")
}

func (r *MermaidRule) Description() string {
	return "Convierte bloques de código Mermaid de formato markdown (```mermaid) al formato SlideLang (<<mermaid>>)"
}

func (r *MermaidRule) Priority() int {
	return 4 // Prioridad 4 - Mejoras de formato específicas
}

func (r *MermaidRule) Category() base.RuleCategory {
	return base.CategoryEnhancement
}
