// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
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
