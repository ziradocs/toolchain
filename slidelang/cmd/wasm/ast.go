// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build js

package main

import (
	"syscall/js"

	"go.ziradocs.com/slidelang/internal/generator"
	"go.ziradocs.com/core/diagnostics"
)

type astError struct {
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
	Error       string                   `json:"error,omitempty"`
}

// slidelangGetAST(source: string) -> JSON string. Same shape as
// `slidelang build --format json` / the MCP `get_ast` tool: the serialized
// AST contract, with *HTML fields pre-populated, via
// Generator.RenderASTJSON — no file written.
func slidelangGetAST(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return errJSON(errMissingArg("source"))
	}
	source := args[0].String()

	astNode, parseDiags, err := parseSlidelang(source)
	if err != nil || astNode == nil || hasErrorDiagnostic(parseDiags) {
		out := astError{Diagnostics: parseDiags}
		if err != nil {
			out.Error = err.Error()
		}
		return mustJSON(out)
	}

	gen := generator.New(logger)
	astJSON, err := gen.RenderASTJSON(astNode)
	if err != nil {
		return mustJSON(astError{Diagnostics: parseDiags, Error: err.Error()})
	}

	return string(astJSON)
}
