// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// Un TEXT sin cuerpo no se traga a los elementos hermanos escritos a su misma
// sangría: el cuerpo de TEXT son las líneas con más sangría que él. Antes
// `TEXT` / `POINTS` / `- uno` salía como el párrafo "POINTS - uno".
func TestStrictText_EmptyBodyDoesNotSwallowSiblings(t *testing.T) {
	for name, tc := range map[string]struct {
		src      string
		document bool
	}{
		"document": {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n  POINTS\n    - uno\n  CHECKLIST\n    - [ ] dos\n", true},
		"slide":    {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n  POINTS\n    - uno\n  CHECKLIST\n    - [ ] dos\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			p := New(util.NewNoop())
			var doc *ast.AST
			if tc.document {
				doc, _ = p.ParseDocument(tc.src, "d.doclang")
			} else {
				doc, _ = p.Parse(tc.src, "s.slidelang")
			}
			var types []string
			for _, el := range doc.ContentBlocks[0].Elements {
				switch e := el.(type) {
				case *ast.TextElement:
					if e.Content != "" {
						t.Errorf("the empty TEXT took content: %q", e.Content)
					}
					types = append(types, "text")
				case *ast.PointsElement:
					types = append(types, "points")
				case *ast.ChecklistElement:
					types = append(types, "checklist")
				}
			}
			if len(types) < 2 || types[len(types)-2] != "points" || types[len(types)-1] != "checklist" {
				t.Errorf("want POINTS and CHECKLIST after the TEXT, got %v", types)
			}
		})
	}
}

// La prosa escrita a la misma sangría que TEXT sigue siendo su cuerpo, como
// siempre. En documento, slide y columna tipada.
func TestStrictText_SameIndentBodyStillWorks(t *testing.T) {
	for name, tc := range sameIndentCases("  hola mundo\n  sigue\n") {
		t.Run(name, func(t *testing.T) {
			if got := strictTexts(t, tc.src, tc.document); len(got) != 1 || got[0] != "hola mundo sigue" {
				t.Errorf("want the TEXT \"hola mundo sigue\", got %q", got)
			}
		})
	}
}

// Solo la primera línea del cuerpo puede cortarlo por tener forma de
// elemento. Una vez que el cuerpo a la misma sangría empezó, una línea
// posterior con forma de elemento (POINTS, CODE, ":::", una palabra clave
// pegada) se lee como siempre: es parte del párrafo. En documento, slide y
// columna tipada.
func TestStrictText_OnlyFirstBodyLineCanEndIt(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"  hola mundo\n  POINTS\n    - uno\n", "hola mundo POINTS - uno"},
		{"  hola mundo\n  CODE js\n    x\n", "hola mundo CODE js x"},
		{"  hola mundo\n  ::: note\n  hi\n  :::\n", "hola mundo ::: note hi :::"},
		{"  hola\n  TEXTO pegado\n", "hola TEXTO pegado"},
		{"  hola\n\n  QUOTE\n    Cita.\n", "hola QUOTE Cita."},
	} {
		for name, c := range sameIndentCases(tc.body) {
			t.Run(name+"/"+tc.want, func(t *testing.T) {
				if got := strictTexts(t, c.src, c.document); len(got) != 1 || got[0] != tc.want {
					t.Errorf("want the TEXT %q, got %q", tc.want, got)
				}
			})
		}
	}
}

type strictCase struct {
	src      string
	document bool
}

// sameIndentCases arma un TEXT con body (escrito con la sangría de un
// elemento de sección, dos espacios) en documento, slide y columna tipada.
func sameIndentCases(body string) map[string]strictCase {
	column := ""
	for _, l := range strings.SplitAfter(body, "\n") {
		if strings.TrimSpace(l) != "" {
			l = "  " + l
		}
		column += l
	}
	return map[string]strictCase{
		"document":     {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n" + body, true},
		"slide":        {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n" + body, false},
		"typed column": {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  <<grid>>\n  <<column typed>>\n    TEXT\n" + column + "  <<end>>\n", true},
	}
}

// strictTexts parsea src sin errores y devuelve el contenido de cada TEXT.
func strictTexts(t *testing.T, src string, document bool) []string {
	t.Helper()
	p := New(util.NewNoop())
	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if document {
		doc, diags = p.ParseDocument(src, "d.doclang")
	} else {
		doc, diags = p.Parse(src, "s.slidelang")
	}
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error: %v", d)
		}
	}
	var texts []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		if e, ok := n.(*ast.TextElement); ok {
			texts = append(texts, e.Content)
		}
		return nil
	})
	return texts
}

