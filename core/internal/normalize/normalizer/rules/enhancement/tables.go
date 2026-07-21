// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"

	"go.ziradocs.com/core/internal/normalize/normalizer/base"
)

// TablesRule detecta y transforma listas que deberían ser tablas
type TablesRule struct {
	classifier       *base.ContentClassifier
	documentAnalyzer *base.DocumentAnalyzer
}

// NewTablesRule crea una nueva instancia de la regla
func NewTablesRule() *TablesRule {
	return &TablesRule{
		classifier:       base.NewContentClassifier(),
		documentAnalyzer: base.NewDocumentAnalyzer(),
	}
}

func (r *TablesRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	modified := false

	startLine := r.documentAnalyzer.SkipFrontmatter(lines)

	// Buscar patrones de listas que sugieren tablas
	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		// Si encontramos una tabla Markdown existente, no modificar
		if strings.Contains(line, "|") && strings.Contains(line, "-") {
			continue
		}

		// Si la línea parece ser parte de una tabla Markdown existente, saltar
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") && strings.Count(line, "|") > 2 {
			continue
		}

		// Saltar bloques especiales (mermaid, chart, map)
		if r.isInSpecialBlock(lines, i) {
			continue
		} // Buscar listas estructuradas que sugieren tablas
		if r.looksLikeTableData(lines, i) {
			tableData, consumed := r.extractTableData(lines, i)
			if len(tableData) >= 3 { // Al menos 3 filas para justificar una tabla
				markdownTable := r.convertToMarkdownTable(tableData)

				// Reemplazar las líneas originales con la tabla
				newLines := make([]string, 0, len(lines))
				newLines = append(newLines, lines[:i]...)
				newLines = append(newLines, markdownTable)
				newLines = append(newLines, lines[i+consumed:]...)

				lines = newLines
				modified = true
				break // Procesar una tabla a la vez
			}
		}
	}

	if modified {
		return strings.Join(lines, "\n"), nil
	}
	return content, nil
}

// isInCodeFence detecta si la línea en `index` está dentro de un code fence
// delimitado por "```" (p.ej. ```chart, ```map, ```json, o un fence sin
// lenguaje). Espeja EXACTAMENTE la asimetría open/close que usa el parser
// real (internal/elements/code.go, CodeParser.parseFlexCode): una línea
// SIN fence abierto que empieza con "```" (con o sin tag de lenguaje) abre
// uno; una línea CON fence abierto solo lo cierra si es un "```" EXACTO
// tras trim (parseFlexCode: `strings.TrimSpace(line) == "```"`) — NO
// cualquier línea que empiece con backticks. Un simple toggle por prefix
// match (versión anterior de esta función) trataba erróneamente una línea
// de contenido DENTRO del fence que solo empieza con "```" (p.ej. un
// string JSON que documenta su propia sintaxis con literal "```json" como
// texto) como el cierre, reabriendo la puerta al mismo bug que esta regla
// existe para arreglar — encontrado en code-review, no en el corpus.
func (r *TablesRule) isInCodeFence(lines []string, index int) bool {
	inFence := false
	for i := 0; i < index && i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if inFence {
			if line == "```" {
				inFence = false
			}
			continue
		}
		if strings.HasPrefix(line, "```") {
			inFence = true
		}
	}
	return inFence
}

// isInSpecialBlock detecta si la línea actual está dentro de un bloque especial
func (r *TablesRule) isInSpecialBlock(lines []string, index int) bool {
	// Los code fences (```chart, ```map, ```json, etc.) siempre son bloques
	// especiales: nunca reescribir su contenido como tabla Markdown.
	if r.isInCodeFence(lines, index) {
		return true
	}

	// Buscar hacia atrás para encontrar el inicio de un bloque especial
	for i := index - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])

		// Si encontramos un separador de slides, no estamos en un bloque especial
		if line == "---" {
			return false
		}

		// Detectar inicio de bloques especiales de SlideLang
		if strings.HasPrefix(line, "<<") && strings.HasSuffix(line, ">>") {
			// Verificar si aún estamos dentro del bloque
			// Los bloques terminan con línea vacía o "---"
			for j := i + 1; j <= index; j++ {
				checkLine := strings.TrimSpace(lines[j])
				if checkLine == "" || checkLine == "---" {
					return false
				}
			}
			return true
		}
	}
	return false
}

