// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/generator"
)

type previewInput struct {
	Source   string `json:"source" jsonschema:"the .slidelang source content to render"`
	FileName string `json:"fileName,omitempty" jsonschema:"optional file name, used only to label diagnostics"`
	Theme    string `json:"theme,omitempty" jsonschema:"theme name to render with (see list_themes); defaults to the document's own theme or the built-in default"`
}

type previewError struct {
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
	// Error se puebla solo cuando parseSource falla a nivel de guard
	// (timeout/panic) — en ese caso Diagnostics viene vacío.
	Error string `json:"error,omitempty"`
}

// registerPreviewTool registra el tool "preview": parsea source y devuelve
// un HTML autocontenido (CSS/JS embebidos, browser render-mode — mermaid,
// charts y mapas quedan como client-side contra CDN, igual que un
// `slidelang build` sin --render-mode offline) vía
// Generator.RenderHTMLPreview, sin escribir ningún archivo.
func registerPreviewTool(server *sdkmcp.Server, logger util.Logger) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview",
		Description: "Parse SlideLang source content and return a self-contained HTML preview (CSS/JS embedded), without writing any file to disk.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in previewInput) (*sdkmcp.CallToolResult, any, error) {
		// in.Theme es input de un cliente MCP, no el flag --theme del operador
		// — se valida contra la lista de temas conocidos antes de pasarlo a
		// GeneratorOptions (ver validateThemeName en themes.go, ME-2).
		if err := validateThemeName(in.Theme); err != nil {
			out := toolError{Error: err.Error()}
			res, resErr := errorResult(out)
			return res, out, resErr
		}

		astNode, parseDiags, err := parseSource(logger, in.Source, in.FileName)
		if err != nil || astNode == nil || hasErrorDiagnostic(parseDiags) {
			out := previewError{Diagnostics: parseDiags}
			if err != nil {
				// err viene del guard de parseSource (timeout/panic), no de un
				// diagnóstico de contenido -- parseDiags está vacío en este caso
				// y sin esto el cliente no tendría forma de saber por qué falló.
				out.Error = err.Error()
			}
			res, resErr := errorResult(out)
			return res, out, resErr
		}

		gen := generator.New(logger)
		opts := generator.GeneratorOptions{Theme: in.Theme}
		html, err := gen.RenderHTMLPreview(astNode, opts, renderer.NewDefaultRenderContext())
		if err != nil {
			out := toolError{Error: err.Error()}
			res, resErr := errorResult(out)
			return res, out, resErr
		}

		// El segundo valor de retorno se serializa a structuredContent, que
		// el spec de MCP tipa como objeto — un string HTML crudo ahí viola
		// ese contrato y un cliente que valide contra el schema podría
		// rechazar la respuesta (aun siendo el content de texto perfectamente
		// usable). nil hace que el SDK omita structuredContent por completo
		// en vez de escribir un valor no-objeto.
		return textResult(html), nil, nil
	})
}
