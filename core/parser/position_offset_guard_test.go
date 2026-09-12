// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// Este archivo es el guard estructural de issue #245: toda posición del
// CUERPO que un parser escribe en el AST tiene que contar desde el ARCHIVO,
// no desde el cuerpo. La corrección depende de que TODO constructor de
// Position en core/parser y core/internal/elements pase por uno de los
// cuatro helpers que suman lineOffset — un solo sitio nuevo (o revertido)
// que llame a diagnostics.NewPosition directo vuelve a contar desde el
// cuerpo en silencio, y ningún test de comportamiento lo detecta si no
// ejercita justo esa línea. Este test lo cierra por construcción: barre el
// código fuente y falla si aparece un diagnostics.NewPosition(...) o un
// elements.ParseContext{...} fuera de la lista permitida — no importa qué
// tan nuevo o qué tan poco cubierto esté el sitio.
//
// Mismo andamiaje que TestFrontMatterOverridesCoverAllTypedFields
// (core/formatter/frontmatter_typed_fields_test.go:497-565): go/parser sobre
// el archivo en disco, sin importar el paquete como valor Go.

// positionOffsetHelperKeys son los únicos sitios de core/parser y
// core/internal/elements autorizados a llamar diagnostics.NewPosition con
// argumentos que NO son los dos literales enteros de una posición fija
// (frontmatter.go es la otra excepción, ver allowedGuardFile). Cada entrada
// es el helper que le suma lineOffset a un índice de línea del cuerpo antes
// de construir la Position — agregar un helper nuevo requiere agregarlo acá
// explícitamente, no basta con que el nombre "termine en position".
var positionOffsetHelperKeys = map[string]bool{
	"strict.go:strictBody.position":                true,
	"flex.go:FlexParser.position":                  true,
	"document_flex.go:DocumentFlexParser.position": true,
	"common.go:ParseContext.Position":              true,
}

// allowedGuardFile es el único archivo con una excepción POR ARCHIVO
// completo: el frontmatter empieza siempre en la línea 1 del archivo (el
// primer "---" no puede estar precedido por nada), así que
// FrontMatterParser ya construye posiciones absolutas — sumarles lineOffset
// las duplicaría. Ver la sección "El barrido mecánico" del plan de #245.
const allowedGuardFile = "frontmatter.go"

// guardSourceDirs son los directorios que este guard barre — los dos que el
// plan de #245 enumera como blast radius del fix.
var guardSourceDirs = []string{".", "../internal/elements"}

// funcKey identifica una función o método por "<archivo>:<Recv>.<Nombre>" (o
// "<archivo>:<Nombre>" sin receiver), para poder nombrar los helpers
// permitidos sin ambigüedad entre paquetes/tipos.
func funcKey(file string, fn *goast.FuncDecl) string {
	name := fn.Name.Name
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fmt.Sprintf("%s:%s", file, name)
	}
	recvType := fn.Recv.List[0].Type
	if star, ok := recvType.(*goast.StarExpr); ok {
		recvType = star.X
	}
	if ident, ok := recvType.(*goast.Ident); ok {
		return fmt.Sprintf("%s:%s.%s", file, ident.Name, name)
	}
	return fmt.Sprintf("%s:?.%s", file, name)
}

// isIntLiteralPosition reporta si los dos argumentos de una llamada a
// NewPosition son literales enteros (p. ej. NewPosition(1, 1),
// NewPosition(2, 1)) — la forma que usan las posiciones ya absolutas por
// definición (la raíz del AST, un error de preprocesador a nivel de
// archivo). Cubrir esta forma con una regla genérica evita una allowlist de
// una entrada por cada literal disperso en parser.go/strict.go/flex.go/
// document_strict.go/document_flex.go.
func isIntLiteralPosition(call *goast.CallExpr) bool {
	if len(call.Args) < 2 {
		return false
	}
	for _, arg := range call.Args[:2] {
		lit, ok := arg.(*goast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return false
		}
	}
	return true
}

// walkGoFiles corre visit sobre cada archivo *.go no-test de dirs,
// devolviendo el fset compartido para poder mapear posiciones legibles en
// los mensajes de error.
func walkGoFiles(t *testing.T, dirs []string, visit func(file string, fset *token.FileSet, f *goast.File)) {
	t.Helper()
	fset := token.NewFileSet()
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir(%q): %v", dir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			parsed, err := goparser.ParseFile(fset, path, nil, goparser.ParseComments)
			if err != nil {
				t.Fatalf("ParseFile(%q): %v", path, err)
			}
			visit(name, fset, parsed)
		}
	}
}

