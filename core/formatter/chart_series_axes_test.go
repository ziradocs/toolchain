// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// TestFormatChart_SeriesAxes_RoundTrips cubre el hallazgo de que
// formatChart SIEMPRE reserializa un combo chart a la forma plana
// (type:/data:/series:/labels:), incluso cuando el autor lo escribió con la
// forma YAML anidada (data: {labels:, series:}) que parseComboChartYAML
// entiende — ver el comentario de formatChart. Antes de agregar el case
// "yAxisID:" acá y en el switch de propiedades planas de chart.go, un
// yAxisID declarado por el autor sobrevivía al PARSE pero desaparecía en
// cuanto `fmt` lo reescribía a la forma plana: la única forma de recuperarlo
// tras un `doclang fmt`/`slidelang fmt` era que la forma plana también lo
// soportara.
func TestFormatChart_SeriesAxes_RoundTrips(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(3, 1), "combo")
	chart.SeriesTypes = []string{"bar", "line"}
	chart.Series = []string{"Revenue ($M)", "Margin (%)"}
	chart.SeriesAxes = []string{"", "y1"}
	chart.Data = [][]interface{}{
		{"Q1", 120.0, 18.0},
		{"Q2", 145.0, 21.0},
	}

	out, err := FormatStrict(chartDoc(chart))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if !strings.Contains(out, `yAxisID: ["", "y1"]`) {
		t.Fatalf("salida sin yAxisID: %s", out)
	}

	// Reparsear la salida y confirmar que SeriesAxes sobrevive el
	// round-trip completo (parse → format → reparse), no solo que el texto
	// contiene la línea esperada.
	// chartDoc arma un ContentBlock tipo "title" (slide), no una SECTION de
	// documento — reparsear con el parser de SlideLang (Parse), no el de
	// documentos (ParseDocument).
	reparsed, diags := parser.New(util.NewNoop()).Parse(out, "test.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("reparse produjo un error: %v\n%s", d, out)
		}
	}
	var reChart *ast.ChartElement
	for _, block := range reparsed.ContentBlocks {
		for _, el := range block.Elements {
			if c, ok := el.(*ast.ChartElement); ok {
				reChart = c
			}
		}
	}
	if reChart == nil {
		t.Fatalf("no se encontró el ChartElement reparseado:\n%s", out)
	}
	want := []string{"", "y1"}
	if len(reChart.SeriesAxes) != len(want) || reChart.SeriesAxes[0] != want[0] || reChart.SeriesAxes[1] != want[1] {
		t.Errorf("SeriesAxes reparseado = %#v, want %#v", reChart.SeriesAxes, want)
	}
}

// TestFormatChart_NoSeriesAxes_OmitsYAxisIDLine confirma que un combo sin
// ningún eje declarado (SeriesAxes vacío, o todo strings vacíos) no emite
// `yAxisID: [...]` de puro ruido.
func TestFormatChart_NoSeriesAxes_OmitsYAxisIDLine(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(3, 1), "combo")
	chart.SeriesTypes = []string{"bar", "line"}
	chart.Series = []string{"Revenue ($M)", "Margin (%)"}
	chart.Data = [][]interface{}{
		{"Q1", 120.0, 18.0},
		{"Q2", 145.0, 21.0},
	}

	out, err := FormatStrict(chartDoc(chart))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if strings.Contains(out, "yAxisID") {
		t.Errorf("no debería emitirse yAxisID sin ejes declarados:\n%s", out)
	}
}
