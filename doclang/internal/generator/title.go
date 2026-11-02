// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// DocumentTitle resuelve el título de un documento para los metadatos de
// salida (el <title> del HTML, el título del PDF). Orden:
//
//  1. Con frontmatter, su `title:` tal cual, aunque esté vacío: un
//     documento que declara frontmatter conserva el comportamiento de
//     siempre. Los respaldos de abajo son solo para Markdown sin
//     frontmatter.
//  2. El primer `#` del documento: el bloque "title" que arma el parser
//     flex. Se lee del AST y no de las líneas crudas, para no confundir un
//     `# comentario` dentro de un bloque de código con un encabezado.
//  3. El nombre del archivo sin extensión, si se construyó desde un
//     archivo (sourcePath no vacío).
//  4. Vacío: una fuente en memoria (MCP) sin frontmatter ni `#`.
//
// No sintetiza un FrontMatterNode en el AST: eso prendería por defecto el
// TOC y la numeración de `doclang build` (que solo se activan cuando hay
// frontmatter), agregaría un bloque `---` y un `# título` extra a la salida
// Markdown, y cambiaría el AST que devuelve get_ast y que lee `doclang fmt`.
func DocumentTitle(doc *ast.AST, sourcePath string) string {
	if doc == nil {
		return ""
	}
	if doc.FrontMatter != nil {
		return doc.FrontMatter.Title
	}
	for _, block := range doc.ContentBlocks {
		if block.BlockType == "title" && block.Heading != "" {
			return block.Heading
		}
	}
	if sourcePath != "" {
		base := filepath.Base(sourcePath)
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	return ""
}
