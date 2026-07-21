// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/ast"
)

func TestChartParser_CanParse(t *testing.T) {
	parser := &ChartParser{}

	tests := []struct {
		name     string
		line     string
		mode     string
		expected bool
	}{
		{"strict inline", "<<chart: bar>>", "strict", true},
		{"strict multiline", "<<chart", "strict", true},
		{"flex inline", "<<chart: line>>", "flex", true},
		{"flex multiline", "<<chart", "flex", true},
		{"not chart", "some text", "strict", false},
		{"wrong prefix", "<<diagram", "strict", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.CanParse(tt.line, tt.mode)
			if result != tt.expected {
				t.Errorf("CanParse() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestChartParser_Parse_SimpleBar(t *testing.T) {
	parser := &ChartParser{}
	ctx := &ParseContext{
		Lines: []string{
			"<<chart: bar>>",
			"data: [100, 200, 300]",
			"labels: [\"Q1\", \"Q2\", \"Q3\"]",
			"<</chart>>",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	chart, ok := result.Element.(*ast.ChartElement)
	if !ok {
		t.Fatal("Element is not ChartElement")
	}

	if chart.ChartType != "bar" {
		t.Errorf("ChartType = %v, want bar", chart.ChartType)
	}

	if len(chart.Labels) != 3 {
		t.Errorf("len(Labels) = %v, want 3", len(chart.Labels))
	}

	if len(chart.Data) == 0 {
		t.Error("Data should not be empty")
	}
}

func TestChartParser_Parse_WithDimensions(t *testing.T) {
	parser := &ChartParser{}
	ctx := &ParseContext{
		Lines: []string{
			`<<chart: line width="1200" height="800">>`,
			"<</chart>>",
		},
	}

	result := parser.Parse(ctx, 0)
	chart := result.Element.(*ast.ChartElement)

	if chart.Width != 1200 {
		t.Errorf("Width = %v, want 1200", chart.Width)
	}
	if chart.Height != 800 {
		t.Errorf("Height = %v, want 800", chart.Height)
	}
}

// TestChartParser_Parse_NoData_SerializesAsEmptyArray cubre issue #8: un chart
// sin ninguna línea "data:" no debe serializar "data" como JSON null (viola el
// JSON Schema del contrato), sino como [].
func TestChartParser_Parse_NoData_SerializesAsEmptyArray(t *testing.T) {
	parser := &ChartParser{}
	ctx := &ParseContext{
		Lines: []string{
			`<<chart: bar>>`,
			"<</chart>>",
		},
	}

	result := parser.Parse(ctx, 0)
	chart := result.Element.(*ast.ChartElement)

	if chart.Data == nil {
		t.Fatal("Data is nil; want an empty (non-nil) slice")
	}

	data, err := json.Marshal(chart)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if decoded["data"] == nil {
		t.Errorf("serialized data is null, want []: %s", data)
	}
}

// TestChartParser_Parse_EmptyMultiLineData_SerializesAsEmptyArray cubre el caso
// de un bloque "data: [" ... "]" vacío o sin filas válidas: parseMultiLineArray
// puede retornar un slice nil, que se asigna directo a chart.Data antes del
// guard final - debe seguir serializando como [] no null.
func TestChartParser_Parse_EmptyMultiLineData_SerializesAsEmptyArray(t *testing.T) {
	parser := &ChartParser{}
	ctx := &ParseContext{
		Lines: []string{
			`<<chart: bar>>`,
			"data: [",
			"]",
			"<</chart>>",
		},
	}

	result := parser.Parse(ctx, 0)
	chart := result.Element.(*ast.ChartElement)

	if chart.Data == nil {
		t.Fatal("Data is nil; want an empty (non-nil) slice")
	}

	data, err := json.Marshal(chart)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if decoded["data"] == nil {
		t.Errorf("serialized data is null, want []: %s", data)
	}
}

func TestChartParser_ParseArrayRow(t *testing.T) {
	parser := &ChartParser{}

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"simple numbers", "[100, 200, 300]", 3},
		{"with strings", `["Q1", 100, 200]`, 3},
		{"trailing comma", "[100, 200],", 2},
		{"empty", "[]", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := parser.parseArrayRow(tt.input)
			if len(row) != tt.expected {
				t.Errorf("len(row) = %v, want %v", len(row), tt.expected)
			}
		})
	}
}

func TestChartParser_ParseQuotedStrings(t *testing.T) {
	parser := &ChartParser{}

	tests := []struct {
		input    string
		expected int
	}{
		{`["Q1", "Q2", "Q3"]`, 3},
		{`"single"`, 1},
		{`labels: ["Jan", "Feb"]`, 2},
		{"no quotes", 0},
	}

	for _, tt := range tests {
		result := parser.parseQuotedStrings(tt.input)
		if len(result) != tt.expected {
			t.Errorf("parseQuotedStrings(%q) len = %v, want %v", tt.input, len(result), tt.expected)
		}
	}
}

func TestChartParser_ParseNumber(t *testing.T) {
	parser := &ChartParser{}

	tests := []struct {
		input   string
		wantNil bool
	}{
		{"123", false},
		{"45.67", false},
		{"-100", false},
		{"not a number", true},
		{"", true},
	}

	for _, tt := range tests {
		result := parser.parseNumber(tt.input)
		if (result == nil) != tt.wantNil {
			t.Errorf("parseNumber(%q) nil = %v, want nil = %v", tt.input, result == nil, tt.wantNil)
		}
	}
}

func TestChartParser_ParseJSONBlock(t *testing.T) {
	parser := &ChartParser{}

	lines := []string{
		`{"type":"bar","data":{"labels":["A","B"],"datasets":[{"data":[10,20]}]}}`,
		"<</chart>>",
	}

	json, consumed := parser.parseJSONBlock(lines, 0)

	if json == "" {
		t.Error("JSON should not be empty")
	}
	if consumed != 2 {
		t.Errorf("consumed = %v, want 2", consumed)
	}
}

// TestChartParser_ParseJSONBlock_MalformedStopsAtSeparator cubre issue #12e2:
// cuando las llaves nunca balancean (JSON truncado/mal formado), el bloque no
// debe tragarse el separador de slide "---" ni el resto del documento — antes,
// el fallback de líneas no balanceadas devolvía TODO lo que quedaba, línea
// por línea, hasta el final del archivo.
func TestChartParser_ParseJSONBlock_MalformedStopsAtSeparator(t *testing.T) {
	parser := &ChartParser{}

	lines := []string{
		`{"type":"bar","data":{`, // llave sin cerrar: braceCount nunca vuelve a 0
		"---",
		"# Next slide",
		"Some unrelated content",
	}

	json, consumed := parser.parseJSONBlock(lines, 0)

	if strings.Contains(json, "---") {
		t.Errorf("malformed JSON block leaked the slide separator: %q", json)
	}
	if strings.Contains(json, "Next slide") {
		t.Errorf("malformed JSON block leaked content past the slide separator: %q", json)
	}
	if consumed != 1 {
		t.Errorf("consumed = %v, want 1 (only the malformed JSON line itself, leaving '---' for the slide parser)", consumed)
	}
}

// TestChartParser_ParseJSONBlock_BoundaryInsideStringDoesNotBreak cubre una
// regresión encontrada en code-review de #12e2: el nuevo check de límite no
// debe dispararse si el texto "---" o "<</chart>>" aparece DENTRO de un
// valor JSON de tipo string (p.ej. una descripción documentando la sintaxis
// del DSL) — solo debe cortar cuando esas líneas son un límite real, no
// contenido de string.
func TestChartParser_ParseJSONBlock_BoundaryInsideStringDoesNotBreak(t *testing.T) {
	parser := &ChartParser{}

	lines := []string{
		`{"type":"bar","description":"`,
		`---`,
		`","data":{"labels":["A"],"datasets":[{"data":[1]}]}}`,
	}

	json, consumed := parser.parseJSONBlock(lines, 0)

	if json == "" {
		t.Fatal("expected a legitimately-balanced JSON block to be returned, got empty string")
	}
	if consumed != 3 {
		t.Errorf("consumed = %v, want 3 (all three lines belong to one legitimate JSON value)", consumed)
	}
}

// TestChartParser_ParseJSONBlock_MalformedStopsAtOtherBoundaries cubre el
// hallazgo de code-review de que el check original solo reconocía 2 de los
// 5 límites que el loop de propiedades ya conocía (isChartContentBoundary):
// un bloque malformado seguido de "<<end>>", de otro elemento ("<<...>>"), o
// de un heading ("##") tampoco debe tragárselos.
func TestChartParser_ParseJSONBlock_MalformedStopsAtOtherBoundaries(t *testing.T) {
	parser := &ChartParser{}

	tests := []struct {
		name    string
		lines   []string
		leakStr string
	}{
		{
			name:    "<<end>>",
			lines:   []string{`{"type":"bar","data":{`, "<<end>>", "Next content"},
			leakStr: "<<end>>",
		},
		{
			name:    "new element marker",
			lines:   []string{`{"type":"bar","data":{`, "<<image: foo.png>>", "Next content"},
			leakStr: "<<image",
		},
		{
			name:    "subsection heading",
			lines:   []string{`{"type":"bar","data":{`, "## Next section", "Next content"},
			leakStr: "Next section",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			json, consumed := parser.parseJSONBlock(tt.lines, 0)
			if strings.Contains(json, tt.leakStr) {
				t.Errorf("malformed JSON block leaked past a %s boundary: %q", tt.name, json)
			}
			if consumed != 1 {
				t.Errorf("consumed = %v, want 1 (only the malformed JSON line itself)", consumed)
			}
		})
	}
}

func TestChartParser_ParseInlineMatrix(t *testing.T) {
	parser := &ChartParser{}

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"single array", "[[100, 200, 300]]", 1},
		{"multiple arrays", "[[100, 200], [300, 400]]", 2},
		{"empty", "[[]]", 0}, // Empty array should produce 0 rows
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseInlineMatrix(tt.input)
			if len(result) != tt.expected {
				t.Errorf("len(result) = %v, want %v", len(result), tt.expected)
			}
		})
	}
}

