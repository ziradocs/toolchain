// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// MapParser maneja el parsing de mapas Leaflet
type MapParser struct{}

// CanParse determina si puede parsear una línea como Map
func (p *MapParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	switch mode {
	case "strict":
		return strings.HasPrefix(trimmed, "<<map") && strings.Contains(trimmed, ">>")
	case "flex":
		return strings.HasPrefix(trimmed, "<<map") && strings.Contains(trimmed, ">>")
	}

	return false
}

// Parse parsea un elemento Map
func (p *MapParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{Error: nil}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	// Extraer atributos si están presentes: <<map type="city" width="1200" height="800" zoom="10">>
	mapType := "world" // default
	width := 800       // default
	height := 600      // default
	zoom := 0          // 0 means not specified, will use defaults based on type

	// Extraer type si está presente
	if mapTypeAttr := extractAttribute(line, "type"); mapTypeAttr != "" {
		mapType = mapTypeAttr
	}

	// Extraer width si está presente
	if strings.Contains(line, "width=") {
		if w := extractAttribute(line, "width"); w != "" {
			if val, err := strconv.Atoi(w); err == nil && val > 0 {
				width = val
			}
		}
	}

	// Extraer height si está presente
	if strings.Contains(line, "height=") {
		if h := extractAttribute(line, "height"); h != "" {
			if val, err := strconv.Atoi(h); err == nil && val > 0 {
				height = val
			}
		}
	}

	// Extraer zoom si está presente (puede sobrescribirse después)
	if strings.Contains(line, "zoom=") {
		if z := extractAttribute(line, "zoom"); z != "" {
			if val, err := strconv.Atoi(z); err == nil && val >= 0 {
				zoom = val
			}
		}
	}

	mapElement := ast.NewMapElement(pos, mapType)
	mapElement.Width = width
	mapElement.Height = height
	if zoom > 0 {
		mapElement.Zoom = zoom
	}

	consumedLines := 1 // skip <<map>> line
	var currentMarker *ast.MapMarker

	// Parsear propiedades del mapa
parseLoop:
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]
		trimmedLine := strings.TrimSpace(line)

		// Check for closing tag and consume it
		if trimmedLine == "<</map>>" {
			consumedLines++
			break
		}

		// Skip empty lines
		if trimmedLine == "" {
			consumedLines++
			continue
		}

		// Check for inline marker format: "marker: lat, lng, label, details, color"
		if strings.HasPrefix(trimmedLine, "marker:") {
			marker := parseInlineMarker(trimmedLine)
			if marker != nil {
				mapElement.Markers = append(mapElement.Markers, *marker)
			}
			consumedLines++
			continue
		}

		// Check for center format: "center: lat, lng"
		if strings.HasPrefix(trimmedLine, "center:") {
			parts := strings.SplitN(trimmedLine, ":", 2)
			if len(parts) == 2 {
				coords := strings.Split(strings.TrimSpace(parts[1]), ",")
				if len(coords) >= 2 {
					var lat, lng float64
					if val, err := strconv.ParseFloat(strings.TrimSpace(coords[0]), 64); err == nil {
						lat = val
					}
					if val, err := strconv.ParseFloat(strings.TrimSpace(coords[1]), 64); err == nil {
						lng = val
					}
					mapElement.Center = &ast.MapCoordinate{
						Lat: lat,
						Lng: lng,
					}
				}
			}
			consumedLines++
			continue
		}

		// Parsear propiedades como "type:", "markers:", etc.
		if strings.Contains(trimmedLine, ":") {
			parts := strings.SplitN(trimmedLine, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "type":
					mapElement.MapType = value
				case "heatmap":
					mapElement.Heatmap = value == "true"
				case "zoom":
					if value != "" {
						if zoomValue, err := strconv.Atoi(value); err == nil {
							mapElement.Zoom = zoomValue
						} else {
							mapElement.Zoom = 2 // Fallback si no se puede parsear
						}
					}
				case "markers":
					// Inicio de la sección de marcadores, siguiente línea debe ser un marcador
				case "options":
					// Inicio de la sección de opciones
				case "- lat":
					// Nuevo marcador
					if currentMarker != nil {
						// Guardar marcador anterior
						mapElement.Markers = append(mapElement.Markers, *currentMarker)
					}
					// Crear nuevo marcador
					lat := parseLatLng(value)
					currentMarker = &ast.MapMarker{Lat: lat}
				case "lng":
					if currentMarker != nil {
						lng := parseLatLng(value)
						currentMarker.Lng = lng
					}
				case "label":
					if currentMarker != nil {
						currentMarker.Label = strings.Trim(value, "\"")
					}
				case "value":
					if currentMarker != nil {
						val := parseValue(value)
						currentMarker.Value = val
					}
				case "color":
					if currentMarker != nil {
						currentMarker.Color = strings.Trim(value, "\"")
					}
				case "size":
					if currentMarker != nil {
						currentMarker.Size = strings.Trim(value, "\"")
					}
				case "details":
					if currentMarker != nil {
						currentMarker.Details = strings.Trim(value, "\"")
					}
				case "title":
					// Puede ser title dentro de options o title principal
					if mapElement.Options == nil {
						mapElement.Options = make(map[string]interface{})
					}
					mapElement.Options["title"] = strings.Trim(value, "\"")
				case "showValues":
					if mapElement.Options == nil {
						mapElement.Options = make(map[string]interface{})
					}
					mapElement.Options["showValues"] = value == "true"
				case "clustering":
					if mapElement.Options == nil {
						mapElement.Options = make(map[string]interface{})
					}
					mapElement.Options["clustering"] = value == "true"
				default:
					// Línea no reconocida - terminar parsing de este mapa
					break parseLoop
				}
				consumedLines++
			} else {
				// Línea sin ":" - terminar parsing de este mapa
				break parseLoop
			}
		} else {
			// Línea no reconocida - terminar parsing de este mapa
			break parseLoop
		}
	}

	// Agregar último marcador si existe
	if currentMarker != nil {
		mapElement.Markers = append(mapElement.Markers, *currentMarker)
	}

	return &ParseResult{
		Element:       mapElement,
		ConsumedLines: consumedLines,
		Error:         nil,
	}
}

