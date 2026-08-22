// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// TestMergeOptions_NestedAuthorOverrideKeepsSiblingDefaults cubre el mismo eje
// que core's TestMergeChartOptions_AuthorNestedOverrideKeepsSiblingDefaults
// (issue #11/#55): mergeOptions delega el merge recursivo en sí a
// renderer.MergeChartOptions, pero este test es la garantía de que la
// delegación no perdió el comportamiento que slidelang ya tenía — un
// `options:` del autor que solo toca una hoja de un mapa anidado (p. ej.
// scales.y1) no debe borrar las hojas hermanas (scales.y) que
// createComboChartScales puso ahí.
func TestMergeOptions_NestedAuthorOverrideKeepsSiblingDefaults(t *testing.T) {
	target := map[string]interface{}{
		"scales": map[string]interface{}{
			"y": map[string]interface{}{
				"beginAtZero": true,
				"position":    "left",
			},
			"y1": map[string]interface{}{
				"type":        "linear",
				"display":     true,
				"position":    "right",
				"beginAtZero": true,
			},
		},
	}
	source := map[string]interface{}{
		"scales": map[string]interface{}{
			"y1": map[string]interface{}{
				"max": float64(100),
			},
		},
	}

	mergeOptions(target, source)

	scales := target["scales"].(map[string]interface{})

	y1 := scales["y1"].(map[string]interface{})
	if y1["max"] != float64(100) {
		t.Fatalf("scales.y1.max = %v, want the author's override (100)", y1["max"])
	}
	if y1["position"] != "right" {
		t.Fatalf("scales.y1.position = %v, want the createComboChartScales default (\"right\") to survive the author's max override", y1["position"])
	}

	y := scales["y"].(map[string]interface{})
	if y["position"] != "left" {
		t.Fatalf("scales.y.position = %v, want the untouched sibling default (\"left\") to survive", y["position"])
	}
}

// TestMergeOptions_CallbackStringsStillConvertedAfterDelegation cubre que la
// conversión de callbacks JS (específica de slidelang, ver charts.js) sigue
// aplicando después de que el merge recursivo se movió a
// renderer.MergeChartOptions — antes vivía inline en el mismo recorrido
// recursivo, ahora es un post-pase separado (convertCallbackStrings). Este
// caso (target sin "callbacks" previo) ya se comportaba igual con el código
// viejo — ver el siguiente test para el caso que sí cambia.
func TestMergeOptions_CallbackStringsStillConvertedAfterDelegation(t *testing.T) {
	target := map[string]interface{}{}
	source := map[string]interface{}{
		"plugins": map[string]interface{}{
			"tooltip": map[string]interface{}{
				"callbacks": map[string]interface{}{
					"label": "function(ctx) { return ctx.raw; }",
				},
			},
		},
	}

	mergeOptions(target, source)

	callbacks := target["plugins"].(map[string]interface{})["tooltip"].(map[string]interface{})["callbacks"].(map[string]interface{})
	label, ok := callbacks["label"].(map[string]interface{})
	if !ok {
		t.Fatalf("callbacks.label = %v (%T), want the {_function, body} marker map", callbacks["label"], callbacks["label"])
	}
	if label["_function"] != true || label["body"] != "function(ctx) { return ctx.raw; }" {
		t.Fatalf("callbacks.label marker = %v, want _function=true and the original body preserved", label)
	}
}

// TestMergeOptions_CallbackStringsConvertedWhenTargetAlreadyHasCallbacks
// cubre el único caso donde convertCallbackStrings difiere del código viejo:
// cuando target YA trae un mapa "callbacks" (p. ej. un default puesto por el
// renderer) y el autor fusiona otro "callbacks" encima, el código viejo hacía
// merge recursivo de ambos mapas y por eso saltaba el bloque `if key ==
// "callbacks"` (vive en el nivel de arriba, el `continue` del merge
// recursivo ya había disparado) — la string cruda del autor quedaba sin
// envolver en el marcador {_function, body}, así que charts.js nunca la
// reconstruía como función. El post-pase actual recorre el árbol YA
// fusionado buscando "callbacks" a cualquier profundidad, así que sí la
// envuelve. Es un superset estricto deliberado (los defaults de slidelang
// nunca traen "callbacks" hoy, así que no es alcanzable en producción) — ver
// el comentario de convertCallbackStrings en converter.go.
func TestMergeOptions_CallbackStringsConvertedWhenTargetAlreadyHasCallbacks(t *testing.T) {
	target := map[string]interface{}{
		"callbacks": map[string]interface{}{
			"label": "function(ctx) { return 'old'; }",
		},
	}
	source := map[string]interface{}{
		"callbacks": map[string]interface{}{
			"label": "function(ctx) { return 'new'; }",
		},
	}

	mergeOptions(target, source)

	callbacks := target["callbacks"].(map[string]interface{})
	label, ok := callbacks["label"].(map[string]interface{})
	if !ok {
		t.Fatalf("callbacks.label = %v (%T), want the {_function, body} marker map even when target already had a callbacks map", callbacks["label"], callbacks["label"])
	}
	if label["_function"] != true || label["body"] != "function(ctx) { return 'new'; }" {
		t.Fatalf("callbacks.label marker = %v, want the author's overriding body wrapped in the marker", label)
	}
}

// TestConvertChartElementToChartJS_ComboScalesSurviveNestedAuthorOverride es
// la variante end-to-end del primer test: pasa por
// ConvertChartElementToChartJS completo (chart.Options -> mergeOptions) en
// vez de llamar mergeOptions directo, para un chart combo real.
func TestConvertChartElementToChartJS_ComboScalesSurviveNestedAuthorOverride(t *testing.T) {
	chart := &ast.ChartElement{
		ChartType:   "combo",
		SeriesTypes: []string{"bar", "line"},
		Data: [][]interface{}{
			{"Q1", float64(10), float64(1)},
			{"Q2", float64(20), float64(2)},
		},
		Options: map[string]interface{}{
			"scales": map[string]interface{}{
				"y1": map[string]interface{}{
					"max": float64(50),
				},
			},
		},
	}

	config := ConvertChartElementToChartJS(chart, "test", nil)

	scales, ok := config.Options["scales"].(map[string]interface{})
	if !ok {
		t.Fatalf("options.scales missing or not an object: %v", config.Options["scales"])
	}
	y1 := scales["y1"].(map[string]interface{})
	if y1["max"] != float64(50) {
		t.Fatalf("scales.y1.max = %v, want the author's override (50)", y1["max"])
	}
	if y1["position"] != "right" {
		t.Fatalf("scales.y1.position = %v, want the combo-chart default (\"right\") to survive", y1["position"])
	}
	y, ok := scales["y"].(map[string]interface{})
	if !ok || y["position"] != "left" {
		t.Fatalf("scales.y = %v, want the untouched sibling default to survive", scales["y"])
	}
}
