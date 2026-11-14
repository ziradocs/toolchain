// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// Una prueba por cada línea que el dialecto strict se tragaba en silencio en
// origin/main (ver la propiedad en strict_no_silent_drop_test.go). Cada caso
// corre en un SECTION de documento y en un SLIDE: los dos comparten el cuerpo
// strict. La línea marcada con "<" tiene que recibir el aviso con su ID, en
// su propia posición, citando el texto indicado, y como warning: ningún
// documento que compila en main deja de compilar.

type noDropRegression struct {
	name string
	// body son las líneas del cuerpo, ya con la sangría del contenedor (dos
	// espacios). La línea que lleva el aviso termina en "  <".
	body []string
	// rule y quote: el ID esperado y un fragmento del mensaje.
	rule, quote string
	// slideOnly / docOnly limitan el caso a un dialecto.
	slideOnly, docOnly bool
	// header, si no está vacío, reemplaza la línea SLIDE (casos del propio
	// contenedor).
	header string
	// front, si no está vacío, reemplaza el frontmatter (noDropDocHeader).
	front string
	// covers: el aviso puede empezar antes (en la apertura del elemento) y
	// cubrir la línea marcada con su endPosition, como los de un cuerpo YAML
	// o JSON ignorado entero.
	covers bool
	// severity, si no está vacío, es la severidad esperada en vez de warning
	// (un error que ya existía y que ahora también cubre su bloque).
	severity diagnostics.Severity
}

