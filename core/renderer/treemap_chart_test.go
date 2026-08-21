// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// treemapConfig corre GenerateChartConfigWithMode sobre un chart treemap con
// filas fijas y devuelve el config ya decodificado más su dataset único.
func treemapConfig(t *testing.T, forExport bool) (map[string]interface{}, map[string]interface{}) {
	t.Helper()

	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "treemap")
	chart.Data = [][]interface{}{
		{"Large Enterprise", 35},
		{"Mid-Market", 28},
		{"Micro Business", 3},
	}

	raw := GenerateChartConfigWithMode(chart, forExport)

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("el config de treemap no es JSON válido: %v\n%s", err, raw)
	}
	data, ok := decoded["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("config sin data{}: %s", raw)
	}
	datasets, ok := data["datasets"].([]interface{})
	if !ok || len(datasets) != 1 {
		t.Fatalf("se esperaba exactamente 1 dataset, hay %v: %s", data["datasets"], raw)
	}
	return decoded, datasets[0].(map[string]interface{})
}

// La forma del dataset de chartjs-chart-treemap no se parece a la de ningún
// tipo nativo, y cada pieza de abajo se verificó contra el plugin real
// corriendo en Chromium — no salió de leer la documentación. Este test fija
// esas observaciones para que un refactor no las deshaga en silencio: el
// modo de falla de un treemap mal armado es un lienzo en blanco, sin error
// en consola ni diagnóstico, que es exactamente la clase de bug que
// #207/#208 describen desde el otro lado.
func TestGenerateChartConfig_TreemapDatasetShape(t *testing.T) {
	decoded, dataset := treemapConfig(t, false)

	if got := decoded["type"]; got != "treemap" {
		t.Errorf(`type = %v, want "treemap"`, got)
	}

	// data.labels NO se emite: el controlador de treemap no lo consume, cada
	// hoja trae su etiqueta dentro de tree.
	if _, present := decoded["data"].(map[string]interface{})["labels"]; present {
		t.Error("data.labels no debería emitirse para treemap")
	}

	tree, ok := dataset["tree"].([]interface{})
	if !ok {
		t.Fatalf("dataset.tree ausente o con tipo equivocado: %#v", dataset["tree"])
	}
	if len(tree) != 3 {
		t.Fatalf("tree tiene %d hojas, want 3", len(tree))
	}
	first := tree[0].(map[string]interface{})
	if first["label"] != "Large Enterprise" || first["value"] != float64(35) {
		t.Errorf("primera hoja = %#v, want {label:Large Enterprise value:35}", first)
	}

	if got := dataset["key"]; got != "value" {
		t.Errorf(`dataset.key = %v, want "value" (el campo numérico de cada hoja)`, got)
	}

	// groups NO es cosmético aunque cada hoja sea única: sin él, el formatter
	// por defecto de labels dibuja SOLO el número — el nombre sale del campo
	// por el que se agrupa. Verificado en Chromium contra el plugin real.
	groups, ok := dataset["groups"].([]interface{})
	if !ok || len(groups) != 1 || groups[0] != "label" {
		t.Errorf(`dataset.groups = %#v, want ["label"] — sin esto los rectángulos salen sin nombre`, dataset["groups"])
	}

	// labels.display prende el formatter POR DEFECTO del plugin (nombre +
	// valor). Es lo que evita necesitar un callback JS, que no sobreviviría a
	// json.Marshal ni al <script type="application/json"> por donde viaja
	// este config.
	labels, ok := dataset["labels"].(map[string]interface{})
	if !ok || labels["display"] != true {
		t.Errorf("dataset.labels = %#v, want {display:true}", dataset["labels"])
	}
}

// En un TreemapElement backgroundColor NO es indexable: verificado en
// Chromium, un arreglo llega crudo a options.backgroundColor del elemento y
// el rectángulo termina SIN relleno — el treemap se dibuja invisible. Las
// demás ramas de GenerateChartConfigWithMode sí ciclan una paleta por índice,
// así que este es justo el detalle que alguien "uniformaría" sin querer.
func TestGenerateChartConfig_TreemapBackgroundColorIsNotAnArray(t *testing.T) {
	_, dataset := treemapConfig(t, false)

	bg, ok := dataset["backgroundColor"].(string)
	if !ok {
		t.Fatalf("backgroundColor = %#v (%T), want un solo string — un arreglo deja los rectángulos sin relleno", dataset["backgroundColor"], dataset["backgroundColor"])
	}
	if !strings.HasPrefix(bg, "#") {
		t.Errorf("backgroundColor = %q, want un color hex", bg)
	}
}

