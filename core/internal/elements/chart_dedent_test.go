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

// TestChartParser_RecognizedButUnhandledPropertyDoesNotCut cubre la primera
// regresión que introdujo el allowlist: `datasets:` es sintaxis de chart para
// el normalizador (isChartDataLine en
// internal/normalize/normalizer/rules/enhancement/chart_formatter.go, y
// chartProperties en internal/normalize/normalizer/detector.go), que la emite
// normalmente seguida de `data:`. El switch de Parse no la usa, así que caía
// en el default y cortaba el bloque ANTES de los datos: el chart quedaba
// vacío, disparaba CHART001 y el resto del bloque se procesaba como texto.
//
// Reconocida pero no manejada != desconocida: se consume y se sigue.
func TestChartParser_RecognizedButUnhandledPropertyDoesNotCut(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",            // 0
		"  datasets:",               // 1
		"  data: [10, 20, 30]",      // 2
		`  labels: ["A", "B", "C"]`, // 3
	}
	chart := assertChartConsumes(t, lines, 4)

	if len(chart.Data) == 0 {
		t.Error("Data vacío — el bloque se cortó en datasets: y el chart dispararía CHART001")
	}
	if len(chart.Labels) != 3 {
		t.Errorf("Labels = %v, want 3 — el bloque se cortó antes de labels:", chart.Labels)
	}
}

// TestChartParser_UnhandledPropertiesAfterData confirma que el vocabulario
// reconocido tampoco corta cuando aparece DESPUÉS de los datos, mezclado con
// las claves que el switch sí usa.
func TestChartParser_UnhandledPropertiesAfterData(t *testing.T) {
	lines := []string{
		"<<chart: line>>",
		"  data: [1, 2]",
		`  backgroundColor: "#16A085"`,
		"  fill: false",
		"  tension: 0.4",
		`  labels: ["A", "B"]`,
	}
	chart := assertChartConsumes(t, lines, 6)
	if len(chart.Labels) != 2 {
		t.Errorf("Labels = %v, want 2 — una propiedad reconocida cortó el bloque", chart.Labels)
	}
}

// TestChartParser_MarkdownLinkAfterChartSurvives cubre la segunda regresión:
// la excepción de continuación de array se evaluaba para CUALQUIER línea y
// aceptaba todo lo que empezara con "[", así que un enlace Markdown legítimo
// después de un chart cerrado por dedent volvía a consumirse y desaparecía —
// exactamente la pérdida silenciosa que este archivo cubre.
//
// Ahora la excepción solo aplica con un array realmente abierto
// (arrayDepth > 0). Acá el array de data: abre y cierra en su propia línea,
// así que el enlace es contenido de después del bloque.
func TestChartParser_MarkdownLinkAfterChartSurvives(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",                    // 0
		"  data: [10, 20]",                  // 1
		"",                                  // 2
		"[Ver fuente](https://example.com)", // 3
	}
	assertChartConsumes(t, lines, 3)
}

// TestChartParser_MarkdownLinkAfterMultiLineArraySurvives es el mismo cruce
// pero con la forma que sí deja restos para el loop: el array multi-línea con
// el corchete de cierre dedentado. Una vez que ese corchete cierra el array,
// el enlace ya no es continuación.
func TestChartParser_MarkdownLinkAfterMultiLineArraySurvives(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",                    // 0
		"  data: [",                         // 1
		"    [1820, 890],",                  // 2
		"    [1650, 780]",                   // 3
		"  ]",                               // 4
		"",                                  // 5
		"[Ver fuente](https://example.com)", // 6
	}
	assertChartConsumes(t, lines, 6)
}

// TestChartParser_BracketInLabelDoesNotOpenArray verifica que bracketDelta
// ignora los corchetes dentro de una cadena: una etiqueta con corchetes no
// debe dejar un array "abierto" y tragarse lo que sigue.
func TestChartParser_BracketInLabelDoesNotOpenArray(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",
		`  labels: ["Ventas [MXN]", "Costos [MXN]"]`,
		"",
		"[Ver fuente](https://example.com)",
	}
	assertChartConsumes(t, lines, 3)
}

