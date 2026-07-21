// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package xref

import (
	"fmt"

	"go.ziradocs.com/core/ast"
)

// AssignNumbers es la Fase A: recorre doc en orden de documento (ast.Walk),
// asigna Number a cada ImageElement/TableElement/MathElement con Label != ""
// usando contadores SEPARADOS por tipo (una figura, una tabla y una ecuación
// pueden compartir el mismo número sin colisionar — "Figura 1", "Tabla 1" y
// "Ecuación 1" son entidades distintas), y construye la Table label→Entry
// para que ResolveRefs (Fase B) la use. Falla si dos nodos declaran el mismo
// label (ambigüedad real: un \ref a ese label no podría saber cuál de los
// dos referencia).
func AssignNumbers(doc *ast.AST) (Table, error) {
	table := Table{}
	figureCount := 0
	tableCount := 0
	equationCount := 0

	// assign devuelve el número asignado (0 si label está vacío: nada que
	// numerar) — nunca depende de un re-lookup en table para no confiar en
	// el valor-cero implícito de un mapa como señal de "no numerado".
	assign := func(label string, kind Kind, counter *int) (int, error) {
		if label == "" {
			return 0, nil
		}
		if existing, dup := table[label]; dup {
			return 0, fmt.Errorf("label %q duplicado: ya asignado a %s %d", label, existing.Kind, existing.Number)
		}
		*counter++
		table[label] = Entry{Kind: kind, Number: *counter, AnchorID: AnchorID(label)}
		return *counter, nil
	}

	err := ast.Walk(doc, func(n ast.Node) error {
		switch v := n.(type) {
		case *ast.ImageElement:
			number, err := assign(v.Label, KindFigure, &figureCount)
			if err != nil {
				return err
			}
			v.Number = number

		case *ast.TableElement:
			number, err := assign(v.Label, KindTable, &tableCount)
			if err != nil {
				return err
			}
			v.Number = number

		case *ast.MathElement:
			number, err := assign(v.Label, KindEquation, &equationCount)
			if err != nil {
				return err
			}
			v.Number = number
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return table, nil
}
