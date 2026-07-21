// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMermaidRule_Apply(t *testing.T) {
	rule := NewMermaidRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Simple mermaid graph conversion",
			input: `### Diagrama de Tipos de Ataques

` + "```mermaid" + `
graph TD
    A[Ciberataques a PyMEs] --> B[Phishing 35%]
    A --> C[Ransomware 28%]
    A --> D[Malware 18%]
` + "```" + `

**Descripción del diagrama:**`, expected: `### Diagrama de Tipos de Ataques

<<mermaid>>
  graph TD
  A[Ciberataques a PyMEs] --> B[Phishing 35%]
  A --> C[Ransomware 28%]
  A --> D[Malware 18%]

**Descripción del diagrama:**`,
		},
		{
			name: "Sequence diagram conversion",
			input: `## Proceso de Respuesta

` + "```mermaid" + `
sequenceDiagram
    participant U as User
    participant S as Server
    U->>S: Request
    S-->>U: Response
` + "```" + `

### Puntos Clave:`, expected: `## Proceso de Respuesta

<<mermaid>>
  sequenceDiagram
  participant U as User
  participant S as Server
  U->>S: Request
  S-->>U: Response

### Puntos Clave:`,
		},
		{
			name: "Multiple mermaid blocks",
			input: `## Diagrama 1

` + "```mermaid" + `
graph LR
    A --> B
` + "```" + `

## Diagrama 2

` + "```mermaid" + `
pie title Distribución
    "A" : 45
    "B" : 55
` + "```" + ``, expected: `## Diagrama 1

<<mermaid>>
  graph LR
  A --> B

## Diagrama 2

<<mermaid>>
  pie title Distribución
  "A" : 45
  "B" : 55`,
		},
		{
			name: "No mermaid blocks",
			input: `## Regular content

This is just regular markdown content without mermaid blocks.

### Another section

Some more content here.`,
			expected: `## Regular content

This is just regular markdown content without mermaid blocks.

### Another section

Some more content here.`,
		},
		{
			name: "Empty mermaid block",
			input: `## Empty diagram

` + "```mermaid" + `

` + "```" + `

Content after.`,
			expected: `## Empty diagram

<<mermaid>>

Content after.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Errorf("MermaidRule.Apply() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("MermaidRule.Apply() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMermaidRule_CleanMermaidContent(t *testing.T) {
	rule := NewMermaidRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{{
		name: "Clean simple graph",
		input: `graph TD
    A[Start] --> B[End]
    B --> C[Finish]`,
		expected: `  graph TD
  A[Start] --> B[End]
  B --> C[Finish]`,
	},
		{
			name: "Clean with empty lines",
			input: `graph TD
    A[Start] --> B[End]

    B --> C[Finish]


`,
			expected: `  graph TD
  A[Start] --> B[End]

  B --> C[Finish]`,
		},
		{
			name: "Already indented content",
			input: `  graph TD
      A[Start] --> B[End]`,
			expected: `  graph TD
  A[Start] --> B[End]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.cleanMermaidContent(tt.input)
			if result != tt.expected {
				t.Errorf("cleanMermaidContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMermaidRule_Metadata(t *testing.T) {
	rule := NewMermaidRule()

	if rule.Priority() != 4 {
		t.Errorf("Priority() = %v, want %v", rule.Priority(), 4)
	}

	expectedDesc := "Convierte bloques de código Mermaid de formato markdown (```mermaid) al formato SlideLang (<<mermaid>>)"
	if rule.Description() != expectedDesc {
		t.Errorf("Description() = %v, want %v", rule.Description(), expectedDesc)
	}
}

// TestMermaidRule_Integration prueba la regla con archivos reales usando testdata
func TestMermaidRule_Integration(t *testing.T) {
	rule := NewMermaidRule()

	// Leer archivo de entrada
	inputPath := filepath.Join("testdata", "mermaid_input.slidelang")
	inputContent, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("Error reading input file: %v", err)
	}

	// Leer archivo esperado
	expectedPath := filepath.Join("testdata", "mermaid_expected.slidelang")
	expectedContent, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Error reading expected file: %v", err)
	}

	// Aplicar la regla
	result, err := rule.Apply(string(inputContent))
	if err != nil {
		t.Fatalf("MermaidRule.Apply() error = %v", err)
	}

	// Comparar resultado
	if result != string(expectedContent) {
		t.Errorf("Integration test failed. Got:\n%s\n\nExpected:\n%s", result, string(expectedContent))

		// Crear archivo de salida para debugging
		outputPath := filepath.Join("testdata", "mermaid_actual.slidelang")
		if writeErr := os.WriteFile(outputPath, []byte(result), 0644); writeErr == nil {
			t.Logf("Actual output written to: %s", outputPath)
		}
	}
}

// BenchmarkMermaidRule_Apply benchmarks para medir performance
func BenchmarkMermaidRule_Apply(b *testing.B) {
	rule := NewMermaidRule()

	// Contenido de prueba con múltiples bloques mermaid
	content := `## Test

` + "```mermaid" + `
graph TD
    A --> B
    B --> C
` + "```" + `

## Test 2

` + "```mermaid" + `
sequenceDiagram
    A->>B: Hello
    B-->>A: Hi
` + "```" + `

Regular content here.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rule.Apply(content)
		if err != nil {
			b.Fatalf("Error in benchmark: %v", err)
		}
	}
}
