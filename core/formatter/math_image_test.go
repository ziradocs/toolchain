// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"errors"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Una fórmula con render previo (math-image-v1) no tiene sintaxis fuente:
// los tres formateadores la rechazan en vez de perder la imagen.
func TestFormatRejectsMathImage(t *testing.T) {
	build := func() *ast.AST {
		pos := diagnostics.NewPosition(1, 1)
		doc := ast.NewAST(pos)
		block := ast.NewContentBlock(pos, "content")
		block.Title = "Uno"
		m := ast.NewMathElement(pos, "E = mc^2")
		m.Label = "eq:a"
		m.Image = "eq-a.png"
		block.Elements = []ast.Element{m}
		doc.ContentBlocks = []ast.ContentBlock{*block}
		ast.SetTableContract(doc)
		return doc
	}
	for name, format := range map[string]func(*ast.AST) (string, error){
		"document":        FormatDocument,
		"document strict": FormatDocumentStrict,
		"strict":          FormatStrict,
	} {
		_, err := format(build())
		var unsupported *UnsupportedElementError
		if !errors.As(err, &unsupported) || unsupported.NodeType != "math" {
			t.Errorf("%s: want UnsupportedElementError for math, got %v", name, err)
		}
	}
	// Sin imagen la misma fórmula se formatea.
	doc := build()
	doc.ContentBlocks[0].Elements[0].(*ast.MathElement).Image = ""
	ast.SetTableContract(doc)
	doc.SchemaVersion, doc.Capabilities = ast.LegacySchemaVersion, nil
	if _, err := FormatDocument(doc); err != nil {
		t.Fatalf("sin imagen: %v", err)
	}
}
