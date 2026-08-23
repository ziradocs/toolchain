// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// package enhancement_test (no enhancement): estos tests parsean el
// documento completo con parser.NewDocumentFlexParserWithNormalization, así
// que necesitan importar el paquete parser — que a su vez importa
// internal/normalize/normalizer, que importa este paquete. Como paquete de
// test EXTERNO, Go lo compila aparte del grafo de imports de enhancement, así
// que el ciclo no existe en el binario real (mismo patrón que
// core/formatter/document_roundtrip_test.go usa para probar el pipeline
// completo desde texto DSL).
package enhancement_test

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/rules/enhancement"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Estos tests cubren el issue #153: ChartFormatterRule (el re-indentador de
// bloques <<chart:>> del normalizer) reconstruía la indentación del
// sub-bloque options: desde una tabla de 4 nombres fijos (plugins/title/
// display/text), así que cualquier otra clave de Chart.js — scales,
// elements, interaction, cualquier plugin — salía aplanada a hermana de sus
// propios padres. Y peor: el flag que marcaba "estamos dentro de options:"
// nunca se apagaba (era un latch de una sola vía), así que una propiedad de
// primer nivel del chart escrita DESPUÉS de options: se emitía más profunda
// que el propio options: y el parser se la tragaba — pérdida silenciosa de
// datos del chart, no solo nesting corrupto.
//
// El fix reemplaza la reconstrucción semántica por un passthrough: copiar el
// sub-bloque options: preservando su profundidad relativa original. Estos
// tests van end-to-end desde texto DSL (no un AST armado a mano) porque el
// hueco vivía justo en la costura entre "el normalizer reescribe el texto" y
// "el parser lo lee" — un AST a mano se la salta entera.

func findChart(t *testing.T, doc *ast.AST) *ast.ChartElement {
	t.Helper()
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if chart, ok := el.(*ast.ChartElement); ok {
				return chart
			}
		}
	}
	t.Fatal("no se encontró ningún ChartElement en el documento parseado")
	return nil
}

func parseNormalized(t *testing.T, content string) *ast.AST {
	t.Helper()
	p := parser.NewDocumentFlexParserWithNormalization(content, util.NewNoop())
	doc, diags := p.Parse()
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("parse error: %s", d.Message)
		}
	}
	return doc
}

// TestChartFormatterRule_OptionsLatch_DoesNotSwallowFollowingProperty es el
// test discriminante: el único que atrapa el fallo real (más grave que lo
// que describe el issue). Un chart sin indentar, con options: anidado dos
// niveles, SEGUIDO de otra propiedad de primer nivel (title:). Si el
// passthrough calcula mal el baseIndent que va a leer
// ChartParser.parseNestedOptions (core/internal/elements/chart.go:603, que
// toma la profundidad de la PRIMERA línea hija, no de la línea options:),
// una copia "verbatim" ingenua deja title: a la misma indentación que sus
// propios hijos y el parser se lo traga como si fuera parte de options.
func TestChartFormatterRule_OptionsLatch_DoesNotSwallowFollowingProperty(t *testing.T) {
	doc := parseNormalized(t, `---
mode: flex
---

# Chart Test

<<chart: bar>>
data: [
["Q1", 1]
]
options:
  scales:
    y:
      beginAtZero: true
title: "After the options block"
<</chart>>
`)

	chart := findChart(t, doc)

	if chart.Title != "After the options block" {
		t.Errorf("Title = %q, want %q — la propiedad después de options: fue tragada por el sub-bloque (el bug del latch)",
			chart.Title, "After the options block")
	}

	scales, ok := chart.Options["scales"].(map[string]interface{})
	if !ok {
		t.Fatalf(`Options["scales"] no es un map, got %#v (Options=%#v)`, chart.Options["scales"], chart.Options)
	}
	y, ok := scales["y"].(map[string]interface{})
	if !ok {
		t.Fatalf(`Options["scales"]["y"] no es un map, got %#v`, scales["y"])
	}
	if got := y["beginAtZero"]; got != true {
		t.Errorf(`Options.scales.y.beginAtZero = %v, want true`, got)
	}
}