// looksLikeTableData verifica si una serie de líneas parece datos de tabla
func (r *TablesRule) looksLikeTableData(lines []string, startIndex int) bool {
	// Verificar que no estamos dentro de un bloque especial
	if r.isInSpecialBlock(lines, startIndex) {
		return false
	}

	// Verificar si ya es una tabla Markdown bien formada
	currentLine := strings.TrimSpace(lines[startIndex])
	if strings.HasPrefix(currentLine, "|") && strings.HasSuffix(currentLine, "|") {
		// Es una tabla Markdown, verificar si está bien formada
		nextIndex := startIndex + 1
		if nextIndex < len(lines) {
			nextLine := strings.TrimSpace(lines[nextIndex])
			// Si la siguiente línea es un separador, es una tabla Markdown válida
			if strings.Contains(nextLine, "---") || strings.Contains(nextLine, ":-:") ||
				strings.Contains(nextLine, ":--") || strings.Contains(nextLine, "--:") {
				return false // Es una tabla Markdown válida, no procesar
			}
		}
	}

	// Verificar contexto adicional - no procesar si parece configuración
	if startIndex > 0 {
		prevLine := strings.TrimSpace(lines[startIndex-1])
		// No procesar si la línea anterior parece una clave de configuración
		if strings.HasSuffix(prevLine, ":") && !strings.Contains(prevLine, " ") {
			return false
		}
	}

	count := 0
	patternCount := 0
	consistentPattern := true
	var firstPattern string

	for i := startIndex; i < len(lines) && count < 5; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || line == "---" {
			break
		}

		// Si es una lista simple con bullets, NO es tabla
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") ||
			strings.HasPrefix(line, "+ ") || strings.HasPrefix(line, "#") {
			return false
		}

		// No procesar arrays JSON-like
		if strings.HasPrefix(line, "[") || strings.HasPrefix(line, "]") ||
			strings.Contains(line, "[\"") || strings.Contains(line, "\",") {
			return false
		}

		// Detectar y validar el patrón
		currentPattern := r.detectLinePattern(line)
		if currentPattern != "" {
			if firstPattern == "" {
				firstPattern = currentPattern
			}
			if currentPattern == firstPattern {
				patternCount++
			} else {
				consistentPattern = false
			}
		}

		count++
	}

	// Requerir al menos 3 líneas con patrones consistentes
	return count >= 3 && patternCount >= 3 && consistentPattern
}

// detectLinePattern detecta el patrón de separación en una línea
func (r *TablesRule) detectLinePattern(line string) string {
	// Contar delimitadores
	pipeCount := strings.Count(line, "|")
	tabCount := strings.Count(line, "\t")
	colonCount := strings.Count(line, ":")

	// Priorizar pipes (más común en tablas)
	if pipeCount >= 2 {
		return "pipe"
	}
	// Tabs también son comunes
	if tabCount >= 2 {
		return "tab"
	}
	// Solo aceptar dos puntos si parece key:value consistente
	if colonCount == 1 && r.classifier.LooksLikeKeyValuePair(line) &&
		!r.classifier.LooksLikeDescriptiveText(line) {
		// Verificar que no sea código o definición
		if !strings.Contains(line, "//") && !strings.Contains(line, "=>") &&
			!strings.Contains(line, "->") && !strings.Contains(line, "function") &&
			!strings.Contains(line, "var") && !strings.Contains(line, "let") {
			return "colon"
		}
	}

	return ""
}

// extractTableData extrae datos de tabla de las líneas
func (r *TablesRule) extractTableData(lines []string, startIndex int) ([][]string, int) {
	var tableData [][]string
	consumed := 0

	for i := startIndex; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			break
		}

		var row []string
		// Separar por diferentes delimitadores
		if strings.Contains(line, "|") {
			row = strings.Split(line, "|")
			// Limpiar celdas vacías de los extremos (típico en tablas Markdown)
			if len(row) > 0 && strings.TrimSpace(row[0]) == "" {
				row = row[1:]
			}
			if len(row) > 0 && strings.TrimSpace(row[len(row)-1]) == "" {
				row = row[:len(row)-1]
			}
		} else if strings.Contains(line, "\t") {
			row = strings.Split(line, "\t")
		} else if strings.Contains(line, ":") {
			row = strings.SplitN(line, ":", 2)
		} else {
			break
		}

		// Limpiar espacios
		for j := range row {
			row[j] = strings.TrimSpace(row[j])
		}

		tableData = append(tableData, row)
		consumed++

		if consumed >= 10 { // Límite de seguridad
			break
		}
	}

	return tableData, consumed
}

// convertToMarkdownTable convierte datos a tabla Markdown
func (r *TablesRule) convertToMarkdownTable(data [][]string) string {
	if len(data) == 0 {
		return ""
	}

	// Determinar número de columnas
	maxCols := 0
	for _, row := range data {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	if maxCols < 2 {
		return "" // No vale la pena hacer tabla
	}

	var result strings.Builder

	// Header row (primera fila)
	result.WriteString("|")
	for i := 0; i < maxCols; i++ {
		if i < len(data[0]) {
			result.WriteString(" " + data[0][i] + " |")
		} else {
			result.WriteString("   |")
		}
	}
	result.WriteString("\n")

	// Separator row
	result.WriteString("|")
	for i := 0; i < maxCols; i++ {
		result.WriteString("---|")
	}
	result.WriteString("\n")

	// Data rows
	for i := 1; i < len(data); i++ {
		result.WriteString("|")
		for j := 0; j < maxCols; j++ {
			if j < len(data[i]) {
				result.WriteString(" " + data[i][j] + " |")
			} else {
				result.WriteString("   |")
			}
		}
		result.WriteString("\n")
	}

	return result.String()
}

func (r *TablesRule) Description() string {
	return "Convierte listas estructuradas en tablas Markdown cuando es apropiado"
}

func (r *TablesRule) Priority() int {
	return 5
}

func (r *TablesRule) Category() base.RuleCategory {
	return base.CategoryEnhancement
}
