// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/findings"
)

type lintInput struct {
	Source string `json:"source" jsonschema:"the .slidelang source content to validate"`
	// FileName es la ruta del documento como la escribiría `slidelang
	// build`, relativa al directorio de trabajo del servidor: nombra el
	// documento en los diagnósticos, se compara con el scope de
	// lint_policy y es desde donde se resuelven sus @include (ver
	// expandIncludes).
	FileName string `json:"fileName,omitempty" jsonschema:"optional path of the document as passed to slidelang build, relative to the server's working directory; labels diagnostics, matches lint_policy scope globs, and is where @include lines resolve from (the document's directory, confined to the server's working directory)"`
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

// expandIncludes expande los @include de source como `slidelang build`
// (include.ExpandWithSourceMap), con la raíz que build usa por defecto: el
// directorio del documento. fileName es relativo al directorio de trabajo
// del servidor y ese directorio no se sale de él: el contenido de un tool
// call es tan poco confiable como un archivo de entrada, y un @include le
// mostraría al cliente el texto de lo que lea (un FLEX001 cita la línea).
// Por eso un fileName absoluto o que sale del directorio de trabajo no
// rechaza el documento, pero hace fallar cualquier @include que tenga;
// uno sin @include no lee nada y no cambia. Devuelve el contenido expandido,
// el mapa de orígenes y el directorio del documento para
// findings.MapToSource.
func expandIncludes(source, fileName string) (string, include.SourceMap, string, error) {
	if fileName == "" {
		fileName = defaultFileName
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", include.SourceMap{}, "", fmt.Errorf("failed to expand includes: cannot determine the server's working directory: %w", err)
	}
	read := os.ReadFile
	root, rootErr := util.ResolveConfinedPath(wd, filepath.Dir(fileName))
	if rootErr != nil {
		root = wd
		read = func(string) ([]byte, error) {
			return nil, fmt.Errorf("@include needs fileName inside the server's working directory: %w", rootErr)
		}
	}
	expanded, sources, err := include.ExpandWithSourceMap(source, root, read)
	if err != nil {
		return "", include.SourceMap{}, "", fmt.Errorf("failed to expand includes: %w", err)
	}
	// Mismo re-chequeo que build: los @include anidados o repetidos pueden
	// inflar un documento chico por encima del límite.
	if err := util.CheckInputSize(len(expanded), util.DefaultMaxInputBytes); err != nil {
		return "", include.SourceMap{}, "", fmt.Errorf("expanded content exceeds size limit: %w", err)
	}
	return expanded, sources, findings.DocumentDir(filepath.Join(root, filepath.Base(fileName))), nil
}

// registerLintTool registra el tool "lint": parsea + corre el linter sobre
// source (sin escribir nada a disco) y devuelve todos los diagnósticos
// (parser + linter) en un solo JSON — el mismo pipeline y las mismas reglas
// que `slidelang build --lint-only`, pero contra contenido en memoria en vez
// de un archivo, y devolviendo diagnósticos estructurados en vez de texto a
// stderr. Como build, expande los @include antes de parsear y lleva cada
// diagnóstico al archivo y la línea que lo escribió (Diagnostic.File). Solo
// este tool: get_ast y preview siguen sin expandirlos.
func registerLintTool(server *sdkmcp.Server, logger util.Logger) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "lint",
		Description: "Parse and lint SlideLang source content, returning structured diagnostics (errors and warnings) without writing any file to disk. @include lines are expanded as slidelang build does, from the directory of fileName (relative to the server's working directory, which they cannot leave); a diagnostic from an included file names it in file.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in lintInput) (*sdkmcp.CallToolResult, lintOutput, error) {
		if err := util.CheckInputSize(len(in.Source), util.DefaultMaxInputBytes); err != nil {
			out := lintOutput{Valid: false, Error: err.Error()}
			res, resErr := errorResult(out)
			return res, out, resErr
		}
		source, sources, docDir, err := expandIncludes(in.Source, in.FileName)
		if err != nil {
			out := lintOutput{Valid: false, Error: err.Error()}
			res, resErr := errorResult(out)
			return res, out, resErr
		}
		astNode, parseDiags, err := parseSource(logger, source, in.FileName)
		if err != nil {
			out := lintOutput{Diagnostics: findings.MapToSource(sources, parseDiags, docDir), Valid: false, Error: err.Error()}
			res, resErr := errorResult(out)
			return res, out, resErr
		}

		// Los avisos del parser de una regla del linter (LINK001 de una
		// imagen de bloque) van con los del linter, sin duplicarse, igual que
		// en `slidelang build` (ver findings.SplitParserFindings).
		parseDiags, parserFindings := findings.SplitParserFindings(parseDiags)
		// Las posiciones cuentan sobre el contenido expandido: se llevan a
		// su archivo antes de la política, como en build. Los
		// parserFindings se llevan después de juntarlos con los del linter,
		// que los compara con las posiciones del AST.
		parseDiags = findings.MapToSource(sources, parseDiags, docDir)
		slideCount := 0
		if astNode != nil {
			slideCount = len(astNode.ContentBlocks)
		}
		var allDiags []diagnostics.Diagnostic
		if astNode == nil || hasErrorDiagnostic(parseDiags) {
			// Con errores del parser el build se corta antes del linter y de
			// la política, así que acá tampoco se aplican: una política no
			// apaga un error del parser. La telemetría del normalizador no
			// sale en ningún camino.
			allDiags = append(findings.ReportableParserDiagnostics(parseDiags), findings.MapToSource(sources, parserFindings, docDir)...)
		} else {
			// Misma política que `slidelang build` (flag > lint_policy: del
			// frontmatter > default) y por el mismo camino
			// (findings.EvaluatePolicy): se aplica a los avisos del parser y a
			// los del linter.
			policy, err := linter.ResolvePolicyConfig("", astNode.FrontMatter)
			if err != nil {
				out := lintOutput{Diagnostics: append(findings.ReportableParserDiagnostics(parseDiags), findings.MapToSource(sources, parserFindings, docDir)...), Valid: false, Error: err.Error()}
				res, resErr := errorResult(out)
				return res, out, resErr
			}
			lintDiags := linter.New().WithPolicy(policy).LintUnfiltered(astNode)
			lintDiags = findings.MergeParserFindings(astNode, parserFindings, lintDiags)
			lintDiags = findings.MapToSource(sources, lintDiags, docDir)
			evaluated := findings.EvaluatePolicy(policy, parseDiags, lintDiags, astNode.FilePath, time.Now())
			allDiags = append(append([]diagnostics.Diagnostic(nil), evaluated.ParserActive...), evaluated.LintActive...)
		}
		if len(allDiags) == 0 {
			allDiags = nil
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
