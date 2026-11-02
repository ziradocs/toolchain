// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"

	"go.ziradocs.com/doclang/v2/internal/generator"
)

// ChromiumRequirements dice qué partes de doc necesitan Chromium para
// generarse en format con renderMode; vacío = ninguna. Sirve para fallar
// ANTES de generar con un mensaje claro: el generador DOCX, si no encuentra
// Chromium, solo deja un warning en el log y omite los diagramas y las
// ecuaciones, y un servidor MCP no tiene log visible.
//
// Replica, con funciones exportadas de core/renderer, los criterios de
// cada generador:
//   - pdf: siempre (imprime con Chromium).
//   - docx: mermaid, math e imágenes SVG locales siempre; un chart solo si
//     no rasteriza nativo (DOCXGenerator.needsChromiumRendering/
//     canRenderWithoutChromium). Los mapas tienen render nativo.
//   - html: solo en los modos offline, por tipo de elemento presente, con
//     el mismo atajo de charts nativos que HTMLGenerator.
func ChromiumRequirements(doc *ast.AST, format, renderMode, diagramBackend string) []string {
	switch format {
	case "pdf":
		return []string{"PDF output"}
	case "docx":
		var reasons []string
		// El generador DOCX recorre el documento entero (bloques especiales
		// y columnas incluidos, generator.WalkElements); los renderer.Has*
		// de core solo miran el primer nivel de cada sección.
		if docxHas(doc, func(e ast.Element) bool { _, ok := e.(*ast.MermaidElement); return ok }) {
			reasons = append(reasons, "mermaid diagrams in DOCX")
		}
		if docxHas(doc, func(e ast.Element) bool { _, ok := e.(*ast.MathElement); return ok }) {
			reasons = append(reasons, "math equations in DOCX")
		}
		if generator.HasSVGImages(doc) {
			reasons = append(reasons, "SVG images in DOCX")
		}
		if !allChartsNative(doc) {
			reasons = append(reasons, "charts that cannot be rasterized natively in DOCX")
		}
		return reasons
	case "html":
		if renderMode != "offline-assets" && renderMode != "offline-inline" {
			return nil
		}
		var reasons []string
		mermaid := diagramBackend != "kroki" && renderer.HasMermaidElements(doc)
		chart := renderer.HasChartElements(doc)
		mapEl := renderer.HasMapElements(doc)
		math := renderer.HasMathElements(doc)
		if chart && !mermaid && !mapEl && !math {
			if _, ok := renderer.TryAllChartsNative(doc, "png", diagramBackend); ok {
				chart = false
			}
		}
		if mermaid {
			reasons = append(reasons, "mermaid diagrams in offline HTML")
		}
		if chart {
			reasons = append(reasons, "charts in offline HTML")
		}
		if mapEl {
			reasons = append(reasons, "maps in offline HTML")
		}
		if math {
			reasons = append(reasons, "math equations in offline HTML")
		}
		return reasons
	}
	return nil
}

// docxHas dice si algún elemento del documento, a cualquier profundidad,
// cumple match.
func docxHas(doc *ast.AST, match func(ast.Element) bool) bool {
	found := false
	generator.WalkElements(doc, func(e ast.Element) bool {
		found = match(e)
		return !found
	})
	return found
}

// allChartsNative replica DOCXGenerator.canRenderWithoutChromium para
// charts, a cualquier profundidad: elegibilidad no basta, se prueba el
// render real. Sin charts es true.
func allChartsNative(doc *ast.AST) bool {
	return !docxHas(doc, func(elem ast.Element) bool {
		e, ok := elem.(*ast.ChartElement)
		if !ok {
			return false
		}
		width, height := renderer.ChartDimensions(e)
		_, ok, err := renderer.RenderChartNativePNG(e, width, height)
		return !ok || err != nil
	})
}
