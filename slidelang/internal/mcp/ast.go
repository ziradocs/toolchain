// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/util"
	"go.ziradocs.com/slidelang/internal/generator"
)

type getASTInput struct {
	Source   string `json:"source" jsonschema:"the .slidelang source content to parse"`
	FileName string `json:"fileName,omitempty" jsonschema:"optional file name, used only to label diagnostics"`
}

type getASTError struct {
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
	// Error se puebla cuando la falla no es un diagnóstico de parseo (guard
	// de parseSource, o un error de RenderASTJSON con un AST ya parseado sin
	// errores) — en esos casos Diagnostics viene vacío/sin errores y sin esto
	// el cliente no tendría forma de saber qué falló.
	Error string `json:"error,omitempty"`
}

// registerGetASTTool registra el tool "get_ast": parsea source y devuelve el
// mismo JSON que produce `slidelang build --format json`, incluyendo los
// campos *HTML pre-renderizados (issue #64), sin escribir ningún archivo —
// reusa Generator.RenderASTJSON, que es el mismo pipeline (BuildVariables +
// PopulateInlineHTML + SerializeToJSON) menos el os.WriteFile. Si el parseo
// tiene errores, no hay AST válido que devolver: el resultado se marca como
// error y lleva los diagnósticos de parseo en vez del AST.
func registerGetASTTool(server *sdkmcp.Server, logger util.Logger) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_ast",
		Description: "Parse SlideLang source content and return the serialized JSON/AST contract (same shape as `slidelang build --format json`), without writing any file to disk.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in getASTInput) (*sdkmcp.CallToolResult, any, error) {
		astNode, parseDiags, err := parseSource(logger, in.Source, in.FileName)
		if err != nil || astNode == nil || hasErrorDiagnostic(parseDiags) {
			out := getASTError{Diagnostics: parseDiags}
			if err != nil {
				out.Error = err.Error()
			}
			res, resErr := errorResult(out)
			return res, out, resErr
		}

		gen := generator.New(logger)
		astJSON, err := gen.RenderASTJSON(astNode)
		if err != nil {
			out := getASTError{Diagnostics: parseDiags, Error: err.Error()}
			res, resErr := errorResult(out)
			return res, out, resErr
		}

		// astJSON ya es JSON válido (SerializeToJSONCompact). json.RawMessage
		// implementa MarshalJSON devolviendo los bytes tal cual — el SDK lo
		// serializa directo para StructuredContent sin decodificar/re-codificar
		// el AST completo (que antes hacía un round-trip Unmarshal+Marshal
		// completo, costo proporcional al tamaño del AST, para nada).
		return textResult(string(astJSON)), json.RawMessage(astJSON), nil
	})
}