// Tras un TEXT vacío, una línea a su misma sangría que solo empieza con una
// palabra clave ("TEXTO pegado", "POINTSX pegado", "QUOTEado") no abre ningún
// elemento: es el cuerpo del TEXT, como en main. Lo mismo una palabra clave en
// una forma que su elemento no acepta ("TEXT hola", "QUOTE de Ana", "CODE:x",
// "MERMAID", "TABLE OF CONTENTS") y una línea simbólica que no es un elemento
// ("<<foo>> bar", "@usuario dijo hola", "| hola"): en posición de elemento
// perderían el texto o darían error. Con la línea pegada al TEXT y con una
// línea en blanco entre los dos (main los leía igual en los dos casos, como
// párrafo), en documento, slide y columna tipada.
func TestStrictText_EmptyBodyThenGluedKeywordIsProse(t *testing.T) {
	words := []string{
		"TEXTO pegado", "POINTSX pegado", "QUOTEado", "TEXT hola", "QUOTE de Ana",
		"POINTS x", "CODE:x", "MERMAID", "TABLE OF CONTENTS", "IMAGEN",
		"<<foo>> bar", "@usuario dijo hola", "| hola",
	}
	for _, word := range words {
		for _, sep := range []string{"\n", "\n\n"} {
			for name, c := range sameIndentCases(sep + "  " + word + "\n") {
				t.Run(name+"/"+word, func(t *testing.T) {
					got, diags := elementShapes(t, c.src, c.document)
					if want := "text:" + word; strings.Join(got, "|") != want {
						t.Errorf("sep %q: got %v, want [%s] as in main", sep, got, want)
					}
					if len(diags) != 0 {
						t.Errorf("sep %q: unexpected diagnostics %v", sep, diags)
					}
				})
			}
		}
	}
	// En un documento, SECTION indentado no es un encabezado (es un error en
	// posición de elemento): tras un TEXT vacío sigue siendo prosa.
	got, _ := elementShapes(t, "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n  SECTION \"Sub\"\n", true)
	if want := `text:SECTION "Sub"`; strings.Join(got, "|") != want {
		t.Errorf("document SECTION: got %v, want [%s]", got, want)
	}
}