// parseLatLng parsea coordenadas lat/lng
func parseLatLng(value string) float64 {
	// Parsear coordenadas usando strconv.ParseFloat
	val, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		// Log warning - usamos fmt.Printf temporal
		return 0.0
	}
	return val
}

// parseValue parsea valores numéricos
func parseValue(value string) float64 {
	// Parsear valores usando strconv.ParseFloat
	val, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		// Log warning - usamos fmt.Printf temporal
		return 0.0
	}
	return val
}

// parseInlineMarker parsea un marcador en formato inline
// Formato: "marker: lat, lng, label, details, color, value"
// Ejemplo: marker: 40.7128, -74.0060, "New York HQ", "Main headquarters", "blue"
func parseInlineMarker(line string) *ast.MapMarker {
	// Remover el prefijo "marker:"
	content := strings.TrimPrefix(line, "marker:")
	content = strings.TrimSpace(content)

	// Parsear campos separados por comas (pero respetando strings entre comillas)
	fields := parseCSVLine(content)

	if len(fields) < 2 {
		return nil // Necesita al menos lat, lng
	}

	marker := &ast.MapMarker{}

	// Lat (campo 0)
	if lat, err := strconv.ParseFloat(strings.TrimSpace(fields[0]), 64); err == nil {
		marker.Lat = lat
	}

	// Lng (campo 1)
	if lng, err := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64); err == nil {
		marker.Lng = lng
	}

	// Label (campo 2) - opcional
	if len(fields) > 2 {
		marker.Label = strings.Trim(strings.TrimSpace(fields[2]), "\"")
	}

	// Details (campo 3) - opcional
	if len(fields) > 3 {
		marker.Details = strings.Trim(strings.TrimSpace(fields[3]), "\"")
	}

	// Color (campo 4) - opcional
	if len(fields) > 4 {
		marker.Color = strings.Trim(strings.TrimSpace(fields[4]), "\"")
	}

	// Value (campo 5) - opcional
	if len(fields) > 5 {
		if val, err := strconv.ParseFloat(strings.TrimSpace(fields[5]), 64); err == nil {
			marker.Value = val
		}
	}

	return marker
}

// parseCSVLine parsea una línea CSV respetando strings entre comillas
func parseCSVLine(line string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(line); i++ {
		char := line[i]

		switch char {
		case '"':
			inQuotes = !inQuotes
			current.WriteByte(char)
		case ',':
			if inQuotes {
				current.WriteByte(char)
			} else {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(char)
		}
	}

	// Agregar el último campo
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	return fields
}
