// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// Propiedad del dialecto strict: toda línea no vacía que el autor escribió
// termina en el AST (contenido o una propiedad que el elemento de verdad
// lee) o en un diagnóstico anclado a esa línea. Ningún parser de elemento ni
// ningún contenedor (SECTION, SLIDE, columna tipada, callout tipado) puede
// tragarse texto sin decirlo.
//
// El generador es determinista (semillas fijas): mezcla las líneas válidas de
// cada plantilla con líneas sueltas (prosa, claves sin valor, símbolos,
// palabras repetidas, tokens de un carácter) a la sangría del cuerpo, más
// adentro o más afuera, y también agrega texto en la línea de la palabra
// clave.

// noDropLine es una línea de plantilla: su texto, su sangría relativa a la
// cabecera del elemento y los tokens que tienen que llegar al AST (vacío para
// la sintaxis pura: POINTS, <<end>>, :::).
type noDropLine struct {
	text    string
	indent  int
	payload []string
	// exact: los tokens tienen que aparecer como palabras enteras, sin la
	// búsqueda por subcadena (ver takeToken). Es el caso de las líneas que se
	// parecen a sintaxis: "-", "@" o "[ ]" no pueden darse por encontradas
	// dentro de otro texto.
	exact bool
}

type noDropTemplate struct {
	name string
	// lines[0] es la cabecera. La última línea es el cierre cuando closer es
	// true: las líneas sueltas van antes de ella.
	lines  []noDropLine
	closer bool
	// headerSuffix: la cabecera no lleva argumentos, así que un texto
	// agregado a ella (`TEXT hola`, `TEXT: hola`) es contenido que hay que
	// reportar.
	headerSuffix bool
	// extraArgs: la cabecera ya lleva sus argumentos (CODE <lenguaje>), y
	// uno de más (`CODE python main.py extra`) es texto que hay que
	// reportar.
	extraArgs bool
	// keyword: la palabra clave de la cabecera. El despacho strict la
	// reconoce por prefijo, así que una letra pegada (`TEXTZ`, `CODEZ
	// python`) se lee como la palabra clave y se pierde: hay que reportarla.
	keyword string
	// slideOnly: la plantilla solo existe dentro de un SLIDE.
	slideOnly bool
	// yaml: el cuerpo es YAML, donde " #" abre un comentario. Lo que sigue a
	// un comentario no es texto del autor por la gramática, así que no se
	// exige en el AST.
	yaml bool
}

func tok(s ...string) []string { return s }

