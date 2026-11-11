// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// parseFlexBoth parsea body como slide flex (Parse) y como documento flex
// (ParseDocument), con el normalizador apagado, y devuelve los elementos del
// primer bloque de cada uno.
func parseFlexBoth(t *testing.T, body string) map[string][]ast.Element {
	t.Helper()
	out := map[string][]ast.Element{}
	slides := New(util.NewNoop())
	slides.SetNormalization(false)
	src := "---\nmode: flex\n---\n# Bloque\n\n" + body
	docs := New(util.NewNoop())
	docs.SetNormalization(false)
	for name, parse := range map[string]func() (*ast.AST, []diagnostics.Diagnostic){
		"slidelang": func() (*ast.AST, []diagnostics.Diagnostic) { return slides.Parse(src, "grid.slidelang") },
		"doclang":   func() (*ast.AST, []diagnostics.Diagnostic) { return docs.ParseDocument(src, "grid.doclang") },
	} {
		doc, diags := parse()
		if doc == nil || len(doc.ContentBlocks) == 0 {
			t.Fatalf("%s: sin AST: %v", name, diags)
		}
		if countErrors(diags) != 0 {
			t.Fatalf("%s: errores de parseo: %v", name, diags)
		}
		out[name] = doc.ContentBlocks[0].Elements
	}
	return out
}

func singleGrid(t *testing.T, name string, elems []ast.Element) *ast.GridElement {
	t.Helper()
	if len(elems) != 1 {
		t.Fatalf("%s: %d elementos, se esperaba un grid: %#v", name, len(elems), elems)
	}
	grid, ok := elems[0].(*ast.GridElement)
	if !ok {
		t.Fatalf("%s: el elemento es %T, se esperaba *ast.GridElement", name, elems[0])
	}
	return grid
}

// `:::grid` y `:::column` sin el espacio son el mismo grid que con espacio, en
// las cuatro combinaciones y en los dos parsers flex. Antes `:::grid` daba un
// special_block de tipo "grid" (con sus columnas como special_block de tipo
// "column") y el aviso "Unknown special block type: grid".
func TestFlexGrid_NoSpaceSpellings(t *testing.T) {
	for _, grid := range []string{"::: grid", ":::grid"} {
		for _, column := range []string{"::: column", ":::column"} {
			body := fmt.Sprintf("%s\n%s\nIzquierda\n:::\n%s\nDerecha\n:::\n:::\n\nDespués.\n", grid, column, column)
			for name, elems := range parseFlexBoth(t, body) {
				t.Run(grid+"/"+column+"/"+name, func(t *testing.T) {
					if len(elems) != 2 {
						t.Fatalf("%d elementos, se esperaban el grid y el párrafo: %#v", len(elems), elems)
					}
					g := singleGrid(t, name, elems[:1])
					if len(g.Columns) != 2 || g.Columns[0].Content != "Izquierda" || g.Columns[1].Content != "Derecha" || g.Content != "" {
						t.Fatalf("grid = %+v", g)
					}
					if text, ok := elems[1].(*ast.TextElement); !ok || text.Content != "Después." {
						t.Fatalf("el párrafo que sigue al grid: %#v", elems[1])
					}
				})
			}
		}
	}
}

// `:::column typed` es una columna tipada, igual que `::: column typed`.
func TestFlexGrid_NoSpaceTypedColumn(t *testing.T) {
	body := ":::grid\n:::column typed\nTexto tipado.\n:::\n:::column\nCrudo.\n:::\n:::\n"
	for name, elems := range parseFlexBoth(t, body) {
		g := singleGrid(t, name, elems)
		if len(g.Columns) != 2 {
			t.Fatalf("%s: %d columnas", name, len(g.Columns))
		}
		typed := g.Columns[0]
		if typed.Content != "" || len(typed.Elements) != 1 {
			t.Fatalf("%s: columna tipada: content=%q elements=%d", name, typed.Content, len(typed.Elements))
		}
		if text, ok := typed.Elements[0].(*ast.TextElement); !ok || text.Content != "Texto tipado." {
			t.Fatalf("%s: elemento tipado: %#v", name, typed.Elements[0])
		}
		if g.Columns[1].Content != "Crudo." {
			t.Fatalf("%s: columna cruda: %+v", name, g.Columns[1])
		}
	}
}

// Sin el espacio, "grid" y "column" tienen que ser la palabra entera:
// `:::columns` (que usan los ejemplos), `:::gridlock` y `:::grid-note` siguen
// siendo bloques especiales con ese tipo.
func TestFlexGrid_NoSpaceNeedsWholeWord(t *testing.T) {
	for _, tc := range []struct{ open, blockType string }{
		{":::columns", "columns"},
		{`:::columns{ratio="1:2"}`, `columns{ratio="1:2"}`},
		{":::gridlock", "gridlock"},
		{":::grid-note", "grid-note"},
		{":::column-note", "column-note"},
	} {
		for name, elems := range parseFlexBoth(t, tc.open+"\nCuerpo.\n:::\n") {
			if len(elems) != 1 {
				t.Fatalf("%s %s: %d elementos", name, tc.open, len(elems))
			}
			sb, ok := elems[0].(*ast.SpecialBlockElement)
			if !ok || sb.BlockType != tc.blockType {
				t.Errorf("%s %s: %#v, se esperaba un special_block %q", name, tc.open, elems[0], tc.blockType)
			}
		}
	}
}

// Un `:::grid` dentro de un aviso es el mismo grid anidado que `::: grid`.
func TestFlexGrid_NoSpaceInsideCallout(t *testing.T) {
	const inner = "%s\n%s\nA\n:::\n%s\nB\n:::\n:::\n"
	for _, spelling := range [][2]string{{"::: grid", "::: column"}, {":::grid", ":::column"}} {
		body := ":::info\nAntes.\n" + fmt.Sprintf(inner, spelling[0], spelling[1], spelling[1]) + ":::\n"
		for name, elems := range parseFlexBoth(t, body) {
			if len(elems) != 1 {
				t.Fatalf("%s %s: %d elementos", name, spelling[0], len(elems))
			}
			sb, ok := elems[0].(*ast.SpecialBlockElement)
			if !ok || sb.BlockType != "info" {
				t.Fatalf("%s %s: %#v", name, spelling[0], elems[0])
			}
			var grid *ast.GridElement
			for _, nested := range sb.Elements {
				if g, ok := nested.(*ast.GridElement); ok {
					grid = g
				}
			}
			if grid == nil || len(grid.Columns) != 2 || grid.Columns[0].Content != "A" || grid.Columns[1].Content != "B" {
				t.Errorf("%s %s: grid anidado = %+v (elements %#v)", name, spelling[0], grid, sb.Elements)
			}
		}
	}
}

// Dentro de una columna tipada, un `:::grid` anidado es el mismo error que
// `::: grid`.
func TestFlexGrid_NoSpaceNestedInTypedColumnIsError(t *testing.T) {
	p := New(util.NewNoop())
	p.SetNormalization(false)
	src := "---\nmode: flex\n---\n# Bloque\n\n::: grid\n::: column typed\n:::grid\n:::\n:::\n"
	_, diags := p.Parse(src, "grid.slidelang")
	if !hasErrorContaining(diags, "cannot be nested inside a typed column") {
		t.Fatalf("se esperaba el error de grid anidado: %v", diags)
	}
}
