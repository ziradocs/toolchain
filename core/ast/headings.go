// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"fmt"
	"strings"
)

// ValidateHeading comprueba un HeadingElement que llega de cualquier lado
// (parser, JSON, filtro): nivel 1-6, texto de una sola línea no vacío y un
// anchor dentro de la lista blanca [a-z0-9_-] que produce
// renderer.SanitizeAnchor. El anchor se interpola en un atributo HTML, así
// que un valor fuera de esa lista se rechaza en vez de sanearse en silencio.
// Un anchor vacío es válido porque el legado también lo produce (un
// encabezado hecho solo de emoji en DocLang).
func ValidateHeading(h *HeadingElement) error {
	if h.Level < 1 || h.Level > 6 {
		return fmt.Errorf("heading level %d is outside 1-6", h.Level)
	}
	if strings.TrimSpace(h.Text) == "" {
		return fmt.Errorf("heading text cannot be empty")
	}
	if strings.ContainsAny(h.Text, "\r\n") {
		return fmt.Errorf("heading text must be a single line")
	}
	for _, r := range h.Anchor {
		allowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !allowed {
			return fmt.Errorf("heading anchor %q must use only [a-z0-9_-]", h.Anchor)
		}
	}
	return nil
}

// TypedHeadingFingerprints registra el nivel de cada encabezado con NodeID
// autoral. Un filtro puede editar el texto, el anchor o el orden, pero no
// cambiar el nivel de un encabezado identificado ni hacerlo desaparecer
// conservando la capability. Los encabezados sin NodeID no tienen identidad
// que preservar y quedan fuera, igual que en el resto del contrato: nunca se
// identifica por texto ni por posición.
func TypedHeadingFingerprints(doc *AST) map[string]int {
	out := map[string]int{}
	_ = Walk(doc, func(n Node) error {
		if h, ok := n.(*HeadingElement); ok && h.GetNodeID() != "" {
			out[h.GetNodeID()] = h.Level
		}
		return nil
	})
	return out
}

// SourceDeclaresCapability reporta si el frontmatter fuente pidió capability
// en `ast_capabilities`.
func SourceDeclaresCapability(fm *FrontMatterNode, capability string) bool {
	if fm == nil {
		return false
	}
	for _, c := range fm.ASTCapabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// PromoteTypedHeadings reemplaza, cuando el documento fuente declaró
// typed-headings-v1, cada encabezado legado construido por el parser por un
// HeadingElement con la fuente autoral que el parser dejó en memoria
// (TextElement.HeadingSource/HeadingAnchor). Conserva posición, NodeID y
// comentarios. No toca un TextElement RawHTML sin esa fuente (por ejemplo
// uno decodificado de JSON), porque des-renderizar HTML no es invertible.
func PromoteTypedHeadings(doc *AST) {
	if doc == nil || !SourceDeclaresCapability(doc.FrontMatter, TypedHeadingsCapability) {
		return
	}
	for i := range doc.ContentBlocks {
		promoteHeadingElements(doc.ContentBlocks[i].Elements)
	}
}

func promoteHeadingElements(elements []Element) {
	for i, el := range elements {
		switch e := el.(type) {
		case *TextElement:
			if e.IsRawHTML && e.Level > 0 && e.HeadingSource != "" {
				h := NewHeadingElement(e.Position, e.Level, e.HeadingSource, e.HeadingAnchor)
				h.EndPosition = e.EndPosition
				h.NodeID = e.NodeID
				h.Comments = e.Comments
				elements[i] = h
			}
		case *SpecialBlockElement:
			promoteHeadingElements(e.Elements)
		case *GridElement:
			for j := range e.Columns {
				promoteHeadingElements(e.Columns[j].Elements)
			}
		}
	}
}
