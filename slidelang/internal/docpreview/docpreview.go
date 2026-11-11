// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package docpreview lee y dibuja un documento DocLang en memoria con las
// reglas de `doclang build`, para el playground WASM (slidelang/cmd/wasm),
// que no puede importar el módulo doclang. Es el espejo de lo que doclang
// hace sin disco: el parseSource de su servidor MCP
// (doclang/internal/mcp/parse.go) y su Generator.RenderHTMLPreview
// (doclang/internal/generator/generator.go).
//
// Va en un paquete aparte, sin la etiqueta //go:build js de cmd/wasm, para
// que `go test` lo pruebe en el target nativo.
package docpreview

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
)

// Parse lee source como doclang: un guard de tamaño, ParseDocument (que elige
// el dialecto por el `mode:` del frontmatter, strict o flex) y, si no hubo
// errores, el transform de xref (numeración de figuras y tablas, \ref). Con
// errores de parseo devuelve el AST y los diagnósticos sin transformar, como
// el MCP de doclang.
//
// Las líneas @include quedan como las escribió el autor: el playground no
// tiene archivos que incluir, así que no hay expansión ni, por lo tanto, la
// opción KeepIncludesInCode de doclang build.
func Parse(log util.Logger, source string) (*ast.AST, []diagnostics.Diagnostic, error) {
	if err := util.CheckInputSize(len(source), util.DefaultMaxInputBytes); err != nil {
		return nil, nil, err
	}
	doc, diags := parser.New(log).ParseDocument(source, "")
	if doc == nil || hasError(diags) {
		return doc, diags, nil
	}
	// xref.Transform directo y no por transform.RunBuiltins como el MCP de
	// doclang: core/transform no compila con GOOS=js (process_unix.go usa
	// SysProcAttr.Setpgid, que js no tiene), y RunBuiltins solo agrega
	// chequeos de que un built-in no borre marcas de versión del AST.
	transformed, err := xref.Transform(doc)
	if err != nil {
		return doc, diags, err
	}
	return transformed, diags, nil
}

// Context es el contexto de render de doclang build para HTML: el código es
// literal (issue #487), así que una {{variable}} dentro de un bloque de
// código o de un code span queda como se escribió.
func Context() *renderer.RenderContext {
	ctx := renderer.NewDefaultRenderContext()
	ctx.LiteralCode = true
	return ctx
}

// HTML dibuja doc como un HTML autocontenido con opts, después de bajar los
// encabezados tipados como lo hace doclang antes de generar
// (renderer.LowerTypedHeadings) y con Context.
func HTML(doc *ast.AST, opts renderer.DocumentHTMLOptions) string {
	return renderer.GenerateDocumentHTML(renderer.LowerTypedHeadings(doc), opts, Context())
}

func hasError(diags []diagnostics.Diagnostic) bool {
	for _, d := range diags {
		if d.IsError() {
			return true
		}
	}
	return false
}
