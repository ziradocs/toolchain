// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestConvertChartElementToChartJS_FlatSingleRowWithLabels es el caso que
// YA funcionaba correctamente: una fila plana de valores numéricos con
// labels explícitos separados — data: [[45,28,15,12]] + labels: [...] — se
// esparce como UNA serie de 4 valores contra las 4 labels declaradas.
func TestConvertChartElementToChartJS_FlatSingleRowWithLabels(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	chart.Data = [][]interface{}{{45.0, 28.0, 15.0, 12.0}}
	chart.Labels = []string{"Q1", "Q2", "Q3", "Q4"}

	config := ConvertChartElementToChartJS(chart, "chart-0", nil)

	if len(config.Data.Datasets) != 1 {
		t.Fatalf("esperaba 1 dataset, hay %d", len(config.Data.Datasets))
	}
	got := config.Data.Datasets[0].Data
	want := []interface{}{45.0, 28.0, 15.0, 12.0}
	if len(got) != len(want) {
		t.Fatalf("Data = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Data[%d] = %v, want %v", i, got[i], want[i])
		}
	}
	if len(config.Data.Labels) != 4 || config.Data.Labels[0] != "Q1" {
		t.Errorf("Labels = %v, want los 4 labels explícitos sin tocar", config.Data.Labels)
	}
}

// TestConvertChartElementToChartJS_SingleRowWithLeadingLabel es el repro
// del audit 2026-09-11 (F3): data: [["Q1", 45, 32]] (una sola fila, primera
// celda un string) sin `series:` declarado. Antes de este fix,
// createDatasetsFromData decidía solo por len(chart.Data)==1 y esparcía la
// fila ENTERA como datos — "Q1" incluido — produciendo un dataset con 3
// "valores" (uno de ellos un string) contra 1 sola label derivada
// ("Q1"), un desalineamiento silencioso. La forma correcta es la misma que
// una tabla de N filas: "Q1" es la categoría (label), 45/32 son los
// valores de columna.
func TestConvertChartElementToChartJS_SingleRowWithLeadingLabel(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	chart.Data = [][]interface{}{{"Q1", 45.0, 32.0}}
	// Sin chart.Labels ni chart.Series — se derivan de los datos.

	config := ConvertChartElementToChartJS(chart, "chart-0", nil)

	if len(config.Data.Datasets) != 1 {
		t.Fatalf("esperaba 1 dataset, hay %d", len(config.Data.Datasets))
	}
	data := config.Data.Datasets[0].Data
	for _, v := range data {
		if _, isString := v.(string); isString {
			t.Fatalf("Data = %v contiene un string — \"Q1\" se coló como si fuera un valor numérico", data)
		}
	}
	if len(data) != 1 || data[0] != 45.0 {
		t.Errorf("Data = %v, want [45] (la segunda columna de la única fila, misma regla que el caso multi-fila)", data)
	}
	if len(config.Data.Labels) != 1 || config.Data.Labels[0] != "Q1" {
		t.Errorf("Labels = %v, want [\"Q1\"] derivado de la primera columna", config.Data.Labels)
	}
}

// TestConvertChartElementToChartJS_EmptyRow_DoesNotPanic cubre un hallazgo
// de revisión independiente: data: [[]] (una sola fila, VACÍA) pasaba el
// guard len(chart.Data)==0 (hay 1 fila) y entraba en pánico en
// chart.Data[0][0] — el schema no prohíbe una fila vacía, y un AST armado a
// mano (--filter, un consumidor de la API) tampoco. No hay ningún valor que
// graficar, así que el dataset resultante queda vacío en vez de tirar el
// build entero.
func TestConvertChartElementToChartJS_EmptyRow_DoesNotPanic(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	chart.Data = [][]interface{}{{}}

	config := ConvertChartElementToChartJS(chart, "chart-0", nil)

	if len(config.Data.Datasets) != 1 {
		t.Fatalf("esperaba 1 dataset, hay %d", len(config.Data.Datasets))
	}
	if len(config.Data.Datasets[0].Data) != 0 {
		t.Errorf("Data = %v, want vacío (la única fila no tiene celdas)", config.Data.Datasets[0].Data)
	}
}
