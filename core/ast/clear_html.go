// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

// ClearRenderedHTML pone en blanco todos los campos "*HTML" (pre-renderizados,
// pre-sanitizados) de doc. Es la mitad defensiva del diseño de seguridad de
// los filtros de #240 (ast/transform en core, ver esa etapa en
// cada build.go): un filtro de terceros recibe el AST crudo SIN *HTML (el
// slot del pipeline corre antes de renderer.PopulateInlineHTML), pero nunca
// se confía en que el subproceso los deje así — si un filtro buggy o
// malicioso agrega/mantiene contenido en un campo *HTML, ese HTML re-entra
// al pipeline SIN pasar por el sanitizador (ProcessTextWithVariablesAndMarkdownSecure,
// etc.), reabriendo exactamente el vector XSS que el gate de seguridad del
// MVP cerró. ClearRenderedHTML se llama SIEMPRE sobre lo que devuelve un
// filtro externo, antes de seguir el pipeline; *HTML se re-deriva después
// por el camino normal (PopulateInlineHTML).
//
// Cobertura de tipos garantizada por TestClearRenderedHTMLCoversAllImplementers
// (clear_html_test.go) — mismo patrón de introspección de código fuente que
// renderer/element_coverage_test.go: un ast.Element nuevo con campo *HTML sin
// case acá hace fallar ese test.
func ClearRenderedHTML(doc *AST) {
	if doc == nil {
		return
	}
	for i := range doc.ContentBlocks {
		clearContentBlockHTML(&doc.ContentBlocks[i])
	}
}

func clearContentBlockHTML(block *ContentBlock) {
	block.TitleHTML = ""
	block.HeadingHTML = ""
	block.SubtitleHTML = ""
	for _, elem := range block.Elements {
		clearElementHTML(elem)
	}
}

func clearElementHTML(element Element) {
	switch elem := element.(type) {
	case *TextElement:
		elem.ContentHTML = ""

	case *PointsElement:
		for i := range elem.Items {
			clearPointItemHTML(&elem.Items[i])
		}

	case *CodeElement:
		elem.ContentHTML = ""

	case *ImageElement:
		elem.AltHTML = ""
		elem.CaptionHTML = ""

	case *TableElement:
		elem.HeadersHTML = nil
		elem.RowsHTML = nil
		elem.CaptionHTML = ""

	case *QuoteElement:
		elem.ContentHTML = ""
		elem.AuthorHTML = ""
		elem.SourceHTML = ""

	case *ChecklistElement:
		for i := range elem.Items {
			clearChecklistItemHTML(&elem.Items[i])
		}

	case *SpecialBlockElement:
		elem.TitleHTML = ""
		elem.ContentHTML = ""

	case *GridElement:
		elem.ContentHTML = ""
		for i := range elem.Columns {
			clearColumnHTML(&elem.Columns[i])
		}

	case *CodeGroupElement:
		for i := range elem.CodeBlocks {
			elem.CodeBlocks[i].LabelHTML = ""
			elem.CodeBlocks[i].ContentHTML = ""
		}

	case *MermaidElement:
		elem.TitleHTML = ""

	case *PlantUMLElement:
		elem.TitleHTML = ""

	case *ChartElement:
		elem.TitleHTML = ""

	case *MapElement:
		elem.TitleHTML = ""

	case *MathElement:
		elem.CaptionHTML = ""

	case *DirectiveNode, *ColumnElement:
		// Sin campos *HTML propios (DirectiveNode) o ya cubierto por
		// clearColumnHTML cuando aparece dentro de un Grid (ColumnElement no
		// aparece suelto en block.Elements — mismas exclusiones documentadas
		// que excludedFromElementHTMLCoverage en renderer/element_coverage_test.go).
	}
}

func clearColumnHTML(col *ColumnElement) {
	col.ContentHTML = ""
	for _, nested := range col.Elements {
		clearElementHTML(nested)
	}
}

func clearPointItemHTML(item *PointItem) {
	item.ContentHTML = ""
	for i := range item.SubPoints {
		clearPointItemHTML(&item.SubPoints[i])
	}
}

func clearChecklistItemHTML(item *ChecklistItem) {
	item.ContentHTML = ""
	for i := range item.SubItems {
		clearChecklistItemHTML(&item.SubItems[i])
	}
}
