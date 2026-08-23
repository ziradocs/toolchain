// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// chartTagPattern se compila una sola vez a nivel de paquete en vez de
// dentro de extractChartType (ver docs/SECURITY_AUDIT_2026-07.md, BA-9).
var chartTagPattern = regexp.MustCompile(`<<chart:(\w+)`)

// ChartFormatterRule formatea bloques de datos de gráficos sin indentación adecuada
type ChartFormatterRule struct{}

// NewChartFormatterRule crea una nueva instancia de ChartFormatterRule
func NewChartFormatterRule() *ChartFormatterRule {
	return &ChartFormatterRule{}
}

func (r *ChartFormatterRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	var result []string
	inChartBlock := false
	blockStart := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detectar inicio de bloque de gráfico
		if strings.HasPrefix(trimmed, "<<chart:") {
			inChartBlock = true
			blockStart = i
			result = append(result, line)
			continue
		}

		// Si estamos en un bloque de gráfico
		if inChartBlock {
			// Detectar fin del bloque de gráfico - mejorado para detectar >> correctamente
			if trimmed == ">>" || (r.isEndOfChartBlock(trimmed, lines, i) && !r.isChartDataLine(trimmed)) {
				inChartBlock = false
				// Procesar el bloque completo
				chartLines := lines[blockStart+1 : i]
				// Solo formatear si no está ya formateado
				if r.needsFormatting(chartLines) {
					formattedLines := r.formatChartData(chartLines)
					result = append(result, formattedLines...)
				} else {
					// Mantener el formato original
					result = append(result, chartLines...)
				}
				result = append(result, line)
				continue
			}
			// No agregar líneas aquí, se procesarán al final del bloque
			continue
		}

		// Líneas normales fuera de bloques de gráfico
		result = append(result, line)
	}

	// Si terminamos y todavía estamos en un bloque de gráfico
	if inChartBlock && blockStart != -1 {
		chartLines := lines[blockStart+1:]
		if r.needsFormatting(chartLines) {
			formattedLines := r.formatChartData(chartLines)
			result = append(result, formattedLines...)
		} else {
			result = append(result, chartLines...)
		}
	}

	return strings.Join(result, "\n"), nil
}

