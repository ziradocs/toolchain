// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// ChartParser maneja el parsing de gráficos Chart.js
type ChartParser struct{}

// CanParse determina si puede parsear una línea como Chart
func (p *ChartParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	switch mode {
	case "strict":
		// Acepta tanto <<chart: (inline) como <<chart (multilínea)
		return strings.HasPrefix(trimmed, "<<chart:") || strings.HasPrefix(trimmed, "<<chart")
	case "flex":
		// Acepta tanto <<chart: (inline) como <<chart (multilínea)
		return strings.HasPrefix(trimmed, "<<chart:") || strings.HasPrefix(trimmed, "<<chart")
	}

	return false
}

// Parse parsea un elemento Chart
func (p *ChartParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{Error: nil}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	// Extraer tipo y atributos: "<<chart: bar width="1200" height="600">>"
	chartType := "bar"
	width := 800  // default
	height := 600 // default

	if strings.Contains(line, ":") {
		parts := strings.Split(line, ":")
		if len(parts) > 1 {
			attrStr := strings.TrimSpace(parts[1])
			attrStr = strings.TrimSuffix(attrStr, ">>")

			// Extraer tipo (primera palabra)
			tokens := strings.Fields(attrStr)
			if len(tokens) > 0 {
				chartType = tokens[0]
			}

			// Extraer width y height si están presentes
			if w := extractAttribute(attrStr, "width"); w != "" {
				if val, err := strconv.Atoi(w); err == nil && val > 0 {
					width = val
				}
			}
			if h := extractAttribute(attrStr, "height"); h != "" {
				if val, err := strconv.Atoi(h); err == nil && val > 0 {
					height = val
				}
			}
		}
	}
	chart := ast.NewChartElement(pos, chartType)
	chart.Width = width
	chart.Height = height
	consumedLines := 1 // skip <<chart:>> line
	indentDetector := NewAutoDetectIndentation()

	// Detectar si el siguiente contenido es JSON
	if startIndex+1 < len(ctx.Lines) {
		nextLine := strings.TrimSpace(ctx.Lines[startIndex+1])
		if strings.HasPrefix(nextLine, "{") {
			// Es JSON directo, parsearlo como tal
			jsonContent, jsonLines := p.parseJSONBlock(ctx.Lines, startIndex+1)
			if jsonContent != "" {
				consumedLines += jsonLines

				var diags []diagnostics.Diagnostic
				if json.Valid([]byte(jsonContent)) {
					chart.RawJSON = json.RawMessage(jsonContent)
					chart.IsJSONMode = true
				} else {
					// JSON inválido: no se activa IsJSONMode y el chart queda sin datos
					// (CHART001 lo reportará como "sin datos" de forma engañosa), pero
					// el bloque completo (incluyendo su <</chart>> de cierre, ya contado
					// en jsonLines) se consume igual para no dejar el cierre como texto
					// suelto ni reprocesarlo como propiedades data:/series:/etc. Este
					// diagnóstico Warning no aborta el build (a diferencia de Error) y
					// es la única señal específica de "el JSON estaba roto".
					diags = []diagnostics.Diagnostic{
						diagnostics.NewWarning(
							"El JSON del chart es inválido y fue ignorado; el chart quedará sin datos",
							pos, "chart-parser").WithRuleID("CHART002"),
					}
				}
				return &ParseResult{
					Element:       chart,
					ConsumedLines: consumedLines,
					Error:         nil,
					Diagnostics:   diags,
				}
			}
		}
	}

	// Detectar si hay una estructura YAML compleja (para combo charts)
	if chartType == "combo" && startIndex+1 < len(ctx.Lines) {
		yamlContent, yamlLines := p.parseYAMLBlock(ctx.Lines, startIndex+1)
		if yamlContent != "" {
			if p.parseComboChartYAML(chart, yamlContent) {
				consumedLines += yamlLines
				return &ParseResult{
					Element:       chart,
					ConsumedLines: consumedLines,
					Error:         nil,
				}
			}
		}
	}

	// Parsear propiedades del chart
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]
		trimmedLine := strings.TrimSpace(line)

		// Check for closing tag FIRST
		if trimmedLine == "<<end>>" {
			consumedLines++
			break
		}

		// Cualquier otro límite de contenido (separador de slide, otro
		// elemento, heading) — ver isChartContentBoundary.
		if isChartContentBoundary(trimmedLine) {
			break
		}

		// Skip empty lines
		if trimmedLine == "" {
			consumedLines++
			continue
		}

		// Parsear propiedades como "data:", "series:", etc.
		if strings.Contains(trimmedLine, ":") {
			parts := strings.SplitN(trimmedLine, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "data":
					// Detectar si hay datos definidos
					if value == "[" {
						// Datos en formato array multi-línea - parsear las líneas siguientes
						data, linesConsumed := p.parseMultiLineArray(ctx.Lines, i+1, indentDetector)
						chart.Data = data
						i += linesConsumed
						consumedLines += linesConsumed
					} else if strings.Contains(value, "[") {
						// Datos inline - verificar si es array de arrays o array simple
						if strings.HasPrefix(value, "[[") {
							// Array de arrays inline: [[100, 500, 350, 220]]
							chart.Data = p.parseInlineMatrix(value)
						} else {
							// Array simple inline: [100, 500, 350, 220]
							if row := p.parseArrayRow(value); len(row) > 0 {
								chart.Data = [][]interface{}{row}
							}
						}
					}
				case "series":
					// Parsear series como array de strings
					if value == "[" { // Series en formato array multi-línea
						series, linesConsumed := p.parseMultiLineStringArray(ctx.Lines, i+1, indentDetector)
						chart.Series = series
						i += linesConsumed
						consumedLines += linesConsumed
					} else if strings.Contains(value, "[") {
						// Series inline - parsear directamente del value
						if series := p.parseQuotedStrings(value); len(series) > 0 {
							chart.Series = series
						}
					}
				case "labels":
					// Parsear labels como array de strings
					if value == "[" { // Labels en formato array multi-línea
						labels, linesConsumed := p.parseMultiLineStringArray(ctx.Lines, i+1, indentDetector)
						chart.Labels = labels
						i += linesConsumed
						consumedLines += linesConsumed
					} else if strings.Contains(value, "[") {
						// Labels inline - parsear directamente del value
						if labels := p.parseQuotedStrings(value); len(labels) > 0 {
							chart.Labels = labels
						}
					}
				case "title":
					chart.Title = strings.Trim(value, "\"")
				case "type":
					// Parsear type array para combo charts: ["bar", "bar", "line"]
					if value == "[" {
						// Tipos en formato array multi-línea
						types, linesConsumed := p.parseMultiLineStringArray(ctx.Lines, i+1, indentDetector)
						chart.SeriesTypes = types
						i += linesConsumed
						consumedLines += linesConsumed
					} else if strings.Contains(value, "[") {
						// Tipos inline - parsear directamente del value
						if types := p.parseQuotedStrings(value); len(types) > 0 {
							chart.SeriesTypes = types
						}
					}
				}
			}
		}

		consumedLines++
	}

	// chart.Data no lleva omitempty en el AST: si un bloque "data:" vacío o
	// mal formado dejó chart.Data en nil, se serializaría como JSON null en
	// vez de [] (issue #8 - viola el JSON Schema del contrato).
	if chart.Data == nil {
		chart.Data = [][]interface{}{}
	}

	return &ParseResult{
		Element:       chart,
		ConsumedLines: consumedLines,
		Error:         nil,
	}
}