// Tras un TEXT vacío, una línea a su misma sangría que abre un elemento en la
// forma que ese elemento acepta se despacha como elemento, igual que sin el
// TEXT delante. El elemento sin TEXT delante no pierde nada: no da errores ni
// avisos de contenido descartado o ignorado. En documento, slide y columna
// tipada, pegado al TEXT y con línea en blanco.
func TestStrictText_EmptyBodyThenRealElementIsDispatched(t *testing.T) {
	els := []string{
		"  POINTS\n    - uno\n",
		"  QUOTE\n    Cita.\n",
		"  CHECKLIST\n    - [ ] dos\n",
		"  CODE js\n    x\n",
		"  CODE{verbatim} js\n    x\n",
		"  IMAGE foto.png\n",
		"  TABLE\n    headers: [a]\n",
		"  | a |\n  |---|\n  | 1 |\n",
		"  | a | b |\n",
		"  ::: note\n  hola\n  :::\n",
		"  <<math>>\n  x^2\n  <<end>>\n",
		"  <<mermaid>>\n    graph TD\n    A --> B\n  <<end>>\n",
		"  @center\n",
	}
	for _, el := range els {
		for name, bare := range sameIndentCases(el) {
			// sameIndentCases antepone un TEXT; el caso sin TEXT lo quita.
			bareSrc := withoutTextLine(bare.src)
			want, diags := elementShapes(t, bareSrc, bare.document)
			for _, d := range diags {
				if d.IsError() || d.RuleID == "DIRECTIVE003" || d.RuleID == "STRICT003" {
					t.Fatalf("%s %q: fixture loses content: %v", name, el, d)
				}
			}
			for _, sep := range []string{"", "\n"} {
				c := sameIndentCases(sep + el)[name]
				if got, _ := elementShapes(t, c.src, c.document); strings.Join(got, "|") != strings.Join(want, "|") {
					t.Errorf("%s %q after an empty TEXT (blank %q): got %v, want %v as without the TEXT", name, el, sep, got, want)
				}
			}
		}
	}
	// En un SLIDE, SECTION "…" es un encabezado: corta el TEXT vacío.
	slide := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n"
	want, _ := elementShapes(t, slide+"  SECTION \"Sub\"\n", false)
	if got, _ := elementShapes(t, slide+"  TEXT\n  SECTION \"Sub\"\n", false); strings.Join(got, "|") != strings.Join(want, "|") || len(want) != 1 {
		t.Errorf("slide SECTION: got %v, want %v", got, want)
	}
}

// elementShapes parsea src y devuelve tipo y contenido de cada elemento hoja,
// sin posiciones ni node-ids (los grids y sus columnas no cuentan), y los
// diagnósticos que no son TEXT001.
func elementShapes(t *testing.T, src string, document bool) ([]string, []diagnostics.Diagnostic) {
	t.Helper()
	p := New(util.NewNoop())
	var doc *ast.AST
	var all []diagnostics.Diagnostic
	if document {
		doc, all = p.ParseDocument(src, "d.doclang")
	} else {
		doc, all = p.Parse(src, "s.slidelang")
	}
	var diags []diagnostics.Diagnostic
	for _, d := range all {
		if d.RuleID != "TEXT001" {
			diags = append(diags, d)
		}
	}
	var out []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch e := n.(type) {
		case *ast.TextElement:
			out = append(out, "text:"+e.Content)
		case *ast.QuoteElement:
			out = append(out, "quote:"+e.Content)
		case *ast.PointsElement:
			out = append(out, fmt.Sprintf("points:%d", len(e.Items)))
		case *ast.ChecklistElement:
			out = append(out, fmt.Sprintf("checklist:%d", len(e.Items)))
		case *ast.CodeElement:
			out = append(out, "code:"+e.Language+":"+e.Content)
		case *ast.ImageElement:
			out = append(out, "image:"+e.Source)
		case *ast.TableElement:
			out = append(out, fmt.Sprintf("table:%v:%v", e.Headers, e.Rows))
		case *ast.SpecialBlockElement:
			out = append(out, "special:"+e.BlockType+":"+e.Content)
		case *ast.DirectiveNode:
			out = append(out, "directive:"+e.Name)
		case *ast.MathElement, *ast.MermaidElement, *ast.HeadingElement:
			out = append(out, fmt.Sprintf("%T", e))
		}
		return nil
	})
	return out, diags
}

// withoutTextLine quita de src la primera línea que es solo "TEXT".
func withoutTextLine(src string) string {
	lines := strings.SplitAfter(src, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "TEXT" {
			return strings.Join(append(lines[:i:i], lines[i+1:]...), "")
		}
	}
	return src
}

// Un TEXT sin contenido da TEXT001 como warning en los dos dialectos: en un
// documento strict un error rompería documentos que compilaban.
func TestStrictText_EmptyTextIsReported(t *testing.T) {
	_, diags := New(util.NewNoop()).ParseDocument("---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n  POINTS\n    - uno\n", "d.doclang")
	if len(diags) != 1 || diags[0].RuleID != "TEXT001" || !diags[0].IsWarning() || diags[0].Position.Line != 7 {
		t.Errorf("document: want one TEXT001 warning at line 7, got %v", diags)
	}
	_, diags = New(util.NewNoop()).Parse("---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n  POINTS\n    - uno\n", "s.slidelang")
	if len(diags) != 1 || diags[0].RuleID != "TEXT001" || !diags[0].IsWarning() {
		t.Errorf("slide: want one TEXT001 warning, got %v", diags)
	}
}

