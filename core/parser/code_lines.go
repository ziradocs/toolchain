// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"

	"go.ziradocs.com/core/v2/util"
)

// codeLiteralLines dice qué líneas de source, un documento entero
// (frontmatter incluido), lee el parser como código literal: el elemento i es
// la línea i+1. Código literal es lo que CodeParser y CodeGroupParser guardan
// tal cual (elements.ParseContext.RecordCode): la cabecera y el cuerpo de un
// CODE strict, una cerca de código con su apertura y su cierre, y la cabecera y
// cada pestaña de un :::code-group. Corre ParseDocument sin la normalización
// de flex, porque la normalización reescribe el texto y sus líneas ya no son
// las de source.
//
// Es la vista por líneas que usan las pruebas para comparar el parser con
// otros recorridos. core/include no la usa: decide sobre el AST con la
// normalización prendida, como doclang (include.ExpandWithOptions).
func codeLiteralLines(source string) []bool {
	lines := make([]bool, strings.Count(source, "\n")+1)
	p := New(util.NewNoop())
	p.SetNormalization(false)
	p.recordCode = func(line int) {
		if line >= 1 && line <= len(lines) {
			lines[line-1] = true
		}
	}
	p.ParseDocument(source, "")
	return lines
}
