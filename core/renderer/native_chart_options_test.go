// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"testing"

	"github.com/go-analyze/charts"
)

// TestClassifyChartOptions_Table cubre el set completo de la primera pasada
// (issue #148): cada ruta mapeable/ignorable aprueba, y las formas que
// deliberadamente NO están en optionLeafRules (datalabels, ejes
// secundarios, posición left/right de la leyenda, scales.x.grid que ni
// existe en CategoryAxisOption) descalifican.
func TestClassifyChartOptions_Table(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]interface{}
		want    bool
	}{
		{"nil options qualifies (equivalente a Options vacío)", nil, true},
		{"empty options qualifies", map[string]interface{}{}, true},
		{
			"responsive + maintainAspectRatio son ignorables",
			map[string]interface{}{"responsive": true, "maintainAspectRatio": false},
			true,
		},
		{
			"plugins.title.text + display",
			map[string]interface{}{"plugins": map[string]interface{}{
				"title": map[string]interface{}{"text": "Ventas", "display": true},
			}},
			true,
		},
		{
			"plugins.legend.display false",
			map[string]interface{}{"plugins": map[string]interface{}{
				"legend": map[string]interface{}{"display": false},
			}},
			true,
		},
		{
			"plugins.legend.position top",
			map[string]interface{}{"plugins": map[string]interface{}{
				"legend": map[string]interface{}{"position": "top"},
			}},
			true,
		},
		{
			"plugins.legend.position bottom",
			map[string]interface{}{"plugins": map[string]interface{}{
				"legend": map[string]interface{}{"position": "bottom"},
			}},
			true,
		},
		{
			"plugins.legend.position left descalifica (fuerza overlay, no reserva espacio lateral)",
			map[string]interface{}{"plugins": map[string]interface{}{
				"legend": map[string]interface{}{"position": "left"},
			}},
			false,
		},
		{
			"plugins.legend.position right descalifica",
			map[string]interface{}{"plugins": map[string]interface{}{
				"legend": map[string]interface{}{"position": "right"},
			}},
			false,
		},
		{
			"scales.y.beginAtZero",
			map[string]interface{}{"scales": map[string]interface{}{
				"y": map[string]interface{}{"beginAtZero": true},
			}},
			true,
		},
		{
			// Removidos de optionLeafRules en review de PR #159:
			// go-analyze/charts (range.go) solo respeta min/max cuando
			// EXPANDEN el rango de los datos reales, no cuando lo recortan
			// -- la semántica que Chart.js sí tiene. Sin ver los datos del
			// chart, el clasificador no puede distinguir esos dos casos,
			// así que ambos descalifican.
			"scales.y.min descalifica (go-analyze/charts no lo recorta como Chart.js)",
			map[string]interface{}{"scales": map[string]interface{}{
				"y": map[string]interface{}{"min": 50},
			}},
			false,
		},
		{
			"scales.y.max descalifica (mismo motivo que min)",
			map[string]interface{}{"scales": map[string]interface{}{
				"y": map[string]interface{}{"max": 50},
			}},
			false,
		},
		{
			"scales.y1 (eje secundario) descalifica",
			map[string]interface{}{"scales": map[string]interface{}{
				"y1": map[string]interface{}{"type": "linear", "position": "right"},
			}},
			false,
		},
		{
			"scales.x.grid.display descalifica (CategoryAxisOption no tiene grid)",
			map[string]interface{}{"scales": map[string]interface{}{
				"x": map[string]interface{}{"grid": map[string]interface{}{"display": false}},
			}},
			false,
		},
		{
			"plugins.datalabels descalifica (no está en la tabla)",
			map[string]interface{}{"plugins": map[string]interface{}{
				"datalabels": map[string]interface{}{"anchor": "end"},
			}},
			false,
		},
		{
			"scales.y.ticks.callback descalifica",
			map[string]interface{}{"scales": map[string]interface{}{
				"y": map[string]interface{}{"ticks": map[string]interface{}{"callback": "function(v){}"}},
			}},
			false,
		},
		{
			"scales.x.stacked descalifica",
			map[string]interface{}{"scales": map[string]interface{}{
				"x": map[string]interface{}{"stacked": true},
			}},
			false,
		},
		{
			"clave de primer nivel totalmente desconocida descalifica",
			map[string]interface{}{"customPlugin": map[string]interface{}{"anything": true}},
			false,
		},
		{
			"valor con tipo inesperado en ruta conocida descalifica (legend.position no-string)",
			map[string]interface{}{"plugins": map[string]interface{}{
				"legend": map[string]interface{}{"position": 42},
			}},
			false,
		},
		{
			// Review de PR #159: alwaysMappable dejaba pasar cualquier tipo
			// para plugins.title.text, aunque extractNativeChartOverrides
			// solo sabe traducir strings -- un número/lista/nil pasaba el
			// gate y luego se descartaba en silencio en la extracción.
			"plugins.title.text no-string descalifica (el extractor solo traduce strings)",
			map[string]interface{}{"plugins": map[string]interface{}{
				"title": map[string]interface{}{"text": 42},
			}},
			false,
		},
		{
			"sub-mapa vacío no aporta hojas: no descalifica por sí solo",
			map[string]interface{}{"plugins": map[string]interface{}{}},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyChartOptions(tt.options)
			if got != tt.want {
				t.Errorf("classifyChartOptions(%#v) = %v, want %v", tt.options, got, tt.want)
			}
		})
	}
}

