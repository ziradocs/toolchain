// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// MapParser maneja el parsing de mapas Leaflet
type MapParser struct{}

// CanParse determina si puede parsear una línea como Map: `<<map>>` pelado o
// `<<map attr="…">>` con atributos.
//
// La condición anterior —`HasPrefix("<<map") && Contains(">>")`— no tenía
// frontera de palabra: `<<mapa>>`, `<<maps>>` y `<<mapping x="1">>` entraban
// todos al parser de mapas. Medido en flex sobre `main`, `<<mapa>>` producía un
// MapElement, y en un bloque strict el `title:` de abajo terminaba adentro del
// mapa en vez de ser el título del slide — el slide perdía su título y nadie
// nombraba el tag mal escrito.
//
// La regla es la de matchesMediaTag (media.go), que ya litigó exactamente esto
// para `<<video`/`<<audio`: la línea cierra con ">>", y lo que sigue a `<<map`
// es ese ">>" o un espacio (vienen atributos).
func (p *MapParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)
	if mode != "strict" && mode != "flex" {
		return false
	}
	// Forma de fence (```map/````map), espejo de ChartParser.CanParse —
	// solo en flex, mismo motivo (strict es keyword-driven, sin fences).
	if mode == "flex" && isFencedBlockOpener(trimmed, "map") {
		return true
	}
	rest, ok := strings.CutPrefix(trimmed, "<<map")
	if !ok || !closesInlineTagOnce(rest) {
		return false
	}
	return rest == ">>" || strings.HasPrefix(rest, " ")
}

// mapJSONBody es la forma JSON que acepta ```map — la única forma que este
// fence soporta (mismo alcance que ChartParser: ver su comentario en
// chart.go). No es un passthrough tipo Chart.js (MapElement no tiene un
// RawJSON como ChartElement, ni falta que le hace: un mapa no tiene
// configuración de terceros que preservar literal, solo datos geográficos),
// así que el JSON se decodifica a esta forma y se traduce a los mismos
// campos que el loop de propiedades de abajo ya puebla — JSON es aquí una
// SERIALIZACIÓN alternativa de los mismos datos, no un modo aparte.
type mapJSONBody struct {
	Type    string                 `json:"type"`
	Center  []float64              `json:"center"`
	Zoom    int                    `json:"zoom"`
	Heatmap bool                   `json:"heatmap"`
	Title   string                 `json:"title"`
	Width   int                    `json:"width"`
	Height  int                    `json:"height"`
	Markers []mapJSONMarker        `json:"markers"`
	Options map[string]interface{} `json:"options"`
}

// mapJSONMarker acepta tanto la forma "position: [lat,lng]" + "popup"
// (convención Leaflet-nativa, la que usa el corpus real —
// examples/webp_test.doclang) como "lat"/"lng" sueltos + "label" (los
// nombres que usa el resto de este parser, marker: en el loop de
// propiedades) — cualquiera de los dos, o una mezcla, resuelve al mismo
// ast.MapMarker.
type mapJSONMarker struct {
	Position []float64 `json:"position"`
	// Punteros para distinguir "ausente" de un 0 legítimo (el ecuador o el
	// meridiano de Greenwich): un marcador sin coordenadas se rechaza en vez
	// de aparecer en 0,0.
	Lat     *float64 `json:"lat"`
	Lng     *float64 `json:"lng"`
	Popup   string   `json:"popup"`
	Label   string   `json:"label"`
	Details string   `json:"details"`
	Color   string   `json:"color"`
	Size    string   `json:"size"`
	Value   float64  `json:"value"`
}