// TestChartParser_UnclosedDataArrayDoesNotSwallowToEOF cubre la segunda
// regresión de la revisión: mientras arrayDepth > 0 se aceptaba CUALQUIER
// línea como continuación, sin validar su forma. Si al array de data: le
// falta el "]" de cierre, la prosa, @notes: y sus puntos desaparecían hasta
// EOF, un heading u otro elemento — la misma pérdida silenciosa que este
// archivo existe para cerrar, ahora disparada por un array roto en vez de un
// dedent.
func TestChartParser_UnclosedDataArrayDoesNotSwallowToEOF(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",                   // 0
		"  data: [",                        // 1  — nunca se cierra
		"    [1, 2],",                      // 2
		"",                                 // 3
		"**Prosa que debería sobrevivir**", // 4
		"",                                 // 5
		"@notes:",                          // 6
		"- esto no debería desaparecer",    // 7
	}
	chart := assertChartConsumes(t, lines, 4)
	if len(chart.Data) != 1 {
		t.Errorf("Data = %v, want 1 fila — la fila válida antes del corte debe conservarse", chart.Data)
	}
}

// TestChartParser_MalformedOptionsArrayDoesNotLeakIntoData es el mismo cruce
// que TestChartParser_MalformedOptionsDoesNotBreakChart en chart_options_test.go
// pero asertando ConsumedLines directamente: el "[" suelto dentro de un
// options: descartado no debe dejar arrayDepth en positivo para la propiedad
// que sigue.
func TestChartParser_MalformedOptionsArrayDoesNotLeakIntoData(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",         // 0
		"  options:",             // 1
		"    plugins: [unclosed", // 2
		"     : : broken",        // 3
		"  data: [1, 2]",         // 4
		"",                       // 5
		"**Prosa que sigue**",    // 6
	}
	chart := assertChartConsumes(t, lines, 6)
	if len(chart.Data) == 0 || len(chart.Data[0]) != 2 {
		t.Errorf("Data = %v, want [[1 2]] — el options: malformado se tragó data:", chart.Data)
	}
}

// TestChartParser_ColumnZeroSeriesArrayIsNotCutByDataRowShape cubre la
// tercera regresión de la revisión: la validación de continuación exigía
// forma de fila entre corchetes ("[...]") para CUALQUIER array abierto, pero
// parseMultiLineStringArray (series/labels/type) espera strings sueltos SIN
// envolver ("\"Revenue\","), no filas. Con el array en columna 0 — el mismo
// caso que motivó arrayDepth para data — el parser cortaba en la primera
// fila de series y perdía todo lo que seguía, incluido un data: posterior.
func TestChartParser_ColumnZeroSeriesArrayIsNotCutByDataRowShape(t *testing.T) {
	lines := []string{
		"<<chart: bar>>", // 0
		"series: [",      // 1
		`"Revenue",`,     // 2
		`"Cost"`,         // 3
		"]",              // 4
		"data: [1, 2]",   // 5
		"",               // 6
		"**prosa**",      // 7
	}
	chart := assertChartConsumes(t, lines, 7)
	// chart.Series se queda vacío incluso con este fix: es un gap PREEXISTENTE
	// y separado (parseMultiLineStringArray, igual que parseMultiLineArray,
	// devuelve 0 líneas consumidas cuando el array arranca en columna 0, así
	// que nunca llega a extraer los valores) — confirmado que main sin
	// parchear pierde el mismo contenido para "data:" en columna 0. Lo que
	// este test fija es el truncamiento: que "data:" no termine tratado como
	// texto suelto fuera del chart solo porque series: lo antecede sin
	// sangría.
	if len(chart.Data) == 0 {
		t.Errorf("Data = %v — el corte prematuro en series: se llevó data: por delante, tratándolo como texto", chart.Data)
	}
}

// TestChartParser_MarkdownLinkAfterUnclosedDataArraySurvives cubre la cuarta:
// dentro de un array de data: sin cerrar, un enlace Markdown también empieza
// con "[" y calificaba como continuación por esa sola razón — el enlace
// desaparecía igual que si el array nunca hubiera quedado abierto.
// isMarkdownLinkShaped lo distingue de una fila real ("[\"Q1\", 1],") por la
// secuencia "](" que ninguna fila de datos produce.
func TestChartParser_MarkdownLinkAfterUnclosedDataArraySurvives(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",                    // 0
		"  data: [",                         // 1 — nunca se cierra
		"    [1, 2],",                       // 2
		"[Ver fuente](https://example.com)", // 3
		"",                                  // 4
		"**prosa**",                         // 5
	}
	// El array solo tiene una fila antes del enlace (nunca cierra), así que
	// el bloque son las líneas 0-2; el enlace en la 3 debe sobrevivir.
	chart := assertChartConsumes(t, lines, 3)
	if len(chart.Data) != 1 {
		t.Errorf("Data = %v, want 1 fila", chart.Data)
	}
}

