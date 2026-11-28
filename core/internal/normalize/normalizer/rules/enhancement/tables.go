// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"
	"unicode"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
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
	// Qué líneas están dentro de un bloque especial, de una vez: preguntarlo
	// línea por línea (isInSpecialBlock) recorría el documento desde el
	// principio en cada una, y la regla era cuadrática en el largo del
	// documento (30 000 líneas, 6 s solo en esto). Las líneas no cambian
	// durante el recorrido: al reemplazar una tabla se sale del ciclo.
	special := specialBlockLines(lines)
	inTable := tableRowLines(lines, special)

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
		if special[i] {
			continue
		}

		// Buscar listas estructuradas que sugieren tablas
		if r.looksLikeTableRows(lines, i) {
			tableData, consumed := r.extractTableData(lines, i)
			if len(tableData) >= 3 { // Al menos 3 filas para justificar una tabla
				// La regla arma una sola tabla por pasada: la primera
				// candidata del documento termina la pasada, la convierta
				// o no. En los dos casos de abajo la deja como está, pero
				// la pasada termina igual, para que una lista que venga
				// después no se convierta donde antes no se convertía.
				if inTable[i] {
					// Una fila de una tabla que el parser ya empezó a leer
					// arriba. Convertirla le metía un encabezado y un
					// separador propios a media tabla, o, con filas de una
					// sola celda ("| r0 |", dos pipes, que el chequeo de
					// arriba no salta), la borraba.
					break
				}
				markdownTable := r.convertToMarkdownTable(tableData)
				if markdownTable == "" && hasText(tableData) {
					// Menos de dos columnas: no hay tabla que armar.
					// Reemplazarlas por la cadena vacía perdía su texto.
					break
				}

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

// tableRowLines dice, para cada línea, si es una fila de una tabla Markdown
// cuyo encabezado el parser ya leyó en una línea de arriba. Calca a
// TableParser (core/internal/elements/table.go): la tabla empieza en una
// línea que abre con "|" y tiene dos pipes o más (CanParse) y sigue mientras
// cada línea abra con "|" (readMarkdownTable). Antes del encabezado el parser
// salta una línea separadora, una línea con guiones que no es un encabezado
// recuperado (su separador, con el mismo número de celdas, va en la línea
// siguiente) y una línea sin celdas; la primera línea que no salta es el
// encabezado, la primera línea de la tabla, donde la regla convierte como
// siempre. Una línea con un solo pipe no abre tabla: es texto. Las líneas de
// un bloque especial (cerca de código, etiqueta <<...>>) y de un bloque $$
// (MathParser) no son de ninguna tabla.
func tableRowLines(lines []string, special []bool) []bool {
	out := make([]bool, len(lines))
	started := false // el parser ya abrió una tabla en una línea de arriba
	header := false  // y ya leyó su encabezado
	inMath := false  // dentro de un bloque $$ de varias líneas
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if inMath {
			inMath = !strings.HasSuffix(line, "$$")
			started, header = false, false
			continue
		}
		if special[i] || !strings.HasPrefix(line, "|") {
			started, header = false, false
			if !special[i] && strings.HasPrefix(line, "$$") {
				rest := strings.TrimSpace(strings.TrimPrefix(line, "$$"))
				inMath = rest == "" || !strings.HasSuffix(rest, "$$")
			}
			continue
		}
		if !started {
			if strings.Count(line, "|") < 2 {
				continue
			}
			started = true
		}
		if header {
			out[i] = true
			continue
		}
		next := ""
		if i+1 < len(lines) {
			next = strings.TrimSpace(lines[i+1])
		}
		header = !skippedBeforeHeader(line, next)
	}
	return out
}

// skippedBeforeHeader dice si el parser salta line cuando todavía no leyó el
// encabezado de la tabla (readMarkdownTable): una línea separadora
// (IsMarkdownSeparatorLine), una línea con "---", ":-:", ":--" o "--:" que
// no sigue un separador con su mismo número de celdas (legacyMarkdownSeparator,
// sin encabezado recuperado) o una línea sin celdas.
func skippedBeforeHeader(line, next string) bool {
	if isSeparatorLine(line) {
		return true
	}
	cells := rowCells(line)
	if len(cells) == 0 {
		return true
	}
	if strings.Contains(line, "---") || strings.Contains(line, ":-:") ||
		strings.Contains(line, ":--") || strings.Contains(line, "--:") {
		recovered := strings.HasPrefix(next, "|") && isSeparatorLine(next) && len(rowCells(next)) == len(cells)
		return !recovered
	}
	return false
}

// isSeparatorLine es IsMarkdownSeparatorLine (core/internal/elements/table.go).
func isSeparatorLine(line string) bool {
	line = strings.TrimSpace(line)
	for _, r := range line {
		if r != '|' && r != '-' && r != ':' && !unicode.IsSpace(r) {
			return false
		}
	}
	if strings.Contains(line, "---") {
		return true
	}
	return strings.HasPrefix(line, "|") &&
		(strings.Contains(line, ":-:") || strings.Contains(line, ":--") || strings.Contains(line, "--:"))
}

// rowCells parte una fila como SplitMarkdownTableRow y TrimMarkdownRowEdges
// para contar sus celdas: un "|" escapado con una barra invertida, o dentro
// de un code span, no separa celdas, y las celdas vacías de los bordes no
// cuentan.
func rowCells(line string) []string {
	line = strings.TrimSpace(line)
	var cells []string
	var cur strings.Builder
	inSpan, escaped := false, false
	for _, r := range line {
		switch {
		case escaped:
			escaped = false
			cur.WriteRune(r)
		case r == '\\':
			escaped = true
			cur.WriteRune(r)
		case r == '`':
			inSpan = !inSpan
			cur.WriteRune(r)
		case r == '|' && !inSpan:
			cells = append(cells, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	cells = append(cells, cur.String())
	if strings.HasPrefix(line, "|") && len(cells) > 0 && strings.TrimSpace(cells[0]) == "" {
		cells = cells[1:]
	}
	if strings.HasSuffix(line, "|") && len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// hasText dice si alguna celda de rows tiene texto.
func hasText(rows [][]string) bool {
	for _, row := range rows {
		for _, cell := range row {
			if cell != "" {
				return true
			}
		}
	}
	return false
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

// isContainerFence informa si line (ya sin espacios en los extremos) abre o
// cierra un bloque contenedor ":::" — "::: grid", "::: column", ":::note", o
// el ":::" a secas que cierra a cualquiera de ellos.
//
// Esas líneas son frontera de bloque, nunca datos de tabla. Sin este corte,
// una tabla markdown pegada a "::: column" (sin línea en blanco de por medio)
// entraba en la ventana de looksLikeTableData junto con las dos líneas de
// marcadores — que no tienen patrón pero cuentan en `count` sin romper
// `consistentPattern` — y extractTableData partía cada marcador por ":"
// (SplitN) en una fila falsa: "::: grid" → ["", ":: grid"], ":::" → ["", "::"].
// El resultado era una única tabla de nivel slide en lugar de un grid.
func isContainerFence(line string) bool {
	return strings.HasPrefix(line, ":::")
}

// specialBlockLines dice, para cada línea de lines, lo mismo que
// isInSpecialBlock(lines, i), en una sola pasada: el estado de la cerca
// antes de cada línea y, para las etiquetas <<...>>, la última línea "---" o
// <<...>> anterior y si hubo una línea vacía desde ella.
func specialBlockLines(lines []string) []bool {
	out := make([]bool, len(lines))
	inFence := false
	tag, blank := false, false // la última marca es una etiqueta; hubo una línea vacía después
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		// Lo de antes de la línea i: la cerca abierta o, si no, la etiqueta
		// sin una línea vacía o "---" entre ella y la línea i inclusive.
		out[i] = inFence || (tag && !blank && line != "" && line != "---")
		if inFence {
			if line == "```" {
				inFence = false
			}
		} else if strings.HasPrefix(line, "```") {
			inFence = true
		}
		switch {
		case line == "---":
			tag = false
		case strings.HasPrefix(line, "<<") && strings.HasSuffix(line, ">>"):
			tag, blank = true, false
		case line == "":
			blank = true
		}
	}
	return out
}

// looksLikeTableData verifica si una serie de líneas parece datos de tabla
func (r *TablesRule) looksLikeTableData(lines []string, startIndex int) bool {
	// Verificar que no estamos dentro de un bloque especial
	if r.isInSpecialBlock(lines, startIndex) {
		return false
	}
	return r.looksLikeTableRows(lines, startIndex)
}

// looksLikeTableRows es looksLikeTableData para una línea que ya se sabe que
// no está dentro de un bloque especial.
func (r *TablesRule) looksLikeTableRows(lines []string, startIndex int) bool {
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
		if line == "" || line == "---" || isContainerFence(line) {
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

	// Priorizar pipes (más común en tablas). Issue #174: exigir "|" inicial,
	// igual que TableParser.CanParse (core/internal/elements/table.go) y
	// TextParser.isOtherElementType — sin esto, 3+ líneas de prosa seguidas
	// con 2+ "|" cada una (p. ej. varios renglones de "**A:** 1 | **B:** 2")
	// se reescribían en una tabla markdown falsa antes de que el parser
	// corriera.
	if strings.HasPrefix(line, "|") && pipeCount >= 2 {
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
		if line == "" || isContainerFence(line) {
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
