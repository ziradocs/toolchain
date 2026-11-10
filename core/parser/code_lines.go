// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"

	"go.ziradocs.com/core/v2/util"
)

// CodeLiteralLines dice qué líneas de source, un documento entero
// (frontmatter incluido), son código literal: el elemento i es la línea i+1.
// Código literal es lo que el parser guarda tal cual como texto de un bloque
// de código: la cabecera y el cuerpo de un CODE strict, una cerca de código
// con su apertura y su cierre, y la cabecera y cada pestaña de un
// :::code-group. Los diagramas, charts, mapas y ecuaciones no son código,
// tampoco escritos como cerca (```mermaid, ```math…), y una línea suelta
// dentro de un :::code-group, que no llega al AST, tampoco.
//
// No lo decide un recorrido aparte sino el parser de documentos mismo
// (ParseDocument, con el mismo despacho por `mode:`), así que coincide con
// lo que get_ast muestra como código en cualquier estructura: el cuerpo de
// una sección strict, una columna, un ::: flex o tipado. Lo usa core/include
// para no expandir un `@include` escrito dentro de código (issue #487).
//
// El parse corre sin la normalización de flex: lo que decide es la gramática
// de la fuente. En un documento flex que el normalizador reescribe, el AST
// final puede tener código donde la fuente no lo tenía, o al revés.
func CodeLiteralLines(source string) []bool {
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
