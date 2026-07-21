// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/parser"
	"go.ziradocs.com/core/util"
)

// TestDecodeASTRoundTripAgainstExamples es la garantía de fondo del decoder
// JSON discriminado de #240 (ast/decode.go): parsea cada ejemplo real del
// repo (examples/, dos niveles arriba del módulo core), lo
// serializa a JSON, lo decodifica de vuelta con ast.DecodeAST, y verifica que
// re-serializar el resultado produce bytes IDÉNTICOS al primer encode
// (encode→decode→re-encode idempotente) — la propiedad que necesita el
// subproceso de --filter (ast/filter.go): lo que un filtro devuelve debe
// reconstruirse sin pérdida de información antes de re-entrar al pipeline.
//
// Vive en el paquete externo ast_test (no ast) para poder importar `parser`
// sin crear un ciclo de import (parser ya importa ast).
func TestDecodeASTRoundTripAgainstExamples(t *testing.T) {
	examplesDir := filepath.Join("..", "..", "examples")
	if _, err := os.Stat(examplesDir); err != nil {
		t.Fatalf("no se pudo acceder a examples/ (%s): %v — ¿corriste el test desde core/ast?", examplesDir, err)
	}

	var files []string
	err := filepath.WalkDir(examplesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if ext := filepath.Ext(path); ext == ".slidelang" || ext == ".doclang" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("recorriendo examples/ recursivamente: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no se encontró ningún .slidelang/.doclang en examples/ — ¿cambió la estructura del repo?")
	}

	tested := 0
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: no se pudo leer: %v", path, err)
			continue
		}

		p := parser.New(util.NewNoop())
		astNode, diags := p.Parse(string(content), path)
		if astNode == nil {
			// Fixtures deliberadamente inválidos (debug/test de negativos) no
			// producen AST — no son el objetivo de este round-trip, que solo
			// verifica fidelidad de (de)serialización sobre un AST real.
			continue
		}
		hasFatal := false
		for _, d := range diags {
			if d.IsError() {
				hasFatal = true
				break
			}
		}
		if hasFatal {
			continue
		}

		firstJSON, err := json.Marshal(astNode)
		if err != nil {
			t.Errorf("%s: json.Marshal inicial falló: %v", path, err)
			continue
		}

		decoded, err := ast.DecodeAST(firstJSON)
		if err != nil {
			t.Errorf("%s: ast.DecodeAST falló: %v", path, err)
			continue
		}

		secondJSON, err := json.Marshal(decoded)
		if err != nil {
			t.Errorf("%s: json.Marshal tras decode falló: %v", path, err)
			continue
		}

		if string(firstJSON) != string(secondJSON) {
			t.Errorf("%s: el round-trip encode→decode→encode NO es idempotente", path)
		}
		tested++
	}

	if tested == 0 {
		t.Fatal("ningún ejemplo produjo un AST válido para el round-trip — revisar el parser o el filtro de fixtures inválidos")
	}
	t.Logf("round-trip verificado contra %d de %d archivos en examples/", tested, len(files))
}