var noDropRegressions = []noDropRegression{
	// POINTS seguido de líneas más sangradas que no son items: el reporte
	// original. El item "uno" sobrevive y el resto desaparecía.
	{name: "points prose key", body: []string{"  POINTS", "    - uno", "    Importante:  <"}, rule: "POINTS001", quote: `"Importante:"`},
	{name: "points symbols", body: []string{"  POINTS", "    - uno", "    5 € + 3 %  <"}, rule: "POINTS001", quote: `"5 € + 3 %"`},
	{name: "points short", body: []string{"  POINTS", "    - uno", "    o B  <"}, rule: "POINTS001", quote: `"o B"`},
	{name: "points words", body: []string{"  POINTS", "    - uno", "    pie apple  <"}, rule: "POINTS001", quote: `"pie apple"`},
	{name: "points rule", body: []string{"  POINTS", "    - uno", "    ---  <"}, rule: "POINTS001", quote: `"---"`},
	{name: "points empty quotes", body: []string{"  POINTS", "    - uno", `    ""  <`}, rule: "POINTS001", quote: `"\"\""`},
	{name: "points markup", body: []string{"  POINTS", "    - uno", "    -> <> [] {} ## **  <"}, rule: "POINTS001", quote: `"-> <> [] {} ## **"`},
	{name: "points deeper continuation", body: []string{"  POINTS", "    - uno", "      sigue  <"}, rule: "POINTS001", quote: `"sigue"`},
	// Los items a la sangría del propio POINTS: lo que sigue a esa sangría
	// también es del POINTS, aunque sea otro elemento.
	{name: "points sibling at items indent", body: []string{"  POINTS", "  - uno", "  TEXT  <", "    hola"}, rule: "POINTS001", quote: `"TEXT"`},

	// IMAGE: toda llave que no es caption/title/label/fit/focus/context/bleed.
	{name: "image alt key", body: []string{`  IMAGE "foto.png"`, "    alt: perro  <"}, rule: "IMG005", quote: `IMAGE has no alt: property, so this line was ignored: "alt: perro"`},
	{name: "image src key", body: []string{`  IMAGE "foto.png"`, "    src: otra.png  <"}, rule: "IMG005", quote: `"src: otra.png"`},
	{name: "image width key", body: []string{`  IMAGE "foto.png" "perro"`, "    width: 50  <"}, rule: "IMG005", quote: `IMAGE has no width: property`},
	{name: "image height key", body: []string{`  IMAGE "foto.png" "perro"`, "    height: 20  <"}, rule: "IMG005", quote: `IMAGE has no height: property`},
	{name: "image align key", body: []string{`  IMAGE "foto.png" "perro"`, "    align: center  <"}, rule: "IMG005", quote: `IMAGE has no align: property`},
	{name: "image prose", body: []string{`  IMAGE "foto.png" "perro"`, "    un perro en la playa  <"}, rule: "IMG005", quote: `"un perro en la playa"`},
	{name: "image bleed", body: []string{`  IMAGE "foto.png" "perro"`, "    bleed: quizá  <"}, rule: "IMG005", quote: `"bleed: quizá"`},
	{name: "image third argument", body: []string{`  IMAGE "foto.png" "perro" "en la playa"  <`}, rule: "STRICT004", quote: `"en la playa"`},
	{name: "image glued keyword", body: []string{`  IMAGEN "foto.png" "perro"  <`}, rule: "STRICT004", quote: `"N"`},
	// Una propiedad de la sección justo después de un IMAGE, a su misma
	// sangría, la toma el IMAGE.
	{name: "image swallows section id", docOnly: true, body: []string{`  IMAGE "foto.png" "perro"`, "  id: portada  <"}, rule: "IMG005", quote: `IMAGE has no id: property`},

	// Texto en la línea de la palabra clave.
	{name: "text same line", body: []string{"  TEXT hola  <", "    mundo"}, rule: "STRICT004", quote: `The text after TEXT on this line was ignored: "hola"`},
	{name: "text colon", body: []string{"  TEXT: hola  <", "    mundo"}, rule: "STRICT004", quote: `The text after TEXT on this line was ignored: "hola"`},
	{name: "text glued", body: []string{"  TEXTO citado  <", "    mundo"}, rule: "STRICT004", quote: `"O citado"`},
	{name: "text only same line", body: []string{"  TEXT hola  <"}, rule: "STRICT004", quote: `"hola"`},
	{name: "points same line", body: []string{"  POINTS uno  <", "    - dos"}, rule: "STRICT004", quote: `"uno"`},
	{name: "quote same line", body: []string{"  QUOTE cita  <", "    texto"}, rule: "STRICT004", quote: `"cita"`},
	{name: "checklist same line", body: []string{"  CHECKLIST pendientes  <", "    [ ] uno"}, rule: "STRICT004", quote: `"pendientes"`},
	{name: "table same line", body: []string{"  TABLE ventas  <", "    | a | b |"}, rule: "STRICT004", quote: `"ventas"`},
	{name: "code glued", body: []string{"  CODEX python  <", "    print(1)"}, rule: "STRICT004", quote: `"X"`},
	{name: "slide header", slideOnly: true, header: "SLIDE content extra words  <", body: []string{"  TEXT", "    hola"}, rule: "STRICT004", quote: `"extra words"`},

	// QUOTE: un AUTHOR: repetido pisa al primero.
	{name: "quote repeated author", body: []string{"  QUOTE", "    texto", "    AUTHOR: Ana  <", "    AUTHOR: Beto"}, rule: "QUOTE001", quote: `"AUTHOR: Ana"`},
	{name: "quote repeated source", body: []string{"  QUOTE", "    texto", "    SOURCE: libro  <", "    SOURCE: revista"}, rule: "QUOTE001", quote: `"SOURCE: libro"`},

	// CHECKLIST: un ítem sin texto.
	{name: "checklist empty item", body: []string{"  CHECKLIST", "    [ ] uno", "    [ ]  <"}, rule: "CHECKLIST002", quote: `"[ ]"`},

	// TABLE: líneas que ninguna rama lee.
	{name: "table prose", body: []string{"  TABLE", "    | a | b |", "    nota al pie  <"}, rule: "TABLE008", quote: `"nota al pie"`},
	{name: "table key", body: []string{"  TABLE", "    | a | b |", "    title: Ventas  <"}, rule: "TABLE008", quote: `"title: Ventas"`},
	{name: "table row indent", body: []string{"  TABLE", "    headers: [a, b]", "    rows:", "        [c, d]  <"}, rule: "TABLE008", quote: `six spaces`},
	{name: "table headers not array", body: []string{"  TABLE", "    headers: a, b  <", "    | c | d |"}, rule: "TABLE008", quote: `"headers: a, b"`},
	{name: "table row unclosed", body: []string{"  TABLE", "    headers: [a, b]", "    rows:", "      [c, d  <"}, rule: "TABLE008", quote: `"[c, d"`},

	// Tabla Markdown strict: filas que no llegan al AST.
	{name: "markdown row unclosed header", body: []string{"  | a  <"}, rule: "TABLE008", quote: `"| a"`},
	{name: "markdown empty row", body: []string{"  |  <"}, rule: "TABLE008", quote: `"|"`},
	{name: "markdown separator lookalike", body: []string{"  | a | b |", "  | x --- y | z |  <"}, rule: "TABLE008", quote: `"| x --- y | z |"`},

	// <<chart>>: lo anidado bajo una llave que no lo lee, el vocabulario que
	// no se traduce, un options: que no es YAML y un arreglo que no lo es.
	{name: "chart nested line", body: []string{"  <<chart: bar>>", `    data: [["a", 1]]`, `    series: ["s"]`, "      o B  <"}, rule: "CHART007", quote: `nested under series:`},
	{name: "chart unsupported key", body: []string{"  <<chart: bar>>", `    data: [["a", 1]]`, "    backgroundColor: red  <"}, rule: "CHART007", quote: `"backgroundColor: red"`},
	{name: "chart data not array", body: []string{"  <<chart: bar>>", "    data: muchos  <"}, rule: "CHART007", quote: `"data: muchos"`},
	{name: "chart options invalid", body: []string{"  <<chart: bar>>", `    data: [["a", 1]]`, "    options:  <", "      plugins: true", "      plugins: false"}, rule: "CHART007", quote: `could not be read (mapping key "plugins" already defined), so`},

	// Bloques YAML: cada llave desconocida, en su línea.
	{name: "metric unknown key", body: []string{"  <<metric>>", "    label: Ventas", "    Importante:  <", "      alt: perro", "  <<end>>"}, rule: "METRIC006", quote: `"Importante:"`},
	{name: "quiz unknown key", body: []string{"  <<quiz>>", "    question: ¿Qué?", "    options: [a, b]", "    answer: 0", "    alt: perro  <", "  <<end>>"}, rule: "QUIZ007", quote: `"alt: perro"`},
	{name: "poll unknown key", body: []string{"  <<poll>>", "    question: ¿Qué?", "    options: [a, b]", "    zeta: 1  <", "  <<end>>"}, rule: "POLL007", quote: `"zeta: 1"`},

	// :::code-group: cada línea fuera de una cerca, no solo la primera.
	{name: "code group second stray", body: []string{"  :::code-group", "  ```js [a]", "  x()", "  ```", "  primera", "  segunda  <", "  :::"}, rule: "CODEGROUP003", quote: `"segunda"`},

	// Directivas: texto que no llega a ningún parámetro.
	{name: "directive no key", body: []string{"  @transition fade  <"}, rule: "DIRECTIVE004", quote: `"fade"`},
	{name: "directive trailing word", body: []string{`  @highlight color="x" junk  <`}, rule: "DIRECTIVE004", quote: `"junk"`},
	{name: "directive bare at", body: []string{"  @  <"}, rule: "DIRECTIVE004", quote: `"@"`},

	// Un ":::" que no abre ni cierra nada.
	{name: "bare block marker", body: []string{"  :::  <", "  TEXT", "    hola"}, rule: "STRICT003", quote: `":::"`},

	// Cuerpos YAML y JSON: lo que el lector deja afuera sin error, y los
	// avisos de cuerpo ignorado que ahora cubren todo el cuerpo.
	{name: "cells under-indented rows", body: []string{"  TABLE", "    cells:", "        -", "      - [{content: a}, {content: b}]  <"}, rule: "TABLE008", quote: `YAML reader stopped`},
	// Una comilla dentro de una palabra no abre un escalar entre comillas: el
	// cierre de la fila se encuentra y la fila descartada de abajo se avisa.
	{name: "cells apostrophe in a word", body: []string{"  TABLE", "    cells:", "        - [{content: Don't}]", "      - [{content: b}]  <"}, rule: "TABLE008", quote: `YAML reader stopped`},
	{name: "cells inch mark in a word", body: []string{"  TABLE", "    cells:", `        - [{content: 5" screen}]`, "      - [{content: b}]  <"}, rule: "TABLE008", quote: `YAML reader stopped`},
	{name: "cells quote after a space in a word", body: []string{"  TABLE", "    cells:", "        - [{content: rock 'n roll}]", "      - [{content: b}]  <"}, rule: "TABLE008", quote: `YAML reader stopped`},
	{name: "cells double quote after a space", body: []string{"  TABLE", "    cells:", `        - [{content: a "b}]`, "      - [{content: b}]  <"}, rule: "TABLE008", quote: `YAML reader stopped`},
	{name: "cells item at key indent", body: []string{"  TABLE", "    cells:", "      - [{content: a}, {content: b}]", "    - [{content: c}, {content: d}]  <"}, rule: "TABLE008", quote: `YAML reader stopped`},
	{name: "table rows document marker", body: []string{"  TABLE", "    tableRows:", "      - cells: [{content: a}]", "    ---  <"}, rule: "TABLE008", quote: `"---"`},
	{name: "table rows invalid", covers: true, severity: diagnostics.Error, body: []string{"  TABLE", "    tableRows:", "      - cells: [{content: a, falso: 1}]  <"}, rule: "", quote: "invalid tableRows YAML"},
	{name: "cells invalid", covers: true, body: []string{"  TABLE", "    cells:", "      - [{content: a}", "      - nope: [  <"}, rule: "TABLE004", quote: `not valid YAML`},
	{name: "chart json invalid", covers: true, body: []string{"  <<chart: bar>>", `    {"labels": ["a"],`, "    nota  <", `    "datasets": []}`}, rule: "CHART002", quote: "inválido"},
	{name: "chart combo unknown key", body: []string{"  <<chart: combo>>", "    data:", `      labels: ["a"]`, "      series:", `        - name: "s"`, "          values: [1]", "          color: red  <"}, rule: "CHART007", quote: `"color: red"`},
	{name: "chart type in body", body: []string{"  <<chart: bar>>", `    data: [["a", 1]]`, "    type: line  <"}, rule: "CHART007", quote: `<<chart: line>>`},
	{name: "directive highlight example", body: []string{`  @highlight color="x" junk  <`}, rule: "DIRECTIVE004", quote: `@highlight color="yellow"`},
	{name: "quiz flow mapping", body: []string{"  <<quiz>>", "    {question: Q, options: [a, b], answer: 0, zeta: 1}  <", "  <<end>>"}, rule: "QUIZ007", quote: `that key and its value were ignored; the rest of this line is read`},
	{name: "metric invalid yaml", covers: true, body: []string{"  <<metric>>", "    label: [roto", "    value: 1  <", "  <<end>>"}, rule: "METRIC002", quote: "inválido"},

	// Una línea de guiones sin "|" justo después del encabezado de una tabla
	// Markdown strict se toma por separador, aunque no tenga una celda por
	// columna: lo que el autor escribió como línea suelta no estaba en
	// ningún lado. Pasa igual con la tabla que sigue a un TABLE cuyo cuerpo
	// ya terminó, y en una columna tipada.
	{name: "markdown table bare dashes", body: []string{"  | a | b |", "  ---  <"}, rule: "TABLE008", quote: `has no pipes but is read as the table's separator row: "---"`},
	{name: "markdown table long dashes", body: []string{"  | a | b |", "  -----  <", "  | c | d |"}, rule: "TABLE008", quote: `"-----"`},
	{name: "markdown table bare aligned dashes", body: []string{"  | a | b |", "  :---:  <"}, rule: "TABLE008", quote: `":---:"`},
	{name: "markdown table after a TABLE body", body: []string{"  TABLE", `    caption: "x"`, "  | a | b |", "  ---  <"}, rule: "TABLE008", quote: `has no pipes`},
	{name: "markdown table in a typed column", body: []string{"  <<grid>>", "  <<column typed>>", "    | a | b |", "    ---  <", "  <<end>>"}, rule: "TABLE008", quote: `has no pipes`},

	// <<math>>: un caption: o label: repetido pisa al primero (QUOTE001 es
	// el mismo caso con AUTHOR:/SOURCE:).
	{name: "math repeated caption", body: []string{"  <<math>>", "    x + 1", `    caption: "primera"  <`, `    caption: "segunda"`, "  <<end>>"}, rule: "MATH001", quote: `"caption: \"primera\""`},
	{name: "math repeated label", body: []string{"  <<math>>", "    x + 1", "    label: eq:a  <", "    label: eq:b", "  <<end>>"}, rule: "MATH001", quote: `The later "label: eq:b" replaces it`},
	{name: "math source repeated caption", front: noDropMathSource, body: []string{"  <<math>>", "    x + 1", `    caption: "primera"  <`, `    caption: "segunda"`, "  <<end>>"}, rule: "MATH001", quote: `"caption: \"primera\""`},
	{name: "math unclosed repeated caption", body: []string{"  <<math>>", "    x + 1", `    caption: "primera"  <`, `    caption: "segunda"`}, rule: "MATH001", quote: `"caption: \"primera\""`},

	// El error de un elemento dentro de un ":::" se perdía: el elemento
	// salía sin esa parte y sin ningún diagnóstico, con el bloque cerrado o
	// sin cerrar.
	{name: "code extra argument in unclosed callout", body: []string{"  :::note", "  CODE python main.py extra30  <", "    print(1)"}, rule: "SPECIAL003", quote: `unexpected "extra30"`},
	{name: "code extra argument in callout", body: []string{"  :::note", "  CODE python main.py extra30  <", "    print(1)", "  :::"}, rule: "SPECIAL003", quote: `inside the ":::note" block`},
	{name: "code extra argument in nested callout", body: []string{"  :::note", "  :::warning", "  CODE python main.py extra30  <", "    print(1)", "  :::", "  :::"}, rule: "SPECIAL003", quote: `inside the ":::warning" block`},
	{name: "fence extra argument in typed callout", body: []string{"  :::info{typed}", "  ```python main.py extra30  <", "  print(1)", "  ```", "  :::"}, rule: "SPECIAL003", quote: `"python main.py extra30"`},
	{name: "mermaid attribute in callout", body: []string{"  :::note", `  <<mermaid theme="dark">>  <`, "    graph TD", "  :::"}, rule: "SPECIAL003", quote: `"theme"`},
	{name: "plantuml attribute in callout", body: []string{"  :::note", `  <<plantuml theme="x">>  <`, "    @startuml", "    A -> B", "    @enduml", "  :::"}, rule: "SPECIAL003", quote: `unknown <<plantuml>> attribute "theme"`},
	{name: "map marker in callout", body: []string{"  :::note", "  <<map>>  <", "    type: world", "    markers:", "      - lat: 1", `        label: "x"`, "  :::"}, rule: "SPECIAL003", quote: `has no lng`},
	{name: "code group in callout", body: []string{"  :::note", "  :::code-group  <", "  ```js [a]", "  x()", "  ```", "  suelta", "  :::", "  :::"}, rule: "SPECIAL003", quote: `outside a code fence: "suelta"`},

	// Lo que encontró la propiedad extendida (propiedades repetidas y filas
	// con guiones): una propiedad repetida de TABLE, IMAGE o <<chart>> pisa
	// a la anterior, y una fila con "---" de una tabla Markdown dentro de un
	// ":::" se salta como separador.
	{name: "table repeated caption", body: []string{"  TABLE", "    | a | b |", `    caption: "uno"  <`, `    caption: "dos"`}, rule: "TABLE008", quote: `This caption: line of the TABLE was ignored: "caption: \"uno\""`},
	{name: "table repeated label", body: []string{"  TABLE", "    | a | b |", "    label: tbl:a  <", "    label: tbl:b"}, rule: "TABLE008", quote: `The later "label: tbl:b" replaces it`},
	{name: "image repeated caption", body: []string{`  IMAGE "foto.png" "perro"`, `    caption: "uno"  <`, `    caption: "dos"`}, rule: "IMG005", quote: `This caption: line of the IMAGE was ignored`},
	{name: "image repeated label", body: []string{`  IMAGE "foto.png" "perro"`, "    label: fig:a  <", `    caption: "uno"`, "    label: fig:b"}, rule: "IMG005", quote: `"label: fig:a"`},
	{name: "chart repeated data", body: []string{"  <<chart: bar>>", `    data: [["a", 1]]  <`, `    data: [["b", 2]]`}, rule: "CHART007", quote: `This data: line of the chart was ignored`},
	{name: "chart repeated series", body: []string{"  <<chart: bar>>", `    data: [["a", 1]]`, `    series: ["s"]  <`, `    series: ["t"]`}, rule: "CHART007", quote: `The later "series: [\"t\"]" replaces it`},
	{name: "chart repeated title", body: []string{"  <<chart: bar>>", `    data: [["a", 1]]`, `    title: "uno"  <`, `    title: "dos"`}, rule: "CHART007", quote: `"title: \"uno\""`},
	{name: "chart repeated multi-line series", body: []string{"  <<chart: bar>>", `    data: [["a", 1]]`, "    series: [  <", `      "s"`, "    ]", `    series: ["t"]`}, rule: "CHART007", quote: `and the lines of its value below it were ignored`},
	{name: "markdown row with dashes in callout", body: []string{"  :::note", "  | a | b |", "  | x --- y | z |  <", "  :::"}, rule: "TABLE008", quote: `has dashes in it, so its cells were ignored: "| x --- y | z |"`},
	{name: "markdown row with dashes in typed callout", body: []string{"  :::info{typed}", "  | a | b |", "  | --- | --- |", "  | 1 | 2 |", "  | x --- y | z |  <", "  :::"}, rule: "TABLE008", quote: `"| x --- y | z |"`},
}

