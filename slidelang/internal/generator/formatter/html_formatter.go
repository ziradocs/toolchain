// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"encoding/json"
	"regexp"
	"strings"
)

// HTMLFormatter formatea HTML generado para mejorar legibilidad
type HTMLFormatter struct {
	indentSize int
	indentChar string
}

// NewHTMLFormatter crea una nueva instancia del formatter
func NewHTMLFormatter() *HTMLFormatter {
	return &HTMLFormatter{
		indentSize: 4,   // 4 espacios por nivel de indentación
		indentChar: " ", // usar espacios en lugar de tabs
	}
}

// WithIndentSize configura el tamaño de indentación
func (f *HTMLFormatter) WithIndentSize(size int) *HTMLFormatter {
	f.indentSize = size
	return f
}

// WithIndentChar configura el carácter de indentación
func (f *HTMLFormatter) WithIndentChar(char string) *HTMLFormatter {
	f.indentChar = char
	return f
}

// FormatHTML formatea el HTML con indentación apropiada
func (f *HTMLFormatter) FormatHTML(html string) string {
	// Remover espacios en blanco excesivos
	html = f.removeExcessiveWhitespace(html)

	// Formatear con indentación HTML
	html = f.indentHTML(html)

	// Formatear JSON dentro de scripts DESPUÉS del formateo HTML
	html = f.formatJSONInScripts(html)

	// Limpiar líneas vacías múltiples
	html = f.cleanMultipleEmptyLines(html)

	return html
}

// removeExcessiveWhitespace remueve espacios y líneas vacías excesivas
func (f *HTMLFormatter) removeExcessiveWhitespace(html string) string {
	// Remover espacios al inicio y final de cada línea
	lines := strings.Split(html, "\n")
	var cleanedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		} else {
			// Preservar una línea vacía
			cleanedLines = append(cleanedLines, "")
		}
	}

	return strings.Join(cleanedLines, "\n")
}

// containerElements son los tags cuyo anidamiento afecta el nivel de
// indentación.
var containerElements = map[string]bool{
	"html":    true,
	"head":    true,
	"body":    true,
	"div":     true,
	"section": true,
	"article": true,
	"header":  true,
	"footer":  true,
	"main":    true,
	"nav":     true,
	"aside":   true,
	"ul":      true,
	"ol":      true,
	"li":      true,
	"form":    true,
	"script":  true,
	"style":   true,
}

var (
	completeTagRe         = regexp.MustCompile(`</?(\w+)[^<>]*>`)
	unterminatedOpenTagRe = regexp.MustCompile(`^<(\w+)`)
)

// indentHTML aplica indentación apropiada al HTML.
//
// A diferencia de una heurística basada en si la línea EMPIEZA con "<" o
// "</", esto cuenta aperturas/cierres de tags contenedores en cualquier
// posición de la línea y lleva un estado de "tag pendiente" para tags de
// apertura multilínea (atributos que se extienden por varias líneas). Así,
// un cierre "pegado" al final de una línea de atributos (p.ej.
// `data-x="y"></div>`) decrementa correctamente en vez de quedar invisible
// para el contador — esa invisibilidad era la causa del drift de +1 nivel
// de indentación por cada elemento mermaid/map (issue #12a).
func (f *HTMLFormatter) indentHTML(html string) string {
	lines := strings.Split(html, "\n")
	var formattedLines []string
	var indentLevel int
	pendingTag := "" // tag contenedor cuyo '>' de apertura aún no llegó

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			formattedLines = append(formattedLines, "")
			continue
		}

		if pendingTag != "" {
			// Línea de continuación de atributos de un tag multilínea: es
			// parte del mismo elemento lógico que las líneas anteriores, así
			// que se imprime al mismo nivel y no abre/cierra nada más.
			formattedLines = append(formattedLines, f.indentFor(indentLevel)+line)
			if strings.Contains(line, ">") {
				tag := pendingTag
				pendingTag = ""
				if !strings.Contains(line, "</"+tag+">") {
					indentLevel++
				}
			}
			continue
		}

		netOpen, extraCloses := f.containerNetDelta(line)
		if extraCloses > 0 {
			indentLevel = max(0, indentLevel-extraCloses)
		}

		formattedLines = append(formattedLines, f.indentFor(indentLevel)+line)

		if netOpen > 0 {
			indentLevel += netOpen
		}

		if tag := f.unterminatedContainerOpen(line); tag != "" {
			pendingTag = tag
		}
	}

	return strings.Join(formattedLines, "\n")
}

// indentFor retorna la cadena de indentación para un nivel dado.
func (f *HTMLFormatter) indentFor(level int) string {
	return strings.Repeat(f.indentChar, level*f.indentSize)
}

