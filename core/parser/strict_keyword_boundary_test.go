// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// Issue #489: en el cuerpo de un QUOTE o un CHECKLIST, una línea que empieza
// con las letras de una palabra clave pegadas a otra cosa ("TEXTO citado",
// "TEXT-heavy", "MAPA norte") es texto del cuerpo, no un elemento nuevo que lo
// corta. En documentos y en slides.
func TestStrictKeywordBoundary_BodyLinesAreNotNewElements(t *testing.T) {
	for _, line := range []string{"TEXTO citado", "TEXTOS", "TEXTO: citado", "TEXT-heavy slides", "MAPA norte", "CODEX rojo"} {
		for _, dialect := range []string{"document", "slides"} {
			t.Run(dialect+"/"+line, func(t *testing.T) {
				doc := parseBoundary(t, dialect, "  QUOTE\n    "+line+"\n    AUTHOR: Ana\n")
				els := boundaryElements(doc)
				if len(els) != 1 {
					t.Fatalf("want one quote, got %d elements: %+v", len(els), els)
				}
				q, ok := els[0].(*ast.QuoteElement)
				if !ok || q.Content != line || q.Author != "Ana" {
					t.Errorf("want quote %q by Ana, got %+v", line, els[0])
				}
			})
		}
	}

	doc := parseBoundary(t, "document", "  CHECKLIST\n    - [ ] uno\n    TEXTO suelto\n")
	els := boundaryElements(doc)
	if len(els) != 1 {
		t.Fatalf("checklist: want one element, got %+v", els)
	}
	if c, ok := els[0].(*ast.ChecklistElement); !ok || len(c.Items) != 1 || c.Items[0].Content != "uno TEXTO suelto" {
		t.Errorf("checklist: want the line as a continuation of the item, got %+v", els[0])
	}
}

// Una palabra clave seguida de espacio, tab, `:` o `{`, o sola en la línea,
// sigue cortando el cuerpo como siempre.
func TestStrictKeywordBoundary_RealKeywordStillEndsTheBody(t *testing.T) {
	for _, line := range []string{"TEXT", "TEXT hola", "TEXT\thola", "TEXT: hola", "POINTS", "CODE{verbatim} go"} {
		t.Run(line, func(t *testing.T) {
			doc := parseBoundary(t, "document", "  QUOTE\n    cita\n    "+line+"\n")
			els := boundaryElements(doc)
			if len(els) == 0 {
				t.Fatal("want the quote, got no elements")
			}
			if q, ok := els[0].(*ast.QuoteElement); !ok || q.Content != "cita" {
				t.Errorf("want quote \"cita\", got %+v", els[0])
			}
		})
	}
}

// En posición de elemento el despacho sigue como en main: "TEXTO" con cuerpo
// es un TEXT. El arreglo solo cambia dónde termina un cuerpo.
func TestStrictKeywordBoundary_ElementPositionDispatchUnchanged(t *testing.T) {
	for _, dialect := range []string{"document", "slides"} {
		doc := parseBoundary(t, dialect, "  TEXTO\n    contenido\n")
		els := boundaryElements(doc)
		if len(els) != 1 {
			t.Fatalf("%s: want one element, got %+v", dialect, els)
		}
		if txt, ok := els[0].(*ast.TextElement); !ok || txt.Content != "contenido" {
			t.Errorf("%s: want TEXT \"contenido\", got %+v", dialect, els[0])
		}
	}
}

func parseBoundary(t *testing.T, dialect, body string) *ast.AST {
	t.Helper()
	p := New(util.NewNoop())
	if dialect == "document" {
		doc, _ := p.ParseDocument("---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n"+body, "d.doclang")
		return doc
	}
	doc, _ := p.Parse("---\nmode: strict\ntitle: \"D\"\n---\n\nSLIDE content\n  title: \"T\"\n"+body, "d.slidelang")
	return doc
}

func boundaryElements(doc *ast.AST) []ast.Element {
	if doc == nil || len(doc.ContentBlocks) == 0 {
		return nil
	}
	return doc.ContentBlocks[0].Elements
}

