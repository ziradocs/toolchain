// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import "testing"

// consumedFor devuelve cuántas líneas consumió el chart que arranca en la
// línea 0 de lines. Es lo único que decide qué sobrevive en el documento:
// el llamador salta exactamente esas líneas, así que todo lo que el chart
// escanee de más DESAPARECE sin diagnóstico.
func consumedFor(t *testing.T, lines []string) int {
	t.Helper()
	result := (&ChartParser{}).Parse(&ParseContext{Lines: lines}, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	return result.ConsumedLines
}

// Issue #237: un payload JSON cuyas llaves nunca balancean seguía escaneando
// hasta el primer límite duro, borrando todo lo que hubiera en medio. Acá el
// límite duro es el "## Next slide" del final, así que la prosa y la viñeta
// intermedias se perdían enteras, con CHART002 ("el JSON es inválido") como
// única señal — que no dice nada de contenido borrado.
func TestChartParser_UnbalancedJSONDoesNotSwallowFollowingContent(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",                      // 0
		"{",                                   // 1
		`  "type": "bar",`,                    // 2
		`  "data": { "labels": ["Q1","Q2"] }`, // 3  (falta el "}" de cierre)
		"",                                    // 4
		"This prose must survive.",            // 5
		"",                                    // 6
		"- bullet after",                      // 7
		"",                                    // 8
		"## Next slide",                       // 9
	}

	consumed := consumedFor(t, lines)

	if consumed > 5 {
		t.Errorf("ConsumedLines = %d, want <= 5 — el chart se tragó %q y lo que sigue; "+
			"un JSON que no balancea tiene que cerrar en la primera línea que no puede ser JSON",
			consumed, lines[5])
	}
}

// Misma pérdida, disparador distinto y peor alcance: UNA comilla sin cerrar
// deja inString=true para el resto del bloque, y con eso se apagan el chequeo
// de límite y el conteo de llaves — el chart se tragaba el documento entero
// hasta EOF, incluidos los slides siguientes.
//
// El escaneo NO cambió para arreglarlo: sigue arrastrando inString de línea
// en línea, porque de eso depende que un valor JSON que abarca varias líneas
// se reconozca (lo fija TestChartParser_MultiLineStringValueSurvivesTheTruncation,
// más abajo). Lo que lo cierra es el recorte del fallback, que reexamina las
// líneas por su forma cuando ya se sabe que el payload no balanceó.
func TestChartParser_UnterminatedStringDoesNotSwallowToEOF(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",                 // 0
		"{",                              // 1
		`  "type": "bar,`,                // 2  (comilla sin cerrar)
		`  "data": { "labels": ["Q1"] }`, // 3
		"}",                              // 4
		"<<end>>",                        // 5
		"",                               // 6
		"This prose must survive.",       // 7
		"",                               // 8
		"## Next slide",                  // 9
		"Still here.",                    // 10
	}

	consumed := consumedFor(t, lines)

	if consumed > 6 {
		t.Errorf("ConsumedLines = %d, want <= 6 (hasta el <<end>> inclusive) — "+
			"una comilla sin cerrar no puede tragarse el resto del documento", consumed)
	}
}

// Un comentario dentro del JSON no corta el bloque. ChartJSONRule
// (internal/normalize) los limpia antes del parser en el camino de la CLI,
// pero no en el de la API de Go, y ahí un "//" que cortara dejaría el
// comentario como texto suelto en la diapositiva.
func TestChartParser_JSONCommentIsNotABoundary(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",                 // 0
		"{",                              // 1
		"  // el tipo va primero",        // 2
		`  "type": "bar",`,               // 3
		`  "data": { "labels": ["Q1"] }`, // 4  (nunca balancea)
		"Prose after.",                   // 5
	}

	if consumed := consumedFor(t, lines); consumed != 5 {
		t.Errorf("ConsumedLines = %d, want 5 — el comentario es parte del payload, "+
			"y el corte va en %q", consumed, lines[5])
	}
}

