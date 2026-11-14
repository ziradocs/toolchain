// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
//
// A diferencia de build, un @include que no se puede expandir (el archivo no
// existe, queda fuera del directorio de trabajo, o lo que incluye falla) no
// corta: esa línea queda como está y el parser la reporta con DIRECTIVE003,
// como antes de que este tool expandiera. Un cliente cuyo directorio de
// trabajo no es el del documento sigue recibiendo el lint de siempre. Se
// decide por cada línea @include del documento: las que sí se expanden, se
// expanden. skipped dice, por línea del documento (1-based), por qué no se
// expandió la suya (ver skippedIncludeReason).
//
// Devuelve el contenido expandido, el mapa de orígenes y el directorio del
// documento para findings.MapToSource.
func expandIncludes(source, fileName string) (expanded string, sources include.SourceMap, docDir string, skipped map[int]string, err error) {
	if fileName == "" {
		fileName = defaultFileName
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", include.SourceMap{}, "", nil, fmt.Errorf("failed to expand includes: cannot determine the server's working directory: %w", err)
	}
	read := os.ReadFile
	root, rootErr := util.ResolveConfinedPath(wd, filepath.Dir(fileName))
	if rootErr != nil {
		root = wd
		read = func(string) ([]byte, error) { return nil, rootErr }
	}
	docDir = findings.DocumentDir(filepath.Join(root, filepath.Base(fileName)))

	// Cada @include se prueba solo; el que falla se cambia por un marcador
	// único, que la expansión deja pasar como una línea más del documento,
	// y después se vuelve a poner la línea original en su lugar. Así el
	// mapa de orígenes no cambia: esa línea sigue siendo del documento.
	lines := strings.Split(source, "\n")
	placeholders := map[string]string{}
	for i, line := range lines {
		incPath, ok := includePath(line)
		if !ok {
			continue
		}
		if _, _, err := include.ExpandWithSourceMap(line, root, read); err != nil {
			if skipped == nil {
				skipped = map[int]string{}
			}
			skipped[i+1] = skippedIncludeReason(incPath, err)
			mark := fmt.Sprintf("\x00slidelang-mcp-include-%d\x00", i)
			placeholders[mark] = line
			lines[i] = mark
		}
	}
	text, sources, err := include.ExpandWithSourceMap(strings.Join(lines, "\n"), root, read)
	if err != nil {
		// Cada línea se expandía sola; si juntas fallan, el documento se
		// lintea sin expandir, como antes.
		return source, include.SourceMap{}, docDir, nil, nil
	}
	if len(placeholders) > 0 {
		out := strings.Split(text, "\n")
		for i, line := range out {
			if orig, ok := placeholders[line]; ok {
				out[i] = orig
			}
		}
		text = strings.Join(out, "\n")
	}
	// Mismo re-chequeo que build: los @include anidados o repetidos pueden
	// inflar un documento chico por encima del límite.
	if err := util.CheckInputSize(len(text), util.DefaultMaxInputBytes); err != nil {
		return "", include.SourceMap{}, "", nil, fmt.Errorf("expanded content exceeds size limit: %w", err)
	}
	return text, sources, docDir, skipped, nil
}

// includePath reconoce una línea @include como lo hace core/include: la
// línea recortada es "@include" (sin ruta) o empieza con "@include ".
func includePath(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "@include" {
		return "", true
	}
	rest, ok := strings.CutPrefix(trimmed, "@include ")
	return strings.TrimSpace(rest), ok
}

// skippedIncludeReason es lo que se agrega al DIRECTIVE003 de un @include que
// no se expandió: la ruta, por su texto, y por qué. Sin número de línea.
func skippedIncludeReason(incPath string, err error) string {
	why := "no se pudo leer"
	switch {
	case errors.Is(err, util.ErrPathEscapesBase):
		why = "queda fuera del directorio de trabajo del servidor"
	case errors.Is(err, fs.ErrNotExist):
		why = "el archivo no existe"
	}
	if incPath == "" {
		return "; no se expandió: falta la ruta"
	}
	return fmt.Sprintf("; no se expandió %q: %s", incPath, why)
}

// explainSkippedIncludes agrega a cada DIRECTIVE003 de una línea @include que
// expandIncludes no expandió el motivo. La regla y la severidad no cambian.
func explainSkippedIncludes(diags []diagnostics.Diagnostic, skipped map[int]string) []diagnostics.Diagnostic {
	if len(skipped) == 0 {
		return diags
	}
	for i, d := range diags {
		if d.RuleID != "DIRECTIVE003" || d.File != "" || !strings.HasPrefix(d.Message, "La directiva @include ") {
			continue
		}
		if why, ok := skipped[d.Position.Line]; ok {
			diags[i].Message += why
		}
	}
	return diags
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
		Description: "Parse and lint SlideLang source content, returning structured diagnostics (errors and warnings) without writing any file to disk. @include lines are expanded as slidelang build does, from the directory of fileName (relative to the server's working directory, which they cannot leave); a diagnostic from an included file names it in file. An @include that cannot be expanded (missing, or outside the working directory) is left as is and reported with the DIRECTIVE003 warning, which says why.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in lintInput) (*sdkmcp.CallToolResult, lintOutput, error) {
		if err := util.CheckInputSize(len(in.Source), util.DefaultMaxInputBytes); err != nil {
			out := lintOutput{Valid: false, Error: err.Error()}
			res, resErr := errorResult(out)
			return res, out, resErr
		}
		source, sources, docDir, skipped, err := expandIncludes(in.Source, in.FileName)
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
		parseDiags = explainSkippedIncludes(findings.MapToSource(sources, parseDiags, docDir), skipped)
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
