// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/slidelang/v2/internal/generator/css"
)

// TestDetectRequiredElementsFromAST_CodeGroupRequestsCSS es el repro del
// audit 2026-09-11 (F5, C6): faltaba el case *ast.CodeGroupElement en el
// switch de detección de CSS requerido — un deck con un ":::code-group"
// nunca pedía su propio módulo CSS, así que TODOS sus paneles se mostraban
// apilados a la vez, con tabs sin estilo.
func TestDetectRequiredElementsFromAST_CodeGroupRequestsCSS(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	codeGroup := ast.NewCodeGroupElement(pos)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, codeGroup)
	astNode := ast.NewAST(pos)
	astNode.ContentBlocks = append(astNode.ContentBlocks, *block)

	g := &Generator{}
	required := g.detectRequiredElementsFromAST(astNode)

	found := false
	for _, e := range required {
		if e == "code_group" {
			found = true
		}
	}
	if !found {
		t.Fatalf("detectRequiredElementsFromAST no pidió \"code_group\" para un deck con CodeGroupElement, obtenidos: %v", required)
	}

	// Y el módulo pedido tiene que resolver a un archivo CSS real que
	// efectivamente oculte los paneles inactivos — no solo que el nombre
	// del módulo se pida, sino que DetectRequiredElements lo traduzca al
	// mismo módulo (css.go) y que el archivo exista con la regla que
	// importa.
	modules := css.DetectRequiredElements(required)
	loader := css.NewCSSFileLoader()
	loaded, _ := loader.LoadElementCSS(modules)
	if !strings.Contains(loaded, ".slidelang-code-block {") || !strings.Contains(loaded, "display: none") {
		t.Errorf("code_group.css no se cargó o no oculta los paneles inactivos por default; CSS cargado para módulos %v:\n%s", modules, loaded)
	}
}
