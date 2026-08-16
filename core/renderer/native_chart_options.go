// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import "github.com/go-analyze/charts"

// native_chart_options.go clasifica el árbol elem.Options de un chart para
// decidir si el camino nativo (go-analyze/charts) puede dibujarlo sin
// desviarse de lo que el autor pidió (issue #148). Antes de esto,
// SupportsNativeChartRendering descalificaba CUALQUIER Options no vacío —
// correcto pero innecesariamente amplio: la mayoría de los charts reales
// traen options: (aunque sea solo "responsive: true" o
// "plugins.title.text"), y ninguna de esas claves cambia el ráster.
//
// La clasificación es por RUTA COMPLETA, no por clave de primer nivel, y
// recorre TODA hoja del árbol — no solo las rutas que matchean algo
// conocido: un chart con `plugins.legend: {position: bottom, labels: {font:
// {size: 14}}}` debe descalificar por `labels.font.size`, aunque
// `position: bottom` sea mapeable. Aprobar en cuanto se encuentra una
// coincidencia (sin seguir revisando hermanos) es exactamente el bug que
// este diseño evita.

// optionLeafRule clasifica una hoja del árbol de Options por su ruta
// completa en puntos (p. ej. "plugins.legend.position"). Una ruta ausente
// de optionLeafRules descalifica el chart por default — deny-by-default,
// el mismo criterio que nativeChartSupportedTypes ya aplica a ChartType.
type optionLeafRule struct {
	// ignorable: la hoja no afecta un ráster estático (p. ej. "responsive")
	// — se ignora sin más, cualquiera que sea su valor.
	ignorable bool
	// mappable, si no es nil, decide POR VALOR si esta hoja tiene traducción
	// real al renderer nativo. Una ruta puede estar en la tabla y aun así
	// descalificar si el valor concreto no es uno de los que mappable
	// aprueba — p. ej. plugins.legend.position: "bottom" es mapeable,
	// "left"/"right" no (moverían la leyenda a un layout que el renderer
	// nativo no reproduce fielmente: fuerza overlay sobre el chart en vez
	// de reservar espacio lateral como Chart.js).
	mappable func(value interface{}) bool
}

func isBoolValue(v interface{}) bool {
	_, ok := v.(bool)
	return ok
}

func isTopOrBottomPosition(v interface{}) bool {
	s, ok := v.(string)
	return ok && (s == "top" || s == "bottom")
}

// isStringValue es el predicado de plugins.title.text: exactamente lo que
// extractNativeChartOverrides sabe traducir (una string, aprobada aunque
// venga vacía — el extractor ya no-opea sobre "" sin más, ver su
// comentario). alwaysMappable (removido en review de PR #159) dejaba pasar
// listas/números/nil, que el extractor descarta en silencio sin que el
// autor se entere de que su "text:" no llegó al chart.
func isStringValue(v interface{}) bool {
	_, ok := v.(string)
	return ok
}

// optionLeafRules es el set de la primera pasada (issue #148): elegido por
// lo que de veras aparece en el corpus (plugins.title.* + legend.position
// es el patrón dominante) y por lo que go-analyze/charts v0.6.0 puede
// aplicar de verdad — ver applyTitleOverrides/applyLegendOverrides/
// applyYAxisOverrides más abajo. Todo lo demás (datalabels, ticks.callback,
// ejes secundarios como scales.y1, scales.x.grid — que ni existe en
// CategoryAxisOption, stacked, cualquier plugin no listado acá) descalifica
// por ausencia de la tabla.
//
// scales.y.min/scales.y.max NO están en este set (removidos en review de
// PR #159), aunque son numéricos y en principio "traducibles" a
// ValueAxisOption.Min/Max: go-analyze/charts (range.go's
// prepareValueAxisRange) solo aplica minCfg/maxCfg cuando EXPANDEN el rango
// de los datos reales (minCfg <= minVal, maxCfg >= maxVal) — un min/max
// que recorta el rango (la semántica que Chart.js sí tiene, y la razón por
// la que un autor escribiría scales.y.max: 50 sobre datos 0..100) se
// ignora en silencio y el chart nativo sale con un rango distinto al que
// Chart.js habría dibujado. Sin acceso a los datos del chart en este
// clasificador (solo ve el árbol de Options), no se puede distinguir en
// qué caso cae un min/max dado, así que se descalifican los dos por
// completo — degradar a chromedp es correcto acá, un ráster nativo
// silenciosamente distinto no lo es.
var optionLeafRules = map[string]optionLeafRule{
	"responsive":              {ignorable: true},
	"maintainAspectRatio":     {ignorable: true},
	"plugins.title.text":      {mappable: isStringValue},
	"plugins.title.display":   {mappable: isBoolValue},
	"plugins.legend.display":  {mappable: isBoolValue},
	"plugins.legend.position": {mappable: isTopOrBottomPosition},
	"scales.y.beginAtZero":    {mappable: isBoolValue},
}

// walkOptionsLeaves recorre node en profundidad, invocando visit(path,
// value) en cada hoja — cualquier valor que no sea un
// map[string]interface{} (incluye arrays, strings, números, bools, nil).
// Un sub-mapa VACÍO no produce ninguna visita: no hay hoja que clasificar
// ahí, y no hay nada que un autor pudiera haber querido decir con "{}".
func walkOptionsLeaves(node interface{}, pathPrefix string, visit func(path string, value interface{})) {
	m, ok := node.(map[string]interface{})
	if !ok {
		visit(pathPrefix, node)
		return
	}
	for key, child := range m {
		childPath := key
		if pathPrefix != "" {
			childPath = pathPrefix + "." + key
		}
		walkOptionsLeaves(child, childPath, visit)
	}
}

