// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
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
	for _, line := range []string{"TEXTO citado", "TEXTOS", "MAPA norte", "CODEX", "TABLEAU de bord", "POINTSET", "QUOTES", "CHECKLISTS", "IMAGEN", "MATHS", "CHARTER", "MERMAIDS", "PLANTUMLS", "TEXT_X", "TEXT9", "TEXTÓ"} {
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
	for _, header := range []string{"TEXT", "TEXT ", "TEXT\t", "TEXT ", "TEXT{x}", "TEXT:"} {
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
