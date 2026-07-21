// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package base

import "strings"

// FrontmatterParser centraliza la lógica de parsing de frontmatter reutilizable
type FrontmatterParser struct{}

// NewFrontmatterParser crea una nueva instancia del parser
func NewFrontmatterParser() *FrontmatterParser {
	return &FrontmatterParser{}
}

// HasFrontmatter verifica rápidamente si el contenido tiene frontmatter
func (p *FrontmatterParser) HasFrontmatter(content string) bool {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return false
	}

	// Buscar el cierre del frontmatter
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return true
		}
	}

	return false // Frontmatter sin cerrar
}

// CreateBasicFrontmatter crea frontmatter básico con título extraído del contenido
func (p *FrontmatterParser) CreateBasicFrontmatter(content string) string {
	lines := strings.Split(content, "\n")

	// Extraer título del primer heading encontrado
	title := "AI Generated Presentation"
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimSpace(trimmed[2:])
			break
		}
	}

	return `---
title: "` + title + `"
mode: flex-full
ai_generated: true
ai_processed: true
---

`
}

// HasMode verifica si el frontmatter tiene el campo mode especificado
func (p *FrontmatterParser) HasMode(content string) bool {
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

		if inFrontmatter && strings.HasPrefix(trimmed, "mode:") {
			return true
		}
	}

	return false
}

// AddModeToFrontmatter agrega el campo mode al frontmatter existente
func (p *FrontmatterParser) AddModeToFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inFrontmatter := false
	modeAdded := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			if inFrontmatter {
				// Final del frontmatter - agregar mode antes de cerrar si no se agregó
				if !modeAdded {
					result = append(result, "mode: flex-full")
					modeAdded = true
				}
				result = append(result, line)
				inFrontmatter = false
			} else {
				// Inicio del frontmatter
				result = append(result, line)
				inFrontmatter = true
			}
			continue
		}

		if inFrontmatter && !modeAdded {
			// Agregar mode después del title si existe
			if strings.HasPrefix(trimmed, "title:") {
				result = append(result, line)
				result = append(result, "mode: flex-full")
				modeAdded = true
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
