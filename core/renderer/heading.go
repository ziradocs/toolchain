// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"

	"go.ziradocs.com/core/v2/ast"
)

// HeadingHTML es el único productor del fragmento `<hN id="anchor">…</hN>`
// de un encabezado. Lo usan el constructor legado (elements.BuildHeadingElement)
// y la bajada de un ast.HeadingElement, de modo que un documento con
// typed-headings-v1 y el mismo documento sin él producen HTML idéntico byte a
// byte. El TOC, la numeración y el xref de DocLang re-extraen encabezados de
// este mismo fragmento, así que su forma es un contrato y no un detalle.
//
// El texto pasa por ProcessInlineMarkdownSecureLine (escapa y luego aplica el
// Markdown inline de una línea); el anchor debe venir ya saneado.
func HeadingHTML(level int, text, anchor string) string {
	return fmt.Sprintf("<h%d id=\"%s\">%s</h%d>", level, anchor, ProcessInlineMarkdownSecureLine(text), level)
}

// HeadingContentHTML devuelve el `<hN id>` de un TextElement crudo
// (encabezado legado) con sus {{variables}} resueltas, con la misma regla que
// un párrafo: las variables se sustituyen en la fuente y después corre el
// Markdown inline, así que el filtro de esquemas ve el destino final y el
// Markdown de un valor se interpreta igual que en el cuerpo. El parser arma
// Content antes de conocer las variables, por eso se vuelve a armar desde
// HeadingSource.
//
// Solo se reconstruye mientras Content siga siendo el que se armó desde esa
// fuente: igual a HeadingContent (que xref mantiene al día cuando reescribe un
// \ref del encabezado) o igual a lo que HeadingHTML produce con ella. Si no
// hay fuente (llegó por un --filter externo) o Content se cambió por otro
// camino, se sustituye sobre Content con ProcessVariablesEscapeValues, que no
// entra a las etiquetas.
func HeadingContentHTML(el *ast.TextElement, variables map[string]interface{}) string {
	if el.Level > 0 && el.HeadingSource != "" &&
		(el.Content == el.HeadingContent || el.Content == HeadingHTML(el.Level, el.HeadingSource, el.HeadingAnchor)) {
		return HeadingHTML(el.Level, ProcessVariables(el.HeadingSource, variables), el.HeadingAnchor)
	}
	return ProcessVariablesEscapeValues(el.Content, variables)
}

// LegacyHeadingElement baja un HeadingElement al TextElement RawHTML con
// Level que producen los documentos legados. Conserva posición, NodeID,
// comentarios y LangRuns; ContentHTML se reconstruye desde TextHTML cuando ya
// fue poblado. Los generadores (HTML, PDF, Markdown, PPTX, TOC, numeración)
// consumen solo esta forma, por eso no necesitan ramas por dialecto ni por
// versión.
func LegacyHeadingElement(h *ast.HeadingElement) *ast.TextElement {
	el := ast.NewRawHTMLTextElement(h.Position, HeadingHTML(h.Level, h.Text, h.Anchor))
	el.EndPosition = h.EndPosition
	el.NodeID = h.NodeID
	el.Comments = h.Comments
	el.Level = h.Level
	el.LangRuns = h.LangRuns
	el.DiscardedLangRuns = h.DiscardedLangRuns
	el.HeadingSource = h.Text
	el.HeadingAnchor = h.Anchor
	el.HeadingContent = el.Content
	if h.TextHTML != "" {
		el.ContentHTML = fmt.Sprintf("<h%d id=\"%s\">%s</h%d>", h.Level, h.Anchor, h.TextHTML, h.Level)
	}
	return el
}

// LowerTypedHeadings devuelve una copia superficial de doc en la que cada
// HeadingElement (incluidos los de bloques especiales y columnas de grid)
// quedó reemplazado por su forma legada. doc no se modifica: la salida JSON y
// el formatter siguen viendo el nodo tipado. Sin encabezados tipados devuelve
// doc tal cual.
func LowerTypedHeadings(doc *ast.AST) *ast.AST {
	if doc == nil || !ast.UsesTypedHeadings(doc) {
		return doc
	}
	out := *doc
	out.ContentBlocks = make([]ast.ContentBlock, len(doc.ContentBlocks))
	for i, block := range doc.ContentBlocks {
		block.Elements = lowerHeadingElements(block.Elements)
		out.ContentBlocks[i] = block
	}
	return &out
}

func lowerHeadingElements(elements []ast.Element) []ast.Element {
	if elements == nil {
		return nil
	}
	out := make([]ast.Element, len(elements))
	for i, el := range elements {
		switch e := el.(type) {
		case *ast.HeadingElement:
			out[i] = LegacyHeadingElement(e)
		case *ast.SpecialBlockElement:
			copied := *e
			copied.Elements = lowerHeadingElements(e.Elements)
			out[i] = &copied
		case *ast.GridElement:
			copied := *e
			copied.Columns = make([]ast.ColumnElement, len(e.Columns))
			for j, col := range e.Columns {
				col.Elements = lowerHeadingElements(col.Elements)
				copied.Columns[j] = col
			}
			out[i] = &copied
		default:
			out[i] = el
		}
	}
	return out
}

// renderHeadingElement renderiza un HeadingElement exactamente como su forma
// legada, para que un consumidor que llame RenderElement sin bajar antes el
// documento obtenga el mismo HTML.
func renderHeadingElement(elem *ast.HeadingElement, variables map[string]interface{}) string {
	return renderTextElement(LegacyHeadingElement(elem), variables)
}