// Un TEXT sin cuerpo seguido de una línea en blanco no se traga al TEXT ni a
// la QUOTE que vienen después: los dos siguen siendo elementos propios, con su
// contenido. El TEXT vacío no da elemento, solo TEXT001.
func TestStrictText_EmptyBodyThenBlankLineKeepsSiblings(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n  TEXT\n\n  TEXT\n    Segundo.\n\n  QUOTE\n    Cita.\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "d.doclang")
	if len(diags) != 1 || diags[0].RuleID != "TEXT001" || diags[0].Position.Line != 7 {
		t.Errorf("want one TEXT001 at line 7, got %v", diags)
	}
	var texts []string
	var quotes []string
	for _, el := range doc.ContentBlocks[0].Elements {
		switch e := el.(type) {
		case *ast.TextElement:
			texts = append(texts, e.Content)
		case *ast.QuoteElement:
			quotes = append(quotes, e.Content)
		}
	}
	if len(texts) != 1 || texts[0] != "Segundo." {
		t.Errorf("want only the TEXT \"Segundo.\", got %q", texts)
	}
	if len(quotes) != 1 || quotes[0] != "Cita." {
		t.Errorf("want the QUOTE \"Cita.\", got %q", quotes)
	}
}

// Casos de la revisión de #499: tras un TEXT vacío, una línea con forma de
// elemento seguida de prosa a su misma sangría (o de algo que el elemento no
// acepta) no corta el cuerpo. Leerla como elemento perdería la prosa,
// reinterpretaría el texto o rompería el build (un ":::" sin cerrar, un
// "::: grid"); main la leía como el párrafo del TEXT, con todas sus líneas, y
// así sigue. En documento, slide y columna
// tipada.
func TestStrictText_EmptyBodyCutNeverLosesText(t *testing.T) {
	for _, body := range []string{
		"  POINTS\n  Some prose after\n",
		"  POINTS\n\n  Some prose after\n",
		"\n  POINTS\n  Some prose after\n",
		"  IMAGE foto.png\n  Some prose after\n",
		"  IMAGE\n  prose\n",
		"  TABLE\n  Some prose after\n",
		"  IMAGE of the year\n",
		"  :::\n",
		"  CODE red alert now\n  more text\n",
		"  @center\n  Some prose after\n",
		"  @notes hi\n  Some prose after\n",
		"  | a | b |\n  Some prose after\n",
		"  CODE js\n  Some prose after\n",
		"  :::note\n  hola\n  :::\n  Some prose after\n",
		"  <<math>>\n  x^2\n  <<end>>\n  Some prose after\n",
		"  CHECKLIST\n  Some prose after\n",
		"  POINTS\n    - uno\n  Some prose after\n",
		"  POINTS\n    - uno\n\n  Some prose after\n",
		"  POINTS\n    no es un item\n",
		"  QUOTE\n    Cita.\n  nota: algo\n",
		"  :::x hola\n  sigue\n",
		"  ::: grid\n  ::: column\n  a\n  :::\n  :::\n",
		// Re-revisión de 2b51764: el texto se pierde aunque sus palabras
		// aparezcan en otra parte del elemento, o tiene tokens de un carácter.
		"  POINTS\n    - apple pie\n    pie apple\n",
		"  POINTS\n    - Costo\n    5 € + 3 %\n",
		"  POINTS\n    - Plan A\n    o B\n",
		"  POINTS\n    - apple pie\n    apple\n  QUOTE\n    pie\n",
		"  IMAGE foto.png\n    alt: perro\n    caption: perro\n",
		// Tercera revisión (83070de): líneas sin tokens o con solo una clave.
		"  POINTS\n    - uno\n    Importante:\n",
		"  POINTS\n    - uno\n    Nota: -> **\n",
		"  IMAGE foto.png\n    Leyenda:\n",
		"  POINTS\n    - uno\n    ---\n",
		"  POINTS\n    - uno\n    \"\"\n",
		"  POINTS\n    - hola\n    -> <> [] {} ## **\n",
		"  IMAGE foto.png\n    caption: perro\n    caption: perro\n",
		// Cuarta revisión (7dbaf55): una línea de solo signos tiene que ser
		// una línea entera del elemento, y una clave cuenta como propiedad
		// solo si el parser del elemento la lee.
		"  POINTS\n    - uno -> dos\n    ->\n",
		"  POINTS\n    - **uno**\n    **\n",
		"  POINTS\n    - a -- b -- c\n    --\n    --\n",
		"  POINTS\n    - uno\n    listType: unordered\n",
	} {
		var want []string
		for _, l := range strings.Split(body, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				want = append(want, l)
			}
		}
		for name, c := range sameIndentCases(body) {
			t.Run(name+"/"+body, func(t *testing.T) {
				got, diags := elementShapes(t, c.src, c.document)
				if w := "text:" + strings.Join(want, " "); strings.Join(got, "|") != w {
					t.Errorf("got %v, want [%s] as in main", got, w)
				}
				if len(diags) != 0 {
					t.Errorf("unexpected diagnostics %v", diags)
				}
			})
		}
	}
}