// noDropMathSource activa la lectura literal de <<math>> (parseLiteral).
const noDropMathSource = "---\nmode: strict\ntitle: \"Doc\"\nast_capabilities: [\"math-source-v1\"]\n---\n"

// Los bloques que flex lee igual que strict dan los mismos avisos en un
// documento flex: <<math>>, el bloque TABLE, <<chart>>, la tabla Markdown y
// un elemento dentro de un ":::".
func TestStrictNoSilentDrop_FlexSharedBlocks(t *testing.T) {
	const front = "---\nmode: flex\ntitle: \"Doc\"\n---\n# Sec\n\n"
	cases := []struct {
		name, body, rule, quote string
		line                    int
	}{
		{"math repeated caption", "<<math>>\nx + 1\ncaption: \"a\"\ncaption: \"b\"\n<<end>>\n", "MATH001", `"caption: \"a\""`, 9},
		{"table repeated caption", "TABLE\n  | a | b |\n  caption: \"a\"\n  caption: \"b\"\n", "TABLE008", `This caption: line of the TABLE was ignored`, 9},
		{"chart repeated data", "<<chart: bar>>\n  data: [[\"a\", 1]]\n  data: [[\"b\", 2]]\n", "CHART007", `This data: line of the chart was ignored`, 8},
		{"markdown row with dashes", "| a | b |\n| --- | --- |\n| 1 | 2 |\n| x --- y | z |\n", "TABLE008", `"| x --- y | z |"`, 10},
		{"fence extra argument in callout", ":::note\n```python main.py extra30\nprint(1)\n```\n:::\n", "SPECIAL003", `"python main.py extra30"`, 8},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, diags := New(util.NewNoop()).ParseDocument(front+c.body, "flex.doclang")
			for _, d := range diags {
				if d.RuleID == c.rule && d.Position.Line == c.line && strings.Contains(d.Message, c.quote) {
					if !d.IsWarning() {
						t.Errorf("%s is %s", c.rule, d.Severity)
					}
					return
				}
			}
			t.Errorf("want %s on line %d, got %v", c.rule, c.line, diags)
		})
	}
}

