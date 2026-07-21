// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/util"
)

// mockLogger es un logger simple para pruebas
type mockLogger struct{}

func (m *mockLogger) Error(message string, args ...interface{})              {}
func (m *mockLogger) Warn(message string, args ...interface{})               {}
func (m *mockLogger) Info(category, message string, args ...interface{})     {}
func (m *mockLogger) Debug(component, message string, args ...interface{})   {}
func (m *mockLogger) Progress(stage, operation string, progress int)         {}
func (m *mockLogger) Summary(operation string, stats map[string]interface{}) {}
func (m *mockLogger) SetLevel(level util.LogLevel)                           {}

func TestMapParser_CanParse(t *testing.T) {
	parser := &MapParser{}

	tests := []struct {
		name     string
		line     string
		mode     string
		expected bool
	}{
		{
			name:     "strict mode - valid map opening",
			line:     "<<map>>",
			mode:     "strict",
			expected: true,
		},
		{
			name:     "strict mode - map with whitespace",
			line:     "  <<map>>  ",
			mode:     "strict",
			expected: true,
		},
		{
			name:     "flex mode - valid map opening",
			line:     "<<map>>",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - map with whitespace",
			line:     "    <<map>>",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "invalid - not a map",
			line:     "<<chart>>",
			mode:     "strict",
			expected: false,
		},
		{
			name:     "invalid - partial map tag",
			line:     "<<ma",
			mode:     "strict",
			expected: false,
		},
		{
			name:     "invalid - text content",
			line:     "This is regular text",
			mode:     "strict",
			expected: false,
		},
		{
			name:     "invalid - empty line",
			line:     "",
			mode:     "strict",
			expected: false,
		},
		{
			name:     "invalid - mermaid diagram",
			line:     "<<mermaid>>",
			mode:     "flex",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.CanParse(tt.line, tt.mode)
			if result != tt.expected {
				t.Errorf("CanParse() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestMapParser_Parse_BasicMap(t *testing.T) {
	parser := &MapParser{}
	mockLog := &mockLogger{}

	lines := []string{
		"<<map>>",
		"  type: world",
		"  zoom: 5",
		"  heatmap: true",
	}

	ctx := &ParseContext{
		Mode:        "strict",
		CurrentLine: 0,
		Logger:      mockLog,
		Lines:       lines,
	}

	result := parser.Parse(ctx, 0)

	// Verificar que no hay errores
	if result.Error != nil {
		t.Fatalf("Parse() error = %v, expected nil", result.Error)
	}

	// Verificar que se parseó correctamente
	if result.Element == nil {
		t.Fatal("Parse() Element = nil, expected MapElement")
	}

	mapElement, ok := result.Element.(*ast.MapElement)
	if !ok {
		t.Fatalf("Parse() Element type = %T, expected *ast.MapElement", result.Element)
	}

	// Verificar propiedades del mapa
	if mapElement.MapType != "world" {
		t.Errorf("MapType = %v, expected 'world'", mapElement.MapType)
	}

	if mapElement.Zoom != 5 {
		t.Errorf("Zoom = %v, expected 5", mapElement.Zoom)
	}

	if !mapElement.Heatmap {
		t.Errorf("Heatmap = %v, expected true", mapElement.Heatmap)
	}

	// Verificar líneas consumidas
	if result.ConsumedLines != 4 {
		t.Errorf("ConsumedLines = %v, expected 4", result.ConsumedLines)
	}
}

func TestMapParser_Parse_MapWithMarkers(t *testing.T) {
	parser := &MapParser{}
	mockLog := &mockLogger{}

	lines := []string{
		"<<map>>",
		"  type: world",
		"  zoom: 3",
		"  markers:",
		"  - lat: 40.7128",
		"    lng: -74.0060",
		"    label: \"New York\"",
		"    value: 100",
		"  - lat: 34.0522",
		"    lng: -118.2437",
		"    label: \"Los Angeles\"", "    value: 75",
	}

	ctx := &ParseContext{
		Mode:        "flex",
		CurrentLine: 0,
		Logger:      mockLog,
		Lines:       lines,
	}

	result := parser.Parse(ctx, 0)

	// Verificar que no hay errores
	if result.Error != nil {
		t.Fatalf("Parse() error = %v, expected nil", result.Error)
	}

	// Verificar elemento del mapa
	mapElement, ok := result.Element.(*ast.MapElement)
	if !ok {
		t.Fatalf("Parse() Element type = %T, expected *ast.MapElement", result.Element)
	}

	// Verificar propiedades básicas
	if mapElement.MapType != "world" {
		t.Errorf("MapType = %v, expected 'world'", mapElement.MapType)
	}

	if mapElement.Zoom != 3 {
		t.Errorf("Zoom = %v, expected 3", mapElement.Zoom)
	}

	// Verificar marcadores
	if len(mapElement.Markers) != 2 {
		t.Fatalf("Markers length = %v, expected 2", len(mapElement.Markers))
	}

	// Verificar primer marcador
	marker1 := mapElement.Markers[0]
	if marker1.Lat != 40.7128 {
		t.Errorf("Marker1 Lat = %v, expected 40.7128", marker1.Lat)
	}
	if marker1.Lng != -74.0060 {
		t.Errorf("Marker1 Lng = %v, expected -74.0060", marker1.Lng)
	}
	if marker1.Label != "New York" {
		t.Errorf("Marker1 Label = %v, expected 'New York'", marker1.Label)
	}
	if marker1.Value != 100 {
		t.Errorf("Marker1 Value = %v, expected 100", marker1.Value)
	}

	// Verificar segundo marcador
	marker2 := mapElement.Markers[1]
	if marker2.Lat != 34.0522 {
		t.Errorf("Marker2 Lat = %v, expected 34.0522", marker2.Lat)
	}
	if marker2.Lng != -118.2437 {
		t.Errorf("Marker2 Lng = %v, expected -118.2437", marker2.Lng)
	}
	if marker2.Label != "Los Angeles" {
		t.Errorf("Marker2 Label = %v, expected 'Los Angeles'", marker2.Label)
	}
	if marker2.Value != 75 {
		t.Errorf("Marker2 Value = %v, expected 75", marker2.Value)
	}
}

func TestMapParser_Parse_DefaultValues(t *testing.T) {
	parser := &MapParser{}
	mockLog := &mockLogger{}
	lines := []string{
		"<<map>>",
		"", // Línea vacía
	}

	ctx := &ParseContext{
		Mode:        "strict",
		CurrentLine: 0,
		Logger:      mockLog,
		Lines:       lines,
	}

	result := parser.Parse(ctx, 0)

	// Verificar que no hay errores
	if result.Error != nil {
		t.Fatalf("Parse() error = %v, expected nil", result.Error)
	}

	// Verificar elemento del mapa
	mapElement, ok := result.Element.(*ast.MapElement)
	if !ok {
		t.Fatalf("Parse() Element type = %T, expected *ast.MapElement", result.Element)
	}

	// Verificar valores por defecto
	if mapElement.MapType != "world" {
		t.Errorf("MapType = %v, expected 'world' (default)", mapElement.MapType)
	}

	if mapElement.Zoom != 0 {
		t.Errorf("Zoom = %v, expected 0 (default)", mapElement.Zoom)
	}

	if mapElement.Heatmap != false {
		t.Errorf("Heatmap = %v, expected false (default)", mapElement.Heatmap)
	}

	if len(mapElement.Markers) != 0 {
		t.Errorf("Markers length = %v, expected 0 (default)", len(mapElement.Markers))
	}
}

func TestMapParser_Parse_InvalidZoom(t *testing.T) {
	parser := &MapParser{}
	mockLog := &mockLogger{}
	lines := []string{
		"<<map>>",
		"  type: country",
		"  zoom: invalid_number",
		"  heatmap: false",
	}

	ctx := &ParseContext{
		Mode:        "flex",
		CurrentLine: 0,
		Logger:      mockLog,
		Lines:       lines,
	}

	result := parser.Parse(ctx, 0)

	// Verificar que no hay errores (debe usar valor por defecto)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v, expected nil", result.Error)
	}

	// Verificar elemento del mapa
	mapElement, ok := result.Element.(*ast.MapElement)
	if !ok {
		t.Fatalf("Parse() Element type = %T, expected *ast.MapElement", result.Element)
	}

	// Verificar que usa el fallback para zoom inválido
	if mapElement.Zoom != 2 {
		t.Errorf("Zoom = %v, expected 2 (fallback for invalid value)", mapElement.Zoom)
	}

	// Verificar otras propiedades
	if mapElement.MapType != "country" {
		t.Errorf("MapType = %v, expected 'country'", mapElement.MapType)
	}

	if mapElement.Heatmap != false {
		t.Errorf("Heatmap = %v, expected false", mapElement.Heatmap)
	}
}

func TestMapParser_Parse_EmptyZoom(t *testing.T) {
	parser := &MapParser{}
	mockLog := &mockLogger{}
	lines := []string{
		"<<map>>",
		"  zoom:",
		"  type: region",
	}

	ctx := &ParseContext{
		Mode:        "strict",
		CurrentLine: 0,
		Logger:      mockLog,
		Lines:       lines,
	}

	result := parser.Parse(ctx, 0)

	// Verificar que no hay errores
	if result.Error != nil {
		t.Fatalf("Parse() error = %v, expected nil", result.Error)
	}

	// Verificar elemento del mapa
	mapElement, ok := result.Element.(*ast.MapElement)
	if !ok {
		t.Fatalf("Parse() Element type = %T, expected *ast.MapElement", result.Element)
	}

	// Verificar que zoom permanece en valor por defecto cuando está vacío
	if mapElement.Zoom != 0 {
		t.Errorf("Zoom = %v, expected 0 (default for empty value)", mapElement.Zoom)
	}
}

func TestMapParser_Parse_IncompleteMarker(t *testing.T) {
	parser := &MapParser{}
	mockLog := &mockLogger{}

	lines := []string{
		"<<map>>",
		"  markers:",
		"  - lat: 40.7128",
		"    label: \"Incomplete marker\"",
		"  - lat: 51.5074",
		"    lng: -0.1278", "    label: \"London\"",
	}

	ctx := &ParseContext{
		Mode:        "flex",
		CurrentLine: 0,
		Logger:      mockLog,
		Lines:       lines,
	}

	result := parser.Parse(ctx, 0)

	// Verificar que no hay errores
	if result.Error != nil {
		t.Fatalf("Parse() error = %v, expected nil", result.Error)
	}

	// Verificar elemento del mapa
	mapElement, ok := result.Element.(*ast.MapElement)
	if !ok {
		t.Fatalf("Parse() Element type = %T, expected *ast.MapElement", result.Element)
	}

	// Debe tener 2 marcadores, incluso si uno está incompleto
	if len(mapElement.Markers) != 2 {
		t.Fatalf("Markers length = %v, expected 2", len(mapElement.Markers))
	}

	// Verificar marcador incompleto
	marker1 := mapElement.Markers[0]
	if marker1.Lat != 40.7128 {
		t.Errorf("Marker1 Lat = %v, expected 40.7128", marker1.Lat)
	}
	if marker1.Lng != 0 {
		t.Errorf("Marker1 Lng = %v, expected 0 (default for missing lng)", marker1.Lng)
	}
	if marker1.Label != "Incomplete marker" {
		t.Errorf("Marker1 Label = %v, expected 'Incomplete marker'", marker1.Label)
	}

	// Verificar marcador completo
	marker2 := mapElement.Markers[1]
	if marker2.Lat != 51.5074 {
		t.Errorf("Marker2 Lat = %v, expected 51.5074", marker2.Lat)
	}
	if marker2.Lng != -0.1278 {
		t.Errorf("Marker2 Lng = %v, expected -0.1278", marker2.Lng)
	}
	if marker2.Label != "London" {
		t.Errorf("Marker2 Label = %v, expected 'London'", marker2.Label)
	}
}

func TestMapParser_Parse_IndentationDetection(t *testing.T) {
	parser := &MapParser{}
	mockLog := &mockLogger{}

	lines := []string{
		"<<map>>",
		"    type: world",     // 4 espacios
		"    zoom: 6",         // 4 espacios
		"    markers:",        // 4 espacios
		"    - lat: 40.7128",  // 4 espacios
		"      lng: -74.0060", // 6 espacios (más indentado)		"Next slide content",     // Sin indentación, debe terminar el bloque
	}

	ctx := &ParseContext{
		Mode:        "flex",
		CurrentLine: 0,
		Logger:      mockLog,
		Lines:       lines,
	}

	result := parser.Parse(ctx, 0)

	// Verificar que no hay errores
	if result.Error != nil {
		t.Fatalf("Parse() error = %v, expected nil", result.Error)
	}

	// Verificar elemento del mapa
	mapElement, ok := result.Element.(*ast.MapElement)
	if !ok {
		t.Fatalf("Parse() Element type = %T, expected *ast.MapElement", result.Element)
	}

	// Verificar propiedades parseadas
	if mapElement.MapType != "world" {
		t.Errorf("MapType = %v, expected 'world'", mapElement.MapType)
	}

	if mapElement.Zoom != 6 {
		t.Errorf("Zoom = %v, expected 6", mapElement.Zoom)
	}

	// Debe haber parseado un marcador
	if len(mapElement.Markers) != 1 {
		t.Fatalf("Markers length = %v, expected 1", len(mapElement.Markers))
	}

	marker := mapElement.Markers[0]
	if marker.Lat != 40.7128 {
		t.Errorf("Marker Lat = %v, expected 40.7128", marker.Lat)
	}
	if marker.Lng != -74.0060 {
		t.Errorf("Marker Lng = %v, expected -74.0060", marker.Lng)
	}

	// Verificar que paró antes de "Next slide content"
	if result.ConsumedLines != 6 {
		t.Errorf("ConsumedLines = %v, expected 6", result.ConsumedLines)
	}
}

func TestParseLatLng(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name:     "positive latitude",
			input:    "40.7128",
			expected: 40.7128,
		},
		{
			name:     "negative longitude",
			input:    "-74.0060",
			expected: -74.0060,
		},
		{
			name:     "zero value",
			input:    "0",
			expected: 0.0,
		},
		{
			name:     "decimal value",
			input:    "51.5074",
			expected: 51.5074,
		},
		{
			name:     "value with spaces",
			input:    "  34.0522  ",
			expected: 34.0522,
		},
		{
			name:     "invalid value",
			input:    "invalid",
			expected: 0.0,
		},
		{
			name:     "empty value",
			input:    "",
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLatLng(tt.input)
			if result != tt.expected {
				t.Errorf("parseLatLng(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name:     "integer value",
			input:    "100",
			expected: 100.0,
		},
		{
			name:     "decimal value",
			input:    "75.5",
			expected: 75.5,
		},
		{
			name:     "zero value",
			input:    "0",
			expected: 0.0,
		},
		{
			name:     "negative value",
			input:    "-25",
			expected: -25.0,
		},
		{
			name:     "value with spaces",
			input:    "  42.5  ",
			expected: 42.5,
		},
		{
			name:     "invalid value",
			input:    "not_a_number",
			expected: 0.0,
		},
		{
			name:     "empty value",
			input:    "",
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseValue(tt.input)
			if result != tt.expected {
				t.Errorf("parseValue(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapParser_Parse_OutOfBounds(t *testing.T) {
	parser := &MapParser{}
	mockLog := &mockLogger{}

	lines := []string{
		"<<map>>",
	}

	ctx := &ParseContext{
		Mode:        "strict",
		CurrentLine: 0,
		Logger:      mockLog,
		Lines:       lines,
	}

	// Intentar parsear más allá del final de las líneas
	result := parser.Parse(ctx, len(lines))

	// Debe retornar resultado vacío sin error
	if result.Error != nil {
		t.Errorf("Parse() error = %v, expected nil", result.Error)
	}

	if result.Element != nil {
		t.Errorf("Parse() Element = %v, expected nil", result.Element)
	}

	if result.ConsumedLines != 0 {
		t.Errorf("ConsumedLines = %v, expected 0", result.ConsumedLines)
	}
}

func TestMapParser_Parse_MinimalContent(t *testing.T) {
	parser := &MapParser{}
	mockLog := &mockLogger{}

	lines := []string{
		"<<map>>",
	}
	ctx := &ParseContext{
		Mode:        "strict",
		CurrentLine: 0,
		Logger:      mockLog,
		Lines:       lines,
	}

	result := parser.Parse(ctx, 0)

	// Verificar que no hay errores
	if result.Error != nil {
		t.Fatalf("Parse() error = %v, expected nil", result.Error)
	}

	// Verificar elemento del mapa
	mapElement, ok := result.Element.(*ast.MapElement)
	if !ok {
		t.Fatalf("Parse() Element type = %T, expected *ast.MapElement", result.Element)
	}

	// Verificar valores por defecto
	if mapElement.MapType != "world" {
		t.Errorf("MapType = %v, expected 'world'", mapElement.MapType)
	}

	if len(mapElement.Markers) != 0 {
		t.Errorf("Markers length = %v, expected 0", len(mapElement.Markers))
	}

	// Verificar líneas consumidas
	if result.ConsumedLines != 1 {
		t.Errorf("ConsumedLines = %v, expected 1", result.ConsumedLines)
	}
}

func TestMapParser_Parse_NoLogger(t *testing.T) {
	parser := &MapParser{}

	lines := []string{
		"<<map>>",
		"  type: world",
		"  zoom: 5",
	}

	ctx := &ParseContext{
		Mode:        "strict",
		CurrentLine: 0,
		Logger:      nil, // Sin logger
		Lines:       lines,
	}

	// Debe funcionar sin error incluso sin logger
	result := parser.Parse(ctx, 0)

	if result.Error != nil {
		t.Fatalf("Parse() error = %v, expected nil", result.Error)
	}

	if result.Element == nil {
		t.Fatal("Parse() Element = nil, expected MapElement")
	}

	mapElement, ok := result.Element.(*ast.MapElement)
	if !ok {
		t.Fatalf("Parse() Element type = %T, expected *ast.MapElement", result.Element)
	}

	if mapElement.Zoom != 5 {
		t.Errorf("Zoom = %v, expected 5", mapElement.Zoom)
	}
}

// Benchmark para medir el rendimiento del parser
func BenchmarkMapParser_Parse(b *testing.B) {
	parser := &MapParser{}
	mockLog := &mockLogger{}

	lines := []string{
		"<<map>>",
		"  type: world",
		"  zoom: 3",
		"  heatmap: true",
		"  markers:",
		"  - lat: 40.7128",
		"    lng: -74.0060",
		"    label: \"New York\"",
		"    value: 100",
		"  - lat: 34.0522",
		"    lng: -118.2437",
		"    label: \"Los Angeles\"",
		"    value: 75",
		"  - lat: 41.8781",
		"    lng: -87.6298",
		"    label: \"Chicago\"",
		"    value: 50",
	}
	ctx := &ParseContext{
		Mode:        "flex",
		CurrentLine: 0,
		Logger:      mockLog,
		Lines:       lines,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := parser.Parse(ctx, 0)
		if result.Error != nil {
			b.Fatalf("Parse() error = %v", result.Error)
		}
	}
}

func BenchmarkMapParser_CanParse(b *testing.B) {
	parser := &MapParser{}
	line := "<<map>>"
	mode := "strict"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.CanParse(line, mode)
	}
}