// Propiedad: tras un TEXT vacío, cualquier secuencia de líneas a la sangría
// del TEXT o más (que main leía entera como el párrafo del TEXT, sin perder
// nada) conserva cada palabra y cada signo que trae, con sus repeticiones,
// y cada línea que solo trae una clave o solo signos, sin errores ni avisos
// fuera de TEXT001, se corte o no el cuerpo. El vocabulario repite palabras y
// trae tokens de un carácter y signos, para que una palabra perdida no pueda
// esconderse detrás de otra igual. En
// documento, slide y columna tipada.
func TestStrictText_EmptyBodyCutKeepsEveryWord(t *testing.T) {
	same := []string{
		"TEXT", "POINTS", "CHECKLIST", "QUOTE", "TABLE", "IMAGE", "IMAGE foto.png", "IMAGE @ @",
		"CODE js", "CODE{verbatim} go", "CODE @ @ @", "@center", "@notes @", "@usuario @",
		"| @ | @ |", "|---|---|", ":::note", ":::", "<<math>>", "<<end>>", "<<foo>> @",
		"@ @", "TEXTO @", "QUOTE @", "clave: @", "",
	}
	deeper := []string{
		"- @ @", "- [ ] @", "- [x] @", "@ @", "@", "src: foto.png", "alt: @", "caption: @",
		"headers: [@]", "rows:", "- [@]", "AUTHOR: @", "1. @", "",
		"Importante:", "Leyenda:", "Nota: -> **", "---", `""`, "-> <> [] {} ## **",
	}
	// Las líneas que solo traen una clave o solo signos no tienen palabras
	// del vocabulario: se exige que estén tal cual en el AST.
	literal := map[string]bool{"Importante:": true, "Leyenda:": true, "Nota: -> **": true, "---": true, `""`: true, "-> <> [] {} ## **": true, "TEXTO @": false}
	vocab := []string{"apple", "pie", "A", "B", "o", "5", "3", "€", "+", "%", "perro"}
	rng := rand.New(rand.NewSource(499))
	for i := 0; i < 1500; i++ {
		want := map[string]int{}
		fill := func(tpl string) string {
			// "@" al principio de la plantilla es la directiva; los demás
			// son palabras del vocabulario.
			parts := strings.Split(tpl, "@")
			var b strings.Builder
			for j, part := range parts {
				b.WriteString(part)
				if j == len(parts)-1 {
					break
				}
				if j == 0 && part == "" && len(parts) > 1 && strings.HasPrefix(tpl, "@") && tpl != "@" && !strings.HasPrefix(tpl, "@ ") {
					b.WriteString("@")
					continue
				}
				w := vocab[rng.Intn(len(vocab))]
				want[w]++
				b.WriteString(w)
			}
			return b.String()
		}
		var b strings.Builder
		if rng.Intn(3) == 0 {
			b.WriteString("\n")
		}
		// La primera línea con texto va a la sangría del TEXT: si fuera más
		// sangrada, sería el cuerpo del TEXT y las líneas siguientes a su
		// sangría caerían en posición de elemento también en main.
		b.WriteString("  " + fill(same[rng.Intn(len(same)-1)]) + "\n")
		for k := rng.Intn(6); k > 0; k-- {
			if rng.Intn(2) == 0 {
				if l := fill(same[rng.Intn(len(same))]); l != "" {
					b.WriteString("  " + l + "\n")
				} else {
					b.WriteString("\n")
				}
			} else if l := fill(deeper[rng.Intn(len(deeper))]); l != "" {
				b.WriteString("    " + l + "\n")
			} else {
				b.WriteString("\n")
			}
		}
		body := b.String()
		wantLines := map[string]int{}
		for _, l := range strings.Split(body, "\n") {
			if l = strings.TrimSpace(l); literal[l] {
				wantLines[l]++
			}
		}
		for name, c := range sameIndentCases(body) {
			got, diags := elementShapes(t, c.src, c.document)
			for _, d := range diags {
				t.Errorf("%s %q: unexpected diagnostic %v (got %v)", name, body, d, got)
			}
			strs := astStrings(t, c.src, c.document)
			for l, n := range wantLines {
				if strings.Count(strs, l) < n {
					t.Errorf("%s %q: line %q appears %d times, want %d (got %v)", name, body, l, strings.Count(strs, l), n, got)
				}
			}
			have := emptyTextASTTokens(t, c.src, c.document)
			for w, n := range want {
				if have[w] < n {
					t.Errorf("%s %q: %q appears %d times, want %d (got %v)", name, body, w, have[w], n, got)
					break
				}
			}
		}
	}
}

