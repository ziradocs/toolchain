// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChartJSONRule_Apply(t *testing.T) {
	rule := NewChartJSONRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "JSON without comments - no changes",
			input: `## Ventas por Trimestre

<<chart: bar>>
{
  "title": "Ventas Online 2021-2024 (millones MXN)",
  "x": ["2021", "2022", "2023", "2024*"],
  "y": [
    [10, 13, 16, 17]
  ],
  "series": ["Ventas Online"]
}

**Descripción del gráfico:**`,
			expected: `## Ventas por Trimestre

<<chart: bar>>
{
  "title": "Ventas Online 2021-2024 (millones MXN)",
  "x": ["2021", "2022", "2023", "2024*"],
  "y": [
    [10, 13, 16, 17]
  ],
  "series": ["Ventas Online"]
}

**Descripción del gráfico:**`,
		}, {
			name: "JSON with inline comments - real case from chatgtp_4.1.slidelang",
			input: `## Proyección de Ventas

<<chart: line>>
{
  "title": "Proyección de ventas online con nueva estrategia",
  "x": ["2024", "2025"],
  "y": [
    [17, 24]  // Ventas en millones MXN
  ],
  "series": ["Proyección ventas"]
}`, expected: `## Proyección de Ventas

<<chart: line>>
{
  "title": "Proyección de ventas online con nueva estrategia",
  "x": ["2024", "2025"],
  "y": [
    [17, 24]  
  ],
  "series": ["Proyección ventas"]
}`,
		},
		{
			name: "JSON with block comments",
			input: `## Ventas

<<chart: bar>>
{
  "title": "Ventas por mes",
  "x": ["Enero", "Febrero", "Marzo"],
  /* Array de valores de ventas */
  "y": [100, 150, 200],
  "series": ["Ventas 2024"]
}`,
			expected: `## Ventas

<<chart: bar>>
{
  "title": "Ventas por mes",
  "x": ["Enero", "Febrero", "Marzo"],
  
  "y": [100, 150, 200],
  "series": ["Ventas 2024"]
}`,
		}, {
			name: "JSON with trailing commas",
			input: `## Chart with trailing commas

<<chart: bar>>
{
  "title": "Sales Data",
  "data": [
    ["Q1", 45],
    ["Q2", 52],  // Comment to trigger processing
  ],
  "series": ["Revenue",],
}`, expected: `## Chart with trailing commas

<<chart: bar>>
{
  "title": "Sales Data",
  "data": [
    ["Q1", 45],
    ["Q2", 52]],
  "series": ["Revenue"]}`,
		}, {
			name: "Multiple chart blocks with mixed content",
			input: `## Chart 1

<<chart: bar>>
{
  "title": "First Chart", // Primer gráfico
  "data": [["A", 1], ["B", 2]],
  "series": ["Data1"]
}

## Chart 2

<<chart: line>>
{
  "title": "Second Chart",
  "data": [["X", 10], ["Y", 20]], // Datos de línea
  "series": ["Data2"]
}`, expected: `## Chart 1

<<chart: bar>>
{
  "title": "First Chart", 
  "data": [["A", 1], ["B", 2]],
  "series": ["Data1"]
}

## Chart 2

<<chart: line>>
{
  "title": "Second Chart",
  "data": [["X", 10], ["Y", 20]], 
  "series": ["Data2"]
}`,
		}, {
			name: "No chart blocks - no changes",
			input: `## Regular content

This is just regular markdown content without chart blocks.

### Another section

Some more content here.`,
			expected: `## Regular content

This is just regular markdown content without chart blocks.

### Another section

Some more content here.`,
		},
		{
			name: "Real case - chart without comments needs no changes",
			input: `## Situación actual del canal digital

<<chart: bar>>
{
  "title": "Ventas Online 2021-2024 (millones MXN)",
  "x": ["2021", "2022", "2023", "2024*"],
  "y": [
    [10, 13, 16, 17]
  ],
  "series": ["Ventas Online"]
}

- Crecimiento anual promedio`,
			expected: `## Situación actual del canal digital

<<chart: bar>>
{
  "title": "Ventas Online 2021-2024 (millones MXN)",
  "x": ["2021", "2022", "2023", "2024*"],
  "y": [
    [10, 13, 16, 17]
  ],
  "series": ["Ventas Online"]
}

- Crecimiento anual promedio`,
		},
		{
			name: "Mixed comments and trailing commas",
			input: `## Complex JSON

<<chart: bar>>
{
  "title": "Sales Data", // Título del gráfico
  "x": ["Q1", "Q2", "Q3"],
  "y": [100, 200, 150,], // Valores con coma final
  "series": ["Revenue",] /* Serie de datos */
}`, expected: `## Complex JSON

<<chart: bar>>
{
  "title": "Sales Data", 
  "x": ["Q1", "Q2", "Q3"],
  "y": [100, 200, 150], 
  "series": ["Revenue"] 
}`,
		}, {
			name: "Invalid JSON block - no changes",
			input: `## Chart with invalid JSON

<<chart: bar>>
{
  "title": "Broken JSON"
  "data": [invalid json
}

Content after.`,
			expected: `## Chart with invalid JSON

<<chart: bar>>
{
  "title": "Broken JSON"
  "data": [invalid json
}

Content after.`,
		},
		{
			name: "URL with // inside a string value must survive intact (#108, #110)",
			input: `## Chart from remote source

<<chart: bar>>
{
  "type": "bar",
  "source": "https://example.com/data",
  "x": ["Q1", "Q2"],
  "y": [[1, 2]]
}`,
			expected: `## Chart from remote source

<<chart: bar>>
{
  "type": "bar",
  "source": "https://example.com/data",
  "x": ["Q1", "Q2"],
  "y": [[1, 2]]
}`,
		},
		{
			name: "URL in string alongside a real trailing comment - only the real comment is stripped",
			input: `## Mixed URL and comment

<<chart: bar>>
{
  "source": "https://example.com/data", // URL de origen
  "x": ["Q1", "Q2"],
  "y": [[1, 2]]
}`,
			expected: `## Mixed URL and comment

<<chart: bar>>
{
  "source": "https://example.com/data", 
  "x": ["Q1", "Q2"],
  "y": [[1, 2]]
}`,
		},
		{
			// extractJSONBlock's brace counting used to be quote-unaware
			// (same bug class as #108/#110): a literal "}" inside a string
			// value decremented braceCount and made the JSON block end
			// prematurely, right at the "note" line. Any comment on a
			// later line (here, on the "x" line) then fell OUTSIDE the
			// (wrongly) detected block and was never stripped. With the
			// fix, the block boundary is only affected by braces outside
			// strings, so the block extends to the real closing "}" and
			// the later comment is correctly stripped.
			name: "literal brace inside a string value does not truncate the JSON block early",
			input: `## Chart with brace inside string value

<<chart: bar>>
{
  "note": "see appendix }",
  "x": ["Q1", "Q2"], // another comment
  "y": [[1, 2]]
}

Content after the chart.`,
			expected: `## Chart with brace inside string value

<<chart: bar>>
{
  "note": "see appendix }",
  "x": ["Q1", "Q2"], 
  "y": [[1, 2]]
}

Content after the chart.`,
		},
		{
			// extractJSONBlock returned as soon as braceCount hit zero,
			// without scanning the rest of that same line — a trailing
			// comment on the closing-brace line itself (e.g. "} // nota")
			// was never detected, so hasComments stayed false and Apply
			// left the comment in the JSON, which Chart.js can't parse.
			name: "trailing comment on the closing-brace line itself is stripped",
			input: `## Chart with comment on closing brace

<<chart: bar>>
{
  "x": ["Q1", "Q2"],
  "y": [[1, 2]]
} // nota final`,
			expected: `## Chart with comment on closing brace

<<chart: bar>>
{
  "x": ["Q1", "Q2"],
  "y": [[1, 2]]
} `,
		},
		{
			name: "block comment on the closing-brace line itself is stripped",
			input: `## Chart with block comment on closing brace

<<chart: bar>>
{
  "x": ["Q1", "Q2"],
  "y": [[1, 2]]
} /* nota final */`,
			expected: `## Chart with block comment on closing brace

<<chart: bar>>
{
  "x": ["Q1", "Q2"],
  "y": [[1, 2]]
} `,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Errorf("ChartJSONRule.Apply() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("ChartJSONRule.Apply() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestChartJSONRule_CleanJSONComments(t *testing.T) {
	rule := NewChartJSONRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Remove inline comments - real case",
			input: `{
  "title": "Proyección de ventas online con nueva estrategia",
  "x": ["2024", "2025"],
  "y": [
    [17, 24]  // Ventas en millones MXN
  ],
  "series": ["Proyección ventas"]
}`, expected: `{
  "title": "Proyección de ventas online con nueva estrategia",
  "x": ["2024", "2025"],
  "y": [
    [17, 24]  
  ],
  "series": ["Proyección ventas"]
}`,
		},
		{
			name: "Remove block comments",
			input: `{
  "title": "Block Comments",
  /* Este es un comentario
     de múltiples líneas */
  "data": [["X", 10]],
  "series": ["Test"] /* Comentario final */
}`,
			expected: `{
  "title": "Block Comments",
  
  "data": [["X", 10]],
  "series": ["Test"] 
}`,
		},
		{
			name: "No comments or trailing commas",
			input: `{
  "title": "Clean JSON",
  "data": [["A", 1], ["B", 2]],
  "series": ["Test"]
}`,
			expected: `{
  "title": "Clean JSON",
  "data": [["A", 1], ["B", 2]],
  "series": ["Test"]
}`,
		},
		{
			name: "URL with // inside a string value is not treated as a comment (#108, #110)",
			input: `{
  "type": "bar",
  "source": "https://example.com/data",
  "series": ["Test"]
}`,
			expected: `{
  "type": "bar",
  "source": "https://example.com/data",
  "series": ["Test"]
}`,
		},
		{
			name: "URL in string plus a real trailing comment on the same line",
			input: `{
  "source": "https://example.com/data", // nota
  "series": ["Test"]
}`,
			expected: `{
  "source": "https://example.com/data", 
  "series": ["Test"]
}`,
		},
		{
			// An unterminated block comment must NOT be treated as a
			// comment at all (matching the old regexp's behavior, which
			// required an explicit "*/" to match): silently discarding
			// everything from an unclosed "/*" to the end of the string
			// would delete real JSON that comes after a malformed
			// comment opener.
			name: "unterminated block comment is preserved literally, not stripped to end of string",
			input: `{
  "title": "Test", /* unterminated comment
  "data": [1, 2, 3]
}`,
			expected: `{
  "title": "Test", /* unterminated comment
  "data": [1, 2, 3]
}`,
		},
		{
			// A "//" that appears inside a string value delimited by
			// escaped quotes (\"...\") must stay inside the string and
			// not be treated as a comment, while a REAL trailing comment
			// after the string closes must still be stripped.
			name: "escaped quotes inside a string keep an embedded // from being treated as a comment",
			input: `{
  "quote": "She said \"hi // there\"", // real comment
  "x": 1
}`,
			expected: `{
  "quote": "She said \"hi // there\"", 
  "x": 1
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.cleanJSONComments(tt.input)
			if result != tt.expected {
				t.Errorf("cleanJSONComments() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestChartJSONRule_Metadata(t *testing.T) {
	rule := NewChartJSONRule()

	if rule.Priority() != 5 {
		t.Errorf("Priority() = %v, want %v", rule.Priority(), 5)
	}

	expectedDesc := "Limpia comentarios inline en JSON para charts"
	if rule.Description() != expectedDesc {
		t.Errorf("Description() = %v, want %v", rule.Description(), expectedDesc)
	}
}

// BenchmarkChartJSONRule_Apply benchmarks para medir performance
func BenchmarkChartJSONRule_Apply(b *testing.B) {
	rule := NewChartJSONRule()

	// Contenido de prueba con comentarios en JSON
	content := `## Test Chart 1

<<chart: bar>>
{
  "title": "Test Data", // Título del gráfico
  "x": ["A", "B", "C"],
  "y": [[1, 2, 3]], // Datos de prueba
  "series": ["Test"] /* Serie principal */
}

## Test Chart 2

<<chart: line>>
{
  "title": "Line Data",
  "data": [["X", 10], ["Y", 20]],  // Coordenadas
  "series": ["Line",] // Coma final
}

Regular content here.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rule.Apply(content)
		if err != nil {
			b.Fatalf("Error in benchmark: %v", err)
		}
	}
}

// TestChartJSONRule_Integration prueba la regla con archivos reales usando testdata
func TestChartJSONRule_Integration(t *testing.T) {
	rule := NewChartJSONRule()

	// Leer archivo de entrada
	inputPath := filepath.Join("testdata", "chart_json_input.slidelang")
	inputContent, err := os.ReadFile(inputPath)
	if err != nil {
		t.Skipf("Skipping integration test - input file not found: %v", err)
		return
	}

	// Leer archivo esperado
	expectedPath := filepath.Join("testdata", "chart_json_expected.slidelang")
	expectedContent, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Skipf("Skipping integration test - expected file not found: %v", err)
		return
	}

	// Aplicar la regla
	result, err := rule.Apply(string(inputContent))
	if err != nil {
		t.Fatalf("ChartJSONRule.Apply() error = %v", err)
	}

	// Comparar resultado
	if result != string(expectedContent) {
		t.Errorf("Integration test failed. Got:\n%s\n\nExpected:\n%s", result, string(expectedContent))

		// Crear archivo de salida para debugging
		outputPath := filepath.Join("testdata", "chart_json_actual.slidelang")
		if writeErr := os.WriteFile(outputPath, []byte(result), 0644); writeErr == nil {
			t.Logf("Actual output written to: %s", outputPath)
		}
	}
}
