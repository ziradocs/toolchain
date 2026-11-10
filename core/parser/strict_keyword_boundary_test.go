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

// Issue #489: una palabra clave de elemento strict solo cuenta como palabra
// completa. Una línea del cuerpo de una cita que empieza con las letras de
// una palabra clave ("TEXTO citado", "MAPA norte") es texto de la cita, no el
// inicio de otro elemento.
func TestStrictKeywordBoundary_QuoteBodyKeepsWordsThatStartWithAKeyword(t *testing.T) {
	for _, line := range []string{"TEXT-heavy slides aburren", "CODE.", "MAP/territorio", "TEXT(o)", "TEXTO citado", "TEXTOS", "MAPA norte", "CODEX", "TABLEAU de bord", "POINTSET", "QUOTES", "CHECKLISTS", "IMAGEN", "MATHS", "CHARTER", "MERMAIDS", "PLANTUMLS", "TEXT_X", "TEXT9", "TEXTÓ"} {
		for _, document := range []bool{true, false} {
			name := line + map[bool]string{true: "/document", false: "/slide"}[document]
			t.Run(name, func(t *testing.T) {
				head := "SLIDE content\n  title: \"T\""
				if document {
					head = "SECTION \"A\""
				}
				src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\n" + head + "\n  QUOTE\n    " + line + "\n    AUTHOR: Ana\n"
				doc, diags := parseStrictSource(t, src, document)
				if len(diags) != 0 {
					t.Fatalf("want no diagnostics, got %v", diags)
				}
				els := doc.ContentBlocks[0].Elements
				if len(els) != 1 {
					t.Fatalf("want only the quote, got %d elements: %#v", len(els), els)
				}
				q, ok := els[0].(*ast.QuoteElement)
				if !ok {
					t.Fatalf("want a quote, got %T", els[0])
				}
				if q.Content != line || q.Author != "Ana" {
					t.Errorf("quote content %q author %q, want %q and %q", q.Content, q.Author, line, "Ana")
				}
			})
		}
	}
}

// A nivel de elemento, una línea que empieza con las letras de una palabra
// clave no abre ese elemento: en un documento es el error de contenido
// inesperado (nada se descarta en silencio), en un slide el warning STRICT003
// de siempre para una línea no reconocida.
func TestStrictKeywordBoundary_ElementLevelWordIsNotTheKeyword(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  TEXTO citado\n"
	doc, diags := parseStrictSource(t, src, true)
	if len(doc.ContentBlocks[0].Elements) != 0 {
		t.Errorf("TEXTO must not become an element, got %#v", doc.ContentBlocks[0].Elements)
	}
	if len(diags) != 1 || !diags[0].IsError() || !strings.Contains(diags[0].Message, `"TEXTO citado" is not an element`) || diags[0].Position.Line != 7 {
		t.Errorf("want one error at line 7 naming the line, got %v", diags)
	}

	src = "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  TEXTO citado\n"
	doc, diags = parseStrictSource(t, src, false)
	if len(doc.ContentBlocks[0].Elements) != 0 {
		t.Errorf("TEXTO must not become an element, got %#v", doc.ContentBlocks[0].Elements)
	}
	if len(diags) != 1 || diags[0].IsError() || diags[0].RuleID != "STRICT003" {
		t.Errorf("want the STRICT003 warning, got %v", diags)
	}
}

// Lo que la palabra clave aceptaba después y no forma otra palabra sigue
// valiendo: espacio, tab, un espacio Unicode, un atributo `{…}`, `:`.
func TestStrictKeywordBoundary_SeparatorsStillOpenTheElement(t *testing.T) {
	for _, header := range []string{"TEXT", "TEXT ", "TEXT\t", "TEXT ", "TEXT\u3000", "TEXT:"} {
		t.Run(header, func(t *testing.T) {
			src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  " + header + "\n    hola\n"
			doc, diags := parseStrictSource(t, src, true)
			if len(diags) != 0 {
				t.Fatalf("want no diagnostics, got %v", diags)
			}
			els := doc.ContentBlocks[0].Elements
			if len(els) != 1 {
				t.Fatalf("want one element, got %#v", els)
			}
			if text, ok := els[0].(*ast.TextElement); !ok || text.Content != "hola" {
				t.Errorf("want TEXT with %q, got %#v", "hola", els[0])
			}
		})
	}
}

func parseStrictSource(t *testing.T, src string, document bool) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	p := New(util.NewNoop())
	if document {
		return p.ParseDocument(src, "q.doclang")
	}
	return p.Parse(src, "q.slidelang")
}

// Una línea a nivel de elemento que empieza con una palabra clave pegada a
// otra cosa da el diagnóstico de línea no reconocida de cada dialecto (error
// en documento, STRICT003 en slide) y sugiere la palabra clave.
func TestStrictKeywordBoundary_DidYouMean(t *testing.T) {
	for _, tc := range []struct{ line, word, kw string }{
		{"TEXTO citado", "TEXTO", "TEXT"},
		{"TEXT-heavy", "TEXT-heavy", "TEXT"},
		{"MAPA norte", "MAPA", "MAP"},
	} {
		want := fmt.Sprintf("%q is not an element keyword; did you mean %s?", tc.word, tc.kw)
		doc := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  " + tc.line + "\n    contenido\n"
		_, diags := parseStrictSource(t, doc, true)
		if len(diags) == 0 || !diags[0].IsError() || !strings.Contains(diags[0].Message, want) {
			t.Errorf("document %q: want an error with %q, got %v", tc.line, want, diags)
		}
		slide := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  " + tc.line + "\n    contenido\n"
		_, diags = parseStrictSource(t, slide, false)
		if len(diags) == 0 || diags[0].IsError() || diags[0].RuleID != "STRICT003" || !strings.Contains(diags[0].Message, want) {
			t.Errorf("slide %q: want STRICT003 with %q, got %v", tc.line, want, diags)
		}
	}
}

