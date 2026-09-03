// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// ChartParser maneja el parsing de gráficos Chart.js
type ChartParser struct{}

// CanParse determina si puede parsear una línea como Chart
func (p *ChartParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	switch mode {
	case "strict":
		// Acepta tanto <<chart: (inline) como <<chart (multilínea)
		return strings.HasPrefix(trimmed, "<<chart:") || strings.HasPrefix(trimmed, "<<chart")
	case "flex":
		// Acepta tanto <<chart: (inline) como <<chart (multilínea)
		return strings.HasPrefix(trimmed, "<<chart:") || strings.HasPrefix(trimmed, "<<chart")
	}

	return false
}

// Parse parsea un elemento Chart
func (p *ChartParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{Error: nil}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	// Extraer tipo y atributos: "<<chart: bar width="1200" height="600">>"
	chartType := "bar"
	// 0 = el autor no declaró la dimensión. NO se hornea acá el 800x600 del
	// renderer: el AST es el contrato público (schema/ast.schema.json,
	// ast-types), y meterle el default de UN consumidor borra la única
	// información que distingue "el autor pidió 800" de "el autor no dijo
	// nada" — que es justo lo que `doclang fmt` necesitaba para no escribir
	// `width="800" height="600"` en documentos que nunca los declararon.
	// Mismo criterio que `zoom` en map.go, que ya usaba el 0, y que
	// PageConfig.Size/PageMargins en ast/nodes.go, que guardan el texto crudo
	// del autor en vez de resolverlo.
	//
	// Quien renderiza aplica su propio default: renderer.ChartDimensions y el
	// bloque de mapas de renderer/html.go arrancan en 800x600 y solo pisan si
	// el campo es > 0.
	width := 0
	height := 0

	// unknownAttrs junta los atributos de la línea de apertura que el chart no
	// conoce, para reportarlos como CHART005 más abajo. Solo se leen `width` y
	// `height`: cualquier otro par `k="v"` se ignoraba sin dejar rastro, y así
	// shipeó la plantilla `report` de `doclang init` con un
	// `<<chart:bar title="...">>` cuyo título no llegaba nunca al AST ni al
	// render. `title` va como llave del cuerpo (`title:`), no como atributo.
	var unknownAttrs []string

	// SplitN y no Split: con Split, un valor que contenga ':' (p. ej.
	// `title="Ventas: Q4"`) partía la línea en más de dos pedazos y attrStr
	// quedaba truncado en el primer ':'.
	if strings.Contains(line, ":") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) > 1 {
			attrStr := strings.TrimSpace(parts[1])
			attrStr = strings.TrimSuffix(attrStr, ">>")

			// Extraer tipo (primera palabra)
			tokens := strings.Fields(attrStr)
			if len(tokens) > 0 {
				chartType = tokens[0]
			}

			// Extraer width y height si están presentes
			if w := extractAttribute(attrStr, "width"); w != "" {
				if val, err := strconv.Atoi(w); err == nil && val > 0 {
					width = val
				}
			}
			if h := extractAttribute(attrStr, "height"); h != "" {
				if val, err := strconv.Atoi(h); err == nil && val > 0 {
					height = val
				}
			}

			unknownAttrs = unknownOpenerAttributes(attrStr, "width", "height")
		}
	}
	chart := ast.NewChartElement(pos, chartType)
	chart.Width = width
	chart.Height = height

	// diags se arma ACÁ, no en cada return, y de eso depende que CHART005
	// llegue siempre. Parse tiene TRES salidas —JSON directo, YAML de combo, y
	// el loop de propiedades— y el aviso de atributos desconocidos vivía
	// duplicado en dos de ellas: la de combo devolvía un ParseResult sin
	// Diagnostics, así que un `<<chart: combo title="...">>` volvía a perder
	// el título en silencio, que es exactamente el defecto que CHART005
	// existe para señalar (hallazgo de code review del PR #232). Declararlo
	// una sola vez apenas parseada la apertura hace que agregar una cuarta
	// salida no pueda repetir el olvido: el dato ya está en la variable que
	// todas devuelven.
	var diags []diagnostics.Diagnostic
	if len(unknownAttrs) > 0 {
		diags = append(diags, diagnostics.NewWarning(
			fmt.Sprintf("Atributo(s) no reconocido(s) en la apertura del chart (%s); solo se aceptan 'width' y 'height' — ignorado(s). El título va como llave del cuerpo: 'title:'",
				strings.Join(unknownAttrs, ", ")),
			pos, "chart-parser").WithRuleID("CHART005"))
	}
	consumedLines := 1 // skip <<chart:>> line
	indentDetector := NewAutoDetectIndentation()

	// Detectar si el siguiente contenido es JSON
	if startIndex+1 < len(ctx.Lines) {
		nextLine := strings.TrimSpace(ctx.Lines[startIndex+1])
		if strings.HasPrefix(nextLine, "{") {
			// Es JSON directo, parsearlo como tal
			jsonContent, jsonLines := p.parseJSONBlock(ctx.Lines, startIndex+1)
			if jsonContent != "" {
				consumedLines += jsonLines

				if json.Valid([]byte(jsonContent)) {
					chart.RawJSON = json.RawMessage(jsonContent)
					chart.IsJSONMode = true
				} else {
					// JSON inválido: no se activa IsJSONMode y el chart queda sin datos
					// (CHART001 lo reportará como "sin datos" de forma engañosa), pero
					// el bloque completo (incluyendo su <</chart>> de cierre, ya contado
					// en jsonLines) se consume igual para no dejar el cierre como texto
					// suelto ni reprocesarlo como propiedades data:/series:/etc. Este
					// diagnóstico Warning no aborta el build (a diferencia de Error) y
					// es la única señal específica de "el JSON estaba roto".
					diags = append(diags, diagnostics.NewWarning(
						"El JSON del chart es inválido y fue ignorado; el chart quedará sin datos",
						pos, "chart-parser").WithRuleID("CHART002"))
				}
				return &ParseResult{
					Element:       chart,
					ConsumedLines: consumedLines,
					Error:         nil,
					Diagnostics:   diags,
				}
			}
		}
	}

	// Detectar si hay una estructura YAML compleja (para combo charts)
	if chartType == "combo" && startIndex+1 < len(ctx.Lines) {
		yamlContent, yamlLines := p.parseYAMLBlock(ctx.Lines, startIndex+1)
		if yamlContent != "" {
			if ok, unknownKeys := p.parseComboChartYAML(chart, yamlContent); ok {
				consumedLines += yamlLines
				// Mismo criterio que unknownKeys en el loop de propiedades de
				// abajo (ver CHART005 más abajo): ChartConfig deserializa con
				// yaml.v3, que ignora en silencio cualquier llave del mapeo que
				// no tenga campo correspondiente en el struct. Un
				// `<<chart: combo>>` con un `datasets:` de nivel superior (el
				// mismo typo que la plantilla `report` de `doclang init` traía
				// en la forma plana) pasaba por acá sin ningún aviso, aunque el
				// resto del chart tuviera datos válidos.
				if len(unknownKeys) > 0 {
					diags = append(diags, diagnostics.NewWarning(
						fmt.Sprintf("Llave(s) no reconocida(s) en el bloque chart (%s); se esperaba 'data'/'options' — ignorada(s)",
							strings.Join(unknownKeys, ", ")),
						pos, "chart-parser").WithRuleID("CHART005"))
				}
				return &ParseResult{
					Element:       chart,
					ConsumedLines: consumedLines,
					Diagnostics:   diags,
					Error:         nil,
				}
			}
		}
	}

	// Parsear propiedades del chart.
	//
	// El loop lleva etiqueta porque los cortes de abajo viven dentro de un
	// switch: un "break" pelado ahí rompe el SWITCH, no el for, y la
	// ejecución cae en el consumedLines++ del final — que es justo el bug
	// que esto arregla. Mismo motivo por el que map.go etiqueta su
	// parseLoop.
	// arrayDepth lleva la cuenta de corchetes sin cerrar que quedaron
	// abiertos por un array multi-línea; ver el bloque que lo consulta.
	// openArrayKey recuerda QUÉ propiedad lo abrió (data/series/labels/type)
	// para poder validar la forma de las líneas de continuación según el
	// tipo: data usa filas entre corchetes ("["Q1", 1],"), pero
	// series/labels/type aceptan strings sueltos sin envolver
	// (parseMultiLineStringArray no los envuelve en "[...]" — ver esa
	// función más abajo), así que exigirles el prefijo "[" cortaba el
	// bloque de más en cuanto el array vivía en columna 0 (el mismo caso que
	// motivó arrayDepth para data).
	arrayDepth := 0
	openArrayKey := ""
	// bodyIndent fija el nivel de sangría del cuerpo del chart a partir de
	// su PRIMERA línea de contenido. Cumple DOS roles:
	//
	//   - Un dedent por DEBAJO de ese nivel cierra el bloque
	//     INCONDICIONALMENTE — antes de consultar isArrayContinuationForKey
	//     o el allowlist. Es la convención de cierre que este archivo entero
	//     existe para respetar (spec/language-specification.md:75).
	//   - Una línea MÁS profunda que ese nivel (y fuera de un array
	//     abierto — ver el chequeo de arrayDepth más abajo) es contenido de
	//     la llave que la abrió, no una propiedad nueva del chart: se
	//     consume y se ignora sin pasar por el switch. Sin este segundo rol,
	//     el `data:` anidado bajo un `datasets:` inventado se leía como si
	//     fuera EL `data:` de nivel superior — el defecto original que
	//     motivó esta variable en el PR #232, antes de que este archivo
	//     tuviera bodyIndent/arrayDepth/el allowlist de abajo.
	//
	// Ambos roles se saltan a propósito cuando bodyIndent == 0 (chart con
	// propiedades en columna 0 desde la primera línea, como
	// examples/use-cases/educational/machine_learning_intro.slidelang): ahí
	// la sangría no sirve para distinguir nada porque TODO el cuerpo vive en
	// columna 0 — es el tratamiento heredado que ya existía antes de este
	// mecanismo, y sigue intacto.
	bodyIndent := -1
	var unknownKeys []string