// parseMultiLineArray parsea un array multi-línea de datos para charts
func (p *ChartParser) parseMultiLineArray(lines []string, startIndex int, indentDetector *AutoDetectIndentation) ([][]interface{}, int) {
	var data [][]interface{}
	linesConsumed := 0

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check if this line should be processed
		if !indentDetector.ShouldProcessLine(line, false, i+1, "CHART-PARSER") {
			break
		}

		// Skip empty lines
		if trimmed == "" {
			linesConsumed++
			continue
		}

		// Check for end of array
		if strings.Contains(trimmed, "]") && !strings.Contains(trimmed, "[") {
			linesConsumed++
			break
		}

		// Parse array row like ["Q1", 45, 32, 28]
		if strings.HasPrefix(trimmed, "[") {
			row := p.parseArrayRow(trimmed)
			if len(row) > 0 {
				data = append(data, row)
			}
		}

		linesConsumed++
	}

	return data, linesConsumed
}

// parseMultiLineStringArray parsea un array multi-línea de strings para series
func (p *ChartParser) parseMultiLineStringArray(lines []string, startIndex int, indentDetector *AutoDetectIndentation) ([]string, int) {
	var series []string
	linesConsumed := 0

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check if this line should be processed
		if !indentDetector.ShouldProcessLine(line, false, i+1, "CHART-PARSER") {
			break
		}

		// Skip empty lines
		if trimmed == "" {
			linesConsumed++
			continue
		}

		// Check for end of array
		if strings.Contains(trimmed, "]") && !strings.Contains(trimmed, "[") {
			linesConsumed++
			break
		}

		// Parse string items like "Product A", "Product B"
		if strings.Contains(trimmed, "\"") {
			// Extract all quoted strings from the line
			items := p.parseQuotedStrings(trimmed)
			series = append(series, items...)
		}

		linesConsumed++
	}

	return series, linesConsumed
}

