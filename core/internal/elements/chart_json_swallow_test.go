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
// dejaba inString=true para el resto del bloque, y con eso se apagaban el
// chequeo de límite y el conteo de llaves — el chart se tragaba el documento
// entero hasta EOF, incluidos los slides siguientes. inString/escaped ahora
// se reinician en cada línea (JSON no admite saltos de línea literales
// dentro de un string, así que para un payload válido es un no-op).
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
