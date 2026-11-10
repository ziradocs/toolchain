// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "go.ziradocs.com/core/v2/ast"

// PopulateInlineHTML rellena los campos "*HTML" de un AST completo (issue
// #64): title/heading/subtitle de cada ContentBlock y el contenido de prosa
// de cada elemento. Reusa, campo por campo, las mismas funciones de
// sanitización/markdown que RenderElementToHTML — esa es la fuente única de
// verdad del dialecto inline, así que el HTML resultante es idéntico al que
// produce --format html para el mismo elemento. Es aditivo (no toca los
// campos crudos) e idempotente.
func PopulateInlineHTML(doc *ast.AST, variables map[string]interface{}) {
	PopulateInlineHTMLWithOptions(doc, variables, InlineHTMLOptions{})
}

// InlineHTMLOptions ajusta PopulateInlineHTMLWithOptions. El valor cero es el
// comportamiento de PopulateInlineHTML.
type InlineHTMLOptions struct {
	// LiteralCode no sustituye `{{nombre}}` en el código (issue #487): el
	// contentHTML de un CODE y de cada bloque de un code-group queda como su
	// contenido escapado, y en los *HTML de prosa no se tocan los code spans
	// ni las cercas de código. Es el par de RenderContext.LiteralCode: quien
	// renderiza el HTML con una tiene que poblar el AST con la otra, o el
	// contentHTML de get_ast no describe el HTML. doclang las activa;
	// slidelang no.
	LiteralCode bool
}

// PopulateInlineHTMLWithOptions es PopulateInlineHTML con opciones.
func PopulateInlineHTMLWithOptions(doc *ast.AST, variables map[string]interface{}, opts InlineHTMLOptions) {
	if doc == nil {
		return
	}
	if opts.LiteralCode {
		variables = withLiteralCode(variables)
	}
	for i := range doc.ContentBlocks {
		populateContentBlockHTML(&doc.ContentBlocks[i], variables, opts)
	}
}

func populateContentBlockHTML(block *ast.ContentBlock, variables map[string]interface{}, opts InlineHTMLOptions) {
	block.TitleHTML = proseLineVariablesSecure(block.Title, variables)
	block.HeadingHTML = proseLineVariablesSecure(block.Heading, variables)
	block.SubtitleHTML = proseLineVariablesSecure(block.Subtitle, variables)
	block.KickerHTML = proseLineVariablesSecure(block.Kicker, variables)
	for _, elem := range block.Elements {
		populateElementHTML(elem, variables, opts)
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
// https://ziradocs.com/docs/architecture/json-ast-contract/.
// populateElementHTML puebla los *HTML de element. optional lleva las
// opciones (a lo sumo una); sin ellas, las de PopulateInlineHTML.
func populateElementHTML(element ast.Element, variables map[string]interface{}, optional ...InlineHTMLOptions) {
	var opts InlineHTMLOptions
	if len(optional) > 0 {
		opts = optional[0]
	}
	switch elem := element.(type) {
	case *ast.TextElement:
		if elem.IsRawHTML {
			elem.ContentHTML = HeadingContentHTML(elem, variables)
		} else {
			elem.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
		}

	case *ast.HeadingElement:
		// Mismo orden que un párrafo y que el legado (HeadingContentHTML):
		// {{variables}} primero y después el Markdown inline de una línea,
		// para que el filtro de esquemas vea el destino ya sustituido.
		elem.TextHTML = ProcessInlineMarkdownSecureLine(proseLineVariables(elem.Text, variables))

	case *ast.PointsElement:
		for i := range elem.Items {
			populatePointItemHTML(&elem.Items[i], variables)
		}

	case *ast.CodeElement:
		elem.ContentHTML = EscapeHTML(codeBody(elem.Content, variables))

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

	case *ast.QuizElement:
		elem.QuestionHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Question, variables)
		elem.OptionsHTML = processStringSliceHTML(elem.Options, variables)
		elem.ExplanationHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Explanation, variables)

	case *ast.PollElement:
		elem.QuestionHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Question, variables)
		elem.OptionsHTML = processStringSliceHTML(elem.Options, variables)

	case *ast.MetricElement:
		elem.LabelHTML = ProcessVariablesSecure(elem.Label, variables)
		elem.ValueHTML = ProcessVariablesSecure(elem.Value, variables)
		elem.DeltaHTML = ProcessVariablesSecure(elem.Delta, variables)
		elem.CaptionHTML = ProcessVariablesSecure(elem.Caption, variables)

	case *ast.SpecialBlockElement:
		// Title SÍ lleva markdown aquí (a diferencia de otros campos "Title"
		// vars-only de este archivo): el template real de slidelang
		// (internal/generator/template/base.go, `{{.Title | markdown}}`)
		// aplica markdown al título de un callout, a diferencia de
		// RenderElementToHTML (usado solo por doclang, que no emite JSON).
		elem.TitleHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Title, variables)
		elem.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
		for _, nested := range elem.Elements {
			populateElementHTML(nested, variables, opts)
		}

	case *ast.GridElement:
		elem.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
		for i := range elem.Columns {
			populateColumnHTML(&elem.Columns[i], variables, opts)
		}

	case *ast.CodeGroupElement:
		for i := range elem.CodeBlocks {
			block := &elem.CodeBlocks[i]
			block.LabelHTML = EscapeHTML(ProcessVariables(block.Label, variables))
			block.ContentHTML = EscapeHTML(codeBody(block.Content, variables))
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

	case *ast.MediaElement:
		// Source y Poster son URLs, no prosa; Caption (media-figure-v1) es
		// prosa vars-only, igual que MathElement.Caption.
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

func populateColumnHTML(col *ast.ColumnElement, variables map[string]interface{}, opts InlineHTMLOptions) {
	col.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(col.Content, variables)
	for _, nested := range col.Elements {
		populateElementHTML(nested, variables, opts)
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

// processStringSliceHTML renderiza cada string de un slice a su HTML inline,
// devolviendo nil para un slice vacío para que el campo *HTML quede omitido en
// el JSON (`omitempty`) en vez de salir como un array vacío.
func processStringSliceHTML(values []string, variables map[string]interface{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = ProcessTextWithVariablesAndMarkdownSecure(v, variables)
	}
	return out
}
