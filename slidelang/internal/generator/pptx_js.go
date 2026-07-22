// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build js

package generator

import (
	"fmt"

	"go.ziradocs.com/core/v2/ast"
)

// generatePPTX's real implementation (pptx.go) pulls in pptxgo y el pipeline
// de rasterizado nativo/Chromium, que no forman parte de un build WASM.
// generator.go referencia g.generatePPTX incondicionalmente en su switch de
// formatos, así que este stub mantiene ese switch linkable bajo GOOS=js; el
// paquete de entrada wasm (cmd/wasm) nunca ejercita el formato "pptx".
// Mismo patrón que pdf_js.go / offline_js.go.
func (g *Generator) generatePPTX(astNode *ast.AST, outputDir string, opts GeneratorOptions) error {
	return fmt.Errorf("pptx output is not supported in the WebAssembly build")
}
