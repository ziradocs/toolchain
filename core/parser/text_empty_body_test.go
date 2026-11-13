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

// Tras un TEXT vacío, una palabra clave pegada a la sangría del TEXT
// ("TEXTO", "POINTSX", "QUOTEado") está en posición de elemento: se despacha
// igual que sin el TEXT delante, por prefijo, como en main (ver
// elements.EndsStrictBody). Con la línea pegada al TEXT y con una línea en
// blanco entre los dos.
func TestStrictText_EmptyBodyThenGluedKeywordDispatchesAsElement(t *testing.T) {
	head := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n"
	for _, word := range []string{"TEXTO pegado", "POINTSX pegado", "QUOTEado"} {
		bare := elementShapes(t, head+"  "+word+"\n")
		for _, sep := range []string{"\n", "\n\n"} {
			got := elementShapes(t, head+"  TEXT"+sep+"  "+word+"\n")
			if strings.Join(got, "|") != strings.Join(bare, "|") {
				t.Errorf("%q after an empty TEXT (sep %q): got %v, want %v as without the TEXT", word, sep, got, bare)
			}
		}
	}
}

// elementShapes devuelve tipo y contenido de cada elemento de la primera
// sección, sin posiciones ni node-ids.
func elementShapes(t *testing.T, src string) []string {
	t.Helper()
	doc, _ := New(util.NewNoop()).ParseDocument(src, "d.doclang")
	var out []string
	for _, el := range doc.ContentBlocks[0].Elements {
		switch e := el.(type) {
		case *ast.TextElement:
			out = append(out, "text:"+e.Content)
		case *ast.QuoteElement:
			out = append(out, "quote:"+e.Content)
		case *ast.PointsElement:
			out = append(out, fmt.Sprintf("points:%d", len(e.Items)))
		default:
			out = append(out, fmt.Sprintf("%T", el))
		}
	}
	return out
}

// Un TEXT sin contenido da TEXT001 como warning en los dos dialectos: no
// pierde nada, y en un documento strict un error rompería documentos que
// compilaban.
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
