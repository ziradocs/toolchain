// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalizer

import (
	"strings"
	"testing"
)

func TestDetector_ValidSlideLangFileNotDetectedAsAI(t *testing.T) {
	detector := NewDetector()

	// Contenido exacto del archivo 10_advanced_elements_flex.slidelang que está causando problemas
	content := `---
mode: flex
title: "Advanced Elements Examples"
author: "SlideLang Documentation"
---

## Diagrama de Flujo

<<mermaid>>
  graph TD
      A[Inicio] --> B{¿Decisión?}
      B -->|Sí| C[Hacer algo]
      B -->|No| D[Hacer otra cosa]
      C --> E[Fin]
      D --> E

---

## Sequence Diagram

<<mermaid>>
  sequenceDiagram
      participant U as User
      participant S as Server
      participant DB as Database
      
      U->>S: Request
      S->>DB: Query
      DB-->>S: Results
      S-->>U: Response

---

## Sales Performance

<<chart: bar>>
  data: [
    ["Q1", 45, 32, 28],
    ["Q2", 52, 38, 35],
    ["Q3", 61, 45, 42],
    ["Q4", 73, 51, 48]
  ]
  series: ["Product A", "Product B", "Product C"]
  options:
    responsive: true
    plugins:
      title:
        display: true
        text: "Quarterly Sales by Product"

---

## Combined Chart Example

<<chart: combo>>
  type: ["bar", "bar", "line"]
  data: [
    ["Jan", 65, 28, 45],
    ["Feb", 59, 48, 52],
    ["Mar", 80, 40, 61],
    ["Apr", 81, 19, 73]
  ]
  series: ["Desktop", "Mobile", "Total"]
  options:
    responsive: true
    plugins:
      legend:
        position: top
      title:
        display: true
        text: "Traffic by Device"

---

## Global Presence

<<map>>
  type: world
  markers:
    - lat: 40.7128
      lng: -74.0060
      label: "New York"
      value: 45
    - lat: 51.5074
      lng: -0.1278
      label: "London"
      value: 38
    - lat: 35.6762
      lng: 139.6503
      label: "Tokyo"
      value: 52
  heatmap: true
  zoom: 2

---

## Regional Office Locations

<<map>>
  type: region
  center: [39.8283, -98.5795]
  markers:
    - lat: 37.7749
      lng: -122.4194
      label: "San Francisco"
      color: "blue"
    - lat: 41.8781
      lng: -87.6298
      label: "Chicago"
      color: "red"
    - lat: 25.7617
      lng: -80.1918
      label: "Miami"
      color: "green"
  zoom: 4`

	result := detector.Detect(content)

	// Debug: mostrar todos los patrones encontrados
	t.Logf("AI Detection Result: Detected=%v, Score=%.2f, Patterns=%d",
		result.Detected, result.Score, len(result.Patterns))

	for _, pattern := range result.Patterns {
		t.Logf("  Pattern: %s (line %d, confidence: %.2f): %s",
			pattern.Type, pattern.Line, pattern.Confidence, pattern.Description)
	}

	// Verificar líneas específicas que están siendo reportadas como problemáticas
	malformedCharts := result.GetPatternsByType("malformed_chart")
	if len(malformedCharts) > 0 {
		lines := strings.Split(content, "\n")
		for _, pattern := range malformedCharts {
			if pattern.Line > 0 && pattern.Line <= len(lines) {
				actualLine := lines[pattern.Line-1]
				t.Logf("  Line %d: '%s'", pattern.Line, actualLine)
				// Verificar si realmente está mal indentada
				if strings.HasPrefix(actualLine, "  ") || strings.HasPrefix(actualLine, "\t") {
					t.Errorf("Line %d is properly indented but detected as malformed: '%s'", pattern.Line, actualLine)
				}
			}
		}
	}

	// Este archivo NO debería ser detectado como AI-generated porque es válido SlideLang
	if result.Detected && len(malformedCharts) > 0 {
		t.Errorf("Valid SlideLang file incorrectly detected as AI-generated due to malformed_chart patterns")
	}
}
