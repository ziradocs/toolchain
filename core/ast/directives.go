// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import "go.ziradocs.com/core/diagnostics"

// DirectiveNode representa una directiva que modifica el comportamiento
type DirectiveNode struct {
	BaseNode   `tstype:",extends,required"`
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// NewDirectiveNode crea un nuevo nodo de directiva
func NewDirectiveNode(pos diagnostics.Position, name string) *DirectiveNode {
	return &DirectiveNode{
		BaseNode:   NewBaseNode(NodeTypeDirective, pos),
		Name:       name,
		Parameters: make(map[string]interface{}),
	}
}

func (d DirectiveNode) element() {}
