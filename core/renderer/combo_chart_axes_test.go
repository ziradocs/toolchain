// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// decodeComboConfig corre GenerateChartConfigWithMode sobre un combo chart
// y devuelve los yAxisID de cada dataset (en orden) más el mapa de escalas.
func decodeComboConfig(t *testing.T, chart *ast.ChartElement) (axisIDs []string, scales map[string]interface{}) {
	t.Helper()

	raw := GenerateChartConfigWithMode(chart, false, nil)
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("config inválido: %v\n%s", err, raw)
	}
	datasets := decoded["data"].(map[string]interface{})["datasets"].([]interface{})
	for _, raw := range datasets {
		ds := raw.(map[string]interface{})
		id, _ := ds["yAxisID"].(string)
		axisIDs = append(axisIDs, id)
	}
	options, _ := decoded["options"].(map[string]interface{})
	if options != nil {
		scales, _ = options["scales"].(map[string]interface{})
	}
	return axisIDs, scales
}

func comboChart(seriesAxes []string) *ast.ChartElement {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "combo")
	chart.SeriesTypes = []string{"bar", "line"}
	chart.Series = []string{"Revenue ($M)", "Margin (%)"}
	chart.SeriesAxes = seriesAxes
	chart.Data = [][]interface{}{
		{"Q1", 120.0, 18.0},
		{"Q2", 145.0, 21.0},
	}
	return chart
}

// TestGenerateChartConfig_ComboNoDeclaredAxes_FallsBackToIndexHeuristic
// cubre el combo chart preexistente al campo SeriesAxes — ninguna serie
// declaró yAxisID nunca (examples/gallery/04_charts_showcase.slidelang,
// que solo trae type:/data:/series:) — y no debe cambiar de aspecto:
// índice>0 sigue yendo a "y1", igual que el heurístico que ya usa
// slidelang/internal/generator/data/converter.go.
func TestGenerateChartConfig_ComboNoDeclaredAxes_FallsBackToIndexHeuristic(t *testing.T) {
	axisIDs, scales := decodeComboConfig(t, comboChart(nil))

	if len(axisIDs) != 2 {
		t.Fatalf("datasets = %d, want 2", len(axisIDs))
	}
	if axisIDs[0] != "" {
		t.Errorf("datasets[0].yAxisID = %q, want \"\" (escala primaria implícita)", axisIDs[0])
	}
	if axisIDs[1] != "y1" {
		t.Errorf("datasets[1].yAxisID = %q, want \"y1\" (heurístico índice>0)", axisIDs[1])
	}
	if _, ok := scales["y"]; !ok {
		t.Error("scales.y ausente")
	}
	if _, ok := scales["y1"]; !ok {
		t.Error("scales.y1 ausente pese al heurístico índice>0")
	}
}

// TestGenerateChartConfig_ComboDeclaredAxes_HonorsAuthor cubre F2 (audit
// 2026-09-11): con yAxisID explícito, el config lo respeta literalmente en
// vez de recalcularlo por índice — antes de este fix, SeriesAxes ni
// siquiera existía, así que esto era imposible de declarar.
func TestGenerateChartConfig_ComboDeclaredAxes_HonorsAuthor(t *testing.T) {
	axisIDs, scales := decodeComboConfig(t, comboChart([]string{"", "y1"}))

	if axisIDs[0] != "" || axisIDs[1] != "y1" {
		t.Fatalf("axisIDs = %v, want [\"\" \"y1\"]", axisIDs)
	}
	if _, ok := scales["y"]; !ok {
		t.Error("scales.y ausente")
	}
	if _, ok := scales["y1"]; !ok {
		t.Error("scales.y1 ausente")
	}
}

// TestGenerateChartConfig_ComboSingleSeries_NoExtraScale confirma que un
// combo de una sola serie (o donde ninguna serie cae después del índice 0)
// no le agrega una escala y1 de sobra que nadie referencia.
func TestGenerateChartConfig_ComboSingleSeries_NoExtraScale(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "combo")
	chart.SeriesTypes = []string{"bar"}
	chart.Series = []string{"Revenue ($M)"}
	chart.Data = [][]interface{}{{"Q1", 120.0}, {"Q2", 145.0}}

	_, scales := decodeComboConfig(t, chart)
	if _, ok := scales["y"]; !ok {
		t.Error("scales.y ausente")
	}
	if _, ok := scales["y1"]; ok {
		t.Error("scales.y1 no debería existir sin una segunda serie")
	}
}
