// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

// visitFn recibe cada Node del árbol durante un Walk, en orden de documento.
// Un error no-nil detiene el recorrido y se propaga como resultado de Walk.
// No exportado: Go permite pasar un literal de función directamente a Walk
// sin nombrar este tipo (asignabilidad estructural), así que no hace falta
// exportarlo — y si lo estuviera, tygo lo emitiría como "export type Visitor
// = any;" en ast-types (un tipo función de Go no tiene representación TS
// útil), ruido en el contrato público sin necesidad.
type visitFn func(node Node) error

// Walk recorre doc en orden de documento (mismo orden que
// renderer.PopulateInlineHTML, generalizado): ContentBlocks → Elements
// (polimórfico) → sub-estructuras anidadas que SÍ implementan Node
// (PointItem.SubPoints, ChecklistItem.SubItems) → GridElement.Columns →
// ColumnElement.Elements (polimórfico, anidado).
//
// Visita el propio doc primero, luego cada ContentBlock, luego cada Element
// (recursivamente para los que contienen sub-elementos). No visita
// FrontMatterNode (no tiene contenido de prosa recorrible ni sub-elementos;
// ver docs/architecture/json-ast-contract.md) ni CodeGroupElement.CodeBlocks
// (CodeBlock es un struct de valor sin BaseNode/posición — no implementa
// Node, y no es numerable/referenciable por decisión B).
//
// Su cobertura de tipos ast.Element se guarda con TestWalkCoversAllElementImplementers
// (walk_test.go) — el mismo patrón de introspección de código fuente que
// element_coverage_test.go (renderer) y element_sync_test.go (cmd/gen-schema):
// un ast.Element nuevo sin case acá hace fallar ese test, no cae en silencio.
func Walk(doc *AST, visit visitFn) error {
	if doc == nil {
		return nil
	}
	if err := visit(doc); err != nil {
		return err
	}
	for i := range doc.ContentBlocks {
		if err := walkContentBlock(&doc.ContentBlocks[i], visit); err != nil {
			return err
		}
	}
	return nil
}

func walkContentBlock(block *ContentBlock, visit visitFn) error {
	if err := visit(block); err != nil {
		return err
	}
	for _, elem := range block.Elements {
		if err := walkElement(elem, visit); err != nil {
			return err
		}
	}
	return nil
}

// walkElement visita element y desciende a sus sub-estructuras según el tipo
// concreto. Los tipos sin sub-estructura (TextElement, CodeElement,
// ImageElement, TableElement, SpecialBlockElement, MermaidElement,
// PlantUMLElement, ChartElement, MapElement, QuoteElement, DirectiveNode)
// solo se visitan a sí mismos — caen al default.
func walkElement(element Element, visit visitFn) error {
	if err := visit(element); err != nil {
		return err
	}
	switch elem := element.(type) {
	case *PointsElement:
		for i := range elem.Items {
			if err := walkPointItem(&elem.Items[i], visit); err != nil {
				return err
			}
		}
	case *ChecklistElement:
		for i := range elem.Items {
			if err := walkChecklistItem(&elem.Items[i], visit); err != nil {
				return err
			}
		}
	case *GridElement:
		for i := range elem.Columns {
			if err := walkColumn(&elem.Columns[i], visit); err != nil {
				return err
			}
		}
	}
	return nil
}

func walkColumn(col *ColumnElement, visit visitFn) error {
	if err := visit(col); err != nil {
		return err
	}
	for _, nested := range col.Elements {
		if err := walkElement(nested, visit); err != nil {
			return err
		}
	}
	return nil
}

func walkPointItem(item *PointItem, visit visitFn) error {
	if err := visit(item); err != nil {
		return err
	}
	for i := range item.SubPoints {
		if err := walkPointItem(&item.SubPoints[i], visit); err != nil {
			return err
		}
	}
	return nil
}

func walkChecklistItem(item *ChecklistItem, visit visitFn) error {
	if err := visit(item); err != nil {
		return err
	}
	for i := range item.SubItems {
		if err := walkChecklistItem(&item.SubItems[i], visit); err != nil {
			return err
		}
	}
	return nil
}
