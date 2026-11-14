// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// El texto de la línea de un TEXT o un QUOTE strict es la primera línea de
// su contenido. Las líneas que el elemento toma no cambian: el cuerpo
// sangrado sigue igual, y un TEXT sin cuerpo sangrado sigue tomando lo que
// viene a su misma sangría (TEXT002).

// strictReadsCases arma body en un SECTION de documento, en un SLIDE y en
// una columna tipada. body ya lleva la sangría del contenedor (dos espacios).
func strictReadsCases(body string) map[string]strictCase {
	column := ""
	for _, l := range strings.SplitAfter(body, "\n") {
		if strings.TrimSpace(l) != "" {
			l = "  " + l
		}
		column += l
	}
	return map[string]strictCase{
		"document":     {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n" + body, true},
		"slide":        {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n" + body, false},
		"typed column": {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  <<grid>>\n  <<column typed>>\n" + column + "  <<end>>\n", true},
	}
}

// strictReadsParse devuelve los elementos del cuerpo (los de la columna, en
// una columna tipada) y los diagnósticos.
func strictReadsParse(t *testing.T, c strictCase) ([]ast.Element, []diagnostics.Diagnostic) {
	t.Helper()
	p := New(util.NewNoop())
	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if c.document {
		doc, diags = p.ParseDocument(c.src, "r.doclang")
	} else {
		doc, diags = p.Parse(c.src, "r.slidelang")
	}
	els := doc.ContentBlocks[len(doc.ContentBlocks)-1].Elements
	if len(els) == 1 {
		if g, ok := els[0].(*ast.GridElement); ok && len(g.Columns) > 0 {
			els = g.Columns[0].Elements
		}
	}
	return els, diags
}

// elementContent es "text:<contenido>" o "quote:<contenido>|<autor>".
func elementContent(el ast.Element) string {
	switch e := el.(type) {
	case *ast.TextElement:
		return "text:" + e.Content
	case *ast.QuoteElement:
		s := "quote:" + e.Content
		if e.Author != "" {
			s += "|" + e.Author
		}
		return s
	}
	return string(el.GetType())
}

func TestStrictReads_KeywordLineTextIsContent(t *testing.T) {
	cases := []struct {
		name, body string
		want       []string
	}{
		{"text alone", "  TEXT hola\n", []string{"text:hola"}},
		{"text colon", "  TEXT: hola\n", []string{"text:hola"}},
		{"text colon without space", "  TEXT:hola\n", []string{"text:hola"}},
		{"text with body", "  TEXT hola\n    mundo\n    cruel\n", []string{"text:hola mundo cruel"}},
		{"text with inline markup", "  TEXT **hola** [x](https://example.com)\n", []string{"text:**hola** [x](https://example.com)"}},
		{"text with a keyword", "  TEXT POINTS de la agenda\n", []string{"text:POINTS de la agenda"}},
		{"text with a symbol", "  TEXT @usuario\n", []string{"text:@usuario"}},
		{"text then sibling after body", "  TEXT hola\n    mundo\n  POINTS\n    - a\n", []string{"text:hola mundo", "points"}},
		{"quote alone", "  QUOTE una cita\n", []string{"quote:una cita"}},
		{"quote colon", "  QUOTE: una cita\n", []string{"quote:una cita"}},
		{"quote with body", "  QUOTE una cita\n    que sigue\n    AUTHOR: Ana\n", []string{"quote:una cita\nque sigue|Ana"}},
		{"quote with author only", "  QUOTE una cita\n    AUTHOR: Ana\n", []string{"quote:una cita|Ana"}},
		{"quote then sibling", "  QUOTE una cita\n  TEXT\n    hola\n", []string{"quote:una cita", "text:hola"}},
		{"quote with a glued keyword", "  QUOTE TEXTO citado\n", []string{"quote:TEXTO citado"}},
	}
	for _, tc := range cases {
		for name, c := range strictReadsCases(tc.body) {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				els, diags := strictReadsParse(t, c)
				var got []string
				for _, el := range els {
					got = append(got, elementContent(el))
				}
				if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
					t.Errorf("got %q, want %q", got, tc.want)
				}
				for _, d := range diags {
					t.Errorf("unexpected %s %s", d.RuleID, d.Message)
				}
			})
		}
	}
}