// TestChartFormatterRule_OptionsNestingSurvivesFormatting reproduce el repro
// literal del issue #153: options: sin indentar con un sub-bloque anidado
// dos niveles cuyas claves (scales/y/beginAtZero) NO están en la tabla de 4
// nombres que calculateIndentLevelSemantic conocía. Antes del fix salía
// aplanado a tres hermanos: {"beginAtZero":true,"scales":null,"y":null}.
func TestChartFormatterRule_OptionsNestingSurvivesFormatting(t *testing.T) {
	content := `---
mode: flex
---

# Chart Test

<<chart: bar>>
data: [
["Q1", 1]
]
options:
  scales:
    y:
      beginAtZero: true
<</chart>>
`
	normalized := findChart(t, parseNormalized(t, content))
	unnormalized := findChart(t, mustParseUnnormalized(t, content))

	// El parser en sí es correcto (ver core/internal/elements/chart_options_test.go);
	// contrastar contra la ruta SIN normalizer aísla al normalizer como el
	// único responsable de cualquier diferencia — así se diagnosticó el bug
	// originalmente.
	wantScales, ok := unnormalized.Options["scales"].(map[string]interface{})
	if !ok {
		t.Fatalf("fixture inválido: la ruta sin normalizar no dio scales anidado, got %#v", unnormalized.Options)
	}
	wantY, ok := wantScales["y"].(map[string]interface{})
	if !ok || wantY["beginAtZero"] != true {
		t.Fatalf("fixture inválido: la ruta sin normalizar no dio scales.y.beginAtZero, got %#v", wantScales)
	}

	gotScales, ok := normalized.Options["scales"].(map[string]interface{})
	if !ok {
		t.Fatalf(`Options["scales"] no es un map tras normalizar, got %#v (Options=%#v) — el bloque salió aplanado`,
			normalized.Options["scales"], normalized.Options)
	}
	gotY, ok := gotScales["y"].(map[string]interface{})
	if !ok {
		t.Fatalf(`Options["scales"]["y"] no es un map tras normalizar, got %#v`, gotScales["y"])
	}
	if got := gotY["beginAtZero"]; got != true {
		t.Errorf(`Options.scales.y.beginAtZero tras normalizar = %v, want true`, got)
	}
	if _, flattened := normalized.Options["beginAtZero"]; flattened {
		t.Errorf("Options.beginAtZero apareció en el nivel superior — el sub-bloque se aplanó (Options=%#v)", normalized.Options)
	}
	if _, flattened := normalized.Options["y"]; flattened {
		t.Errorf("Options.y apareció en el nivel superior — el sub-bloque se aplanó (Options=%#v)", normalized.Options)
	}
}

