// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// La frontera de tag se prueba en los dos dialectos porque son dos caminos
// distintos al mismo CanParse: strict lo consulta desde una cadena if/else
// escrita a mano (ver strict_dispatch_test.go) y flex desde el registry de
// elementos, después de que corrió el normalizador. Un negativo verde en
// strict no dice nada de flex.
//
// Lo que se fija acá es el terminador: `<<chart` solo en su línea es un
// abridor multilínea válido, pero la forma inline tiene que cerrar con `>>` en
// la misma línea. Sin esa mitad, un tag truncado se llevaba por delante el
// contenido de abajo.
func TestFlexParser_TruncatedChartTagDoesNotSwallowContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		tag  string
	}{
		{name: "sin terminador", tag: "<<chart: bar"},
		{name: "con basura pegada", tag: "<<chart: bar>>basura"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			astNode, _ := parseFlexBody(t,
				"# Deck", "",
				"## Ventas por región",
				tc.tag,
				"Contenido real que sigue abajo.",
			)

			if len(astNode.ContentBlocks) < 2 {
				t.Fatalf("se esperaban al menos 2 bloques, hay %d", len(astNode.ContentBlocks))
			}
			block := astNode.ContentBlocks[1]

			if got := block.Title; got != "Ventas por región" {
				t.Errorf("Title = %q: el tag truncado se quedó con el título del slide", got)
			}
			for _, e := range block.Elements {
				if e.GetType() == ast.NodeTypeChart {
					t.Fatalf("la línea %q produjo un chart; su CanParse la rechaza", tc.tag)
				}
			}

			var texto strings.Builder
			for _, e := range block.Elements {
				if txt, ok := e.(*ast.TextElement); ok {
					texto.WriteString(txt.Content)
					texto.WriteString("\n")
				}
			}
			if !strings.Contains(texto.String(), "Contenido real que sigue abajo.") {
				t.Errorf("el contenido de abajo no sobrevivió; texto del slide: %q", texto.String())
			}
		})
	}
}

// El abridor multilínea sigue siendo válido: la regla nueva no puede exigirle
// el `>>` que por definición no lleva.
//
// No es el único que cazaría esa mutación —"simplificar" CanParse a la forma
// de map/media también rompe TestChartParser_CanParse (strict y flex) y la
// fila `chart multilínea es un chart` del despacho strict—, así que este caso
// no es lo que separa el rojo del verde. Vale por dónde está: en el mismo
// archivo que los negativos, para que quien los lea vea qué NO se está
// prohibiendo.
func TestFlexParser_MultilineChartOpenerStillParses(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"# Deck", "",
		"## Ventas",
		"<<chart",
		"type: bar",
		"labels: [a, b]",
		"data: [1, 2]",
		"<<end>>",
	)

	if len(astNode.ContentBlocks) < 2 {
		t.Fatalf("se esperaban al menos 2 bloques, hay %d", len(astNode.ContentBlocks))
	}
	for _, e := range astNode.ContentBlocks[1].Elements {
		if e.GetType() == ast.NodeTypeChart {
			return
		}
	}
	t.Fatalf("el abridor multilínea no produjo un chart")
}
