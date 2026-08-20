// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// chartDiags corre ElementStructureRule sobre un slide con un solo chart.
func chartDiags(chart *ast.ChartElement) []diagnostics.Diagnostic {
	return (&ElementStructureRule{}).Check(&ast.ContentBlock{
		Elements: []ast.Element{chart},
	})
}

func jsonModeChart(t *testing.T, chartType, payload string) *ast.ChartElement {
	t.Helper()
	if !json.Valid([]byte(payload)) {
		t.Fatalf("el payload del caso de prueba no es JSON válido: %s", payload)
	}
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), chartType)
	chart.RawJSON = json.RawMessage(payload)
	chart.IsJSONMode = true
	return chart
}

func ruleIDs(diags []diagnostics.Diagnostic) []string {
	ids := make([]string, 0, len(diags))
	for _, d := range diags {
		ids = append(ids, d.RuleID)
	}
	return ids
}

// Repro literal del issue #207. Un payload estructuralmente válido pero que
// no es una config de Chart.js (una forma plana: labels/data/series) pasaba
// los tres gates existentes: CHART001 contaba cualquier RawJSON no vacío como
// "tiene datos", CHART002 solo dispara si json.Valid() falla, y
// ResolveChartJSONMode solo si el unmarshal falla o el top-level no es
// objeto. Resultado: build limpio, cero diagnósticos, chart en blanco.
func TestChartElement_JSONModeFlatPayload_EmitsCHART004(t *testing.T) {
	chart := jsonModeChart(t, "json", `{
		"labels": ["Q1", "Q2"],
		"data": [[10, 20], [5, 15]],
		"series": ["Revenue", "Cost"]
	}`)

	diags := chartDiags(chart)

	diag := findDiagnosticByRuleID(diags, "CHART004")
	if diag == nil {
		t.Fatalf("se esperaba CHART004, obtenidos: %v", ruleIDs(diags))
	}
	if !strings.Contains(diag.Message, "data.datasets") {
		t.Errorf("el mensaje debería nombrar data.datasets, obtenido: %s", diag.Message)
	}

	// CHART001 no debe acompañarlo: sí hay payload, lo que falla es su forma.
	// Reportar los dos por el mismo defecto es ruido y manda al autor a
	// buscar datos que sí escribió.
	if findDiagnosticByRuleID(diags, "CHART001") != nil {
		t.Errorf("no se esperaba CHART001 junto a CHART004, obtenidos: %v", ruleIDs(diags))
	}
}

// Segunda mitad del mismo repro, que es el issue #208: el tag decía
// "<<chart: json>>", y "json" no es un tipo de chart en ningún vocabulario.
// ResolveChartJSONMode lo copiaba tal cual al config que recibe Chart.js
// ("type":"json") porque el token del tag nunca se validó.
func TestChartElement_UnknownTypeInTag_EmitsCHART003(t *testing.T) {
	diags := chartDiags(jsonModeChart(t, "json", `{"data": {"datasets": [{"data": [1]}]}}`))

	diag := findDiagnosticByRuleID(diags, "CHART003")
	if diag == nil {
		t.Fatalf("se esperaba CHART003, obtenidos: %v", ruleIDs(diags))
	}
	if !strings.Contains(diag.Message, "json") {
		t.Errorf("el mensaje debería nombrar el tipo ofensor, obtenido: %s", diag.Message)
	}
	// El mensaje tiene que decir cuáles SÍ valen: un "unknown type" a secas
	// obliga a ir a buscar la lista a otro lado.
	if !strings.Contains(diag.Message, "bar") || !strings.Contains(diag.Message, "treemap") {
		t.Errorf("el mensaje debería listar el vocabulario, obtenido: %s", diag.Message)
	}
}

// El caso que el gate de #207 NO cubre y que es la razón de que validar el
// tipo se gane su radio de impacto: un typo en el tag con cuerpo YAML normal.
// Renderiza en blanco igual, y ningún gate lo veía.
func TestChartElement_TypoedTypeWithYAMLBody_EmitsCHART003(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "barr")
	chart.Data = [][]interface{}{{"Q1", 10}}

	diags := chartDiags(chart)

	if findDiagnosticByRuleID(diags, "CHART003") == nil {
		t.Fatalf("se esperaba CHART003, obtenidos: %v", ruleIDs(diags))
	}
	// Sí tiene datos: CHART001 no aplica.
	if findDiagnosticByRuleID(diags, "CHART001") != nil {
		t.Errorf("no se esperaba CHART001 (el chart sí trae datos), obtenidos: %v", ruleIDs(diags))
	}
}

// El falso positivo que hay que evitar, y el motivo de que el tipo EFECTIVO
// se resuelva igual que en renderer.ResolveChartJSONMode: un <<chart: bar>>
// con payload sin "type" renderiza perfecto (el renderer rellena el tipo
// desde el tag). Si el linter exigiera el "type" dentro del payload, o si
// leyera solo el tag ignorando el payload, este caso lintearía sucio.
func TestChartElement_ValidConfigs_LintClean(t *testing.T) {
	withData := func(chart *ast.ChartElement) *ast.ChartElement {
		chart.Data = [][]interface{}{{"Q1", 10}}
		return chart
	}

	cases := map[string]*ast.ChartElement{
		"tag tipado + payload sin type": jsonModeChart(t, "bar",
			`{"data": {"datasets": [{"label": "Rev", "data": [1, 2]}]}}`),
		"payload con type propio": jsonModeChart(t, "bar",
			`{"type": "line", "data": {"datasets": [{"data": [1]}]}}`),
		"el payload manda sobre un tag raro": jsonModeChart(t, "json",
			`{"type": "doughnut", "data": {"datasets": [{"data": [1]}]}}`),
		"YAML normal":         withData(ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")),
		"pseudo-tipo del DSL": withData(ast.NewChartElement(diagnostics.NewPosition(1, 1), "combo")),
		"treemap":             withData(ast.NewChartElement(diagnostics.NewPosition(1, 1), "treemap")),
	}

	for name, chart := range cases {
		t.Run(name, func(t *testing.T) {
			if diags := chartDiags(chart); len(diags) != 0 {
				t.Errorf("se esperaba lint limpio, obtenidos: %v", ruleIDs(diags))
			}
		})
	}
}

// CHART001 conserva su significado y su alcance en el camino YAML: un chart
// sin datos de ninguna forma lo sigue disparando, sin cambios.
func TestChartElement_NoDataAtAll_StillEmitsCHART001(t *testing.T) {
	diags := chartDiags(ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar"))

	if findDiagnosticByRuleID(diags, "CHART001") == nil {
		t.Fatalf("se esperaba CHART001, obtenidos: %v", ruleIDs(diags))
	}
	if findDiagnosticByRuleID(diags, "CHART004") != nil {
		t.Errorf("CHART004 es solo del camino JSON, obtenidos: %v", ruleIDs(diags))
	}
}

// Los tres IDs son configurables por RulePolicy como cualquier otro, que es
// media razón por la que este check vive en el linter y no en el parser.
func TestChartRules_AreSuppressibleByPolicy(t *testing.T) {
	disabled := false
	policy := &PolicyConfig{Rules: map[string]RulePolicy{
		"CHART003": {Enabled: &disabled},
		"CHART004": {Enabled: &disabled},
	}}

	diags := policy.Apply(chartDiags(jsonModeChart(t, "json", `{"labels": ["Q1"]}`)))

	if len(diags) != 0 {
		t.Errorf("la política debería haber suprimido los dos, quedaron: %v", ruleIDs(diags))
	}
}
