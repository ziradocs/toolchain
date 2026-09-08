// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import "testing"

// La frontera de un tag inline tiene TRES mitades, y cada ronda de arreglos
// cerró una y declaró cerrada la clase:
//
//  1. el prefijo, con frontera de palabra (`<<charts>>` no es un chart);
//  2. el terminador, que la forma inline tiene que traer (`<<chart: bar` no lo
//     es);
//  3. y que ese terminador sea el ÚNICO — `<<chart: bar>>basura>>` termina en
//     ">>" y pasaba las dos anteriores.
//
// Esta tabla existe para que la clase se pruebe entera y de una sola vez, sobre
// TODOS los tags que la tienen, en los dos dialectos. Un tag nuevo con
// atributos se agrega acá; si no aparece en esta tabla, no está cerrado.
func TestInlineTags_CloseExactlyOnce(t *testing.T) {
	registry := GetDefaultRegistry()

	// parserFor devuelve el primer ElementParser del registry que acepta la
	// línea, o nil. Es el mismo camino que usa flex, y el que strict consulta
	// vía canParseStrict, así que medir acá cubre los dos dialectos siempre que
	// el despacho no invente su propia sintaxis (eso lo fija
	// parser/strict_dispatch_test.go).
	parserFor := func(line, mode string) ElementParser {
		for _, p := range registry.parsers {
			if p.CanParse(line, mode) {
				return p
			}
		}
		return nil
	}

	for _, tc := range []struct {
		line string
		want bool
	}{
		// Formas válidas: abren y cierran una vez.
		{`<<chart: bar>>`, true},
		{`<<chart`, true}, // abridor multilínea, cerrado por <<end>>
		{`<<map>>`, true},
		{`<<map lat="19.4" lng="-99.1">>`, true},
		{`<<video src="a.mp4">>`, true},
		{`<<audio src="a.mp3">>`, true},
		{`<<mermaid>>`, true},
		{`<<plantuml>>`, true},
		{`<<math>>`, true},

		// Un segundo ">>" en la misma línea. Todos terminan en ">>", así que
		// todos pasaban el chequeo de sufijo.
		{`<<chart: bar>>basura>>`, false},
		{`<<map lat="19.4">>basura>>`, false},
		{`<<video src="a.mp4">>basura>>`, false},
		{`<<audio src="a.mp3">>basura>>`, false},

		// Sin terminador.
		{`<<chart: bar`, false},
		{`<<map lat="19.4"`, false},
		{`<<video src="a.mp4"`, false},

		// Basura pegada a un tag bien formado, que es lo que el issue #289
		// dejaba abierto para los tres tags sin atributos.
		{`<<chart: bar>>basura`, false},
		{`<<map>>basura`, false},
		{`<<video src="a.mp4">>basura`, false},
		{`<<mermaid>>basura`, false},
		{`<<plantuml>>basura`, false},
		{`<<math>>basura`, false},

		// Frontera de palabra, que ya estaba cerrada y no puede reabrirse.
		{`<<charts>>`, false},
		{`<<chartfoo>>`, false},
		{`<<mapa>>`, false},
		{`<<maps>>`, false},
		{`<<mermaidfoo>>`, false},
	} {
		for _, mode := range []string{"strict", "flex"} {
			t.Run(mode+" "+tc.line, func(t *testing.T) {
				got := parserFor(tc.line, mode)
				// TextParser es el catch-all del registry: que una línea caiga
				// ahí es exactamente "no es un elemento embebido".
				_, isText := got.(*TextParser)
				isEmbedded := got != nil && !isText
				if isEmbedded != tc.want {
					t.Errorf("%q en %s → parser %T; se esperaba que %s fuera un elemento embebido",
						tc.line, mode, got, map[bool]string{true: "SÍ", false: "NO"}[tc.want])
				}
			})
		}
	}
}
