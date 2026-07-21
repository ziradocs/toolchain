// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"
)

// ElementClosingTagsRule normaliza los tags de cierre de elementos especiales
// Convierte >> a <</element>> para charts, maps, plantuml, etc.
type ElementClosingTagsRule struct{}

// NewElementClosingTagsRule crea una nueva instancia
func NewElementClosingTagsRule() *ElementClosingTagsRule {
	return &ElementClosingTagsRule{}
}

func (r *ElementClosingTagsRule) Name() string {
	return "ElementClosingTagsRule"
}

func (r *ElementClosingTagsRule) Description() string {
	return "Normaliza tags de cierre >> a <</element>> para charts, maps, plantuml, etc."
}

func (r *ElementClosingTagsRule) Priority() int {
	return 1 // Prioridad alta para ejecutar temprano
}

func (r *ElementClosingTagsRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	// Elementos que soportan cierre con >>
	var currentElement string
	inElement := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Detectar inicio de elemento (sin dos puntos, modo multilínea)
		if !inElement {
			if strings.HasPrefix(trimmed, "<<chart") && !strings.Contains(trimmed, ":") && !strings.HasSuffix(trimmed, ">>") {
				currentElement = "chart"
				inElement = true
				result = append(result, line)
				continue
			}
			if strings.HasPrefix(trimmed, "<<map") && !strings.Contains(trimmed, ":") && !strings.HasSuffix(trimmed, ">>") {
				currentElement = "map"
				inElement = true
				result = append(result, line)
				continue
			}
			if strings.HasPrefix(trimmed, "<<plantuml") && !strings.HasSuffix(trimmed, ">>") {
				currentElement = "plantuml"
				inElement = true
				result = append(result, line)
				continue
			}
		}

		// Si estamos dentro de un elemento, buscar el cierre
		if inElement {
			// Si encontramos >> solo, convertirlo al tag de cierre apropiado
			if trimmed == ">>" {
				result = append(result, strings.Replace(line, ">>", "<</"+currentElement+">>", 1))
				inElement = false
				currentElement = ""
				continue
			}

			// También detectar cierre correcto para salir del modo
			if trimmed == "<</"+currentElement+">>" {
				inElement = false
				currentElement = ""
				result = append(result, line)
				continue
			}
		}

		// Línea normal
		result = append(result, line)
	}

	return strings.Join(result, "\n"), nil
}
