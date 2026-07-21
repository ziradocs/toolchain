// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package structure

import (
	"strings"

	"go.ziradocs.com/core/internal/normalize/normalizer/base"
)

// SeparatorsRule normaliza separadores Markdown y maneja estructura de slides
type SeparatorsRule struct{}

func (r *SeparatorsRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	modified := false
	processedLines := make([]string, len(lines))

	// Solo hacer normalización básica de separadores Markdown estándar
	for i := 0; i < len(lines); i++ {
		originalLine := lines[i]
		trimmedLine := strings.TrimSpace(originalLine)

		// Si ya es el separador correcto, preservar la línea original
		if trimmedLine == "---" {
			processedLines[i] = originalLine
			continue
		}

		// Detectar separadores horizontales de Markdown
		if r.isMarkdownSeparator(trimmedLine) {
			// Convertir a separador slidelang estándar
			processedLines[i] = "---"
			modified = true
		} else {
			// Preservar la línea original sin cambios
			processedLines[i] = originalLine
		}
	}

	if modified {
		return strings.Join(processedLines, "\n"), nil
	}
	return content, nil
}

// isMarkdownSeparator detecta si una línea es un separador horizontal de Markdown
func (r *SeparatorsRule) isMarkdownSeparator(line string) bool {
	if len(line) < 3 {
		return false
	}

	firstChar := line[0]
	if firstChar != '-' && firstChar != '*' && firstChar != '_' {
		return false
	}

	count := 0
	for _, char := range line {
		if char == rune(firstChar) {
			count++
		} else if char != ' ' {
			return false
		}
	}

	return count >= 3
}

func (r *SeparatorsRule) Description() string {
	return "Normaliza separadores Markdown estándar (---, ***, ___) y maneja estructura de slides"
}

func (r *SeparatorsRule) Priority() int {
	return 2
}

func (r *SeparatorsRule) Category() base.RuleCategory {
	return base.CategoryStructure
}
