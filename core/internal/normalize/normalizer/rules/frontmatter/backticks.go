// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package frontmatter

import (
	"strings"

	"go.ziradocs.com/core/internal/normalize/normalizer/base"
)

// BackticksCleanupRule elimina backticks de markdown que envuelven incorrectamente el frontmatter
type BackticksCleanupRule struct{}

// Apply aplica la regla de limpieza de backticks
func (r *BackticksCleanupRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 {
		return content, nil
	}

	modified := false

	// CASO 1: Eliminar ```markdown que envuelve todo el archivo
	if r.hasMarkdownWrapperPattern(lines) {
		lines, modified = r.removeMarkdownWrapper(lines)
	}

	// CASO 2: Eliminar ```yaml que envuelve frontmatter específico
	if !modified { // Solo si no se aplicó el caso 1
		lines, modified = r.removeYamlFrontmatterBackticks(lines)
	}

	if modified {
		return strings.Join(lines, "\n"), nil
	}
	return content, nil
}

func (r *BackticksCleanupRule) Description() string {
	return "Elimina backticks de markdown (```yaml/```, ```markdown) que envuelven incorrectamente el frontmatter o todo el archivo"
}

func (r *BackticksCleanupRule) Priority() int {
	return 0 // Máxima prioridad
}

func (r *BackticksCleanupRule) Category() base.RuleCategory {
	return base.CategoryFrontmatter
}

// hasMarkdownWrapperPattern detecta si el archivo completo está envuelto en ```markdown
func (r *BackticksCleanupRule) hasMarkdownWrapperPattern(lines []string) bool {
	if len(lines) < 3 {
		return false
	}

	firstLine := strings.TrimSpace(lines[0])
	lastLine := strings.TrimSpace(lines[len(lines)-1])

	// Buscar ```markdown al inicio y ``` al final
	return firstLine == "```markdown" && lastLine == "```"
}

// removeMarkdownWrapper elimina ```markdown del inicio y ``` del final
func (r *BackticksCleanupRule) removeMarkdownWrapper(lines []string) ([]string, bool) {
	if len(lines) < 3 {
		return lines, false
	}

	// Eliminar primera línea (```markdown) y última línea (```
	result := lines[1 : len(lines)-1]

	// Limpiar líneas vacías al inicio y final si existen
	for len(result) > 0 && strings.TrimSpace(result[0]) == "" {
		result = result[1:]
	}
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	return result, true
}

// removeYamlFrontmatterBackticks maneja el caso específico de ```yaml alrededor del frontmatter
func (r *BackticksCleanupRule) removeYamlFrontmatterBackticks(lines []string) ([]string, bool) {
	result := make([]string, 0, len(lines))
	modified := false

	// Buscar patrón: ```yaml seguido de frontmatter y terminado con ```
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Detectar inicio de bloque markdown yaml que precede a frontmatter
		if (line == "```yaml" || line == "```yml" || line == "```") && i+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[i+1])

			// Si la siguiente línea es el inicio de frontmatter
			if nextLine == "---" {
				// Buscar el final del bloque markdown
				frontmatterStart := i + 1
				frontmatterEnd := -1
				markdownEnd := -1

				// Buscar el final del frontmatter
				for j := frontmatterStart + 1; j < len(lines); j++ {
					if strings.TrimSpace(lines[j]) == "---" {
						frontmatterEnd = j
						break
					}
				}

				// Buscar el cierre del bloque markdown después del frontmatter
				if frontmatterEnd != -1 {
					for j := frontmatterEnd + 1; j < len(lines); j++ {
						if strings.TrimSpace(lines[j]) == "```" {
							markdownEnd = j
							break
						}
					}
				}

				// Si encontramos el patrón completo, eliminamos los backticks
				if frontmatterEnd != -1 && markdownEnd != -1 {
					// Saltar el ```yaml inicial
					// Agregar líneas del frontmatter (desde --- hasta ---)
					for k := frontmatterStart; k <= frontmatterEnd; k++ {
						result = append(result, lines[k])
					}
					// Saltar el ``` final
					i = markdownEnd + 1
					modified = true
					continue
				}
			}
		}

		// Si no es el patrón especial, agregar la línea normal
		result = append(result, lines[i])
		i++
	}

	return result, modified
}