var noDropTemplates = []noDropTemplate{
	{name: "POINTS", headerSuffix: true, keyword: "POINTS", lines: []noDropLine{
		{"POINTS", 0, nil, false}, {"- alfa1", 2, tok("alfa1"), false}, {"- beta1", 2, tok("beta1"), false}, {"- gamma1", 4, tok("gamma1"), false}}},
	{name: "CHECKLIST", headerSuffix: true, keyword: "CHECKLIST", lines: []noDropLine{
		{"CHECKLIST", 0, nil, false}, {"- [ ] alfa2", 2, tok("alfa2"), false}, {"- [x] beta2", 2, tok("beta2"), false}}},
	{name: "TEXT", headerSuffix: true, keyword: "TEXT", lines: []noDropLine{
		{"TEXT", 0, nil, false}, {"alfa3 beta3", 2, tok("alfa3", "beta3"), false}, {"gamma3", 2, tok("gamma3"), false}}},
	{name: "QUOTE", headerSuffix: true, keyword: "QUOTE", lines: []noDropLine{
		{"QUOTE", 0, nil, false}, {"alfa4 beta4", 2, tok("alfa4", "beta4"), false}, {"AUTHOR: gamma4", 2, tok("gamma4"), false}}},
	{name: "CODE", keyword: "CODE", extraArgs: true, lines: []noDropLine{
		{"CODE python", 0, tok("python"), false}, {"print(alfa5)", 2, tok("print(alfa5)"), false}, {"beta5 = 1", 2, tok("beta5", "=", "1"), false}}},
	{name: "IMAGE", keyword: "IMAGE", lines: []noDropLine{
		{`IMAGE "foto6.png" "alfa6"`, 0, tok("foto6.png", "alfa6"), false}, {`caption: "beta6"`, 2, tok("beta6"), false}, {`label: "gamma6"`, 2, tok("gamma6"), false}}},
	{name: "TABLE", headerSuffix: true, keyword: "TABLE", lines: []noDropLine{
		{"TABLE", 0, nil, false}, {"| alfa7 | beta7 |", 2, tok("alfa7", "beta7"), false}, {"| gamma7 | delta7 |", 2, tok("gamma7", "delta7"), false}, {`caption: "eps7"`, 2, tok("eps7"), false}}},
	{name: "TABLE_YAML", headerSuffix: true, keyword: "TABLE", lines: []noDropLine{
		{"TABLE", 0, nil, false}, {"headers: [alfa17, beta17]", 2, tok("alfa17", "beta17"), false}, {"rows:", 2, nil, false}, {"[gamma17, delta17]", 4, tok("gamma17", "delta17"), false}}},
	{name: "MDTABLE", lines: []noDropLine{
		{"| alfa18 | beta18 |", 0, tok("alfa18", "beta18"), false}, {"| --- | --- |", 0, nil, false}, {"| gamma18 | delta18 |", 0, tok("gamma18", "delta18"), false}}},
	// Sin fila separadora: la línea suelta que cae justo después del
	// encabezado es la que el parser puede tomar por separador. Con el
	// separador de MDTABLE, sus "---" ocupaban el lugar del token "---" de una
	// línea suelta perdida y la propiedad no lo veía.
	{name: "MDTABLE_NODELIM", lines: []noDropLine{
		{"| alfa30 | beta30 |", 0, tok("alfa30", "beta30"), false}, {"| gamma30 | delta30 |", 0, tok("gamma30", "delta30"), false}}},
	// La tabla Markdown a la sangría de un TABLE cuyo cuerpo ya terminó.
	{name: "TABLE_THEN_MDTABLE", lines: []noDropLine{
		{"TABLE", 0, nil, false}, {`caption: "alfa31"`, 2, tok("alfa31"), false}, {"| beta31 | gamma31 |", 0, tok("beta31", "gamma31"), false}}},
	{name: "CHART", lines: []noDropLine{
		{"<<chart: bar>>", 0, nil, false}, {`data: [["alfa8", 1], ["beta8", 2]]`, 2, tok("alfa8", "beta8"), false}, {`series: ["gamma8"]`, 2, tok("gamma8"), false}}},
	{name: "MAP", yaml: true, lines: []noDropLine{
		{"<<map>>", 0, nil, false}, {"type: world", 2, nil, false}, {"markers:", 2, nil, false}, {"- lat: 1", 4, nil, false}, {"lng: 2", 6, nil, false}, {`label: "alfa9"`, 6, tok("alfa9"), false}}},
	{name: "PLANTUML", lines: []noDropLine{
		{"<<plantuml>>", 0, nil, false}, {"@startuml", 2, tok("@startuml"), false}, {"Alice -> alfa10", 2, tok("Alice", "->", "alfa10"), false}, {"@enduml", 2, tok("@enduml"), false}}},
	{name: "MATH", closer: true, lines: []noDropLine{
		{"<<math>>", 0, nil, false}, {"alfa11 + 1", 2, tok("alfa11", "+", "1"), false}, {"<<end>>", 0, nil, false}}},
	{name: "MERMAID", lines: []noDropLine{
		{"<<mermaid>>", 0, nil, false}, {"graph TD", 2, tok("graph", "TD"), false}, {"A --> alfa12", 2, tok("A", "-->", "alfa12"), false}}},
	{name: "METRIC", yaml: true, closer: true, lines: []noDropLine{
		{"<<metric>>", 0, nil, false}, {"label: alfa13", 2, tok("alfa13"), false}, {"value: beta13", 2, tok("beta13"), false}, {"<<end>>", 0, nil, false}}},
	{name: "QUIZ", yaml: true, closer: true, lines: []noDropLine{
		{"<<quiz>>", 0, nil, false}, {"question: alfa14", 2, tok("alfa14"), false}, {"options: [beta14, gamma14]", 2, tok("beta14", "gamma14"), false}, {"answer: 0", 2, nil, false}, {"<<end>>", 0, nil, false}}},
	{name: "POLL", yaml: true, closer: true, lines: []noDropLine{
		{"<<poll>>", 0, nil, false}, {"question: alfa15", 2, tok("alfa15"), false}, {"options: [beta15, gamma15]", 2, tok("beta15", "gamma15"), false}, {"<<end>>", 0, nil, false}}},
	{name: "SPECIAL", closer: true, lines: []noDropLine{
		{":::note delta16", 0, tok("delta16"), false}, {"alfa16 beta16", 0, tok("alfa16", "beta16"), false}, {":::", 0, nil, false}}},
	{name: "MEDIA", lines: []noDropLine{
		{`<<video src="alfa19.mp4" controls>>`, 0, tok("alfa19.mp4"), false}}},
	{name: "SECTION_HEADING", slideOnly: true, lines: []noDropLine{
		{`SECTION "alfa20"`, 0, tok("alfa20"), false}, {"level: 4", 2, nil, false}}},
	{name: "GRID", closer: true, lines: []noDropLine{
		{"<<grid>>", 0, nil, false}, {"<<column>>", 0, nil, false}, {"alfa21 beta21", 2, tok("alfa21", "beta21"), false}, {"<<end>>", 0, nil, false}}},
	{name: "CODEGROUP", closer: true, lines: []noDropLine{
		{":::code-group", 0, nil, false}, {"```js [alfa22]", 0, tok("alfa22"), false}, {"beta22()", 0, tok("beta22()"), false}, {"```", 0, nil, false}, {":::", 0, nil, false}}},
	{name: "CHART_JSON", lines: []noDropLine{
		{"<<chart: bar>>", 0, nil, false}, {`{"labels": ["alfa24"],`, 2, tok("alfa24"), false}, {`"datasets": [{"label": "beta24", "data": [1]}]}`, 2, tok("beta24"), false}}},
	{name: "CHART_COMBO", yaml: true, lines: []noDropLine{
		{"<<chart: combo>>", 0, nil, false}, {"data:", 2, nil, false}, {`labels: ["alfa25"]`, 4, tok("alfa25"), false}, {"series:", 4, nil, false},
		{`- name: "beta25"`, 6, tok("beta25"), false}, {`type: "bar"`, 8, nil, false}, {"values: [1]", 8, nil, false}}},
	{name: "TABLE_CELLS", yaml: true, keyword: "TABLE", lines: []noDropLine{
		{"TABLE", 0, nil, false}, {"cells:", 2, nil, false}, {"- [{content: alfa26, header: true}, {content: beta26, header: true}]", 4, tok("alfa26", "beta26"), false},
		{"- [{content: gamma26}, {content: delta26}]", 4, tok("gamma26", "delta26"), false}}},
	{name: "TABLE_ROWS", yaml: true, keyword: "TABLE", lines: []noDropLine{
		{"TABLE", 0, nil, false}, {"tableRows:", 2, nil, false}, {"- cells: [{content: alfa27, header: true}, {content: beta27, header: true}]", 4, tok("alfa27", "beta27"), false},
		{"- cells: [{content: gamma27}, {content: delta27}]", 4, tok("gamma27", "delta27"), false}}},
	{name: "MATH_CAPTION", closer: true, lines: []noDropLine{
		{"<<math>>", 0, nil, false}, {"alfa28 + 1", 2, tok("alfa28", "+", "1"), false}, {`caption: "beta28"`, 2, tok("beta28"), false}, {"label: eq:gamma28", 2, tok("eq:gamma28"), false}, {"<<end>>", 0, nil, false}}},
	{name: "NOTES", lines: []noDropLine{
		{"@notes", 0, nil, false}, {"alfa29 beta29", 2, tok("alfa29", "beta29"), false}, {"gamma29", 2, tok("gamma29"), false}}},
	// Un @notes de varias líneas y un elemento hermano a su sangría: las
	// líneas sueltas entre los dos son nota o van al hermano, pero no se
	// pierden.
	{name: "NOTES_THEN_TEXT", lines: []noDropLine{
		{"@notes", 0, nil, false}, {"alfa34 beta34", 2, tok("alfa34", "beta34"), false}, {"TEXT", 0, nil, false}, {"gamma34", 2, tok("gamma34"), false}}},
	{name: "NOTES_SAME_INDENT_THEN_POINTS", lines: []noDropLine{
		{"@notes:", 0, nil, false}, {"alfa35", 0, tok("alfa35"), false}, {"POINTS", 0, nil, false}, {"- beta35", 2, tok("beta35"), false}}},
	{name: "DIRECTIVE", headerSuffix: true, lines: []noDropLine{
		{`@transition duration="alfa23"`, 0, tok("alfa23"), false}}},
}

