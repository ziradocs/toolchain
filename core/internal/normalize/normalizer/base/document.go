// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package base

import "strings"

// DocumentAnalyzer analiza y normaliza la estructura del documento
type DocumentAnalyzer struct{}

// NewDocumentAnalyzer crea una nueva instancia del analizador
func NewDocumentAnalyzer() *DocumentAnalyzer {
	return &DocumentAnalyzer{}
}

// SkipFrontmatter retorna el índice donde continuar después del frontmatter
func (da *DocumentAnalyzer) SkipFrontmatter(lines []string) int {
	if len(lines) == 0 || lines[0] != "---" {
		return 0
	}

	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return i + 1 // Después del cierre
		}
	}
	return 0
}

// ExtractFrontmatterTitle extrae solo el título del frontmatter
func (da *DocumentAnalyzer) ExtractFrontmatterTitle(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return ""
	}

	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			break
		}
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "title:") {
			title := strings.TrimSpace(line[6:])
			// Quitar comillas si existen
			if len(title) >= 2 && title[0] == '"' && title[len(title)-1] == '"' {
				return title[1 : len(title)-1]
			}
			return title
		}
	}
	return ""
}

// TitlesAreIdentical verifica si dos títulos son exactamente iguales
func (da *DocumentAnalyzer) TitlesAreIdentical(title1, title2 string) bool {
	normalize := func(s string) string {
		return strings.ToLower(strings.TrimSpace(s))
	}
	return normalize(title1) == normalize(title2)
}

// FindFirstHeader encuentra el primer header después del frontmatter
func (da *DocumentAnalyzer) FindFirstHeader(lines []string) (headerText string, line int) {
	startLine := 0
	if len(lines) > 0 && lines[0] == "---" {
		for i := 1; i < len(lines); i++ {
			if lines[i] == "---" {
				startLine = i + 1
				break
			}
		}
	}

	for i := startLine; i < len(lines); i++ {
		lineText := strings.TrimSpace(lines[i])
		if strings.HasPrefix(lineText, "# ") {
			text := strings.TrimSpace(lineText[2:])
			return text, i
		}
	}
	return "", -1
}

// FindNextNonEmptyLine encuentra la siguiente línea no vacía
func (da *DocumentAnalyzer) FindNextNonEmptyLine(lines []string, startIndex int) int {
	for i := startIndex + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return -1
}

// FindPrevNonEmptyLine encuentra la línea anterior no vacía
func (da *DocumentAnalyzer) FindPrevNonEmptyLine(lines []string, startIndex int) int {
	for i := startIndex - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return -1
}
