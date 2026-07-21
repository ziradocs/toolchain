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
	Source string `json:"source" jsonschema:"the .doclang source content to validate"`
}

type lintOutput struct {
	Diagnostics  []diagnostics.Diagnostic `json:"diagnostics"`
	SectionCount int                      `json:"sectionCount"`
	Valid        bool                     `json:"valid"`
	// Error se puebla solo cuando parseSource falla a nivel de guard
	// (timeout/tamaño) o de transform (label duplicado, \ref sin resolver —
	// ver parse.go) — en esos casos Diagnostics puede venir vacío y no hay
	// otra forma de que el cliente sepa por qué.
	Error string `json:"error,omitempty"`
}

// registerLintTool registra el tool "lint": parsea + corre el linter sobre
// source (sin escribir nada a disco) y devuelve todos los diagnósticos
// (parser + linter) en un solo JSON — el mismo pipeline y las mismas reglas
// que `doclang build --lint-only`, pero contra contenido en memoria en vez
// de un archivo. WithPolicy(ResolvePolicyConfig) reusa exactamente la misma
// resolución de política que el CLI (flag > lint_policy: de frontmatter >
// default) — a diferencia de la tool "lint" de slidelang, que hoy no aplica
// política alguna (linter.New() a secas).
func registerLintTool(server *sdkmcp.Server, logger util.Logger) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "lint",
		Description: "Parse and lint DocLang source content, returning structured diagnostics (errors and warnings) without writing any file to disk.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in lintInput) (*sdkmcp.CallToolResult, lintOutput, error) {
		astNode, parseDiags, err := parseSource(logger, in.Source)
		if err != nil {
			out := lintOutput{Diagnostics: parseDiags, Valid: false, Error: err.Error()}
			res, resErr := errorResult(out)
			return res, out, resErr
		}

		allDiags := parseDiags
		sectionCount := 0
		if astNode != nil && !hasErrorDiagnostic(parseDiags) {
			policy, err := linter.ResolvePolicyConfig("", astNode.FrontMatter)
			if err != nil {
				out := lintOutput{Diagnostics: allDiags, Valid: false, Error: err.Error()}
				res, resErr := errorResult(out)
				return res, out, resErr
			}
			lintDiags := linter.New().WithPolicy(policy).Lint(astNode)
			allDiags = append(allDiags, lintDiags...)
			sectionCount = len(astNode.ContentBlocks)
		}

		out := lintOutput{
			Diagnostics:  allDiags,
			SectionCount: sectionCount,
			Valid:        !hasErrorDiagnostic(allDiags),
		}
		res, resErr := jsonResult(out)
		return res, out, resErr
	})
}