// Lo que se escribe en la misma línea que TEXT, POINTS, CHECKLIST, QUOTE o
// TABLE se perdía sin aviso. Ahora es el diagnóstico de contenido descartado
// de cada dialecto, y el elemento se sigue leyendo.
func TestStrictKeyword_TextOnTheKeywordLine(t *testing.T) {
	for _, tc := range []struct{ header, body, dropped string }{
		{"TEXT hola mundo", "    cuerpo\n", "hola mundo"},
		{"TEXT: hola", "    cuerpo\n", "hola"},
		{"TEXT{x}", "    cuerpo\n", "{x}"},
		{"POINTS lista", "    - uno\n", "lista"},
		{"CHECKLIST tareas", "    - [ ] a\n", "tareas"},
		{"QUOTE cita", "    texto\n", "cita"},
		{"TABLE 3.1: Resultados", "    headers: [\"a\"]\n    rows: [[\"1\"]]\n", "3.1: Resultados"},
	} {
		want := fmt.Sprintf("%q was discarded", tc.dropped)
		doc := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  " + tc.header + "\n" + tc.body
		d, diags := parseStrictSource(t, doc, true)
		if len(diags) != 1 || !diags[0].IsError() || !strings.Contains(diags[0].Message, want) || diags[0].Position.Line != 7 {
			t.Errorf("document %q: want one error at line 7 with %q, got %v", tc.header, want, diags)
		}
		if len(d.ContentBlocks[0].Elements) != 1 {
			t.Errorf("document %q: the element must still be read, got %d elements", tc.header, len(d.ContentBlocks[0].Elements))
		}
		slide := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  " + tc.header + "\n" + tc.body
		_, diags = parseStrictSource(t, slide, false)
		if len(diags) != 1 || diags[0].IsError() || diags[0].RuleID != "STRICT003" || !strings.Contains(diags[0].Message, want) {
			t.Errorf("slide %q: want STRICT003 with %q, got %v", tc.header, want, diags)
		}
	}
}

// Dentro de una columna tipada de un slide, el texto en la línea de TEXT es
// el warning STRICT003, como fuera de la columna: un deck que compilaba no se
// rompe. En un documento es error.
func TestStrictKeyword_TextOnTheKeywordLineInsideATypedColumn(t *testing.T) {
	column := "  <<grid>>\n  <<column typed>>\n    TEXT hola\n      cuerpo\n  <<end>>\n"
	_, diags := parseStrictSource(t, "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n"+column, false)
	if len(diags) != 1 || diags[0].IsError() || diags[0].RuleID != "STRICT003" || !strings.Contains(diags[0].Message, `"hola" was discarded`) {
		t.Errorf("slide: want one STRICT003 warning, got %v", diags)
	}
	_, diags = parseStrictSource(t, "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n"+column, true)
	if len(diags) != 1 || !diags[0].IsError() || !strings.Contains(diags[0].Message, `"hola" was discarded`) {
		t.Errorf("document: want one error, got %v", diags)
	}
}

// Una palabra clave pegada seguida de ':' no es una propiedad desconocida:
// es la línea no reconocida de cada dialecto, con la sugerencia.
func TestStrictKeywordBoundary_GluedKeywordWithColon(t *testing.T) {
	for _, tc := range []struct{ line, word string }{
		{"TEXTO: algo", "TEXTO:"},
		{"TEXT-heavy: no", "TEXT-heavy:"},
	} {
		want := fmt.Sprintf("%q is not an element keyword; did you mean TEXT?", tc.word)
		_, diags := parseStrictSource(t, "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  "+tc.line+"\n", true)
		if len(diags) != 1 || !diags[0].IsError() || !strings.Contains(diags[0].Message, want) || strings.Contains(diags[0].Message, "property:") {
			t.Errorf("document %q: want one error with %q, got %v", tc.line, want, diags)
		}
		_, diags = parseStrictSource(t, "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  "+tc.line+"\n", false)
		if len(diags) != 1 || diags[0].IsError() || diags[0].RuleID != "STRICT003" || !strings.Contains(diags[0].Message, want) {
			t.Errorf("slide %q: want STRICT003 with %q, got %v", tc.line, want, diags)
		}
	}
}

// Sin sugerencia, los mensajes de línea no reconocida quedan exactamente como
// eran (sin un punto de más al final).
func TestStrictKeywordBoundary_MessagesWithoutHintUnchanged(t *testing.T) {
	_, diags := parseStrictSource(t, "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  foo bar\n", true)
	if len(diags) != 1 || diags[0].Message != `unexpected content inside SECTION: "foo bar" is not an element, a property, or a new SECTION` {
		t.Errorf("document message changed: %v", diags)
	}
	column := "  <<grid>>\n  <<column typed>>\n    foo\n  <<end>>\n"
	_, diags = parseStrictSource(t, "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n"+column, true)
	if len(diags) != 1 || diags[0].Message != `unexpected content inside a typed column: "foo" is not an element` {
		t.Errorf("typed column message changed: %v", diags)
	}
}