// Las filas con menos de dos columnas no tienen valor que graficar; se
// omiten en vez de entrar al tree con un value nil, que el plugin no puede
// acomodar.
func TestGenerateChartConfig_TreemapSkipsRowsWithoutValue(t *testing.T) {
	chart := ast.NewChartElement(diagnostics.NewPosition(1, 1), "treemap")
	chart.Data = [][]interface{}{
		{"Con valor", 10},
		{"Sin valor"},
		{},
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(GenerateChartConfig(chart)), &decoded); err != nil {
		t.Fatalf("config inválido: %v", err)
	}
	datasets := decoded["data"].(map[string]interface{})["datasets"].([]interface{})
	tree := datasets[0].(map[string]interface{})["tree"].([]interface{})
	if len(tree) != 1 {
		t.Errorf("tree tiene %d hojas, want 1 (solo la fila con valor)", len(tree))
	}
}

// applyExportOptimizations arma scales.x/scales.y con grid.display:true,
// pensado para los tipos cartesianos. El controlador de treemap declara sus
// propios ejes ocultos, así que dejar ese bloque le pinta una rejilla y unos
// ticks encima. El resto de las optimizaciones de export (padding, fuentes)
// sí se quieren igual.
func TestGenerateChartConfigForExport_TreemapDropsCartesianScales(t *testing.T) {
	decoded, _ := treemapConfig(t, true)

	options := decoded["options"].(map[string]interface{})
	if _, present := options["scales"]; present {
		t.Errorf("options.scales no debería existir en un treemap de export: %#v", options["scales"])
	}
	if _, present := options["layout"]; !present {
		t.Error("options.layout sí debería seguir puesto — solo las escalas sobran")
	}

	// Contraprueba: un bar SÍ conserva sus escalas por este mismo camino.
	bar := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	bar.Data = [][]interface{}{{"Q1", 1}}
	var barDecoded map[string]interface{}
	if err := json.Unmarshal([]byte(GenerateChartConfigForExport(bar)), &barDecoded); err != nil {
		t.Fatalf("config de bar inválido: %v", err)
	}
	if _, present := barDecoded["options"].(map[string]interface{})["scales"]; !present {
		t.Error("un bar de export perdió sus escalas — el borrado se está aplicando de más")
	}
}

// La leyenda de un treemap solo repite el label del dataset ("Data"), no
// discrimina nada: cada rectángulo ya trae su nombre dentro. Quitar
// dataset.label en su lugar NO sirve — la leyenda entonces dibuja
// "undefined" (verificado en Chromium).
func TestGenerateChartConfig_TreemapHidesLegendByDefault(t *testing.T) {
	decoded, _ := treemapConfig(t, false)

	plugins, ok := decoded["options"].(map[string]interface{})["plugins"].(map[string]interface{})
	if !ok {
		t.Fatalf("options.plugins ausente: %#v", decoded["options"])
	}
	legend, ok := plugins["legend"].(map[string]interface{})
	if !ok || legend["display"] != false {
		t.Errorf("plugins.legend = %#v, want {display:false}", plugins["legend"])
	}
}

// El plugin se auto-registra contra el `Chart` global que publica el bundle
// base, así que el orden de los dos <script> no es cosmético: al revés, el
// controlador "treemap" no queda registrado y todo treemap sale en blanco.
func TestChartJSTreemapCDNScriptTag_LoadsAfterChartJS(t *testing.T) {
	if !strings.Contains(ChartJSTreemapCDNScriptTag, "chartjs-chart-treemap@") {
		t.Error("el tag del plugin debe apuntar a una versión exacta, no flotante")
	}
	if !strings.Contains(ChartJSTreemapCDNScriptTag, `integrity="sha384-`) {
		t.Error("el tag del plugin debe llevar SRI, igual que los demás CDN")
	}

	scripts := generateDocumentScripts(DocumentHTMLOptions{ChartMode: "browser"})
	base := strings.Index(scripts, ChartJSCDNScriptTag)
	plugin := strings.Index(scripts, ChartJSTreemapCDNScriptTag)
	if base < 0 || plugin < 0 {
		t.Fatalf("browser mode debería emitir los dos tags (base=%d plugin=%d)", base, plugin)
	}
	if plugin < base {
		t.Error("el plugin de treemap se emitió ANTES que el bundle base de Chart.js — no se auto-registraría")
	}

	offline := generateDocumentScripts(DocumentHTMLOptions{ChartMode: "offline-inline"})
	if strings.Contains(offline, ChartJSTreemapCDNScriptTag) {
		t.Error("en modo offline los charts van pre-rasterizados; el plugin no debería emitirse")
	}
}
