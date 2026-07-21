// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/diagnostics"
)

// TestClearRenderedHTMLActuallyClears verifica la propiedad de seguridad en
// sí, no solo la cobertura del switch: puebla cada campo *HTML alcanzable
// (incluido el de CodeGroupElement.CodeBlocks, que ast.Walk NO visita por
// diseño pero que clearElementHTML sí cubre directamente) con contenido
// marcado como "hostil" y verifica que ClearRenderedHTML lo deja vacío.
func TestClearRenderedHTMLActuallyClears(t *testing.T) {
	const hostile = `<img src=x onerror=alert(1)>`
	pos := diagnostics.Position{Line: 1, Column: 1}

	text := NewTextElement(pos, "x")
	text.ContentHTML = hostile

	img := NewImageElement(pos, "x.png", "alt")
	img.AltHTML = hostile
	img.CaptionHTML = hostile

	table := NewTableElement(pos)
	table.HeadersHTML = []string{hostile}
	table.RowsHTML = [][]string{{hostile}}
	table.CaptionHTML = hostile

	codeGroup := NewCodeGroupElement(pos)
	codeGroup.CodeBlocks = append(codeGroup.CodeBlocks, CodeBlock{
		Language: "go", Label: "x", LabelHTML: hostile, Content: "x", ContentHTML: hostile,
	})

	grid := NewGridElement(pos)
	grid.ContentHTML = hostile
	col := NewColumnElement(pos, "x")
	col.ContentHTML = hostile
	nestedText := NewTextElement(pos, "y")
	nestedText.ContentHTML = hostile
	col.Elements = append(col.Elements, nestedText)
	grid.Columns = append(grid.Columns, *col)

	block := NewContentBlock(pos, "content")
	block.TitleHTML = hostile
	block.HeadingHTML = hostile
	block.SubtitleHTML = hostile
	block.Elements = append(block.Elements, text, img, table, codeGroup, grid)

	doc := NewAST(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	ClearRenderedHTML(doc)

	b := &doc.ContentBlocks[0]
	if b.TitleHTML != "" || b.HeadingHTML != "" || b.SubtitleHTML != "" {
		t.Errorf("ContentBlock *HTML no quedó vacío: %+v", b)
	}
	gotText := b.Elements[0].(*TextElement)
	if gotText.ContentHTML != "" {
		t.Errorf("TextElement.ContentHTML no quedó vacío: %q", gotText.ContentHTML)
	}
	gotImg := b.Elements[1].(*ImageElement)
	if gotImg.AltHTML != "" || gotImg.CaptionHTML != "" {
		t.Errorf("ImageElement *HTML no quedó vacío: %+v", gotImg)
	}
	gotTable := b.Elements[2].(*TableElement)
	if gotTable.HeadersHTML != nil || gotTable.RowsHTML != nil || gotTable.CaptionHTML != "" {
		t.Errorf("TableElement *HTML no quedó vacío: %+v", gotTable)
	}
	gotCodeGroup := b.Elements[3].(*CodeGroupElement)
	if gotCodeGroup.CodeBlocks[0].LabelHTML != "" || gotCodeGroup.CodeBlocks[0].ContentHTML != "" {
		t.Errorf("CodeGroupElement.CodeBlocks *HTML no quedó vacío: %+v", gotCodeGroup.CodeBlocks[0])
	}
	gotGrid := b.Elements[4].(*GridElement)
	if gotGrid.ContentHTML != "" {
		t.Errorf("GridElement.ContentHTML no quedó vacío: %q", gotGrid.ContentHTML)
	}
	gotCol := &gotGrid.Columns[0]
	if gotCol.ContentHTML != "" {
		t.Errorf("ColumnElement.ContentHTML no quedó vacío: %q", gotCol.ContentHTML)
	}
	gotNested := gotCol.Elements[0].(*TextElement)
	if gotNested.ContentHTML != "" {
		t.Errorf("TextElement anidado en Column no quedó vacío: %q", gotNested.ContentHTML)
	}
}

// excludedFromClearHTMLCoverage documenta, tipo por tipo, por qué un
// implementador de ast.Element (el ast del DSL — package ast, no el go/ast de
// este archivo de test) deliberadamente no tiene case propio en
// clearElementHTML (clear_html.go): porque no tiene NINGÚN campo "*HTML".
// Mismo patrón/motivo que excludedFromElementHTMLCoverage en
// renderer/element_coverage_test.go, pero acá la exclusión es válida incluso
// vacía (`case *X:` sin cuerpo) — lo que importa es que el nombre del tipo
// aparezca listado en algún case del switch, no que haga algo.
var excludedFromClearHTMLCoverage = map[string]string{}

// TestClearRenderedHTMLCoversAllImplementers garantiza que ClearRenderedHTML
// tiene un case (aunque sea vacío y documentado) para cada tipo que
// implementa ast.Element — la garantía de seguridad de #240 (ver el docstring
// de ClearRenderedHTML) depende de que NINGÚN campo *HTML de NINGÚN tipo
// nuevo quede sin blanquear tras decodificar la salida de un filtro externo.
// Sin este test, un ast.Element nuevo con su propio campo *HTML caería en
// silencio al no tener case, y ese HTML no sanitizado sobreviviría intacto —
// mismo patrón de bug que TestPopulateAndRenderElementHTMLCoverAllImplementers
// (renderer/element_coverage_test.go) previene para el lado de renderizado.
func TestClearRenderedHTMLCoversAllImplementers(t *testing.T) {
	implementers, err := findLocalElementImplementers(".")
	if err != nil {
		t.Fatalf("findLocalElementImplementers: %v", err)
	}
	if len(implementers) == 0 {
		t.Fatal("no se encontró ningún implementador de element() en el paquete ast; ¿cambió el nombre del método marcador?")
	}

	cases, err := findLocalSwitchCaseTypes("clear_html.go", "clearElementHTML")
	if err != nil {
		t.Fatalf("findLocalSwitchCaseTypes(clear_html.go, clearElementHTML): %v", err)
	}

	var missing []string
	for name := range implementers {
		if excludedFromClearHTMLCoverage[name] != "" {
			continue
		}
		if !cases[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("clearElementHTML no tiene case para: %v\n"+
			"→ agregá un case (aunque esté vacío si el tipo no tiene campos *HTML), o documentá la exclusión en excludedFromClearHTMLCoverage", missing)
	}

	var stale []string
	for name := range cases {
		if !implementers[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("clearElementHTML tiene case(s) para tipos que ya no implementan element(): %v", stale)
	}
}

// findLocalElementImplementers — mismo algoritmo que findElementImplementers
// en renderer/element_coverage_test.go y cmd/gen-schema/element_sync_test.go,
// duplicado acá porque cada uno vive en un paquete Go distinto (tercera
// copia del mismo patrón ya tolerado dos veces en el repo).
func findLocalElementImplementers(dir string) (map[string]bool, error) {
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
			if name := localReceiverTypeName(fn.Recv.List[0].Type); name != "" {
				found[name] = true
			}
		}
	}
	return found, nil
}

func localReceiverTypeName(expr ast.Expr) string {
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

// findLocalSwitchCaseTypes parsea file (dentro del propio paquete ast, así
// que los case son `*X`, no `*ast.X` como en renderer/element_coverage_test.go)
// en busca de funcName y devuelve el set de nombres de tipo cubiertos por los
// case de su primer type switch. Soporta case combinados (`case *A, *B:`).
func findLocalSwitchCaseTypes(file, funcName string) (map[string]bool, error) {
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
					if name := localCaseTypeName(expr); name != "" {
						found[name] = true
					}
				}
			}
			return false
		})
	}
	return found, nil
}

// localCaseTypeName extrae "X" de una expresión de case `*X` (sin
// calificador de paquete, a diferencia de caseTypeName en
// renderer/element_coverage_test.go, porque este test vive dentro del propio
// paquete ast).
func localCaseTypeName(expr ast.Expr) string {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return ""
	}
	if id, ok := star.X.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
