// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import "go.ziradocs.com/core/v2/diagnostics"

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
//
// 2.1.0 (issue #22): TextElement.Level (additive, omitempty) exposes the
// heading level as a semantic field, so an A11Y rulepack doesn't have to
// re-parse the rendered `<hN>` in Content.
//
// 2.1.0 (issue #20): TableElement.Cells (additive, omitempty) exposes real
// cell structure (scope, colspan, rowspan) alongside Headers/Rows, which are
// kept unchanged for compatibility.
//
// 2.1.0 (issue #21): new MediaElement (discriminator "media") for embedded
// audio/video, with Autoplay/Controls/Loop/Muted — additive, a new element
// type doesn't break any existing consumer of the contract.
//
// 2.2.0 (issues #62/#63 prerequisite): FrontMatterNode.Lang (additive,
// omitempty) exposes the document's declared language as a first-class BCP
// 47 field, so a renderer can emit a real `<html lang>` and a rulepack (e.g.
// A11Y005) can read it without depending on the author having written it
// into the free-form Variables map.
//
// 2.4.0 (issue #63): new LangRun type (Text, Lang) plus a LangRuns
// ([]LangRun, additive, omitempty) field on TextElement, PointItem,
// ChecklistItem, QuoteElement and — after that issue's code review, finding
// #9 — SpecialBlockElement, GridElement and ColumnElement too, since each of
// those also carries its own loose Content prose that PopulateLangRuns was
// skipping. Exposes [texto]{lang=xx} inline spans as structured runs, so a
// rulepack can flag a passage marked in a different language than
// FrontMatter.Lang without re-parsing rendered HTML. Derived fresh from
// Content on every build (renderer.PopulateLangRuns), same
// re-derive-never-trust posture as the *HTML fields, but NOT cleared by
// ast.ClearRenderedHTML — see that field's own doc comment for why.
//
// (Both halves are labelled 2.4.0 on purpose — issue #91: the first half used
// to say 2.3.0, but 38a06dc landed after the core/v2.3.0 tag and is first
// contained in core/v2.4.0, which is also what this file's own SchemaVersion
// constant said at the time.)
//
// 2.5.0 (issue #100): FrontMatterNode.Numbering (additive, omitempty, tri-
// state *bool) lets a document declare `numbering: false`/`numbering: true`
// so `doclang build`'s section auto-numbering default no longer has to be a
// hardcoded `true` whenever front matter is present — see
// doclang/internal/cli/build.go.
//
// 2.6.0 (issue #115 follow-up): FrontMatterNode.TOC (*TOCConfig) and .Page
// (*PageConfig), both additive/omitempty — the parsed forms of the `toc:`
// and `page:` front matter namespaces, previously silently-ignored unknown
// keys (see llm-kit/reference/frontmatter.md). Page/PageMargins hold the
// author's length strings verbatim ("A4", "2cm"), not resolved to any
// renderer's unit — see core/util/length.go for the shared resolver.
//
// 2.7.0 (issue #92): DiscardedLangRuns ([]LangRun, additive, omitempty),
// the mirror image of 2.3.0/2.4.0's LangRuns, on the same 7 types
// (TextElement, PointItem, ChecklistItem, QuoteElement, SpecialBlockElement,
// GridElement, ColumnElement). renderer.PopulateLangRuns was silently
// discarding any [texto]{lang=xx} span whose tag failed a11y.IsValidLangTag
// — correct for LangRuns itself, but nothing else in the AST recorded that
// a discard happened, so an external rulepack (which receives the AST as
// serialized JSON, not in-process) had no way to learn that a language mark
// existed and didn't take without re-deriving it by hand. Same
// derivation/posture as LangRuns.
//
// 2.8.0 (issue #179): FrontMatterNode.Watermark (*WatermarkConfig,
// additive, omitempty) — a repeating, semi-transparent text overlay
// namespace (`watermark:`), rendered behind content on every
// slide/page. FontSize is stored verbatim like PageConfig.Size, not
// resolved to any renderer's unit.
const SchemaVersion = "2.8.0"

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
	NodeTypeMedia         NodeType = "media"          // Audio/video embebido (issue #21)
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
	FrontMatter *FrontMatterNode `json:"frontMatter,omitempty"`
	// ContentBlocks está en orden de documento; ver el doc comment de
	// ContentBlock.Elements (issue #62) para el contrato de preservación de
	// orden que aplica igual a este slice.
	ContentBlocks []ContentBlock `json:"contentBlocks"` // Bloques de contenido (slides en presentaciones, secciones en documentos)
	FilePath      string         `json:"-"`             // No se serializa
}

// NewAST crea un nuevo AST
func NewAST(pos diagnostics.Position) *AST {
	return &AST{
		BaseNode:      NewBaseNode(NodeTypePresentation, pos),
		SchemaVersion: SchemaVersion,
		ContentBlocks: make([]ContentBlock, 0),
	}
}