// buildNoDropRegression arma el documento del caso en el dialecto pedido y
// devuelve la línea (1-based) que debe llevar el aviso.
func buildNoDropRegression(c noDropRegression, slide bool) (string, int) {
	front := noDropDocHeader
	if c.front != "" {
		front = c.front
	}
	var b strings.Builder
	b.WriteString(front)
	line := strings.Count(front, "\n")
	want := 0
	mark := func(s string) string {
		line++
		if t, ok := strings.CutSuffix(s, "  <"); ok {
			want = line
			return t
		}
		return s
	}
	switch {
	case slide && c.header != "":
		b.WriteString(mark(c.header) + "\n")
		b.WriteString(mark(`  title: "T"`) + "\n")
	case slide:
		b.WriteString(mark("SLIDE content") + "\n")
		b.WriteString(mark(`  title: "T"`) + "\n")
	default:
		b.WriteString(mark(`SECTION "S"`) + "\n")
	}
	for _, l := range c.body {
		b.WriteString(mark(l) + "\n")
	}
	return b.String(), want
}

func TestStrictNoSilentDrop_Regressions(t *testing.T) {
	for _, c := range noDropRegressions {
		for _, slide := range []bool{false, true} {
			if (c.slideOnly && !slide) || (c.docOnly && slide) {
				continue
			}
			dialect := "document"
			if slide {
				dialect = "slide"
			}
			t.Run(dialect+"/"+c.name, func(t *testing.T) {
				src, want := buildNoDropRegression(c, slide)
				p := New(util.NewNoop())
				var diags []diagnostics.Diagnostic
				if slide {
					_, diags = p.Parse(src, "case.slidelang")
				} else {
					_, diags = p.ParseDocument(src, "case.doclang")
				}
				var got []string
				for _, d := range diags {
					got = append(got, fmt.Sprintf("%d:%s:%s %s", d.Position.Line, d.Severity, d.RuleID, d.Message))
					if d.RuleID != c.rule || !strings.Contains(d.Message, c.quote) {
						continue
					}
					end := d.Position.Line
					if d.EndPosition != nil {
						end = d.EndPosition.Line
					}
					if d.Position.Line != want && (!c.covers || d.Position.Line >= want || want > end) {
						continue
					}
					if severity := c.severity; severity == "" && !d.IsWarning() || severity != "" && d.Severity != severity {
						t.Errorf("%s is %s", c.rule, d.Severity)
					}
					if !strings.Contains(d.Message, c.quote) {
						t.Errorf("%s message %q does not contain %q", c.rule, d.Message, c.quote)
					}
					return
				}
				t.Errorf("want %s on line %d\n--- source ---\n%s--- diagnostics ---\n%s", c.rule, want, src, strings.Join(got, "\n"))
			})
		}
	}
}

