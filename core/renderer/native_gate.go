// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import "go.ziradocs.com/core/v2/ast"

// native_gate.go generaliza el gate que slidelang introdujo en
// tryBuildNativeContext (issue #164, slidelang/internal/generator/offline.go)
// para que doclang pueda evitar instanciar Chromium en el mismo caso común:
// un documento cuyo único elemento "rico" es un chart nativo-capaz (bar/
// line/pie/doughnut, sin Options ni IsJSONMode — ver SupportsNativeChartRendering
// en native_chart.go). slidelang mantiene su propia copia (no se refactorizó
// para reusar esto: ya tenía cobertura de tests completa antes de este
// cambio, y tocarla solo por deduplicación agregaba riesgo de regresión sin
// beneficio funcional) — este helper es la versión que doclang consume
// (issue "quitar Chrome del pipeline").

// TryAllChartsNative reporta si TODO chart en astNode rasteriza nativo
// (Go puro, sin Chromium) con ÉXITO — probado de verdad vía
// RenderChartNativePNG, no solo por elegibilidad de tipo (un chart
// type-eligible con datos malformados igual falla el render real). Devuelve
// (fetcher, false) en cuanto CUALQUIER elemento fuerza Chromium: math,
// mapas, un chart no nativo-capaz (combo/scatter/JSON-mode/con Options), un
// chart cuyo render real falla, o mermaid — salvo que diagramBackend ==
// "kroki", en cuyo caso mermaid se resuelve por HTTP puro (KrokiFetcher) y
// no debe hacer bail-out del gate (hallazgo de code-review sobre PR #138:
// esta función ignoraba diagramBackend por completo, así que un documento
// con mermaid-vía-kroki + un chart nativo seguía exigiendo Chromium solo
// por la presencia del MermaidElement, aunque ningún elemento del documento
// lo necesitara de verdad — mismo criterio que
// slidelang/internal/generator/offline.go's tryBuildNativeContext, que ya
// tenía este mismo split). imageFormat == "webp" fuerza false sin más:
// go-analyze/charts solo produce PNG.
//
// El fetcher retornado ya trae los bytes de CADA chart sembrados
// (ChartFetcher.Seed) cuando allNative == true, así que el caller nunca
// vuelve a rasterizar — una sola pasada por chart en todo el build, igual
// que el patrón original en slidelang.
func TryAllChartsNative(astNode *ast.AST, imageFormat string, diagramBackend string) (fetcher *NativeChartFetcher, allNative bool) {
	if imageFormat == "webp" {
		return nil, false
	}
	fetcher = NewNativeChartFetcher()
	if astNode == nil {
		return fetcher, true
	}
	for _, block := range astNode.ContentBlocks {
		for _, el := range block.Elements {
			switch e := el.(type) {
			case *ast.MermaidElement:
				if diagramBackend != "kroki" {
					return nil, false
				}
			case *ast.MathElement, *ast.MapElement:
				return nil, false
			case *ast.ChartElement:
				if !SupportsNativeChartRendering(e) {
					return nil, false
				}
				width, height := ChartDimensions(e)
				// Paleta por defecto: TryAllChartsNative es el gate de doclang
				// (issue "quitar Chrome del pipeline"), que no consume temas de
				// slidelang — motor-temas-v2.md queda fuera de doclang a
				// propósito (no comparte internal/generator/css/).
				data, ok, err := RenderChartNativePNG(e, width, height)
				if !ok || err != nil {
					return nil, false
				}
				fetcher.Seed(e, width, height, data)
			}
		}
	}
	return fetcher, true
}

// HasMermaidElements, HasChartElements, HasMapElements y HasMathElements
// reportan si astNode contiene al menos un elemento de ese tipo — mismo
// chequeo de presencia real que ya usaba slidelang's offline.go
// (hasMermaidElements, hasChromiumOnlyElements) para no arrancar Chromium
// por un modo pedido "por si acaso" (p. ej. --render-mode offline-inline en
// doclang aplica el mismo modo a mermaid/chart/map/math a la vez,
// independientemente de qué elementos tenga el documento — sin este
// chequeo, un documento de solo texto+mermaid igual pedía Chromium para
// chart/map/math que nunca aparecen, issue "quitar Chrome del pipeline").

func HasMermaidElements(astNode *ast.AST) bool {
	return hasElementType(astNode, func(el ast.Element) bool {
		_, ok := el.(*ast.MermaidElement)
		return ok
	})
}

func HasChartElements(astNode *ast.AST) bool {
	return hasElementType(astNode, func(el ast.Element) bool {
		_, ok := el.(*ast.ChartElement)
		return ok
	})
}

func HasMapElements(astNode *ast.AST) bool {
	return hasElementType(astNode, func(el ast.Element) bool {
		_, ok := el.(*ast.MapElement)
		return ok
	})
}

func HasMathElements(astNode *ast.AST) bool {
	return hasElementType(astNode, func(el ast.Element) bool {
		_, ok := el.(*ast.MathElement)
		return ok
	})
}

func hasElementType(astNode *ast.AST, match func(ast.Element) bool) bool {
	if astNode == nil {
		return false
	}
	for _, block := range astNode.ContentBlocks {
		for _, el := range block.Elements {
			if match(el) {
				return true
			}
		}
	}
	return false
}
