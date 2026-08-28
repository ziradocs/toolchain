// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// ChartParser y MapParser horneaban su default de render (800x600) dentro del
// AST, así que "el autor pidió 800" y "el autor no dijo nada" quedaban
// indistinguibles. Consecuencias, las dos visibles:
//
//   - formatChart emitía `width="800" height="600"` en la apertura de todo
//     chart, incluidos los que nunca las declararon.
//   - formatMap las omitía comparándolas contra una constante espejo del
//     default, así que un mapa que SÍ declaraba `width="800"` lo perdía en el
//     round-trip.
//
// Ahora 0 es "sin declarar" y quien renderiza aplica su propio default
// (renderer.ChartDimensions, el bloque de mapas de renderer/html.go). El
// harness de round-trip no puede ver ninguna de las dos cosas: compara
// parse→format→reparse, y tanto 800 como 0 round-trippean consigo mismos.

func chartDoc(el ast.Element) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Mode = "strict"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "title")
	block.Heading = "S"
	block.Elements = append(block.Elements, el)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

func TestFormat_OmitsUndeclaredChartAndMapDimensions(t *testing.T) {
	newChart := func(w, h int) ast.Element {
		c := ast.NewChartElement(diagnostics.NewPosition(3, 1), "bar")
		c.Data = [][]interface{}{{1, 2}}
		c.Width, c.Height = w, h
		return c
	}
	newMap := func(w, h int) ast.Element {
		m := ast.NewMapElement(diagnostics.NewPosition(3, 1), "osm")
		m.Width, m.Height = w, h
		return m
	}

	cases := []struct {
		name       string
		el         ast.Element
		wantSubstr []string
		wantAbsent []string
	}{
		{
			name:       "chart sin dimensiones declaradas",
			el:         newChart(0, 0),
			wantSubstr: []string{"<<chart: bar>>"},
			wantAbsent: []string{"width=", "height="},
		},
		{
			name:       "chart con solo width",
			el:         newChart(1200, 0),
			wantSubstr: []string{`<<chart: bar width="1200">>`},
			wantAbsent: []string{"height="},
		},
		{
			// El valor que coincide con el default del renderer NO es un caso
			// especial: si está en el AST es porque el autor lo escribió.
			// Tratarlo como "es el default, no lo emitas" era justo el falso
			// positivo de la constante espejo que formatMap usaba.
			name:       "dimensiones declaradas que coinciden con el default del renderer",
			el:         newChart(800, 600),
			wantSubstr: []string{`<<chart: bar width="800" height="600">>`},
		},
		{
			name:       "mapa sin dimensiones declaradas",
			el:         newMap(0, 0),
			wantSubstr: []string{"<<map>>"},
			wantAbsent: []string{"width=", "height="},
		},
		{
			name:       "mapa con dimensiones declaradas iguales al default",
			el:         newMap(800, 600),
			wantSubstr: []string{`<<map width="800" height="600">>`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := FormatStrict(chartDoc(tc.el))
			if err != nil {
				t.Fatalf("FormatStrict: %v", err)
			}
			for _, want := range tc.wantSubstr {
				if !strings.Contains(out, want) {
					t.Errorf("falta %q en la salida:\n%s", want, out)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(out, absent) {
					t.Errorf("la salida trae %q, que el autor nunca declaró — el default del renderer no debe "+
						"materializarse en el texto:\n%s", absent, out)
				}
			}
		})
	}
}