// Los avisos nuevos son de las líneas perdidas, no de la sintaxis válida: un
// cuerpo que lo lee todo no recibe ninguno. Incluye las formas que un primer
// borrador de este cambio marcaba de más (el "]" que cierra un `rows: [`
// multilínea, `CODE{verbatim}`, el bloque options: válido).
func TestStrictNoSilentDrop_ValidBodiesStayQuiet(t *testing.T) {
	body := []string{
		"  TEXT",
		"    hola mundo",
		"  POINTS",
		"    - uno",
		"      - dos",
		"  CHECKLIST",
		"    - [ ] tarea",
		"    [x] hecha",
		"  QUOTE",
		"    cita",
		"    AUTHOR: Ana",
		"    SOURCE: libro",
		"  CODE{verbatim} python main.py",
		"    print(1)",
		`  IMAGE "foto.png" "perro"`,
		`    caption: "En la playa"`,
		"    label: fig:perro",
		"    fit: cover",
		"    focus: 50% 50%",
		"    bleed: true",
		"  TABLE",
		`    headers: ["a", "b"]`,
		"    rows: [",
		`      ["c", "d"],`,
		`      ["e", "f"]`,
		"    ]",
		`    caption: "Tabla"`,
		"  | x | y |",
		"  | :-- | --: |",
		"  | 1 | 2 |",
		"  <<chart: bar>>",
		`    data: [["a", 1]]`,
		`    series: ["s"]`,
		"    options:",
		"      plugins:",
		"        title:",
		"          text: Ventas",
		"    options:",
		"      # sin opciones todavía",
		"  <<metric>>",
		"    label: Ventas",
		"    value: 10",
		"  <<end>>",
		// Un ":" pegado a la palabra clave no pierde texto.
		"  POINTS:",
		"    - uno",
		"  TEXT:",
		"    hola",
		"  CHECKLIST:",
		"    [ ] tarea",
		"  CODE: python",
		"    print(2)",
		`  @transition type="fade" duration="300"`,
		"  :::info Nota",
		"    texto",
		"  :::",
		// Un caption: y un label: por <<math>>, un CODE completo dentro de un
		// ":::" y el separador sin "|" de una tabla de una sola columna (el de
		// GFM) no pierden nada.
		"  <<math>>",
		"    e^{i\\pi} + 1 = 0",
		`    caption: "Euler"`,
		"    label: eq:euler",
		"  <<end>>",
		"  :::note",
		"  CODE python main.py",
		"    print(3)",
		"  :::",
		"  | z |",
		"  ---",
		"  | 3 |",
	}
	for _, slide := range []bool{false, true} {
		src, _ := buildNoDropRegression(noDropRegression{body: body}, slide)
		p := New(util.NewNoop())
		var diags []diagnostics.Diagnostic
		if slide {
			_, diags = p.Parse(src, "quiet.slidelang")
		} else {
			_, diags = p.ParseDocument(src, "quiet.doclang")
		}
		for _, d := range diags {
			t.Errorf("slide=%v: unexpected %d:%s %s", slide, d.Position.Line, d.RuleID, d.Message)
		}
	}
}

