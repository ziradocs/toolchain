// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// docxThemeSample cubre texto, enlace, código, tabla, listas (ordenada, con
// viñetas y anidada) y los cinco bloques especiales en un solo documento.
// Solo usa constructores que ya existían: las huellas y los golden de los
// temas embebidos se generaron con este mismo documento sobre origin/main.
func docxThemeSample() *ast.AST {
	doc := astWithTable()
	pos := diagnostics.NewPosition(7, 1)

	ordered := ast.NewPointsElement(pos)
	ordered.ListType = "ordered"
	ordered.Items = []ast.PointItem{
		{Content: "Primero", SubPoints: []ast.PointItem{{Content: "Anidado uno"}, {Content: "Anidado dos"}}},
		{Content: "Segundo"},
	}
	bullets := ast.NewPointsElement(pos)
	bullets.Items = []ast.PointItem{{Content: "Viñeta A"}, {Content: "Viñeta B"}}

	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements,
		ast.NewTextElement(pos, "Texto con `codigo` y [enlace](https://example.com)."),
		ast.NewCodeElement(diagnostics.NewPosition(8, 1), "go", "package main\n"),
		ordered,
		bullets,
	)
	for _, kind := range []string{"info", "warning", "danger", "success", "tip"} {
		doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements,
			ast.NewSpecialBlockElement(pos, kind, "Contenido del bloque "+kind),
		)
	}
	return doc
}
