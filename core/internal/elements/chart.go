// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
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

	// unknownAttrs junta los atributos de la línea de apertura que el chart no
	// conoce, para reportarlos como CHART005 más abajo. Solo se leen `width` y
	// `height`: cualquier otro par `k="v"` se ignoraba sin dejar rastro, y así
	// shipeó la plantilla `report` de `doclang init` con un
	// `<<chart:bar title="...">>` cuyo título no llegaba nunca al AST ni al
	// render. `title` va como llave del cuerpo (`title:`), no como atributo.
	var unknownAttrs []string

	// SplitN y no Split: con Split, un valor que contenga ':' (p. ej.
	// `title="Ventas: Q4"`) partía la línea en más de dos pedazos y attrStr
	// quedaba truncado en el primer ':'.
	if strings.Contains(line, ":") {
		parts := strings.SplitN(line, ":", 2)
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

			unknownAttrs = unknownOpenerAttributes(attrStr, "width", "height")
		}
	}
	chart := ast.NewChartElement(pos, chartType)
	chart.Width = width
	chart.Height = height

	// diags se arma ACÁ, no en cada return, y de eso depende que CHART005
	// llegue siempre. Parse tiene TRES salidas —JSON directo, YAML de combo, y
	// el loop de propiedades— y el aviso de atributos desconocidos vivía
	// duplicado en dos de ellas: la de combo devolvía un ParseResult sin
	// Diagnostics, así que un `<<chart: combo title="...">>` volvía a perder
	// el título en silencio, que es exactamente el defecto que CHART005
	// existe para señalar (hallazgo de code review del PR #232). Declararlo
	// una sola vez apenas parseada la apertura hace que agregar una cuarta
	// salida no pueda repetir el olvido: el dato ya está en la variable que
	// todas devuelven.
	var diags []diagnostics.Diagnostic
	if len(unknownAttrs) > 0 {
		diags = append(diags, diagnostics.NewWarning(
			fmt.Sprintf("Atributo(s) no reconocido(s) en la apertura del chart (%s); solo se aceptan 'width' y 'height' — ignorado(s). El título va como llave del cuerpo: 'title:'",
				strings.Join(unknownAttrs, ", ")),
			pos, "chart-parser").WithRuleID("CHART005"))
	}
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
					diags = append(diags, diagnostics.NewWarning(
						"El JSON del chart es inválido y fue ignorado; el chart quedará sin datos",
						pos, "chart-parser").WithRuleID("CHART002"))
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
					Diagnostics:   diags,
					Error:         nil,
				}
			}
		}
	}

	// baseIndent es la sangría de la PRIMERA línea de contenido del bloque, y
	// define qué es una llave de nivel superior del chart. Sin este dato el
	// loop de abajo era plano: escaneaba línea por línea y hacía switch sobre
	// la llave sin importar a qué profundidad estuviera, así que un bloque
	// anidado que el chart no conoce quedaba medio absorbido y medio tirado.
	// El caso real (plantilla `report` de `doclang init`):
	//
	//	<<chart:bar>>
	//	  labels: [...]
	//	  datasets:                    <- llave inexistente en el DSL
	//	    data: [85, 90, 88, 95]     <- se capturaba como si fuera top-level
	//	    backgroundColor: "#3498db" <- se descartaba en silencio
	//	<<end>>
	//
	// Acotar el switch a baseIndent arregla las dos mitades: el `data:`
	// anidado deja de robarle el lugar al de verdad, y `datasets:` queda como
	// lo que es, una llave desconocida — que ahora se reporta (CHART005) en
	// vez de evaporarse.
	//
	// Lo que hay MÁS profundo que baseIndent no se toca ni se reporta: es
	// contenido de la llave que lo abrió. Para `options:` eso es deliberado y
	// correcto (es config arbitraria de Chart.js, la captura entera
	// parseNestedOptions); para las filas de un `data: [` multilínea, el
	// índice ya viene adelantado por parseMultiLineArray y no llegan acá.
	baseIndent := -1
	var unknownKeys []string

	// TODO (bug aparte, preexistente, encontrado al barrer el corpus con
	// CHART005): este loop no se detiene donde termina el bloque. Un chart sin
	// `<<end>>` explícito —la convención de cierre por dedent que el corpus
	// usa en todos lados— sigue escaneando y CONSUMIENDO las líneas de después
	// hasta que isChartContentBoundary diga basta, y esas líneas se pierden
	// del documento. En examples/use-cases/educational/ml_fundamentals.slidelang
	// (chart de la línea 252) desaparecen del HTML renderizado tanto el
	// párrafo "**Different algorithms excel at different problems**" como el
	// bloque @notes: entero. Son 20 fixtures del corpus con la misma forma. El
	// chart sí renderiza, así que la pérdida es silenciosa. Arreglarlo es
	// tocar isChartContentBoundary y la terminación por dedent, con su propia
	// validación contra los dos corpus — no va mezclado acá.

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
		// elemento, heading) — ver isChartContentBoundary. Se pasa la línea
		// cruda (line, no trimmedLine): el chequeo de límite strict necesita
		// la sangría para no confundirse con contenido indentado.
		if isChartContentBoundary(line) {
			break
		}

		// Skip empty lines
		if trimmedLine == "" {
			consumedLines++
			continue
		}

		if baseIndent < 0 {
			baseIndent = lineIndent(line)
		}
		if lineIndent(line) > baseIndent {
			// Contenido de una llave anidada, no una llave del chart.
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
				case "options":
					// El bloque options: es YAML anidado arbitrario (la config
					// de Chart.js: plugins, scales, datalabels...), no una
					// propiedad de una línea como las de al lado, así que se
					// captura entero por sangría y se deserializa aparte.
					//
					// Hasta este parche NO existía este case: el switch solo
					// conocía data/series/labels/title/type, y todo bloque
					// options: del DSL se descartaba en silencio antes de
					// llegar al AST. Eso no era un hueco solo de PPTX —
					// también borraba el título de los charts cuyo texto vive
					// en options.plugins.title.text (que es como lo escriben
					// los ejemplos de examples/02_diagrams_and_charts/), así
					// que el HTML llevaba tiempo perdiendo títulos que el
					// autor sí había pedido.
					opts, linesConsumed := p.parseNestedOptions(ctx.Lines, i+1)
					if opts != nil {
						chart.Options = opts
					}
					i += linesConsumed
					consumedLines += linesConsumed
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
				default:
					// Llave de nivel superior que el chart no conoce. Antes
					// caía acá sin case y desaparecía sin dejar rastro; ver
					// CHART005 abajo y el comentario de baseIndent arriba.
					//
					// El filtro por forma de identificador NO es cosmético.
					// Este loop no siempre se detiene donde termina el bloque
					// (bug aparte, ver el TODO más abajo), así que a veces
					// escanea prosa del slide siguiente — y la prosa tiene dos
					// puntos: "**Overall readiness**: 68%", "@notes:",
					// "- **Metrics**: ...". Sin el filtro, CHART005 disparaba
					// sobre 20 fixtures del corpus con "llaves" como
					// "- **Traces" o cadena vacía. Un aviso que grita en
					// contenido legítimo es peor que no avisar: se aprende a
					// ignorarlo y deja de servir para el caso real
					// (`datasets:`), que sí tiene forma de identificador.
					if chartKeyRe.MatchString(key) {
						unknownKeys = append(unknownKeys, key)
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

	// CHART005: una llave de nivel superior que el chart no conoce se ignora,
	// pero se avisa. Warning y no Error, y sigue el mismo criterio que
	// FRONT005/FRONT006/FRONT007 para `toc:`/`page:`/`watermark:`: un typo o
	// una llave inventada no puede tumbar el build, pero tampoco puede
	// evaporarse sin señal. Esa evaporación es justo cómo la plantilla
	// `report` de `doclang init` shipeó con un `datasets:`/`backgroundColor:`
	// que ni el AST ni el render veían nunca.
	//
	// Solo llaves de nivel superior a propósito: dentro de `options:` va
	// config arbitraria de Chart.js (plugins, scales, datalabels...), así que
	// validar ahí dispararía sobre cada llave legítima.
	if len(unknownKeys) > 0 {
		diags = append(diags, diagnostics.NewWarning(
			fmt.Sprintf("Llave(s) no reconocida(s) en el bloque chart (%s); se esperaba 'data'/'series'/'labels'/'options'/'title'/'type' — ignorada(s). La config arbitraria de Chart.js va dentro de 'options:'",
				strings.Join(unknownKeys, ", ")),
			pos, "chart-parser").WithRuleID("CHART005"))
	}

	return &ParseResult{
		Element:       chart,
		ConsumedLines: consumedLines,
		Diagnostics:   diags,
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

	// Respeta las comillas: una etiqueta con coma (["Berlin, Germany", 45])
	// se partía en dos items, así que la fila ganaba una columna y los
	// valores numéricos quedaban corridos una posición — corrupción
	// silenciosa de los datos de la gráfica, sin diagnóstico que la delate
	// (CHART001 solo verifica que HAYA datos). Mismo bug que el splitter de
	// TABLE (ver splitInlineArray en table.go); acá el helper compartido
	// devuelve los items en bruto porque el loop de abajo necesita ver las
	// comillas para distinguir etiqueta de número.
	parts, ok := splitTopLevelCommas(content)
	if !ok {
		// Comillas desbalanceadas: split ingenuo de siempre.
		parts = strings.Split(content, ",")
	}

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

// isChartContentBoundary reporta si rawLine marca el fin del contenido de un
// chart: el cierre del propio bloque ("<</chart>>"), un separador de slide
// flex ("---"), un límite de bloque strict ("SLIDE "/"SECTION " en columna 0
// — ver IsStrictBlockBoundary), o el inicio de un nuevo elemento/sección.
// Recibe la línea SIN trim a propósito: el chequeo de límite strict necesita
// la sangría cruda para no confundir un límite real con una fila de datos
// indentada que casualmente empieza con esas palabras. Compartida entre el
// loop de propiedades y parseJSONBlock para que ambos no puedan
// desincronizarse sobre qué cuenta como límite (issue #12e2 — la revisión de
// esa misma PR encontró que el check original de parseJSONBlock solo
// reconocía 2 de los 5 límites que el loop de propiedades ya conocía; issue
// #107 — el mismo defecto de desincronización, esta vez porque ninguno de
// los dos conocía "SLIDE ", así que un chart en modo strict sin <<end>> se
// tragaba todos los slides siguientes hasta EOF).
func isChartContentBoundary(rawLine string) bool {
	if IsStrictBlockBoundary(rawLine) {
		return true
	}
	trimmed := strings.TrimSpace(rawLine)
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
			if isChartContentBoundary(line) {
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

// parseNestedOptions captura el bloque YAML anidado que sigue a "options:" y
// lo deserializa a map[string]interface{} — la misma forma que
// parseComboChartYAML ya produce para chart.Options por la ruta combo, y la
// que el renderer HTML espera para inyectarla como config de Chart.js.
//
// El bloque se delimita por sangría: pertenecen a options: todas las líneas
// más indentadas que la propia "options:", más las vacías intercaladas. Se
// dedenta al mínimo común antes de parsear porque yaml.Unmarshal rechaza un
// documento cuya primera línea ya viene sangrada.
//
// Devuelve (nil, n) si el YAML no parsea: un options: mal formado no debe
// tumbar el chart entero — se descarta la config y el chart se renderiza sin
// ella, que es exactamente el comportamiento que había antes de que este
// parser existiera.
func (p *ChartParser) parseNestedOptions(lines []string, startIndex int) (map[string]interface{}, int) {
	if startIndex >= len(lines) {
		return nil, 0
	}

	baseIndent := countLeadingSpaces(lines[startIndex])
	if baseIndent == 0 {
		// Sin sangría no hay bloque anidado que capturar.
		return nil, 0
	}

	var block []string
	consumed := 0
	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			// Una línea vacía no cierra el bloque, pero tampoco lo extiende
			// por sí sola: solo se incluye si algo más indentado la sigue.
			block = append(block, "")
			consumed++
			continue
		}
		if countLeadingSpaces(line) < baseIndent {
			break
		}
		block = append(block, line[baseIndent:])
		consumed++
	}

	// Recortar las vacías del final, que no pertenecen al bloque.
	for len(block) > 0 && strings.TrimSpace(block[len(block)-1]) == "" {
		block = block[:len(block)-1]
		consumed--
	}
	if len(block) == 0 {
		return nil, 0
	}

	var opts map[string]interface{}
	if err := yaml.Unmarshal([]byte(strings.Join(block, "\n")), &opts); err != nil {
		return nil, consumed
	}
	return opts, consumed
}

// countLeadingSpaces cuenta la sangría de una línea tratando un tab como un
// espacio — basta para comparar niveles relativos, que es lo único que
// parseNestedOptions necesita.
func countLeadingSpaces(line string) int {
	n := 0
	for _, r := range line {
		if r != ' ' && r != '\t' {
			break
		}
		n++
	}
	return n
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

	// Reorganizar datos al formato canónico fila-por-categoría que el resto
	// del repo espera de chart.Data: cada fila es [etiqueta, valor_serie0,
	// valor_serie1, ...] — el mismo shape que produce la forma plana
	// "data: [[fila],...]" + "series:" (ver examples/02_diagrams_and_charts,
	// y core/formatter/strict.go:544 sobre por qué esa es la única forma
	// combo que el round-trip strict conoce). Tanto
	// core/renderer/html.go:GenerateChartConfigWithMode como
	// slidelang/internal/generator/data/converter.go:createDatasetsFromSeries
	// leen row[i+1] de cada FILA para extraer la serie i (columna 0 se
	// descarta como la etiqueta). Antes, esta función guardaba chart.Data
	// indexado POR SERIE (chart.Data[serieIdx] = valores completos de esa
	// serie) — un layout que ningún renderer del repo entiende: con N
	// series de M valores cada una, row[i+1] leía el valor M-ésimo de la
	// serie i+1 en vez del i-ésimo de la serie i, así que Chart.js terminaba
	// con las labels sustituidas por los valores de la primera serie, la
	// primera serie corrida una serie a la derecha, y la última serie vacía
	// (fuera de rango) — un chart con datos, que pasa CHART001, pero visualmente
	// incorrecto.
	if len(config.Data.Series) > 0 {
		// Verificar que todas las series tengan el mismo número de valores
		seriesLength := len(config.Data.Series[0].Values)
		for _, series := range config.Data.Series {
			if len(series.Values) != seriesLength {
				return false // Inconsistencia en datos
			}
		}

		chart.Data = make([][]interface{}, seriesLength)
		for catIdx := 0; catIdx < seriesLength; catIdx++ {
			row := make([]interface{}, 0, len(config.Data.Series)+1)
			if catIdx < len(config.Data.Labels) {
				row = append(row, config.Data.Labels[catIdx])
			} else {
				row = append(row, "")
			}
			for _, series := range config.Data.Series {
				row = append(row, series.Values[catIdx])
			}
			chart.Data[catIdx] = row
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
// unknownOpenerAttributes devuelve los nombres de atributo `k="v"` presentes
// en attrStr que no estén en known. attrStr es lo que va después del ':' de la
// apertura, ya sin el '>>' (p. ej. `bar title="Ventas" width="1200"`); el
// primer token es el tipo del chart, no un atributo, y no se considera.
func unknownOpenerAttributes(attrStr string, known ...string) []string {
	isKnown := make(map[string]bool, len(known))
	for _, k := range known {
		isKnown[k] = true
	}

	var unknown []string
	for _, m := range openerAttrRe.FindAllStringSubmatch(attrStr, -1) {
		if name := m[1]; !isKnown[name] {
			unknown = append(unknown, name)
		}
	}
	return unknown
}

// chartKeyRe describe la forma de una llave del bloque chart: un
// identificador en minúsculas, como las seis que el parser reconoce
// (data/series/labels/options/title/type). Se usa solo para decidir si vale la
// pena reportar una llave desconocida — ver el comentario del `default:` en
// Parse.
var chartKeyRe = regexp.MustCompile(`^[a-z][a-zA-Z0-9_]*$`)

// openerAttrRe matchea un par `nombre="valor"` o `nombre='valor'` de la línea
// de apertura. Deliberadamente exige comillas: es la forma que emite el
// formatter y la única que extractAttribute sabe leer, así que un
// `width=1200` sin comillas ya se ignoraba antes de este cambio y no se
// empieza a reportar acá (sería un cambio de comportamiento distinto, no el
// hueco que se está tapando).
var openerAttrRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_-]*)\s*=\s*["'][^"']*["']`)

// lineIndent devuelve cuántos espacios/tabs de sangría trae line. Un tab
// cuenta como uno, igual que el resto del parser: lo que importa acá es
// comparar líneas del mismo bloque entre sí, no medir columnas.
func lineIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

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