// parseArrayRow parsea una fila de array como ["Q1", 45, 32, 28]
func (p *ChartParser) parseArrayRow(line string) []interface{} {
	var row []interface{}

	// Remove brackets properly - handle cases like ["January", 40], and ["June", 150]
	content := strings.TrimSpace(line)
	content = strings.TrimPrefix(content, "[")
	content = strings.TrimSuffix(content, "]")
	content = strings.TrimSuffix(content, "],") // Handle trailing comma
	content = strings.TrimSpace(content)

	parts := strings.Split(content, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Try to parse as number
		if strings.Contains(part, "\"") {
			// String value
			str := strings.Trim(part, "\"")
			row = append(row, str)
		} else {
			// Try to parse as number
			if val := p.parseNumber(part); val != nil {
				row = append(row, val)
			}
		}
	}

	return row
}

// parseQuotedStrings extrae todas las cadenas entre comillas de una línea
func (p *ChartParser) parseQuotedStrings(line string) []string {
	var result []string
	inQuotes := false
	var current strings.Builder

	for _, char := range line {
		if char == '"' {
			if inQuotes {
				// End of quoted string
				result = append(result, current.String())
				current.Reset()
				inQuotes = false
			} else {
				// Start of quoted string
				inQuotes = true
			}
		} else if inQuotes {
			current.WriteRune(char)
		}
	}

	return result
}

// parseNumber intenta parsear un string como número
func (p *ChartParser) parseNumber(str string) interface{} {
	str = strings.TrimSpace(str)

	// Try integer first
	if val, err := strconv.Atoi(str); err == nil {
		return val
	}

	// Try float
	if val, err := strconv.ParseFloat(str, 64); err == nil {
		return val
	}

	return nil
}

// isChartContentBoundary reporta si trimmed marca el fin del contenido de un
// chart: el cierre del propio bloque ("<</chart>>"), un separador de slide,
// o el inicio de un nuevo elemento/sección. Compartida entre el loop de
// propiedades y parseJSONBlock para que ambos no puedan desincronizarse
// sobre qué cuenta como límite (issue #12e2 — la revisión de esta misma PR
// encontró que el check original de parseJSONBlock solo reconocía 2 de los
// 5 límites que el loop de propiedades ya conocía).
func isChartContentBoundary(trimmed string) bool {
	if trimmed == "<<end>>" || trimmed == "---" {
		return true
	}
	if strings.HasPrefix(trimmed, "<<") {
		return true
	}
	if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "##") {
		return true // H1 crea nuevas secciones, no H2/H3 ("##", "###")
	}
	if strings.HasPrefix(trimmed, "##") {
		return true // H2/H3 son subsection headers
	}
	return false
}

