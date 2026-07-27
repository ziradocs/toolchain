// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

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

// excludedFromElementCoverage documenta, tipo por tipo, por qué un
// implementador de ast.Element (el ast del DSL, en go.ziradocs.com/core/v2/ast
// — no el go/ast de este archivo) deliberadamente no tiene un case propio en
// el switch principal de PrepareTemplateDataWithRenderMode (converter.go).
var excludedFromElementCoverage = map[string]string{
	// Issue #38: descubiertos por este mismo guard, no estaban en el alcance
	// original de #35/#37.
	"PlantUMLElement": "issue #38 — sin case en converter.go/template/base.go/pptx.go",
	"MathElement":     "issue #38 — sin case en converter.go/template/base.go/pptx.go",
}

// TestConverterCoversAllElementImplementers cubre issue #35: el switch
// principal de PrepareTemplateDataWithRenderMode (converter.go) debe tener un
// case para cada tipo que implementa ast.Element (identificado por su método
// marcador `element()`), salvo los documentados en
// excludedFromElementCoverage arriba.
//
// Sin este test, un tipo nuevo en core (o uno que se agregó y se olvidó
// registrar) cae al elementData casi vacío que arma el bucle exterior (solo
// Type/ElementID/SlideIndex) y desaparece en template/base.go, que tampoco
// tiene un {{else}} — sin ningún warning ni error. Pasó eso con
// *ast.MediaElement (issue #21) y, se descubrió al escribir este guard,
// también con *ast.PlantUMLElement y *ast.MathElement (issue #38): core SÍ
// tiene este guard (core/renderer/element_coverage_test.go, issue #82) y
// nunca perdió ninguno de los tres; slidelang no lo tenía.
func TestConverterCoversAllElementImplementers(t *testing.T) {
	implementers, err := findElementImplementers(filepath.Join("..", "..", "..", "..", "core", "ast"))
	if err != nil {
		t.Fatalf("findElementImplementers: %v", err)
	}
	if len(implementers) == 0 {
		t.Fatal("no se encontró ningún implementador de element() en ../../../../core/ast; ¿cambió la ruta o el nombre del método marcador?")
	}

	converterCases, err := findSwitchCaseTypes("converter.go", "PrepareTemplateDataWithRenderMode")
	if err != nil {
		t.Fatalf("findSwitchCaseTypes(converter.go, PrepareTemplateDataWithRenderMode): %v", err)
	}

	var missing []string
	for name := range implementers {
		if excludedFromElementCoverage[name] != "" {
			continue
		}
		if !converterCases[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("PrepareTemplateDataWithRenderMode no tiene case para: %v\n"+
			"→ agregá un case, o documentá la exclusión en excludedFromElementCoverage (element_coverage_test.go) con el motivo", missing)
	}

	var stale []string
	for name := range converterCases {
		if !implementers[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("PrepareTemplateDataWithRenderMode tiene case(s) para tipos que ya no implementan element(): %v", stale)
	}
}

// findElementImplementers parsea los .go (no _test.go) de dir y devuelve el
// set de nombres de tipo cuyo receiver define el método marcador `element()`
// — mismo patrón que core/renderer/element_coverage_test.go y
// core/cmd/gen-schema/element_sync_test.go (issue #61), duplicado acá porque
// esos viven en módulos/paquetes distintos y no son importables desde acá.
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

// findSwitchCaseTypes parsea file (relativo a este paquete) en busca de la
// función funcName, y devuelve el set de nombres de tipo `*ast.X` cubiertos
// por CUALQUIER type switch (`switch x := y.(type)`) dentro de su cuerpo —
// no solo el primero, porque PrepareTemplateDataWithRenderMode tiene un
// segundo switch más abajo (el pre-render offline de mermaid/chart/map, que
// es un subconjunto del principal). Unir ambos es seguro: nunca agrega un
// tipo que el switch principal no cubra ya.
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
			return true
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