// Una línea en blanco tras `QUOTE una cita` corta la cita, como cuando la
// cita no tenía texto: las líneas sangradas de abajo no pasan a ser suyas.
// Dentro de un cuerpo que ya tiene texto, la línea en blanco sigue siendo un
// párrafo de la cita.
func TestStrictReads_QuoteBlankLineKeepsItsLines(t *testing.T) {
	for name, c := range strictReadsCases("  QUOTE una cita\n\n    otra\n") {
		t.Run(name, func(t *testing.T) {
			els, _ := strictReadsParse(t, c)
			if len(els) == 0 || elementContent(els[0]) != "quote:una cita" {
				t.Fatalf("got %v", els)
			}
		})
	}
	for name, c := range strictReadsCases("  QUOTE una cita\n    sigue\n\n    otra\n") {
		t.Run(name+"/paragraph", func(t *testing.T) {
			els, diags := strictReadsParse(t, c)
			if len(els) != 1 || elementContent(els[0]) != "quote:una cita\nsigue\n\notra" || len(diags) != 0 {
				t.Fatalf("got %v %v", els, diags)
			}
		})
	}
}

// Un TEXT con texto en su línea no está vacío (no hay TEXT001), pero sin
// cuerpo sangrado sigue tomando lo que viene a su misma sangría, como antes:
// TEXT002 lo dice.
func TestStrictReads_TextWithoutIndentedBodyStillAbsorbs(t *testing.T) {
	for name, c := range strictReadsCases("  TEXT hola\n  POINTS\n    - a\n") {
		t.Run(name, func(t *testing.T) {
			els, diags := strictReadsParse(t, c)
			if len(els) != 1 || elementContent(els[0]) != "text:hola POINTS - a" {
				t.Fatalf("got %v", els)
			}
			if len(diags) != 1 || diags[0].RuleID != "TEXT002" || !diags[0].IsWarning() ||
				!strings.Contains(diags[0].Message, `TEXT has no indented body: the POINTS element on the line "POINTS"`) ||
				!strings.Contains(diags[0].Message, `after "hola"`) {
				t.Fatalf("want one TEXT002 naming POINTS and the text on the TEXT line, got %v", diags)
			}
		})
	}
	for name, c := range strictReadsCases("  TEXT hola\n") {
		t.Run(name+"/no TEXT001", func(t *testing.T) {
			if _, diags := strictReadsParse(t, c); len(diags) != 0 {
				t.Fatalf("unexpected %v", diags)
			}
		})
	}
}

// Lo que TEXT y QUOTE no leen de su línea queda como antes: el mismo
// elemento (o ninguno, para un TEXT vacío) y STRICT004.
func TestStrictReads_KeywordLineTextNotRead(t *testing.T) {
	cases := []struct {
		name, body, fix string
		want            []string
	}{
		{"text glued", "  TEXTO citado\n    mundo\n", "only after a space or a colon (TEXT hola)", []string{"text:mundo"}},
		{"text ref", "  TEXT ver \\ref{fig:a}\n    mundo\n", `A \ref on the TEXT line is not read`, []string{"text:mundo"}},
		{"text node-id", "  TEXT <!-- node-id: n -->\n    mundo\n", "node-id or @include directive", []string{"text:mundo"}},
		{"text include", "  TEXT @include otro.doclang\n    mundo\n", "node-id or @include directive", []string{"text:mundo"}},
		{"quote glued", "  QUOTEado\n    cita\n", "only after a space or a colon (QUOTE hola)", []string{"quote:cita"}},
		{"quote author", "  QUOTE AUTHOR: Ana\n    cita\n", "not as AUTHOR:, SOURCE:", []string{"quote:cita"}},
		{"quote source", "  QUOTE SOURCE: libro\n    cita\n", "not as AUTHOR:, SOURCE:", []string{"quote:cita"}},
		{"quote rule", "  QUOTE ---\n    cita\n", "the end of the quote", []string{"quote:cita"}},
		{"quote keyword", "  QUOTE TEXT hola\n    cita\n", "another element", []string{"quote:cita"}},
		{"quote marker", "  QUOTE | a |\n    cita\n", "another element", []string{"quote:cita"}},
		{"quote ref", "  QUOTE \\ref{tbl:a}\n    cita\n", `A \ref on the QUOTE line is not read`, []string{"quote:cita"}},
	}
	for _, tc := range cases {
		for name, c := range strictReadsCases(tc.body) {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				els, diags := strictReadsParse(t, c)
				var got []string
				for _, el := range els {
					got = append(got, elementContent(el))
				}
				if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
					t.Errorf("got %q, want %q", got, tc.want)
				}
				if len(diags) != 1 || diags[0].RuleID != "STRICT004" || !diags[0].IsWarning() || !strings.Contains(diags[0].Message, tc.fix) {
					t.Errorf("want one STRICT004 with %q, got %v", tc.fix, diags)
				}
			})
		}
	}
}