// Un marcador node-id se quita antes de parsear, y las posiciones se vuelven a
// llevar a la línea del archivo; el texto de un aviso no pasa por ese ajuste,
// así que no puede nombrar líneas por número. El aviso queda en la línea
// ignorada del archivo, con su rango.
func TestStrictNoSilentDrop_NodeIDKeepsLines(t *testing.T) {
	src := noDropDocHeader + "SECTION \"S\"\n" +
		"  <!-- node-id: Cita -->\n" + // 6
		"  QUOTE\n" + // 7
		"    cita\n" + // 8
		"    AUTHOR: Ana\n" + // 9
		"    AUTHOR: Beto\n" + // 10
		"  <!-- node-id: Kpi -->\n" + // 11
		"  <<metric>>\n" + // 12
		"    label: L\n" + // 13
		"    notas:\n" + // 14
		"      - x\n" + // 15
		"  <<end>>\n" + // 16
		"  <!-- node-id: Euler -->\n" + // 17
		"  <<math>>\n" + // 18
		"    x + 1\n" + // 19
		"    caption: \"uno\"\n" + // 20
		"    caption: \"dos\"\n" + // 21
		"  <<end>>\n" + // 22
		"  <!-- node-id: Nota -->\n" + // 23
		"  :::note\n" + // 24
		"  CODE python main.py extra\n" + // 25
		"    print(1)\n" + // 26
		"  :::\n" + // 27
		"  <!-- node-id: Serie -->\n" + // 28
		"  <<chart: bar>>\n" + // 29
		"    data: [[\"a\", 1]]\n" + // 30
		"    series: [\n" + // 31
		"      \"s\"\n" + // 32
		"    ]\n" + // 33
		"    series: [\"t\"]\n" // 34
	_, diags := New(util.NewNoop()).ParseDocument(src, "ids.doclang")
	want := map[string][2]int{"QUOTE001": {9, 9}, "METRIC006": {14, 15}, "MATH001": {20, 20}, "SPECIAL003": {25, 25}, "CHART007": {31, 33}}
	lineNumber := regexp.MustCompile(`\bline \d+`)
	for _, d := range diags {
		w, ok := want[d.RuleID]
		if !ok {
			continue
		}
		delete(want, d.RuleID)
		end := d.Position.Line
		if d.EndPosition != nil {
			end = d.EndPosition.Line
		}
		if d.Position.Line != w[0] || end != w[1] {
			t.Errorf("%s: want lines %d-%d, got %d-%d", d.RuleID, w[0], w[1], d.Position.Line, end)
		}
		if m := lineNumber.FindString(d.Message); m != "" {
			t.Errorf("%s names %q in its text: %s", d.RuleID, m, d.Message)
		}
	}
	for id := range want {
		t.Errorf("missing %s in %v", id, diags)
	}
}