// TestChartParser_Parse_JSONMode_NoDoubleEncoding cubre issue #11: el config de un
// chart en modo JSON directo debe quedar como json.RawMessage (objeto JSON real al
// serializar), nunca como string re-escapado ("[object Object]"-style bugs).
func TestChartParser_Parse_JSONMode_NoDoubleEncoding(t *testing.T) {
	parser := &ChartParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			`<<chart: bar>>`,
			`{"type":"bar","data":{"labels":["A","B"],"datasets":[{"label":"S1","data":[1,2]}]}}`,
			`<</chart>>`,
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	chart, ok := result.Element.(*ast.ChartElement)
	if !ok {
		t.Fatal("Element is not ChartElement")
	}

	if !chart.IsJSONMode {
		t.Fatal("expected IsJSONMode = true")
	}
	if len(chart.RawJSON) == 0 {
		t.Fatal("expected RawJSON to be populated")
	}

	// Serializar el elemento completo, tal como lo hace --format json
	serialized, err := json.Marshal(chart)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(serialized, &decoded); err != nil {
		t.Fatalf("failed to decode serialized chart: %v", err)
	}

	rawJSON, ok := decoded["rawJSON"]
	if !ok {
		t.Fatal("expected 'rawJSON' key in serialized output")
	}

	// Debe ser un objeto JSON real (map), NUNCA un string
	if _, isString := rawJSON.(string); isString {
		t.Fatalf("rawJSON serialized as a string (double-encoded), want nested object: %v", rawJSON)
	}
	rawJSONMap, ok := rawJSON.(map[string]interface{})
	if !ok {
		t.Fatalf("rawJSON is not an object: %T", rawJSON)
	}
	if rawJSONMap["type"] != "bar" {
		t.Errorf("rawJSON.type = %v, want bar", rawJSONMap["type"])
	}

	// La forma serializada nunca debe contener el literal "[object Object]"
	if strings.Contains(string(serialized), "[object Object]") {
		t.Fatal("serialized chart contains literal \"[object Object]\"")
	}
}

