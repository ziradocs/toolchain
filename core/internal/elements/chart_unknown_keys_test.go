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

// TestChartParser_ReportsUnknownAttributeOnEveryReturnPath es la regresión del
// hallazgo de code review del PR #232: ChartParser.Parse tiene TRES salidas
// —JSON directo, YAML de combo, y el loop de propiedades— y el aviso de
// atributos desconocidos estaba escrito a mano en dos de ellas. La de combo
// devolvía un ParseResult sin Diagnostics, así que
// `<<chart: combo title="...">>` volvía a perder el título en silencio: justo
// el defecto que CHART005 existe para señalar.
//
// El arreglo estructural es armar el diagnóstico una sola vez apenas parseada
// la apertura, en una variable que las tres salidas devuelven. Este test es lo
// que fija esa propiedad: si mañana aparece una cuarta salida que se olvide
// de `diags`, hay que agregarle su caso acá y va a fallar hasta que lo haga.
func TestChartParser_ReportsUnknownAttributeOnEveryReturnPath(t *testing.T) {
	cases := []struct {
		name string
		// path nombra la salida de Parse que ejercita el caso, para que al
		// fallar quede claro cuál se rompió.
		path string
		src  string
	}{
		{
			name: "loop de propiedades",
			path: "salida final, tras el loop de propiedades",
			src: `<<chart:bar title="Performance Metrics">>
  data: [85, 90]
<<end>>`,
		},
		{
			name: "JSON directo",
			path: "return temprano de la rama IsJSONMode",
			src: `<<chart:bar title="Performance Metrics">>
{"type": "bar", "data": {"labels": ["Q1"], "datasets": [{"data": [85]}]}}
<</chart>>`,
		},
		{
			name: "YAML de combo",
			path: "return temprano de la rama parseComboChartYAML",
			src: `<<chart: combo title="Performance Metrics">>
  data:
    labels: ["Ene", "Feb"]
    series:
      - name: "Desktop"
        type: "bar"
        values: [65, 59]
      - name: "Total"
        type: "line"
        values: [45, 52]
<<end>>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := parseChartBlock(t, tc.src)
			if !hasDiagnostic(diags, "CHART005", "title") {
				t.Errorf("no se reportó `title=` por la %s.\n"+
					"El diagnóstico se arma una sola vez apenas parseada la apertura; esta salida está devolviendo un "+
					"ParseResult sin propagar `diags`.\ndiagnósticos: %v", tc.path, diags)
			}
		})
	}
}

// TestChartParser_ComboYAMLPathIsActuallyExercised evita que el caso combo de
// arriba pase por la razón equivocada. `<<chart: combo>>` con la forma PLANA
// (type/data/series) no entra a parseComboChartYAML: cae al loop de
// propiedades, que ya reportaba bien. Si el fixture de combo dejara de tomar
// la rama YAML —por un cambio en ChartConfig, en parseYAMLBlock o en el shape
// del test— aquel caso seguiría verde sin cubrir nada.
//
// La huella de haber pasado por parseComboChartYAML es Labels: el loop plano
// solo puebla Labels desde una llave `labels:` de nivel superior, que este
// fixture no tiene.
func TestChartParser_ComboYAMLPathIsActuallyExercised(t *testing.T) {
	chart, _ := parseChartBlock(t, `<<chart: combo>>
  data:
    labels: ["Ene", "Feb"]
    series:
      - name: "Desktop"
        type: "bar"
        values: [65, 59]
<<end>>`)

	if len(chart.Labels) != 2 || chart.Labels[0] != "Ene" {
		t.Fatalf("el fixture de combo no está tomando la rama parseComboChartYAML (Labels = %v).\n"+
			"Sin esa rama, el caso \"YAML de combo\" de TestChartParser_ReportsUnknownAttributeOnEveryReturnPath "+
			"no cubre el return temprano que motivó el test.", chart.Labels)
	}
}

// TestChartParser_ReportsUnknownKeyInComboYAML es la regresión de un segundo
// hallazgo de code review sobre el mismo defecto: TestChartParser_
// ReportsUnknownAttributeOnEveryReturnPath cubre la llave de la APERTURA
// (`title=`) en las tres salidas, pero `parseComboChartYAML` decodifica el
// cuerpo YAML con yaml.Unmarshal a un struct tagueado (ChartConfig), que
// ignora en silencio cualquier llave del mapeo sin campo correspondiente —
// a diferencia del loop de propiedades de la forma plana, que sí las junta
// línea por línea. Un `datasets:` de nivel superior en un combo chart (el
// mismo typo de la plantilla `report` de `doclang init`, pero en la forma
// YAML en vez de la plana) se perdía sin CHART005 aunque el resto del chart
// tuviera datos válidos.
func TestChartParser_ReportsUnknownKeyInComboYAML(t *testing.T) {
	chart, diags := parseChartBlock(t, `<<chart: combo>>
  data:
    labels: ["Ene", "Feb"]
    series:
      - name: "Desktop"
        type: "bar"
        values: [65, 59]
  datasets:
    backgroundColor: "#3498db"
<<end>>`)

	if !hasDiagnostic(diags, "CHART005", "datasets") {
		t.Errorf("no se reportó `datasets:` como llave desconocida del cuerpo YAML del combo chart.\ndiagnósticos: %v", diags)
	}
	// El combo debe seguir parseando bien pese a la llave desconocida: es un
	// aviso, no un error que tumbe el chart.
	if len(chart.Labels) != 2 {
		t.Errorf("la llave desconocida no debería impedir que el resto del combo se parsee: Labels = %v", chart.Labels)
	}
}

// TestChartParser_ReportsUnknownKeyInsideComboData es la regresión de un
// cuarto hallazgo de code review, un nivel más profundo que
// TestChartParser_ReportsUnknownKeyInComboYAML: aquel cubre una llave
// desconocida de NIVEL SUPERIOR (`datasets:` junto a `data:`), pero
// ChartData/ChartSeries (los structs que `data:` decodifica) tienen el MISMO
// defecto en su interior — yaml.Unmarshal a un struct tagueado ignora en
// silencio cualquier campo del mapeo sin tag correspondiente. `data.Labels:`
// (mayúscula, en vez de `labels:`) no dispara ningún CHART005 y el chart se
// renderiza con Labels vacío: el defecto no está resuelto, solo tapado un
// nivel arriba.
func TestChartParser_ReportsUnknownKeyInsideComboData(t *testing.T) {
	chart, diags := parseChartBlock(t, `<<chart: combo>>
  data:
    Labels: ["Ene", "Feb"]
    series:
      - name: "Desktop"
        type: "bar"
        values: [65, 59]
<<end>>`)

	if !hasDiagnostic(diags, "CHART005", "data.Labels") {
		t.Errorf("no se reportó `data.Labels:` (mayúscula, en vez de `labels:`) como llave desconocida.\ndiagnósticos: %v", diags)
	}
	if len(chart.Labels) != 0 {
		t.Errorf("chart.Labels = %v; `Labels:` con mayúscula no es la llave `labels:` que ChartData conoce, no debería poblarse", chart.Labels)
	}
}

// TestChartParser_ReportsUnknownKeyInsideComboSeriesItem cubre el mismo
// defecto un nivel más adentro todavía: un campo desconocido DENTRO de un
// elemento de `data.series:` (ChartSeries), no en `data:` mismo. Un solo
// aviso por llave, no uno por cada serie que repita el typo — de ahí el
// fixture con DOS series con la misma llave desconocida.
func TestChartParser_ReportsUnknownKeyInsideComboSeriesItem(t *testing.T) {
	chart, diags := parseChartBlock(t, `<<chart: combo>>
  data:
    labels: ["Ene", "Feb"]
    series:
      - name: "Desktop"
        type: "bar"
        Values: [65, 59]
      - name: "Mobile"
        type: "line"
        Values: [10, 20]
<<end>>`)

	count := 0
	for _, d := range diags {
		if strings.HasPrefix(d, "CHART005:") {
			count += strings.Count(d, "data.series[].Values")
		}
	}
	if count != 1 {
		t.Errorf("se esperaba que `data.series[].Values` aparezca UNA vez en los diagnósticos (una por llave, no una por serie que la repita), apareció %d veces: %v", count, diags)
	}
	if len(chart.Series) != 2 || chart.Series[0] != "Desktop" {
		t.Errorf("el resto del combo debe seguir parseando pese al campo desconocido: Series = %v", chart.Series)
	}
}

// TestChartParser_ReportsCapitalizedUnknownKey es la regresión de un tercer
// hallazgo: chartKeyRe exigía minúscula inicial para decidir si una llave
// desconocida del cuerpo valía la pena reportarse, un filtro pensado para
// evitar falsos positivos de prosa que el loop escanea de más (ver el
// comentario del `default:` en Parse) — pero esos falsos positivos
// documentados ("- **Traces", cadena vacía) ya quedan afuera por tener
// espacios y puntuación, no por el case. La minúscula inicial de más solo
// lograba que un typo tan real como `Title:` (en vez de `title:`) se
// descartara en silencio, sin CHART005, pese a no ser prosa de ningún tipo.
func TestChartParser_ReportsCapitalizedUnknownKey(t *testing.T) {
	_, diags := parseChartBlock(t, `<<chart: bar>>
  Title: "Ventas"
  data: [85, 90]
<<end>>`)

	if !hasDiagnostic(diags, "CHART005", "Title") {
		t.Errorf("no se reportó `Title:` (mayúscula inicial) como llave desconocida del cuerpo.\ndiagnósticos: %v", diags)
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
