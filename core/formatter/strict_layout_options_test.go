// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// blockWithLayoutConfig arma un slide mínimo con opciones de layout.
func blockWithLayoutConfig(blockType string, cfg *ast.LayoutConfig) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, blockType)
	block.Title = "X"
	block.LayoutConfig = cfg
	block.Elements = []ast.Element{ast.NewTextElement(pos, "Texto.")}
	return &ast.AST{
		FrontMatter:   &ast.FrontMatterNode{BaseNode: ast.BaseNode{Type: ast.NodeTypeFrontMatter, Position: pos}, Mode: "strict", Title: "Deck"},
		ContentBlocks: []ast.ContentBlock{*block},
	}
}

// Lo que sale del formatter tiene que volver a entrar.
//
// Un AST con `hero` y `Columns: 3` es alcanzable: el linter solo ADVIERTE
// (LAYOUT_OPTION_NOT_APPLICABLE) y ast/decode.go lo acepta desde JSON. Emitir
// esa opción producía un `columns: 3` bajo un `SLIDE hero` que el parser
// strict rechaza con un error duro — el formatter fabricaba un archivo que no
// compila, que es peor que no re-emitir una opción que ese layout nunca iba a
// leer.
func TestFormatStrict_DropsOptionsTheLayoutDoesNotAccept(t *testing.T) {
	out, err := FormatStrict(blockWithLayoutConfig("hero", &ast.LayoutConfig{Columns: 3, Align: "left"}))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}

	if strings.Contains(out, "columns:") {
		t.Errorf("se emitió `columns` sobre un `hero`, que no la acepta:\n%s", out)
	}
	// `align` sí es suya y tiene que seguir estando.
	if !strings.Contains(out, "align: left") {
		t.Errorf("se perdió `align`, que `hero` sí acepta:\n%s", out)
	}

	// Y lo emitido re-parsea sin errores, que es el contrato entero.
	_, diags := parser.New(util.NewNoop()).Parse(out, "test.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("lo emitido no vuelve a parsear: %s\n%s", d.Message, out)
		}
	}
}

// El caso normal no cambia: una opción que el layout sí acepta se re-emite.
func TestFormatStrict_KeepsAcceptedOptions(t *testing.T) {
	out, err := FormatStrict(blockWithLayoutConfig("comparison", &ast.LayoutConfig{Columns: 2}))
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if !strings.Contains(out, "columns: 2") {
		t.Errorf("se perdió `columns`, que `comparison` sí acepta:\n%s", out)
	}
}
