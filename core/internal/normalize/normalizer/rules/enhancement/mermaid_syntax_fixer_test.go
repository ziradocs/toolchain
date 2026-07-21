// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"testing"
)

func TestMermaidSyntaxFixerRule_Apply(t *testing.T) {
	rule := NewMermaidSyntaxFixerRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Fix gantt dateFormat from YYYY-MM to YYYY-MM-DD",
			input: `## Roadmap

<<mermaid>>
  gantt
  dateFormat  YYYY-MM
  title Project Timeline
  section Phase 1
  Task 1 :2024-01, 2024-02

---`,
			expected: `## Roadmap

<<mermaid>>
  gantt
  dateFormat  YYYY-MM-DD
  title Project Timeline
  section Phase 1
  Task 1 :2024-01-01, 2024-02-28

---`,
		},
		{
			name: "Fix gantt task dates from YYYY-MM to YYYY-MM-DD",
			input: `## Implementation

<<mermaid>>
  gantt
  dateFormat  YYYY-MM-DD
  title Roadmap Estrategia Digital
  section Fase 1: Preparación
  Análisis y benchmarking       :done, 2024-07, 2024-07
  Selección de herramientas     :done, 2024-07, 2024-07
  section Fase 2: Ejecución
  Rediseño UX/UI                :active, 2024-08, 2024-09
  Setup de automatización       :2024-08, 2024-09

Next section`,
			expected: `## Implementation

<<mermaid>>
  gantt
  dateFormat  YYYY-MM-DD
  title Roadmap Estrategia Digital
  section Fase 1: Preparación
  Análisis y benchmarking       :done, 2024-07-01, 2024-07-31
  Selección de herramientas     :done, 2024-07-01, 2024-07-31
  section Fase 2: Ejecución
  Rediseño UX/UI                :active, 2024-08-01, 2024-09-30
  Setup de automatización       :2024-08-01, 2024-09-30

Next section`,
		},
		{
			name: "Fix complex gantt with multiple sections",
			input: `## Timeline

<<mermaid>>
  gantt
  dateFormat  YYYY-MM
  title Complex Project Timeline
  section Phase 1
  Task A :done, 2024-01, 2024-02
  Task B :active, 2024-02, 2024-03
  section Phase 2
  Task C :2024-03, 2024-04
  Task D :2024-04, 2024-06

Final content`,
			expected: `## Timeline

<<mermaid>>
  gantt
  dateFormat  YYYY-MM-DD
  title Complex Project Timeline
  section Phase 1
  Task A :done, 2024-01-01, 2024-02-28
  Task B :active, 2024-02-01, 2024-03-31
  section Phase 2
  Task C :2024-03-01, 2024-04-30
  Task D :2024-04-01, 2024-06-30

Final content`,
		},
		{
			name: "No changes for non-gantt diagrams",
			input: `## Flow

<<mermaid>>
  flowchart TD
  A[Start] --> B[End]
  B --> C[Finish]

Content after`,
			expected: `## Flow

<<mermaid>>
  flowchart TD
  A[Start] --> B[End]
  B --> C[Finish]

Content after`,
		},
		{
			name: "No changes for already correct gantt dates",
			input: `## Correct Gantt

<<mermaid>>
  gantt
  dateFormat  YYYY-MM-DD
  title Already Correct
  section Phase 1
  Task 1 :2024-01-01, 2024-02-28
  Task 2 :2024-02-01, 2024-03-31

End`,
			expected: `## Correct Gantt

<<mermaid>>
  gantt
  dateFormat  YYYY-MM-DD
  title Already Correct
  section Phase 1
  Task 1 :2024-01-01, 2024-02-28
  Task 2 :2024-02-01, 2024-03-31

End`,
		},
		{
			name: "Multiple mermaid blocks with mixed types",
			input: `## Multiple Diagrams

<<mermaid>>
  flowchart TD
  A --> B

## Gantt Chart

<<mermaid>>
  gantt
  dateFormat  YYYY-MM
  title Timeline
  section Phase 1
  Task 1 :2024-01, 2024-02

## Sequence

<<mermaid>>
  sequenceDiagram
  A->>B: Hello

Final`,
			expected: `## Multiple Diagrams

<<mermaid>>
  flowchart TD
  A --> B

## Gantt Chart

<<mermaid>>
  gantt
  dateFormat  YYYY-MM-DD
  title Timeline
  section Phase 1
  Task 1 :2024-01-01, 2024-02-28

## Sequence

<<mermaid>>
  sequenceDiagram
  A->>B: Hello

Final`,
		},
		{
			name: "Handle February dates correctly",
			input: `## February Test

<<mermaid>>
  gantt
  dateFormat  YYYY-MM
  title February Timeline
  section Tasks
  Feb Task :2024-02, 2024-02

End`,
			expected: `## February Test

<<mermaid>>
  gantt
  dateFormat  YYYY-MM-DD
  title February Timeline
  section Tasks
  Feb Task :2024-02-01, 2024-02-28

End`,
		},
		{
			name: "Handle different month lengths",
			input: `## Month Lengths

<<mermaid>>
  gantt
  dateFormat  YYYY-MM
  title Various Months
  section Tasks
  Jan Task :2024-01, 2024-01
  Apr Task :2024-04, 2024-04
  Dec Task :2024-12, 2024-12

End`,
			expected: `## Month Lengths

<<mermaid>>
  gantt
  dateFormat  YYYY-MM-DD
  title Various Months
  section Tasks
  Jan Task :2024-01-01, 2024-01-31
  Apr Task :2024-04-01, 2024-04-30
  Dec Task :2024-12-01, 2024-12-31

End`,
		},
		{
			name: "No changes for empty mermaid block",
			input: `## Empty

<<mermaid>>

Content after`,
			expected: `## Empty

<<mermaid>>

Content after`,
		},
		{
			name: "No changes for non-mermaid content",
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
				t.Errorf("MermaidSyntaxFixerRule.Apply() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("MermaidSyntaxFixerRule.Apply() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMermaidSyntaxFixerRule_FixGanttSyntax(t *testing.T) {
	rule := NewMermaidSyntaxFixerRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Fix dateFormat line",
			input:    "  dateFormat  YYYY-MM",
			expected: "  dateFormat  YYYY-MM-DD",
		},
		{
			name:     "Fix simple task with :done prefix",
			input:    "  Task 1 :done, 2024-01, 2024-02",
			expected: "  Task 1 :done, 2024-01-01, 2024-02-28",
		},
		{
			name:     "Fix simple task with :active prefix",
			input:    "  Task 2 :active, 2024-03, 2024-04",
			expected: "  Task 2 :active, 2024-03-01, 2024-04-30",
		},
		{
			name:     "Fix simple task without prefix",
			input:    "  Task 3 :2024-05, 2024-06",
			expected: "  Task 3 :2024-05-01, 2024-06-30",
		},
		{
			name:     "No change for already correct dateFormat",
			input:    "  dateFormat  YYYY-MM-DD",
			expected: "  dateFormat  YYYY-MM-DD",
		},
		{
			name:     "No change for already correct task dates",
			input:    "  Task 1 :done, 2024-01-01, 2024-02-28",
			expected: "  Task 1 :done, 2024-01-01, 2024-02-28",
		},
		{
			name:     "No change for non-gantt lines",
			input:    "  flowchart TD",
			expected: "  flowchart TD",
		},
		{
			name:     "No change for section headers",
			input:    "  section Phase 1",
			expected: "  section Phase 1",
		},
		{
			name:     "No change for title lines",
			input:    "  title Project Timeline",
			expected: "  title Project Timeline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.fixGanttSyntax(tt.input)
			if result != tt.expected {
				t.Errorf("fixGanttSyntax() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMermaidSyntaxFixerRule_ConvertToFullDate(t *testing.T) {
	rule := NewMermaidSyntaxFixerRule()

	tests := []struct {
		name     string
		dateStr  string
		isStart  bool
		expected string
	}{
		{
			name:     "January start date",
			dateStr:  "2024-01",
			isStart:  true,
			expected: "2024-01-01",
		},
		{
			name:     "January end date",
			dateStr:  "2024-01",
			isStart:  false,
			expected: "2024-01-31",
		},
		{
			name:     "February start date",
			dateStr:  "2024-02",
			isStart:  true,
			expected: "2024-02-01",
		},
		{
			name:     "February end date",
			dateStr:  "2024-02",
			isStart:  false,
			expected: "2024-02-28",
		},
		{
			name:     "April end date (30 days)",
			dateStr:  "2024-04",
			isStart:  false,
			expected: "2024-04-30",
		},
		{
			name:     "December end date",
			dateStr:  "2024-12",
			isStart:  false,
			expected: "2024-12-31",
		},
		{
			name:     "Invalid format (no change)",
			dateStr:  "2024",
			isStart:  true,
			expected: "2024",
		},
		{
			name:     "Already full date (no change)",
			dateStr:  "2024-01-15",
			isStart:  true,
			expected: "2024-01-15",
		},
		{
			name:     "Invalid month (fallback to 01)",
			dateStr:  "2024-13",
			isStart:  false,
			expected: "2024-13-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.convertToFullDate(tt.dateStr, tt.isStart)
			if result != tt.expected {
				t.Errorf("convertToFullDate(%q, %v) = %q, want %q", tt.dateStr, tt.isStart, result, tt.expected)
			}
		})
	}
}

func TestMermaidSyntaxFixerRule_Metadata(t *testing.T) {
	rule := NewMermaidSyntaxFixerRule()

	if rule.Priority() != 6 {
		t.Errorf("Priority() = %v, want %v", rule.Priority(), 6)
	}

	expectedDesc := "Corrige errores de sintaxis específicos en diagramas Mermaid (fechas en gantt, etc.)"
	if rule.Description() != expectedDesc {
		t.Errorf("Description() = %v, want %v", rule.Description(), expectedDesc)
	}
}

// TestMermaidSyntaxFixerRule_Integration prueba la regla con el archivo real de ChatGPT
func TestMermaidSyntaxFixerRule_Integration(t *testing.T) {
	rule := NewMermaidSyntaxFixerRule()

	// Contenido simulando el archivo chatgtp_4.1.slidelang con el problema del gantt
	chatGPTContent := `---
title: "Implementación de nueva estrategia de marketing digital"
author: "Misael Monterroca"
mode: flex
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
  Integración chatbots          :2024-09, 2024-10
  Personalización campañas      :2024-09, 2024-11
  section Fase 3: Optimización
  Pruebas A/B y ajustes         :2024-10, 2025-01
  Medición y escalamiento       :2024-12, 2025-03

---`

	expected := `---
title: "Implementación de nueva estrategia de marketing digital"
author: "Misael Monterroca"
mode: flex
---

## Implementación: Roadmap

<<mermaid>>
  gantt
  dateFormat  YYYY-MM-DD
  title Roadmap Estrategia Digital
  section Fase 1: Preparación
  Análisis y benchmarking       :done, 2024-07-01, 2024-07-31
  Selección de herramientas     :done, 2024-07-01, 2024-07-31
  section Fase 2: Ejecución
  Rediseño UX/UI                :active, 2024-08-01, 2024-09-30
  Setup de automatización       :2024-08-01, 2024-09-30
  Integración chatbots          :2024-09-01, 2024-10-31
  Personalización campañas      :2024-09-01, 2024-11-30
  section Fase 3: Optimización
  Pruebas A/B y ajustes         :2024-10-01, 2025-01-31
  Medición y escalamiento       :2024-12-01, 2025-03-31

---`

	// Aplicar la regla
	result, err := rule.Apply(chatGPTContent)
	if err != nil {
		t.Fatalf("MermaidSyntaxFixerRule.Apply() error = %v", err)
	}

	// Comparar resultado
	if result != expected {
		t.Errorf("Integration test failed. Got:\n%s\n\nExpected:\n%s", result, expected)
	}
}

// BenchmarkMermaidSyntaxFixerRule_Apply benchmarks para medir performance
func BenchmarkMermaidSyntaxFixerRule_Apply(b *testing.B) {
	rule := NewMermaidSyntaxFixerRule()

	// Contenido de prueba con múltiples bloques gantt sin corregir
	content := `## Test 1

<<mermaid>>
  gantt
  dateFormat YYYY-MM
  title Timeline 1
  section Phase 1
  Task 1 :2024-01, 2024-02

## Test 2

<<mermaid>>
  gantt
  dateFormat YYYY-MM
  title Timeline 2
  section Phase 1
  Task 2 :2024-03, 2024-04

## Test 3

<<mermaid>>
  flowchart TD
  A --> B

Regular content here.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rule.Apply(content)
		if err != nil {
			b.Fatalf("Error in benchmark: %v", err)
		}
	}
}
