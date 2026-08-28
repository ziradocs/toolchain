// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// Hasta CHART005, todo lo que el parser de charts no reconocía se evaporaba
// sin dejar rastro — ni en el AST ni en un diagnóstico. Dos formas de
// evaporarse, y la plantilla `report` de `doclang init` shipeó con las dos:
//
//	<<chart:bar title="Performance Metrics">>   <- atributo inexistente
//	  labels: [...]
//	  datasets:                                 <- llave inexistente
//	    data: [85, 90, 88, 95]
//	    backgroundColor: "#3498db"
//	<<end>>
//
// El título no llegaba al AST (así que `doclang build` renderizaba el chart
// sin título), backgroundColor se perdía, y el `data:` ANIDADO se capturaba
// como si fuera la llave de nivel superior, porque el loop de propiedades era
// plano y no miraba la sangría.
//
// Lo que NO es este bug, y conviene tener claro para no "arreglarlo" de más:
// `fmt` nunca perdió nada. Emitía exactamente lo que había en el AST; la
// pérdida era del parser, o sea que `doclang build` la sufría igual.

func parseChartBlock(t *testing.T, src string) (*ast.ChartElement, []string) {
	t.Helper()
	ctx := &ParseContext{Lines: strings.Split(src, "\n"), Mode: "flex"}
	res := (&ChartParser{}).Parse(ctx, 0)
	if res.Element == nil {
		t.Fatalf("el parser no devolvió elemento para:\n%s", src)
	}
	chart, ok := res.Element.(*ast.ChartElement)
	if !ok {
		t.Fatalf("se esperaba *ast.ChartElement, llegó %T", res.Element)
	}
	var messages []string
	for _, d := range res.Diagnostics {
		messages = append(messages, d.RuleID+": "+d.Message)
	}
	return chart, messages
}

func TestChartParser_ReportsUnknownOpenerAttribute(t *testing.T) {
	chart, diags := parseChartBlock(t, `<<chart:bar title="Performance Metrics">>
  data: [85, 90]
<<end>>`)

	if !hasDiagnostic(diags, "CHART005", "title") {
		t.Errorf("no se reportó `title=` como atributo desconocido de la apertura.\n"+
			"Sin ese aviso, el título no llega al AST y `doclang build` renderiza el chart sin título, en silencio.\n"+
			"diagnósticos: %v", diags)
	}
	// El título NO se adopta: `title=` no es sintaxis del lenguaje (ningún
	// elemento acepta title= en la apertura). La forma documentada es la
	// llave del cuerpo, y eso es lo que se avisa.
	if chart.Title != "" {
		t.Errorf("chart.Title = %q; `title=` en la apertura no debe adoptarse, solo reportarse", chart.Title)
	}
}

func TestChartParser_ReportsUnknownBodyKey(t *testing.T) {
	chart, diags := parseChartBlock(t, `<<chart: bar>>
  labels: ["Q1", "Q2"]
  datasets:
    data: [85, 90]
    backgroundColor: "#3498db"
<<end>>`)

	if !hasDiagnostic(diags, "CHART005", "datasets") {
		t.Errorf("no se reportó `datasets:` como llave desconocida del cuerpo.\ndiagnósticos: %v", diags)
	}
	// backgroundColor está DENTRO de datasets:, o sea más profundo que la
	// sangría base — es contenido de una llave desconocida, no una llave
	// desconocida más. Se reporta el bloque, no cada una de sus líneas.
	if hasDiagnostic(diags, "CHART005", "backgroundColor") {
		t.Errorf("se reportó `backgroundColor`, que es contenido anidado de `datasets:`, no una llave de nivel superior.\n"+
			"Un aviso por línea anidada ahoga el que importa.\ndiagnósticos: %v", diags)
	}
	// Y el `data:` anidado no puede hacerse pasar por el de nivel superior.
	if len(chart.Data) != 0 {
		t.Errorf("chart.Data = %v; el `data:` anidado dentro de `datasets:` no es la llave de nivel superior "+
			"y no debe capturarse (el loop tiene que respetar la sangría base)", chart.Data)
	}
}

// TestChartParser_DocumentedShapeIsSilentAndComplete es la otra mitad: la
// forma documentada no puede disparar el aviso nuevo, y tiene que capturar
// todo. Sin este test, un CHART005 demasiado entusiasta (por ejemplo validando
// dentro de `options:`, que es config arbitraria de Chart.js) pasaría los dos
// tests de arriba mientras llena de ruido cada chart legítimo del corpus.
func TestChartParser_DocumentedShapeIsSilentAndComplete(t *testing.T) {
	chart, diags := parseChartBlock(t, `<<chart: bar width="1200">>
  title: "Performance Metrics"
  labels: ["Q1", "Q2", "Q3", "Q4"]
  data: [85, 90, 88, 95]
  series: ["Ventas"]
  options:
    responsive: true
    datasets:
      bar:
        backgroundColor: "#3498db"
    plugins:
      title:
        display: true
<<end>>`)

	if len(diags) != 0 {
		t.Errorf("la forma documentada disparó diagnósticos: %v", diags)
	}
	if chart.Title != "Performance Metrics" {
		t.Errorf("chart.Title = %q, se esperaba %q", chart.Title, "Performance Metrics")
	}
	if chart.Width != 1200 {
		t.Errorf("chart.Width = %d, se esperaba 1200", chart.Width)
	}
	if len(chart.Data) != 1 || len(chart.Data[0]) != 4 {
		t.Errorf("chart.Data = %v, se esperaba una fila de 4", chart.Data)
	}
	if len(chart.Labels) != 4 {
		t.Errorf("chart.Labels = %v, se esperaban 4", chart.Labels)
	}
	// options: es YAML anidado arbitrario y se captura entero, incluida una
	// sub-llave que se llame igual que una llave de nivel superior.
	datasets, ok := chart.Options["datasets"].(map[string]interface{})
	if !ok {
		t.Fatalf("chart.Options[\"datasets\"] no llegó: %#v", chart.Options)
	}
	bar, ok := datasets["bar"].(map[string]interface{})
	if !ok || bar["backgroundColor"] != "#3498db" {
		t.Errorf("options.datasets.bar.backgroundColor no sobrevivió: %#v", datasets)
	}
}

func hasDiagnostic(diags []string, ruleID, needle string) bool {
	for _, d := range diags {
		if strings.HasPrefix(d, ruleID+":") && strings.Contains(d, needle) {
			return true
		}
	}
	return false
}
