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
)

// excludedFromWalkDescentCoverage documenta, tipo por tipo, por qué un
// implementador de Element que SÍ tiene un campo slice recorrible
// deliberadamente no lleva su propio case en walkElement.
var excludedFromWalkDescentCoverage = map[string]string{
	"CodeGroupElement": "CodeBlocks []CodeBlock: CodeBlock es un struct de valor sin BaseNode/posición — no implementa Node, así que no hay nada que pasarle a visit (decisión B, ver el doc comment de Walk)",
	"ColumnElement":    "sub-elemento de GridElement.Columns: nunca aparece directo en ContentBlock.Elements, se recorre por walkColumn desde el case *GridElement",
}

// TestWalkCoversAllElementImplementers es el test que el doc comment de Walk
// dice que guarda su cobertura de tipos. Issue #79: la cita existía desde
// antes que el test, así que Walk estuvo sin esa red durante todo ese tiempo.
//
// A diferencia de sus hermanos (TestClearRenderedHTMLCoversAllImplementers acá
// mismo, renderer/element_coverage_test.go, cmd/gen-schema/element_sync_test.go)
// acá NO se exige un case por cada implementador de Element: walkElement
// visita el elemento ANTES del switch, así que un tipo hoja (TextElement,
// CodeElement, …) ya queda cubierto sin case propio. Lo que se guarda es el
// DESCENSO: un tipo con sub-estructura recorrible debe tener case, y uno sin
// ella no debe tenerlo.
//
// Los dos lados se derivan del código, no de una lista escrita a mano:
//
//   - qué cuenta como "sub-estructura recorrible" sale de los tipos que
//     reciben las propias funciones walkX de walk.go;
//   - qué tipos la tienen sale de los campos slice declarados en nodes.go.
//
// Así, un Element nuevo con Items/Columns/Elements que se olvide de agregar a
// walkElement hace fallar este test en vez de recorrerse a medias en silencio.
func TestWalkCoversAllElementImplementers(t *testing.T) {
	implementers, err := findLocalElementImplementers(".")
	if err != nil {
		t.Fatalf("findLocalElementImplementers: %v", err)
	}
	if len(implementers) == 0 {
		t.Fatal("no se encontró ningún implementador de element() en el paquete ast; ¿cambió el nombre del método marcador?")
	}

	walkable, err := walkableTypes("walk.go")
	if err != nil {
		t.Fatalf("walkableTypes: %v", err)
	}
	// Element entra por definición: un campo []Element se recorre con
	// walkElement, que es el punto de entrada y no aparece como sub-tipo de
	// ninguna otra walkX.
	walkable["Element"] = true

	sliceFields, err := sliceFieldElemTypes(".")
	if err != nil {
		t.Fatalf("sliceFieldElemTypes: %v", err)
	}

	cases, err := findLocalSwitchCaseTypes("walk.go", "walkElement")
	if err != nil {
		t.Fatalf("findLocalSwitchCaseTypes(walk.go, walkElement): %v", err)
	}

	var missing, spurious []string
	for name := range implementers {
		descends := false
		for _, elemType := range sliceFields[name] {
			if walkable[elemType] {
				descends = true
				break
			}
		}
		if excludedFromWalkDescentCoverage[name] != "" {
			if cases[name] {
				spurious = append(spurious, name+" (documentado como excluido, pero tiene case)")
			}
			continue
		}
		switch {
		case descends && !cases[name]:
			missing = append(missing, name)
		case !descends && cases[name]:
			spurious = append(spurious, name+" (tiene case pero ningún campo slice recorrible)")
		}
	}

	sort.Strings(missing)
	sort.Strings(spurious)
	if len(missing) > 0 {
		t.Errorf("walkElement no desciende a: %v\n"+
			"→ agregá un case que recorra su sub-estructura, o documentá la exclusión en excludedFromWalkDescentCoverage (walk_coverage_test.go) con el motivo", missing)
	}
	if len(spurious) > 0 {
		t.Errorf("walkElement tiene case(s) de más: %v", spurious)
	}

	for name := range excludedFromWalkDescentCoverage {
		if !implementers[name] {
			t.Errorf("excludedFromWalkDescentCoverage menciona %q, que ya no implementa element() — borrá la entrada", name)
		}
	}
}

// walkableTypes devuelve el set de tipos que las funciones walkX de file
// reciben como primer parámetro — o sea, aquello que Walk sabe recorrer.
// Derivarlo de las firmas en vez de listarlo a mano es lo que hace que una
// walkFoo nueva extienda sola la cobertura de este test.
func walkableTypes(file string) (map[string]bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return nil, err
	}

	found := make(map[string]bool)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "walk") {
			continue
		}
		if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
			continue
		}
		if name := localReceiverTypeName(fn.Type.Params.List[0].Type); name != "" {
			found[name] = true
		}
	}
	return found, nil
}

// sliceFieldElemTypes mapea cada struct declarado en dir al tipo de elemento
// de sus campos slice (`Items []PointItem` → "PointItem").
func sliceFieldElemTypes(dir string) (map[string][]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	found := make(map[string][]string)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				return true
			}
			for _, field := range st.Fields.List {
				arr, ok := field.Type.(*ast.ArrayType)
				if !ok || arr.Len != nil {
					continue
				}
				if name := localReceiverTypeName(arr.Elt); name != "" {
					found[ts.Name.Name] = append(found[ts.Name.Name], name)
				}
			}
			return true
		})
	}
	return found, nil
}
