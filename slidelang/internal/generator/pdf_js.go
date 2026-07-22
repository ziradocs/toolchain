// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build js

package generator

import (
	"fmt"

	"go.ziradocs.com/core/v2/ast"
)

// generatePDF's real implementation (pdf.go) needs renderer/chromium, which
// isn't part of a WASM build (no headless Chrome in a browser). generator.go's
// format switch references g.generatePDF unconditionally, so this stub keeps
// that switch linkable under GOOS=js; the wasm entry package (cmd/wasm) never
// exercises the "pdf" format.
func (g *Generator) generatePDF(astNode *ast.AST, outputDir string, opts GeneratorOptions) error {
	return fmt.Errorf("pdf output is not supported in the WebAssembly build")
}