// TestChartParser_BackgroundColorArrayIsTracked cubre la primera regresión
// de la cuarta revisión: isArrayValuedKey solo listaba data/series/labels/
// type a mano, las únicas que el switch de arriba maneja directamente. Pero
// cualquier propiedad del vocabulario (chartPropertyKeys) puede escribirse
// como array multi-línea aunque el switch la deje en el default sin
// procesar — backgroundColor/borderColor son las de Chart.js. Sin tracking
// para ellas, "backgroundColor: [" cortaba el chart en la primera línea de
// colores y perdía cualquier data: posterior.
func TestChartParser_BackgroundColorArrayIsTracked(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",       // 0
		"  backgroundColor: [", // 1
		`    "#FF0000",`,       // 2
		`    "#00FF00"`,        // 3
		"  ]",                  // 4
		"  data: [1, 2]",       // 5
	}
	chart := assertChartConsumes(t, lines, 6)
	if len(chart.Data) == 0 {
		t.Errorf("Data = %v — backgroundColor: sin tracking se llevó data: por delante", chart.Data)
	}
}

// TestChartParser_DataRowWithEmbeddedLinkTextSurvives cubre la segunda: la
// validación anterior de "data" descartaba una fila legítima cuando su VALOR
// de texto contenía algo con forma de enlace ("[\"[Ver fuente](url)\", 5],"),
// porque buscaba la secuencia "](" con un substring plano, sin saber que
// estaba dentro de una cadena entre comillas. isDataArrayRow ahora escanea
// consciente de comillas (igual que bracketDelta), así que el contenido de
// una cadena no puede disparar el rechazo.
func TestChartParser_DataRowWithEmbeddedLinkTextSurvives(t *testing.T) {
	lines := []string{
		"<<chart: bar>>", // 0
		"data: [",        // 1
		`["[Ver fuente](https://example.com)", 5],`, // 2
		"[10, 20]",           // 3
		"]",                  // 4
		`labels: ["A", "B"]`, // 5
	}
	chart := assertChartConsumes(t, lines, 6)
	// chart.Data se queda vacío para las filas de este array: es el mismo
	// gap PREEXISTENTE de columna 0 que TestChartParser_
	// ColumnZeroSeriesArrayIsNotCutByDataRowShape ya documenta para series —
	// parseMultiLineArray devuelve 0 líneas consumidas ahí, así que nunca
	// llega a extraer los valores; nuestro tracking de arrayDepth solo evita
	// el corte prematuro, no sustituye la extracción. Lo que este test fija
	// es que la fila con el enlace embebido NO se rechaza como si fuera el
	// fin del bloque: labels:, que va DESPUÉS del array, tiene que
	// sobrevivir.
	if len(chart.Labels) != 2 {
		t.Errorf("Labels = %v, want 2 — la fila con el enlace embebido se rechazó como límite y se llevó labels: por delante", chart.Labels)
	}
}

// TestChartParser_ReferenceStyleLinkAfterUnclosedDataArraySurvives cubre la
// tercera: un enlace por referencia ("[texto][ref]") también empieza con
// "[", igual que uno inline, y tiene que rechazarse igual dentro de un array
// de data sin cerrar. isDataArrayRow lo hace por gramática (tras cerrar el
// primer corchete no queda fin de línea ni coma, sino otro "[...]"), no por
// buscar una forma de enlace específica.
func TestChartParser_ReferenceStyleLinkAfterUnclosedDataArraySurvives(t *testing.T) {
	lines := []string{
		"<<chart: bar>>",       // 0
		"  data: [",            // 1 — nunca se cierra
		"    [1, 2],",          // 2
		"[Ver fuente][fuente]", // 3
		"",                     // 4
		"**prosa**",            // 5
	}
	chart := assertChartConsumes(t, lines, 3)
	if len(chart.Data) != 1 {
		t.Errorf("Data = %v, want 1 fila", chart.Data)
	}
}
