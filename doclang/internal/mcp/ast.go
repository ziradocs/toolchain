// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/internal/generator"
)

type getASTInput struct {
	Source string `json:"source" jsonschema:"the .doclang source content to parse"`
}

type getASTError struct {
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
	// Error se puebla cuando la falla no es un diagnóstico de parseo (guard
	// de parseSource, transform fallido, o un error de RenderASTJSON con un
	// AST ya parseado sin errores) — en esos casos Diagnostics puede venir
	// vacío/sin errores y sin esto el cliente no tendría forma de saber qué
	// falló.
	Error string `json:"error,omitempty"`
}

// registerGetASTTool registra el tool "get_ast": parsea source y devuelve el
// AST serializado a JSON — build-faithful (ver parse.go): incluye la
// numeración de figuras/tablas/ecuaciones y los \ref ya resueltos, porque
// parseSource corre la misma etapa de transform que `doclang build`, no solo
// el parseo — a diferencia de la primera versión del tool equivalente en
// slidelang, que quedó parse-only por ser anterior a esa etapa (ver el
// comentario de parseSource). Sin escribir ningún archivo a disco.
func registerGetASTTool(server *sdkmcp.Server, logger util.Logger) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_ast",
		Description: "Parse DocLang source content and return the serialized JSON/AST contract, including built-in transforms (figure/table/equation numbering, resolved \\ref), without writing any file to disk.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in getASTInput) (*sdkmcp.CallToolResult, any, error) {
		astNode, parseDiags, err := parseSource(logger, in.Source)
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

		return textResult(string(astJSON)), json.RawMessage(astJSON), nil
	})
}
