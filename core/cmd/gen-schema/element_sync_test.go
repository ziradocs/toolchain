// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package main

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

// TestElementTypesCoversAllImplementers cubre issue #61: `elementTypes` (la lista
// hardcodeada que gen-schema usa para construir la unión discriminada del schema
// y, vía el schema, la unión TS) debe cubrir EXACTAMENTE los tipos que implementan
// la interfaz ast.Element (identificados por su método marcador `element()`).
//
// Go no permite enumerar los implementadores de una interfaz en runtime, así que
// parseamos el código fuente del paquete ast. Si alguien agrega un TipoElemento
// con `func (X) element() {}` pero olvida registrarlo en `elementTypes`, este
// test falla — cosa que el CI de schema-drift por sí solo NO detectaría
// (regenera con la misma lista incompleta ⇒ sin diff ⇒ verde).
func TestElementTypesCoversAllImplementers(t *testing.T) {
	// CWD del test = core/cmd/gen-schema/ ; el paquete ast está en ../../ast.
	astDir := filepath.Join("..", "..", "ast")

	implementers, err := findElementImplementers(astDir)
	if err != nil {
		t.Fatalf("findElementImplementers(%q): %v", astDir, err)
	}
	if len(implementers) == 0 {
		t.Fatalf("no se encontró ningún implementador de element() en %q; ¿cambió la ruta o el nombre del método marcador?", astDir)
	}

	registered := make(map[string]bool, len(elementTypes))
	for _, et := range elementTypes {
		registered[et.name] = true
	}

	var missing []string
	for name := range implementers {
		if !registered[name] {
			missing = append(missing, name)
		}
	}
	var extra []string
	for name := range registered {
		if !implementers[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) > 0 {
		t.Errorf("tipos que implementan element() pero faltan en elementTypes (gen-schema/main.go): %v\n"+
			"→ agregalos a `elementTypes` (y se propagan al schema y a la unión TS)", missing)
	}
	if len(extra) > 0 {
		t.Errorf("entradas en elementTypes que ya no implementan element(): %v\n"+
			"→ quitalas de `elementTypes` o restaurá el método element()", extra)
	}
}

// findElementImplementers parsea los .go (no _test.go) de dir y devuelve el set
// de nombres de tipo cuyo receiver define el método marcador `element()`.
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

// receiverTypeName extrae el nombre del tipo receiver, manejando tanto value
// receiver (`X`) como pointer receiver (`*X`).
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