// parseJSONBlock parsea un bloque JSON completo desde las líneas
func (p *ChartParser) parseJSONBlock(lines []string, startIndex int) (string, int) {
	var jsonLines []string
	braceCount := 0
	inString := false
	escaped := false
	linesConsumed := 0

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]

		// Si las llaves nunca balancean (JSON truncado/mal formado), no
		// tragarse el separador de slide ni el cierre del propio bloque:
		// detenerse aquí y dejar la línea sin consumir para que el parser
		// de nivel superior la procese normalmente (issue #12e2 — antes,
		// el fallback de líneas 459 y siguientes devolvía TODO el resto del
		// documento, incluyendo "---", como si fuera parte del JSON).
		// Gateado por "!inString": un valor JSON legítimo puede contener,
		// en una línea propia, el texto exacto "---" o "<</chart>>" (p.ej.
		// una descripción documentando la sintaxis del DSL) sin que eso
		// signifique que el bloque terminó.
		if !inString {
			if isChartContentBoundary(strings.TrimSpace(line)) {
				break
			}
		}

		jsonLines = append(jsonLines, line)
		linesConsumed++

		// Procesar caracteres para encontrar el final del JSON
		for _, char := range line {
			if escaped {
				escaped = false
				continue
			}

			if char == '\\' {
				escaped = true
				continue
			}

			if char == '"' {
				inString = !inString
				continue
			}

			if !inString {
				switch char {
				case '{':
					braceCount++
				case '}':
					braceCount--
					if braceCount == 0 {
						// JSON completo encontrado
						// Check if next line is closing tag
						if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "<</chart>>" {
							linesConsumed++ // Consume closing tag line
						}

						// Compactar para evitar problemas de template
						jsonStr := strings.Join(jsonLines, "\n")
						// Remover saltos de línea y espacios extra para compactar
						compactJSON := strings.ReplaceAll(jsonStr, "\n", "")
						compactJSON = strings.ReplaceAll(compactJSON, "\t", "")
						// Remover espacios múltiples pero preservar espacios en strings
						return compactJSON, linesConsumed
					}
				}
			}
		}
	}

	// Si llegamos aquí, el JSON no está completo o hay error
	if len(jsonLines) > 0 {
		return strings.Join(jsonLines, "\n"), linesConsumed
	}
	return "", 0
}

// parseYAMLBlock extrae un bloque YAML completo
func (p *ChartParser) parseYAMLBlock(lines []string, startIndex int) (string, int) {
	var yamlLines []string
	linesConsumed := 0
	indentDetector := NewAutoDetectIndentation()

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]

		// Check if this line should be processed
		if !indentDetector.ShouldProcessLine(line, false, i+1, "CHART-YAML-PARSER") {
			break
		}

		trimmed := strings.TrimSpace(line)

		// Skip empty lines but include them in YAML
		if trimmed == "" {
			yamlLines = append(yamlLines, line)
			linesConsumed++
			continue
		}

		yamlLines = append(yamlLines, line)
		linesConsumed++
	}

	if len(yamlLines) > 0 {
		return strings.Join(yamlLines, "\n"), linesConsumed
	}
	return "", 0
}

// ChartData representa la estructura de datos para combo charts
type ChartData struct {
	Labels []string      `yaml:"labels"`
	Series []ChartSeries `yaml:"series"`
}

// ChartSeries representa una serie de datos en combo charts
type ChartSeries struct {
	Name    string        `yaml:"name"`
	Type    string        `yaml:"type"`
	Values  []interface{} `yaml:"values"`
	YAxisID string        `yaml:"yAxisID,omitempty"`
}

// ChartConfig representa la configuración completa del combo chart
type ChartConfig struct {
	Data    ChartData   `yaml:"data"`
	Options interface{} `yaml:"options,omitempty"`
}