chartLoop:
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]
		trimmedLine := strings.TrimSpace(line)
		lineStart := i

		// Check for closing tag FIRST
		if trimmedLine == "<<end>>" {
			consumedLines++
			break
		}

		// Cualquier otro límite de contenido (separador de slide, otro
		// elemento, heading) — ver isChartContentBoundary. Se pasa la línea
		// cruda (line, no trimmedLine): el chequeo de límite strict necesita
		// la sangría para no confundirse con contenido indentado.
		if isChartContentBoundary(line) {
			break
		}

		// Skip empty lines
		if trimmedLine == "" {
			consumedLines++
			continue
		}

		// bodyIndent se fija UNA sola vez, con la sangría de la primera
		// línea de contenido del chart. De ahí en más, cualquier línea que
		// dedente por debajo de ese nivel cierra el bloque de inmediato —
		// SIN mirar si tiene forma de propiedad reconocida
		// (isChartPropertyKey) ni de continuación de array
		// (isArrayContinuationForKey). El dedent es más fuerte que
		// cualquiera de los dos: un chart con cuerpo sangrado que vuelve a
		// columna 0 ya salió del bloque sin importar que esa línea diga
		// "title: ..." (parece propiedad) o sea un string completo entre
		// comillas (parece continuación de un array de labels/series sin
		// cerrar) — de otro modo cualquiera de las dos listas podía
		// "reconocer" contenido que el dedent ya había dejado afuera, y
		// tragárselo.
		currentIndent := CalculateIndentLevel(line)
		if bodyIndent == -1 {
			bodyIndent = currentIndent
		} else if bodyIndent > 0 && currentIndent < bodyIndent {
			break chartLoop
		}

		// Dentro de un array multi-línea que su sub-parser dejó abierto,
		// toda línea es contenido del array, no el principio de lo que
		// sigue. parseMultiLineArray corta por sangría (ShouldProcessLine) y
		// eso lo deja corto en dos formas que el corpus sí escribe: el
		// corchete de cierre dedentado respecto a sus filas (el corte
		// dispara ANTES de su chequeo de "]"), y el array entero en columna
		// 0 (ExpectedIndent == -1 con sangría 0 devuelve false en la PRIMERA
		// fila, así que consume 0 líneas y caen todas acá).
		//
		// Se decide por PROFUNDIDAD DE CORCHETES, no por la forma de la
		// línea. Una heurística de "empieza con [" tragaba un enlace
		// Markdown legítimo después de un chart cerrado por dedent
		// ("[Ver fuente](https://example.com)"), que es exactamente la
		// pérdida silenciosa que este allowlist existe para cerrar. Con
		// arrayDepth la excepción solo aplica cuando hay un array realmente
		// abierto.
		//
		// Los límites duros (<<end>>, "---", "<<", headings) se chequean
		// ARRIBA de esto a propósito: un array sin cerrar no puede tragarse
		// el resto del documento. Este chequeo va ANTES del de "más profundo
		// que bodyIndent" de abajo: una continuación de array casi siempre
		// vive más indentada que la propiedad que la abrió, así que sin este
		// orden caería en el chequeo de nesting genérico sin decrementar
		// arrayDepth.
		if arrayDepth > 0 {
			// Un array sin cerrar (falta el "]") no puede tragarse el resto
			// del documento solo porque arrayDepth siga en positivo: eso
			// reabre la misma pérdida silenciosa que este archivo cubre,
			// nada más que con un array roto en vez de un dedent. Antes de
			// aceptar la línea como continuación hay que verificar que TIENE
			// FORMA de continuación de array PARA LA PROPIEDAD que lo abrió
			// (isArrayContinuationForKey): un enlace Markdown
			// ("[Ver fuente](https://example.com)") también empieza con
			// "[", así que aceptar cualquier "[" a secas volvía a tragarlo
			// si el array anterior había quedado sin cerrar. Si la línea no
			// encaja, ya es prosa — se corta sin consumirla, igual que el
			// resto de los límites de este loop.
			if !isArrayContinuationForKey(openArrayKey, trimmedLine) {
				break chartLoop
			}
			arrayDepth += bracketDelta(trimmedLine)
			consumedLines++
			continue
		}

		// Más profundo que bodyIndent y sin un array abierto: contenido de
		// la llave que lo abrió (una desconocida, o `options:` — data/
		// series/labels/type multi-línea ya avanzaron `i` en su propio case
		// y no llegan acá), no una propiedad nueva del chart. Ver el
		// segundo rol de bodyIndent en su declaración, arriba, para el caso
		// real que esto evita: el `data:`/`backgroundColor:` anidados bajo
		// un `datasets:` inventado robándole el lugar al `data:` real de
		// nivel superior.
		if bodyIndent > 0 && currentIndent > bodyIndent {
			consumedLines++
			continue
		}

		// Todo contenido del chart es una propiedad "clave: valor" del
		// allowlist de abajo. Cualquier otra cosa —prosa, "@notes:", una
		// clave que no pertenece al vocabulario de charts— es la PRIMERA
		// línea de después del bloque, así que cierra el chart cerrado por
		// dedent y se corta SIN consumirla, para que el parser de nivel
		// superior la procese como lo que es.
		//
		// Antes no había ninguno de estos cortes: la línea no reconocida
		// caía hasta el consumedLines++ del final del loop, y como
		// ConsumedLines es lo que le dice al llamador cuántas líneas
		// saltar, todo lo que el chart escaneara de más DESAPARECÍA del
		// documento sin diagnóstico (el chart en sí renderizaba bien, así
		// que la pérdida era muda). Con un <<chart>> sin <<end>> —la
		// convención documentada en spec/language-specification.md:75,
		// element_terminator ::= "<<end>>" | block_boundary | EOF— eso se
		// tragaba el párrafo y el bloque @notes: siguientes.
		//
		// El allowlist es el mecanismo que la spec (sec. "element_data",
		// misma línea) ya nombra para esta garantía, y el que map.go usa
		// desde siempre en su parseLoop. La alternativa de mermaid.go
		// (ShouldProcessLine, corte por sangría) NO sirve acá: hay charts
		// con las propiedades en columna 0 (examples/use-cases/educational/
		// machine_learning_intro.slidelang), y ahí ShouldProcessLine corta
		// en la primera línea del bloque y borra el chart entero.
		if !strings.Contains(trimmedLine, ":") {
			break chartLoop
		}
		parts := strings.SplitN(trimmedLine, ":", 2)
		if len(parts) != 2 {
			break chartLoop
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "data":
			// Detectar si hay datos definidos
			if value == "[" {
				// Datos en formato array multi-línea - parsear las líneas siguientes
				data, linesConsumed := p.parseMultiLineArray(ctx.Lines, i+1, indentDetector)
				chart.Data = data
				i += linesConsumed
				consumedLines += linesConsumed
			} else if strings.Contains(value, "[") {
				// Datos inline - verificar si es array de arrays o array simple
				if strings.HasPrefix(value, "[[") {
					// Array de arrays inline: [[100, 500, 350, 220]]
					chart.Data = p.parseInlineMatrix(value)
				} else {
					// Array simple inline: [100, 500, 350, 220]
					if row := p.parseArrayRow(value); len(row) > 0 {
						chart.Data = [][]interface{}{row}
					}
				}
			}
		case "series":
			// Parsear series como array de strings
			if value == "[" { // Series en formato array multi-línea
				series, linesConsumed := p.parseMultiLineStringArray(ctx.Lines, i+1, indentDetector)
				chart.Series = series
				i += linesConsumed
				consumedLines += linesConsumed
			} else if strings.Contains(value, "[") {
				// Series inline - parsear directamente del value
				if series := p.parseQuotedStrings(value); len(series) > 0 {
					chart.Series = series
				}
			}
		case "labels":
			// Parsear labels como array de strings
			if value == "[" { // Labels en formato array multi-línea
				labels, linesConsumed := p.parseMultiLineStringArray(ctx.Lines, i+1, indentDetector)
				chart.Labels = labels
				i += linesConsumed
				consumedLines += linesConsumed
			} else if strings.Contains(value, "[") {
				// Labels inline - parsear directamente del value
				if labels := p.parseQuotedStrings(value); len(labels) > 0 {
					chart.Labels = labels
				}
			}
		case "options":
			// El bloque options: es YAML anidado arbitrario (la config
			// de Chart.js: plugins, scales, datalabels...), no una
			// propiedad de una línea como las de al lado, así que se
			// captura entero por sangría y se deserializa aparte.
			//
			// Hasta este parche NO existía este case: el switch solo
			// conocía data/series/labels/title/type, y todo bloque
			// options: del DSL se descartaba en silencio antes de
			// llegar al AST. Eso no era un hueco solo de PPTX —
			// también borraba el título de los charts cuyo texto vive
			// en options.plugins.title.text (que es como lo escriben
			// los ejemplos de examples/02_diagrams_and_charts/), así
			// que el HTML llevaba tiempo perdiendo títulos que el
			// autor sí había pedido.
			opts, linesConsumed := p.parseNestedOptions(ctx.Lines, i+1)
			if opts != nil {
				chart.Options = opts
			}
			i += linesConsumed
			consumedLines += linesConsumed
		case "title":
			chart.Title = strings.Trim(value, "\"")
		case "type":
			// Parsear type array para combo charts: ["bar", "bar", "line"]
			if value == "[" {
				// Tipos en formato array multi-línea
				types, linesConsumed := p.parseMultiLineStringArray(ctx.Lines, i+1, indentDetector)
				chart.SeriesTypes = types
				i += linesConsumed
				consumedLines += linesConsumed
			} else if strings.Contains(value, "[") {
				// Tipos inline - parsear directamente del value
				if types := p.parseQuotedStrings(value); len(types) > 0 {
					chart.SeriesTypes = types
				}
			}
		default:
			// isChartPropertyKey distingue dos formas de "no manejada por
			// el switch": vocabulario de Chart.js que el normalizador
			// reconoce y emite como sintaxis de chart (datasets:,
			// backgroundColor:, fill:...) pero que el switch no consume, y
			// una clave que no pertenece a ese vocabulario en absoluto.
			//
			// La primera SIGUE siendo contenido del bloque: cortar en ella
			// dejaba el chart sin datos, disparaba CHART001 y mandaba el
			// resto del bloque a texto (regresión que el PR de dedent
			// encontró). Pero silenciarla del todo reabre el defecto
			// original de CHART005 (PR #232): la plantilla `report` de
			// `doclang init` shipeó con un `datasets:`/`backgroundColor:`
			// que ni el AST ni el render veían nunca, sin ningún aviso. Se
			// consume igual (no rompe el chart) pero se junta para el
			// CHART005 de más abajo.
			//
			// La segunda no es una propiedad del chart, así que la línea ya
			// pertenece a lo que sigue al bloque. Cortar sin consumirla.
			// Espeja el "default: break parseLoop" de map.go.
			if isChartPropertyKey(key) {
				unknownKeys = append(unknownKeys, key)
			} else {
				break chartLoop
			}
		}

		// Contar los corchetes de la línea de la propiedad MÁS los de lo que
		// el sub-parser haya avanzado: si queda algo abierto, las líneas que
		// siguen son del array y no límite del bloque.
		//
		// SOLO para las llaves del vocabulario de charts que aceptan forma
		// de array (isArrayValuedKey): son las únicas cuyo valor puede dejar
		// un corchete sin cerrar (los dos casos del corpus: cierre
		// dedentado, array en columna 0). options: captura su bloque por
		// sangría con un mecanismo aparte (parseNestedOptions) y descarta el
		// contenido si no parsea como YAML — sumar sus corchetes acá contaba
		// texto que el chart ni siquiera conserva. Sin este filtro, un
		// options: malformado con un "[" suelto (p. ej. "plugins: [unclosed")
		// dejaba arrayDepth en positivo y la línea "data: [1, 2]" que le
		// sigue se leía como continuación de un array ajeno en vez de como
		// la propiedad data: que es — el chart perdía sus datos.
		if isArrayValuedKey(key) {
			openArrayKey = key
			for k := lineStart; k <= i && k < len(ctx.Lines); k++ {
				arrayDepth += bracketDelta(strings.TrimSpace(ctx.Lines[k]))
			}
			if arrayDepth < 0 {
				arrayDepth = 0
			}
			if arrayDepth == 0 {
				openArrayKey = ""
			}
		}

		consumedLines++
	}

	// chart.Data no lleva omitempty en el AST: si un bloque "data:" vacío o
	// mal formado dejó chart.Data en nil, se serializaría como JSON null en
	// vez de [] (issue #8 - viola el JSON Schema del contrato).
	if chart.Data == nil {
		chart.Data = [][]interface{}{}
	}

	// CHART005: una propiedad reconocida por el vocabulario de charts pero
	// no manejada por el switch se ignora, pero se avisa. Warning y no
	// Error, y sigue el mismo criterio que FRONT005/FRONT006/FRONT007 para
	// `toc:`/`page:`/`watermark:`: un typo o una llave que el parser todavía
	// no soporta no puede tumbar el build, pero tampoco puede evaporarse sin
	// señal. Esa evaporación es justo cómo la plantilla `report` de
	// `doclang init` shipeó con un `datasets:`/`backgroundColor:` que ni el
	// AST ni el render veían nunca.
	//
	// Una clave que NO pertenece al vocabulario de charts en absoluto nunca
	// llega acá: isChartPropertyKey ya cortó el bloque más arriba antes de
	// juntarla, así que esta lista solo contiene vocabulario real de
	// Chart.js que el parser todavía no traduce al AST — no prosa ni typos
	// de otra naturaleza.
	if len(unknownKeys) > 0 {
		diags = append(diags, diagnostics.NewWarning(
			fmt.Sprintf("Llave(s) de Chart.js reconocida(s) pero no soportada(s) por el parser (%s); su valor se ignora. La config arbitraria de Chart.js va dentro de 'options:'",
				strings.Join(unknownKeys, ", ")),
			pos, "chart-parser").WithRuleID("CHART005"))
	}

	return &ParseResult{
		Element:       chart,
		ConsumedLines: consumedLines,
		Diagnostics:   diags,
		Error:         nil,
	}
}

