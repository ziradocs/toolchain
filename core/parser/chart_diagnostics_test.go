// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/util"
)

// CHART005 decide qué es una llave de nivel superior del chart comparando
// contra la sangría de la primera línea del bloque (baseIndent, en
// internal/elements/chart.go). Este test fija la interacción con el otro bug
// que vive en ese mismo loop y que todavía NO está arreglado: un chart cerrado
// por dedent, sin `<<end>>`, sigue escaneando las líneas que le siguen (ver el
// TODO ahí; el corpus pierde párrafos y bloques @notes por eso).
//
// La pregunta que este test contesta: ¿ese sobre-escaneo del PRIMER chart
// puede dejar mal calibrado el baseIndent del SEGUNDO y hacer que su llave
// desconocida se cuele sin aviso? No, porque baseIndent es local a cada
// llamada a ChartParser.Parse. Vale la pena tenerlo fijado: es justo la clase
// de acoplamiento que aparecería al tocar la terminación por dedent, y el
// síntoma sería un CHART005 que deja de avisar en silencio.
func TestChartDiagnostics_UnknownKeyAfterDedentClosedChart(t *testing.T) {
	src := `---
title: T
mode: flex
---

# Primera

<<chart: doughnut>>
  data: [30, 25]
  labels: ["A", "B"]

Prosa después de un chart cerrado por dedent, sin <<end>>.

# Segunda

<<chart: bar>>
  labels: ["Q1"]
  datasets:
    data: [85]
<<end>>
`

	_, diags := New(util.NewNoop()).ParseDocument(src, "d.doclang")

	var found bool
	for _, d := range diags {
		if d.RuleID == "CHART005" && strings.Contains(d.Message, "datasets") {
			found = true
		}
	}
	if !found {
		t.Errorf("CHART005 no avisó de `datasets:` en el segundo chart.\n"+
			"Si el primer chart (cerrado por dedent) dejó de calibrarse solo, el baseIndent del segundo quedó mal y su "+
			"llave desconocida se está tragando en silencio.\ndiagnósticos: %v", diags)
	}
}