// noDropStrays son las líneas sueltas: ninguna es sintaxis válida en ningún
// lugar de un cuerpo strict, salvo como texto de un elemento que guarda su
// cuerpo crudo (TEXT, CODE, un diagrama, un callout).
var noDropStrays = []string{
	"Importante:",
	"5 € + 3 %",
	"o B",
	"pie apple",
	"---",
	`""`,
	"-> <> [] {} ## **",
	"x",
	"eco eco eco",
	"zeta:",
	"alt: perro",
	"width: 50",
	"hola mundo cruel",
	"%",
	"? ¿",
}

// noDropLookalikes son líneas sueltas que se parecen a sintaxis. Las que
// tienen texto se buscan como palabras exactas; `<<end>>` y `:::` son cierres
// válidos en muchos bloques, así que se insertan sin exigir nada de ellas: lo
// que se prueba es que, si cierran antes de tiempo, el resto de las líneas siga
// llegando al AST o a un diagnóstico.
var noDropLookalikes = []noDropLine{
	{"-", 0, tok("-"), true},
	{"*", 0, tok("*"), true},
	// Una fila `[ ]` de rows: es una fila vacía, que sí llega al AST: no hay
	// texto que exigir. El ítem vacío de CHECKLIST tiene su prueba propia.
	{"[ ]", 0, nil, true},
	{"#", 0, tok("#"), true},
	{"@", 0, tok("@"), true},
	{"| a", 0, tok("a"), true},
	{"<<end>>", 0, nil, true},
	{":::", 0, nil, true},
}