// `CODE: python` lee su lenguaje como siempre, y un encabezado Markdown sin
// cierre que además contiene "---" da un solo aviso, no dos.
func TestStrictNoSilentDrop_HeaderEdges(t *testing.T) {
	src := noDropDocHeader + "SECTION \"S\"\n  CODE: python\n    print(1)\n  | a --- b\n"
	tree, diags := New(util.NewNoop()).ParseDocument(src, "edges.doclang")
	code, ok := tree.ContentBlocks[0].Elements[0].(*ast.CodeElement)
	if !ok || code.Language != "python" {
		t.Fatalf("CODE: python: got %#v", tree.ContentBlocks[0].Elements[0])
	}
	n := 0
	for _, d := range diags {
		if d.RuleID == "TABLE008" && d.Position.Line == 8 {
			n++
		}
		if d.RuleID == "STRICT004" {
			t.Errorf("unexpected STRICT004: %s", d.Message)
		}
	}
	if n != 1 {
		t.Errorf("want one TABLE008 on line 8, got %d: %v", n, diags)
	}
}

// Una fila de flujo o un escalar entre comillas partidos en varias líneas son
// parte de la tabla aunque la continuación vaya menos sangrada que el guion:
// yaml.v3 los lee, y el aviso de líneas descartadas no puede salir. Son los
// tres casos que la primera versión, que miraba la sangría del texto, marcaba
// de más.
func TestStrictNoSilentDrop_SplitTableRowsStayQuiet(t *testing.T) {
	cases := map[string][]string{
		"tableRows flow split, continuation less indented": {
			"  TABLE", "    tableRows:",
			`      - cells: [{content: "a1x0", header: true}, {content: "b1x0", header: true}]`,
			`      - cells: [{content: "a1x1"},`,
			`     {content: "b1x1"}]`,
		},
		"cells flow split, continuation at 6": {
			"  TABLE", "    cells:",
			`        - [{content: "Name", header: true},`,
			`      {content: "Role", header: true}]`,
			`        - [{content: "Ana"}, {content: "Dev"}]`,
		},
		"cells quoted scalar split": {
			"  TABLE", "    cells:",
			`        - [{content: "Long`,
			`      header", header: true}]`,
			`        - [{content: "x"}]`,
		},
	}
	for name, body := range cases {
		src, _ := buildNoDropRegression(noDropRegression{body: body}, false)
		tree, diags := New(util.NewNoop()).ParseDocument(src, "split.doclang")
		for _, d := range diags {
			t.Errorf("%s: unexpected %d:%s %s", name, d.Position.Line, d.RuleID, d.Message)
		}
		if len(tree.ContentBlocks) == 0 || len(tree.ContentBlocks[0].Elements) == 0 {
			t.Fatalf("%s: no table", name)
		}
		raw, _ := json.Marshal(tree.ContentBlocks[0].Elements[0])
		for _, w := range []string{"b1x1", "Role", "Long header"} {
			if strings.Contains(strings.Join(body, "\n"), strings.Fields(w)[0]) && !strings.Contains(string(raw), w) {
				t.Errorf("%s: %q missing from the table", name, w)
			}
		}
	}
}

