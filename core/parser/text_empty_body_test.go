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

// Un TEXT strict se lee como siempre: su cuerpo son las líneas que le siguen
// con sangría, que puede ser la suya. Así, un TEXT sin cuerpo propio seguido
// de un elemento a su misma sangría se lleva ese elemento, y lo que le sigue a
// esa sangría, como su párrafo (`TEXT` / `POINTS` / `- uno` es el párrafo
// "POINTS - uno"). Ese caso no cambia de lectura, pero avisa: TEXT002.

// Un TEXT sin cuerpo propio seguido de elementos a su misma sangría los lee
// como su párrafo, como en main, y da un TEXT002 que va de la línea TEXT a la
// última línea del párrafo. No da TEXT001: el TEXT no está vacío.
func TestStrictText_EmptyBodyAbsorbsSiblingsWithTEXT002(t *testing.T) {
	for name, tc := range map[string]struct {
		src      string
		document bool
		textLine int
	}{
		"document": {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n  POINTS\n    - uno\n  CHECKLIST\n    - [ ] dos\n", true, 7},
		"slide":    {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n  POINTS\n    - uno\n  CHECKLIST\n    - [ ] dos\n", false, 7},
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
			var texts []string
			for _, el := range doc.ContentBlocks[0].Elements {
				if e, ok := el.(*ast.TextElement); ok {
					texts = append(texts, e.Content)
				} else {
					t.Errorf("want only the paragraph, got a %T", el)
				}
			}
			if want := "POINTS - uno CHECKLIST - [ ] dos"; len(texts) != 1 || texts[0] != want {
				t.Errorf("want the paragraph %q as in main, got %q", want, texts)
			}
			if len(diags) != 1 {
				t.Fatalf("want one TEXT002, got %v", diags)
			}
			d := diags[0]
			if d.RuleID != "TEXT002" || !d.IsWarning() || d.Position.Line != tc.textLine ||
				d.EndPosition == nil || d.EndPosition.Line != tc.textLine+4 {
				t.Errorf("want a TEXT002 warning from line %d to %d, got %v", tc.textLine, tc.textLine+4, d)
			}
			// El mensaje nombra el elemento y su línea por el contenido, sin
			// números de línea (un @include o un node-id los moverían).
			if !strings.Contains(d.Message, "POINTS element") || !strings.Contains(d.Message, `"POINTS"`) ||
				strings.ContainsAny(d.Message, "0123456789") {
				t.Errorf("message should name the POINTS line by its content, with no line numbers: %q", d.Message)
			}
		})
	}
}

// La prosa escrita a la misma sangría que TEXT sigue siendo su cuerpo, como
// siempre, y no avisa. En documento, slide y columna tipada.
func TestStrictText_SameIndentBodyStillWorks(t *testing.T) {
	for name, tc := range sameIndentCases("  hola mundo\n  sigue\n") {
		t.Run(name, func(t *testing.T) {
			if got := strictTexts(t, tc.src, tc.document); len(got) != 1 || got[0] != "hola mundo sigue" {
				t.Errorf("want the TEXT \"hola mundo sigue\", got %q", got)
			}
			if _, diags := elementShapes(t, tc.src, tc.document); len(diags) != 0 {
				t.Errorf("unexpected diagnostics %v", diags)
			}
		})
	}
}

