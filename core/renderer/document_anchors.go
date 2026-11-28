// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/internal/anchors"
)

// DocumentSectionAnchors devuelve el id del <h1> de cada bloque de un
// documento, por índice de doc.ContentBlocks ("" para un bloque sin título,
// que no dibuja <h1>). Es la única fuente de ese id para todo lo que lo
// escribe o lo enlaza: el cuerpo HTML y su vista de página, el TOC, la barra
// lateral, el DOCX y el Markdown (issue #267). Antes cada uno lo volvía a
// derivar del título, así que dos secciones con el mismo título compartían
// id y el TOC llevaba las dos al primero.
//
// literalCode es RenderContext.LiteralCode: decide, como en el HTML, si un
// {{nombre}} dentro de un code span del título se sustituye. Todo formato de
// un mismo documento tiene que pasar el mismo valor para obtener los mismos
// ids (doclang usa true en todos).
//
// Los encabezados de nivel 2-6 traen su id en el AST (el parser ya los dejó
// únicos), así que se reservan tal cual; después cada título de sección
// toma, en orden, su id base (DocumentSectionAnchorBases) o el primer sufijo
// -N libre.
//
// Con los mismos títulos, esto da los mismos ids que la cuenta del parser,
// que reparte secciones y encabezados intercalados en orden de documento:
// los ids que acá se reservan de antemano son justo los que el parser le
// hizo evitar a cada título, y ningún encabezado posterior pudo quedarse con
// el id de un título, porque ese id ya estaba tomado cuando le tocó. Si el
// título que se dibuja no es el que se contó al parsear (un \ref que
// resolvió xref, un filtro, o una variable que cambió después: `doclang
// build --title` fija {{title}} en la copia que ven los generadores), la
// prioridad puede cambiar, pero los ids siguen siendo únicos y cada enlace
// del TOC llega a su encabezado: un título nunca toma un id que ya tenga un
// encabezado. Con `title: Notas`, `# {{title}}` y `## Notas`, compilar con
// `--title Otra` da otra y notas-2, no otra y notas: el parser contó la
// sección como notas. Es un límite conocido.
func DocumentSectionAnchors(doc *ast.AST, literalCode bool) []string {
	return documentSectionAnchors(doc, documentVariables(doc, literalCode))
}

// DocumentSectionAnchorBases devuelve el id base del título de cada bloque
// de doc, por índice ("" para un bloque sin título): DeriveAnchor del título
// con sus {{variables}} sustituidas como en el HTML, o "seccion" si no queda
// ningún carácter. El parser lo usa para contar las secciones al repartir
// los ids de los encabezados, y DocumentSectionAnchors para el id de cada
// sección, así que las dos cuentas parten de la misma base.
func DocumentSectionAnchorBases(doc *ast.AST, literalCode bool) []string {
	return sectionAnchorBases(doc, documentVariables(doc, literalCode))
}

// documentVariables son las variables con que GenerateDocumentHTML
// sustituye el texto del documento.
func documentVariables(doc *ast.AST, literalCode bool) map[string]interface{} {
	if doc == nil {
		return nil
	}
	variables := doc.FrontMatter.BuildVariables()
	if variables == nil {
		variables = make(map[string]interface{})
	}
	if literalCode {
		variables = withLiteralCode(variables)
	}
	return variables
}

func sectionAnchorBases(doc *ast.AST, variables map[string]interface{}) []string {
	if doc == nil {
		return nil
	}
	out := make([]string, len(doc.ContentBlocks))
	anchors.WalkDocument(doc, func(i int, title string) {
		out[i] = anchors.DocumentBase(DeriveAnchor(proseLineVariables(title, variables)))
	}, nil)
	return out
}

func documentSectionAnchors(doc *ast.AST, variables map[string]interface{}) []string {
	if doc == nil {
		return nil
	}
	var set anchors.Set
	anchors.WalkDocument(doc, nil, func(el ast.Element) {
		if id, ok := anchors.HeadingID(el); ok && id != "" {
			set.Reserve(id)
		}
	})
	bases := sectionAnchorBases(doc, variables)
	out := make([]string, len(doc.ContentBlocks))
	anchors.WalkDocument(doc, func(i int, _ string) {
		out[i] = set.Unique(bases[i])
	}, nil)
	return out
}
