// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// chart_options_test.go cubre un hallazgo de code-review: el switch de
// propiedades del ChartParser solo conocía data/series/labels/title/type, así
// que TODO bloque `options:` escrito en el DSL se descartaba en silencio
// antes de llegar al AST — ChartElement.Options solo se llenaba por la ruta
// combo (parseComboChartYAML), nunca por la normal.
//
// El daño no era solo de configuración perdida. Los ejemplos de
// examples/02_diagrams_and_charts/ ponen el título del chart en
// options.plugins.title.text (que es donde lo espera Chart.js), no en un
// `title:` de primer nivel — así que el HTML llevaba tiempo perdiendo
// títulos que el autor sí había pedido. Y en PPTX el efecto era peor: el
// gate nativo (renderer.SupportsNativeChartRendering) decide con
// elem.Options, y al estar siempre vacío daba por nativo-capaz un chart que
// traía configuración que el renderer nativo no sabe aplicar, rasterizando
// un PNG sin ejes, leyenda ni datalabels y sin ningún warning.

func parseChartLines(t *testing.T, lines []string) *ast.ChartElement {
	t.Helper()
	parser := &ChartParser{}
	result := parser.Parse(&ParseContext{Lines: lines}, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	chart, ok := result.Element.(*ast.ChartElement)
	if !ok {
		t.Fatalf("Element is not *ast.ChartElement, got %T", result.Element)
	}
	return chart
}

func TestChartParser_PopulatesNestedOptions(t *testing.T) {
	chart := parseChartLines(t, []string{
		"<<chart: bar>>",
		"data: [100, 200]",
		`labels: ["Q1", "Q2"]`,
		"options:",
		"  responsive: true",
		"  plugins:",
		"    title:",
		"      display: true",
		`      text: "Sales by Channel"`,
		"    legend:",
		"      position: top",
		"  scales:",
		"    y:",
		"      beginAtZero: true",
		"<</chart>>",
	})

	if chart.Options == nil {
		t.Fatal("Options is nil — the options: block was dropped by the parser")
	}

	if got := chart.Options["responsive"]; got != true {
		t.Errorf(`Options["responsive"] = %v, want true`, got)
	}

	plugins, ok := chart.Options["plugins"].(map[string]interface{})
	if !ok {
		t.Fatalf(`Options["plugins"] is not a map, got %T`, chart.Options["plugins"])
	}
	title, ok := plugins["title"].(map[string]interface{})
	if !ok {
		t.Fatalf(`plugins["title"] is not a map, got %T`, plugins["title"])
	}
	// Éste es el caso que motivó el fix: el título real del chart vive acá,
	// no en un `title:` de primer nivel.
	if got := title["text"]; got != "Sales by Channel" {
		t.Errorf(`plugins.title.text = %v, want "Sales by Channel"`, got)
	}

	scales, ok := chart.Options["scales"].(map[string]interface{})
	if !ok {
		t.Fatalf(`Options["scales"] is not a map, got %T`, chart.Options["scales"])
	}
	if _, ok := scales["y"]; !ok {
		t.Error("scales.y did not survive — nested maps deeper than one level were lost")
	}

	// Las propiedades vecinas no deben romperse por capturar el bloque.
	if len(chart.Labels) != 2 {
		t.Errorf("len(Labels) = %d, want 2 — capturing options: consumed too many lines", len(chart.Labels))
	}
	if len(chart.Data) == 0 {
		t.Error("Data was lost while parsing the options: block")
	}
}

// TestChartParser_OptionsBlockDoesNotSwallowFollowingProperties confirma que
// el bloque se cierra por sangría: una propiedad al mismo nivel que
// `options:` sigue parseándose.
func TestChartParser_OptionsBlockDoesNotSwallowFollowingProperties(t *testing.T) {
	chart := parseChartLines(t, []string{
		"<<chart: line>>",
		"options:",
		"  responsive: false",
		`title: "After the options block"`,
		"data: [1, 2, 3]",
		"<</chart>>",
	})

	if chart.Options == nil {
		t.Fatal("Options is nil")
	}
	if got := chart.Options["responsive"]; got != false {
		t.Errorf(`Options["responsive"] = %v, want false`, got)
	}
	if chart.Title != "After the options block" {
		t.Errorf("Title = %q — the options: block swallowed the properties after it", chart.Title)
	}
	if len(chart.Data) == 0 {
		t.Error("Data after the options: block was lost")
	}
}

// TestChartParser_MalformedOptionsDoesNotBreakChart: un options: que no
// parsea como YAML descarta la config pero deja el chart utilizable — mismo
// comportamiento que antes de que este parser existiera, no una regresión
// que tumbe el documento.
func TestChartParser_MalformedOptionsDoesNotBreakChart(t *testing.T) {
	chart := parseChartLines(t, []string{
		"<<chart: bar>>",
		"options:",
		"  plugins: [unclosed",
		"   : : broken",
		"data: [1, 2]",
		"<</chart>>",
	})

	if chart.ChartType != "bar" {
		t.Errorf("ChartType = %q, want bar — a malformed options: block must not break the chart", chart.ChartType)
	}

	// El "[unclosed" dentro del options: descartado no debe filtrarse al
	// tracking de arrayDepth: si lo hace, la línea "data: [1, 2]" que sigue
	// se lee como continuación de ESE corchete ajeno en vez de como la
	// propiedad data: que es, y chart.Data se pierde en silencio.
	if len(chart.Data) == 0 || len(chart.Data[0]) != 2 {
		t.Errorf("Data = %v, want [[1 2]] — el bloque options: malformado contaminó el arrayDepth y se tragó data:", chart.Data)
	}
}

// TestChartParser_NoOptionsBlockLeavesOptionsNil protege el gate nativo de
// PPTX/DOCX por el otro lado: un chart SIN options: tiene que dejar el campo
// en nil (o vacío) — renderer.classifyChartOptions (issue #148) trata un
// Options nil/vacío como trivialmente calificado (no hay ninguna hoja que
// clasificar), así que este caso sigue siendo nativo-capaz sin cambios.
func TestChartParser_NoOptionsBlockLeavesOptionsNil(t *testing.T) {
	chart := parseChartLines(t, []string{
		"<<chart: bar>>",
		"data: [100, 200]",
		`labels: ["Q1", "Q2"]`,
		"<</chart>>",
	})

	if len(chart.Options) != 0 {
		t.Errorf("Options = %v, want empty for a chart with no options: block", chart.Options)
	}
}