// Una propiedad repetida de un SECTION de documento (level:, id:) o de un
// SLIDE (title:) pisa a la anterior, como antes, y la anterior lleva
// STRICT005 en su línea, citando la posterior por su texto.
func TestStrictReads_RepeatedContainerProperties(t *testing.T) {
	const front = "---\nmode: strict\ntitle: \"D\"\n---\n" // 4 líneas
	cases := []struct {
		name     string
		document bool
		src      string
		line     int
		quote    string
		check    func(*ast.AST) bool
	}{
		{
			name: "section level", document: true,
			src:  front + "SECTION \"A\"\n  TEXT\n    a\nSECTION \"B\"\n  level: 2\n  level: 3\n  TEXT\n    b\n",
			line: 9, quote: `This level: line of the SECTION was ignored: "level: 2". The later "level: 3" replaces it; only the last level: is read.`,
			check: func(doc *ast.AST) bool {
				h, ok := doc.ContentBlocks[0].Elements[1].(*ast.TextElement)
				return ok && strings.HasPrefix(h.Content, "<h3")
			},
		},
		{
			name: "section id", document: true,
			src:  front + "SECTION \"A\"\n  TEXT\n    a\nSECTION \"B\"\n  level: 2\n  id: uno\n  id: dos\n",
			line: 10, quote: `"id: uno"`,
			check: func(doc *ast.AST) bool {
				h, ok := doc.ContentBlocks[0].Elements[1].(*ast.TextElement)
				return ok && strings.Contains(h.Content, `id="dos"`)
			},
		},
		{
			name: "slide title",
			src:  front + "SLIDE content\n  title: \"Uno\"\n  TEXT\n    a\n  title: \"Dos\"\n",
			line: 6, quote: `This title: line of the SLIDE was ignored: "title: \"Uno\"". The later "title: \"Dos\"" replaces it`,
			check: func(doc *ast.AST) bool { return doc.ContentBlocks[0].Title == "Dos" },
		},
		{
			name: "slide layout option",
			src:  front + "SLIDE hero\n  title: \"T\"\n  align: left\n  align: center\n",
			line: 7, quote: `"align: left"`,
			check: func(doc *ast.AST) bool {
				return doc.ContentBlocks[0].LayoutConfig != nil && doc.ContentBlocks[0].LayoutConfig.Align == "center"
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New(util.NewNoop())
			var doc *ast.AST
			var diags []diagnostics.Diagnostic
			if tc.document {
				doc, diags = p.ParseDocument(tc.src, "p.doclang")
			} else {
				doc, diags = p.Parse(tc.src, "p.slidelang")
			}
			if !tc.check(doc) {
				t.Errorf("the later value does not win in the AST")
			}
			found := false
			for _, d := range diags {
				if d.RuleID == "STRICT005" && d.Position.Line == tc.line && d.IsWarning() && strings.Contains(d.Message, tc.quote) {
					found = true
					continue
				}
				t.Errorf("unexpected %d:%s %s", d.Position.Line, d.RuleID, d.Message)
			}
			if !found {
				t.Errorf("want STRICT005 on line %d with %q, got %v", tc.line, tc.quote, diags)
			}
		})
	}
}

// Un valor que el contenedor rechaza con un error no reemplaza al anterior,
// así que no hay STRICT005.
func TestStrictReads_RejectedValueDoesNotReplace(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"D\"\n---\nSECTION \"A\"\n  TEXT\n    a\nSECTION \"B\"\n  level: 2\n  level: siete\n"
	_, diags := New(util.NewNoop()).ParseDocument(src, "r.doclang")
	for _, d := range diags {
		if d.RuleID == "STRICT005" {
			t.Errorf("unexpected STRICT005: %s", d.Message)
		}
	}
	src = "---\nmode: strict\n---\nSLIDE comparison\n  title: \"T\"\n  columns: 2\n  columns: 9\n  TEXT\n    a\n"
	_, diags = New(util.NewNoop()).Parse(src, "r.slidelang")
	for _, d := range diags {
		if d.RuleID == "STRICT005" {
			t.Errorf("unexpected STRICT005: %s", d.Message)
		}
	}
}

