// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"testing"

	"github.com/go-analyze/charts"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// El render nativo dibuja un null como hueco (el centinela de la librería)
// en su propia serie y fila; no corre valores ni aborta el render.
func TestNativeChartKeepsNullAsGap(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	chart.Series = []string{"S1", "S2"}
	chart.Data = [][]interface{}{{"A", 1, 10}, {"B", nil, 20}, {"C", 3, nil}}
	values, labels, err := chartSeriesValues(chart)
	if err != nil {
		t.Fatal(err)
	}
	null := charts.GetNullValue()
	if values[0][1] != null || values[1][2] != null || values[1][1] != 20 || values[0][2] != 3 || labels[1] != "B" {
		t.Fatalf("values = %v labels = %v", values, labels)
	}
	if _, ok, err := RenderChartNativePNGWithColors(chart, 400, 300, nil); err != nil || !ok {
		t.Fatalf("native render with nulls: ok=%v err=%v", ok, err)
	}
}
