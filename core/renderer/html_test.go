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

// TestRenderChartElement_JSONMode_EscapesScriptBreakout cubre issue #19 (CR-1
// del audit de seguridad 2026-07): un chart en modo JSON directo cuyo RawJSON
// contiene un literal "</script>" no debe poder cerrar el <script
// type="application/json"> en el que renderChartBrowser lo embebe e inyectar
// HTML/JS ejecutable.
func TestRenderChartElement_JSONMode_EscapesScriptBreakout(t *testing.T) {
	ctx := &RenderContext{ChartMode: "browser"}

	pos := diagnostics.NewPosition(1, 1)
	chart := ast.NewChartElement(pos, "bar")
	chart.IsJSONMode = true
	chart.RawJSON = json.RawMessage(`{"type":"bar","data":{"labels":["</script><img src=x onerror=alert(document.domain)>"]}}`)

	html := renderChartElement(chart, nil, ctx)

	if strings.Contains(html, "</script><img") {
		t.Fatalf("chart HTML contains an unescaped script-breakout payload:\n%s", html)
	}

	// El JSON embebido debe seguir siendo válido y, al decodificarse, producir
	// el string original intacto (Chart.js debe recibir el dato tal cual).
	start := strings.Index(html, `class="chart-config">`) + len(`class="chart-config">`)
	end := strings.Index(html[start:], "</script>")
	if start < len(`class="chart-config">`) || end < 0 {
		t.Fatalf("could not locate the chart-config <script> block in: %s", html)
	}
	embedded := html[start : start+end]

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(embedded), &decoded); err != nil {
		t.Fatalf("embedded chart config is not valid JSON: %v\n%s", err, embedded)
	}
	labels := decoded["data"].(map[string]interface{})["labels"].([]interface{})
	want := `</script><img src=x onerror=alert(document.domain)>`
	if labels[0] != want {
		t.Errorf("decoded label = %q, want %q (round-trip must preserve the original string)", labels[0], want)
	}
}

// TestGenerateChartConfig_PieDoughnutMoreThanEightSeries cubre issue #244:
// GenerateChartConfigWithMode paniqueaba ("slice bounds out of range") en la
// rama pie/doughnut con más de 8 filas de datos — colors[:len(values)] sobre
// una paleta de 8 colores, con input perfectamente válido. El fix cicla la
// paleta con el módulo (colors[i%len(colors)]), igual que las ramas
// combo/bar/line — este test construye 9 filas (una más que la paleta) para
// que un regreso al slicing directo vuelva a paniquear.
func TestGenerateChartConfig_PieDoughnutMoreThanEightSeries(t *testing.T) {
	for _, chartType := range []string{"pie", "doughnut"} {
		t.Run(chartType, func(t *testing.T) {
			pos := diagnostics.NewPosition(1, 1)
			chart := ast.NewChartElement(pos, chartType)
			chart.Data = make([][]interface{}, 9)
			for i := range chart.Data {
				chart.Data[i] = []interface{}{"Label", float64(i + 1)}
			}

			var config string
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("GenerateChartConfig panicked with %d data rows: %v", len(chart.Data), r)
					}
				}()
				config = GenerateChartConfig(chart)
			}()

			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(config), &decoded); err != nil {
				t.Fatalf("chart config is not valid JSON: %v\n%s", err, config)
			}
			datasets := decoded["data"].(map[string]interface{})["datasets"].([]interface{})
			dataset := datasets[0].(map[string]interface{})
			backgroundColor := dataset["backgroundColor"].([]interface{})
			if len(backgroundColor) != len(chart.Data) {
				t.Errorf("backgroundColor has %d entries, want %d (one per data row, palette cycled)", len(backgroundColor), len(chart.Data))
			}
		})
	}
}