// TestChartFormatterRule_ComboDataBlock_SurvivesFormatting cubre el hermano
// del bug de options: (issue #153) en el bloque "data:" de un combo chart:
// ChartParser.parseComboChartYAML (core/internal/elements/chart.go) espera
// "data:" bare seguido de un sub-árbol YAML anidado (labels: + series:, con
// "- name/type/values" por serie). isMainChartProperty reconoce "series:" y
// "labels:" por nombre sin mirar el contexto, así que — antes del bypass —
// calculateIndentLevelSemantic los aplanaba a hermanas de "data:" en vez de
// preservarlos anidados, y needsFormatting los contaba como mal formados
// aunque el bloque ya fuera válido, dañando el ratio por debajo de 0.7 y
// disparando exactamente esa reescritura destructiva.
//
// La reescritura solo se dispara cuando el detector de "contenido AI" del
// documento completo cruza el umbral (por una frase en CUALQUIER parte del
// documento, no necesariamente cerca del chart) — repro real: un combo chart
// bien formado en examples/use-cases/consulting/client_discovery_workshop.slidelang
// se corrompía en silencio solo porque el documento contenía la sección
// "Immediate Action Items" varios cientos de líneas más abajo.
func TestChartFormatterRule_ComboDataBlock_SurvivesFormatting(t *testing.T) {
	content := `---
mode: flex
---

# Combo Chart Test

<<chart: combo>>
  data:
    labels: ["2022", "2023"]
    series:
      - name: "Revenue ($M)"
        type: "bar"
        values: [28, 35]
      - name: "Profit Margin (%)"
        type: "line"
        values: [12, 15]
        yAxisID: "y1"
  options:
    responsive: true
<</chart>>

---

# Immediate Action Items

Trigger AI-content detection so the normalizer's transform pipeline runs.
`

	normalized := findChart(t, parseNormalized(t, content))
	unnormalized := findChart(t, mustParseUnnormalized(t, content))

	if len(unnormalized.Series) != 2 {
		t.Fatalf("fixture inválido: la ruta sin normalizar no dio 2 series, got %#v", unnormalized.Series)
	}

	if len(normalized.Series) != 2 {
		t.Fatalf("Series tras normalizar = %#v, want 2 series — el bloque data: anidado se aplanó/perdió", normalized.Series)
	}
	if len(normalized.Data) != 2 || len(normalized.Data[0]) != 2 || len(normalized.Data[1]) != 2 {
		t.Fatalf("Data tras normalizar = %#v, want 2 series de 2 valores cada una", normalized.Data)
	}
	if normalized.Data[0][0] != 28 || normalized.Data[0][1] != 35 {
		t.Errorf("Data[0] = %#v, want [28 35] (serie Revenue)", normalized.Data[0])
	}
	if normalized.Data[1][0] != 12 || normalized.Data[1][1] != 15 {
		t.Errorf("Data[1] = %#v, want [12 15] (serie Profit Margin)", normalized.Data[1])
	}
}

// TestChartFormatterRule_Apply_WellFormedComboDataIsNotChurned confirma
// no-churn a nivel de Apply(): un combo chart cuyo bloque data: ya está bien
// anidado no debe reescribirse byte por byte, igual que el caso ya cubierto
// de options: (TestChartFormatterRule_Apply_WellIndentedOptionsIsNotChurned).
func TestChartFormatterRule_Apply_WellFormedComboDataIsNotChurned(t *testing.T) {
	rule := enhancement.NewChartFormatterRule()

	input := `<<chart: combo>>
  data:
    labels: ["2022", "2023"]
    series:
      - name: "Revenue ($M)"
        type: "bar"
        values: [28, 35]
      - name: "Profit Margin (%)"
        type: "line"
        values: [12, 15]
        yAxisID: "y1"
  options:
    responsive: true
<</chart>>`

	got, err := rule.Apply(input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got != input {
		t.Errorf("Apply() reescribió un combo chart ya bien formado.\n--- got ---\n%s\n--- want (input sin cambios) ---\n%s", got, input)
	}
}

func mustParseUnnormalized(t *testing.T, content string) *ast.AST {
	t.Helper()
	p := parser.NewDocumentFlexParser(content, util.NewNoop())
	doc, diags := p.Parse()
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("parse error (unnormalized): %s", d.Message)
		}
	}
	return doc
}

// TestChartFormatterRule_Apply_WellIndentedOptionsIsNotChurned cubre el hueco
// de isSubProperty que needsFormatting tenía: un chart con options: YA bien
// anidado (a niveles que la tabla de 6 nombres de isSubProperty no conoce)
// podía seguir cayendo bajo el umbral de 0.7 y ser reescrito igual, aunque
// no fuera destructivo tras el fix del passthrough. Confirma no-churn:
// Apply() debe devolver el input byte-idéntico.
func TestChartFormatterRule_Apply_WellIndentedOptionsIsNotChurned(t *testing.T) {
	rule := enhancement.NewChartFormatterRule()

	input := `<<chart:bar>>
  title: Well Formed
  labels: ["A", "B"]
  datasets:
    data: [1, 2]
  options:
    responsive: true
    scales:
      y:
        beginAtZero: true
      x:
        display: false
>>`

	got, err := rule.Apply(input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got != input {
		t.Errorf("Apply() reescribió un chart ya bien formateado.\n--- got ---\n%s\n--- want (input sin cambios) ---\n%s", got, input)
	}
}
