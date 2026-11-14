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
	{name: "chart options invalid", body: []string{"  <<chart: bar>>", `    data: [["a", 1]]`, "    options:  <", "      plugins: true", "      plugins: false"}, rule: "CHART007", quote: `not valid YAML (mapping key "plugins" already defined), so`},

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
}

// buildNoDropRegression arma el documento del caso en el dialecto pedido y
// devuelve la línea (1-based) que debe llevar el aviso.
func buildNoDropRegression(c noDropRegression, slide bool) (string, int) {
	var b strings.Builder
	b.WriteString(noDropDocHeader)
	line := strings.Count(noDropDocHeader, "\n")
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
		"  <<end>>\n"
	_, diags := New(util.NewNoop()).ParseDocument(src, "ids.doclang")
	want := map[string][2]int{"QUOTE001": {9, 9}, "METRIC006": {14, 15}}
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