// parseMultiLineArray parsea un array multi-línea de datos para charts
func (p *ChartParser) parseMultiLineArray(lines []string, startIndex int, indentDetector *AutoDetectIndentation) ([][]interface{}, int) {
	var data [][]interface{}
	linesConsumed := 0

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check if this line should be processed
		if !indentDetector.ShouldProcessLine(line, false, i+1, "CHART-PARSER") {
			break
		}

		// Skip empty lines
		if trimmed == "" {
			linesConsumed++
			continue
		}

		// Check for end of array
		if strings.Contains(trimmed, "]") && !strings.Contains(trimmed, "[") {
			linesConsumed++
			break
		}

		// Parse array row like ["Q1", 45, 32, 28]
		if strings.HasPrefix(trimmed, "[") {
			row := p.parseArrayRow(trimmed)
			if len(row) > 0 {
				data = append(data, row)
			}
		}

		linesConsumed++
	}

	return data, linesConsumed
}

// parseMultiLineStringArray parsea un array multi-línea de strings para series
func (p *ChartParser) parseMultiLineStringArray(lines []string, startIndex int, indentDetector *AutoDetectIndentation) ([]string, int) {
	var series []string
	linesConsumed := 0

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check if this line should be processed
		if !indentDetector.ShouldProcessLine(line, false, i+1, "CHART-PARSER") {
			break
		}

		// Skip empty lines
		if trimmed == "" {
			linesConsumed++
			continue
		}

		// Check for end of array
		if strings.Contains(trimmed, "]") && !strings.Contains(trimmed, "[") {
			linesConsumed++
			break
		}

		// Parse string items like "Product A", "Product B"
		if strings.Contains(trimmed, "\"") {
			// Extract all quoted strings from the line
			items := p.parseQuotedStrings(trimmed)
			series = append(series, items...)
		}

		linesConsumed++
	}

	return series, linesConsumed
}