// Una entrada grande, con cientos de miles de TEXT vacíos seguidos o
// alternados con elementos, se lee sin desbordar la pila y en tiempo lineal:
// la prueba del corte no es recursiva. Con 2b51764, las dos primeras daban
// "fatal error: stack overflow".
func TestStrictText_EmptyBodyCutLargeInput(t *testing.T) {
	head := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n"
	for name, src := range map[string]string{
		"empty TEXT lines": head + strings.Repeat("  TEXT\n", 250000),
		"TEXT then POINTS": head + strings.Repeat("  TEXT\n  POINTS\n    - a\n", 200000),
		"TEXT then prose":  head + strings.Repeat("  TEXT\n  POINTS\n  prosa\n", 20000),
		"TEXT then QUOTE":  head + strings.Repeat("  TEXT\n  QUOTE\n    cita\n", 20000),
	} {
		t.Run(name, func(t *testing.T) {
			// Con el presupuesto de siempre y sin él: sin él, la prueba recorre
			// la entrada entera, que es lo que desbordaba la pila.
			for _, budget := range []int{emptyTextTrialBudget, 1 << 30} {
				func() {
					defer func(b int) { emptyTextTrialBudget = b }(emptyTextTrialBudget)
					emptyTextTrialBudget = budget
					doc, _ := New(util.NewNoop()).ParseDocument(src, "d.doclang")
					if doc == nil || len(doc.ContentBlocks) != 1 {
						t.Fatalf("budget %d: no document", budget)
					}
				}()
			}
		})
	}
}