// En <<map>> y en una directiva gana el valor de siempre: el último.
func TestStrictReads_RepeatedKeysKeepTheLaterValue(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"D\"\n---\nSECTION \"A\"\n" +
		"  <<map type=\"city\">>\n    type: world\n    type: europe\n    markers:\n    - lat: 1\n      lng: 2\n      label: \"a\"\n      label: \"b\"\n" +
		"  @transition type=\"fade\" type=\"slide\"\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "k.doclang")
	m, ok := doc.ContentBlocks[0].Elements[0].(*ast.MapElement)
	if !ok || m.MapType != "europe" || len(m.Markers) != 1 || m.Markers[0].Label != "b" {
		t.Fatalf("map: %#v", doc.ContentBlocks[0].Elements[0])
	}
	d, ok := doc.ContentBlocks[0].Elements[1].(*ast.DirectiveNode)
	if !ok || d.Parameters["type"] != "slide" {
		t.Fatalf("directive: %#v", doc.ContentBlocks[0].Elements[1])
	}
	want := map[string]int{"MAP003": 3, "DIRECTIVE004": 1}
	for _, x := range diags {
		want[x.RuleID]--
		if !x.IsWarning() {
			t.Errorf("%s is %s", x.RuleID, x.Severity)
		}
	}
	for id, n := range want {
		if n != 0 {
			t.Errorf("%s: %d more or fewer than expected in %v", id, n, diags)
		}
	}
}

// El aviso de un cuerpo YAML inválido no nombra números de línea: los de
// yaml.v3 cuentan desde el cuerpo, no desde el archivo.
func TestStrictReads_InvalidYAMLMessageHasNoLineNumbers(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"D\"\n---\nSECTION \"A\"\n" +
		"  <<quiz>>\n    question: Q\n    options: [a, b]\n    answer: 0\n    answer: 1\n  <<end>>\n" +
		"  <<poll>>\n    question: Q\n    question: R\n    options: [a]\n  <<end>>\n" +
		"  <<metric>>\n    label: L\n    label: M\n  <<end>>\n"
	_, diags := New(util.NewNoop()).ParseDocument(src, "y.doclang")
	seen := map[string]bool{}
	lineNumber := regexp.MustCompile(`\bline \d+`)
	for _, d := range diags {
		seen[d.RuleID] = true
		if lineNumber.MatchString(d.Message) {
			t.Errorf("%s names a line number: %s", d.RuleID, d.Message)
		}
	}
	for _, id := range []string{"QUIZ004", "QUIZ007", "POLL006", "POLL007", "METRIC002", "METRIC006"} {
		if !seen[id] {
			t.Errorf("missing %s in %v", id, diags)
		}
	}
}

// Con marcadores node-id, que se quitan antes de parsear, cada aviso queda en
// su línea del archivo, y ningún texto nombra un número de línea.
func TestStrictReads_NodeIDKeepsLines(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"D\"\n---\n" + // 1-4
		"SECTION \"A\"\n" + // 5
		"  <!-- node-id: Uno -->\n" + // 6
		"  TEXT\n" + // 7
		"    a\n" + // 8
		"<!-- node-id: Sec -->\n" + // 9
		"SECTION \"B\"\n" + // 10
		"  level: 2\n" + // 11
		"  level: 3\n" + // 12
		"  <!-- node-id: Mapa -->\n" + // 13
		"  <<map>>\n" + // 14
		"    type: world\n" + // 15
		"    type: europe\n" + // 16
		"  <!-- node-id: Kpi -->\n" + // 17
		"  <<metric>>\n" + // 18
		"    label: L\n" + // 19
		"    label: M\n" + // 20
		"  <<end>>\n" + // 21
		"  <!-- node-id: Cita -->\n" + // 22
		"  QUOTE AUTHOR: Ana\n" + // 23
		"    cita\n" // 24
	_, diags := New(util.NewNoop()).ParseDocument(src, "ids.doclang")
	want := map[string]int{"STRICT005": 11, "MAP003": 15, "METRIC006": 19, "STRICT004": 23}
	lineNumber := regexp.MustCompile(`\bline \d+`)
	for _, d := range diags {
		w, ok := want[d.RuleID]
		if !ok {
			continue
		}
		delete(want, d.RuleID)
		if d.Position.Line != w {
			t.Errorf("%s: want line %d, got %d", d.RuleID, w, d.Position.Line)
		}
		if m := lineNumber.FindString(d.Message); m != "" {
			t.Errorf("%s names %q in its text: %s", d.RuleID, m, d.Message)
		}
	}
	for id := range want {
		t.Errorf("missing %s in %v", id, diags)
	}
}