// parseArrayRow parsea una fila de array como ["Q1", 45, 32, 28]
func (p *ChartParser) parseArrayRow(line string) []interface{} {
	var row []interface{}

	// Remove brackets properly - handle cases like ["January", 40], and ["June", 150]
	content := strings.TrimSpace(line)
	content = strings.TrimPrefix(content, "[")
	content = strings.TrimSuffix(content, "]")
	content = strings.TrimSuffix(content, "],") // Handle trailing comma
	content = strings.TrimSpace(content)

	// Respeta las comillas: una etiqueta con coma (["Berlin, Germany", 45])
	// se partía en dos items, así que la fila ganaba una columna y los
	// valores numéricos quedaban corridos una posición — corrupción
	// silenciosa de los datos de la gráfica, sin diagnóstico que la delate
	// (CHART001 solo verifica que HAYA datos). Mismo bug que el splitter de
	// TABLE (ver splitInlineArray en table.go); acá el helper compartido
	// devuelve los items en bruto porque el loop de abajo necesita ver las
	// comillas para distinguir etiqueta de número.
	parts, ok := splitTopLevelCommas(content)
	if !ok {
		// Comillas desbalanceadas: split ingenuo de siempre.
		parts = strings.Split(content, ",")
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Try to parse as number
		if strings.Contains(part, "\"") {
			// String value
			str := strings.Trim(part, "\"")
			row = append(row, str)
		} else {
			// Try to parse as number
			if val := p.parseNumber(part); val != nil {
				row = append(row, val)
			}
		}
	}

	return row
}

// parseQuotedStrings extrae todas las cadenas entre comillas de una línea
func (p *ChartParser) parseQuotedStrings(line string) []string {
	var result []string
	inQuotes := false
	var current strings.Builder

	for _, char := range line {
		if char == '"' {
			if inQuotes {
				// End of quoted string
				result = append(result, current.String())
				current.Reset()
				inQuotes = false
			} else {
				// Start of quoted string
				inQuotes = true
			}
		} else if inQuotes {
			current.WriteRune(char)
		}
	}

	return result
}

// parseNumber intenta parsear un string como número
func (p *ChartParser) parseNumber(str string) interface{} {
	str = strings.TrimSpace(str)

	// Try integer first
	if val, err := strconv.Atoi(str); err == nil {
		return val
	}

	// Try float
	if val, err := strconv.ParseFloat(str, 64); err == nil {
		return val
	}

	return nil
}

// bracketDelta cuenta los corchetes de apertura menos los de cierre de una
// línea, ignorando los que van dentro de una cadena entre comillas (una
// etiqueta como "Ventas [MXN]" no debe alterar la cuenta). El loop de
// propiedades lo usa para saber si un array multi-línea quedó abierto.
func bracketDelta(trimmed string) int {
	depth := 0
	inString := false
	escaped := false
	for _, r := range trimmed {
		if escaped {
			escaped = false
			continue
		}
		switch {
		case r == '\\':
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// dentro de una cadena los corchetes son texto
		case r == '[':
			depth++
		case r == ']':
			depth--
		}
	}
	return depth
}

// isArrayContinuationForKey reporta si trimmed tiene forma de contenido de
// continuación para el array multi-línea que abrió openKey. Usado solo
// dentro de la rama arrayDepth > 0 para no tragar prosa (ni un enlace
// Markdown, en cualquiera de sus dos formas — inline "[texto](url)" o por
// referencia "[texto][ref]") cuando el array nunca cierra.
//
// Antes esto se decidía por PREFIJO ("empieza con \"[\"" o "contiene una
// comilla"), y las dos formas se dejaban engañar por contenido que solo
// PARECE continuación:
//   - Un enlace también empieza con "[", así que calificaba como fila de
//     data en cuanto el array anterior quedaba sin cerrar.
//   - Cualquier frase con una comilla en cualquier parte calificaba como
//     elemento de series/labels/type, aunque no fuera un string completo.
//   - Y al revés: una fila de data legítima cuyo VALOR de texto contiene un
//     enlace embebido ("[\"[Ver fuente](url)\", 5],") se rechazaba, porque
//     el substring "](" aparecía sin que el chequeo supiera que estaba
//     dentro de una cadena entre comillas.
//
// Ahora se valida por GRAMÁTICA, no por prefijo, con el mismo escaneo
// consciente de comillas que bracketDelta:
//   - "data": una fila entre corchetes (isDataArrayRow, la forma matriz de
//     parseArrayRow) O un escalar suelto (isScalarContinuation) — data
//     también soporta la forma plana de un solo valor por línea
//     ("data: [\n1,\n2\n]"), que parseMultiLineArray tolera igual (ignora
//     silenciosamente las filas sin corchetes) pero que antes ninguna de
//     las dos formas de continuación reconocía.
//   - cualquier otra clave array-valuada (series/labels/type/
//     backgroundColor/borderColor/borderWidth/pointRadius/...): un escalar
//     suelto (isScalarContinuation) — string entre comillas para las
//     propiedades de texto, número para las numéricas
//     (borderWidth/pointRadius/...). No hay una tabla de tipos por
//     propiedad: aceptar cualquiera de las dos formas es más simple que
//     mantenerla, y el costo (aceptar un número donde solo cabría un string,
//     o viceversa) es el mismo tipo de ambigüedad ya asumida para el
//     escalar entre comillas — contenido malformado o un array roto, no el
//     corpus real.
//
// En cualquier caso, puros delimitadores de cierre/coma ("]", "],", "}]")
// cierran el array sin importar el tipo.
func isArrayContinuationForKey(openKey, trimmed string) bool {
	if trimmed == "" {
		return false
	}
	if isPureArrayCloser(trimmed) {
		return true
	}
	if openKey == "data" && isDataArrayRow(trimmed) {
		return true
	}
	return isScalarContinuation(trimmed)
}

// isPureArrayCloser reporta si trimmed consiste únicamente en delimitadores
// de cierre y comas ("]", "],", "}]"...) — el resto de un array multi-línea
// que su sub-parser dejó sin consumir, sin importar el tipo de array.
func isPureArrayCloser(trimmed string) bool {
	for _, r := range trimmed {
		if r != ']' && r != '}' && r != ',' {
			return false
		}
	}
	return true
}

// isDataArrayRow reporta si trimmed es una fila de data completa en su
// propia línea: un grupo delimitado y balanceado (consciente de comillas,
// como bracketDelta) que arranca en la posición 0 y no deja nada después de
// cerrar salvo una coma opcional.
//
// Dos formas, según el delimitador con el que arranca la línea:
//   - "[...]": la fila matriz que produce parseArrayRow ("[\"Q1\", 1]," o
//     "[10, 20]").
//   - "{...}": un objeto de punto ("{x: 10, y: 20},"), la forma que
//     Chart.js espera para datasets de scatter/bubble
//     (https://www.chartjs.org/docs/latest/general/data-structures.html) —
//     el corpus solo la escribe hoy dentro del modo JSON crudo (que tiene su
//     propio parser, parseJSONBlock, ajeno a este), pero la forma DSL
//     "data: [...]" con objetos por línea es sintaxis igual de válida y no
//     tenía ninguna forma de sobrevivir como continuación.
//
// Un enlace Markdown también empieza con "[" pero después de su "]" de
// cierre le sigue "(url)" o "[ref]", no fin de línea ni coma — eso es lo que
// lo descarta en la forma "[...]"; ningún enlace empieza con "{", así que la
// forma "{...}" no necesita ese mismo chequeo.
func isDataArrayRow(trimmed string) bool {
	switch {
	case strings.HasPrefix(trimmed, "["):
		return isBalancedGroupRow(trimmed, '[', ']')
	case strings.HasPrefix(trimmed, "{"):
		return isBalancedGroupRow(trimmed, '{', '}')
	default:
		return false
	}
}

