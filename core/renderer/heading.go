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
