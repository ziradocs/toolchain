// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// El despacho de elementos del dialecto strict tiene que coincidir con lo que
// cada ElementParser declara en su CanParse.
//
// Strict no usa GetDefaultRegistry: parseIndentedElements es una cadena if/else
// escrita a mano, así que durante un tiempo cada tag tuvo su propio HasPrefix
// duplicando —mal— la declaración del elemento. Las copias se separaron en las
// dos direcciones a la vez: strict aceptaba `<<quiz>>loquesea` y `<<grid>>xyz`,
// que el CanParse de esos elementos rechaza por exigir la línea exacta (el
// mismo documento era un quiz en strict y texto en flex); y rechazaba
// `<<map …>>` con atributos y la forma multilínea `<<chart`, que sus CanParse
// sí aceptan, mandándolos al catch-all como línea no reconocida.
//
// Este test fija las cuatro, que es lo que impide que la deriva se reabra: un
// HasPrefix nuevo en el despacho rompe alguna de las cuatro filas.
func TestStrictParser_DispatchMatchesElementCanParse(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []string
		// want es el tipo de nodo que el primer elemento del slide debe tener,
		// o "" si esa línea no debe producir ningún elemento.
		want ast.NodeType
	}{
		{
			name: "quiz exacto es un quiz",
			body: []string{"<<quiz>>", "question: Q", "options: [a, b]", "answer: 0", "<<end>>"},
			want: ast.NodeTypeQuiz,
		},
		{
			// El caso que motiva todo esto: en flex la misma línea es texto,
			// porque el registry pregunta por CanParse. Que strict la tomara
			// como un quiz hacía que el dialecto cambiara el significado.
			name: "quiz con basura pegada no es un quiz",
			body: []string{"<<quiz>>basura", "question: Q", "options: [a, b]", "<<end>>"},
			want: "",
		},
		{
			name: "poll con basura pegada no es un poll",
			body: []string{"<<poll>>basura", "question: Q", "options: [a, b]", "<<end>>"},
			want: "",
		},
		{
			name: "grid con basura pegada no es un grid",
			body: []string{"<<grid>>xyz", "::: col", "hola", ":::", "<<end>>"},
			want: "",
		},
		{
			// La deriva en la otra dirección: MapParser.CanParse acepta
			// `<<map` + `>>`, así que un mapa con atributos es sintaxis
			// declarada. El despacho pedía `<<map>>` pelado y lo descartaba.
			name: "map con atributos es un map",
			body: []string{`<<map lat="19.4" lng="-99.1" zoom="10">>`},
			want: ast.NodeTypeMap,
		},
		{
			name: "chart multilínea es un chart",
			body: []string{"<<chart", "type: bar", "labels: [a, b]", "data: [1, 2]", "<<end>>"},
			want: ast.NodeTypeChart,
		},
		{
			name: "chart inline es un chart",
			body: []string{"<<chart: bar>>", "labels: [a, b]", "data: [1, 2]", "<<end>>"},
			want: ast.NodeTypeChart,
		},
		// Los negativos de frontera de palabra. Sin ellos, "consultar el
		// CanParse" arregla la deriva de quiz/poll/grid y a la vez PROMUEVE la
		// laxitud de chart/map, que aceptaban cualquier prefijo: la suite
		// quedaba verde encima de una regresión. Un tag mal escrito que se
		// parsea como otro elemento se traga las líneas de abajo —el `title:`
		// de un slide strict terminaba adentro del mapa— y nunca se reporta.
		{name: "charts no es un chart", body: []string{"<<charts>>", "type: bar"}, want: ""},
		{name: "chartfoo no es un chart", body: []string{"<<chartfoo>>", "type: bar"}, want: ""},
		{name: "chart-de-cuentas no es un chart", body: []string{"<<chart-de-cuentas>>"}, want: ""},
		{name: "mapa no es un map", body: []string{"<<mapa>>"}, want: ""},
		{name: "maps no es un map", body: []string{"<<maps>>"}, want: ""},
		{name: "mapping con atributos no es un map", body: []string{`<<mapping x="1">>`}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := []string{"SLIDE content", `  title: "S"`}
			for _, l := range tc.body {
				lines = append(lines, "  "+l)
			}
			astNode, _ := NewStrictParser(strings.Join(lines, "\n"), util.NewNoop()).Parse()

			if len(astNode.ContentBlocks) != 1 {
				t.Fatalf("se esperaba 1 bloque, hay %d", len(astNode.ContentBlocks))
			}
			if tc.want == "" {
				// Basta con que NINGÚN elemento sea del tipo del tag escrito:
				// la línea puede caer en un TextElement o descartarse, las dos
				// cosas son aceptables; lo que no puede es haber sido tomada
				// como el elemento que su propio parser rechaza.
				for _, e := range astNode.ContentBlocks[0].Elements {
					switch e.GetType() {
					case ast.NodeTypeQuiz, ast.NodeTypePoll, ast.NodeTypeGrid,
						ast.NodeTypeChart, ast.NodeTypeMap:
						t.Fatalf("la línea produjo un %s; su CanParse la rechaza", e.GetType())
					}
				}
				return
			}

			got := ast.NodeType("")
			for _, e := range astNode.ContentBlocks[0].Elements {
				if e.GetType() == tc.want {
					got = tc.want
					break
				}
			}
			if got != tc.want {
				types := make([]string, 0, len(astNode.ContentBlocks[0].Elements))
				for _, e := range astNode.ContentBlocks[0].Elements {
					types = append(types, string(e.GetType()))
				}
				t.Fatalf("no se produjo un %s; elementos: %v", tc.want, types)
			}
		})
	}
}

// Un tag mal escrito no puede quedarse con las propiedades del slide.
//
// Este es el daño concreto de que `<<map` no tuviera frontera de palabra:
// `<<mapa>>` se parseaba como un mapa, el mapa consumía el `title:` de abajo
// como uno de sus atributos, y el slide terminaba sin título. Un typo de una
// letra borraba el título y no se reportaba en ningún lado.
func TestStrictParser_MistypedTagDoesNotSwallowSlideProperties(t *testing.T) {
	astNode, _ := NewStrictParser(strings.Join([]string{
		"SLIDE content",
		"  <<mapa>>",
		`  title: "Ventas por región"`,
		"  TEXT",
		"    Contenido real",
	}, "\n"), util.NewNoop()).Parse()

	if len(astNode.ContentBlocks) != 1 {
		t.Fatalf("se esperaba 1 bloque, hay %d", len(astNode.ContentBlocks))
	}
	if got := astNode.ContentBlocks[0].Title; got != "Ventas por región" {
		t.Errorf("Title = %q: el tag mal escrito se quedó con la propiedad del slide", got)
	}
}