// needsFormatting determina si un bloque de gráfico necesita formateo
func (r *ChartFormatterRule) needsFormatting(lines []string) bool {
	// Verificar si el content ya tiene el formato correcto esperado por SlideLang
	// Un chart bien formateado debería tener:
	// - Propiedades principales con 2 espacios de indentación
	// - Propiedades dentro de datasets: con 4 espacios de indentación

	// Contar cuántas líneas siguen el patrón esperado
	correctlyFormattedLines := 0
	totalNonEmptyLines := 0
	inDatasetsContext := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		totalNonEmptyLines++

		// Detectar cuando entramos o salimos del contexto datasets
		if trimmed == "datasets:" {
			inDatasetsContext = true
			// Check if datasets: has correct indentation (2 spaces)
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
				correctlyFormattedLines++
			}
			continue
		}

		// El sub-bloque de options: es YAML arbitrario de Chart.js (scales,
		// elements, interaction, cualquier plugin...) — no tiene sentido
		// juzgarlo contra isSubProperty, que solo conoce 6 nombres (issue
		// #153: eso era justo lo que hacía que un chart YA bien indentado
		// pudiera seguir cayendo bajo el 0.7 y ser reescrito). Se cuenta la
		// línea "options:" misma y se salta entero lo que cuelga de ella,
		// ni suma ni resta al ratio.
		if trimmed == "options:" {
			inDatasetsContext = false
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
				correctlyFormattedLines++
			}
			i = r.skipOptionsBlock(lines, i)
			continue
		}

		// "data:" sin valor (ni "[" de array multilínea) es el formato YAML
		// anidado que ChartParser.parseComboChartYAML espera para combo
		// charts (data.labels + data.series, ver core/internal/elements/
		// chart.go) — mismo caso que options:, un sub-árbol YAML arbitrario
		// que isMainProperty/isSubProperty (tablas de nombres fijos) no
		// pueden juzgar. Sin este bypass, "series:"/"labels:" anidados bajo
		// data: se contaban como propiedades de nivel top (isMainProperty
		// las reconoce por nombre sin mirar el contexto) y arrastraban el
		// ratio por debajo de 0.7 aunque el bloque ya estuviera bien
		// formado — disparando formatChartData, que sí lo aplanaba y
		// rompía el chart (ver el bypass simétrico más abajo).
		if trimmed == "data:" {
			inDatasetsContext = false
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
				correctlyFormattedLines++
			}
			i = r.skipOptionsBlock(lines, i)
			continue
		}

		// Si encontramos otra propiedad principal, salimos del contexto datasets
		if inDatasetsContext && r.isMainProperty(trimmed) && trimmed != "data:" {
			inDatasetsContext = false
		}

		// Propiedades principales deberían empezar con 2 espacios (excepto si están dentro de datasets)
		if !inDatasetsContext && r.isMainProperty(trimmed) {
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
				correctlyFormattedLines++
			}
			continue
		}

		// Propiedades dentro de datasets deberían tener 4 espacios
		if inDatasetsContext {
			if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") {
				correctlyFormattedLines++
			}
			continue
		}

		// Sub-propiedades deberían tener más indentación
		if r.isSubProperty(trimmed) && strings.HasPrefix(line, "    ") {
			correctlyFormattedLines++
			continue
		}

		// Arrays y elementos de datos pueden tener indentación variable pero consistente
		if (strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "]") ||
			strings.HasPrefix(trimmed, "\"") || strings.Contains(trimmed, ",")) &&
			strings.HasPrefix(line, "  ") {
			correctlyFormattedLines++
			continue
		}
	}

	// Si menos del 70% de las líneas están correctamente formateadas, necesita formateo
	if totalNonEmptyLines == 0 {
		return false
	}

	ratio := float64(correctlyFormattedLines) / float64(totalNonEmptyLines)
	return ratio < 0.7
}

// isMainProperty verifica si una línea es una propiedad principal del chart
// Note: This is used by needsFormatting, separate from isMainChartProperty used for formatting
func (r *ChartFormatterRule) isMainProperty(line string) bool {
	mainProperties := []string{"series:", "options:", "type:", "title:", "labels:", "datasets:"}
	for _, prop := range mainProperties {
		if strings.HasPrefix(line, prop) {
			return true
		}
	}
	return false
}

// isSubProperty verifica si una línea es una sub-propiedad
func (r *ChartFormatterRule) isSubProperty(line string) bool {
	subProperties := []string{"plugins:", "title:", "display:", "text:", "legend:", "responsive:"}
	for _, prop := range subProperties {
		if strings.HasPrefix(line, prop) {
			return true
		}
	}
	return false
}

// extractChartType extrae el tipo de gráfico del tag de apertura
func (r *ChartFormatterRule) extractChartType(line string) string {
	matches := chartTagPattern.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// isEndOfChartBlock determina si una línea marca el final de un bloque de gráfico
func (r *ChartFormatterRule) isEndOfChartBlock(trimmed string, lines []string, index int) bool {
	// Final explícito con >>
	if trimmed == ">>" {
		return true
	}

	// Línea vacía seguida de contenido que no es de gráfico
	if trimmed == "" && index+1 < len(lines) {
		nextLine := strings.TrimSpace(lines[index+1])
		return !r.isChartDataLine(nextLine) && nextLine != ""
	}

	// Nueva sección/slide
	if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "##") {
		return true
	}

	// Otro bloque SlideLang
	if strings.HasPrefix(trimmed, "<<") {
		return true
	}

	return false
}