// Una línea con forma de elemento después de la primera línea del cuerpo es
// parte del párrafo, como siempre, y no avisa: el TEXT tiene cuerpo propio.
// En documento, slide y columna tipada.
func TestStrictText_LaterElementLineIsParagraph(t *testing.T) {
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
				if _, diags := elementShapes(t, c.src, c.document); len(diags) != 0 {
					t.Errorf("unexpected diagnostics %v", diags)
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
// elemento: es el cuerpo del TEXT, como en main, y no avisa. Lo mismo una
// palabra clave en una forma que su elemento no acepta ("TEXT hola", "QUOTE
// de Ana", "CODE:x", "MERMAID", "TABLE OF CONTENTS", "IMAGE of the year") y
// una línea simbólica que no es un elemento ("<<foo>> bar", "@usuario dijo
// hola", "| hola", un ":::" solo, "::: grid"): ahí no hay un elemento bien
// escrito que nombrar. Con la línea pegada al TEXT y con una línea en blanco
// entre los dos, en documento, slide y columna tipada.
func TestStrictText_EmptyBodyThenGluedKeywordIsProse(t *testing.T) {
	words := []string{
		"TEXTO pegado", "POINTSX pegado", "QUOTEado", "TEXT hola", "QUOTE de Ana",
		"POINTS x", "CODE:x", "MERMAID", "TABLE OF CONTENTS", "IMAGEN", "IMAGE of the year",
		"CODE red alert now", "<<foo>> bar", "@usuario dijo hola", "| hola", ":::", "::: grid",
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
	// En un documento, SECTION indentado no es un encabezado: tras un TEXT
	// vacío es prosa y no avisa.
	got, diags := elementShapes(t, "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n  SECTION \"Sub\"\n", true)
	if want := `text:SECTION "Sub"`; strings.Join(got, "|") != want || len(diags) != 0 {
		t.Errorf("document SECTION: got %v %v, want [%s] and no diagnostics", got, diags, want)
	}
}

// Tras un TEXT vacío, una línea a su misma sangría que abre un elemento en la
// forma que ese elemento acepta sigue siendo el párrafo del TEXT, como en
// main, y da un TEXT002. En documento, slide y columna tipada, pegada al TEXT
// y con línea en blanco.
func TestStrictText_EmptyBodyThenRealElementIsParagraphWithTEXT002(t *testing.T) {
	els := []string{
		"  POINTS\n    - uno\n",
		"  QUOTE\n    Cita.\n",
		"  CHECKLIST\n    - [ ] dos\n",
		"  CODE js\n    x\n",
		"  CODE{verbatim} js\n    x\n",
		"  IMAGE foto.png\n",
		"  IMAGE\n",
		"  TABLE\n    headers: [a]\n",
		"  | a |\n  |---|\n  | 1 |\n",
		"  | a | b |\n",
		"  ::: note\n  hola\n  :::\n",
		"  <<math>>\n  x^2\n  <<end>>\n",
		"  <<mermaid>>\n    graph TD\n    A --> B\n  <<end>>\n",
		"  @center\n",
		"  TEXT\n    hola\n",
		// La forma documentada de IMAGE, con fuente y texto alternativo.
		"  IMAGE \"logo.png\" \"alt\"\n",
		"  IMAGE foto.png \"alt\"\n",
		// Un ":" pelado tras la palabra clave se lee igual que la palabra sola.
		"  TABLE:\n    | a | b |\n",
		"  POINTS:\n    - uno\n",
		"  CHECKLIST:\n    - [ ] dos\n",
		"  QUOTE:\n    Cita.\n",
		"  TEXT:\n    hola\n",
		"  IMAGE:\n",
		"  IMAGE: foto.png\n",
		"  CODE: python\n    x\n",
	}
	for _, el := range els {
		var want []string
		for _, l := range strings.Split(el, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				want = append(want, l)
			}
		}
		for _, sep := range []string{"", "\n"} {
			for name, c := range sameIndentCases(sep + el) {
				t.Run(name+"/"+want[0], func(t *testing.T) {
					got, diags := elementShapes(t, c.src, c.document)
					if w := "text:" + strings.Join(want, " "); strings.Join(got, "|") != w {
						t.Errorf("got %v, want [%s] as in main", got, w)
					}
					if len(diags) != 1 || diags[0].RuleID != "TEXT002" || !diags[0].IsWarning() {
						t.Errorf("want one TEXT002 warning, got %v", diags)
					} else if kw := strings.TrimSuffix(strings.Fields(want[0])[0], ":"); want[0][0] != '|' && want[0][0] != ':' &&
						want[0][0] != '<' && want[0][0] != '@' && !strings.Contains(diags[0].Message, "the "+strings.TrimSuffix(kw, "{verbatim}")+" element") {
						t.Errorf("the message should name the %s element: %q", kw, diags[0].Message)
					}
				})
			}
		}
	}
	// En un SLIDE, SECTION "…" es un encabezado: también avisa.
	got, diags := elementShapes(t, "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n  SECTION \"Sub\"\n", false)
	if strings.Join(got, "|") != `text:SECTION "Sub"` || len(diags) != 1 || diags[0].RuleID != "TEXT002" {
		t.Errorf("slide SECTION: got %v %v", got, diags)
	}
}

// Un TEXT más sangrado que el elemento que le sigue también se lo lleva como
// párrafo: main toma como cuerpo la primera línea con sangría aunque esté
// menos sangrada que TEXT. Avisa igual, con el rango hasta la última línea.
func TestStrictText_LessIndentedElementIsParagraphWithTEXT002(t *testing.T) {
	for name, tc := range map[string]struct {
		src      string
		document bool
	}{
		"slide":    {"---\nmode: strict\n---\n\nSLIDE content\n  title: \"F\"\n    TEXT\n  POINTS\n    - lower\n", false},
		"document": {"---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n    TEXT\n  POINTS\n    - lower\n", true},
	} {
		t.Run(name, func(t *testing.T) {
			got, diags := elementShapes(t, tc.src, tc.document)
			if strings.Join(got, "|") != "text:POINTS - lower" {
				t.Errorf("got %v, want the paragraph of main", got)
			}
			if len(diags) != 1 || diags[0].RuleID != "TEXT002" || diags[0].Position.Line != 7 ||
				diags[0].EndPosition == nil || diags[0].EndPosition.Line != 9 || !strings.Contains(diags[0].Message, `the POINTS element on the line "POINTS"`) {
				t.Errorf("want one TEXT002 from line 7 to 9 naming POINTS, got %v", diags)
			}
		})
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

// Un TEXT sin contenido da TEXT001 como warning en los dos dialectos: en un
// documento strict un error rompería documentos que compilaban. Uno que
// absorbió algo no está vacío: da TEXT002 y no TEXT001.
func TestStrictText_EmptyTextIsReported(t *testing.T) {
	_, diags := New(util.NewNoop()).ParseDocument("---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\nSECTION \"B\"\n  TEXT\n    hola\n", "d.doclang")
	if len(diags) != 1 || diags[0].RuleID != "TEXT001" || !diags[0].IsWarning() || diags[0].Position.Line != 7 {
		t.Errorf("document: want one TEXT001 warning at line 7, got %v", diags)
	}
	_, diags = New(util.NewNoop()).Parse("---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n\nSLIDE content\n  title: \"U\"\n", "s.slidelang")
	if len(diags) != 1 || diags[0].RuleID != "TEXT001" || !diags[0].IsWarning() {
		t.Errorf("slide: want one TEXT001 warning, got %v", diags)
	}
	_, diags = New(util.NewNoop()).ParseDocument("---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n  POINTS\n    - uno\n", "d.doclang")
	if len(diags) != 1 || diags[0].RuleID != "TEXT002" {
		t.Errorf("absorbing TEXT: want only TEXT002, got %v", diags)
	}
}

// Un TEXT sin cuerpo seguido de una línea en blanco lee como su párrafo lo que
// sigue a su misma sangría, como en main: el TEXT y la QUOTE de después
// también. Avisa con TEXT002, no con TEXT001.
func TestStrictText_EmptyBodyThenBlankLineIsParagraph(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n  TEXT\n\n  TEXT\n    Segundo.\n\n  QUOTE\n    Cita.\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "d.doclang")
	if len(diags) != 1 || diags[0].RuleID != "TEXT002" || diags[0].Position.Line != 7 ||
		diags[0].EndPosition == nil || diags[0].EndPosition.Line != 13 {
		t.Errorf("want one TEXT002 from line 7 to 13, got %v", diags)
	}
	var texts []string
	for _, el := range doc.ContentBlocks[0].Elements {
		if e, ok := el.(*ast.TextElement); ok {
			texts = append(texts, e.Content)
		} else {
			t.Errorf("want only the paragraph, got a %T", el)
		}
	}
	if want := "TEXT Segundo. QUOTE Cita."; len(texts) != 1 || texts[0] != want {
		t.Errorf("want the paragraph %q as in main, got %q", want, texts)
	}
}

// Tras un TEXT vacío, una línea con forma de elemento seguida de prosa a su
// misma sangría, o de algo que el elemento no acepta, es el párrafo del TEXT
// con todas sus líneas, como en main. Avisa con TEXT002 solo si la primera
// línea abre un elemento en la forma que acepta. En documento, slide y
// columna tipada.
func TestStrictText_EmptyBodyIsAlwaysTheParagraph(t *testing.T) {
	for _, tc := range []struct {
		body   string
		warned bool
	}{
		{"  POINTS\n  Some prose after\n", true},
		{"  POINTS\n\n  Some prose after\n", true},
		{"\n  POINTS\n  Some prose after\n", true},
		{"  IMAGE foto.png\n  Some prose after\n", true},
		{"  IMAGE\n  prose\n", true},
		{"  TABLE\n  Some prose after\n", true},
		{"  IMAGE of the year\n", false},
		{"  :::\n", false},
		{"  CODE red alert now\n  more text\n", false},
		{"  @center\n  Some prose after\n", true},
		{"  @notes hi\n  Some prose after\n", true},
		{"  | a | b |\n  Some prose after\n", true},
		{"  CODE js\n  Some prose after\n", true},
		{"  :::note\n  hola\n  :::\n  Some prose after\n", true},
		{"  <<math>>\n  x^2\n  <<end>>\n  Some prose after\n", true},
		{"  CHECKLIST\n  Some prose after\n", true},
		{"  POINTS\n    - uno\n  Some prose after\n", true},
		{"  POINTS\n    - uno\n\n  Some prose after\n", true},
		{"  POINTS\n    no es un item\n", true},
		{"  QUOTE\n    Cita.\n  nota: algo\n", true},
		{"  :::x hola\n  sigue\n", true},
		{"  ::: grid\n  ::: column\n  a\n  :::\n  :::\n", false},
		// Los casos de las revisiones de la lectura a prueba, que ya no
		// existe: siguen siendo el párrafo de main.
		{"  POINTS\n    - apple pie\n    pie apple\n", true},
		{"  POINTS\n    - Costo\n    5 € + 3 %\n", true},
		{"  POINTS\n    - Plan A\n    o B\n", true},
		{"  POINTS\n    - apple pie\n    apple\n  QUOTE\n    pie\n", true},
		{"  IMAGE foto.png\n    alt: perro\n    caption: perro\n", true},
		{"  POINTS\n    - uno\n    Importante:\n", true},
		{"  POINTS\n    - uno\n    Nota: -> **\n", true},
		{"  IMAGE foto.png\n    Leyenda:\n", true},
		{"  POINTS\n    - uno\n    ---\n", true},
		{"  POINTS\n    - uno\n    \"\"\n", true},
		{"  POINTS\n    - hola\n    -> <> [] {} ## **\n", true},
		{"  IMAGE foto.png\n    caption: perro\n    caption: perro\n", true},
		{"  POINTS\n    - uno -> dos\n    ->\n", true},
		{"  POINTS\n    - **uno**\n    **\n", true},
		{"  POINTS\n    - a -- b -- c\n    --\n    --\n", true},
		{"  POINTS\n    - uno\n    listType: unordered\n", true},
	} {
		var want []string
		for _, l := range strings.Split(tc.body, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				want = append(want, l)
			}
		}
		for name, c := range sameIndentCases(tc.body) {
			t.Run(name+"/"+tc.body, func(t *testing.T) {
				got, diags := elementShapes(t, c.src, c.document)
				if w := "text:" + strings.Join(want, " "); strings.Join(got, "|") != w {
					t.Errorf("got %v, want [%s] as in main", got, w)
				}
				warned := len(diags) == 1 && diags[0].RuleID == "TEXT002"
				if warned != tc.warned || (!tc.warned && len(diags) != 0) {
					t.Errorf("want TEXT002 %v, got %v", tc.warned, diags)
				}
			})
		}
	}
}

// Un <!-- node-id --> justo antes de un TEXT vacío se enlaza al párrafo del
// TEXT, como en main: el TEXT se trata igual que uno sin id, así que también
// avisa, y el build no falla.
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
			warned := false
			for _, d := range diags {
				if d.IsError() {
					t.Errorf("unexpected error: %v", d)
				}
				warned = warned || d.RuleID == "TEXT002"
			}
			if !warned {
				t.Errorf("want TEXT002, got %v", diags)
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

// El aviso no agrega costo por TEXT: miles de TEXT vacíos seguidos, o de
// secciones con un TEXT que absorbe un elemento cada una, se leen en tiempo
// lineal.
func TestStrictText_AbsorbedElementWarningScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	for name, gen := range map[string]func(n int) string{
		"empty TEXT lines": func(n int) string {
			return noDropDocHeader + "SECTION \"S\"\n" + strings.Repeat("  TEXT\n", n)
		},
		"TEXT then POINTS": func(n int) string {
			return noDropDocHeader + "SECTION \"S\"\n" + strings.Repeat("  TEXT\n  POINTS\n    - a\n", n)
		},
		"one warning per section": func(n int) string {
			return noDropDocHeader + strings.Repeat("SECTION \"S\"\n  TEXT\n  POINTS\n    - a\n", n)
		},
		"empty TEXT per section": func(n int) string {
			return noDropDocHeader + strings.Repeat("SECTION \"S\"\n  TEXT\n", n)
		},
	} {
		t.Run(name, func(t *testing.T) { assertLinear(t, gen, false) })
	}
}
