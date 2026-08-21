// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

func treemapChart() *ast.ChartElement {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "treemap")
	chart.Data = [][]interface{}{
		{"Large Enterprise", 35},
		{"Mid-Market", 28},
		{"Micro Business", 3},
	}
	return chart
}

// convertChartType tenía una whitelist con fallback silencioso a "bar", así
// que un <<chart: treemap>> se dibujaba como barras — un modo de falla peor
// que el lienzo en blanco, porque el deck se ve "bien" y nadie revisa.
func TestConvertChartElementToChartJS_TreemapKeepsItsType(t *testing.T) {
	config := ConvertChartElementToChartJS(treemapChart(), "chart-0", nil)
	if config.Type != "treemap" {
		t.Errorf("Type = %q, want \"treemap\" — ¿volvió a caer al fallback de convertChartType?", config.Type)
	}
}

// La forma del dataset de chartjs-chart-treemap se verificó contra el plugin
// real en Chromium; cada pieza de abajo es necesaria y su ausencia deja el
// treemap ilegible o invisible, sin error de consola ni diagnóstico.
func TestConvertChartElementToChartJS_TreemapDatasetShape(t *testing.T) {
	config := ConvertChartElementToChartJS(treemapChart(), "chart-0", nil)

	if len(config.Data.Datasets) != 1 {
		t.Fatalf("se esperaba 1 dataset, hay %d", len(config.Data.Datasets))
	}
	ds := config.Data.Datasets[0]

	if len(config.Data.Labels) != 0 {
		t.Errorf("Data.Labels = %v, want vacío — el controlador de treemap no consume data.labels", config.Data.Labels)
	}
	if len(ds.Data) != 0 {
		t.Errorf("Dataset.Data = %v, want vacío — el treemap se alimenta de tree, no de data[]", ds.Data)
	}
	if len(ds.Tree) != 3 {
		t.Fatalf("Tree tiene %d hojas, want 3", len(ds.Tree))
	}
	if ds.Tree[0]["label"] != "Large Enterprise" || ds.Tree[0]["value"] != 35 {
		t.Errorf("primera hoja = %#v", ds.Tree[0])
	}
	if ds.Key != "value" {
		t.Errorf("Key = %q, want \"value\"", ds.Key)
	}
	// Sin Groups, el formatter por defecto de Labels dibuja SOLO el número.
	if len(ds.Groups) != 1 || ds.Groups[0] != "label" {
		t.Errorf("Groups = %v, want [label] — sin esto los rectángulos salen sin nombre", ds.Groups)
	}
	// Labels{display:true} usa el formatter por defecto del plugin, que es lo
	// que evita necesitar un callback JS.
	if ds.Labels["display"] != true {
		t.Errorf("Labels = %#v, want {display:true}", ds.Labels)
	}
	// backgroundColor NO es indexable en un TreemapElement: un arreglo llega
	// crudo a options.backgroundColor y el rectángulo queda sin relleno.
	if _, isString := ds.BackgroundColor.(string); !isString {
		t.Errorf("BackgroundColor = %#v (%T), want un solo string", ds.BackgroundColor, ds.BackgroundColor)
	}
}

// Los dos DSLs tienen que emitir el MISMO config para el mismo chart: esa es
// la lección de #11 (el mismo chart se veía distinto en cada uno) y la razón
// por la que #55 unificó ResolveChartJSONMode. Acá no hay símbolo compartido
// que forzarlo — son dos implementaciones paralelas — así que el test compara
// la salida real de las dos.
func TestTreemapConfigMatchesCoreRenderer(t *testing.T) {
	chart := treemapChart()

	slidelangJSON, err := json.Marshal(ConvertChartElementToChartJS(chart, "chart-0", nil).Data)
	if err != nil {
		t.Fatal(err)
	}

	var coreConfig map[string]interface{}
	if err := json.Unmarshal([]byte(renderer.GenerateChartConfig(chart)), &coreConfig); err != nil {
		t.Fatal(err)
	}
	coreJSON, err := json.Marshal(coreConfig["data"])
	if err != nil {
		t.Fatal(err)
	}

	var a, b interface{}
	if err := json.Unmarshal(slidelangJSON, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(coreJSON, &b); err != nil {
		t.Fatal(err)
	}
	if string(mustCanonical(t, a)) != string(mustCanonical(t, b)) {
		t.Errorf("los dos DSLs emiten data{} distinto para el mismo treemap:\nslidelang: %s\ncore:      %s", mustCanonical(t, a), mustCanonical(t, b))
	}
}

func mustCanonical(t *testing.T, v interface{}) []byte {
	t.Helper()
	// json.Marshal ordena las claves de un map[string]interface{}, así que
	// re-serializar la forma ya decodificada canonicaliza el orden.
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

