// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// excludedFromElementHTMLCoverage documenta, tipo por tipo, por qué un
// implementador de ast.Element (el ast del DSL, no go/ast) deliberadamente
// no tiene un case propio en RenderElementToHTML (html.go) ni en
// populateElementHTML (populate_inline_html.go).
var excludedFromElementHTMLCoverage = map[string]string{
	"DirectiveNode": "presenter notes/directivas (@notes, etc.) — no llevan contenido de prosa renderizable; se filtran explícitamente antes de llegar al pipeline de HTML (ver extractPresenterNotes en slidelang/internal/generator/data/converter.go)",
	"ColumnElement": "sub-elemento de GridElement.Columns, no aparece directamente en block.Elements — se renderiza con su propio helper dedicado (renderGridElement en html.go, populateColumnHTML en populate_inline_html.go)",
}

// TestPopulateAndRenderElementHTMLCoverAllImplementers cubre issue #82: tanto
// RenderElementToHTML (html.go, usado por doclang) como populateElementHTML
// (populate_inline_html.go, usado por --format json de slidelang) deben tener
// un case para cada tipo que implementa ast.Element (identificado por su
// método marcador `element()`), salvo los documentados en
// excludedFromElementHTMLCoverage arriba.
//
// Sin este test, un tipo nuevo (o uno que se agregó y se olvidó registrar en
// alguno de los dos switches) cae silenciosamente al `default:` de cada
// función — un comentario HTML invisible en RenderElementToHTML, un no-op
// total en populateElementHTML — sin ningún error ni log. Pasó exactamente
// eso con *ast.CodeGroupElement durante la revisión de la PR que agregó
// populate_inline_html.go, antes de que existiera este test.
func TestPopulateAndRenderElementHTMLCoverAllImplementers(t *testing.T) {
	implementers, err := findElementImplementers(filepath.Join("..", "ast"))
	if err != nil {
		t.Fatalf("findElementImplementers: %v", err)
	}
	if len(implementers) == 0 {
		t.Fatal("no se encontró ningún implementador de element() en ../ast; ¿cambió la ruta o el nombre del método marcador?")
	}

	renderCases, err := findSwitchCaseTypes("html.go", "RenderElementToHTML")
	if err != nil {
		t.Fatalf("findSwitchCaseTypes(html.go, RenderElementToHTML): %v", err)
	}
	populateCases, err := findSwitchCaseTypes("populate_inline_html.go", "populateElementHTML")
	if err != nil {
		t.Fatalf("findSwitchCaseTypes(populate_inline_html.go, populateElementHTML): %v", err)
	}

	checkCoverage(t, "RenderElementToHTML", implementers, renderCases)
	checkCoverage(t, "populateElementHTML", implementers, populateCases)
}

func checkCoverage(t *testing.T, funcName string, implementers, cases map[string]bool) {
	t.Helper()

	var missing []string
	for name := range implementers {
		if excludedFromElementHTMLCoverage[name] != "" {
			continue
		}
		if !cases[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%s no tiene case para: %v\n"+
			"→ agregá un case, o documentá la exclusión en excludedFromElementHTMLCoverage (element_coverage_test.go) con el motivo", funcName, missing)
	}

	var stale []string
	for name := range cases {
		if !implementers[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("%s tiene case(s) para tipos que ya no implementan element(): %v", funcName, stale)
	}
}

// findElementImplementers parsea los .go (no _test.go) de dir y devuelve el
// set de nombres de tipo cuyo receiver define el método marcador `element()`
// — mismo patrón que cmd/gen-schema/element_sync_test.go (issue #61),
// duplicado acá porque ese vive en un paquete `main` distinto y no es
// importable.
func findElementImplementers(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	found := make(map[string]bool)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "element" || len(fn.Recv.List) == 0 {
				continue
			}
			if name := receiverTypeName(fn.Recv.List[0].Type); name != "" {
				found[name] = true
			}
		}
	}
	return found, nil
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

// findSwitchCaseTypes parsea file (relativo al paquete renderer) en busca de
// la función funcName, y devuelve el set de nombres de tipo `*ast.X`
// cubiertos por los `case` de su primer type switch (`switch x := y.(type)`).
func findSwitchCaseTypes(file, funcName string) (map[string]bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return nil, err
	}

	found := make(map[string]bool)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
			continue
		}

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSwitchStmt)
			if !ok {
				return true
			}
			for _, stmt := range ts.Body.List {
				clause, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range clause.List {
					if name := caseTypeName(expr); name != "" {
						found[name] = true
					}
				}
			}
			return false
		})
	}
	return found, nil
}

// caseTypeName extrae "X" de una expresión de case `*ast.X` (paquete "ast",
// el DSL de este repo — no el go/ast de este archivo de test).
func caseTypeName(expr ast.Expr) string {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return ""
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "ast" {
		return ""
	}
	return sel.Sel.Name
}
