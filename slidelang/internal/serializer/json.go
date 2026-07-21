// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package serializer

import (
	"encoding/json"

	"go.ziradocs.com/core/ast"
)

type JSONSerializer struct{}

func New() *JSONSerializer {
	return &JSONSerializer{}
}

// SerializeToJSON convierte un AST a su representación JSON
func (s *JSONSerializer) SerializeToJSON(astNode *ast.AST) ([]byte, error) {
	return json.MarshalIndent(astNode, "", "  ")
}

// SerializeToJSONCompact convierte un AST a JSON compacto
func (s *JSONSerializer) SerializeToJSONCompact(astNode *ast.AST) ([]byte, error) {
	return json.Marshal(astNode)
}