// classifyChartOptions decide si TODAS las hojas de options son ignorables
// o mapeables — ver el doc comment del archivo para la invariante completa
// (toda hoja, no toda ruta que matchee).
func classifyChartOptions(options map[string]interface{}) bool {
	qualified := true
	walkOptionsLeaves(options, "", func(path string, value interface{}) {
		if !qualified {
			return
		}
		rule, known := optionLeafRules[path]
		if known && (rule.ignorable || (rule.mappable != nil && rule.mappable(value))) {
			return
		}
		qualified = false
	})
	return qualified
}

// nativeChartOverrides son los valores YA extraídos de un elem.Options
// clasificado como qualified — separado de classifyChartOptions a propósito:
// el clasificador solo necesita saber SI una hoja es segura, el extractor
// necesita el VALOR real para traducirlo. Mismo patrón defensivo (type-assert
// capa por capa, sin panicar ante formas inesperadas) que
// slidelang/internal/generator/pptx.go's chartTitleFromOptions ya usaba
// solo para plugins.title.* en PPTX — este archivo lo generaliza al resto
// del set y lo comparte entre los cuatro tipos de chart nativo y los tres
// formatos que los consumen.
type nativeChartOverrides struct {
	titleText    string
	titleTextSet bool // options.plugins.title.text vino como string no vacío
	titleHide    bool // options.plugins.title.display == false
	legendHide   bool // options.plugins.legend.display == false
	legendBottom bool // options.plugins.legend.position == "bottom" ("top" es el default nativo, no requiere override)
	yBeginAtZero bool // options.scales.y.beginAtZero == true
}

// extractNativeChartOverrides lee las claves mapeables de options
// directamente (sin depender de optionLeafRules ni de walkOptionsLeaves):
// solo se llama después de que classifyChartOptions ya confirmó que options
// no contiene nada fuera de este set, así que cada extracción exitosa acá
// es sabida segura de antemano.
func extractNativeChartOverrides(options map[string]interface{}) nativeChartOverrides {
	var out nativeChartOverrides

	plugins, _ := options["plugins"].(map[string]interface{})

	if title, ok := plugins["title"].(map[string]interface{}); ok {
		if display, ok := title["display"].(bool); ok && !display {
			out.titleHide = true
		}
		if text, ok := title["text"].(string); ok && text != "" {
			out.titleText, out.titleTextSet = text, true
		}
	}

	if legend, ok := plugins["legend"].(map[string]interface{}); ok {
		if display, ok := legend["display"].(bool); ok && !display {
			out.legendHide = true
		}
		if position, ok := legend["position"].(string); ok && position == "bottom" {
			out.legendBottom = true
		}
	}

	if scales, ok := options["scales"].(map[string]interface{}); ok {
		if y, ok := scales["y"].(map[string]interface{}); ok {
			if begin, ok := y["beginAtZero"].(bool); ok && begin {
				out.yBeginAtZero = true
			}
		}
	}

	return out
}

// applyTitleOverrides traduce plugins.title.* sobre el TitleOption nativo
// (Title y Legend son campos idénticos en los cuatro structs de opciones de
// chart — Bar/Line/Pie/DoughnutChartOption — así que un solo aplicador
// sirve a los cuatro tipos, sin repetir la lógica por tipo). elemTitle es
// elem.Title (el campo de primer nivel del DSL, no options): gana por ser
// más explícito, igual que decidió chartTitleFromOptions para PPTX — la
// rescate de options solo aplica cuando elemTitle vino vacío.
func applyTitleOverrides(t *charts.TitleOption, elemTitle string, ov nativeChartOverrides) {
	t.Text = elemTitle
	if t.Text == "" && ov.titleTextSet {
		t.Text = ov.titleText
	}
	if ov.titleHide {
		t.Show = charts.Ptr(false)
	}
}

// applyLegendOverrides traduce plugins.legend.display/position. "top" no
// necesita override (es el default nativo cuando Offset.Top va vacío,
// ver legend.go's computeLayoutParams); "bottom" se traduce a
// PositionBottom. "left"/"right" NO están en optionLeafRules (ver su
// comentario) — nunca llegan acá.
func applyLegendOverrides(l *charts.LegendOption, ov nativeChartOverrides) {
	if ov.legendHide {
		l.Show = charts.Ptr(false)
	}
	if ov.legendBottom {
		l.Offset.Top = charts.PositionBottom
	}
}

// applyYAxisOverrides traduce scales.y.beginAtZero sobre el eje Y
// (ValueAxisOption — el mismo tipo subyacente que BarChartOption.ValueAxis[0]
// y LineChartOption.YAxis[0] usan, YAxisOption es un alias, no un tipo
// nuevo). Solo aplica a bar/line: pie/doughnut no tienen eje, así que un
// chart de esos tipos con scales.y.* en options simplemente no tiene a
// quién aplicárselo — exactamente lo que Chart.js también hace (scales no
// significa nada para un pie chart).
//
// scales.y.min/max NO se traducen acá (removidos de optionLeafRules en
// review de PR #159, ver su comentario): go-analyze/charts solo respeta un
// Min/Max configurado cuando EXPANDE el rango de los datos reales, no
// cuando lo recorta, así que un min/max que sí llegara hasta acá podría
// dibujar un rango distinto al que Chart.js habría producido. El
// clasificador ya los descalifica antes de que extractNativeChartOverrides
// los vea.
func applyYAxisOverrides(axis *charts.ValueAxisOption, ov nativeChartOverrides) {
	if ov.yBeginAtZero {
		axis.Min = charts.Ptr(0.0)
	}
}
