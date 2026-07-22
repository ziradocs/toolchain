// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build js

package generator

import (
	"fmt"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
)

// SetupOfflineRenderContext's real implementation (offline.go) needs
// renderer/chromium, which isn't part of a WASM build (no headless Chrome in
// a browser). internal/cli/build.go references gen.SetupOfflineRenderContext
// unconditionally, so `go build ./...` under GOOS=js needs this method to
// exist even though the wasm entry package (cmd/wasm) never imports
// internal/cli and so never calls it — same reasoning as pdf_js.go's
// generatePDF stub.
func (g *Generator) SetupOfflineRenderContext(astNode *ast.AST, outputDir string, opts GeneratorOptions) (*renderer.RenderContext, func(), error) {
	return nil, func() {}, fmt.Errorf("offline rendering is not supported in the WebAssembly build")
}
