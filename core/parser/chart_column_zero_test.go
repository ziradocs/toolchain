// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// Issue #236 por la ruta donde el defecto era observable: con la normalización
// APAGADA. Por CLI el hueco quedaba enmascarado porque ChartFormatterRule
// reindenta el cuerpo del chart antes de que el parser lo vea, así que solo se
// reproducía vía API (SetNormalization(false)) o en un documento `mode:
// strict`, donde el normalizador no corre nunca.
//
// La forma es la de examples/use-cases/educational/machine_learning_intro.slidelang:
// propiedades y filas del array sin sangrar bajo el tag.
func TestParse_ChartColumnZeroArraysReachTheAST(t *testing.T) {
	content := strings.Join([]string{
		"---",
		"mode: flex",
		`title: "T"`,
		"---",
		"",
		"## Datos",
		"",
		"<<chart: radar>>",
		`title: "ML Impact"`,
		"data: [",
		`  ["Healthcare", 78, 95],`,
		`  ["Finance", 85, 92]`,
		"]",
		`series: ["Current", "Expected"]`,
		"<<end>>",
		"",
		"Prosa después del chart.",
	}, "\n")

	p := New(util.NewNoop())
	p.SetNormalization(false)
	astNode, diags := p.Parse(content, "test.slidelang")

	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("error de parseo: %s", d.String())
		}
	}

	var chart *ast.ChartElement
	proseFound := false
	for i := range astNode.ContentBlocks {
		for _, el := range astNode.ContentBlocks[i].Elements {
			switch e := el.(type) {
			case *ast.ChartElement:
				chart = e
			case *ast.TextElement:
				if strings.Contains(e.Content, "Prosa después") {
					proseFound = true
				}
			}
		}
	}

	if chart == nil {
		t.Fatal("no se produjo ningún ChartElement")
	}
	if len(chart.Data) != 2 {
		t.Errorf("Data = %v, se esperaban 2 filas — el array en columna 0 no llegó al AST", chart.Data)
	}
	if len(chart.Series) != 2 {
		t.Errorf("Series = %v, se esperaban 2", chart.Series)
	}
	if chart.Title != "ML Impact" {
		t.Errorf("Title = %q", chart.Title)
	}
	// El otro medio del contrato: arreglar la extracción no puede hacer que el
	// bloque se coma lo que sigue.
	if !proseFound {
		t.Error("la prosa posterior al chart se perdió")
	}
}