// Comportamiento elegido, no accidental: si el payload arranca indentado, una
// línea que dedenta por debajo de ese nivel cierra el bloque aunque tenga
// forma de JSON. Es la misma convención que el issue #234 fijó para el loop
// de propiedades. Un payload que arranca en columna 0 no la aplica (ahí la
// sangría no distingue nada), igual que allá.
func TestChartParser_JSONDedentClosesBlock(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",     // 0
		"  {",                // 1  (payload indentado)
		`    "type": "bar",`, // 2
		`"data": [1, 2]`,     // 3  dedenta a columna 0: ya salió
		"Prose after.",       // 4
	}

	if consumed := consumedFor(t, lines); consumed != 3 {
		t.Errorf("ConsumedLines = %d, want 3 — el dedent cierra el bloque antes que "+
			"cualquier heurística de forma", consumed)
	}
}

// El guard de forma valida TOKENS COMPLETOS, no primeros caracteres. Su
// primera versión miraba solo el carácter inicial y por eso seguía tragándose
// Markdown corriente: los tres casos de acá pasaban por empezar con dígito,
// con "[" y con comilla respectivamente.
func TestChartParser_BrokenJSONDoesNotSwallowOrdinaryMarkdown(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"lista numerada", "1. Primer paso"},
		{"enlace Markdown", "[Más contexto](https://example.com)"},
		{"párrafo entrecomillado", `"Una cita que abre con comillas"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := []string{
				"<<chart: bar>>",   // 0
				"{",                // 1
				`  "type": "bar",`, // 2  (nunca balancea)
				"",                 // 3
				tt.line,            // 4
			}

			if consumed := consumedFor(t, lines); consumed > 4 {
				t.Errorf("ConsumedLines = %d, want <= 4 — el chart se tragó %q", consumed, tt.line)
			}
		})
	}
}

// Lo que el guard NO puede rechazar sin romper JSON legítimo: dentro de un
// array abierto, un escalar suelto es un elemento de verdad — el último de un
// array multi-línea va sin coma. Ese contexto es justamente lo que distingue
// a "B" de un párrafo entrecomillado suelto, que es el caso de arriba.
func TestChartParser_LastArrayElementWithoutCommaStaysInThePayload(t *testing.T) {
	lines := []string{
		"<<chart: bar>>", // 0
		"{",              // 1
		`  "labels": [`,  // 2  abre un array que nunca se cierra
		`    "A",`,       // 3
		`    "B"`,        // 4  sin coma: sigue siendo del array
		"",               // 5
		"Prosa después.", // 6
	}

	consumed := consumedFor(t, lines)

	if consumed < 5 {
		t.Errorf("ConsumedLines = %d, want >= 5 — %q es un elemento del array, no prosa", consumed, lines[4])
	}
	if consumed > 6 {
		t.Errorf("ConsumedLines = %d, want <= 6 — %q sí es prosa", consumed, lines[6])
	}
}

func TestIsJSONPayloadLine(t *testing.T) {
	tests := []struct {
		line        string
		insideArray bool
		want        bool
	}{
		// Estructura y pares clave/valor.
		{"{", false, true},
		{"},", false, true},
		{"}]", false, true},
		{`"type": "bar",`, false, true},
		{`"data": {`, false, true},
		{`"labels": [`, false, true},
		{`["Q1", 1],`, false, true},
		{`{"x": 10, "y": 20},`, false, true},
		{"// un comentario", false, true},

		// Escalares: solo dentro de un array.
		{`"B"`, true, true},
		{"42,", true, true},
		{"true", true, true},
		{"null,", true, true},
		{`"B"`, false, false},
		{"42,", false, false},

		// Prosa que la versión por primer carácter dejaba pasar.
		{"1. Primer paso", false, false},
		{"1. Primer paso", true, false},
		{"[Más contexto](https://example.com)", false, false},
		{"[Más contexto](https://example.com)", true, false},
		{`"Una cita que abre con comillas"`, false, false},
		{"Prosa normal.", false, false},
		{"- viñeta", false, false},
		{"type: bar", false, false}, // YAML, degradación documentada
	}

	for _, tt := range tests {
		if got := isJSONPayloadLine(tt.line, tt.insideArray); got != tt.want {
			t.Errorf("isJSONPayloadLine(%q, insideArray=%v) = %v, want %v",
				tt.line, tt.insideArray, got, tt.want)
		}
	}
}
