// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// Los dos tests de este archivo arrancan de TEXTO DSL real, no de un AST
// construido a mano. Es deliberado: el hueco del issue #78 vivía justo entre
// "el parser produce el elemento" y "el formatter lo sabe emitir", y un AST
// armado a mano salta esa costura entera. Además ejercita el normalizer, que
// es donde apareció la segunda mitad del problema del chart.

// TestFormatDocument_Math_RoundTrips cubre el issue #78: un .doclang con math
// moría en `fmt` con "tipo de elemento no reconocido por el formatter de
// DocLang" porque formatDocumentElement no tenía case para *ast.MathElement.
//
// No alcanza con confiar en el corpus: el único fixture con math
// (examples/advanced_elements_test.doclang) también tiene un GRID, que el
// dialecto declara como no representable y aparece ANTES en el documento, así
// que el harness lo saltea antes de llegar al math.
func TestFormatDocument_Math_RoundTrips(t *testing.T) {
	const src = `---
title: Math
---

# Ecuaciones

Prosa antes.

<<math>>
E = mc^2
label: "eq:einstein"
<<end>>

Prosa después.

$$
x^2 + y^2 = z^2
$$
`

	doc, err := parseDocument(t, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}

	reparsed, err := parseDocument(t, out)
	if err != nil {
		t.Fatalf("el output no re-parsea: %v\n%s", err, out)
	}

	want, got := collectMath(normalizeForComparison(doc)), collectMath(normalizeForComparison(reparsed))
	if len(want) != 2 {
		t.Fatalf("el fixture debería producir 2 MathElement, produjo %d — ¿cambió MathParser?", len(want))
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("los MathElement no round-tripean:\nwant %+v\ngot  %+v\nformateado:\n%s", want, got, out)
	}

	// La forma canónica de fmt es el bloque <<math>>, también para un $$…$$:
	// las dos son sintaxis válida en flex, y canonicalizar es justamente lo
	// que hace fmt. Si esto cambiara, el segundo pase dejaría de ser idéntico
	// al primero.
	if n := strings.Count(out, "<<math>>"); n != 2 {
		t.Errorf("se esperaban 2 bloques <<math>> en la salida canónica, hay %d:\n%s", n, out)
	}

	// Idempotencia: `fmt` promete salida determinista, y el harness del corpus
	// solo compara ASTs tras UN pase — la estabilidad byte a byte entre dos
	// pases no la cubre nadie más.
	out2, err := FormatDocument(reparsed)
	if err != nil {
		t.Fatalf("FormatDocument (2º pase): %v", err)
	}
	if out != out2 {
		t.Errorf("fmt no es idempotente para math:\n--- pase 1 ---\n%s\n--- pase 2 ---\n%s", out, out2)
	}
}

// TestFormatDocument_ChartOptions_RoundTrips cubre la regresión que el issue
// #146 introdujo sin querer: al enseñarle al parser a capturar `options:`,
// ChartElement.Options dejó de estar siempre vacío — y formatChart devolvía
// UnsupportedElementError justo en ese caso, con el argumento (ya falso) de
// que el parser nunca lo poblaba. O sea que `fmt` empezó a morir en cualquier
// chart con options:, que en la práctica son casi todos.
//
// El bloque anidado que usa este test NO es decorativo: `scales.y.beginAtZero`
// tiene tres niveles y ninguno de sus nombres está en la tabla que usa
// ChartFormatterRule (el re-indentador del normalizer) para reconstruir la
// indentación de un chart. Con la salida sin indentar que emitía formatChart
// antes, esa regla aplanaba los tres niveles a uno.
func TestFormatDocument_ChartOptions_RoundTrips(t *testing.T) {
	const src = `---
title: Chart
---

# Ventas

<<chart: bar>>
  data: [
    ["Q1", 120]
    ["Q2", 250]
  ]
  options:
    responsive: true
    scales:
      y:
        beginAtZero: true
    plugins:
      title:
        display: true
        text: "Ventas por trimestre"
<</chart>>
`

	doc, err := parseDocument(t, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := collectCharts(normalizeForComparison(doc))
	if len(want) != 1 {
		t.Fatalf("el fixture debería producir 1 ChartElement, produjo %d", len(want))
	}
	if len(want[0].Options) == 0 {
		t.Fatal("el parser no pobló Options — este test dejaría de probar lo que dice probar")
	}

	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}

	reparsed, err := parseDocument(t, out)
	if err != nil {
		t.Fatalf("el output no re-parsea: %v\n%s", err, out)
	}

	got := collectCharts(normalizeForComparison(reparsed))
	if !reflect.DeepEqual(want, got) {
		t.Errorf("el chart no round-trippea:\nwant %+v\ngot  %+v\nformateado:\n%s", want[0], got[0], out)
	}

	out2, err := FormatDocument(reparsed)
	if err != nil {
		t.Fatalf("FormatDocument (2º pase): %v", err)
	}
	if out != out2 {
		t.Errorf("fmt no es idempotente para un chart con options:\n--- pase 1 ---\n%s\n--- pase 2 ---\n%s", out, out2)
	}
}

func collectMath(doc *ast.AST) []*ast.MathElement {
	var out []*ast.MathElement
	for _, b := range doc.ContentBlocks {
		for _, el := range b.Elements {
			if m, ok := el.(*ast.MathElement); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

func collectCharts(doc *ast.AST) []*ast.ChartElement {
	var out []*ast.ChartElement
	for _, b := range doc.ContentBlocks {
		for _, el := range b.Elements {
			if c, ok := el.(*ast.ChartElement); ok {
				out = append(out, c)
			}
		}
	}
	return out
}
