// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package xref

import (
	"fmt"

	"go.ziradocs.com/core/ast"
)

// Transform es el built-in de numeración+refs cruzadas de #239, con la
// firma que espera transform.RunBuiltins (core/transform):
// func(*ast.AST) (*ast.AST, error). Se registra ANTES que los filtros de
// terceros (--filter) en la etapa ordenada de #240, y corre DESPUÉS de la
// expansión de @include (#238) — así numera/resuelve sobre el documento ya
// fusionado, en su orden final.
//
// Compone las dos fases explícitas del paquete: AssignNumbers primero
// (completo, sobre todo el documento) y solo entonces ResolveRefs — nunca
// interleaved, por los forward references (ver doc.go del paquete).
func Transform(doc *ast.AST) (*ast.AST, error) {
	table, err := AssignNumbers(doc)
	if err != nil {
		return nil, fmt.Errorf("xref: asignando numeración: %w", err)
	}
	if err := ResolveRefs(doc, table); err != nil {
		return nil, fmt.Errorf("xref: %w", err)
	}
	return doc, nil
}
