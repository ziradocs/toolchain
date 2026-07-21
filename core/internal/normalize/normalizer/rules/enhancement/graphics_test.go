// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGraphicsRule_Apply(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		input    string
		expected string
	}{
		{
			name: "Convert simple chart placeholder",
			mode: "convert",
			input: `### Análisis de Ventas

CHART: Ventas mensuales por región

**Descripción del gráfico:**`,
			expected: `### Análisis de Ventas

<chart type="bar" title="Ventas mensuales por región">
<!-- Datos del gráfico aquí -->
</chart>

**Descripción del gráfico:**`,
		},
		{
			name: "Comment chart placeholder",
			mode: "comment",
			input: `### Análisis de Ventas

GRÁFICO: Distribución de productos

**Análisis:**`,
			expected: `### Análisis de Ventas

<!-- GRÁFICO: Distribución de productos -->

**Análisis:**`,
		},
		{
			name: "Remove chart placeholder",
			mode: "remove",
			input: `### Datos importantes

DIAGRAM: Flujo de proceso

Continuamos con el análisis.`,
			expected: `### Datos importantes



Continuamos con el análisis.`,
		}, {
			name: "Multiple language placeholders",
			mode: "convert",
			input: `## Reportes

CHART: Ventas anuales en barras
GRAPHIQUE: Distribution des clients circular
DIAGRAMM: Prozessfluss

## Fin`,
			expected: `## Reportes

<chart type="bar" title="Ventas anuales en barras">
<!-- Datos del gráfico aquí -->
</chart>
<chart type="pie" title="Distribution des clients circular">
<!-- Datos del gráfico aquí -->
</chart>
<chart type="flow" title="Prozessfluss">
<!-- Datos del gráfico aquí -->
</chart>

## Fin`,
		}, {
			name: "Should not affect valid SlideLang elements",
			mode: "convert",
			input: `### Gráficos Válidos

<<chart: bar>>
{
  "title": "Ventas Online 2021-2024 (millones MXN)",
  "x": ["2021", "2022", "2023", "2024*"],
  "y": [
    [10, 13, 16, 17]
  ]
}

CHART: Este sí es un placeholder

<<mermaid>>
flowchart TD
    A[Inicio] --> B[Fin]

DIAGRAM: Otro flow placeholder aquí`,
			expected: `### Gráficos Válidos

<<chart: bar>>
{
  "title": "Ventas Online 2021-2024 (millones MXN)",
  "x": ["2021", "2022", "2023", "2024*"],
  "y": [
    [10, 13, 16, 17]
  ]
}

<chart type="bar" title="Este sí es un placeholder">
<!-- Datos del gráfico aquí -->
</chart>

<<mermaid>>
flowchart TD
    A[Inicio] --> B[Fin]

<chart type="flow" title="Otro flow placeholder aquí">
<!-- Datos del gráfico aquí -->
</chart>`,
		}, {
			name: "Should not affect SlideLang near placeholders without obvious keywords",
			mode: "convert",
			input: `### Análisis

<<chart: line>>
data: [1, 2, 3, 4]

CHART: Este contenido está cerca de elementos válidos

title: "Gráfico de líneas"
labels: ["A", "B", "C"]`,
			expected: `### Análisis

<<chart: line>>
data: [1, 2, 3, 4]

CHART: Este contenido está cerca de elementos válidos

title: "Gráfico de líneas"
labels: ["A", "B", "C"]`,
		}, {
			name: "Should detect AI placeholder patterns",
			mode: "convert",
			input: `## Sección

CHART: Insertar gráfico aquí
GRÁFICO: Agregar descripción aquí
DIAGRAM: Placeholder for flow here

## Fin`,
			expected: `## Sección

<chart type="bar" title="Insertar gráfico aquí">
<!-- Datos del gráfico aquí -->
</chart>
<chart type="bar" title="Agregar descripción aquí">
<!-- Datos del gráfico aquí -->
</chart>
<chart type="flow" title="Placeholder for flow here">
<!-- Datos del gráfico aquí -->
</chart>

## Fin`,
		},
		{
			name: "Should not affect complex chart descriptions",
			mode: "convert",
			input: `### Resultados

CHART: Análisis detallado de ventas trimestrales con comparación año anterior mostrando crecimiento del 15%

## Conclusiones`,
			expected: `### Resultados

CHART: Análisis detallado de ventas trimestrales con comparación año anterior mostrando crecimiento del 15%

## Conclusiones`,
		},
		{
			name: "No placeholders found",
			mode: "convert",
			input: `## Regular content

This is just regular markdown content without chart placeholders.

### Another section

Some more content here with no graphics mentioned.`,
			expected: `## Regular content

This is just regular markdown content without chart placeholders.

### Another section

Some more content here with no graphics mentioned.`,
		},
		{
			name: "Default mode should be comment",
			mode: "",
			input: `### Test

CHART: Simple placeholder

Content after.`,
			expected: `### Test

<!-- CHART: Simple placeholder -->

Content after.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewGraphicsRule(tt.mode)
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Errorf("GraphicsRule.Apply() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("GraphicsRule.Apply() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGraphicsRule_IsGraphicPlaceholder(t *testing.T) {
	rule := NewGraphicsRule("convert")

	tests := []struct {
		name     string
		line     string
		keyword  string
		expected bool
	}{
		{
			name:     "Simple chart placeholder",
			line:     "CHART: Simple description",
			keyword:  "CHART",
			expected: true,
		},
		{
			name:     "Placeholder with 'aquí'",
			line:     "GRÁFICO: Insertar aquí",
			keyword:  "GRÁFICO",
			expected: true,
		},
		{
			name:     "Placeholder with 'here'",
			line:     "DIAGRAM: Add diagram here",
			keyword:  "DIAGRAM",
			expected: true,
		},
		{
			name:     "Short placeholder description",
			line:     "CHART: Test",
			keyword:  "CHART",
			expected: true,
		},
		{
			name:     "SlideLang element should not match",
			line:     "<<chart: bar>>",
			keyword:  "CHART",
			expected: false,
		}, {
			name:     "Complex chart description should not match",
			line:     "CHART: Análisis detallado de ventas trimestrales con comparación año anterior mostrando crecimiento del 15%",
			keyword:  "CHART",
			expected: false,
		},
		{
			name:     "No colon should not match",
			line:     "CHART without colon",
			keyword:  "CHART",
			expected: false,
		},
		{
			name:     "Different keyword should not match",
			line:     "CHART: Description",
			keyword:  "GRAPH",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isGraphicPlaceholder(tt.line, tt.keyword)
			if result != tt.expected {
				t.Errorf("isGraphicPlaceholder(%q, %q) = %v, want %v", tt.line, tt.keyword, result, tt.expected)
			}
		})
	}
}

func TestGraphicsRule_IsSlideLangElement(t *testing.T) {
	rule := NewGraphicsRule("convert")

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "Chart element",
			line:     "<<chart: bar>>",
			expected: true,
		},
		{
			name:     "Mermaid element",
			line:     "<<mermaid>>",
			expected: true,
		},
		{
			name:     "Image element",
			line:     "<<image: path/to/image.png>>",
			expected: true,
		},
		{
			name:     "Map element",
			line:     "<<map: coordinates>>",
			expected: true,
		},
		{
			name:     "Any element with << >>",
			line:     "<<custom: element>>",
			expected: true,
		},
		{
			name:     "Regular text",
			line:     "This is regular text",
			expected: false,
		},
		{
			name:     "Chart placeholder",
			line:     "CHART: Description",
			expected: false,
		},
		{
			name:     "Incomplete element",
			line:     "<<chart: bar",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isSlideLangElement(tt.line)
			if result != tt.expected {
				t.Errorf("isSlideLangElement(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestGraphicsRule_IsNearValidSlideLangElement(t *testing.T) {
	rule := NewGraphicsRule("convert")
	lines := []string{
		"# Title",
		"<<chart: bar>>",
		"data: [1, 2, 3]",
		"",
		"CHART: This placeholder is near valid elements",
		"",
		"title: Chart title",
		"labels: [A, B, C]",
		"",
		"",
		"",
		"",
		"CHART: This one is far from valid elements",
		"",
		"Regular text here",
		"More regular text",
	}

	tests := []struct {
		name         string
		currentIndex int
		expected     bool
	}{
		{
			name:         "Near chart element at index 1",
			currentIndex: 4, // Line with "CHART: This placeholder is near valid elements"
			expected:     true,
		},
		{
			name:         "Near title/labels at index 6-7",
			currentIndex: 4, // Same line, should detect title/labels nearby
			expected:     true,
		}, {
			name:         "Far from valid elements",
			currentIndex: 12, // Line with "CHART: This one is far from valid elements"
			expected:     false,
		},
		{
			name:         "At the beginning",
			currentIndex: 0,
			expected:     true, // Near <<chart: bar>> at index 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isNearValidSlideLangElement(lines, tt.currentIndex)
			if result != tt.expected {
				t.Errorf("isNearValidSlideLangElement() at index %d = %v, want %v", tt.currentIndex, result, tt.expected)
			}
		})
	}
}

func TestGraphicsRule_CreateChartBlock(t *testing.T) {
	rule := NewGraphicsRule("convert")

	tests := []struct {
		name        string
		chartType   string
		description string
		expected    string
	}{
		{
			name:        "Bar chart",
			chartType:   "bar",
			description: "Sales data",
			expected: `<chart type="bar" title="Sales data">
<!-- Datos del gráfico aquí -->
</chart>`,
		},
		{
			name:        "Pie chart",
			chartType:   "pie",
			description: "Distribution",
			expected: `<chart type="pie" title="Distribution">
<!-- Datos del gráfico aquí -->
</chart>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.createChartBlock(tt.chartType, tt.description)
			if result != tt.expected {
				t.Errorf("createChartBlock(%q, %q) = %v, want %v", tt.chartType, tt.description, result, tt.expected)
			}
		})
	}
}

