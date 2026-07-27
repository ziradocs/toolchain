// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

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
// markdown.go's renderElement ni en docx.go's renderElement.
var excludedFromElementCoverage = map[string]string{
	"ColumnElement": "sub-elemento de GridElement.Columns, no aparece directamente en block.Elements/section.Elements — ambos generators lo consumen vía column.Content dentro del case de GridElement, no por su propio case",
	"DirectiveNode": "doclang no filtra @directivas antes del pipeline de render (a diferencia de slidelang, que las extrae vía extractPresenterNotes) ni tiene un concepto establecido de qué debería hacer una directiva en un documento — cae al warning genérico del default hasta que se decida su tratamiento (issue de seguimiento pendiente)",

	// Los siguientes 4 tipos faltan HOY solo en markdown.go (docx.go los
	// cubre) — pero como este guard es por-generador (ver checkCoverage) se
	// documentan acá para que la corrida de markdown pase; docx.go no los
	// necesita en excludedFromElementCoverage porque sí tiene sus cases.
	"PlantUMLElement":  "issue de seguimiento — markdown.go no tiene equivalente para diagramas PlantUML (docx.go sí lo cubre)",
	"MapElement":       "issue de seguimiento — markdown.go no tiene equivalente para mapas interactivos (docx.go sí lo cubre)",
	"CodeGroupElement": "issue de seguimiento — markdown.go no tiene equivalente para grupos de bloques de código con tabs (docx.go sí lo cubre)",
	"MathElement":      "issue de seguimiento — markdown.go no tiene equivalente para expresiones matemáticas (docx.go sí lo cubre)",
}

// TestGeneratorsCoverAllElementImplementers cubre issue #35: tanto
// markdown.go's renderElement como docx.go's renderElement deben tener un
// case para cada tipo que implementa ast.Element (identificado por su método
// marcador `element()`), salvo los documentados en
// excludedFromElementCoverage arriba.
//
// Sin este test, un tipo nuevo en core (o uno que se agregó y se olvidó
// registrar en alguno de los dos switches) cae silenciosamente al `default:`
// de cada función — en docx.go, hasta hace poco, ni siquiera con un mensaje
// de warning legible (ver el fix de logger.Warn en este mismo commit). Pasó
// exactamente eso con *ast.MediaElement (issue #21): core/renderer lo cubrió
// el mismo día porque core SÍ tiene este guard
// (core/renderer/element_coverage_test.go, issue #82); doclang no, y el
// gap sobrevivió sin detección hasta ahora.
func TestGeneratorsCoverAllElementImplementers(t *testing.T) {
	implementers, err := findElementImplementers(filepath.Join("..", "..", "..", "core", "ast"))
	if err != nil {
		t.Fatalf("findElementImplementers: %v", err)
	}
	if len(implementers) == 0 {
		t.Fatal("no se encontró ningún implementador de element() en ../../../core/ast; ¿cambió la ruta o el nombre del método marcador?")
	}

	markdownCases, err := findSwitchCaseTypes("markdown.go", "renderElement")
	if err != nil {
		t.Fatalf("findSwitchCaseTypes(markdown.go, renderElement): %v", err)
	}
	docxCases, err := findSwitchCaseTypes("docx.go", "renderElement")
	if err != nil {
		t.Fatalf("findSwitchCaseTypes(docx.go, renderElement): %v", err)
	}

	checkCoverage(t, "markdown.go renderElement", implementers, markdownCases)
	checkCoverage(t, "docx.go renderElement", implementers, docxCases)
}

func checkCoverage(t *testing.T, funcName string, implementers, cases map[string]bool) {
	t.Helper()

	var missing []string
	for name := range implementers {
		if excludedFromElementCoverage[name] != "" {
			continue
		}
		if !cases[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%s no tiene case para: %v\n"+
			"→ agregá un case, o documentá la exclusión en excludedFromElementCoverage (element_coverage_test.go) con el motivo", funcName, missing)
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
// por los `case` de su primer type switch (`switch x := y.(type)`).
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