func (m mapJSONMarker) toMarker() (ast.MapMarker, error) {
	marker := ast.MapMarker{
		Label:   m.Label,
		Details: m.Details,
		Color:   m.Color,
		Size:    m.Size,
		Value:   m.Value,
	}
	switch {
	case len(m.Position) >= 2:
		marker.Lat = m.Position[0]
		marker.Lng = m.Position[1]
	case m.Lat != nil && m.Lng != nil:
		marker.Lat, marker.Lng = *m.Lat, *m.Lng
	default:
		return marker, fmt.Errorf("map marker %q has no coordinates: use position: [lat, lng] or both lat and lng", firstNonEmpty(m.Popup, m.Label))
	}
	if m.Popup != "" {
		marker.Label = m.Popup
	}
	return marker, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Parse parsea un elemento Map
func (p *MapParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{Error: nil}
	}

	pos := ctx.Position(startIndex)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	// Forma de fence (```map ... ```): único cuerpo que soporta es JSON —
	// mismo alcance que ChartParser.Parse, mismo motivo (collectFencedBody
	// delimita el cuerpo sin ambigüedad, así que no hace falta el loop de
	// propiedades key:value de abajo).
	if ctx.Mode == "flex" && isFencedBlockOpener(line, "map") {
		body, consumed := collectFencedBody(ctx.Lines, startIndex)
		trimmedBody := strings.TrimSpace(body)

		var parsed mapJSONBody
		if err := json.Unmarshal([]byte(trimmedBody), &parsed); err != nil {
			mapType := "world"
			return &ParseResult{
				Element:       ast.NewMapElement(pos, mapType),
				ConsumedLines: consumed,
				Error:         nil,
				Diagnostics: []diagnostics.Diagnostic{
					diagnostics.NewWarning(
						"El JSON del mapa es inválido y fue ignorado; el mapa quedará sin datos",
						pos, "map-parser").WithRuleID("MAP002"),
				},
			}
		}

		mapType := parsed.Type
		if mapType == "" {
			mapType = "world"
		}
		mapElement := ast.NewMapElement(pos, mapType)
		mapElement.Zoom = parsed.Zoom
		mapElement.Heatmap = parsed.Heatmap
		mapElement.Width = parsed.Width
		mapElement.Height = parsed.Height
		if parsed.Title != "" {
			if mapElement.Options == nil {
				mapElement.Options = make(map[string]interface{})
			}
			mapElement.Options["title"] = parsed.Title
		}
		for k, v := range parsed.Options {
			if mapElement.Options == nil {
				mapElement.Options = make(map[string]interface{})
			}
			mapElement.Options[k] = v
		}
		if len(parsed.Center) >= 2 {
			mapElement.Center = &ast.MapCoordinate{Lat: parsed.Center[0], Lng: parsed.Center[1]}
		}
		var markerErr error
		for _, m := range parsed.Markers {
			marker, err := m.toMarker()
			if err != nil {
				if markerErr == nil {
					markerErr = err
				}
				continue
			}
			mapElement.Markers = append(mapElement.Markers, marker)
		}

		return &ParseResult{Element: mapElement, ConsumedLines: consumed, Error: markerErr}
	}

	// Extraer atributos si están presentes: <<map type="city" width="1200" height="800" zoom="10">>
	mapType := "world" // default
	// 0 = sin declarar, igual que zoom acá abajo (que ya lo hacía) y que
	// ChartParser. Ver el comentario largo en internal/elements/chart.go para
	// por qué el default del renderer no va en el AST.
	width := 0
	height := 0
	zoom := 0 // 0 means not specified, will use defaults based on type

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
	currentHasLng := false
	// markerErr registra el primer marcador sin coordenadas válidas. Antes
	// esos marcadores salían en 0,0 (o se descartaban) sin diagnóstico.
	var markerErr error
	fail := func(err error) {
		if markerErr == nil {
			markerErr = err
		}
	}
	finishMarker := func() {
		if currentMarker == nil {
			return
		}
		if !currentHasLng {
			fail(fmt.Errorf("map marker at lat %v has no lng", currentMarker.Lat))
		} else {
			mapElement.Markers = append(mapElement.Markers, *currentMarker)
		}
		currentMarker = nil
	}

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
			marker, err := parseInlineMarker(trimmedLine)
			if err != nil {
				fail(err)
			} else {
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
					// Nuevo marcador: cierra el anterior, que necesita su lng.
					finishMarker()
					lat, err := parseCoordinate("lat", value)
					if err != nil {
						fail(err)
					}
					currentMarker = &ast.MapMarker{Lat: lat}
					currentHasLng = false
				case "lng":
					if currentMarker != nil {
						lng, err := parseCoordinate("lng", value)
						if err != nil {
							fail(err)
						} else {
							currentMarker.Lng = lng
							currentHasLng = true
						}
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
	finishMarker()

	return &ParseResult{
		Element:       mapElement,
		ConsumedLines: consumedLines,
		Error:         markerErr,
	}
}

// parseCoordinate lee una latitud o longitud; a diferencia de parseLatLng, un
// valor ilegible es un error y no un 0 silencioso.
func parseCoordinate(name, value string) (float64, error) {
	v, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(value), "\""), 64)
	if err != nil {
		return 0, fmt.Errorf("map marker %s %q is not a number", name, value)
	}
	return v, nil
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
func parseInlineMarker(line string) (*ast.MapMarker, error) {
	// Remover el prefijo "marker:"
	content := strings.TrimPrefix(line, "marker:")
	content = strings.TrimSpace(content)

	// Parsear campos separados por comas (pero respetando strings entre comillas)
	fields := parseCSVLine(content)

	if len(fields) < 2 {
		return nil, fmt.Errorf("map marker %q needs at least lat, lng", content)
	}

	marker := &ast.MapMarker{}
	var err error
	if marker.Lat, err = parseCoordinate("lat", fields[0]); err != nil {
		return nil, err
	}
	if marker.Lng, err = parseCoordinate("lng", fields[1]); err != nil {
		return nil, err
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

	return marker, nil
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