// isBalancedGroupRow reporta si trimmed es un único grupo delimitado por
// open/close, balanceado y consciente de comillas (igual que bracketDelta),
// que no deja nada después de cerrar salvo una coma opcional. Compartido por
// isDataArrayRow para sus dos formas ("[...]" y "{...}") — el delimitador
// contrario al elegido no se rastrea, así que un "{" dentro de una fila
// "[...]" (o viceversa) se trata como texto normal, igual que cualquier otro
// carácter que no sea el par que se está balanceando.
func isBalancedGroupRow(trimmed string, open, close rune) bool {
	depth := 0
	inString := false
	escaped := false
	closeIdx := -1
	for idx, r := range trimmed {
		if escaped {
			escaped = false
			continue
		}
		switch {
		case r == '\\':
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// dentro de una cadena los delimitadores son texto, igual que
			// en bracketDelta — así una fila cuyo VALOR contiene un enlace
			// embebido ("[\"[Ver fuente](url)\", 5]") no se confunde con
			// el propio enlace.
		case r == open:
			depth++
		case r == close:
			depth--
			if depth == 0 {
				closeIdx = idx
			}
		}
		if closeIdx != -1 {
			break
		}
	}
	if closeIdx == -1 {
		// El delimitador inicial nunca cerró en esta línea: no es una fila
		// autocontenida, así que no calificar como continuación es lo
		// conservador (evita tragar contenido que no se puede validar).
		return false
	}
	rest := strings.TrimSpace(trimmed[closeIdx+1:])
	return rest == "" || rest == ","
}

// isQuotedScalar reporta si trimmed es un único string entre comillas
// completo en su propia línea, con nada después salvo una coma opcional —
// la forma que parseMultiLineStringArray espera para elementos de
// series/labels/type y para cualquier otra propiedad array-valuada de solo
// strings (backgroundColor, borderColor...). Una frase suelta con una
// comilla en cualquier parte ("Nota: usa \"comillas\" aquí.") no arranca
// con comilla, así que no califica; y una frase que sí arranca con comilla
// pero sigue después de cerrarla ("\"cita\" y algo más") tampoco, porque el
// cierre no queda al final.
func isQuotedScalar(trimmed string) bool {
	s := strings.TrimSuffix(trimmed, ",")
	if len(s) < 2 || s[0] != '"' {
		return false
	}
	escaped := false
	for idx := 1; idx < len(s); idx++ {
		c := s[idx]
		if escaped {
			escaped = false
			continue
		}
		switch c {
		case '\\':
			escaped = true
		case '"':
			return idx == len(s)-1
		}
	}
	return false
}

// isScalarContinuation reporta si trimmed es un único escalar completo en su
// propia línea, con nada después salvo una coma opcional: un string entre
// comillas (isQuotedScalar), un número (isNumericScalar), o el literal
// "null" — Chart.js documenta null como el valor explícito para un punto
// omitido dentro de un dataset (https://www.chartjs.org/docs/latest/general/
// data-structures.html), así que es una forma tan legítima de fila plana
// como un número. Cubre la forma plana de cualquier array multi-línea de un
// solo valor por línea, sea de texto (series/labels/type/backgroundColor/
// borderColor) o numérico (borderWidth/pointRadius/pointHoverRadius, y la
// forma plana de data).
func isScalarContinuation(trimmed string) bool {
	return isQuotedScalar(trimmed) || isNumericScalar(trimmed) || isNullScalar(trimmed)
}

// isNullScalar reporta si trimmed es el literal "null" completo en su propia
// línea, con nada después salvo una coma opcional. No extiende
// isNumericScalar (ParseFloat legítimamente no debe aceptar "null" como
// número) sino que se prueba aparte, como una tercera forma de escalar.
func isNullScalar(trimmed string) bool {
	s := strings.TrimSuffix(strings.TrimSpace(trimmed), ",")
	return strings.TrimSpace(s) == "null"
}

// isNumericScalar reporta si trimmed es un único número completo en su
// propia línea, con nada después salvo una coma opcional ("1,", "2.5",
// "-3"). strconv.ParseFloat ya rechaza cualquier cosa con texto extra
// (no hace falta un chequeo de "nada después": ParseFloat falla en cuanto
// sobra algo que no sea parte del número).
func isNumericScalar(trimmed string) bool {
	s := strings.TrimSuffix(strings.TrimSpace(trimmed), ",")
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// isArrayValuedKey reporta si key es una propiedad reconocida del vocabulario
// de charts (chartPropertyKeys) DISTINTA de "options": cualquiera de ellas
// puede llevar un valor de array multi-línea que un sub-parser deje sin
// cerrar en las dos formas que el corpus escribe (cierre dedentado, array en
// columna 0), así que todas necesitan el tracking de arrayDepth.
//
// "options" es la única exclusión deliberada: captura su bloque por sangría
// con un mecanismo aparte (parseNestedOptions) que descarta el contenido
// entero si no parsea como YAML — sumar sus corchetes al tracking contaba
// texto que el chart ni siquiera conserva (un "[" suelto dentro de un
// options: malformado dejaba arrayDepth en positivo para la propiedad que
// seguía).
//
// Antes esto listaba a mano solo {data, series, labels, type} — el switch de
// arriba solo ACTÚA sobre esas cuatro, pero otras del vocabulario
// (backgroundColor, borderColor...) también pueden escribirse en forma de
// array multi-línea aunque el switch las deje en el default sin procesar; no
// estar en la lista a mano las dejaba sin tracking y un
// "backgroundColor: [" cortaba el chart en la primera línea de colores.
func isArrayValuedKey(key string) bool {
	return key != "options" && isChartPropertyKey(key)
}

// chartPropertyKeys es el vocabulario de propiedades que el DSL reconoce como
// sintaxis de chart. El switch de Parse solo ACTÚA sobre unas pocas
// (data/series/labels/options/title/type); el resto se reconocen para poder
// consumirlas sin cortar el bloque.
//
// La lista espeja isChartDataLine en
// internal/normalize/normalizer/rules/enhancement/chart_formatter.go y
// chartProperties en internal/normalize/normalizer/detector.go — el
// normalizador emite estas claves y las trata como chart, así que el parser
// no puede tratarlas como el fin del bloque. Mantenerlas en sync.
var chartPropertyKeys = map[string]bool{
	// las que el switch sí maneja
	"data": true, "series": true, "labels": true,
	"options": true, "title": true, "type": true,
	// reconocidas por el normalizador, aún no consumidas por el switch
	"datasets": true, "backgroundColor": true, "borderColor": true,
	"borderWidth": true, "label": true, "fill": true, "tension": true,
	"pointRadius": true, "pointHoverRadius": true, "xAxisID": true,
	"yAxisID": true, "plugins": true, "legend": true, "responsive": true,
	"text": true, "display": true, "position": true,
}

// isChartPropertyKey reporta si key pertenece al vocabulario de propiedades
// de chart, es decir si la línea sigue siendo contenido del bloque aunque el
// switch no la use.
func isChartPropertyKey(key string) bool {
	return chartPropertyKeys[key]
}

// isChartContentBoundary reporta si rawLine marca el fin del contenido de un
// chart: el cierre del propio bloque ("<</chart>>"), un separador de slide
// flex ("---"), un límite de bloque strict ("SLIDE "/"SECTION " en columna 0
// — ver IsStrictBlockBoundary), o el inicio de un nuevo elemento/sección.
// Recibe la línea SIN trim a propósito: el chequeo de límite strict necesita
// la sangría cruda para no confundir un límite real con una fila de datos
// indentada que casualmente empieza con esas palabras. Compartida entre el
// loop de propiedades y parseJSONBlock para que ambos no puedan
// desincronizarse sobre qué cuenta como límite (issue #12e2 — la revisión de
// esa misma PR encontró que el check original de parseJSONBlock solo
// reconocía 2 de los 5 límites que el loop de propiedades ya conocía; issue
// #107 — el mismo defecto de desincronización, esta vez porque ninguno de
// los dos conocía "SLIDE ", así que un chart en modo strict sin <<end>> se
// tragaba todos los slides siguientes hasta EOF).
func isChartContentBoundary(rawLine string) bool {
	if IsStrictBlockBoundary(rawLine) {
		return true
	}
	trimmed := strings.TrimSpace(rawLine)
	if trimmed == "<<end>>" || trimmed == "---" {
		return true
	}
	if strings.HasPrefix(trimmed, "<<") {
		return true
	}
	if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "##") {
		return true // H1 crea nuevas secciones, no H2/H3 ("##", "###")
	}
	if strings.HasPrefix(trimmed, "##") {
		return true // H2/H3 son subsection headers
	}
	return false
}

