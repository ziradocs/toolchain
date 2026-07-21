// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "go.ziradocs.com/core/ast"

// PopulateInlineHTML rellena los campos "*HTML" de un AST completo (issue
// #64): title/heading/subtitle de cada ContentBlock y el contenido de prosa
// de cada elemento. Reusa, campo por campo, las mismas funciones de
// sanitización/markdown que RenderElementToHTML — esa es la fuente única de
// verdad del dialecto inline, así que el HTML resultante es idéntico al que
// produce --format html para el mismo elemento. Es aditivo (no toca los
// campos crudos) e idempotente.
func PopulateInlineHTML(doc *ast.AST, variables map[string]interface{}) {
	if doc == nil {
		return
	}
	for i := range doc.ContentBlocks {
		populateContentBlockHTML(&doc.ContentBlocks[i], variables)
	}
}

func populateContentBlockHTML(block *ast.ContentBlock, variables map[string]interface{}) {
	block.TitleHTML = ProcessVariablesSecure(block.Title, variables)
	block.HeadingHTML = ProcessVariablesSecure(block.Heading, variables)
	block.SubtitleHTML = ProcessVariablesSecure(block.Subtitle, variables)
	for _, elem := range block.Elements {
		populateElementHTML(elem, variables)
	}
}

// populateElementHTML cubre cada ast.Element con contenido de prosa. Para la
// mayoría de campos espeja field-por-field el tratamiento de
// RenderElementToHTML (html.go) — que doclang usa vía document_html.go — pero
// el "ground truth" real es slidelang, el ÚNICO CLI que emite --format
// json: sus dos pipelines de HTML no siempre coinciden entre sí (p. ej.
// SpecialBlockElement.Title lleva markdown en el template de slidelang
// pero no en RenderElementToHTML), así que cada divergencia deliberada de
// RenderElementToHTML está anotada en el case correspondiente.
//
// Mermaid/PlantUML/Chart/Map: solo Title lleva *HTML (vars-only, sin
// markdown, igual que el resto de campos "Title" de este archivo). Su
// "content"/"data"/"series"/"labels"/"rawJSON"/"markers" es fuente de
// diagrama o config, no texto para el DOM — ver la nota de no-goals en
// docs/architecture/json-ast-contract.md.
func populateElementHTML(element ast.Element, variables map[string]interface{}) {
	switch elem := element.(type) {
	case *ast.TextElement:
		if elem.IsRawHTML {
			elem.ContentHTML = ProcessVariablesEscapeValues(elem.Content, variables)
		} else {
			elem.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
		}

	case *ast.PointsElement:
		for i := range elem.Items {
			populatePointItemHTML(&elem.Items[i], variables)
		}

	case *ast.CodeElement:
		elem.ContentHTML = EscapeHTML(ProcessVariables(elem.Content, variables))

	case *ast.ImageElement:
		elem.AltHTML = ProcessVariablesSecure(elem.Alt, variables)
		elem.CaptionHTML = ProcessVariablesSecure(elem.Caption, variables)

	case *ast.TableElement:
		elem.HeadersHTML = make([]string, len(elem.Headers))
		for i, header := range elem.Headers {
			elem.HeadersHTML[i] = ProcessTextWithVariablesAndMarkdownSecure(header, variables)
		}
		elem.RowsHTML = make([][]string, len(elem.Rows))
		for i, row := range elem.Rows {
			rowHTML := make([]string, len(row))
			for j, cell := range row {
				rowHTML[j] = ProcessTextWithVariablesAndMarkdownSecure(cell, variables)
			}
			elem.RowsHTML[i] = rowHTML
		}
		elem.CaptionHTML = ProcessVariablesSecure(elem.Caption, variables)

	case *ast.QuoteElement:
		elem.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
		elem.AuthorHTML = ProcessVariablesSecure(elem.Author, variables)
		elem.SourceHTML = ProcessVariablesSecure(elem.Source, variables)

	case *ast.ChecklistElement:
		for i := range elem.Items {
			populateChecklistItemHTML(&elem.Items[i], variables)
		}

	case *ast.SpecialBlockElement:
		// Title SÍ lleva markdown aquí (a diferencia de otros campos "Title"
		// vars-only de este archivo): el template real de slidelang
		// (internal/generator/template/base.go, `{{.Title | markdown}}`)
		// aplica markdown al título de un callout, a diferencia de
		// RenderElementToHTML (usado solo por doclang, que no emite JSON).
		elem.TitleHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Title, variables)
		elem.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)

	case *ast.GridElement:
		elem.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
		for i := range elem.Columns {
			populateColumnHTML(&elem.Columns[i], variables)
		}

	case *ast.CodeGroupElement:
		for i := range elem.CodeBlocks {
			block := &elem.CodeBlocks[i]
			block.LabelHTML = EscapeHTML(ProcessVariables(block.Label, variables))
			block.ContentHTML = EscapeHTML(ProcessVariables(block.Content, variables))
		}

	case *ast.MermaidElement:
		elem.TitleHTML = ProcessVariablesSecure(elem.Title, variables)

	case *ast.PlantUMLElement:
		elem.TitleHTML = ProcessVariablesSecure(elem.Title, variables)

	case *ast.ChartElement:
		elem.TitleHTML = ProcessVariablesSecure(elem.Title, variables)

	case *ast.MapElement:
		elem.TitleHTML = ProcessVariablesSecure(elem.Title, variables)

	case *ast.MathElement:
		// Content es LaTeX crudo (fuente de fórmula, no prosa) — mismo no-goal
		// que Mermaid/PlantUML/Chart/Map documentado arriba en el paquete;
		// solo Caption (si existe) es prosa vars-only.
		elem.CaptionHTML = ProcessVariablesSecure(elem.Caption, variables)

	default:
		// Issue #82: silencio explícito, no un olvido. Todo ast.Element que
		// SÍ lleva contenido de prosa tiene un case arriba — verificado por
		// TestPopulateAndRenderElementHTMLCoverAllImplementers
		// (element_coverage_test.go), que falla si un tipo nuevo (o uno
		// existente sin `element()`) queda sin case acá y sin excepción
		// documentada en ese test. *ast.DirectiveNode (notes/directivas) y
		// *ast.ColumnElement (sub-elemento de GridElement.Columns, con su
		// propio populateColumnHTML) están documentados ahí como exclusiones
		// deliberadas.
	}
}

func populateColumnHTML(col *ast.ColumnElement, variables map[string]interface{}) {
	col.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(col.Content, variables)
	for _, nested := range col.Elements {
		populateElementHTML(nested, variables)
	}
}

func populatePointItemHTML(item *ast.PointItem, variables map[string]interface{}) {
	item.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(item.Content, variables)
	for i := range item.SubPoints {
		populatePointItemHTML(&item.SubPoints[i], variables)
	}
}

func populateChecklistItemHTML(item *ast.ChecklistItem, variables map[string]interface{}) {
	item.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(item.Content, variables)
	for i := range item.SubItems {
		populateChecklistItemHTML(&item.SubItems[i], variables)
	}
}