// Propiedad: una tabla cells: o tableRows: que yaml.v3 lee sin error, con sus
// filas partidas en varias líneas a sangrías al azar (más adentro que la
// llave, para seguir en el bloque) y algún escalar entre comillas partido,
// no recibe ningún TABLE008, y todas sus celdas llegan al AST.
func TestStrictNoSilentDrop_SplitTableRowsProperty(t *testing.T) {
	valid := 0
	for seed := int64(1); seed <= 400; seed++ {
		rng := rand.New(rand.NewSource(seed))
		key := "cells:"
		if rng.Intn(2) == 0 {
			key = "tableRows:"
		}
		dash := []int{6, 8}[rng.Intn(2)]
		var lines []string
		var want []string
		for r := 0; r < 1+rng.Intn(4); r++ {
			var cells []string
			for c := 0; c < 1+rng.Intn(3); c++ {
				word := fmt.Sprintf("r%dc%ds%d", r, c, seed)
				want = append(want, word)
				if rng.Intn(4) == 0 {
					// Escalar entre comillas partido: la segunda palabra va en
					// otra línea.
					cells = append(cells, fmt.Sprintf("{content: \"%s\n%sz%s\"}", word, strings.Repeat(" ", 5+rng.Intn(8)), word))
					continue
				}
				if rng.Intn(4) == 0 {
					// Una comilla dentro de una palabra es texto plano.
					cells = append(cells, fmt.Sprintf("{content: Don't%s}", word))
					continue
				}
				cells = append(cells, fmt.Sprintf("{content: %s}", word))
			}
			// Cada coma puede partir la fila en otra línea.
			var b strings.Builder
			for c, cell := range cells {
				if c > 0 {
					b.WriteString(",")
					if rng.Intn(2) == 0 {
						b.WriteString("\n" + strings.Repeat(" ", 5+rng.Intn(8)))
					} else {
						b.WriteString(" ")
					}
				}
				b.WriteString(cell)
			}
			prefix := "- ["
			if key == "tableRows:" {
				prefix = "- cells: ["
			}
			lines = append(lines, strings.Split(strings.Repeat(" ", dash)+prefix+b.String()+"]", "\n")...)
		}
		src := noDropDocHeader + "SECTION \"S\"\n  TABLE\n    " + key + "\n" + strings.Join(lines, "\n") + "\n"
		tree, diags := New(util.NewNoop()).ParseDocument(src, "prop.doclang")
		bad := false
		for _, d := range diags {
			if d.IsError() || d.RuleID == "TABLE004" {
				bad = true // yaml.v3 no la leyó: no es una tabla válida
			}
		}
		if bad {
			continue
		}
		valid++
		for _, d := range diags {
			if d.RuleID == "TABLE008" {
				t.Errorf("seed %d: unexpected TABLE008 on line %d: %s\n%s", seed, d.Position.Line, d.Message, src)
			}
		}
		raw, _ := json.Marshal(tree)
		for _, w := range want {
			if !strings.Contains(string(raw), w) {
				t.Errorf("seed %d: cell %q missing from the AST\n%s", seed, w, src)
			}
		}
	}
	t.Logf("%d of 400 generated tables are valid YAML", valid)
	if valid < 200 {
		t.Fatalf("only %d of 400 generated tables were valid YAML: the generator drifted", valid)
	}
}
