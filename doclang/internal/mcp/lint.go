// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/internal/pipeline"
)

type lintInput struct {
	Source string `json:"source" jsonschema:"the .doclang source content to validate"`
	// Path es la ruta del documento como la escribiría `doclang build`
	// (relativa al proyecto). Solo sirve para que una regla de lint_policy
	// con scope: aplique igual que en el build; sin ella, una regla con
	// scope no aplica, como en el build de un documento sin ruta.
	Path string `json:"path,omitempty" jsonschema:"optional path of the document as passed to doclang build; used only to match lint_policy scope globs"`
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
// default), y la aplica también a los avisos del parser
// (pipeline.EvaluatePolicy), como el build.
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

		// Los avisos del parser de una regla del linter (LINK001 de una
		// imagen de bloque) pasan por la política con los del linter y sin
		// duplicarse, igual que en `doclang build` (ver
		// pipeline.SplitParserFindings).
		parseDiags, parserFindings := pipeline.SplitParserFindings(parseDiags)
		allDiags := parseDiags
		sectionCount := 0
		if astNode == nil || hasErrorDiagnostic(parseDiags) {
			allDiags = append(allDiags, parserFindings...)
		} else {
			policy, err := linter.ResolvePolicyConfig("", astNode.FrontMatter)
			if err != nil {
				out := lintOutput{Diagnostics: append(allDiags, parserFindings...), Valid: false, Error: err.Error()}
				res, resErr := errorResult(out)
				return res, out, resErr
			}
			// Mismo dialecto que `doclang build` (internal/cli/build.go).
			lintDiags := linter.New().WithDialect(linter.DialectDocuments).WithPolicy(policy).LintUnfiltered(astNode)
			lintDiags = pipeline.MergeParserFindings(astNode, parserFindings, lintDiags)
			// La política se aplica a los avisos del parser y a los del
			// linter por el mismo camino que `doclang build`: un TABLE004
			// apagado no sale, y uno subido a error deja valid=false.
			evaluated := pipeline.EvaluatePolicy(policy, parseDiags, lintDiags, in.Path, time.Now())
			allDiags = append(append([]diagnostics.Diagnostic(nil), evaluated.ParserActive...), evaluated.LintActive...)
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
