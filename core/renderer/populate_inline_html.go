// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "go.ziradocs.com/core/v2/ast"

// PopulateInlineHTML rellena los campos "*HTML" de un AST completo (issue
// #64): title/heading/subtitle/kicker de cada ContentBlock (con el Markdown
// inline de una línea, ver BlockTitleHTML) y el contenido de prosa de cada
// elemento. Reusa, campo por campo, las mismas funciones de
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
	// PlainBlockTitles deja title/heading/subtitle/kicker de cada
	// ContentBlock como texto plano: {{variables}} sustituidas y todo
	// escapado, sin Markdown. Es lo que pinta doclang, que escribe el título
	// de una sección escapado en el <h1>, el TOC y el anchor; doclang la
	// activa para que su titleHTML describa ese HTML. Sin la opción esos
	// campos llevan el Markdown inline de una línea, como un encabezado `##`:
	// es lo que pinta slidelang en el encabezado de un slide.
	PlainBlockTitles bool
	// BlockProse arma la prosa de un ":::bloque" como la dibuja
	// RenderElementToHTML: el contentHTML de un SpecialBlockElement y el de
	// cada TextElement suelto entre sus Elements salen de
	// renderProseWithLists, con las listas numeradas y anidadas y un <p> por
	// párrafo cuando hay más de uno. Es lo que pinta doclang (HTML y PDF), que
	// la activa para que get_ast describa ese HTML. Sin la opción esos campos
	// pasan por ProcessInlineMarkdownSecure, como el contenido de un callout
	// en la plantilla de slidelang ({{.Content | markdown}}): una lista
	// numerada queda en líneas con <br>, una anidada sale plana y dos
	// párrafos salen juntos.
	BlockProse bool
}

// SetBlockProse fija BlockProse. Mismo motivo que SetPlainBlockTitles: un CLI
// que compila contra un core publicado sin el campo lo pide por interfaz.
func (o *InlineHTMLOptions) SetBlockProse(on bool) {
	o.BlockProse = on
}

// SetPlainBlockTitles fija PlainBlockTitles. Existe para que un CLI que
// compila contra un core publicado sin el campo lo pida por interfaz
// (interface{ SetPlainBlockTitles(bool) }) sin dejar de compilar, igual que
// RenderContext.SetLiteralCode.
func (o *InlineHTMLOptions) SetPlainBlockTitles(plain bool) {
	o.PlainBlockTitles = plain
}

// PopulateInlineHTMLWithOptions es PopulateInlineHTML con opciones.
func PopulateInlineHTMLWithOptions(doc *ast.AST, variables map[string]interface{}, opts InlineHTMLOptions) {
	if doc == nil {
		return
	}
	if opts.LiteralCode {
		variables = withLiteralCode(variables)
	}
	blockLine := BlockTitleHTML
	if opts.PlainBlockTitles {
		blockLine = proseLineVariablesSecure
	}
	for i := range doc.ContentBlocks {
		populateContentBlockHTML(&doc.ContentBlocks[i], variables, blockLine)
		if opts.BlockProse {
			populateBlockProseHTML(doc.ContentBlocks[i].Elements, variables, false)
		}
	}
}

// populateBlockProseHTML reescribe, sobre lo que ya pobló populateElementHTML,
// el contentHTML de la prosa de cada ":::bloque" (InlineHTMLOptions.BlockProse)
// con el mismo HTML que renderSpecialBlockElement: Content con
// renderLiteralProse y, entre sus Elements, cada TextElement que no es HTML
// crudo también (el renderer lo envuelve después con wrapTextHTML, como a un
// párrafo suelto, y su contentHTML no lleva ese <p>, igual que el de un
// TextElement de la sección). inBlock dice si elements son los Elements de un
// bloque; los de una columna de grid se dibujan como los de una sección.
func populateBlockProseHTML(elements []ast.Element, variables map[string]interface{}, inBlock bool) {
	for _, element := range elements {
		switch elem := element.(type) {
		case *ast.SpecialBlockElement:
			elem.ContentHTML = renderLiteralProse(elem.Content, variables, false)
			populateBlockProseHTML(elem.Elements, variables, true)
		case *ast.TextElement:
			if inBlock && !elem.IsRawHTML {
				elem.ContentHTML = renderLiteralProse(elem.Content, variables, false)
			}
		case *ast.GridElement:
			for i := range elem.Columns {
				populateBlockProseHTML(elem.Columns[i].Elements, variables, false)
			}
		}
	}
}