// parseJSONBlock parsea un bloque JSON completo desde las líneas
func (p *ChartParser) parseJSONBlock(lines []string, startIndex int) (string, int) {
	var jsonLines []string
	braceCount := 0
	inString := false
	escaped := false
	linesConsumed := 0

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]

		// Si las llaves nunca balancean (JSON truncado/mal formado), no
		// tragarse el separador de slide ni el cierre del propio bloque:
		// detenerse aquí y dejar la línea sin consumir para que el parser
		// de nivel superior la procese normalmente (issue #12e2 — antes,
		// el fallback de líneas 459 y siguientes devolvía TODO el resto del
		// documento, incluyendo "---", como si fuera parte del JSON).
		// Gateado por "!inString": un valor JSON legítimo puede contener,
		// en una línea propia, el texto exacto "---" o "<</chart>>" (p.ej.
		// una descripción documentando la sintaxis del DSL) sin que eso
		// signifique que el bloque terminó.
		if !inString {
			if isChartContentBoundary(line) {
				break
			}
		}

		jsonLines = append(jsonLines, line)
		linesConsumed++

		// Procesar caracteres para encontrar el final del JSON
		for _, char := range line {
			if escaped {
				escaped = false
				continue
			}

			if char == '\\' {
				escaped = true
				continue
			}

			if char == '"' {
				inString = !inString
				continue
			}

			if !inString {
				switch char {
				case '{':
					braceCount++
				case '}':
					braceCount--
					if braceCount == 0 {
						// JSON completo encontrado
						// Check if next line is closing tag
						if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "<</chart>>" {
							linesConsumed++ // Consume closing tag line
						}

						// Compactar para evitar problemas de template
						jsonStr := strings.Join(jsonLines, "\n")
						// Remover saltos de línea y espacios extra para compactar
						compactJSON := strings.ReplaceAll(jsonStr, "\n", "")
						compactJSON = strings.ReplaceAll(compactJSON, "\t", "")
						// Remover espacios múltiples pero preservar espacios en strings
						return compactJSON, linesConsumed
					}
				}
			}
		}
	}

	// Si llegamos aquí, el JSON NUNCA balanceó: está truncado o mal formado.
	//
	// Devolver todo lo escaneado era la pérdida silenciosa del issue #237.
	// El loop de arriba solo se detiene ante un límite duro
	// (isChartContentBoundary) o EOF, así que un payload roto se llevaba por
	// delante todo lo que hubiera en medio —prosa, viñetas, el <<end>> del
	// propio chart, y con una comilla sin cerrar hasta los slides
	// siguientes enteros, porque inString apagaba también el chequeo de
	// límite. El chart quedaba sin datos (CHART002 lo dice) pero el
	// contenido borrado no lo reportaba nadie: ConsumedLines es lo único
	// que decide qué sobrevive.
	//
	// El recorte va ACÁ y no dentro del loop a propósito. El escaneo de
	// arriba queda intacto, y con él el caso que su propio code-review fijó
	// (#12e2): un string JSON que abarca varias líneas es legítimo en este
	// parser, porque la compactación borra los saltos de línea antes de
	// validar — ver TestChartParser_ParseJSONBlock_BoundaryInsideStringDoesNotBreak.
	// Ese payload BALANCEA, así que retorna arriba y nunca llega hasta acá.
	// Solo el payload que no balanceó se recorta, y ahí la forma de cada
	// línea ya es la única evidencia disponible.
	if len(jsonLines) > 0 {
		extent := jsonPayloadExtent(jsonLines)
		return strings.Join(jsonLines[:extent], "\n"), extent
	}
	return "", 0
}

// jsonPayloadExtent devuelve cuántas de jsonLines pertenecen de verdad al
// payload de un chart cuyo JSON no balanceó. Se llama solo en ese caso: un
// payload que cierra retorna antes, sin pasar por acá.
//
// Dos criterios, el mismo par que el issue #234 fijó para el loop de
// propiedades:
//
//   - Dedent: si el payload arranca indentado, una línea menos sangrada que
//     su primera línea ya salió del bloque. No aplica cuando arranca en
//     columna 0, donde la sangría no distingue nada.
//   - Forma: una línea que no puede ser JSON (isJSONPayloadLine) es la
//     primera de lo que sigue, no la continuación de lo que se rompió.
//
// La línea 0 se exime del criterio de forma: es la que abrió el bloque, y
// Parse solo llama a parseJSONBlock cuando empieza con "{".
func jsonPayloadExtent(jsonLines []string) int {
	bodyIndent := -1
	// arrayDepth lleva la cuenta de corchetes sin cerrar, igual que el loop
	// de propiedades. Decide si un escalar suelto ("B", 42) tiene dónde
	// vivir: dentro de un array es un elemento legítimo; fuera no es JSON de
	// ninguna forma, y por lo tanto es prosa. Sin ese contexto habría que
	// aceptar todo escalar suelto, y con eso se cuela cualquier párrafo
	// entrecomillado.
	arrayDepth := 0

	for i, line := range jsonLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		currentIndent := CalculateIndentLevel(line)
		if bodyIndent == -1 {
			bodyIndent = currentIndent
		} else if bodyIndent > 0 && currentIndent < bodyIndent {
			return i
		}

		if i > 0 && !isJSONPayloadLine(trimmed, arrayDepth > 0) {
			return i
		}

		arrayDepth += bracketDelta(trimmed)
		if arrayDepth < 0 {
			arrayDepth = 0
		}
	}

	return len(jsonLines)
}

// isJSONPayloadLine reporta si trimmed puede ser una línea del payload JSON
// de un chart. Solo lo consulta jsonPayloadExtent, o sea solo cuando el
// payload ya se sabe roto: la pregunta no es "¿esto es JSON válido?" sino
// "¿esto puede seguir siendo el JSON, o ya es la prosa de después?".
//
// Cada forma se valida como TOKEN COMPLETO, no por su primer carácter. La
// primera versión de este guard miraba solo el carácter inicial, y con eso
// seguía tragándose Markdown perfectamente común: "1. Primer paso" pasaba
// por empezar con dígito, "[Más contexto](url)" por empezar con "[", y un
// párrafo entrecomillado por empezar con comilla. Los tres son exactamente
// la pérdida silenciosa que este recorte existe para cerrar.
//
// insideArray dice si hay un "[" sin cerrar entre las líneas ya aceptadas.
// Es lo que separa un escalar suelto legítimo —el último elemento de un
// array multi-línea, que va sin coma— de un párrafo que casualmente abre y
// cierra comillas.
//
// Se aceptan comentarios ("//", "/*", "*/") porque el JSON que escribe un
// modelo los trae seguido — es la razón de existir de ChartJSONRule
// (internal/normalize), que los limpia antes del parser en el camino de la
// CLI pero no en el de la API de Go.
//
// Degradación conocida y elegida: una llave sin comillas ("type: bar", que
// es YAML y no JSON) corta el bloque y sale como texto. Y dentro de un array
// abierto, un párrafo que empieza y termina con comillas sigue siendo
// indistinguible de un elemento del array — ahí la forma de la línea no
// alcanza para decidir. En los dos casos la prosa queda VISIBLE en la
// diapositiva, que es lo que este recorte prefiere sobre borrarla.
func isJSONPayloadLine(trimmed string, insideArray bool) bool {
	if trimmed == "" {
		return true
	}

	for _, prefix := range []string{"//", "/*", "*/"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	// Solo estructura: "{", "}", "},", "]", "}]", "[", ...
	if isJSONPunctuationOnly(trimmed) {
		return true
	}

	// Un par `"clave": ...`, la forma más común dentro de un objeto.
	if isJSONKeyLine(trimmed) {
		return true
	}

	// Una fila completa —objeto {..} o array [..]— sola en su línea y sin
	// nada después salvo una coma. isDataArrayRow verifica la FORMA (que el
	// grupo cierre y no quede nada detrás), y json.Valid verifica que ADEMÁS
	// sea JSON.
	//
	// Los dos chequeos hacen falta y ninguno sobra. Solo la forma dejaba
	// pasar "[Más contexto]", que cierra limpio pero cuyo contenido no es
	// JSON de ninguna manera — un enlace de referencia Markdown, prosa
	// perfectamente común. Solo json.Valid dejaría pasar una línea que
	// arranca con un valor válido y sigue con otra cosa.
	//
	// isDataArrayRow queda intacta: la usa también el loop de propiedades,
	// donde el DSL SÍ admite filas que no son JSON (la forma de punto
	// "{x: 10, y: 20}" de scatter, con las llaves sin comillas). Ese
	// requisito extra vale solo acá, donde el bloque se declaró JSON.
	if isDataArrayRow(trimmed) && json.Valid([]byte(strings.TrimSpace(strings.TrimSuffix(trimmed, ",")))) {
		return true
	}

	// Escalares sueltos: solo tienen dónde vivir dentro de un array.
	if insideArray && (isScalarContinuation(trimmed) || isBooleanScalar(trimmed)) {
		return true
	}

	return false
}

// isJSONPunctuationOnly reporta si trimmed se compone solo de delimitadores
// JSON y espacios — las líneas que abren o cierran objetos y arrays.
func isJSONPunctuationOnly(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		switch r {
		case '{', '}', '[', ']', ',', ' ', '\t':
		default:
			return false
		}
	}
	return true
}

