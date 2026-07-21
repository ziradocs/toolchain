// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/ast"
)

func TestChartParser_SimpleInlineFormat(t *testing.T) {
	// IMPORTANTE: Este formato es el que usa plan_proyecto_erp_2026.doclang
	// después de la normalización (>> se convierte a <</chart>>)
	input := `<<chart
type: bar
title: "Hitos del Proyecto"
labels: ["Análisis", "Desarrollo", "Pruebas", "Despliegue"]
data: [2, 5, 2, 1]
color: "#16A085"
<</chart>>`

	lines := strings.Split(input, "\n")
	ctx := &ParseContext{
		Mode:        "flex",
		CurrentLine: 0,
		Lines:       lines,
	}

	parser := &ChartParser{}

	t.Logf("Input lines:")
	for i, line := range lines {
		t.Logf("  [%d] %q", i, line)
	}

	result := parser.Parse(ctx, 0)

	if result.Element == nil {
		t.Fatal("Expected chart element, got nil")
	}

	t.Logf("Consumed lines: %d", result.ConsumedLines)

	chart, ok := result.Element.(*ast.ChartElement)
	if !ok {
		t.Fatalf("Expected *ast.ChartElement, got %T", result.Element)
	}

	// Verificar tipo
	if chart.ChartType != "bar" {
		t.Errorf("Expected chart type 'bar', got '%s'", chart.ChartType)
	}

	// Verificar título
	if chart.Title != "Hitos del Proyecto" {
		t.Errorf("Expected title 'Hitos del Proyecto', got '%s'", chart.Title)
	}

	// Verificar labels
	expectedLabels := []string{"Análisis", "Desarrollo", "Pruebas", "Despliegue"}
	if len(chart.Labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(chart.Labels))
	}
	for i, label := range expectedLabels {
		if i < len(chart.Labels) && chart.Labels[i] != label {
			t.Errorf("Label %d: expected '%s', got '%s'", i, label, chart.Labels[i])
		}
	}

	// Verificar data
	if len(chart.Data) == 0 {
		t.Fatal("Expected chart data, got empty")
	}
	if len(chart.Data[0]) != 4 {
		t.Errorf("Expected 4 data points, got %d", len(chart.Data[0]))
	}

	expectedData := []interface{}{2, 5, 2, 1}
	for i, expected := range expectedData {
		if i < len(chart.Data[0]) {
			got := chart.Data[0][i]
			if got != expected {
				t.Errorf("Data point %d: expected %v, got %v", i, expected, got)
			}
		}
	}

	t.Logf("✅ Chart parsed successfully")
	t.Logf("  Type: %s", chart.ChartType)
	t.Logf("  Title: %s", chart.Title)
	t.Logf("  Labels: %v", chart.Labels)
	t.Logf("  Data: %v", chart.Data)
}
