// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// chart_dedent_test.go cubre la pérdida silenciosa de contenido de un
// <<chart>> cerrado por dedent (sin <<end>> explícito), que es la convención
// documentada en spec/language-specification.md:75
// (element_terminator ::= "<<end>>" | block_boundary | EOF).
//
// El loop de propiedades no tenía NINGÚN corte para una línea que no fuera
// una propiedad conocida: la prosa y el bloque @notes: que seguían al chart
// caían hasta el consumedLines++ del final del loop. Como ConsumedLines es
// lo que le dice al llamador cuántas líneas saltar, todo lo escaneado de más
// DESAPARECÍA del documento. El chart en sí renderizaba bien, así que la
// pérdida no dejaba rastro: ni diagnóstico, ni salida rara.
//
// Se asserta sobre ConsumedLines y no sobre el HTML porque es lo que fija el
// defecto directamente: no depende del renderer ni de cómo se emiten las
// @notes. El harness de round-trip tampoco lo veía (compara
// parse→format→reparse, y el contenido ya se perdió antes del primer
// format).

// assertChartConsumes parsea lines[0:] como chart y verifica que consuma
// exactamente wantConsumed líneas — es decir, que el bloque termine donde
// termina y no se coma lo que sigue.
func assertChartConsumes(t *testing.T, lines []string, wantConsumed int) *ast.ChartElement {
	t.Helper()
	parser := &ChartParser{}
	result := parser.Parse(&ParseContext{Mode: "flex", Lines: lines}, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	if result.ConsumedLines != wantConsumed {
		t.Errorf("ConsumedLines = %d, want %d — el chart se comió %d línea(s) de más, que el llamador salta y se pierden del documento",
			result.ConsumedLines, wantConsumed, result.ConsumedLines-wantConsumed)
		for i, l := range lines {
			mark := "  "
			if i < result.ConsumedLines {
				mark = "->"
			}
			t.Logf("%s [%d] %q", mark, i, l)
		}
	}
	chart, ok := result.Element.(*ast.ChartElement)
	if !ok {
		t.Fatalf("Element is not *ast.ChartElement, got %T", result.Element)
	}
	return chart
}

// TestChartParser_DedentClosedDoesNotSwallowProse es el repro literal del
// bug, calcado de examples/use-cases/educational/ml_fundamentals.slidelang
// (chart en la línea 252): un doughnut con options: anidado, cerrado por
// dedent, seguido de un párrafo y un bloque @notes:. Antes se consumían las
// 13 líneas — el párrafo y las notas completas se perdían.
func TestChartParser_DedentClosedDoesNotSwallowProse(t *testing.T) {
	lines := []string{
		"<<chart: doughnut>>",                 // 0
		"  data: [30, 25, 20]",                // 1
		`  labels: ["Tree-based", "NN", "X"]`, // 2
		"  options:",                          // 3
		"    responsive: true",                // 4
		"    plugins:",                        // 5
		"      title:",                        // 6
		"        display: true",               // 7
		`        text: "Popular Types"`,       // 8
		"",                                    // 9
		"**Different algorithms excel at different problems**", // 10
		"",        // 11
		"@notes:", // 12
		"- Industry usage gives real-world perspective", // 13
	}
	// El bloque son las líneas 0-8; la 9 en blanco se consume como parte del
	// bloque. Todo desde la 10 debe quedar para el parser de nivel superior.
	chart := assertChartConsumes(t, lines, 10)

	if chart.Title == "" && chart.Options == nil {
		t.Error("el chart perdió su configuración: Options y Title vacíos")
	}
	if len(chart.Labels) != 3 {
		t.Errorf("Labels = %v, want 3 elementos", chart.Labels)
	}
}

// TestChartParser_DedentClosedStopsAtNotes fija el caso mínimo: una
// directiva @notes: pegada al chart, sin párrafo de por medio.
func TestChartParser_DedentClosedStopsAtNotes(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",
		"  data: [1, 2]",
		"@notes:",
		"- se pierde si el chart se la come",
	}
	assertChartConsumes(t, lines, 2)
}

// TestChartParser_MultiLineArrayCloserIsBlockContent fija la primera de las
// dos formas en que los sub-parsers de array quedan cortos y dejan restos
// para el loop de propiedades: parseMultiLineArray corta por sangría
// (ShouldProcessLine) ANTES de llegar a su chequeo de "]", así que el
// corchete de cierre dedentado respecto a sus filas nunca lo consume él.
//
// Si el loop lo tratara como límite en vez de como continuación, el chart
// terminaría en el corchete y perdería labels:/series:/options: — datos del
// propio chart, peor que el bug que este archivo cubre.
func TestChartParser_MultiLineArrayCloserIsBlockContent(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",               // 0
		"  data: [",                    // 1
		"    [1820, 890],",             // 2
		"    [1650, 780]",              // 3
		"  ]",                          // 4
		`  labels: ["Ent", "Mid"]`,     // 5
		`  series: ["Q1", "Q4"]`,       // 6
		"",                             // 7
		"**Prosa que sigue al chart**", // 8
	}
	chart := assertChartConsumes(t, lines, 8)

	if len(chart.Labels) != 2 {
		t.Errorf("Labels = %v, want 2 — el corchete de cierre cortó el bloque antes de labels:", chart.Labels)
	}
	if len(chart.Series) != 2 {
		t.Errorf("Series = %v, want 2 — el corchete de cierre cortó el bloque antes de series:", chart.Series)
	}
}

// TestChartParser_UnindentedArrayRowsAreBlockContent fija la segunda forma:
// con el array en columna 0, ShouldProcessLine devuelve false en la PRIMERA
// fila (ExpectedIndent == -1 y sangría 0), así que parseMultiLineArray
// consume 0 líneas y todas las filas caen al loop de propiedades.
//
// Es la forma del fixture de
// internal/normalize/normalizer/rules/enhancement/chart_formatter_options_test.go
// y de examples/use-cases/educational/machine_learning_intro.slidelang, donde
// las propiedades van sin sangrar. Por eso el corte por sangría de mermaid.go
// no sirve para charts: ahí borraría el bloque entero.
func TestChartParser_UnindentedArrayRowsAreBlockContent(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",               // 0
		"data: [",                      // 1
		`["Q1", 1]`,                    // 2
		"]",                            // 3
		"options:",                     // 4
		"  scales:",                    // 5
		"    y:",                       // 6
		"      beginAtZero: true",      // 7
		"",                             // 8
		"**Prosa que sigue al chart**", // 9
	}
	chart := assertChartConsumes(t, lines, 9)

	if chart.Options == nil {
		t.Fatal("Options = nil — las filas del array en columna 0 cortaron el bloque antes de options:")
	}
	scales, ok := chart.Options["scales"].(map[string]interface{})
	if !ok {
		t.Fatalf(`Options["scales"] no es un map, got %#v`, chart.Options["scales"])
	}
	if _, ok := scales["y"].(map[string]interface{}); !ok {
		t.Errorf(`Options["scales"]["y"] no es un map, got %#v`, scales["y"])
	}
}

// TestChartParser_ExplicitEndStillConsumesIt confirma que la forma cerrada
// con <<end>> no cambió: el cierre se consume y nada de lo que sigue entra.
func TestChartParser_ExplicitEndStillConsumesIt(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",
		"  data: [1, 2]",
		"<<end>>",
		"**Prosa que sigue al chart**",
	}
	assertChartConsumes(t, lines, 3)
}
