// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strings"
	"testing"

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
	{name: "text colon", body: []string{"  TEXT: hola  <", "    mundo"}, rule: "STRICT004", quote: `": hola"`},
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
	{name: "chart options invalid", body: []string{"  <<chart: bar>>", `    data: [["a", 1]]`, "    options:  <", "      plugins: true", "      plugins: false"}, rule: "CHART007", quote: `through line`},

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
					if d.RuleID != c.rule || d.Position.Line != want {
						continue
					}
					if !d.IsWarning() {
						t.Errorf("%s is %s, want a warning", c.rule, d.Severity)
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
		"  <<metric>>",
		"    label: Ventas",
		"    value: 10",
		"  <<end>>",
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