// isChartDataLine determina si una línea contiene datos de gráfico
func (r *ChartFormatterRule) isChartDataLine(line string) bool {
	chartDataPrefixes := []string{
		"title:",
		"labels:",
		"datasets:",
		"data:",
		"backgroundColor:",
		"borderColor:",
		"borderWidth:",
		"type:",
		"label:",
		"fill:",
		"tension:",
		"pointRadius:",
		"pointHoverRadius:",
		"xAxisID:",
		"yAxisID:",
		"series:",
		"options:",
		"plugins:",
		"legend:",
		"responsive:",
		"text:",
		"display:",
		"position:",
	}

	trimmed := strings.TrimSpace(line)

	// Verificar prefijos específicos de chart
	for _, prefix := range chartDataPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	// Arrays with brackets
	if strings.HasPrefix(trimmed, "[") {
		return true
	}

	// List items (YAML array style)
	if strings.HasPrefix(trimmed, "-") && strings.Contains(trimmed, ":") {
		return true
	}

	// List items without properties (simple list)
	if strings.HasPrefix(trimmed, "- ") {
		return true
	}

	// String values in quotes
	if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
		return true
	}

	return false
}

// formatChartData formatea las líneas de datos del gráfico con indentación apropiada
func (r *ChartFormatterRule) formatChartData(lines []string) []string {
	var result []string
	baseIndent := "  " // 2 espacios de indentación base
	inArrayContext := false
	inDatasetsContext := false
	arrayDepth := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Saltar líneas vacías
		if trimmed == "" {
			result = append(result, "")
			continue
		}

		// El sub-bloque de options: se copia preservando su indentación
		// relativa ORIGINAL en vez de reconstruirse desde una tabla de
		// nombres — ver appendOptionsBlock para el porqué (issue #153).
		if trimmed == "options:" && !inArrayContext {
			result = append(result, baseIndent+"options:")
			consumed := r.appendOptionsBlock(&result, lines, i, baseIndent)
			i += consumed
			inArrayContext = false
			inDatasetsContext = false
			arrayDepth = 0
			continue
		}

		// "data:" sin valor es el YAML anidado de combo charts
		// (data.labels + data.series) — mismo passthrough que options: y
		// por la misma razón: reconstruirlo desde calculateIndentLevelSemantic
		// trata "labels:"/"series:" anidados como propiedades de nivel top
		// (son isMainChartProperty por nombre) y los aplana a hermanas de
		// data:, destruyendo la estructura que parseComboChartYAML necesita.
		if trimmed == "data:" && !inArrayContext {
			result = append(result, baseIndent+"data:")
			consumed := r.appendOptionsBlock(&result, lines, i, baseIndent)
			i += consumed
			inArrayContext = false
			inDatasetsContext = false
			arrayDepth = 0
			continue
		}

		// Determinar el contexto y nivel de indentación basado en el contenido
		indentLevel := r.calculateIndentLevelSemantic(trimmed, &inArrayContext, &inDatasetsContext, &arrayDepth)

		// Formatear la línea con la indentación apropiada
		formattedLine := r.buildIndentedLine(trimmed, indentLevel, baseIndent)
		result = append(result, formattedLine)
	}

	return result
}