// BlockTitleHTML es el HTML inline de un campo de una línea del encabezado de
// un slide (title, heading, subtitle, kicker): las {{variables}} se sustituyen
// en la fuente y después corre el Markdown inline de una línea
// (ProcessInlineMarkdownSecureLine), en el mismo orden que un encabezado `##`
// (HeadingElement.TextHTML). Un "- " o un "1. " al principio no abre una
// lista y nunca sale HTML de bloque, porque va dentro de un <h1> o un <p>.
// Sin la opción de código literal es exactamente
// ProcessInlineMarkdownSecureLine(ProcessVariables(text, variables)), que es
// lo que slidelang pinta en el encabezado del slide.
func BlockTitleHTML(text string, variables map[string]interface{}) string {
	if text == "" {
		return ""
	}
	source, marks := proseLineMarkdownVariables(text, variables)
	return marks.html(ProcessInlineMarkdownSecureLine(source))
}

func populateContentBlockHTML(block *ast.ContentBlock, variables map[string]interface{}, blockLine func(string, map[string]interface{}) string) {
	block.TitleHTML = blockLine(block.Title, variables)
	block.HeadingHTML = blockLine(block.Heading, variables)
	block.SubtitleHTML = blockLine(block.Subtitle, variables)
	block.KickerHTML = blockLine(block.Kicker, variables)
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
// https://ziradocs.com/docs/architecture/json-ast-contract/.
func populateElementHTML(element ast.Element, variables map[string]interface{}) {
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
		text, marks := proseLineMarkdownVariables(elem.Text, variables)
		elem.TextHTML = marks.html(ProcessInlineMarkdownSecureLine(text))

	case *ast.PointsElement:
		for i := range elem.Items {
			populatePointItemHTML(&elem.Items[i], variables)
		}

	case *ast.CodeElement:
		elem.ContentHTML = EscapeHTML(codeBody(elem.Content, variables))

	case *ast.ImageElement:
		elem.AltHTML = proseLineVariablesSecure(elem.Alt, variables)
		elem.CaptionHTML = proseLineVariablesSecure(elem.Caption, variables)

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
		elem.CaptionHTML = proseLineVariablesSecure(elem.Caption, variables)

	case *ast.QuoteElement:
		elem.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
		elem.AuthorHTML = proseLineVariablesSecure(elem.Author, variables)
		elem.SourceHTML = proseLineVariablesSecure(elem.Source, variables)

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
		elem.LabelHTML = proseLineVariablesSecure(elem.Label, variables)
		elem.ValueHTML = proseLineVariablesSecure(elem.Value, variables)
		elem.DeltaHTML = proseLineVariablesSecure(elem.Delta, variables)
		elem.CaptionHTML = proseLineVariablesSecure(elem.Caption, variables)

	case *ast.SpecialBlockElement:
		// Title SÍ lleva markdown aquí (a diferencia de otros campos "Title"
		// vars-only de este archivo): el template real de slidelang
		// (internal/generator/template/base.go, `{{.Title | markdown}}`)
		// aplica markdown al título de un callout, a diferencia de
		// RenderElementToHTML (usado solo por doclang, que no emite JSON).
		elem.TitleHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Title, variables)
		elem.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
		for _, nested := range elem.Elements {
			populateElementHTML(nested, variables)
		}

	case *ast.GridElement:
		elem.ContentHTML = ProcessTextWithVariablesAndMarkdownSecure(elem.Content, variables)
		for i := range elem.Columns {
			populateColumnHTML(&elem.Columns[i], variables)
		}

	case *ast.CodeGroupElement:
		for i := range elem.CodeBlocks {
			block := &elem.CodeBlocks[i]
			block.LabelHTML = EscapeHTML(proseLineVariables(block.Label, variables))
			block.ContentHTML = EscapeHTML(codeBody(block.Content, variables))
		}

	case *ast.MermaidElement:
		elem.TitleHTML = proseLineVariablesSecure(elem.Title, variables)

	case *ast.PlantUMLElement:
		elem.TitleHTML = proseLineVariablesSecure(elem.Title, variables)

	case *ast.ChartElement:
		elem.TitleHTML = proseLineVariablesSecure(elem.Title, variables)

	case *ast.MapElement:
		elem.TitleHTML = proseLineVariablesSecure(elem.Title, variables)

	case *ast.MathElement:
		// Content es LaTeX crudo (fuente de fórmula, no prosa) — mismo no-goal
		// que Mermaid/PlantUML/Chart/Map documentado arriba en el paquete;
		// solo Caption (si existe) es prosa vars-only.
		elem.CaptionHTML = proseLineVariablesSecure(elem.Caption, variables)

	case *ast.MediaElement:
		// Source y Poster son URLs, no prosa; Caption (media-figure-v1) es
		// prosa vars-only, igual que MathElement.Caption.
		elem.CaptionHTML = proseLineVariablesSecure(elem.Caption, variables)

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
