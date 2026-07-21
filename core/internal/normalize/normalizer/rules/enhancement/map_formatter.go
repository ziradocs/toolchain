// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"
)

// MapFormatterRule formatea bloques de datos de mapas sin indentación adecuada
type MapFormatterRule struct{}

// NewMapFormatterRule crea una nueva instancia de MapFormatterRule
func NewMapFormatterRule() *MapFormatterRule {
	return &MapFormatterRule{}
}

func (r *MapFormatterRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	var result []string
	inMapBlock := false
	blockStart := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detectar inicio de bloque de mapa
		if strings.HasPrefix(trimmed, "<<map>>") {
			inMapBlock = true
			blockStart = i
			result = append(result, line)
			continue
		}

		// Si estamos en un bloque de mapa
		if inMapBlock {
			// Detectar fin del bloque de mapa
			if trimmed == ">>" || (r.isEndOfMapBlock(trimmed, lines, i) && !r.isMapDataLine(trimmed)) {
				inMapBlock = false
				// Procesar el bloque completo
				mapLines := lines[blockStart+1 : i]
				// Solo formatear si no está ya formateado
				if r.needsFormatting(mapLines) {
					formattedLines := r.formatMapData(mapLines)
					result = append(result, formattedLines...)
				} else {
					// Mantener el formato original
					result = append(result, mapLines...)
				}
				result = append(result, line)
				continue
			}
			// No agregar líneas aquí, se procesarán al final del bloque
			continue
		}

		// Líneas normales fuera de bloques de mapa
		result = append(result, line)
	}

	// Si terminamos y todavía estamos en un bloque de mapa
	if inMapBlock && blockStart != -1 {
		mapLines := lines[blockStart+1:]
		if r.needsFormatting(mapLines) {
			formattedLines := r.formatMapData(mapLines)
			result = append(result, formattedLines...)
		} else {
			result = append(result, mapLines...)
		}
	}

	return strings.Join(result, "\n"), nil
}

// needsFormatting determina si un bloque de mapa necesita formateo
func (r *MapFormatterRule) needsFormatting(lines []string) bool {
	// Si todas las líneas ya tienen indentación consistente, no reformatear
	hasProperIndentation := true
	foundContent := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue // Ignorar líneas vacías
		}

		foundContent = true

		// Las propiedades principales deben tener 2 espacios de indentación
		if strings.HasPrefix(trimmed, "type:") ||
			strings.HasPrefix(trimmed, "center:") ||
			strings.HasPrefix(trimmed, "zoom:") ||
			strings.HasPrefix(trimmed, "heatmap:") ||
			strings.HasPrefix(trimmed, "markers:") {
			if !strings.HasPrefix(line, "  ") {
				hasProperIndentation = false
				break
			}
		}

		// Los marcadores deben tener 2 espacios de indentación
		if strings.HasPrefix(trimmed, "- lat:") {
			if !strings.HasPrefix(line, "  ") {
				hasProperIndentation = false
				break
			}
		}

		// Las propiedades de marcadores deben tener 4 espacios de indentación
		if strings.HasPrefix(trimmed, "lng:") ||
			strings.HasPrefix(trimmed, "label:") ||
			strings.HasPrefix(trimmed, "value:") ||
			strings.HasPrefix(trimmed, "icon:") {
			if !strings.HasPrefix(line, "    ") {
				hasProperIndentation = false
				break
			}
		}
	}

	// Si no encontramos contenido, no hay nada que formatear
	if !foundContent {
		return false
	}

	// Si ya tiene la indentación correcta, no reformatear
	return !hasProperIndentation
}

// isEndOfMapBlock determina si una línea marca el final de un bloque de mapa
func (r *MapFormatterRule) isEndOfMapBlock(trimmed string, lines []string, index int) bool {
	// Final explícito con >>
	if trimmed == ">>" {
		return true
	}

	// Línea vacía seguida de contenido que no es de mapa
	if trimmed == "" && index+1 < len(lines) {
		nextLine := strings.TrimSpace(lines[index+1])
		return !r.isMapDataLine(nextLine) && nextLine != ""
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

// isMapDataLine determina si una línea contiene datos de mapa
func (r *MapFormatterRule) isMapDataLine(line string) bool {
	mapDataPrefixes := []string{
		"type:",
		"center:",
		"zoom:",
		"markers:",
		"- lat:",
		"lat:",
		"lng:",
		"label:",
		"value:",
		"icon:",
		"heatmap:",
	}

	trimmed := strings.TrimSpace(line)
	for _, prefix := range mapDataPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	// También considerar líneas con guión al inicio (elementos de lista YAML)
	if strings.HasPrefix(trimmed, "- ") {
		return true
	}

	// Líneas con propiedades anidadas (indentadas)
	if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
		return true
	}

	return false
}

// formatMapData formatea las líneas de datos de mapa con indentación apropiada
func (r *MapFormatterRule) formatMapData(lines []string) []string {
	var result []string
	inMarkers := false
	inMarkerItem := false
	justAddedMarkers := false // Track if we just added the markers: line

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			// Skip blank lines immediately after markers:
			if justAddedMarkers {
				justAddedMarkers = false
				continue
			}
			result = append(result, "")
			continue
		}

		justAddedMarkers = false // Reset flag when we see non-empty content

		// Propiedades de nivel superior del mapa
		if strings.HasPrefix(trimmed, "type:") ||
			strings.HasPrefix(trimmed, "center:") ||
			strings.HasPrefix(trimmed, "zoom:") ||
			strings.HasPrefix(trimmed, "heatmap:") {
			result = append(result, "  "+trimmed) // Indentar propiedades principales
			inMarkers = false
			continue
		}
		// Sección de marcadores
		if strings.HasPrefix(trimmed, "markers:") {
			inMarkers = true
			justAddedMarkers = true
			result = append(result, "  "+trimmed) // Indentar markers:
			continue
		}

		// Si estamos en la sección de marcadores
		if inMarkers {
			// Nuevo marcador
			if strings.HasPrefix(trimmed, "- lat:") || strings.HasPrefix(trimmed, "lat:") {
				inMarkerItem = true
				if strings.HasPrefix(trimmed, "- lat:") {
					result = append(result, "  "+trimmed)
				} else {
					result = append(result, "  - "+trimmed)
				}
				continue
			}

			// Propiedades del marcador actual
			if inMarkerItem && (strings.HasPrefix(trimmed, "lng:") ||
				strings.HasPrefix(trimmed, "label:") ||
				strings.HasPrefix(trimmed, "value:") ||
				strings.HasPrefix(trimmed, "icon:")) {
				result = append(result, "    "+trimmed)
				continue
			}

			// Si no es una propiedad reconocida de marcador, terminamos la sección
			inMarkers = false
			inMarkerItem = false
		}

		// Línea no reconocida, agregarla con indentación de 2 espacios (asumiendo que es una propiedad del mapa)
		result = append(result, "  "+trimmed)
	}

	return result
}

// GetName retorna el nombre de la regla
func (r *MapFormatterRule) GetName() string {
	return "MapFormatter"
}

// Description retorna la descripción de la regla
func (r *MapFormatterRule) Description() string {
	return "Formats map blocks with proper YAML indentation"
}

// Priority retorna la prioridad de la regla
func (r *MapFormatterRule) Priority() int {
	return 5 // Prioridad alta para ejecutar antes de otros formatters
}