// appendOptionsBlock copia el sub-bloque options: preservando su
// profundidad relativa ORIGINAL, en vez de reconstruirla desde una tabla de
// 4 nombres (issue #153: options: es YAML arbitrario de Chart.js — scales,
// elements, interaction, cualquier plugin — y una tabla de nombres fijos no
// puede representarlo; cualquier clave fuera de esa tabla salía aplanada a
// hermana de sus propios padres).
//
// ChartParser.parseNestedOptions (core/internal/elements/chart.go) toma su
// propia profundidad de la PRIMERA línea hija, no de la línea "options:", y
// solo compara indentación relativa (countLeadingSpaces no distingue tabs de
// espacios a propósito). Por eso "copiar verbatim" sería incorrecto: si el
// bloque original venía a 2 espacios y "options:" se reemite a nivel 1 (2
// espacios), ambos quedan en la misma columna y el parser se traga toda
// propiedad de primer nivel que venga después. Lo correcto es desplazar TODO
// el sub-bloque por una constante — delta = 4 menos la indentación original
// de su primera línea hija — de forma que esa primera línea quede exactamente
// en nivel 2 (4 espacios) y el resto conserve su profundidad relativa exacta:
// new_indent(línea) = old_indent(línea) + delta para cada línea del bloque,
// así que la diferencia entre cualquier par de líneas no cambia. El límite
// del sub-bloque se calcula igual que el parser: todo lo que esté MÁS
// indentado que la línea "options:" original (blancas intercaladas
// incluidas), recortando las blancas del final.
//
// lines[startIdx] es la línea "options:"; startIdx+1 en adelante es el
// cuerpo. Devuelve cuántas líneas de body se consumieron, para que el loop
// que llama pueda saltarlas.
func (r *ChartFormatterRule) appendOptionsBlock(result *[]string, lines []string, startIdx int, baseIndent string) int {
	optionsLineIndent := countLeadingSpaces(lines[startIdx])

	end := startIdx + 1
	for end < len(lines) {
		if strings.TrimSpace(lines[end]) == "" {
			end++
			continue
		}
		if countLeadingSpaces(lines[end]) <= optionsLineIndent {
			break
		}
		end++
	}
	// Recortar las líneas vacías del final: no pertenecen al bloque, las
	// procesará el loop normal como separador con lo que venga después.
	for end > startIdx+1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	if end == startIdx+1 {
		return 0
	}

	firstChildIndent := -1
	for j := startIdx + 1; j < end; j++ {
		if strings.TrimSpace(lines[j]) != "" {
			firstChildIndent = countLeadingSpaces(lines[j])
			break
		}
	}
	if firstChildIndent == -1 {
		// Solo blancas: nada que preservar.
		return end - startIdx - 1
	}

	const optionsBodyIndent = 4 // nivel 2 (2 * "  ")
	delta := optionsBodyIndent - firstChildIndent

	for j := startIdx + 1; j < end; j++ {
		if strings.TrimSpace(lines[j]) == "" {
			*result = append(*result, "")
			continue
		}
		newIndent := countLeadingSpaces(lines[j]) + delta
		if newIndent < 0 {
			newIndent = 0
		}
		*result = append(*result, strings.Repeat(" ", newIndent)+strings.TrimLeft(lines[j], " \t"))
	}

	return end - startIdx - 1
}

// skipOptionsBlock encuentra el mismo límite de sub-bloque que
// appendOptionsBlock (todo lo más indentado que la línea "options:" en
// startIdx, blancas intercaladas incluidas, sin las blancas finales) pero
// sin reformatear nada — needsFormatting lo usa para excluir esas líneas de
// su ratio en vez de juzgarlas contra isSubProperty. Devuelve el índice de la
// última línea del sub-bloque (o startIdx si no hay body), para que el loop
// que llama pueda saltarlo con i = skipOptionsBlock(...); continue.
func (r *ChartFormatterRule) skipOptionsBlock(lines []string, startIdx int) int {
	optionsLineIndent := countLeadingSpaces(lines[startIdx])

	end := startIdx + 1
	for end < len(lines) {
		if strings.TrimSpace(lines[end]) == "" {
			end++
			continue
		}
		if countLeadingSpaces(lines[end]) <= optionsLineIndent {
			break
		}
		end++
	}
	for end > startIdx+1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end - 1
}

// countLeadingSpaces cuenta espacios y tabs iniciales para comparar
// profundidad relativa — mismo criterio que su análoga no exportada en
// core/internal/elements/chart.go (paquetes distintos, no se puede reusar
// directamente: countLeadingSpaces ahí tampoco distingue tabs de espacios,
// porque lo único que le importa a parseNestedOptions es el orden relativo).
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