// Un <!-- node-id --> justo antes de un TEXT vacío se enlazaba en main al
// párrafo del TEXT. El TEXT no suelta lo que sigue, así que el id sigue
// teniendo su nodo y el build no falla.
func TestStrictText_EmptyBodyWithNodeIDKeepsParagraph(t *testing.T) {
	for name, tc := range map[string]struct {
		src      string
		document bool
	}{
		"document": {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  <!-- node-id: abc -->\n  TEXT\n  POINTS\n    - a\n", true},
		"slide":    {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  <!-- node-id: abc -->\n  TEXT\n  POINTS\n    - a\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			p := New(util.NewNoop())
			var doc *ast.AST
			var diags []diagnostics.Diagnostic
			if tc.document {
				doc, diags = p.ParseDocument(tc.src, "d.doclang")
			} else {
				doc, diags = p.Parse(tc.src, "s.slidelang")
			}
			for _, d := range diags {
				if d.IsError() {
					t.Errorf("unexpected error: %v", d)
				}
			}
			var texts []*ast.TextElement
			_ = ast.Walk(doc, func(n ast.Node) error {
				if e, ok := n.(*ast.TextElement); ok {
					texts = append(texts, e)
				}
				return nil
			})
			if len(texts) != 1 || texts[0].Content != "POINTS - a" || texts[0].GetNodeID() != "abc" {
				t.Errorf("want the paragraph \"POINTS - a\" with node-id abc, got %+v", texts)
			}
		})
	}
}

// Los anchors que registra un parse de prueba no quedan en el deck: un
// encabezado leído a prueba y rechazado (lo sigue prosa) no le cambia el
// anchor al encabezado real del slide siguiente.
func TestStrictText_RejectedTrialLeavesNoAnchors(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"S\"\n  SECTION \"Intro\"\n\nSLIDE content\n  title: \"T\"\n  TEXT\n  SECTION \"Sub\"\n  prosa\n\nSLIDE content\n  title: \"U\"\n  SECTION \"Sub\"\n"
	doc, diags := New(util.NewNoop()).Parse(src, "s.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error: %v", d)
		}
	}
	var html []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		if e, ok := n.(*ast.TextElement); ok {
			html = append(html, e.Content)
		}
		return nil
	})
	if len(html) != 3 || html[1] != `SECTION "Sub" prosa` || !strings.Contains(html[2], `id="heading-sub"`) {
		t.Errorf("want the paragraph and a heading with anchor heading-sub, got %q", html)
	}
}

// emptyTextASTTokens cuenta los tokens (como los cuenta accountsFor) de las cadenas
// del AST de src que son texto del autor.
func emptyTextASTTokens(t *testing.T, src string, document bool) map[string]int {
	t.Helper()
	p := New(util.NewNoop())
	var doc *ast.AST
	if document {
		doc, _ = p.ParseDocument(src, "d.doclang")
	} else {
		doc, _ = p.Parse(src, "s.slidelang")
	}
	raw, err := json.Marshal(doc.ContentBlocks)
	if err != nil {
		t.Fatal(err)
	}
	var tree interface{}
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	have := map[string]int{}
	collectTokens(tree, have)
	return have
}

// collectTokens suma a have los tokens de cada cadena de v, un valor JSON
// decodificado, salvo las de derivedOrEnumField y los campos *HTML.
func collectTokens(v interface{}, have map[string]int) {
	switch t := v.(type) {
	case string:
		for _, tok := range tokens(t) {
			have[tok]++
		}
	case []interface{}:
		for _, e := range t {
			collectTokens(e, have)
		}
	case map[string]interface{}:
		for k, e := range t {
			if derivedOrEnumField[k] || strings.HasSuffix(k, "HTML") {
				continue
			}
			collectTokens(e, have)
		}
	}
}

// astStrings junta las cadenas del AST de src que son texto del autor, una
// por línea.
func astStrings(t *testing.T, src string, document bool) string {
	t.Helper()
	p := New(util.NewNoop())
	var doc *ast.AST
	if document {
		doc, _ = p.ParseDocument(src, "d.doclang")
	} else {
		doc, _ = p.Parse(src, "s.slidelang")
	}
	var texts []string
	collectElementTokens(reflect.ValueOf(doc.ContentBlocks), map[string]int{}, &texts)
	return strings.Join(texts, "\n")
}