// TestClassifyChartOptions_SiblingDiscriminant es el test que atrapa un
// clasificador con retorno temprano: un clasificador que baja por
// plugins.legend.position, la encuentra mapeable y se detiene ahí,
// aprobaría este caso incorrectamente. La invariante real es "toda hoja",
// no "toda ruta que matchee" — un hermano no reconocido bajo el MISMO
// padre debe descalificar el chart entero, aunque otra clave del mismo
// objeto sí sea mapeable.
func TestClassifyChartOptions_SiblingDiscriminant(t *testing.T) {
	options := map[string]interface{}{
		"plugins": map[string]interface{}{
			"legend": map[string]interface{}{
				"position": "bottom", // mapeable por sí sola
				"labels": map[string]interface{}{
					"font": map[string]interface{}{"size": 14}, // NO está en la tabla
				},
			},
		},
	}
	if classifyChartOptions(options) {
		t.Error("classifyChartOptions() = true, want false: labels.font.size no reconocido debe descalificar aunque position sea mapeable")
	}
}

// TestClassifyChartOptions_MalformedShapesDoNotPanic cubre formas
// inesperadas de Options (config Chart.js escrita a mano, cualquier YAML
// es válido acá) — ninguna debe panicar, deben descalificar de forma
// segura. Mismo caso que ya prueba slidelang's pptx_coverage_test.go para
// pptxChartIgnoringOptions, verificado acá para el clasificador nuevo.
func TestClassifyChartOptions_MalformedShapesDoNotPanic(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]interface{}
	}{
		{"plugins no es un mapa", map[string]interface{}{"plugins": "not a map"}},
		{"plugins.legend no es un mapa", map[string]interface{}{
			"plugins": map[string]interface{}{"legend": "not a map"},
		}},
		{"scales.y no es un mapa", map[string]interface{}{
			"scales": map[string]interface{}{"y": "not a map"},
		}},
		{"un array en vez de un mapa anidado", map[string]interface{}{
			"plugins": []interface{}{"a", "b"},
		}},
		{"nil anidado", map[string]interface{}{"plugins": nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("classifyChartOptions panicked on %#v: %v", tt.options, r)
				}
			}()
			// No se asserta el resultado -- todas estas formas deben
			// descalificar de forma segura, pero lo único que este test
			// garantiza es que ninguna panica.
			classifyChartOptions(tt.options)
		})
	}
}

