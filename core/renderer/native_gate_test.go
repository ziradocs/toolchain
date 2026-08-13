// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func astWithOneElement(el ast.Element) *ast.AST {
	pos := diagnostics.Position{Line: 1, Column: 1}
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, el)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

func TestHasElementHelpers_OnlyMatchTheirOwnType(t *testing.T) {
	mermaid := astWithOneElement(ast.NewMermaidElement(diagnostics.Position{Line: 1}, "flowchart", "graph TD; A-->B"))
	chart := astWithOneElement(ast.NewChartElement(diagnostics.Position{Line: 1}, "bar"))
	mapDoc := astWithOneElement(&ast.MapElement{})
	math := astWithOneElement(ast.NewMathElement(diagnostics.Position{Line: 1}, "E = mc^2"))
	text := astWithOneElement(ast.NewTextElement(diagnostics.Position{Line: 1}, "just text"))

	cases := []struct {
		name string
		fn   func(*ast.AST) bool
		yes  *ast.AST
		no   []*ast.AST
	}{
		{"Mermaid", HasMermaidElements, mermaid, []*ast.AST{chart, mapDoc, math, text}},
		{"Chart", HasChartElements, chart, []*ast.AST{mermaid, mapDoc, math, text}},
		{"Map", HasMapElements, mapDoc, []*ast.AST{mermaid, chart, math, text}},
		{"Math", HasMathElements, math, []*ast.AST{mermaid, chart, mapDoc, text}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.fn(tc.yes) {
				t.Errorf("expected Has%sElements to be true for a document containing one", tc.name)
			}
			for _, doc := range tc.no {
				if tc.fn(doc) {
					t.Errorf("expected Has%sElements to be false for a document without one", tc.name)
				}
			}
		})
	}
}

func TestHasElementHelpers_NilASTIsFalse(t *testing.T) {
	if HasMermaidElements(nil) || HasChartElements(nil) || HasMapElements(nil) || HasMathElements(nil) {
		t.Error("expected all Has*Elements helpers to return false for a nil AST")
	}
}

func nativeBarChartElementForGateTest() *ast.ChartElement {
	c := ast.NewChartElement(diagnostics.Position{Line: 1}, "bar")
	c.Data = [][]interface{}{{"A", 10.0}, {"B", 20.0}}
	c.Labels = []string{"A", "B"}
	return c
}

// TestTryAllChartsNative_MermaidBailsOutRegardlessOfBackend confirma el
// comportamiento histórico para el backend default: un mermaid en el
// documento siempre fuerza Chromium cuando diagramBackend != "kroki",
// exactamente como antes de que este parámetro existiera.
func TestTryAllChartsNative_MermaidBailsOutRegardlessOfBackend(t *testing.T) {
	doc := ast.NewAST(diagnostics.Position{Line: 1})
	block := ast.NewContentBlock(diagnostics.Position{Line: 1}, "content")
	block.Elements = append(block.Elements,
		ast.NewMermaidElement(diagnostics.Position{Line: 1}, "flowchart", "graph TD; A-->B"),
		nativeBarChartElementForGateTest(),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	if _, ok := TryAllChartsNative(doc, "png", "chromium"); ok {
		t.Error("expected TryAllChartsNative to bail out on mermaid when diagramBackend is chromium")
	}
	if _, ok := TryAllChartsNative(doc, "png", ""); ok {
		t.Error("expected TryAllChartsNative to bail out on mermaid when diagramBackend is empty (default)")
	}
}

// TestTryAllChartsNative_KrokiMermaidDoesNotBlockNativeCharts cubre un
// hallazgo de code-review sobre PR #138: con mermaid vía Kroki y un chart
// nativo-capaz en el mismo documento, TryAllChartsNative ignoraba
// diagramBackend por completo y hacía bail-out en cuanto veía CUALQUIER
// MermaidElement — así que needsChromium (doclang/internal/generator/
// html.go) seguía exigiendo Chromium aunque ningún elemento del documento
// lo necesitara de verdad (mermaid ya resuelto por HTTP puro vía
// KrokiFetcher, chart nativo-capaz). Con diagramBackend=="kroki", un
// MermaidElement ya no debe hacer bail-out — mismo criterio que
// slidelang/internal/generator/offline.go's tryBuildNativeContext, que ya
// tenía este split.
func TestTryAllChartsNative_KrokiMermaidDoesNotBlockNativeCharts(t *testing.T) {
	doc := ast.NewAST(diagnostics.Position{Line: 1})
	block := ast.NewContentBlock(diagnostics.Position{Line: 1}, "content")
	block.Elements = append(block.Elements,
		ast.NewMermaidElement(diagnostics.Position{Line: 1}, "flowchart", "graph TD; A-->B"),
		nativeBarChartElementForGateTest(),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	fetcher, ok := TryAllChartsNative(doc, "png", "kroki")
	if !ok {
		t.Fatal("expected TryAllChartsNative to succeed for mermaid-via-kroki + a native-capable chart")
	}
	if fetcher == nil {
		t.Fatal("expected a non-nil fetcher when allNative == true")
	}
}

// TestTryAllChartsNative_KrokiStillBailsOnMapAndMath confirma que el
// backend kroki solo cubre mermaid — math y mapas siguen forzando Chromium
// sin importar diagramBackend, porque Kroki no los renderiza.
func TestTryAllChartsNative_KrokiStillBailsOnMapAndMath(t *testing.T) {
	mapDoc := ast.NewAST(diagnostics.Position{Line: 1})
	mapBlock := ast.NewContentBlock(diagnostics.Position{Line: 1}, "content")
	mapBlock.Elements = append(mapBlock.Elements, &ast.MapElement{})
	mapDoc.ContentBlocks = append(mapDoc.ContentBlocks, *mapBlock)
	if _, ok := TryAllChartsNative(mapDoc, "png", "kroki"); ok {
		t.Error("expected TryAllChartsNative to bail out on a map even with diagramBackend=kroki")
	}

	mathDoc := ast.NewAST(diagnostics.Position{Line: 1})
	mathBlock := ast.NewContentBlock(diagnostics.Position{Line: 1}, "content")
	mathBlock.Elements = append(mathBlock.Elements, ast.NewMathElement(diagnostics.Position{Line: 1}, "E = mc^2"))
	mathDoc.ContentBlocks = append(mathDoc.ContentBlocks, *mathBlock)
	if _, ok := TryAllChartsNative(mathDoc, "png", "kroki"); ok {
		t.Error("expected TryAllChartsNative to bail out on math even with diagramBackend=kroki")
	}
}
