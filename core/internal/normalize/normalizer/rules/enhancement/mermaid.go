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
	// Buscar los bloques ```mermaid … ``` línea por línea, con la lectura de
	// cercas del parser (fence.At). Solo se reescribe una cerca mermaid que
	// está suelta: el cuerpo de cualquier otra cerca (un ````md que muestra
	// un ```mermaid) y un :::code-group entero son código y se copian tal
	// cual. Antes un regexp tomaba también un ````mermaid y cerraba en
	// cualquier línea que empezara con ```, y un ```mermaid dentro de otra
	// cerca o de una pestaña salía como <<mermaid>>.
	lines := strings.Split(content, "\n")
	blocks := fence.Walk(lines, 0)
	var out []string
	changed := false
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if fence.IsCodeGroupOpener(trimmed) {
			end, _ := fence.ContainerEnd(lines, i, len(lines), nil)
			if end >= len(lines) {
				end = len(lines) - 1
			}
			out = append(out, lines[i:end+1]...)
			i = end
			continue
		}
		block := blocks[i]
		if block == nil {
			out = append(out, lines[i])
			continue
		}
		end := block.End
		if block.Reading != fence.CommonMark || fence.Info(trimmed) != "mermaid" {
			// Otra cerca, o una mermaid sin un cierre de CommonMark: queda
			// como está, con todo su cuerpo.
			if end < 0 {
				end = block.Limit - 1
			}
			out = append(out, lines[i:end+1]...)
			i = end
			continue
		}
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
		mermaid := indent + "<<mermaid>>"
		if clean := r.cleanMermaidContent(strings.Join(lines[i+1:end], "\n")); strings.TrimSpace(clean) != "" {
			mermaid += "\n" + clean
		}
		out = append(out, mermaid)
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
