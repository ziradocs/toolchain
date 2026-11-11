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
	"errors"
	"fmt"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
)

// Parse lee source como doclang: un guard de tamaño, ParseDocument (que elige
// el dialecto por el `mode:` del frontmatter, strict o flex) y, si no hubo
// errores, el transform de xref (numeración de figuras y tablas, \ref) con las
// validaciones que doclang build corre después de él. Con errores de parseo
// devuelve el AST y los diagnósticos sin transformar, como el MCP de doclang.
//
// Si el transform o esas validaciones fallan, doclang build no genera nada;
// aquí se devuelve el error junto con un AST que se puede dibujar, para que la
// vista previa no quede en blanco mientras se escribe un \ref a un label que
// todavía no existe. Con un \ref sin resolver es el documento numerado, con
// los \ref que sí resuelven y los demás como texto (ResolveRefs recorre todo
// el documento antes de fallar). Con un label duplicado, AssignNumbers corta a
// la mitad y deja numerado solo lo de antes, así que se vuelve a leer la
// fuente y se dibuja sin numerar, con cada \ref como texto.
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
	// Las dos fases de xref.Transform por separado, con sus mismos mensajes
	// de error, para saber cuál falló. Se corren aquí y no por
	// transform.RunBuiltins como el MCP de doclang porque core/transform no
	// compila con GOOS=js (process_unix.go usa SysProcAttr.Setpgid, que js no
	// tiene). Después de cada built-in, RunBuiltins revisa que no haya
	// borrado marcas de versión del AST (tipos de sublista, inicio de lista,
	// math-source, encabezados tipados), corre ast.ValidateNodeIDs y, si el
	// documento declara el contrato extendido (ast.DeclaresExtendedContract),
	// ast.ValidateTableContract. Las dos validaciones se corren igual, en
	// validateLikeBuiltins. La revisión de marcas no: xref solo reescribe
	// texto y asigna Number, así que no puede borrarlas.
	table, err := xref.AssignNumbers(doc)
	if err != nil {
		clean, _ := parser.New(log).ParseDocument(source, "")
		return clean, diags, fmt.Errorf("xref: asignando numeración: %w", err)
	}
	if err := xref.ResolveRefs(doc, table); err != nil {
		return doc, diags, fmt.Errorf("xref: %w", err)
	}
	return doc, diags, validateLikeBuiltins(doc)
}

// validateLikeBuiltins corre las validaciones que transform.RunBuiltins hace
// después de un built-in: las identidades de nodo y, si el documento declara
// el contrato extendido, el contrato de tablas.
func validateLikeBuiltins(doc *ast.AST) error {
	if issues := ast.ValidateNodeIDs(doc); len(issues) != 0 {
		return errors.New(issues[0].String())
	}
	if ast.DeclaresExtendedContract(doc) {
		return ast.ValidateTableContract(doc)
	}
	return nil
}

// Result es lo que la vista previa muestra de un documento: el HTML que se
// pudo dibujar, los diagnósticos del parseo, si el documento es válido para
// doclang build y el error, si lo hubo.
type Result struct {
	HTML        string
	Diagnostics []diagnostics.Diagnostic
	Valid       bool
	Err         error
}

// Render lee source con Parse y lo dibuja con HTML, con las opciones que
// options arma desde el AST leído (título, tema y chrome salen del
// frontmatter). Con errores de parseo, o si no hay AST, no hay HTML. Si Parse
// devuelve un error con AST (el transform de xref o sus validaciones), se
// dibuja igual y el error viaja junto con el HTML; Valid queda en false, como
// en doclang build, que con ese error no genera nada.
func Render(log util.Logger, source string, options func(*ast.AST) renderer.DocumentHTMLOptions) Result {
	doc, diags, err := Parse(log, source)
	out := Result{Diagnostics: diags, Err: err}
	if doc == nil || hasError(diags) {
		return out
	}
	out.HTML = HTML(doc, options(doc))
	out.Valid = err == nil
	return out
}

// Context es el contexto de render de doclang build para HTML: el código es
// literal (issue #487), así que una {{variable}} dentro de un bloque de
// código o de un code span queda como se escribió.
func Context() *renderer.RenderContext {
	ctx := renderer.NewDefaultRenderContext()
	ctx.LiteralCode = true
	return ctx
}

// HTML dibuja doc como un HTML autocontenido con opts y con Context, después
// de bajar los encabezados tipados como lo hace doclang antes de generar
// (renderer.LowerTypedHeadings). GenerateDocumentHTML dibuja cada
// HeadingElement por su forma legada, pero el TOC de subsecciones
// (extractSubsections, con opts.TOCDepth > 1) solo lee esa forma: sin bajar
// los encabezados, un documento con typed-headings-v1 pierde sus subsecciones
// en el TOC.
func HTML(doc *ast.AST, opts renderer.DocumentHTMLOptions) string {
	return renderer.GenerateDocumentHTML(renderer.LowerTypedHeadings(doc), opts, Context())
}

// DefaultTOCDepth es la profundidad del TOC de doclang build cuando el
// frontmatter no declara toc.depth (pipeline.Generate).
const DefaultTOCDepth = 3

// TOCOptions dice si la vista previa lleva TOC y hasta qué nivel, con la regla
// de doclang build sin --toc (pipeline.ResolveTOCAndNumbering y la
// profundidad de pipeline.Generate): con frontmatter, el toc.enabled que
// declare o, sin él, prendido; sin frontmatter, apagado. La profundidad es el
// toc.depth del frontmatter o DefaultTOCDepth.
func TOCOptions(doc *ast.AST) (enabled bool, depth int) {
	depth = DefaultTOCDepth
	fm := doc.FrontMatter
	if fm == nil {
		return false, depth
	}
	enabled = true
	if fm.TOC != nil {
		if fm.TOC.Enabled != nil {
			enabled = *fm.TOC.Enabled
		}
		if fm.TOC.Depth != nil {
			depth = *fm.TOC.Depth
		}
	}
	return enabled, depth
}

func hasError(diags []diagnostics.Diagnostic) bool {
	for _, d := range diags {
		if d.IsError() {
			return true
		}
	}
	return false
}