// Los pasos de una misma prueba ven los anchors de los anteriores: dos
// encabezados leídos a prueba con el mismo id son un error en el parse real,
// así que el TEXT no suelta lo que sigue y queda el párrafo de main, sin
// "duplicate heading id".
func TestStrictText_TrialStepsShareAnchors(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n  SECTION \"B\"\n    id: x\n  SECTION \"C\"\n    id: x\n"
	got, diags := elementShapes(t, src, false)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic %v", d)
	}
	if want := `text:SECTION "B" id: x SECTION "C" id: x`; strings.Join(got, "|") != want {
		t.Errorf("got %v, want [%s] as in main", got, want)
	}
}

// Varios encabezados de prueba con ids distintos cortan: un paso que ya se
// leyó no se vuelve a leer al conocerse el resultado del siguiente, así que
// sus anchors no se registran dos veces.
func TestStrictText_TrialStepsWithDistinctIDsCut(t *testing.T) {
	slide := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n"
	body := "  SECTION \"B\"\n    id: x\n  SECTION \"C\"\n    id: y\n"
	want, wantDiags := elementShapes(t, slide+body, false)
	if len(wantDiags) != 0 || len(want) != 2 {
		t.Fatalf("fixture: %v %v", want, wantDiags)
	}
	got, diags := elementShapes(t, slide+"  TEXT\n"+body, false)
	if len(diags) != 0 || strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %v %v, want %v as without the TEXT", got, diags, want)
	}
}

// Con el presupuesto de líneas de prueba agotado, el TEXT vacío no suelta lo
// que sigue: queda el párrafo de main, sin perder nada.
func TestStrictText_TrialBudgetFallsBackToParagraph(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n  POINTS\n    - uno\n  QUOTE\n    Cita.\n  CHECKLIST\n    - [ ] dos\n"
	if got, _ := elementShapes(t, src, true); len(got) != 3 {
		t.Fatalf("with the default budget the TEXT cuts: got %v", got)
	}
	defer func(b int) { emptyTextTrialBudget = b }(emptyTextTrialBudget)
	emptyTextTrialBudget = 3
	got, diags := elementShapes(t, src, true)
	if want := "text:POINTS - uno QUOTE Cita. CHECKLIST - [ ] dos"; strings.Join(got, "|") != want || len(diags) != 0 {
		t.Errorf("got %v %v, want [%s]", got, diags, want)
	}
}

// Escala: un CODE o un bloque ":::" con 80 000 líneas sin tokens tras un TEXT
// vacío se prueban en tiempo y memoria lineales (antes, la comparación tal
// cual reconstruía la cadena por línea: 25 y 46 GB asignados). Sin
// presupuesto, para medir la prueba entera; con el de siempre, el TEXT queda
// como párrafo y tampoco cuesta.
func TestStrictText_EmptyBodyCutScalesLinearly(t *testing.T) {
	head := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n"
	inputs := map[string]func(n int) string{
		"CODE": func(n int) string {
			return head + "  TEXT\n  CODE js\n" + strings.Repeat("    x\n    }\n", n)
		},
		"special block": func(n int) string {
			return head + "  TEXT\n  :::note\n" + strings.Repeat("  hola\n  --\n", n) + "  :::\n"
		},
	}
	measure := func(src string) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		New(util.NewNoop()).ParseDocument(src, "d.doclang")
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	for name, input := range inputs {
		t.Run(name, func(t *testing.T) {
			for _, budget := range []int{1 << 30, emptyTextTrialBudget} {
				func() {
					defer func(b int) { emptyTextTrialBudget = b }(emptyTextTrialBudget)
					emptyTextTrialBudget = budget
					small, large := input(20000), input(80000)
					a, b := measure(small), measure(large)
					// Lineal: 4 veces la entrada, unas 4 veces la memoria. Con
					// el costo cuadrático de antes eran miles de veces.
					if b > 8*a || b > 400*uint64(len(large)) {
						t.Errorf("budget %d: %d bytes for n=20k, %d for n=80k (%d bytes of input)", budget, a, b, len(large))
					}
				}()
			}
		})
	}
}
