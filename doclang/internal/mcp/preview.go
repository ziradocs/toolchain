// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/internal/generator"
)

type previewInput struct {
	Source string `json:"source" jsonschema:"the .doclang source content to render"`
	Theme  string `json:"theme,omitempty" jsonschema:"theme name to render with (see list_themes); defaults to the document's own theme or 'professional'"`
}

type previewError struct {
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
	// Error se puebla cuando parseSource falla a nivel de guard/transform, o
	// cuando el theme pedido es inválido — en esos casos Diagnostics viene
	// vacío.
	Error string `json:"error,omitempty"`
}

// registerPreviewTool registra el tool "preview": parsea source y devuelve
// un HTML autocontenido (CSS embebido, modo browser — mermaid/chart/map/
// plantuml/math quedan como client-side contra CDN, igual que un `doclang
// build` sin --render-mode offline) vía Generator.RenderHTMLPreview, sin
// escribir ningún archivo. html-only deliberado: a diferencia de slidelang
// (solo HTML), doclang también genera pdf/docx, pero ambos son binarios que
// requieren Chromium o un archivo de salida — no tiene sentido "previsualizar"
// ninguno de los dos como texto en una tool call.
func registerPreviewTool(server *sdkmcp.Server, logger util.Logger) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview",
		Description: "Parse DocLang source content and return a self-contained HTML preview (CSS embedded), without writing any file to disk.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in previewInput) (*sdkmcp.CallToolResult, any, error) {
		// in.Theme es input de un cliente MCP, no el flag --theme del
		// operador — se valida contra la lista de temas conocidos antes de
		// pasarlo a RenderHTMLPreview (ver validateThemeName, ME-2).
		if err := validateThemeName(in.Theme); err != nil {
			out := toolError{Error: err.Error()}
			res, resErr := errorResult(out)
			return res, out, resErr
		}

		astNode, parseDiags, err := parseSource(logger, in.Source)
		if err != nil || astNode == nil || hasErrorDiagnostic(parseDiags) {
			out := previewError{Diagnostics: parseDiags}
			if err != nil {
				out.Error = err.Error()
			}
			res, resErr := errorResult(out)
			return res, out, resErr
		}

		gen := generator.New(logger)
		html := gen.RenderHTMLPreview(astNode, in.Theme)

		// nil en vez de un string crudo: structuredContent se tipa como
		// objeto en el spec de MCP (ver el mismo comentario en
		// slidelang/internal/mcp/preview.go) — un cliente que valide
		// contra el schema podría rechazar la respuesta.
		return textResult(html), nil, nil
	})
}