// calculateIndentLevelSemantic calcula el nivel de indentación basado en el
// contexto semántico. options: nunca llega hasta acá — se intercepta antes,
// en formatChartData, porque su contenido es YAML arbitrario que esta
// función no puede reconstruir de forma segura (ver appendOptionsBlock).
func (r *ChartFormatterRule) calculateIndentLevelSemantic(line string, inArray *bool, inDatasets *bool, arrayDepth *int) int {
	// Handle data property specially so multi-line arrays keep their context
	if strings.HasPrefix(line, "data:") {
		trimmedData := strings.TrimSpace(line)

		if strings.HasSuffix(trimmedData, "[") && !strings.Contains(trimmedData, "]") {
			// data: [  -> start of multi-line array
			*inArray = true
			*arrayDepth = 1
		} else {
			// Inline array or scalar value
			*inArray = false
			*arrayDepth = 0
		}

		if *inDatasets {
			return 2
		}
		return 1
	}

	// First check if we're in datasets context and this is a child property
	if *inDatasets && !r.isMainChartProperty(line) {
		// Properties within datasets go to level 2
		return 2
	}

	// Propiedades principales del chart (nivel 1) - solo si no estamos en contexto anidado
	if r.isMainChartProperty(line) && !*inArray {
		// Reset contexts when we hit a main property
		*inArray = false

		// Check if this is datasets: to set the context
		if line == "datasets:" {
			*inDatasets = true
		} else {
			// Any other main property ends the datasets context
			*inDatasets = false
		}

		*arrayDepth = 0

		// Caso especial: "data: [" debe activar inArray
		if strings.HasPrefix(line, "data:") && strings.HasSuffix(line, "[") {
			*inArray = true
			*arrayDepth = 1
		}

		return 1
	}

	// Manejo especial para arrays
	if strings.HasPrefix(line, "[") && !strings.Contains(line, ",") {
		// Es el inicio del array (solo "[")
		*inArray = true
		*arrayDepth = 1
		return 1
	}

	if *inArray {
		// Elementos dentro del array
		if strings.HasPrefix(line, "[") && strings.Contains(line, ",") {
			return 2 // Elementos del array van en nivel 2
		}
		if strings.HasPrefix(line, "]") && !strings.Contains(line, ",") {
			// Es el cierre del array (solo "]")
			*inArray = false
			*arrayDepth = 0
			return 1
		}
		// Líneas dentro del array (elementos con formato ["item", value])
		return 2
	}

	// Properties directas del chart que no son principales (como series cuando no estamos en array)
	if strings.Contains(line, ":") && !*inArray {
		return 1
	}

	// Default para líneas que no tienen contexto específico
	return 1
}

// isMainChartProperty verifica si es una propiedad principal del chart (top-level)
// These properties should be at indent level 1 when not in a nested context
func (r *ChartFormatterRule) isMainChartProperty(line string) bool {
	mainProps := []string{"series:", "options:", "type:", "title:", "labels:", "datasets:"}
	for _, prop := range mainProps {
		if strings.HasPrefix(line, prop) {
			return true
		}
	}
	return false
}

// buildIndentedLine construye una línea con la indentación apropiada
func (r *ChartFormatterRule) buildIndentedLine(line string, level int, baseIndent string) string {
	indent := strings.Repeat(baseIndent, level)
	return indent + line
}

// CalculateIndentLevelSemanticPublic expone calculateIndentLevelSemantic para testing
func (r *ChartFormatterRule) CalculateIndentLevelSemanticPublic(line string, inArray *bool, inDatasets *bool, arrayDepth *int) int {
	return r.calculateIndentLevelSemantic(line, inArray, inDatasets, arrayDepth)
}

// NeedsFormattingPublic expone needsFormatting para testing
func (r *ChartFormatterRule) NeedsFormattingPublic(lines []string) bool {
	return r.needsFormatting(lines)
}

func (r *ChartFormatterRule) Description() string {
	return "Formatea bloques de datos de gráficos con indentación apropiada para AI-generated content"
}

func (r *ChartFormatterRule) Priority() int {
	return 5 // Prioridad alta para ejecutar antes de otros formatters
}

func (r *ChartFormatterRule) Category() base.RuleCategory {
	return base.CategoryEnhancement
}