// isJSONKeyLine reporta si trimmed abre con una cadena JSON completa seguida
// de ":" y de algo que PUEDA ser un valor JSON.
//
// Validar el arranque del valor no es opcional: sin eso, `"Nota": este texto
// debe sobrevivir` —prosa que abre con una palabra entrecomillada y dos
// puntos, cosa que el español escribe todo el tiempo— tenía exactamente la
// forma de una clave y se tragaba el resto del bloque.
//
// Acá sí alcanza con mirar el primer carácter, a diferencia del chequeo de
// línea completa: la gramática de JSON dice que un valor solo puede empezar
// con comilla, "{", "[", "-" o dígito. Los literales true/false/null se
// exigen completos, porque si no cualquier palabra que empiece con "t", "f"
// o "n" pasaría — "texto", justamente.
func isJSONKeyLine(trimmed string) bool {
	end := jsonStringEnd(trimmed)
	if end == -1 {
		return false
	}
	rest := strings.TrimSpace(trimmed[end+1:])
	if !strings.HasPrefix(rest, ":") {
		return false
	}
	return isJSONValueStart(strings.TrimSpace(rest[1:]))
}

// isJSONValueStart reporta si rest puede ser el principio de un valor JSON.
// El vacío cuenta: una clave puede dejar su valor para las líneas de abajo
// (`"data":` y en la siguiente el `{`).
func isJSONValueStart(rest string) bool {
	if rest == "" {
		return true
	}

	switch rest[0] {
	case '"', '{', '[', '-':
		return true
	}
	if rest[0] >= '0' && rest[0] <= '9' {
		return true
	}

	return isBooleanScalar(rest) || isNullScalar(rest)
}

// jsonStringEnd devuelve el índice de la comilla que cierra la cadena que
// empieza en trimmed[0], o -1 si trimmed no abre con una cadena completa.
// Consciente de escapes, igual que bracketDelta e isQuotedScalar.
func jsonStringEnd(trimmed string) int {
	if len(trimmed) < 2 || trimmed[0] != '"' {
		return -1
	}
	escaped := false
	for idx := 1; idx < len(trimmed); idx++ {
		switch {
		case escaped:
			escaped = false
		case trimmed[idx] == '\\':
			escaped = true
		case trimmed[idx] == '"':
			return idx
		}
	}
	return -1
}

// isBooleanScalar completa a isScalarContinuation, que cubre cadena, número
// y null pero no los booleanos.
func isBooleanScalar(trimmed string) bool {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(trimmed), ","))
	return s == "true" || s == "false"
}

// parseYAMLBlock extrae un bloque YAML completo
func (p *ChartParser) parseYAMLBlock(lines []string, startIndex int) (string, int) {
	var yamlLines []string
	linesConsumed := 0
	indentDetector := NewAutoDetectIndentation()

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]

		// Check if this line should be processed
		if !indentDetector.ShouldProcessLine(line, false, i+1, "CHART-YAML-PARSER") {
			break
		}

		trimmed := strings.TrimSpace(line)

		// Skip empty lines but include them in YAML
		if trimmed == "" {
			yamlLines = append(yamlLines, line)
			linesConsumed++
			continue
		}

		yamlLines = append(yamlLines, line)
		linesConsumed++
	}

	if len(yamlLines) > 0 {
		return strings.Join(yamlLines, "\n"), linesConsumed
	}
	return "", 0
}

// parseNestedOptions captura el bloque YAML anidado que sigue a "options:" y
// lo deserializa a map[string]interface{} — la misma forma que
// parseComboChartYAML ya produce para chart.Options por la ruta combo, y la
// que el renderer HTML espera para inyectarla como config de Chart.js.
//
// El bloque se delimita por sangría: pertenecen a options: todas las líneas
// más indentadas que la propia "options:", más las vacías intercaladas. Se
// dedenta al mínimo común antes de parsear porque yaml.Unmarshal rechaza un
// documento cuya primera línea ya viene sangrada.
//
// Devuelve (nil, n) si el YAML no parsea: un options: mal formado no debe
// tumbar el chart entero — se descarta la config y el chart se renderiza sin
// ella, que es exactamente el comportamiento que había antes de que este
// parser existiera.
func (p *ChartParser) parseNestedOptions(lines []string, startIndex int) (map[string]interface{}, int) {
	if startIndex >= len(lines) {
		return nil, 0
	}

	baseIndent := countLeadingSpaces(lines[startIndex])
	if baseIndent == 0 {
		// Sin sangría no hay bloque anidado que capturar.
		return nil, 0
	}

	var block []string
	consumed := 0
	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			// Una línea vacía no cierra el bloque, pero tampoco lo extiende
			// por sí sola: solo se incluye si algo más indentado la sigue.
			block = append(block, "")
			consumed++
			continue
		}
		if countLeadingSpaces(line) < baseIndent {
			break
		}
		block = append(block, line[baseIndent:])
		consumed++
	}

	// Recortar las vacías del final, que no pertenecen al bloque.
	for len(block) > 0 && strings.TrimSpace(block[len(block)-1]) == "" {
		block = block[:len(block)-1]
		consumed--
	}
	if len(block) == 0 {
		return nil, 0
	}

	var opts map[string]interface{}
	if err := yaml.Unmarshal([]byte(strings.Join(block, "\n")), &opts); err != nil {
		return nil, consumed
	}
	return opts, consumed
}

// countLeadingSpaces cuenta la sangría de una línea tratando un tab como un
// espacio — basta para comparar niveles relativos, que es lo único que
// parseNestedOptions necesita.
func countLeadingSpaces(line string) int {
	n := 0
	for _, r := range line {
		if r != ' ' && r != '\t' {
			break
		}
		n++
	}
	return n
}

// ChartData representa la estructura de datos para combo charts
type ChartData struct {
	Labels []string      `yaml:"labels"`
	Series []ChartSeries `yaml:"series"`
}

// ChartSeries representa una serie de datos en combo charts
type ChartSeries struct {
	Name    string        `yaml:"name"`
	Type    string        `yaml:"type"`
	Values  []interface{} `yaml:"values"`
	YAxisID string        `yaml:"yAxisID,omitempty"`
}

// ChartConfig representa la configuración completa del combo chart
type ChartConfig struct {
	Data    ChartData   `yaml:"data"`
	Options interface{} `yaml:"options,omitempty"`
}

