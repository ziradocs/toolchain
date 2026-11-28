// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/internal/anchors"
	"go.ziradocs.com/core/v2/renderer"
)

// assignDocumentAnchors deja únicos los ids de los encabezados de un
// documento (issue #267). Antes, dos `## Notas` daban dos id="notas": HTML
// inválido, y el segundo encabezado inalcanzable desde un [x](#notas-2), el
// TOC o un \ref.
//
// Cuentan, en orden de documento, todos los encabezados que DocLang dibuja:
// el título de cada sección (nivel 1) y los de nivel 2-6, también los de un
// bloque ::: (anchors.WalkDocument). El primero de cada id base lo conserva
// y los siguientes llevan -2, -3, … con la misma cuenta que SlideLang
// (anchors.Set). El id base de un encabezado de nivel 2-6 es el de siempre
// (el texto tal cual, o anchors.DocumentFallback si queda vacío); el de una
// sección es el que le da el HTML (renderer.DocumentSectionAnchorBases, con
// las {{variables}} sustituidas, en el modo de código literal de doclang).
//
// Un `id:` del autor (strict, HeadingAnchorDeclared) no se renombra: se
// reserva ANTES de repartir los demás, para que ningún encabezado derivado,
// anterior o posterior, choque con él. Dos `id:` iguales del autor se quedan
// como están, igual que antes de este cambio.
//
// El título de sección no tiene campo de id en el AST: el renderer lo vuelve
// a calcular con renderer.DocumentSectionAnchors, reservando primero los ids
// que esta pasada dejó en los encabezados de nivel 2-6. Con los mismos
// títulos, esa cuenta da los mismos ids que la de acá (ver su comentario).
//
// Solo se reescribe un encabezado cuyo id cambia, así que un documento sin
// ids repetidos ni vacíos queda byte a byte igual que antes.
func assignDocumentAnchors(doc *ast.AST) {
	var set anchors.Set
	anchors.WalkDocument(doc, nil, func(el ast.Element) {
		if t, ok := el.(*ast.TextElement); ok && t.HeadingAnchorDeclared {
			set.Reserve(t.HeadingAnchor)
		}
	})
	sections := renderer.DocumentSectionAnchorBases(doc, true)
	anchors.WalkDocument(doc,
		func(i int, _ string) {
			set.Unique(sections[i])
		},
		func(el ast.Element) {
			t, ok := el.(*ast.TextElement)
			if !ok || t.HeadingAnchorDeclared {
				return
			}
			if t.HeadingSource == "" {
				// Sin la fuente no se puede volver a armar el <hN>: se
				// respeta el id que trae. El parser siempre la deja; esto
				// cubre un elemento armado por otro camino.
				if id, ok := anchors.HeadingID(t); ok {
					set.Reserve(id)
				}
				return
			}
			anchor := set.Unique(anchors.DocumentBase(t.HeadingAnchor))
			if anchor != t.HeadingAnchor {
				setHeadingAnchor(t, anchor)
			}
		})
}

// setHeadingAnchor cambia el id de un encabezado legado que el parser acaba
// de armar, manteniendo juntos los tres campos que lo describen (ver
// ast.TextElement.HeadingContent).
func setHeadingAnchor(t *ast.TextElement, anchor string) {
	content := renderer.HeadingHTML(t.Level, t.HeadingSource, anchor)
	if t.Content == t.HeadingContent {
		t.Content = content
	}
	t.HeadingAnchor = anchor
	t.HeadingContent = content
}