// noDropRuleStrays son líneas de símbolos que se parecen a una regla
// horizontal o a un separador de tabla. Las elige un generador aparte (ver
// noDropExtras) para que las líneas sueltas de siempre sigan saliendo iguales
// con cada semilla.
var noDropRuleStrays = []noDropLine{
	{"---", 0, tok("---"), false},
	{"-----", 0, tok("-----"), false},
	{":---:", 0, tok(":---:"), false},
	{"***", 0, tok("***"), false},
	{"___", 0, tok("___"), false},
	{"===", 0, tok("==="), false},
	{"+++", 0, tok("+++"), false},
	{"--", 0, tok("--"), false},
	// Una fila con "---" adentro: sus celdas son las palabras, sin los "|".
	{"| x --- y |", 0, tok("x", "---", "y"), false},
}

type noDropContext struct {
	name  string
	slide bool
	// prefix es el texto antes del cuerpo; indent, la sangría del cuerpo;
	// suffix, lo que lo cierra.
	prefix string
	indent int
	suffix string
}

const noDropDocHeader = "---\nmode: strict\ntitle: \"Doc\"\n---\n"

// noDropTypedLists activa el otro parser de listas (parseTypedList).
const noDropTypedLists = "---\nmode: strict\ntitle: \"Doc\"\nast_capabilities: [\"nested-list-types-v1\", \"list-start-v1\"]\n---\n"

