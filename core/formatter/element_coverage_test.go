// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

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

// excludedFromFormatterCoverage documenta, tipo por tipo, por qué un
// implementador de ast.Element deliberadamente no tiene case propio en los
// switches de los dos formatters.
//
// Ojo con lo que NO va acá: un tipo que el dialecto no puede representar
// (GridElement en DocLang) SÍ lleva case — uno que devuelve
// newUnsupported con un motivo concreto. Esa es justamente la diferencia
// entre un hueco declarado y uno que cae al default genérico, y es la que
// este test existe para mantener.
var excludedFromFormatterCoverage = map[string]string{
	"ColumnElement": "sub-elemento de GridElement.Columns, no aparece directo en ContentBlock.Elements — lo serializa formatStrictGrid con su propio helper",
}

// TestFormattersCoverAllElementImplementers cubre el hueco que reportó el
// issue #78: core/formatter/ era el único switch por tipo de elemento del
// repo sin guard de cobertura, y por eso *ast.MathElement se quedó sin case
// en formatDocumentElement — un .doclang con math moría en el `default:` con
// "tipo de elemento no reconocido", en vez de round-tripear.
//
// Lo que hizo que el hueco durara: el harness de round-trip sobre el corpus
// (document_roundtrip_test.go) SKIPea todo fixture cuyo formateo devuelva
// UnsupportedElementError, así que el único fixture con math
// (examples/advanced_elements_test.doclang) se saltaba entero — y encima por
// GRID, que aparece antes. Ese skip ahora está acotado a un allowlist; este
// test es la otra mitad del cierre.
func TestFormattersCoverAllElementImplementers(t *testing.T) {
	implementers, err := findElementImplementers(filepath.Join("..", "ast"))
	if err != nil {
		t.Fatalf("findElementImplementers: %v", err)
	}
	if len(implementers) == 0 {
		t.Fatal("no se encontró ningún implementador de element() en ../ast; ¿cambió la ruta o el nombre del método marcador?")
	}

	for _, tc := range []struct{ file, fn string }{
		{"strict.go", "formatStrictElement"},
		{"document.go", "formatDocumentElement"},
	} {
		cases, err := findSwitchCaseTypes(tc.file, tc.fn)
		if err != nil {
			t.Fatalf("findSwitchCaseTypes(%s, %s): %v", tc.file, tc.fn, err)
		}
		checkFormatterCoverage(t, tc.fn, implementers, cases)
	}

	for name := range excludedFromFormatterCoverage {
		if !implementers[name] {
			t.Errorf("excludedFromFormatterCoverage menciona %q, que ya no implementa element() — borrá la entrada", name)
		}
	}
}

func checkFormatterCoverage(t *testing.T, funcName string, implementers, cases map[string]bool) {
	t.Helper()

	var missing []string
	for name := range implementers {
		if excludedFromFormatterCoverage[name] != "" {
			continue
		}
		if !cases[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%s no tiene case para: %v\n"+
			"→ agregá un case (si el dialecto no puede representarlo, uno que devuelva newUnsupported con el motivo concreto), o documentá la exclusión en excludedFromFormatterCoverage (element_coverage_test.go)", funcName, missing)
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

// findElementImplementers devuelve el set de nombres de tipo cuyo receiver
// define el método marcador `element()`. Mismo patrón (y misma duplicación
// deliberada) que renderer/element_coverage_test.go, ast/clear_html_test.go y
// cmd/gen-schema/element_sync_test.go: viven en paquetes distintos y un helper
// de test no es importable entre ellos.
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

// findSwitchCaseTypes devuelve el set de tipos `*ast.X` cubiertos por los case
// del primer type switch de funcName dentro de file.
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

// caseTypeName extrae "X" de una expresión de case `*ast.X` (el paquete ast
// del DSL, no el go/ast de este archivo de test).
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