// A la sangría de la cabecera del QUOTE o del CHECKLIST la línea está en
// posición de elemento: una palabra pegada corta el cuerpo y se despacha como
// en main ("TEXTO" abre un TEXT, "CODEX" un CODE). El límite de palabra es
// solo para las líneas más sangradas que la cabecera.
func TestStrictKeywordBoundary_SameIndentAsHeaderKeepsMainCut(t *testing.T) {
	for _, dialect := range []string{"document", "slides"} {
		els := boundaryElements(parseBoundary(t, dialect, "  QUOTE\n    Hola\n  TEXTO\n    cuerpo\n"))
		if len(els) != 2 {
			t.Fatalf("%s QUOTE+TEXTO: want quote and text, got %d elements", dialect, len(els))
		}
		if q, ok := els[0].(*ast.QuoteElement); !ok || q.Content != "Hola" {
			t.Errorf("%s: want quote \"Hola\", got %+v", dialect, els[0])
		}
		if txt, ok := els[1].(*ast.TextElement); !ok || txt.Content != "cuerpo" {
			t.Errorf("%s: want TEXT \"cuerpo\", got %+v", dialect, els[1])
		}

		els = boundaryElements(parseBoundary(t, dialect, "  QUOTE\n    Hola\n  CODEX python\n    print(1)\n"))
		if len(els) != 2 {
			t.Fatalf("%s QUOTE+CODEX: want quote and code, got %d elements", dialect, len(els))
		}
		if q, ok := els[0].(*ast.QuoteElement); !ok || q.Content != "Hola" {
			t.Errorf("%s: want quote \"Hola\", got %+v", dialect, els[0])
		}
		if _, ok := els[1].(*ast.CodeElement); !ok {
			t.Errorf("%s: want a CODE element after the quote, got %T", dialect, els[1])
		}

		els = boundaryElements(parseBoundary(t, dialect, "  CHECKLIST\n  TEXTO\n    cuerpo\n"))
		var text *ast.TextElement
		for _, el := range els {
			if txt, ok := el.(*ast.TextElement); ok {
				text = txt
			}
		}
		if text == nil || text.Content != "cuerpo" {
			t.Errorf("%s CHECKLIST+TEXTO: want TEXT \"cuerpo\", got %+v", dialect, els)
		}
	}
}

// Una línea del cuerpo de un CHECKLIST antes de su primer ítem se descarta,
// como en main, pero con el aviso CHECKLIST001. Vale para cualquier línea.
func TestStrictChecklist_LineBeforeFirstItemIsReported(t *testing.T) {
	for _, line := range []string{"TEXTO suelto", "texto suelto"} {
		for _, dialect := range []string{"document", "slides"} {
			p := New(util.NewNoop())
			var doc *ast.AST
			var diags []diagnostics.Diagnostic
			body := "  CHECKLIST\n    " + line + "\n    - [ ] uno\n"
			if dialect == "document" {
				doc, diags = p.ParseDocument("---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n"+body, "d.doclang")
			} else {
				doc, diags = p.Parse("---\nmode: strict\ntitle: \"D\"\n---\n\nSLIDE content\n  title: \"T\"\n"+body, "d.slidelang")
			}
			els := boundaryElements(doc)
			if len(els) != 1 {
				t.Fatalf("%s %q: want one checklist, got %+v", dialect, line, els)
			}
			if c, ok := els[0].(*ast.ChecklistElement); !ok || len(c.Items) != 1 || c.Items[0].Content != "uno" {
				t.Errorf("%s %q: want the item \"uno\", got %+v", dialect, line, els[0])
			}
			found := false
			for _, d := range diags {
				if d.RuleID == "CHECKLIST001" && d.IsWarning() {
					found = true
				}
			}
			if !found {
				t.Errorf("%s %q: want a CHECKLIST001 warning, got %v", dialect, line, diags)
			}
		}
	}
}
