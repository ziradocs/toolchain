// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/internal/normalize/normalizer/base"
)

// Patrones compilados una sola vez a nivel de paquete — chartBlockPattern en
// particular se recompilaba dentro del loop por línea de Apply, el peor caso
// de los hallados (ver docs/SECURITY_AUDIT_2026-07.md, BA-9).
//
// Los comentarios "//" y "/* */" ya NO se remueven vía regexp (ver
// cleanJSONComments): una regexp ciega no distingue un "//" que forma
// parte de un valor string (p.ej. una URL) de un comentario real, y
// terminaba corrompiendo JSON válido (#108, #110).
var (
	jsonTrailingCommaObj = regexp.MustCompile(`,\s*}`)
	jsonTrailingCommaArr = regexp.MustCompile(`,\s*]`)
	chartBlockPattern    = regexp.MustCompile(`^(<<chart:\s*\w+>>)\s*$`)
)

// ChartJSONRule limpia comentarios inline en JSON para charts si es necesario
type ChartJSONRule struct{}

// NewChartJSONRule crea una nueva instancia de la regla
func NewChartJSONRule() *ChartJSONRule {
	return &ChartJSONRule{}
}

// cleanJSONComments limpia comentarios que Chart.js no soporta.
//
// Es "quote-aware": recorre el texto byte a byte (los delimitadores que nos
// interesan — ", \, /, *, \n, \r — son todos ASCII de 1 byte, y ningún byte
// de continuación/inicio de una secuencia UTF-8 multibyte puede coincidir
// con ellos, así que escanear por byte es tan correcto como escanear por
// rune para este propósito, sin el costo de convertir todo el string a
// []rune) llevando el estado de si estamos dentro de un string JSON
// (respetando escapes \") para NO tratar como comentario un "//" que en
// realidad forma parte de un valor string (p.ej.
// "source": "https://example.com/data"). Antes se usaba una regexp ciega
// (jsLineCommentPattern) que truncaba cualquier "//" sin importar si estaba
// dentro de comillas, corrompiendo URLs (#108, #110).
//
// Un comentario de bloque "/* ... */" sin cierre NO se trata como
// comentario — se preserva tal cual, igual que hacía la regexp original
// (`/\*[\s\S]*?\*/`, que requiere un cierre explícito para siquiera hacer
// match). Descartar todo hasta el final del string ante un "/*" mal
// formado borraría en silencio JSON válido que viniera después.
func (r *ChartJSONRule) cleanJSONComments(jsonStr string) string {
	var sb strings.Builder
	sb.Grow(len(jsonStr))

	n := len(jsonStr)
	inString := false
	escaped := false

	for i := 0; i < n; i++ {
		c := jsonStr[i]

		if inString {
			sb.WriteByte(c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			sb.WriteByte(c)
			continue
		}

		// Comentario de línea "//" (solo fuera de strings)
		if c == '/' && i+1 < n && jsonStr[i+1] == '/' {
			j := i + 2
			for j < n && jsonStr[j] != '\n' && jsonStr[j] != '\r' {
				j++
			}
			i = j - 1
			continue
		}

		// Comentario de bloque "/* ... */" (solo fuera de strings)
		if c == '/' && i+1 < n && jsonStr[i+1] == '*' {
			j := i + 2
			for j+1 < n && (jsonStr[j] != '*' || jsonStr[j+1] != '/') {
				j++
			}
			if j+1 < n {
				// Se encontró el cierre "*/"
				i = j + 1
				continue
			}
			// Sin cierre: no es un comentario real, cae al WriteByte de
			// abajo y se preserva el "/" (y el resto del texto se procesa
			// normalmente carácter por carácter, sin borrar nada).
		}

		sb.WriteByte(c)
	}

	result := sb.String()

	// Remover comas finales antes de } o ] (JSON estricto)
	result = jsonTrailingCommaObj.ReplaceAllString(result, "}")
	result = jsonTrailingCommaArr.ReplaceAllString(result, "]")

	return result
}

// Apply solo limpia comentarios, mantiene el JSON como JSON nativo
func (r *ChartJSONRule) Apply(content string) (string, error) {
	// Buscar bloques chart seguidos de JSON con comentarios
	lines := strings.Split(content, "\n")
	var result []string
	i := 0
	modified := false

	for i < len(lines) {
		line := lines[i]

		// Buscar patrón <<chart: tipo>>
		chartMatch := chartBlockPattern.FindStringSubmatch(strings.TrimSpace(line))
		if chartMatch != nil {
			result = append(result, line)
			i++

			// Buscar inicio de bloque JSON
			if i < len(lines) {
				nextLine := strings.TrimSpace(lines[i])
				if strings.HasPrefix(nextLine, "{") {
					// Procesar bloque JSON completo
					jsonBlock, endIndex, hasComments := r.extractJSONBlock(lines, i)
					if hasComments {
						// Limpiar comentarios del bloque completo
						cleanedJSON := r.cleanJSONComments(jsonBlock)
						// Dividir de nuevo en líneas y agregar
						cleanedLines := strings.Split(cleanedJSON, "\n")
						result = append(result, cleanedLines...)
						modified = true
						i = endIndex + 1
						continue
					} else {
						// No hay comentarios, procesar normalmente
						for j := i; j <= endIndex; j++ {
							result = append(result, lines[j])
						}
						i = endIndex + 1
						continue
					}
				}
			}
		}

		result = append(result, line)
		i++
	}

	if modified {
		return strings.Join(result, "\n"), nil
	}
	return content, nil
}

// extractJSONBlock extrae un bloque JSON completo desde el índice dado.
//
// El conteo de llaves y la detección de comentarios comparten un único
// estado "¿estamos dentro de un string?" (con el mismo manejo de escapes \"
// que cleanJSONComments) que se mantiene A TRAVÉS de todo el bloque, línea
// por línea, sin reiniciarse — antes, el conteo de llaves era ciego a los
// strings (una llave literal dentro de un valor, p.ej.
// "note": "ver apéndice }", decrementaba el contador y cortaba el bloque
// JSON prematuramente) y la detección de comentarios reiniciaba su estado
// en cada línea por separado (podía perder un "//" real si un valor string
// contenía un salto de línea). Ambos son la misma clase de bug que #108/
// #110: un escaneo que no distingue contenido dentro de un string.
func (r *ChartJSONRule) extractJSONBlock(lines []string, startIndex int) (string, int, bool) {
	var jsonLines []string
	braceCount := 0
	hasComments := false
	endIndex := startIndex

	inString := false
	escaped := false

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		jsonLines = append(jsonLines, line)

		n := len(line)
		for j := 0; j < n; j++ {
			c := line[j]

			if inString {
				switch {
				case escaped:
					escaped = false
				case c == '\\':
					escaped = true
				case c == '"':
					inString = false
				}
				continue
			}

			switch c {
			case '"':
				inString = true
			case '{':
				braceCount++
			case '}':
				braceCount--
				if braceCount == 0 {
					endIndex = i
					// El bloque ya cerró, pero la llave de cierre puede
					// llevar un comentario en la misma línea (p.ej.
					// "} // nota" o "} /* nota */") — sin este escaneo del
					// resto de la línea, ese comentario nunca se detecta
					// (hasComments queda false) y Apply no limpia esa línea,
					// dejando el comentario en el JSON final.
					for k := j + 1; k < n; k++ {
						if line[k] == '/' && k+1 < n && (line[k+1] == '/' || line[k+1] == '*') {
							hasComments = true
							break
						}
					}
					return strings.Join(jsonLines, "\n"), endIndex, hasComments
				}
			case '/':
				if j+1 < n && (line[j+1] == '/' || line[j+1] == '*') {
					hasComments = true
				}
			}
		}
	}

	return strings.Join(jsonLines, "\n"), endIndex, hasComments
}

func (r *ChartJSONRule) Description() string {
	return "Limpia comentarios inline en JSON para charts"
}

func (r *ChartJSONRule) Priority() int {
	return 5 // Prioridad 5 - Mejoras de formato específicas
}

func (r *ChartJSONRule) Category() base.RuleCategory {
	return base.CategoryEnhancement
}