// enclosingFuncKey devuelve funcKey de la función que contiene node, o ""
// si node no está dentro de ninguna (no debería pasar para un CallExpr/
// CompositeLit real, pero evita un nil panic si algún día lo está).
func enclosingFuncKey(file string, root *goast.File, node goast.Node) string {
	var key string
	goast.Inspect(root, func(n goast.Node) bool {
		fn, ok := n.(*goast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		if fn.Body.Pos() <= node.Pos() && node.Pos() < fn.Body.End() {
			key = funcKey(file, fn)
		}
		return true
	})
	return key
}

// TestNewPositionCallsGoThroughOffsetHelpers es el guard central: todo
// diagnostics.NewPosition(...) en core/parser o core/internal/elements tiene
// que ser (a) un literal entero fijo, (b) estar en frontmatter.go, o (c)
// estar en uno de los cuatro helpers de positionOffsetHelperKeys.
func TestNewPositionCallsGoThroughOffsetHelpers(t *testing.T) {
	seenHelpers := map[string]bool{}
	totalCalls := 0

	walkGoFiles(t, guardSourceDirs, func(file string, fset *token.FileSet, f *goast.File) {
		goast.Inspect(f, func(n goast.Node) bool {
			call, ok := n.(*goast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*goast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*goast.Ident)
			if !ok || pkgIdent.Name != "diagnostics" || sel.Sel.Name != "NewPosition" {
				return true
			}
			totalCalls++

			if file == allowedGuardFile {
				return true
			}
			if isIntLiteralPosition(call) {
				return true
			}
			key := enclosingFuncKey(file, f, call)
			if positionOffsetHelperKeys[key] {
				seenHelpers[key] = true
				return true
			}
			t.Errorf(
				"%s: diagnostics.NewPosition(...) fuera de los helpers permitidos (func %q).\n"+
					"Issue #245: toda posición del cuerpo tiene que contar desde el ARCHIVO. "+
					"Usá p.position(i)/ctx.Position(i) en vez de construir la Position directo, "+
					"o si esta posición ya es absoluta por definición (p. ej. la raíz del AST), "+
					"documentalo acá agregando el sitio a positionOffsetHelperKeys con la razón.",
				fset.Position(call.Pos()), key)
			return true
		})
	})

	if totalCalls == 0 {
		t.Fatal("no se encontró ninguna llamada a diagnostics.NewPosition; ¿cambió el nombre del paquete/función?")
	}

	var missing []string
	for key := range positionOffsetHelperKeys {
		if !seenHelpers[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("positionOffsetHelperKeys menciona %s, pero no se encontró ninguna llamada a diagnostics.NewPosition ahí — ¿se renombró o se borró el helper?",
			strings.Join(missing, ", "))
	}
}

// elementsParseContextHelperKeys son los únicos sitios autorizados a
// construir un elements.ParseContext{...} — los tres métodos parseContext()
// que hilan lineOffset. Cualquier otro literal se saltaría ese hilado.
var elementsParseContextHelperKeys = map[string]bool{
	"strict.go:strictBody.parseContext":                true,
	"flex.go:FlexParser.parseContext":                  true,
	"document_flex.go:DocumentFlexParser.parseContext": true,
}

// TestParseContextLiteralsGoThroughHelpers es el guard equivalente para
// elements.ParseContext{...}: todo literal en core/parser tiene que estar
// dentro de uno de los tres métodos parseContext().
func TestParseContextLiteralsGoThroughHelpers(t *testing.T) {
	seen := map[string]bool{}

	walkGoFiles(t, []string{"."}, func(file string, fset *token.FileSet, f *goast.File) {
		goast.Inspect(f, func(n goast.Node) bool {
			lit, ok := n.(*goast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := lit.Type.(*goast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*goast.Ident)
			if !ok || pkgIdent.Name != "elements" || sel.Sel.Name != "ParseContext" {
				return true
			}

			key := enclosingFuncKey(file, f, lit)
			if elementsParseContextHelperKeys[key] {
				seen[key] = true
				return true
			}
			t.Errorf(
				"%s: elements.ParseContext{...} construido fuera de parseContext() (func %q).\n"+
					"Issue #245: un literal directo no hila LineOffset. Usá ctx := p.parseContext().",
				fset.Position(lit.Pos()), key)
			return true
		})
	})

	if len(seen) != len(elementsParseContextHelperKeys) {
		var missing []string
		for key := range elementsParseContextHelperKeys {
			if !seen[key] {
				missing = append(missing, key)
			}
		}
		sort.Strings(missing)
		t.Errorf("elementsParseContextHelperKeys menciona %s, pero no se encontró ningún elements.ParseContext{...} ahí — ¿se renombró o se borró el helper?",
			strings.Join(missing, ", "))
	}
}

// TestParseDocumentFrontMatterCommentIsFileAbsolute confirma que el doc
// comment de parseDocumentFrontMatter (document_flex.go) quedó reescrito:
// ya no describe la relatividad al cuerpo como diseño intencional (issue
// #245 lo corrigió) y menciona el archivo como el marco de referencia real.
func TestParseDocumentFrontMatterCommentIsFileAbsolute(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := goparser.ParseFile(fset, "document_flex.go", nil, goparser.ParseComments)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var doc string
	found := false
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*goast.FuncDecl)
		if !ok || fn.Name.Name != "parseDocumentFrontMatter" {
			continue
		}
		found = true
		if fn.Doc != nil {
			doc = fn.Doc.Text()
		}
	}
	if !found {
		t.Fatal("no se encontró parseDocumentFrontMatter en document_flex.go — ¿se renombró?")
	}
	if doc == "" {
		t.Fatal("parseDocumentFrontMatter no tiene doc comment")
	}
	if strings.Contains(doc, "relativas al cuerpo") {
		t.Errorf("el doc comment de parseDocumentFrontMatter todavía dice \"relativas al cuerpo\" — issue #245 corrigió justamente eso:\n%s", doc)
	}
	if !strings.Contains(doc, "archivo") && !strings.Contains(doc, "ARCHIVO") {
		t.Errorf("el doc comment de parseDocumentFrontMatter no menciona el archivo como marco de referencia:\n%s", doc)
	}
}

// TestBodyLineOffset fija el invariante de bodyLineOffset (parser.go): 0
// para un frontmatter nil, y el largo real del frontmatter en cualquier
// otro caso — no una constante asumida.
func TestBodyLineOffset(t *testing.T) {
	if got := bodyLineOffset(nil); got != 0 {
		t.Errorf("bodyLineOffset(nil) = %d, want 0", got)
	}

	fm := &ast.FrontMatterNode{}
	fm.EndPosition.Line = 4
	if got := bodyLineOffset(fm); got != 4 {
		t.Errorf("bodyLineOffset(fm con EndPosition.Line=4) = %d, want 4", got)
	}
}
