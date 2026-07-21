// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/linter"
	"go.ziradocs.com/core/util"
)

type lintInput struct {
	Source   string `json:"source" jsonschema:"the .slidelang source content to validate"`
	FileName string `json:"fileName,omitempty" jsonschema:"optional file name, used only to label diagnostics"`
}

type lintOutput struct {
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
	SlideCount  int                      `json:"slideCount"`
	Valid       bool                     `json:"valid"`
	// Error se puebla solo cuando parseSource falla a nivel de guard
	// (timeout/panic, no un diagnóstico de contenido) — en ese caso
	// Diagnostics viene vacío y no hay otra forma de que el cliente sepa
	// por qué.
	Error string `json:"error,omitempty"`
}

// registerLintTool registra el tool "lint": parsea + corre el linter sobre
// source (sin escribir nada a disco) y devuelve todos los diagnósticos
// (parser + linter) en un solo JSON — el mismo pipeline y las mismas reglas
// que `slidelang build --lint-only`, pero contra contenido en memoria en vez
// de un archivo, y devolviendo diagnósticos estructurados en vez de texto a
// stderr.
func registerLintTool(server *sdkmcp.Server, logger util.Logger) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "lint",
		Description: "Parse and lint SlideLang source content, returning structured diagnostics (errors and warnings) without writing any file to disk.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in lintInput) (*sdkmcp.CallToolResult, lintOutput, error) {
		astNode, parseDiags, err := parseSource(logger, in.Source, in.FileName)
		if err != nil {
			out := lintOutput{Diagnostics: parseDiags, Valid: false, Error: err.Error()}
			res, resErr := errorResult(out)
			return res, out, resErr
		}

		allDiags := parseDiags
		slideCount := 0
		if astNode != nil {
			lintDiags := linter.New().Lint(astNode)
			allDiags = append(allDiags, lintDiags...)
			slideCount = len(astNode.ContentBlocks)
		}

		out := lintOutput{
			Diagnostics: allDiags,
			SlideCount:  slideCount,
			Valid:       !hasErrorDiagnostic(allDiags),
		}
		res, resErr := jsonResult(out)
		return res, out, resErr
	})
}