var noDropContexts = []noDropContext{
	{name: "document section", prefix: noDropDocHeader + "SECTION \"Sec\"\n", indent: 2},
	{name: "slide", slide: true, prefix: noDropDocHeader + "SLIDE content\n  title: \"Tit\"\n", indent: 2},
	{name: "typed column (document)", prefix: noDropDocHeader + "SECTION \"Sec\"\n  <<grid>>\n  <<column typed>>\n", indent: 4, suffix: "  <<end>>\n"},
	{name: "typed column (slide)", slide: true, prefix: noDropDocHeader + "SLIDE content\n  title: \"Tit\"\n  <<grid>>\n  <<column typed>>\n", indent: 4, suffix: "  <<end>>\n"},
	{name: "typed callout (document)", prefix: noDropDocHeader + "SECTION \"Sec\"\n  :::info{typed}\n", indent: 2, suffix: "  :::\n"},
	{name: "typed callout (slide)", slide: true, prefix: noDropDocHeader + "SLIDE content\n  title: \"Tit\"\n  :::info{typed}\n", indent: 2, suffix: "  :::\n"},
	{name: "document section with typed lists", prefix: noDropTypedLists + "SECTION \"Sec\"\n", indent: 2},
	{name: "slide with typed lists", slide: true, prefix: noDropTypedLists + "SLIDE content\n  title: \"Tit\"\n", indent: 2},
	// Un callout sin {typed}: su cuerpo delega a los parsers de elemento
	// (CODE, TABLE, diagramas…) y guarda el resto como prosa.
	{name: "callout (document)", prefix: noDropDocHeader + "SECTION \"Sec\"\n  :::note\n", indent: 2, suffix: "  :::\n"},
	{name: "callout (slide)", slide: true, prefix: noDropDocHeader + "SLIDE content\n  title: \"Tit\"\n  :::note\n", indent: 2, suffix: "  :::\n"},
}

// noDropCase es un documento generado y, por línea del archivo (1-based), los
// tokens que esa línea tiene que dejar en el AST.
type noDropCase struct {
	src      string
	payloads map[int][]string
	texts    map[int]string
	exact    map[int]bool
}

// exactToken, al frente de un token, lo marca como exacto (ver
// noDropLine.exact) aunque el resto de la línea no lo sea.
const exactToken = "\x01"

// noDropHeader es la forma en que se altera la cabecera de la plantilla.
type noDropHeader struct {
	// sep va entre la cabecera y suffix: " " (`TEXT hola`) o ":" (`TEXT:
	// hola`).
	sep, suffix string
	// glued pega una letra a la palabra clave: `TEXTZ`, `CODEZ python`.
	glued bool
}

// buildNoDropCase arma el documento. strays son (posición dentro del cuerpo,
// sangría relativa, línea); header altera la cabecera.
func buildNoDropCase(ctx noDropContext, tpl noDropTemplate, strays []noDropStray, header noDropHeader) noDropCase {
	lines := append([]noDropLine(nil), tpl.lines...)
	if header.suffix != "" {
		lines[0].text += header.sep + " " + header.suffix
		lines[0].payload = append(append([]string(nil), lines[0].payload...), strings.Fields(header.suffix)...)
	}
	if header.glued {
		lines[0].text = tpl.keyword + "Z" + strings.TrimPrefix(lines[0].text, tpl.keyword)
		lines[0].payload = append(append([]string(nil), lines[0].payload...), exactToken+tpl.keyword+"Z")
	}
	end := len(lines)
	if tpl.closer {
		end--
	}
	// Se insertan de atrás para adelante para que las posiciones sigan valiendo.
	sort.SliceStable(strays, func(a, b int) bool { return strays[a].at > strays[b].at })
	for _, s := range strays {
		at := 1 + s.at%end
		if at > end {
			at = end
		}
		l := s.line
		l.indent = s.indent
		if tpl.yaml && (l.text == "-" || l.text == "*" || l.text == "#") {
			// En YAML son sintaxis: un ítem nulo, un alias, un comentario.
			l.payload = nil
		}
		if tpl.yaml {
			for k, w := range l.payload {
				if strings.HasPrefix(w, "#") {
					l.payload = l.payload[:k]
					break
				}
			}
		}
		lines = append(lines[:at], append([]noDropLine{l}, lines[at:]...)...)
		end++
	}

	var b strings.Builder
	b.WriteString(ctx.prefix)
	lineNo := strings.Count(ctx.prefix, "\n")
	c := noDropCase{payloads: map[int][]string{}, texts: map[int]string{}, exact: map[int]bool{}}
	for _, l := range lines {
		indent := ctx.indent + l.indent
		if indent < 0 {
			indent = 0
		}
		lineNo++
		b.WriteString(strings.Repeat(" ", indent) + l.text + "\n")
		c.payloads[lineNo] = l.payload
		c.texts[lineNo] = l.text
		if l.exact {
			c.exact[lineNo] = true
		}
	}
	b.WriteString(ctx.suffix)
	c.src = b.String()
	return c
}

