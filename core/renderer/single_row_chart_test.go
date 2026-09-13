// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// decodeChartConfig corre GenerateChartConfigWithMode y devuelve data.labels
// y el primer dataset ya decodificados, para no repetir el mismo
// json.Unmarshal en cada caso de esta serie.
func decodeChartConfig(t *testing.T, chart *ast.ChartElement) (labels []interface{}, dataset map[string]interface{}) {
	t.Helper()

	raw := GenerateChartConfigWithMode(chart, false, nil)

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("el config no es JSON válido: %v\n%s", err, raw)
	}
	data, ok := decoded["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("config sin data{}: %s", raw)
	}
	if l, present := data["labels"]; present {
		labels = l.([]interface{})
	}
	datasets, ok := data["datasets"].([]interface{})
	if !ok || len(datasets) == 0 {
		t.Fatalf("config sin datasets: %s", raw)
	}
	return labels, datasets[0].(map[string]interface{})
}

// TestGenerateChartConfig_FlatSingleRowWithLabels es el caso que YA
// funcionaba correctamente antes de este fix: una fila plana de valores
// numéricos con labels explícitos separados — data: [[45,28,15,12]] +
// labels: [...] — se esparce como UNA serie de 4 valores contra las 4
// labels declaradas (mismo fixture que
// examples/gallery/04_analytics_charts_and_maps.doclang's doughnut).
func TestGenerateChartConfig_FlatSingleRowWithLabels(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	chart.Data = [][]interface{}{{45.0, 28.0, 15.0, 12.0}}
	chart.Labels = []string{"Q1", "Q2", "Q3", "Q4"}

	labels, dataset := decodeChartConfig(t, chart)

	data, ok := dataset["data"].([]interface{})
	if !ok || len(data) != 4 {
		t.Fatalf("dataset.data = %#v, want 4 valores", dataset["data"])
	}
	want := []float64{45, 28, 15, 12}
	for i, w := range want {
		if got, ok := data[i].(float64); !ok || got != w {
			t.Errorf("data[%d] = %v, want %v", i, data[i], w)
		}
	}
	if len(labels) != 4 || labels[0] != "Q1" {
		t.Errorf("labels = %v, want los 4 labels explícitos sin tocar", labels)
	}
}

// TestGenerateChartConfig_SingleRowWithLeadingLabel es el repro del audit
// 2026-09-11 (F3): data: [["Q1", 45, 32]] (una sola fila, primera celda un
// string) sin `series:` declarado. Antes de este fix, la rama "normal"
// asumía SIEMPRE forma tabular (columna 0 = label) y creaba 2 series de 1
// punto cada una ("Q1" tratado como fila de labels, 45 y 32 como 2 series);
// con una sola fila eso deja categorías/series desalineadas. La forma
// correcta es la misma que una tabla de N filas: "Q1" es la categoría
// (label), 45 y 32 son los valores de columna — 2 series de 1 punto sobre
// 1 categoría, no 1 serie plana.
func TestGenerateChartConfig_SingleRowWithLeadingLabel(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	chart.Data = [][]interface{}{{"Q1", 45.0, 32.0}}

	raw := GenerateChartConfigWithMode(chart, false, nil)
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("config inválido: %v\n%s", err, raw)
	}
	data := decoded["data"].(map[string]interface{})
	labels, _ := data["labels"].([]interface{})
	if len(labels) != 1 || labels[0] != "Q1" {
		t.Errorf("labels = %v, want [\"Q1\"]", labels)
	}
	datasets, _ := data["datasets"].([]interface{})
	if len(datasets) != 2 {
		t.Fatalf("datasets = %d, want 2 (una por columna de datos)", len(datasets))
	}
	for i, want := range []float64{45, 32} {
		ds := datasets[i].(map[string]interface{})
		points := ds["data"].([]interface{})
		if len(points) != 1 || points[0].(float64) != want {
			t.Errorf("datasets[%d].data = %v, want [%v]", i, points, want)
		}
		if s, isString := ds["data"].([]interface{})[0].(string); isString {
			t.Fatalf("datasets[%d].data contiene un string %q — la categoría se coló como valor", i, s)
		}
	}
}

// TestGenerateChartConfig_MultiRowTabular es el caso de referencia sin
// ambigüedad de forma — varias filas [label, v1, v2] — que no debe cambiar
// con este fix: sigue siendo 1 categoría por fila, 1 serie por columna.
func TestGenerateChartConfig_MultiRowTabular(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	chart.Data = [][]interface{}{
		{"Q1", 45.0, 32.0},
		{"Q2", 50.0, 30.0},
	}

	labels, _ := decodeChartConfig(t, chart)
	if len(labels) != 2 || labels[0] != "Q1" || labels[1] != "Q2" {
		t.Errorf("labels = %v, want [Q1 Q2]", labels)
	}

	raw := GenerateChartConfigWithMode(chart, false, nil)
	var decoded map[string]interface{}
	_ = json.Unmarshal([]byte(raw), &decoded)
	datasets := decoded["data"].(map[string]interface{})["datasets"].([]interface{})
	if len(datasets) != 2 {
		t.Fatalf("datasets = %d, want 2", len(datasets))
	}
}

// TestGenerateChartConfig_DoughnutFlatSingleRow es la misma fila plana del
// primer test pero por la rama pie/doughnut, que antes solo tomaba row[1]
// (un único valor) de cualquier fila — con una sola fila de 4 celdas eso
// dejaba 3 de los 4 valores fuera del gráfico en silencio.
func TestGenerateChartConfig_DoughnutFlatSingleRow(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "doughnut")
	chart.Data = [][]interface{}{{42.0, 27.0, 18.0, 13.0}}
	chart.Labels = []string{"Organic Search", "Direct", "Referral", "Social"}

	labels, dataset := decodeChartConfig(t, chart)
	if len(labels) != 4 {
		t.Fatalf("labels = %v, want 4", labels)
	}
	data := dataset["data"].([]interface{})
	if len(data) != 4 {
		t.Fatalf("dataset.data = %v, want 4 valores (los 4 de la fila plana)", data)
	}
}
