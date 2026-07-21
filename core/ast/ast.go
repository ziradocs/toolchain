// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import "go.ziradocs.com/core/diagnostics"

// SchemaVersion es la versión semver del contrato JSON/AST expuesto por
// --format json (ver schema/ast.schema.json y el paquete @ziradocs/ast-types).
// Política de compatibilidad: un cambio breaking en la forma serializada
// (campo eliminado/renombrado, tipo de campo cambiado, discriminador de
// elemento modificado) incrementa el componente MAJOR.
//
// 2.0.0 (issues #60, #64): ChecklistItem dejó de compartir el discriminador
// "point_item" con PointItem (ahora "checklist_item", un cambio breaking de
// discriminador); se agregaron campos "*HTML" aditivos (contentHTML, etc.)
// con la prosa pre-renderizada a HTML inline.
const SchemaVersion = "2.0.0"

// Node representa un nodo base en el AST
type Node interface {
	GetType() NodeType
	GetPosition() diagnostics.Position
	GetEndPosition() diagnostics.Position
}

type NodeType string

const (
	NodeTypePresentation NodeType = "presentation"
	NodeTypeFrontMatter  NodeType = "frontmatter"
	NodeTypeContentBlock NodeType = "content_block" // Bloque de contenido (slide en presentaciones, sección en documentos)
	NodeTypeText         NodeType = "text"
	NodeTypePoints       NodeType = "points"
	NodeTypeCode         NodeType = "code"
	NodeTypeImage        NodeType = "image"
	NodeTypePointItem    NodeType = "point_item"
	NodeTypeDirective    NodeType = "directive"
	// New advanced elements
	NodeTypeTable         NodeType = "table"
	NodeTypeSpecialBlock  NodeType = "special_block"
	NodeTypeCodeGroup     NodeType = "code_group"
	NodeTypeMermaid       NodeType = "mermaid"
	NodeTypePlantUML      NodeType = "plantuml" // Diagramas PlantUML
	NodeTypeChart         NodeType = "chart"
	NodeTypeMap           NodeType = "map"
	NodeTypeQuote         NodeType = "quote"          // Citas en bloque
	NodeTypeChecklist     NodeType = "checklist"      // Listas de tareas con checkboxes
	NodeTypeChecklistItem NodeType = "checklist_item" // Item dentro de un checklist (issue #60: antes compartía "point_item" con PointItem)
	NodeTypeGrid          NodeType = "grid"           // Grid layout container
	NodeTypeColumn        NodeType = "column"         // Column within grid layout
	NodeTypeMath          NodeType = "math"           // Ecuación/fórmula LaTeX (issue #239)
)

// BaseNode contiene campos comunes para todos los nodos
type BaseNode struct {
	Type        NodeType             `json:"type"`
	Position    diagnostics.Position `json:"position"`
	EndPosition diagnostics.Position `json:"endPosition"`
	Comments    []string             `json:"comments,omitempty"`
}

func (b BaseNode) GetType() NodeType {
	return b.Type
}

func (b BaseNode) GetPosition() diagnostics.Position {
	return b.Position
}

func (b BaseNode) GetEndPosition() diagnostics.Position {
	return b.EndPosition
}

// NewBaseNode crea un nuevo BaseNode con tipo y posición
func NewBaseNode(nodeType NodeType, pos diagnostics.Position) BaseNode {
	return BaseNode{
		Type:        nodeType,
		Position:    pos,
		EndPosition: pos, // Se actualiza luego
	}
}

// AST es el nodo raíz de un documento (presentación o documento)
type AST struct {
	BaseNode      `tstype:",extends,required"`
	SchemaVersion string `json:"schemaVersion"`
	// omitempty: doclang tolera archivos sin frontmatter (a diferencia de
	// slidelang, que lo exige), y sin omitempty este puntero nil serializaría
	// como "frontMatter": null, violando el JSON Schema (que lo declara
	// required y sin alternativa null) en cuanto doclang emita --format json.
	FrontMatter   *FrontMatterNode `json:"frontMatter,omitempty"`
	ContentBlocks []ContentBlock   `json:"contentBlocks"` // Bloques de contenido (slides en presentaciones, secciones en documentos)
	FilePath      string           `json:"-"`             // No se serializa
}

// NewAST crea un nuevo AST
func NewAST(pos diagnostics.Position) *AST {
	return &AST{
		BaseNode:      NewBaseNode(NodeTypePresentation, pos),
		SchemaVersion: SchemaVersion,
		ContentBlocks: make([]ContentBlock, 0),
	}
}