// TestExtractNativeChartOverrides_TranslatesEachMappableKey es el test del
// traductor (issue #148): dado un ChartElement con cada clave mapeable,
// confirma que el campo nativo correspondiente queda seteado -- struct
// assert, no golden image (ver decodePNGAndCheck's propio comentario sobre
// por qué comparar píxeles contra Chart.js no tiene sentido acá).
func TestExtractNativeChartOverrides_TranslatesEachMappableKey(t *testing.T) {
	options := map[string]interface{}{
		"plugins": map[string]interface{}{
			"title":  map[string]interface{}{"text": "Ventas Q1", "display": false},
			"legend": map[string]interface{}{"display": false, "position": "bottom"},
		},
		"scales": map[string]interface{}{
			"y": map[string]interface{}{"beginAtZero": true},
		},
	}

	ov := extractNativeChartOverrides(options)

	if !ov.titleTextSet || ov.titleText != "Ventas Q1" {
		t.Errorf("titleText = %q (set=%v), want %q (set=true)", ov.titleText, ov.titleTextSet, "Ventas Q1")
	}
	if !ov.titleHide {
		t.Error("titleHide = false, want true (plugins.title.display: false)")
	}
	if !ov.legendHide {
		t.Error("legendHide = false, want true (plugins.legend.display: false)")
	}
	if !ov.legendBottom {
		t.Error("legendBottom = false, want true (plugins.legend.position: bottom)")
	}
	if !ov.yBeginAtZero {
		t.Error("yBeginAtZero = false, want true")
	}
}

// TestApplyTitleOverrides_ElemTitleWinsOverOptionsRescue confirma la
// precedencia documentada: elem.Title (campo de primer nivel del DSL) gana
// sobre plugins.title.text por ser más explícito -- mismo criterio que
// slidelang's chartTitleFromOptions ya aplicaba solo a PPTX.
func TestApplyTitleOverrides_ElemTitleWinsOverOptionsRescue(t *testing.T) {
	ov := nativeChartOverrides{titleText: "From Options", titleTextSet: true}

	var withTopLevel charts.TitleOption
	applyTitleOverrides(&withTopLevel, "From Top Level", ov)
	if withTopLevel.Text != "From Top Level" {
		t.Errorf("Text = %q, want top-level title to win", withTopLevel.Text)
	}

	var rescued charts.TitleOption
	applyTitleOverrides(&rescued, "", ov)
	if rescued.Text != "From Options" {
		t.Errorf("Text = %q, want options.plugins.title.text rescued when elem.Title is empty", rescued.Text)
	}
}

// TestApplyYAxisOverrides_BeginAtZeroSetsMinToZero cubre la única
// traducción que applyYAxisOverrides hace hoy: scales.y.min/max se
// removieron de optionLeafRules (review de PR #159, ver su comentario), así
// que ya no hay una precedencia explícita-min-vs-beginAtZero que probar.
func TestApplyYAxisOverrides_BeginAtZeroSetsMinToZero(t *testing.T) {
	ov := nativeChartOverrides{yBeginAtZero: true}

	var axis charts.ValueAxisOption
	applyYAxisOverrides(&axis, ov)

	if axis.Min == nil || *axis.Min != 0 {
		t.Errorf("Min = %v, want 0", axis.Min)
	}
}

// TestRenderChartNativePNG_ApprovedOptionsStillRasterize es el smoke de
// píxeles (issue #148): un chart con options aprobadas por el clasificador
// debe seguir produciendo un PNG válido de las dimensiones pedidas -- no
// solo "no descalifica", sino que el camino completo (clasificar, extraer,
// aplicar, rasterizar) no rompe el render.
func TestRenderChartNativePNG_ApprovedOptionsStillRasterize(t *testing.T) {
	elem := newTestChartElement("bar")
	elem.Options = map[string]interface{}{
		"responsive": true,
		"plugins": map[string]interface{}{
			"legend": map[string]interface{}{"position": "bottom"},
		},
		"scales": map[string]interface{}{
			"y": map[string]interface{}{"beginAtZero": true},
		},
	}

	if !SupportsNativeChartRenderingWithOptions(elem) {
		t.Fatal("SupportsNativeChartRenderingWithOptions() = false, want true for an all-approved options set")
	}

	data, ok, err := RenderChartNativePNG(elem, 640, 480)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decodePNGAndCheck(t, data, 640, 480)
}
