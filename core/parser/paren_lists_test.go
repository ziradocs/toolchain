// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/elements"
	"go.ziradocs.com/core/v2/util"
)

// parseWithin corre fn con un plazo: un parser que no avanza no cuelga la
// suite, falla la prueba.
func parseWithin(t *testing.T, fn func() (*ast.AST, []diagnostics.Diagnostic)) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	type out struct {
		doc   *ast.AST
		diags []diagnostics.Diagnostic
	}
	done := make(chan out, 1)
	go func() {
		doc, diags := fn()
		done <- out{doc, diags}
	}()
	select {
	case r := <-done:
		return r.doc, r.diags
	case <-time.After(5 * time.Second):
		t.Fatal("parse did not finish in 5s: a parser is not advancing")
		return nil, nil
	}
}

func allPoints(doc *ast.AST) []*ast.PointsElement {
	var pts []*ast.PointsElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if p, ok := n.(*ast.PointsElement); ok {
			pts = append(pts, p)
		}
		return nil
	})
	return pts
}

// parenParsers arma el mismo cuerpo flex como slidelang y como doclang, con y
// sin nested-list-types-v1.
func parenParsers(body string) map[string]func() (*ast.AST, []diagnostics.Diagnostic) {
	typed := "---\nmode: flex\nast_capabilities: [\"nested-list-types-v1\"]\n---\n# T\n\n" + body
	plain := "---\nmode: flex\n---\n# T\n\n" + body
	return map[string]func() (*ast.AST, []diagnostics.Diagnostic){
		"slidelang typed": func() (*ast.AST, []diagnostics.Diagnostic) { return New(util.NewNoop()).Parse(typed, "") },
		"slidelang plain": func() (*ast.AST, []diagnostics.Diagnostic) { return New(util.NewNoop()).Parse(plain, "") },
		"doclang typed": func() (*ast.AST, []diagnostics.Diagnostic) {
			return New(util.NewNoop()).ParseDocument(typed, "")
		},
		"doclang plain": func() (*ast.AST, []diagnostics.Diagnostic) {
			return New(util.NewNoop()).ParseDocument(plain, "")
		},
	}
}

// "1)" es un marcador numerado con y sin nested-list-types-v1. Con la
// capacidad declarada el parser tipado rechazaba la primera línea sin
// consumirla y el bucle flex no avanzaba (cuelgue de v2.49.0).
func TestParenOrderedListParsesWithAndWithoutTypedLists(t *testing.T) {
	for name, fn := range parenParsers("1) uno\n2) dos\n") {
		t.Run(name, func(t *testing.T) {
			doc, diags := parseWithin(t, fn)
			if hasErrors(diags) {
				t.Fatalf("diagnostics %+v", diags)
			}
			pts := allPoints(doc)
			if len(pts) != 1 || pts[0].ListType != "ordered" || len(pts[0].Items) != 2 {
				t.Fatalf("want one ordered list with two items, got %+v", pts)
			}
			for i, want := range []string{"uno", "dos"} {
				it := pts[0].Items[i]
				if it.Content != want || it.Delimiter != ")" {
					t.Fatalf("item %d = %q delimiter %q", i, it.Content, it.Delimiter)
				}
			}
		})
	}
}

// Cambiar de "1." a "1)" entre hermanos abre otra lista, como en CommonMark,
// también con la capacidad.
func TestMixedDelimitersSplitTheList(t *testing.T) {
	for name, fn := range parenParsers("1. a\n2. b\n1) c\n2) d\n") {
		t.Run(name, func(t *testing.T) {
			doc, diags := parseWithin(t, fn)
			if hasErrors(diags) {
				t.Fatalf("diagnostics %+v", diags)
			}
			pts := allPoints(doc)
			if len(pts) != 2 {
				t.Fatalf("want two lists, got %d: %+v", len(pts), pts)
			}
			for i, d := range []string{".", ")"} {
				if pts[i].ListType != "ordered" || len(pts[i].Items) != 2 || pts[i].Items[0].Delimiter != d {
					t.Fatalf("list %d = %+v", i, pts[i])
				}
			}
		})
	}
}

// Sublistas con paréntesis: "1)" anidado es una sublista ordenada; "a)" no es
// un marcador del dialecto (sí lo es "a.") y no debe colgar ni perder el item.
func TestNestedParenMarkersTerminate(t *testing.T) {
	t.Run("numbered", func(t *testing.T) {
		for name, fn := range parenParsers("- x\n  1) y\n  2) z\n") {
			t.Run(name, func(t *testing.T) {
				doc, diags := parseWithin(t, fn)
				if hasErrors(diags) {
					t.Fatalf("diagnostics %+v", diags)
				}
				pts := allPoints(doc)
				if len(pts) != 1 || len(pts[0].Items) != 1 || len(pts[0].Items[0].SubPoints) != 2 {
					t.Fatalf("want x with two sub-points, got %+v", pts)
				}
				x := pts[0].Items[0]
				if strings.HasSuffix(name, "typed") && x.SubListType != "ordered" {
					t.Fatalf("typed sublist type %q", x.SubListType)
				}
				if !strings.HasSuffix(name, "typed") && x.SubListMarker != "ordered" {
					t.Fatalf("legacy sublist marker %q", x.SubListMarker)
				}
			})
		}
	})
	t.Run("letters", func(t *testing.T) {
		for name, fn := range parenParsers("- x\n  a) y\n  b) z\n") {
			t.Run(name, func(t *testing.T) {
				doc, diags := parseWithin(t, fn)
				if hasErrors(diags) {
					t.Fatalf("diagnostics %+v", diags)
				}
				pts := allPoints(doc)
				if len(pts) != 1 || len(pts[0].Items) != 1 || !strings.HasPrefix(pts[0].Items[0].Content, "x") {
					t.Fatalf("list lost: %+v", pts)
				}
			})
		}
	})
}