// parseComboChartYAML parsea un combo chart usando YAML. Además de (bool)
// éxito, devuelve las llaves de nivel superior del YAML que ChartConfig no
// modela: yaml.Unmarshal a un struct tagueado ignora en silencio cualquier
// llave del mapeo sin campo correspondiente (a diferencia del loop de
// propiedades de la forma plana, que sí las junta línea por línea en
// unknownKeys), así que sin este segundo unmarshal a mapa un `datasets:` de
// nivel superior en un combo chart se perdía sin CHART005 (hallazgo de code
// review del PR #232).
func (p *ChartParser) parseComboChartYAML(chart *ast.ChartElement, yamlContent string) (bool, []string) {
	var config ChartConfig

	err := yaml.Unmarshal([]byte(yamlContent), &config)
	if err != nil {
		return false, nil
	}

	var unknownKeys []string
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &raw); err == nil {
		for key := range raw {
			if key != "data" && key != "options" {
				unknownKeys = append(unknownKeys, key)
			}
		}
		unknownKeys = append(unknownKeys, unknownComboDataKeys(raw)...)
		sort.Strings(unknownKeys)
	}

	// Asignar labels
	chart.Labels = config.Data.Labels

	// Procesar series
	chart.Series = make([]string, len(config.Data.Series))
	chart.SeriesTypes = make([]string, len(config.Data.Series))

	// Reorganizar datos al formato canónico fila-por-categoría que el resto
	// del repo espera de chart.Data: cada fila es [etiqueta, valor_serie0,
	// valor_serie1, ...] — el mismo shape que produce la forma plana
	// "data: [[fila],...]" + "series:" (ver examples/02_diagrams_and_charts,
	// y core/formatter/strict.go:544 sobre por qué esa es la única forma
	// combo que el round-trip strict conoce). Tanto
	// core/renderer/html.go:GenerateChartConfigWithMode como
	// slidelang/internal/generator/data/converter.go:createDatasetsFromSeries
	// leen row[i+1] de cada FILA para extraer la serie i (columna 0 se
	// descarta como la etiqueta). Antes, esta función guardaba chart.Data
	// indexado POR SERIE (chart.Data[serieIdx] = valores completos de esa
	// serie) — un layout que ningún renderer del repo entiende: con N
	// series de M valores cada una, row[i+1] leía el valor M-ésimo de la
	// serie i+1 en vez del i-ésimo de la serie i, así que Chart.js terminaba
	// con las labels sustituidas por los valores de la primera serie, la
	// primera serie corrida una serie a la derecha, y la última serie vacía
	// (fuera de rango) — un chart con datos, que pasa CHART001, pero visualmente
	// incorrecto.
	if len(config.Data.Series) > 0 {
		// Verificar que todas las series tengan el mismo número de valores
		seriesLength := len(config.Data.Series[0].Values)
		for _, series := range config.Data.Series {
			if len(series.Values) != seriesLength {
				return false, nil // Inconsistencia en datos
			}
		}

		chart.Data = make([][]interface{}, seriesLength)
		for catIdx := 0; catIdx < seriesLength; catIdx++ {
			row := make([]interface{}, 0, len(config.Data.Series)+1)
			if catIdx < len(config.Data.Labels) {
				row = append(row, config.Data.Labels[catIdx])
			} else {
				row = append(row, "")
			}
			for _, series := range config.Data.Series {
				row = append(row, series.Values[catIdx])
			}
			chart.Data[catIdx] = row
		}
	}

	// Asignar nombres y tipos de series
	for i, series := range config.Data.Series {
		chart.Series[i] = series.Name
		chart.SeriesTypes[i] = series.Type
	}

	// Asignar options si existen
	if config.Options != nil {
		if options, ok := config.Options.(map[string]interface{}); ok {
			chart.Options = options
		}
	}

	return true, unknownKeys
}

// unknownComboDataKeys extiende la detección de llaves desconocidas del
// combo chart un nivel más adentro: el chequeo de nivel superior (data vs
// options) no ve nada mal en un `data:` bien formado que en su INTERIOR
// tenga un typo, porque yaml.Unmarshal a ChartData/ChartSeries ignora en
// silencio cualquier campo del mapeo sin tag correspondiente — el mismo
// defecto de fondo que motivó el chequeo de nivel superior (hallazgo de
// code review), solo que un nivel más profundo. `data.Labels:` (mayúscula,
// en vez de `labels:`) es el caso real: el chart queda con Labels vacío y
// se renderiza sin etiquetas, sin ningún CHART005 que lo delate.
//
// raw ya viene deserializado a mapas genéricos (mismo yamlContent que
// ChartConfig, ver la llamada en parseComboChartYAML), así que esto es
// lectura pura, sin volver a tocar el YAML.
func unknownComboDataKeys(raw map[string]interface{}) []string {
	dataMap, ok := raw["data"].(map[string]interface{})
	if !ok {
		return nil
	}

	var unknown []string
	for key := range dataMap {
		if key != "labels" && key != "series" {
			unknown = append(unknown, "data."+key)
		}
	}

	seriesList, ok := dataMap["series"].([]interface{})
	if !ok {
		return unknown
	}
	seenSeriesKey := map[string]bool{}
	for _, item := range seriesList {
		seriesMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		for key := range seriesMap {
			switch key {
			case "name", "type", "values", "yAxisID":
				continue
			}
			// Una sola entrada por llave, no una por cada serie que la repita:
			// series suele tener varios elementos con la misma forma, y un
			// typo en el nombre del campo se repite en todos.
			if !seenSeriesKey[key] {
				seenSeriesKey[key] = true
				unknown = append(unknown, "data.series[]."+key)
			}
		}
	}
	return unknown
}

// parseInlineMatrix parsea arrays de arrays inline como [[100, 500, 350, 220]]
func (p *ChartParser) parseInlineMatrix(value string) [][]interface{} {
	var result [][]interface{}

	// Remover corchetes externos
	content := strings.TrimSpace(value)
	content = strings.TrimPrefix(content, "[")
	content = strings.TrimSuffix(content, "]")
	content = strings.TrimSpace(content)

	// Ahora tenemos algo como "[100, 500, 350, 220]" o múltiples arrays
	// Dividir por arrays individuales
	var arrays []string
	var currentArray strings.Builder
	inBrackets := 0

	for _, char := range content {
		switch char {
		case '[':
			inBrackets++
			currentArray.WriteRune(char)
		case ']':
			inBrackets--
			currentArray.WriteRune(char)
			if inBrackets == 0 {
				// Terminamos un array completo
				arrays = append(arrays, currentArray.String())
				currentArray.Reset()
			}
		case ',':
			if inBrackets == 0 {
				// Coma fuera de brackets, separador de arrays
				continue
			}
			currentArray.WriteRune(char)
		default:
			if inBrackets > 0 || char != ' ' {
				currentArray.WriteRune(char)
			}
		}
	}

	// Si queda algo en currentArray, agregarlo
	if currentArray.Len() > 0 {
		arrays = append(arrays, currentArray.String())
	}

	// Parsear cada array individual
	for _, arrayStr := range arrays {
		arrayStr = strings.TrimSpace(arrayStr)
		if arrayStr != "" {
			if row := p.parseArrayRow(arrayStr); len(row) > 0 {
				result = append(result, row)
			}
		}
	}

	return result
}

// extractAttribute extrae el valor de un atributo HTML-style del string
// unknownOpenerAttributes devuelve los nombres de atributo `k="v"` presentes
// en attrStr que no estén en known. attrStr es lo que va después del ':' de la
// apertura, ya sin el '>>' (p. ej. `bar title="Ventas" width="1200"`); el
// primer token es el tipo del chart, no un atributo, y no se considera.
func unknownOpenerAttributes(attrStr string, known ...string) []string {
	isKnown := make(map[string]bool, len(known))
	for _, k := range known {
		isKnown[k] = true
	}

	var unknown []string
	for _, m := range openerAttrRe.FindAllStringSubmatch(attrStr, -1) {
		if name := m[1]; !isKnown[name] {
			unknown = append(unknown, name)
		}
	}
	return unknown
}

// openerAttrRe matchea un par `nombre="valor"` o `nombre='valor'` de la línea
// de apertura. Deliberadamente exige comillas: es la forma que emite el
// formatter y la única que extractAttribute sabe leer, así que un
// `width=1200` sin comillas ya se ignoraba antes de este cambio y no se
// empieza a reportar acá (sería un cambio de comportamiento distinto, no el
// hueco que se está tapando).
var openerAttrRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_-]*)\s*=\s*["'][^"']*["']`)

// Ejemplo: `bar width="1200" height="600"` → extractAttribute(..., "width") = "1200"
func extractAttribute(str, attrName string) string {
	// Buscar patrón: attrName="value" o attrName='value'
	patterns := []string{
		attrName + `="`,
		attrName + `='`,
	}

	for _, pattern := range patterns {
		idx := strings.Index(str, pattern)
		if idx == -1 {
			continue
		}

		startIdx := idx + len(pattern)
		quote := str[idx+len(attrName)+1] // " o '

		// Buscar el cierre de la comilla
		endIdx := strings.IndexRune(str[startIdx:], rune(quote))
		if endIdx == -1 {
			continue
		}

		return str[startIdx : startIdx+endIdx]
	}

	return ""
}