// TestChartParser_Parse_JSONMode_InvalidJSON_FallsBack cubre el caso defensivo:
// un bloque que empieza con "{" pero no es JSON válido no debe activar IsJSONMode
// con contenido corrupto.
func TestChartParser_Parse_JSONMode_InvalidJSON_FallsBack(t *testing.T) {
	parser := &ChartParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			`<<chart: bar>>`,
			`{not valid json,,,}`,
			`<</chart>>`,
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	chart, ok := result.Element.(*ast.ChartElement)
	if !ok {
		t.Fatal("Element is not ChartElement")
	}

	if chart.IsJSONMode {
		t.Fatal("expected IsJSONMode = false for invalid JSON block")
	}
	if len(chart.RawJSON) != 0 {
		t.Errorf("expected RawJSON to be empty for invalid JSON, got %q", string(chart.RawJSON))
	}
}

func TestExtractAttribute(t *testing.T) {
	tests := []struct {
		str      string
		attr     string
		expected string
	}{
		{`bar width="1200" height="600"`, "width", "1200"},
		{`bar width="1200" height="600"`, "height", "600"},
		{`line width='800'`, "width", "800"},
		{"no attributes", "width", ""},
		{`invalid width="`, "width", ""},
	}

	for _, tt := range tests {
		result := extractAttribute(tt.str, tt.attr)
		if result != tt.expected {
			t.Errorf("extractAttribute(%q, %q) = %q, want %q", tt.str, tt.attr, result, tt.expected)
		}
	}
}