// En strict, un POINTS con "1)" y la capacidad declarada se lee como lista
// numerada en vez de marcar cada item como marcador inválido.
func TestStrictParenPointsWithTypedLists(t *testing.T) {
	src := "---\nmode: strict\nast_capabilities: [\"nested-list-types-v1\"]\n---\nSECTION \"S\"\n  level: 1\n  POINTS\n    1) uno\n    2) dos\n"
	doc, diags := parseWithin(t, func() (*ast.AST, []diagnostics.Diagnostic) {
		return New(util.NewNoop()).ParseDocument(src, "")
	})
	if hasErrors(diags) {
		t.Fatalf("diagnostics %+v", diags)
	}
	pts := allPoints(doc)
	if len(pts) != 1 || pts[0].ListType != "ordered" || len(pts[0].Items) != 2 {
		t.Fatalf("want one ordered list with two items, got %+v", pts)
	}
}

// Con list-start-v1 "1)" también es numerado y conserva el número de inicio.
func TestParenOrderedListWithListStart(t *testing.T) {
	src := "---\nmode: flex\nast_capabilities: [\"list-start-v1\"]\n---\n# T\n\n3) a\n4) b\n"
	for name, fn := range map[string]func() (*ast.AST, []diagnostics.Diagnostic){
		"slidelang": func() (*ast.AST, []diagnostics.Diagnostic) { return New(util.NewNoop()).Parse(src, "") },
		"doclang":   func() (*ast.AST, []diagnostics.Diagnostic) { return New(util.NewNoop()).ParseDocument(src, "") },
	} {
		t.Run(name, func(t *testing.T) {
			doc, diags := parseWithin(t, fn)
			if hasErrors(diags) {
				t.Fatalf("diagnostics %+v", diags)
			}
			pts := allPoints(doc)
			if len(pts) != 1 || len(pts[0].Items) != 2 || pts[0].Start == nil || *pts[0].Start != 3 {
				t.Fatalf("want one list starting at 3, got %+v", pts)
			}
		})
	}
}

// stuckParser reclama toda línea que empiece con "!!" y devuelve un elemento
// sin consumir nada: el bug que colgaba la CLI, aislado.
type stuckParser struct{}

func (stuckParser) CanParse(line, _ string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "!!")
}
func (stuckParser) Parse(ctx *elements.ParseContext, i int) *elements.ParseResult {
	return &elements.ParseResult{Element: ast.NewTextElement(ctx.Position(i), "stuck"), ConsumedLines: 0}
}

func stuckRegistry() *elements.Registry {
	r := elements.NewRegistry()
	r.Register(stuckParser{})
	r.Register(&elements.TextParser{})
	return r
}

// Si un parser de elemento devuelve un elemento sin consumir líneas, los dos
// bucles flex reportan un error y siguen con la línea de abajo.
func TestFlexLoopsSkipAParserThatConsumesNothing(t *testing.T) {
	body := "# T\n\n!! atorado\n\nDespués\n"
	run := map[string]func() (*ast.AST, []diagnostics.Diagnostic){
		"slidelang": func() (*ast.AST, []diagnostics.Diagnostic) {
			p := newFlexBodyParser(body, 0, util.NewNoop())
			p.registry = stuckRegistry()
			return p.Parse()
		},
		"doclang": func() (*ast.AST, []diagnostics.Diagnostic) {
			p := NewDocumentFlexParser(body, util.NewNoop())
			p.registry = stuckRegistry()
			return p.Parse()
		},
	}
	for name, fn := range run {
		t.Run(name, func(t *testing.T) {
			doc, diags := parseWithin(t, fn)
			found := false
			for _, d := range diags {
				if d.IsError() && strings.Contains(d.Message, "consumed no lines") {
					found = true
				}
			}
			if !found {
				t.Fatalf("no zero-consumption error in %+v", diags)
			}
			var texts []string
			_ = ast.Walk(doc, func(n ast.Node) error {
				if te, ok := n.(*ast.TextElement); ok {
					texts = append(texts, te.Content)
				}
				return nil
			})
			if strings.Join(texts, "|") != "Después" {
				t.Fatalf("texts after the stuck line %q", texts)
			}
		})
	}
}