type noDropStray struct {
	at     int
	indent int
	line   noDropLine
}

// astTokens junta, como multiconjunto, las palabras de todo valor de texto
// del AST, sin los campos que nombran tipos ("type", "listType"…), que no son
// texto del autor. En un bloque especial con elementos anidados cuenta solo
// Elements: es lo que se renderiza (Content queda como fuente para fmt), así
// que una línea que solo sobreviviera en Content se perdería en la salida.
func astTokens(t *testing.T, tree *ast.AST) (map[string]int, []byte) {
	t.Helper()
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	var corpus []byte
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if x["type"] == "special_block" {
				if els, ok := x["elements"].([]any); ok && len(els) > 0 {
					for k, child := range x {
						if k != "content" {
							walk(child)
						}
					}
					return
				}
			}
			for k, child := range x {
				if k == "position" || k == "endPosition" || k == "schemaVersion" || k == "context" || strings.HasSuffix(k, "type") || strings.HasSuffix(k, "Type") {
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		case string:
			for _, w := range strings.Fields(x) {
				out[w]++
			}
			corpus = append(append(corpus, x...), 0)
		case float64:
			out[fmt.Sprint(x)]++
		}
	}
	walk(v)
	return out, corpus
}

// takeToken consume una aparición de w: como palabra entera o, si tiene al
// menos cuatro caracteres, dentro de un texto más largo (un encabezado
// guarda su texto dentro de su HTML, un callout guarda `src="x.mp4"` como
// una sola palabra). Un token corto ("x", "%") tiene que aparecer como
// palabra: como subcadena se encontraría en cualquier parte.
func takeToken(tokens map[string]int, corpus []byte, w string, exact bool) bool {
	if tokens[w] > 0 {
		tokens[w]--
		return true
	}
	if exact || len(w) < 4 {
		return false
	}
	i := strings.Index(string(corpus), w)
	if i < 0 {
		return false
	}
	for k := i; k < i+len(w); k++ {
		corpus[k] = 1
	}
	return true
}

