// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// textResult envuelve payload (ya serializado) como el único content block
// de un CallToolResult exitoso.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// jsonResult serializa v de forma compacta y lo envuelve como texto — todos
// los tools devuelven JSON como su content block, incluso cuando también
// devuelven un Out tipado, para no depender de que el cliente MCP lea
// structured content.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool result: %w", err)
	}
	return textResult(string(data)), nil
}

// errorResult marca el CallToolResult como fallido (IsError=true) pero
// devuelve payload igual como content — a diferencia de retornar un error de
// Go plano, esto permite que el content lleve diagnósticos estructurados en
// vez de solo un mensaje de error de una línea (ver MCP: los errores de
// ejecución de un tool se reportan en el resultado, no como error de
// protocolo).
func errorResult(v any) (*mcp.CallToolResult, error) {
	res, err := jsonResult(v)
	if err != nil {
		return nil, err
	}
	res.IsError = true
	return res, nil
}

// toolError es el payload de error genérico para tools que no tienen un
// tipo de diagnóstico más específico que devolver (p. ej. una falla operativa
// como no poder leer el directorio de temas externos).
type toolError struct {
	Error string `json:"error"`
}
