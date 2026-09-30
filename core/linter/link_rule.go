// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

// LinkDestinationRule (LINK001) avisa cuando un enlace o una imagen apunta a
// un destino que el renderer no va a emitir: un esquema fuera de la
// allowlist (javascript:, data:, vbscript:, file: o cualquier esquema
// desconocido) o una URL que no se puede interpretar. El renderer conserva
// el texto del enlace (o el alt de la imagen inline) sin href, y una imagen
// de bloque no se dibuja; sin esta regla ese cambio pasaba sin ningún aviso.
//
// La decisión no se recalcula aquí: renderer.FindInlineLinks corre las
// mismas pasadas del renderer y reporta qué destinos descartó, así que la
// regla marca un destino exactamente cuando el HTML lo pierde. Para la imagen
// de bloque se usa renderer.ValidateURLScheme, el mismo filtro que aplican
// los generadores. ClassifyURL solo decide el texto del mensaje. No resuelve
// variables: un destino con {{...}} se conoce hasta renderizar y se deja
// pasar.
type LinkDestinationRule struct{}

func (r *LinkDestinationRule) Check(node ast.Node) []diagnostics.Diagnostic {
	block, ok := node.(*ast.ContentBlock)
	if !ok {
		return nil
	}
	var diags []diagnostics.Diagnostic
	inline := func(pos diagnostics.Position, texts ...string) {
		for _, text := range texts {
			for _, link := range renderer.FindInlineLinks(text) {
				if !link.Dropped || strings.Contains(link.Destination, "{{") {
					continue
				}
				dest := renderer.DecodeLinkDestination(link.Destination)
				diags = append(diags, linkDestinationDiagnostic(link.Destination, dest, link.Image, false, pos))
			}
		}
	}

	pos := block.GetPosition()
	inline(pos, block.Title, block.Heading, block.Subtitle)

	var visit func(ast.Element)
	var visitPoints func([]ast.PointItem, diagnostics.Position)
	var visitChecklist func([]ast.ChecklistItem, diagnostics.Position)
	visitPoints = func(items []ast.PointItem, fallback diagnostics.Position) {
		for _, it := range items {
			p := positionOr(it.GetPosition(), fallback)
			inline(p, it.Content)
			visitPoints(it.SubPoints, p)
		}
	}
	visitChecklist = func(items []ast.ChecklistItem, fallback diagnostics.Position) {
		for _, it := range items {
			p := positionOr(it.GetPosition(), fallback)
			inline(p, it.Content)
			visitChecklist(it.SubItems, p)
		}
	}
	visit = func(el ast.Element) {
		p := positionOr(el.GetPosition(), pos)
		switch e := el.(type) {
		case *ast.TextElement:
			inline(p, e.Content)
		case *ast.HeadingElement:
			inline(p, e.Text)
		case *ast.PointsElement:
			visitPoints(e.Items, p)
		case *ast.ChecklistElement:
			visitChecklist(e.Items, p)
		case *ast.QuoteElement:
			inline(p, e.Content)
		case *ast.TableElement:
			inline(p, e.Headers...)
			for _, row := range e.Rows {
				inline(p, row...)
			}
			for _, row := range e.TableRows {
				for _, cell := range row.Cells {
					inline(p, cell.Content)
				}
			}
		case *ast.SpecialBlockElement:
			inline(p, e.Title, e.Content)
			for _, nested := range e.Elements {
				visit(nested)
			}
		case *ast.GridElement:
			inline(p, e.Content)
			for _, col := range e.Columns {
				cp := positionOr(col.GetPosition(), p)
				inline(cp, col.Content)
				for _, nested := range col.Elements {
					visit(nested)
				}
			}
		case *ast.MetricElement:
			inline(p, e.Label, e.Value, e.Delta, e.Caption)
		case *ast.QuizElement:
			inline(p, e.Question, e.Explanation)
			inline(p, e.Options...)
		case *ast.PollElement:
			inline(p, e.Question)
			inline(p, e.Options...)
		case *ast.ImageElement:
			// IMG001 ya cubre la fuente vacía.
			if e.Source != "" && !strings.Contains(e.Source, "{{") && renderer.ValidateURLScheme(e.Source) == "" {
				diags = append(diags, linkDestinationDiagnostic(e.Source, e.Source, true, true, p))
			}
		}
	}
	for _, el := range block.Elements {
		visit(el)
	}
	return diags
}

// linkDestinationDiagnostic arma el LINK001 de un destino que el renderer
// descartó: written es el destino como aparece en la fuente (lo que muestra
// el mensaje) y dest el que se validó, ya decodificado. block distingue la
// imagen de bloque (que no se dibuja) de la inline (que deja su alt como
// texto).
func linkDestinationDiagnostic(written, dest string, image, block bool, pos diagnostics.Position) diagnostics.Diagnostic {
	problem, scheme := renderer.ClassifyURL(dest)
	kind, effect := "Link", "it is rendered as its text, without the link"
	if image {
		kind, effect = "Image", "it is rendered as its alt text, without the image"
		if block {
			effect = "the image is not rendered"
		}
	}
	var msg string
	if problem == renderer.URLSchemeNotAllowed {
		msg = fmt.Sprintf("%s destination %q uses the %q scheme, which is not allowed (use http, https, mailto, tel, ftp or a relative URL); %s",
			kind, written, scheme+":", effect)
	} else {
		msg = fmt.Sprintf("%s destination %q is not a valid URL; %s", kind, written, effect)
	}
	return diagnostics.NewWarning(msg, pos, "linter").WithRuleID("LINK001")
}

// positionOr devuelve p, o fallback si p no tiene línea.
func positionOr(p, fallback diagnostics.Position) diagnostics.Position {
	if p.Line == 0 {
		return fallback
	}
	return p
}