// TestResolveChartJSONMode cubre issue #55: la resolución de un ChartElement
// en modo JSON directo es ahora una única fuente de verdad compartida por
// slidelang (converter.go) y este mismo paquete (renderChartElement,
// usado por doclang) — antes cada uno la reimplementaba de forma
// independiente, y solo una de las dos respetaba RawJSON/IsJSONMode
// correctamente (issue histórico #11).
func TestResolveChartJSONMode(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	t.Run("not JSON mode returns nil without error", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "bar")
		config, chartType, err := ResolveChartJSONMode(chart)
		if err != nil || config != nil || chartType != "" {
			t.Fatalf("got (%v, %q, %v), want (nil, \"\", nil)", config, chartType, err)
		}
	})

	t.Run("invalid RawJSON returns an error", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "bar")
		chart.IsJSONMode = true
		chart.RawJSON = json.RawMessage(`{not valid json`)
		config, _, err := ResolveChartJSONMode(chart)
		if err == nil || config != nil {
			t.Fatalf("got (%v, err=%v), want (nil, non-nil error)", config, err)
		}
	})

	t.Run("type present in JSON is preserved as-is", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "bar")
		chart.IsJSONMode = true
		chart.RawJSON = json.RawMessage(`{"type":"pie","data":{}}`)
		config, chartType, err := ResolveChartJSONMode(chart)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chartType != "pie" || config["type"] != "pie" {
			t.Fatalf("got type=%q config[type]=%v, want \"pie\"", chartType, config["type"])
		}
	})

	t.Run("missing type in JSON falls back to the <<chart: TYPE>> tag", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "bar")
		chart.IsJSONMode = true
		chart.RawJSON = json.RawMessage(`{"data":{}}`)
		config, chartType, err := ResolveChartJSONMode(chart)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chartType != "bar" || config["type"] != "bar" {
			t.Fatalf("got type=%q config[type]=%v, want \"bar\" (from the tag)", chartType, config["type"])
		}
	})
}

// TestRenderTextElement_RawHTMLEscapesVariableValues es una regresión
// encontrada en code-review de la PR de XSS (docs/SECURITY_AUDIT_2026-07.md,
// CR-2): un TextElement crudo (p. ej. un heading de subsección) sustituía
// {{variable}} vía ProcessVariables (sin escapar), así que un valor de
// variable con HTML se inyectaba directo en el <h2>/<h3> del cuerpo del
// documento. renderTextElement ahora usa ProcessVariablesEscapeValues.
func TestRenderTextElement_RawHTMLEscapesVariableValues(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewRawHTMLTextElement(pos, `<h2 id="config">Config {{evil}}</h2>`)

	variables := map[string]interface{}{"evil": "<script>alert(1)</script>"}
	html := renderTextElement(elem, variables)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("raw HTML text element leaked an unescaped variable value:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("expected the variable's value to be HTML-escaped, got:\n%s", html)
	}
	if !strings.HasPrefix(strings.TrimSpace(html), "<h2") {
		t.Errorf("expected the surrounding <h2> HTML to be preserved untouched, got:\n%s", html)
	}
}

// TestRenderMermaidBrowser_EscapesContent cubre issue #73: a diferencia de
// los raster builders de chromium_renderer.go (que ya escapaban el source vía
// EscapeHTML desde PR #67), renderMermaidBrowser emitía el diagrama sin
// escapar en el HTML por defecto de doclang (modo "browser"), un XSS
// zero-interaction en la salida estándar de `doclang build`.
func TestRenderMermaidBrowser_EscapesContent(t *testing.T) {
	payload := `</div><img src=x onerror=alert(document.domain)><script>alert(1)</script>`
	html := renderMermaidBrowser(payload)

	if strings.Contains(html, "<img src=x onerror") || strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("mermaid browser content was not escaped, breakout survived:\n%s", html)
	}
	if !strings.Contains(html, "&lt;img") || !strings.Contains(html, "&lt;/div&gt;") {
		t.Errorf("expected escaped payload to appear as HTML entities, got:\n%s", html)
	}
}

// TestRenderMermaidElement_DefaultModeEscapesContent prueba el sink completo
// alcanzado por el pipeline real de doclang (renderMermaidElement, modo
// "browser" por defecto), no solo la función interna.
func TestRenderMermaidElement_DefaultModeEscapesContent(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewMermaidElement(pos, "flowchart", `A["</div><img src=x onerror=alert(document.domain)>"] --> B`)

	// ctx nil -> resolveRenderContext usa el default (MermaidMode "browser")
	html := renderMermaidElement(elem, nil, nil)

	if strings.Contains(html, "<img src=x onerror") {
		t.Fatalf("mermaid element (default/browser mode) leaked unescaped content:\n%s", html)
	}
	if !strings.Contains(html, "&lt;img") {
		t.Errorf("expected escaped payload to appear as HTML entities, got:\n%s", html)
	}
}
