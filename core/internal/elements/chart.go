// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
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

	// Parsear propiedades del chart.
	//
	// El loop lleva etiqueta porque los cortes de abajo viven dentro de un
	// switch: un "break" pelado ahí rompe el SWITCH, no el for, y la
	// ejecución cae en el consumedLines++ del final — que es justo el bug
	// que esto arregla. Mismo motivo por el que map.go etiqueta su
	// parseLoop.
	// arrayDepth lleva la cuenta de corchetes sin cerrar que quedaron
	// abiertos por un array multi-línea; ver el bloque que lo consulta.
	arrayDepth := 0
chartLoop:
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]
		trimmedLine := strings.TrimSpace(line)
		lineStart := i

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

		// Dentro de un array multi-línea que su sub-parser dejó abierto,
		// toda línea es contenido del array, no el principio de lo que
		// sigue. parseMultiLineArray corta por sangría (ShouldProcessLine) y
		// eso lo deja corto en dos formas que el corpus sí escribe: el
		// corchete de cierre dedentado respecto a sus filas (el corte
		// dispara ANTES de su chequeo de "]"), y el array entero en columna
		// 0 (ExpectedIndent == -1 con sangría 0 devuelve false en la PRIMERA
		// fila, así que consume 0 líneas y caen todas acá).
		//
		// Se decide por PROFUNDIDAD DE CORCHETES, no por la forma de la
		// línea. Una heurística de "empieza con [" tragaba un enlace
		// Markdown legítimo después de un chart cerrado por dedent
		// ("[Ver fuente](https://example.com)"), que es exactamente la
		// pérdida silenciosa que este allowlist existe para cerrar. Con
		// arrayDepth la excepción solo aplica cuando hay un array realmente
		// abierto.
		//
		// Los límites duros (<<end>>, "---", "<<", headings) se chequean
		// ARRIBA de esto a propósito: un array sin cerrar no puede tragarse
		// el resto del documento.
		if arrayDepth > 0 {
			arrayDepth += bracketDelta(trimmedLine)
			consumedLines++
			continue
		}

		// Todo contenido del chart es una propiedad "clave: valor" del
		// allowlist de abajo. Cualquier otra cosa —prosa, "@notes:", una
		// clave desconocida— es la PRIMERA línea de después del bloque, así
		// que cierra el chart cerrado por dedent y se corta SIN consumirla,
		// para que el parser de nivel superior la procese como lo que es.
		//
		// Antes no había ninguno de estos tres cortes: la línea no
		// reconocida caía hasta el consumedLines++ del final del loop, y
		// como ConsumedLines es lo que le dice al llamador cuántas líneas
		// saltar, todo lo que el chart escaneara de más DESAPARECÍA del
		// documento sin diagnóstico (el chart en sí renderizaba bien, así
		// que la pérdida era muda). Con un <<chart>> sin <<end>> —la
		// convención documentada en spec/language-specification.md:75,
		// element_terminator ::= "<<end>>" | block_boundary | EOF— eso se
		// tragaba el párrafo y el bloque @notes: siguientes.
		//
		// El allowlist es el mecanismo que la spec (sec. "element_data",
		// misma línea) ya nombra para esta garantía, y el que map.go usa
		// desde siempre en su parseLoop. La alternativa de mermaid.go
		// (ShouldProcessLine, corte por sangría) NO sirve acá: hay charts
		// con las propiedades en columna 0 (examples/use-cases/educational/
		// machine_learning_intro.slidelang), y ahí ShouldProcessLine corta
		// en la primera línea del bloque y borra el chart entero.
		if !strings.Contains(trimmedLine, ":") {
			break chartLoop
		}
		parts := strings.SplitN(trimmedLine, ":", 2)
		if len(parts) != 2 {
			break chartLoop
		}
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
			// Una propiedad reconocida del vocabulario de charts que este
			// switch no llega a usar (datasets:, backgroundColor:, fill:...)
			// es contenido del bloque igual: se consume y se sigue. El
			// normalizador las emite y las trata como sintaxis de chart
			// (ver isChartDataLine en rules/enhancement/chart_formatter.go),
			// así que cortar en ellas dejaba el chart sin datos, disparaba
			// CHART001 y mandaba el resto del bloque a texto.
			if !isChartPropertyKey(key) {
				// Clave desconocida: no es una propiedad del chart, así que
				// la línea ya pertenece a lo que sigue al bloque. Cortar sin
				// consumirla. Espeja el "default: break parseLoop" de map.go.
				break chartLoop
			}
		}

		// Contar los corchetes de la línea de la propiedad MÁS los de lo que
		// el sub-parser haya avanzado: si queda algo abierto, las líneas que
		// siguen son del array y no límite del bloque.
		for k := lineStart; k <= i && k < len(ctx.Lines); k++ {
			arrayDepth += bracketDelta(strings.TrimSpace(ctx.Lines[k]))
		}
		if arrayDepth < 0 {
			arrayDepth = 0
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

// bracketDelta cuenta los corchetes de apertura menos los de cierre de una
// línea, ignorando los que van dentro de una cadena entre comillas (una
// etiqueta como "Ventas [MXN]" no debe alterar la cuenta). El loop de
// propiedades lo usa para saber si un array multi-línea quedó abierto.
func bracketDelta(trimmed string) int {
	depth := 0
	inString := false
	escaped := false
	for _, r := range trimmed {
		if escaped {
			escaped = false
			continue
		}
		switch {
		case r == '\\':
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// dentro de una cadena los corchetes son texto
		case r == '[':
			depth++
		case r == ']':
			depth--
		}
	}
	return depth
}

// chartPropertyKeys es el vocabulario de propiedades que el DSL reconoce como
// sintaxis de chart. El switch de Parse solo ACTÚA sobre unas pocas
// (data/series/labels/options/title/type); el resto se reconocen para poder
// consumirlas sin cortar el bloque.
//
// La lista espeja isChartDataLine en
// internal/normalize/normalizer/rules/enhancement/chart_formatter.go y
// chartProperties en internal/normalize/normalizer/detector.go — el
// normalizador emite estas claves y las trata como chart, así que el parser
// no puede tratarlas como el fin del bloque. Mantenerlas en sync.
var chartPropertyKeys = map[string]bool{
	// las que el switch sí maneja
	"data": true, "series": true, "labels": true,
	"options": true, "title": true, "type": true,
	// reconocidas por el normalizador, aún no consumidas por el switch
	"datasets": true, "backgroundColor": true, "borderColor": true,
	"borderWidth": true, "label": true, "fill": true, "tension": true,
	"pointRadius": true, "pointHoverRadius": true, "xAxisID": true,
	"yAxisID": true, "plugins": true, "legend": true, "responsive": true,
	"text": true, "display": true, "position": true,
}

// isChartPropertyKey reporta si key pertenece al vocabulario de propiedades
// de chart, es decir si la línea sigue siendo contenido del bloque aunque el
// switch no la use.
func isChartPropertyKey(key string) bool {
	return chartPropertyKeys[key]
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
