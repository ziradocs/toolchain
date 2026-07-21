// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"testing"
)

func TestMermaidFormatterRule_Apply(t *testing.T) {
	rule := NewMermaidFormatterRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Format flowchart without indentation",
			input: `## Análisis del problema actual

<<mermaid>>
flowchart TD
    A[Tráfico alto en web] --> B[Baja tasa de conversión]
    B --> C[Falta de personalización en campañas]
    B --> D[Experiencia de compra poco optimizada]

- **Abandono de carrito:** 74%`,
			expected: `## Análisis del problema actual

<<mermaid>>
  flowchart TD
  A[Tráfico alto en web] --> B[Baja tasa de conversión]
  B --> C[Falta de personalización en campañas]
  B --> D[Experiencia de compra poco optimizada]

- **Abandono de carrito:** 74%`,
		},
		{
			name: "Format gantt chart without indentation",
			input: `## Roadmap

<<mermaid>>
gantt
    dateFormat  YYYY-MM
    title Roadmap Estrategia Digital
    section Fase 1: Preparación
    Análisis y benchmarking       :done, 2024-07, 2024-07
    Selección de herramientas     :done, 2024-07, 2024-07

---`,
			expected: `## Roadmap

<<mermaid>>
  gantt
  dateFormat  YYYY-MM
  title Roadmap Estrategia Digital
  section Fase 1: Preparación
  Análisis y benchmarking       :done, 2024-07, 2024-07
  Selección de herramientas     :done, 2024-07, 2024-07

---`,
		},
		{
			name: "Format sequence diagram",
			input: `## Proceso

<<mermaid>>
sequenceDiagram
    participant U as User
    participant S as Server
    U->>S: Request
    S-->>U: Response

### Siguiente sección`,
			expected: `## Proceso

<<mermaid>>
  sequenceDiagram
  participant U as User
  participant S as Server
  U->>S: Request
  S-->>U: Response

### Siguiente sección`,
		},
		{
			name: "Already formatted content (no changes)",
			input: `## Diagrama

<<mermaid>>
  graph TD
  A --> B
  B --> C

Content after`,
			expected: `## Diagrama

<<mermaid>>
  graph TD
  A --> B
  B --> C

Content after`,
		},
		{
			name: "Multiple mermaid blocks",
			input: `## Diagrama 1

<<mermaid>>
flowchart TD
    A --> B

## Diagrama 2

<<mermaid>>
pie title Distribución
    "A" : 45
    "B" : 55

Final content`,
			expected: `## Diagrama 1

<<mermaid>>
  flowchart TD
  A --> B

## Diagrama 2

<<mermaid>>
  pie title Distribución
  "A" : 45
  "B" : 55

Final content`,
		},
		{
			name: "Empty mermaid block",
			input: `## Empty

<<mermaid>>

Content after`,
			expected: `## Empty

<<mermaid>>

Content after`,
		},
		{
			name: "Mixed indentation (some correct, some incorrect)",
			input: `## Mixed

<<mermaid>>
  graph TD
A --> B
  C --> D
    E --> F

Next content`,
			expected: `## Mixed

<<mermaid>>
  graph TD
  A --> B
  C --> D
  E --> F

Next content`,
		},
		{
			name: "Content with special characters and quotes",
			input: `## Special chars

<<mermaid>>
flowchart LR
    S[Visita web/app]
    S --> T["Personalización de experiencia"]
    T --> U[Campañas automatizadas]

Text after`,
			expected: `## Special chars

<<mermaid>>
  flowchart LR
  S[Visita web/app]
  S --> T["Personalización de experiencia"]
  T --> U[Campañas automatizadas]

Text after`,
		},
		{
			name: "No mermaid blocks",
			input: `## Regular content

This is just regular content without mermaid blocks.

### Another section

Some more content here.`,
			expected: `## Regular content

This is just regular content without mermaid blocks.

### Another section

Some more content here.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Errorf("MermaidFormatterRule.Apply() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("MermaidFormatterRule.Apply() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMermaidFormatterRule_FormatMermaidContent(t *testing.T) {
	rule := NewMermaidFormatterRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Simple flowchart formatting",
			input: `flowchart TD
    A[Start] --> B[End]
    B --> C[Finish]`,
			expected: `  flowchart TD
  A[Start] --> B[End]
  B --> C[Finish]`,
		},
		{
			name: "Gantt chart formatting",
			input: `gantt
    dateFormat  YYYY-MM
    title Project Timeline
    section Phase 1
    Task 1 :2024-01, 2024-02`,
			expected: `  gantt
  dateFormat  YYYY-MM
  title Project Timeline
  section Phase 1
  Task 1 :2024-01, 2024-02`,
		},
		{
			name: "Already formatted content",
			input: `  graph TD
  A --> B
  B --> C`,
			expected: `  graph TD
  A --> B
  B --> C`,
		},
		{
			name: "Content with empty lines",
			input: `flowchart TD
    A[Start] --> B[End]

    B --> C[Finish]


`,
			expected: `  flowchart TD
  A[Start] --> B[End]

  B --> C[Finish]`,
		},
		{
			name: "Mixed indentation",
			input: `  graph TD
A --> B
    C --> D`,
			expected: `  graph TD
  A --> B
  C --> D`,
		},
		{
			name:     "Empty content",
			input:    ``,
			expected: ``,
		},
		{
			name: "Only whitespace",
			input: `   
   
`,
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.formatMermaidContent(tt.input)
			if result != tt.expected {
				t.Errorf("formatMermaidContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMermaidFormatterRule_NeedsFormatting(t *testing.T) {
	rule := NewMermaidFormatterRule()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name: "Needs formatting - no indentation",
			input: `graph TD
A --> B`,
			expected: true,
		},
		{
			name: "Needs formatting - mixed indentation",
			input: `  graph TD
A --> B
  C --> D`,
			expected: true,
		},
		{
			name: "Needs formatting - too much indentation",
			input: `    graph TD
    A --> B`,
			expected: true,
		},
		{
			name: "No formatting needed - correct indentation",
			input: `  graph TD
  A --> B
  C --> D`,
			expected: false,
		},
		{
			name:     "No formatting needed - empty content",
			input:    ``,
			expected: false,
		},
		{
			name: "No formatting needed - only whitespace",
			input: `   

   `,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.needsFormatting(tt.input)
			if result != tt.expected {
				t.Errorf("needsFormatting() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMermaidFormatterRule_Metadata(t *testing.T) {
	rule := NewMermaidFormatterRule()

	if rule.Priority() != 5 {
		t.Errorf("Priority() = %v, want %v", rule.Priority(), 5)
	}

	expectedDesc := "Formatea bloques <<mermaid>> agregando la indentación requerida (2 espacios) para compatibilidad con el parser flex"
	if rule.Description() != expectedDesc {
		t.Errorf("Description() = %v, want %v", rule.Description(), expectedDesc)
	}
}

// TestMermaidFormatterRule_Integration prueba la regla con el archivo real de ChatGPT
func TestMermaidFormatterRule_Integration(t *testing.T) {
	rule := NewMermaidFormatterRule()

	// Contenido simulando el archivo chatgtp_4.1.slidelang
	chatGPTContent := `---
title: "Implementación de nueva estrategia de marketing digital"
author: "Misael Monterroca"
mode: flex
---

## Análisis del problema actual

::: warning
A pesar de mayor inversión, la tasa de conversión se estancó y el ROI publicitario cayó 22% en el último año.
:::

<<mermaid>>
flowchart TD
    A[Tráfico alto en web] --> B[Baja tasa de conversión]
    B --> C[Falta de personalización en campañas]
    B --> D[Experiencia de compra poco optimizada]
    C --> E[Segmentación limitada]
    D --> F[Procesos de pago confusos]
    F --> G[Abandono de carrito alto]

- **Abandono de carrito:** 74%

---

## Propuesta de nueva estrategia digital

<<mermaid>>
flowchart LR
    S[Visita web/app]
    S --> T[Personalización de experiencia]
    T --> U[Campañas automatizadas]
    U --> V[Chatbot y atención personalizada]
    V --> W[Checkout optimizado]
    W --> X[Venta y remarketing]

---

## Implementación: Roadmap

<<mermaid>>
gantt
    dateFormat  YYYY-MM
    title Roadmap Estrategia Digital
    section Fase 1: Preparación
    Análisis y benchmarking       :done, 2024-07, 2024-07
    Selección de herramientas     :done, 2024-07, 2024-07
    section Fase 2: Ejecución
    Rediseño UX/UI                :active, 2024-08, 2024-09
    Setup de automatización       :2024-08, 2024-09

---`

	expected := `---
title: "Implementación de nueva estrategia de marketing digital"
author: "Misael Monterroca"
mode: flex
---

## Análisis del problema actual

::: warning
A pesar de mayor inversión, la tasa de conversión se estancó y el ROI publicitario cayó 22% en el último año.
:::

<<mermaid>>
  flowchart TD
  A[Tráfico alto en web] --> B[Baja tasa de conversión]
  B --> C[Falta de personalización en campañas]
  B --> D[Experiencia de compra poco optimizada]
  C --> E[Segmentación limitada]
  D --> F[Procesos de pago confusos]
  F --> G[Abandono de carrito alto]

- **Abandono de carrito:** 74%

---

## Propuesta de nueva estrategia digital

<<mermaid>>
  flowchart LR
  S[Visita web/app]
  S --> T[Personalización de experiencia]
  T --> U[Campañas automatizadas]
  U --> V[Chatbot y atención personalizada]
  V --> W[Checkout optimizado]
  W --> X[Venta y remarketing]

---

## Implementación: Roadmap

<<mermaid>>
  gantt
  dateFormat  YYYY-MM
  title Roadmap Estrategia Digital
  section Fase 1: Preparación
  Análisis y benchmarking       :done, 2024-07, 2024-07
  Selección de herramientas     :done, 2024-07, 2024-07
  section Fase 2: Ejecución
  Rediseño UX/UI                :active, 2024-08, 2024-09
  Setup de automatización       :2024-08, 2024-09

---`

	// Aplicar la regla
	result, err := rule.Apply(chatGPTContent)
	if err != nil {
		t.Fatalf("MermaidFormatterRule.Apply() error = %v", err)
	}

	// Comparar resultado
	if result != expected {
		t.Errorf("Integration test failed. Got:\n%s\n\nExpected:\n%s", result, expected)
	}
}

// BenchmarkMermaidFormatterRule_Apply benchmarks para medir performance
func BenchmarkMermaidFormatterRule_Apply(b *testing.B) {
	rule := NewMermaidFormatterRule()

	// Contenido de prueba con múltiples bloques mermaid sin formatear
	content := `## Test 1

<<mermaid>>
flowchart TD
    A --> B
    B --> C

## Test 2

<<mermaid>>
gantt
    dateFormat YYYY-MM
    title Timeline
    section Phase 1
    Task 1 :2024-01, 2024-02

## Test 3

<<mermaid>>
sequenceDiagram
    A->>B: Hello
    B-->>A: Hi

Regular content here.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rule.Apply(content)
		if err != nil {
			b.Fatalf("Error in benchmark: %v", err)
		}
	}
}
