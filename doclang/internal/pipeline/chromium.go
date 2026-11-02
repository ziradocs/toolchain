// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
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
//   - docx: mermaid y math siempre; un chart solo si no rasteriza nativo
//     (DOCXGenerator.needsChromiumRendering/canRenderWithoutChromium). Los
//     mapas tienen render nativo.
//   - html: solo en los modos offline, por tipo de elemento presente, con
//     el mismo atajo de charts nativos que HTMLGenerator.
func ChromiumRequirements(doc *ast.AST, format, renderMode, diagramBackend string) []string {
	switch format {
	case "pdf":
		return []string{"PDF output"}
	case "docx":
		var reasons []string
		if renderer.HasMermaidElements(doc) {
			reasons = append(reasons, "mermaid diagrams in DOCX")
		}
		if renderer.HasMathElements(doc) {
			reasons = append(reasons, "math equations in DOCX")
		}
		if renderer.HasChartElements(doc) && !allChartsNative(doc) {
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

// allChartsNative replica DOCXGenerator.canRenderWithoutChromium para
// charts: elegibilidad no basta, se prueba el render real.
func allChartsNative(doc *ast.AST) bool {
	for _, block := range doc.ContentBlocks {
		for _, elem := range block.Elements {
			if e, ok := elem.(*ast.ChartElement); ok {
				width, height := renderer.ChartDimensions(e)
				if _, ok, err := renderer.RenderChartNativePNG(e, width, height); !ok || err != nil {
					return false
				}
			}
		}
	}
	return true
}
