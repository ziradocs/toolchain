// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"fmt"
)

// discriminator es la forma mínima para leer el campo "type" (BaseNode.Type)
// de un fragmento JSON de Element sin comprometerse a decodificarlo entero.
type discriminator struct {
	Type NodeType `json:"type"`
}

// DecodeElement decodifica un único fragmento JSON de ast.Element,
// despachando por su discriminador ("type", BaseNode.Type) al struct
// concreto correspondiente. Es la contraparte de decodificación del
// contrato JSON —hoy solo hay encoder (json.Marshal vía la interfaz
// Element/Node)— necesaria porque encoding/json no puede deserializar una
// interfaz sellada (Element.element() no exportado) sin ayuda: no hay forma
// de que encoding/json sepa a qué struct concreto instanciar un
// []Element sin este paso explícito.
//
// Usado por ContentBlock.UnmarshalJSON y ColumnElement.UnmarshalJSON —los
// dos únicos sitios con un campo []Element (nodes.go: ContentBlock.Elements,
// ColumnElement.Elements)— y por DecodeAST para reconstruir un *AST completo
// desde JSON (p. ej. la salida de un filtro externo del --filter de #240;
// ver ast/filter.go para las garantías de seguridad de ESE caller
// específico — este decoder es de propósito general y no las impone).
func DecodeElement(raw json.RawMessage) (Element, error) {
	var d discriminator
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("decoding element discriminator: %w", err)
	}

	var target Element
	switch d.Type {
	case NodeTypeText:
		target = &TextElement{}
	case NodeTypePoints:
		target = &PointsElement{}
	case NodeTypeCode:
		target = &CodeElement{}
	case NodeTypeImage:
		target = &ImageElement{}
	case NodeTypeDirective:
		target = &DirectiveNode{}
	case NodeTypeTable:
		target = &TableElement{}
	case NodeTypeSpecialBlock:
		target = &SpecialBlockElement{}
	case NodeTypeCodeGroup:
		target = &CodeGroupElement{}
	case NodeTypeMermaid:
		target = &MermaidElement{}
	case NodeTypePlantUML:
		target = &PlantUMLElement{}
	case NodeTypeChart:
		target = &ChartElement{}
	case NodeTypeMap:
		target = &MapElement{}
	case NodeTypeQuote:
		target = &QuoteElement{}
	case NodeTypeChecklist:
		target = &ChecklistElement{}
	case NodeTypeGrid:
		target = &GridElement{}
	case NodeTypeColumn:
		target = &ColumnElement{}
	case NodeTypeMath:
		target = &MathElement{}
	default:
		return nil, fmt.Errorf("unknown element discriminator %q", d.Type)
	}

	// target ya es un puntero (p. ej. *TextElement); json.Unmarshal necesita
	// el puntero al puntero solo si target fuera una interfaz vacía — acá
	// target es concreto, así que se decodifica directo sobre él.
	if err := json.Unmarshal(raw, target); err != nil {
		return nil, fmt.Errorf("decoding %s element: %w", d.Type, err)
	}
	return target, nil
}

// contentBlockAlias evita la recursión infinita de UnmarshalJSON: mismo
// layout que ContentBlock pero con Elements como []json.RawMessage — un
// alias de tipo (`type X ContentBlock`) NO hereda los métodos de ContentBlock
// (UnmarshalJSON), así que json.Unmarshal usa la reflexión por defecto sobre
// él en vez de volver a llamarse a sí mismo.
type contentBlockAlias struct {
	BaseNode
	BlockType            string                            `json:"blockType,omitempty"`
	Title                string                            `json:"title,omitempty"`
	TitleHTML            string                            `json:"titleHTML,omitempty"`
	Heading              string                            `json:"heading,omitempty"`
	HeadingHTML          string                            `json:"headingHTML,omitempty"`
	Subtitle             string                            `json:"subtitle,omitempty"`
	SubtitleHTML         string                            `json:"subtitleHTML,omitempty"`
	Logo                 string                            `json:"logo,omitempty"`
	Elements             []json.RawMessage                 `json:"elements"`
	HeaderFooterOverride *ContentBlockHeaderFooterOverride `json:"header_footer_override,omitempty"`
}

// UnmarshalJSON decodifica un ContentBlock, despachando polimórficamente su
// campo Elements vía DecodeElement (nodes.go:150 es uno de los 2 únicos
// sitios []Element del AST).
func (c *ContentBlock) UnmarshalJSON(data []byte) error {
	var alias contentBlockAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("decoding content block: %w", err)
	}

	c.BaseNode = alias.BaseNode
	c.BlockType = alias.BlockType
	c.Title = alias.Title
	c.TitleHTML = alias.TitleHTML
	c.Heading = alias.Heading
	c.HeadingHTML = alias.HeadingHTML
	c.Subtitle = alias.Subtitle
	c.SubtitleHTML = alias.SubtitleHTML
	c.Logo = alias.Logo
	c.HeaderFooterOverride = alias.HeaderFooterOverride

	c.Elements = make([]Element, 0, len(alias.Elements))
	for _, raw := range alias.Elements {
		elem, err := DecodeElement(raw)
		if err != nil {
			return fmt.Errorf("decoding content block elements: %w", err)
		}
		c.Elements = append(c.Elements, elem)
	}
	return nil
}

// columnElementAlias — mismo patrón que contentBlockAlias, para el segundo
// (y único otro) sitio []Element: ColumnElement.Elements (nodes.go:564).
type columnElementAlias struct {
	BaseNode
	Content     string            `json:"content"`
	ContentHTML string            `json:"contentHTML,omitempty"`
	Elements    []json.RawMessage `json:"elements,omitempty"`
}

func (c *ColumnElement) UnmarshalJSON(data []byte) error {
	var alias columnElementAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("decoding column element: %w", err)
	}

	c.BaseNode = alias.BaseNode
	c.Content = alias.Content
	c.ContentHTML = alias.ContentHTML

	c.Elements = make([]Element, 0, len(alias.Elements))
	for _, raw := range alias.Elements {
		elem, err := DecodeElement(raw)
		if err != nil {
			return fmt.Errorf("decoding column elements: %w", err)
		}
		c.Elements = append(c.Elements, elem)
	}
	return nil
}

// DecodeAST decodifica un *AST completo desde JSON. encoding/json invoca
// automáticamente ContentBlock.UnmarshalJSON para cada elemento de
// AST.ContentBlocks (es un []ContentBlock, no []Element, así que el propio
// AST no necesita su propio UnmarshalJSON custom); FrontMatterNode no es
// polimórfico (Variables es un map plano) y se decodifica con reflexión
// estándar.
func DecodeAST(data []byte) (*AST, error) {
	var doc AST
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decoding AST: %w", err)
	}
	return &doc, nil
}

// element() no aplica acá — element() es el marcador de Element, y ColumnElement
// ya lo implementa en nodes.go; este archivo solo agrega (de)serialización.
var _ Element = (*ColumnElement)(nil)