// checkNoDrop devuelve la primera línea cuyo texto no llegó ni al AST ni a un
// diagnóstico anclado en ella, o "" si todas llegaron.
func checkNoDrop(t *testing.T, ctx noDropContext, c noDropCase) string {
	t.Helper()
	p := New(util.NewNoop())
	var tree *ast.AST
	var diags []diagnostics.Diagnostic
	if ctx.slide {
		tree, diags = p.Parse(c.src, "nodrop.slidelang")
	} else {
		tree, diags = p.ParseDocument(c.src, "nodrop.doclang")
	}
	noDropDump(t, ctx, c, tree, diags)
	// Un diagnóstico cubre su línea o, si trae EndPosition, el tramo entero:
	// así se reporta un cuerpo YAML que se ignoró completo.
	diagLines := map[int]bool{}
	for _, d := range diags {
		end := d.Position.Line
		if d.EndPosition != nil && d.EndPosition.Line > end {
			end = d.EndPosition.Line
		}
		for l := d.Position.Line; l <= end; l++ {
			diagLines[l] = true
		}
	}
	tokens, corpus := astTokens(t, tree)
	var lines []int
	for n := range c.payloads {
		lines = append(lines, n)
	}
	sort.Ints(lines)
	for _, n := range lines {
		if diagLines[n] {
			continue
		}
		for _, w := range c.payloads[n] {
			exact := c.exact[n]
			if rest, ok := strings.CutPrefix(w, exactToken); ok {
				w, exact = rest, true
			}
			if !takeToken(tokens, corpus, w, exact) {
				var ds []string
				for _, d := range diags {
					ds = append(ds, fmt.Sprintf("%d:%s %s", d.Position.Line, d.RuleID, d.Message))
				}
				return fmt.Sprintf("line %d %q lost %q with no diagnostic on it\n--- source ---\n%s--- diagnostics ---\n%s",
					n, c.texts[n], w, c.src, strings.Join(ds, "\n"))
			}
		}
	}
	return ""
}

