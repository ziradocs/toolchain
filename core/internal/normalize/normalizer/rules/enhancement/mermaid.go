// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"

	"go.ziradocs.com/core/v2/internal/fence"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// MermaidRule convierte bloques de código Mermaid de formato markdown (```mermaid) al formato SlideLang (<<mermaid>>)
type MermaidRule struct{}

// NewMermaidRule crea una nueva instancia de la regla
func NewMermaidRule() *MermaidRule {
	return &MermaidRule{}
}

func (r *MermaidRule) Apply(content string) (string, error) {
	// Buscar los bloques ```mermaid … ``` línea por línea, con la regla de
	// cercas del parser (internal/fence): abre una cerca cuyo info string es
	// "mermaid" y cierra una línea de solo backticks, al menos tantos como la
	// apertura. Antes un regexp (```mermaid … ```) también tomaba un
	// ````mermaid y cerraba en cualquier línea que empezara con ```.
	lines := strings.Split(content, "\n")
	var out []string
	changed := false
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		ticks := fence.Open(trimmed)
		if ticks == 0 || fence.Info(trimmed) != "mermaid" {
			out = append(out, lines[i])
			continue
		}
		end := -1
		for j := i + 1; j < len(lines); j++ {
			if fence.Closes(strings.TrimSpace(lines[j]), ticks) {
				end = j
				break
			}
		}
		if end < 0 {
			// Sin cierre no hay bloque: queda como está, igual que antes.
			out = append(out, lines[i:]...)
			break
		}
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
		block := indent + "<<mermaid>>"
		if clean := r.cleanMermaidContent(strings.Join(lines[i+1:end], "\n")); strings.TrimSpace(clean) != "" {
			block += "\n" + clean
		}
		out = append(out, block)
		changed = true
		i = end
	}
	if !changed {
		return content, nil
	}
	return strings.Join(out, "\n"), nil
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
