// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"fmt"

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
// los generadores. ClassifyURL solo decide el texto del mensaje.
//
// Las {{variables}} del frontmatter se sustituyen antes de buscar enlaces,
// igual que el renderer (FrontMatterNode.BuildVariables y después las pasadas
// inline), así que se evalúa el destino que de verdad se emite, aunque la
// variable sea solo una parte de él. Por eso la regla corre sobre el
// documento completo (*ast.AST) y no por bloque: el bloque no conoce el
// frontmatter. Un placeholder sin variable definida queda literal, como en el
// HTML.
type LinkDestinationRule struct{}

func (r *LinkDestinationRule) Check(node ast.Node) []diagnostics.Diagnostic {
	doc, ok := node.(*ast.AST)
	if !ok {
		return nil
	}
	variables := doc.FrontMatter.BuildVariables()
	var diags []diagnostics.Diagnostic
	for i := range doc.ContentBlocks {
		diags = append(diags, checkLinkDestinations(&doc.ContentBlocks[i], variables)...)
	}
	return diags
}

func checkLinkDestinations(block *ast.ContentBlock, variables map[string]interface{}) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	inline := func(pos diagnostics.Position, texts ...string) {
		for _, text := range texts {
			for _, link := range renderer.FindInlineLinks(renderer.ProcessVariables(text, variables)) {
				if !link.Dropped {
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
			// Un encabezado legado guarda en Content el <hN> ya armado, donde
			// no queda sintaxis de enlace; su fuente es HeadingSource, la
			// misma de la que el renderer lo vuelve a armar con las
			// variables sustituidas.
			if e.IsRawHTML && e.HeadingSource != "" {
				inline(p, e.HeadingSource)
			} else {
				inline(p, e.Content)
			}
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
			if source := renderer.ProcessVariables(e.Source, variables); source != "" && renderer.ValidateURLScheme(source) == "" {
				diags = append(diags, linkDestinationDiagnostic(source, source, true, true, p))
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
