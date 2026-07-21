// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"testing"

	"go.ziradocs.com/core/diagnostics"
)

// TestWalkVisitsEveryReachableNode construye un AST con anidamiento real en
// los tres contenedores conocidos (Grid→Column→Elements anidados, Points con
// sub-points a 2 niveles, Checklist con sub-items a 2 niveles) y verifica que
// Walk visita exactamente el conjunto esperado de nodos, en orden de
// documento. A diferencia de element_coverage_test.go (renderer) —que
// verifica "todo tipo Element tiene un case"— esto verifica "todo nodo
// alcanzable se visita", que es la propiedad real que Walk debe garantizar:
// walkElement ya visita cada Element incondicionalmente (la cobertura de
// TIPOS es trivial por construcción); lo frágil es el DESCENSO a contenedores
// anidados, que solo un test de comportamiento como este detecta si se
// olvida al agregar un contenedor nuevo.
func TestWalkVisitsEveryReachableNode(t *testing.T) {
	pos := diagnostics.Position{Line: 1, Column: 1}

	grid := NewGridElement(pos)
	col1 := NewColumnElement(pos, "")
	col1.Elements = append(col1.Elements, NewTextElement(pos, "en columna 1"))
	col2 := NewColumnElement(pos, "")
	col2.Elements = append(col2.Elements, NewImageElement(pos, "x.png", "alt"))
	grid.Columns = append(grid.Columns, *col1, *col2)

	points := NewPointsElement(pos)
	leaf := NewPointItem(pos, "nieto")
	child := NewPointItem(pos, "hijo")
	child.SubPoints = append(child.SubPoints, *leaf)
	points.Items = append(points.Items, *child)

	checklist := NewChecklistElement(pos)
	leafItem := NewChecklistItem(pos, "nieto", false)
	childItem := NewChecklistItem(pos, "hijo", false)
	childItem.SubItems = append(childItem.SubItems, *leafItem)
	checklist.Items = append(checklist.Items, *childItem)

	block := NewContentBlock(pos, "content")
	block.Elements = append(block.Elements,
		grid,
		points,
		checklist,
		NewTableElement(pos), // leaf: no debe agregar visitas extra
	)

	doc := NewAST(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	var visited []NodeType
	err := Walk(doc, func(n Node) error {
		visited = append(visited, n.GetType())
		return nil
	})
	if err != nil {
		t.Fatalf("Walk devolvió error inesperado: %v", err)
	}

	want := []NodeType{
		NodeTypePresentation,  // doc
		NodeTypeContentBlock,  // block
		NodeTypeGrid,          // grid
		NodeTypeColumn,        // col1
		NodeTypeText,          // texto en col1
		NodeTypeColumn,        // col2
		NodeTypeImage,         // imagen en col2
		NodeTypePoints,        // points
		NodeTypePointItem,     // child
		NodeTypePointItem,     // leaf (nieto)
		NodeTypeChecklist,     // checklist
		NodeTypeChecklistItem, // childItem
		NodeTypeChecklistItem, // leafItem (nieto)
		NodeTypeTable,         // tabla leaf
	}

	if len(visited) != len(want) {
		t.Fatalf("Walk visitó %d nodos, esperaba %d.\nVisitados: %v\nEsperados: %v", len(visited), len(want), visited, want)
	}
	for i, wantType := range want {
		if visited[i] != wantType {
			t.Errorf("posición %d: visitado %q, esperaba %q\nVisitados: %v", i, visited[i], wantType, visited)
		}
	}
}

// TestWalkStopsOnError verifica que un error del visitor detiene el
// recorrido y se propaga — la etapa de transform de #240 depende de esto
// para abortar limpio si un pase built-in falla a mitad de documento.
func TestWalkStopsOnError(t *testing.T) {
	pos := diagnostics.Position{Line: 1, Column: 1}
	block := NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, NewTextElement(pos, "a"), NewTextElement(pos, "b"))
	doc := NewAST(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	sentinel := errTestSentinel{}
	visitCount := 0
	err := Walk(doc, func(n Node) error {
		visitCount++
		if n.GetType() == NodeTypeText {
			return sentinel
		}
		return nil
	})
	if err != sentinel {
		t.Fatalf("Walk devolvió %v, esperaba el sentinel del visitor", err)
	}
	// doc + block + primer TextElement = 3 visitas antes de abortar.
	if visitCount != 3 {
		t.Fatalf("Walk visitó %d nodos antes de abortar, esperaba 3", visitCount)
	}
}

type errTestSentinel struct{}

func (errTestSentinel) Error() string { return "sentinel" }
