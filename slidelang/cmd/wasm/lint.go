// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build js

package main

import (
	"syscall/js"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

// lintOutput mirrors slidelang/internal/mcp/lint.go's lintOutput shape —
// same pipeline (parser + linter.New().Lint), same JSON shape, so tooling
// written against the MCP `lint` tool reads a wasm response identically.
type lintOutput struct {
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
	SlideCount  int                      `json:"slideCount"`
	Valid       bool                     `json:"valid"`
	Error       string                   `json:"error,omitempty"`
}

// slidelangLint(source: string) -> JSON string. This is the same pipeline
// as `slidelang build --lint-only` / the MCP `lint` tool, run in-browser
// with no network round-trip.
func slidelangLint(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return errJSON(errMissingArg("source"))
	}
	source := args[0].String()

	astNode, parseDiags, err := parseSlidelang(source)
	if err != nil {
		return mustJSON(lintOutput{Diagnostics: parseDiags, Valid: false, Error: err.Error()})
	}

	allDiags := parseDiags
	slideCount := 0
	if astNode != nil {
		lintDiags := linter.New().Lint(astNode)
		allDiags = append(allDiags, lintDiags...)
		slideCount = len(astNode.ContentBlocks)
	}

	return mustJSON(lintOutput{
		Diagnostics: allDiags,
		SlideCount:  slideCount,
		Valid:       !hasErrorDiagnostic(allDiags),
	})
}