func TestGraphicsRule_Metadata(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{"convert", "convert"},
		{"comment", "comment"},
		{"remove", "remove"},
		{"default", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewGraphicsRule(tt.mode)

			if rule.Priority() != 3 {
				t.Errorf("Priority() = %v, want %v", rule.Priority(), 3)
			}

			expectedMode := tt.mode
			if expectedMode == "" {
				expectedMode = "comment"
			}
			expectedDesc := "Maneja placeholders de gráficos generados por AI (modo: " + expectedMode + ")"
			if rule.Description() != expectedDesc {
				t.Errorf("Description() = %v, want %v", rule.Description(), expectedDesc)
			}
		})
	}
}

// TestGraphicsRule_Integration prueba la regla con archivos reales usando testdata
func TestGraphicsRule_Integration(t *testing.T) {
	rule := NewGraphicsRule("convert")

	// Leer archivo de entrada
	inputPath := filepath.Join("testdata", "graphics_input.slidelang")
	inputContent, err := os.ReadFile(inputPath)
	if err != nil {
		t.Skipf("Skipping integration test - input file not found: %v", err)
		return
	}

	// Leer archivo esperado
	expectedPath := filepath.Join("testdata", "graphics_expected.slidelang")
	expectedContent, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Skipf("Skipping integration test - expected file not found: %v", err)
		return
	}

	// Aplicar la regla
	result, err := rule.Apply(string(inputContent))
	if err != nil {
		t.Fatalf("GraphicsRule.Apply() error = %v", err)
	}

	// Comparar resultado
	if result != string(expectedContent) {
		t.Errorf("Integration test failed. Got:\n%s\n\nExpected:\n%s", result, string(expectedContent))

		// Crear archivo de salida para debugging
		outputPath := filepath.Join("testdata", "graphics_actual.slidelang")
		if writeErr := os.WriteFile(outputPath, []byte(result), 0644); writeErr == nil {
			t.Logf("Actual output written to: %s", outputPath)
		}
	}
}

// BenchmarkGraphicsRule_Apply benchmarks para medir performance
func BenchmarkGraphicsRule_Apply(b *testing.B) {
	rule := NewGraphicsRule("convert")

	// Contenido de prueba con múltiples placeholders
	content := `## Test

CHART: Ventas mensuales
GRÁFICO: Distribución de productos
DIAGRAM: Flujo de proceso

<<chart: bar>>
data: [1, 2, 3]

GRAPH: Otro placeholder
TABLE: Datos tabulares

Regular content here.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rule.Apply(content)
		if err != nil {
			b.Fatalf("Error in benchmark: %v", err)
		}
	}
}