// containerNetDelta procesa, en orden, los tags COMPLETOS (con '>' en la
// misma línea) de elementos contenedores y retorna:
//   - netOpen: aperturas sin cerrar que quedan pendientes al final de la línea
//   - extraCloses: cierres que exceden lo abierto en la misma línea (cierran
//     algo abierto en una línea anterior)
func (f *HTMLFormatter) containerNetDelta(line string) (netOpen int, extraCloses int) {
	matches := completeTagRe.FindAllStringSubmatch(line, -1)
	depth := 0
	for _, m := range matches {
		token := m[0]
		name := strings.ToLower(m[1])
		if !containerElements[name] {
			continue
		}
		if strings.HasPrefix(token, "</") {
			if depth > 0 {
				depth--
			} else {
				extraCloses++
			}
			continue
		}
		if strings.HasSuffix(token, "/>") {
			continue // auto-cerrado, no afecta el nivel
		}
		depth++
	}
	return depth, extraCloses
}

// unterminatedContainerOpen detecta un tag de apertura de contenedor cuyo
// '>' de cierre no aparece en esta línea (sus atributos continúan en líneas
// siguientes).
func (f *HTMLFormatter) unterminatedContainerOpen(line string) string {
	lastOpen := strings.LastIndex(line, "<")
	if lastOpen == -1 {
		return ""
	}
	rest := line[lastOpen:]
	if strings.Contains(rest, ">") {
		return "" // ya se cierra en esta misma línea
	}
	if strings.HasPrefix(rest, "</") {
		return "" // un cierre nunca continúa en líneas siguientes
	}
	m := unterminatedOpenTagRe.FindStringSubmatch(rest)
	if len(m) < 2 {
		return ""
	}
	name := strings.ToLower(m[1])
	if containerElements[name] {
		return name
	}
	return ""
}

// cleanMultipleEmptyLines reduce líneas vacías múltiples a máximo 2 consecutivas
func (f *HTMLFormatter) cleanMultipleEmptyLines(html string) string {
	// Regex para encontrar 3 o más líneas vacías consecutivas
	re := regexp.MustCompile(`\n\s*\n\s*\n(\s*\n)*`)

	// Reemplazar con máximo 2 líneas vacías
	return re.ReplaceAllString(html, "\n\n")
}

// max retorna el valor máximo entre dos enteros
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// formatJSONInScripts encuentra y formatea JSON dentro de tags <script type="application/json">
func (f *HTMLFormatter) formatJSONInScripts(html string) string {
	// Regex para encontrar script tags con type="application/json" (con flag (?s) para que . incluya \n)
	re := regexp.MustCompile(`(?s)(<script[^>]*type="application/json"[^>]*>)(.*?)(</script>)`)

	// Debug: verificar si encuentra matches
	matches := re.FindAllString(html, -1)
	if len(matches) == 0 {
		// Si no encuentra nada, retornar original
		return html
	}

	return re.ReplaceAllStringFunc(html, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		openTag := parts[1]
		jsonContent := parts[2]
		closeTag := parts[3]

		// Limpiar el JSON content
		jsonContent = strings.TrimSpace(jsonContent)

		// Si el JSON está vacío, retornar tal como está
		if jsonContent == "" {
			return match
		}

		// Intentar parsear y formatear el JSON
		formattedJSON := f.formatJSONContent(jsonContent)

		// Retornar con indentación apropiada para el script tag
		scriptIndent := strings.Repeat(f.indentChar, f.indentSize*2) // indentación del script
		jsonIndent := strings.Repeat(f.indentChar, f.indentSize*3)   // indentación del JSON

		// Aplicar indentación a cada línea del JSON
		jsonLines := strings.Split(formattedJSON, "\n")
		var indentedLines []string
		for _, line := range jsonLines {
			if strings.TrimSpace(line) != "" {
				indentedLines = append(indentedLines, jsonIndent+line)
			} else {
				indentedLines = append(indentedLines, "")
			}
		}

		result := openTag + "\n" + strings.Join(indentedLines, "\n") + "\n" + scriptIndent + closeTag
		return result
	})
}

// formatJSONContent formatea una cadena JSON con indentación
func (f *HTMLFormatter) formatJSONContent(jsonStr string) string {
	// Remover espacios en blanco innecesarios
	jsonStr = strings.TrimSpace(jsonStr)

	// Intentar parsear el JSON
	var jsonData interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonData); err != nil {
		// Si no se puede parsear, intentar arreglar escapes comunes
		jsonStr = f.fixJSONEscapes(jsonStr)
		if err := json.Unmarshal([]byte(jsonStr), &jsonData); err != nil {
			// Si aún falla, retornar tal como está
			return jsonStr
		}
	}

	// Formatear con indentación (usar 4 espacios para el JSON)
	indent := strings.Repeat(f.indentChar, f.indentSize)
	formattedBytes, err := json.MarshalIndent(jsonData, "", indent)
	if err != nil {
		// Si hay error al formatear, retornar el original
		return jsonStr
	}

	result := string(formattedBytes)
	return result
}

// fixJSONEscapes arregla escapes incorrectos en JSON
func (f *HTMLFormatter) fixJSONEscapes(jsonStr string) string {
	// Arreglar escapes de comillas simples que están mal formateados
	// Cambiar \' por ' (las comillas simples no necesitan escape en JSON)
	jsonStr = strings.ReplaceAll(jsonStr, `\'`, `'`)

	// Arreglar otros escapes problemáticos
	jsonStr = strings.ReplaceAll(jsonStr, `\u000A`, `\n`)

	return jsonStr
}