// noDropDump, con NODROP_DUMP=<dir>, escribe el AST y los errores de cada
// caso generado. Sirve para comparar dos versiones del parser: los avisos de
// este cambio son warnings y no tocan el AST, así que el volcado tiene que
// salir idéntico antes y después.
func noDropDump(t *testing.T, ctx noDropContext, c noDropCase, tree *ast.AST, diags []diagnostics.Diagnostic) {
	dir := os.Getenv("NODROP_DUMP")
	if dir == "" {
		return
	}
	var errs []diagnostics.Diagnostic
	for _, d := range diags {
		if d.IsError() {
			errs = append(errs, d)
		}
	}
	raw, err := json.MarshalIndent(struct {
		AST    *ast.AST
		Errors []diagnostics.Diagnostic
	}{tree, errs}, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("%x.json", sha256.Sum256([]byte(ctx.name+"\x00"+c.src)))
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

const noDropSeeds = 60

// noDropStrayLine elige una línea suelta: de cada cuatro, una se parece a
// sintaxis.
func noDropStrayLine(rng *rand.Rand) noDropLine {
	if rng.Intn(4) == 0 {
		return noDropLookalikes[rng.Intn(len(noDropLookalikes))]
	}
	text := noDropStrays[rng.Intn(len(noDropStrays))]
	return noDropLine{text: text, payload: strings.Fields(text)}
}

// noDropHeaderFor elige cómo alterar la cabecera de tpl: casi siempre no se
// toca; si no lleva argumentos, a veces se le agrega texto (con espacio o con
// dos puntos); si tiene palabra clave, a veces se le pega una letra.
func noDropHeaderFor(rng *rand.Rand, tpl noDropTemplate) noDropHeader {
	switch rng.Intn(6) {
	case 3, 4:
		if tpl.headerSuffix {
			sep := ""
			if rng.Intn(2) == 0 && tpl.keyword != "" {
				sep = ":"
			}
			return noDropHeader{sep: sep, suffix: noDropStrays[rng.Intn(len(noDropStrays))]}
		}
		if tpl.extraArgs {
			return noDropHeader{suffix: "main.py extra33"}
		}
	case 5:
		if tpl.keyword != "" {
			return noDropHeader{glued: true}
		}
	}
	return noDropHeader{}
}

// noDropExtras agrega, con un generador propio, lo que la propiedad no
// generaba en su primera versión: a veces una línea de símbolos de
// noDropRuleStrays y a veces una propiedad repetida (ver withRepeatedProperty).
// Usa su propio rng para no alterar las líneas sueltas de cada semilla.
func noDropExtras(seed int64, tpl noDropTemplate) (noDropTemplate, []noDropStray) {
	ext := rand.New(rand.NewSource(seed + 1<<20))
	var strays []noDropStray
	if ext.Intn(2) == 0 {
		strays = append(strays, noDropStray{
			at:     ext.Intn(16),
			indent: []int{2, 2, 4, 6, 0, -2}[ext.Intn(6)],
			line:   noDropRuleStrays[ext.Intn(len(noDropRuleStrays))],
		})
	}
	if ext.Intn(3) == 0 {
		tpl = withRepeatedProperty(ext, tpl)
	}
	return tpl, strays
}

// noDropKeyLine reconoce una línea de propiedad: `caption: …`, `AUTHOR: …`.
var noDropKeyLine = regexp.MustCompile(`^[A-Za-z]+:`)

// withRepeatedProperty copia una de las líneas de propiedad de tpl que llevan
// texto (`caption: "beta28"`) y la inserta más abajo en el cuerpo con otro
// texto (`caption: "BETA28"`): una propiedad repetida pisa a la anterior en
// muchos bloques, y la que se pierde tiene que llevar su aviso. Devuelve tpl
// sin cambios si no tiene ninguna.
func withRepeatedProperty(rng *rand.Rand, tpl noDropTemplate) noDropTemplate {
	var keyed []int
	for k := 1; k < len(tpl.lines); k++ {
		if l := tpl.lines[k]; len(l.payload) > 0 && noDropKeyLine.MatchString(l.text) {
			keyed = append(keyed, k)
		}
	}
	if len(keyed) == 0 {
		return tpl
	}
	k := keyed[rng.Intn(len(keyed))]
	dup := tpl.lines[k]
	dup.payload = nil
	for _, w := range tpl.lines[k].payload {
		// En mayúsculas y no con un sufijo: takeToken busca un token largo
		// también como subcadena, y "beta28" se encontraría dentro de
		// "beta28r" aunque la primera línea se hubiera perdido.
		dup.text = strings.ReplaceAll(dup.text, w, strings.ToUpper(w))
		dup.payload = append(dup.payload, strings.ToUpper(w))
	}
	end := len(tpl.lines)
	if tpl.closer {
		end--
	}
	at := k + 1 + rng.Intn(end-k)
	lines := append([]noDropLine(nil), tpl.lines[:at]...)
	lines = append(lines, dup)
	tpl.lines = append(lines, tpl.lines[at:]...)
	return tpl
}

func TestStrictNoSilentDrop_Property(t *testing.T) {
	for _, ctx := range noDropContexts {
		for _, tpl := range noDropTemplates {
			if tpl.slideOnly && !ctx.slide {
				continue
			}
			t.Run(ctx.name+"/"+tpl.name, func(t *testing.T) {
				// La plantilla sola no pierde nada: si fallara, el problema
				// sería la plantilla y no las líneas sueltas.
				if msg := checkNoDrop(t, ctx, buildNoDropCase(ctx, tpl, nil, noDropHeader{})); msg != "" {
					if os.Getenv("NODROP_ALL") == "" {
						t.Fatalf("template alone: %s", msg)
					}
					t.Errorf("template alone: %s", strings.SplitN(msg, "\n", 2)[0])
				}
				for seed := int64(1); seed <= noDropSeeds; seed++ {
					rng := rand.New(rand.NewSource(seed))
					var strays []noDropStray
					for n := 1 + rng.Intn(3); n > 0; n-- {
						strays = append(strays, noDropStray{
							at:     rng.Intn(16),
							indent: []int{2, 2, 4, 6, 0, -2}[rng.Intn(6)],
							line:   noDropStrayLine(rng),
						})
					}
					header := noDropHeaderFor(rng, tpl)
					caseTpl, extra := noDropExtras(seed, tpl)
					strays = append(strays, extra...)
					if msg := checkNoDrop(t, ctx, buildNoDropCase(ctx, caseTpl, strays, header)); msg != "" {
						if os.Getenv("NODROP_ALL") != "" {
							t.Errorf("seed %d: %s", seed, strings.SplitN(msg, "\n", 2)[0])
							continue
						}
						t.Fatalf("seed %d: %s", seed, msg)
					}
				}
			})
		}
	}
}