// parseComboChartYAML parsea un combo chart usando YAML
func (p *ChartParser) parseComboChartYAML(chart *ast.ChartElement, yamlContent string) bool {
	var config ChartConfig

	err := yaml.Unmarshal([]byte(yamlContent), &config)
	if err != nil {
		return false
	}

	// Asignar labels
	chart.Labels = config.Data.Labels

	// Procesar series
	chart.Series = make([]string, len(config.Data.Series))
	chart.SeriesTypes = make([]string, len(config.Data.Series))

	// Reorganizar datos en el formato correcto para Chart.js
	// Formato: cada serie es un array de valores [[serie1_values], [serie2_values], ...]
	if len(config.Data.Series) > 0 {
		// Verificar que todas las series tengan el mismo número de valores
		seriesLength := len(config.Data.Series[0].Values)
		for _, series := range config.Data.Series {
			if len(series.Values) != seriesLength {
				return false // Inconsistencia en datos
			}
		}

		// Crear un array por cada serie con sus valores
		chart.Data = make([][]interface{}, len(config.Data.Series))
		for seriesIdx, series := range config.Data.Series {
			chart.Data[seriesIdx] = make([]interface{}, len(series.Values))
			copy(chart.Data[seriesIdx], series.Values)
		}
	}

	// Asignar nombres y tipos de series
	for i, series := range config.Data.Series {
		chart.Series[i] = series.Name
		chart.SeriesTypes[i] = series.Type
	}

	// Asignar options si existen
	if config.Options != nil {
		if options, ok := config.Options.(map[string]interface{}); ok {
			chart.Options = options
		}
	}

	return true
}

// parseInlineMatrix parsea arrays de arrays inline como [[100, 500, 350, 220]]
func (p *ChartParser) parseInlineMatrix(value string) [][]interface{} {
	var result [][]interface{}

	// Remover corchetes externos
	content := strings.TrimSpace(value)
	content = strings.TrimPrefix(content, "[")
	content = strings.TrimSuffix(content, "]")
	content = strings.TrimSpace(content)

	// Ahora tenemos algo como "[100, 500, 350, 220]" o múltiples arrays
	// Dividir por arrays individuales
	var arrays []string
	var currentArray strings.Builder
	inBrackets := 0

	for _, char := range content {
		switch char {
		case '[':
			inBrackets++
			currentArray.WriteRune(char)
		case ']':
			inBrackets--
			currentArray.WriteRune(char)
			if inBrackets == 0 {
				// Terminamos un array completo
				arrays = append(arrays, currentArray.String())
				currentArray.Reset()
			}
		case ',':
			if inBrackets == 0 {
				// Coma fuera de brackets, separador de arrays
				continue
			}
			currentArray.WriteRune(char)
		default:
			if inBrackets > 0 || char != ' ' {
				currentArray.WriteRune(char)
			}
		}
	}

	// Si queda algo en currentArray, agregarlo
	if currentArray.Len() > 0 {
		arrays = append(arrays, currentArray.String())
	}

	// Parsear cada array individual
	for _, arrayStr := range arrays {
		arrayStr = strings.TrimSpace(arrayStr)
		if arrayStr != "" {
			if row := p.parseArrayRow(arrayStr); len(row) > 0 {
				result = append(result, row)
			}
		}
	}

	return result
}

// extractAttribute extrae el valor de un atributo HTML-style del string
// Ejemplo: `bar width="1200" height="600"` → extractAttribute(..., "width") = "1200"
func extractAttribute(str, attrName string) string {
	// Buscar patrón: attrName="value" o attrName='value'
	patterns := []string{
		attrName + `="`,
		attrName + `='`,
	}

	for _, pattern := range patterns {
		idx := strings.Index(str, pattern)
		if idx == -1 {
			continue
		}

		startIdx := idx + len(pattern)
		quote := str[idx+len(attrName)+1] // " o '

		// Buscar el cierre de la comilla
		endIdx := strings.IndexRune(str[startIdx:], rune(quote))
		if endIdx == -1 {
			continue
		}

		return str[startIdx : startIdx+endIdx]
	}

	return ""
}
