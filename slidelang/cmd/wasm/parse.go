// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build js

package main

import (
	"encoding/json"
	"fmt"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/parser"
	"go.ziradocs.com/core/util"
)

const defaultFileName = "playground.slidelang"

// logger is shared by every export in this package. The wasm playground is a
// single client-controlled instance (unlike the MCP server, a long-lived
// process serving many callers) — there is no benefit to per-call structured
// logging that only the user's own browser console would see, and a Noop
// keeps every export a pure function of its string input.
var logger = util.NewNoop()

// parseSlidelang mirrors slidelang/internal/mcp/parse.go's parseSource:
// a size guard before the parser even runs, then parser.New(logger).Parse.
// It intentionally skips that file's concurrency semaphore and
// timeout-with-goroutine-leak guard (util.RunGuarded) — those exist because
// the MCP server is a long-lived process fielding concurrent, untrusted
// callers, where a hung parse leaks a goroutine for the process's entire
// lifetime. A wasm playground tab has none of that: only one call is ever in
// flight (the browser awaits each exported function before making another),
// and a hang only affects the user's own tab, which they can reload.
func parseSlidelang(source string) (*ast.AST, []diagnostics.Diagnostic, error) {
	if err := util.CheckInputSize(len(source), util.DefaultMaxInputBytes); err != nil {
		return nil, nil, err
	}
	p := parser.New(logger)
	astNode, diags := p.Parse(source, defaultFileName)
	return astNode, diags, nil
}

// parseDoclang parses DocLang source the same way doclang build does:
// always DocumentFlexParser, mode: is ignored, frontmatter is optional.
func parseDoclang(source string) (*ast.AST, []diagnostics.Diagnostic, error) {
	if err := util.CheckInputSize(len(source), util.DefaultMaxInputBytes); err != nil {
		return nil, nil, err
	}
	p := parser.NewDocumentFlexParserWithNormalization(source, logger)
	astNode, diags := p.Parse()
	return astNode, diags, nil
}

func hasErrorDiagnostic(diags []diagnostics.Diagnostic) bool {
	for _, d := range diags {
		if d.IsError() {
			return true
		}
	}
	return false
}

// diagError is the {error} shape every export falls back to on failure —
// kept minimal and uniform so playground.js has one error path to check,
// regardless of which exported function it called.
type diagError struct {
	Error string `json:"error"`
}

func errJSON(err error) string {
	return mustJSON(diagError{Error: err.Error()})
}

// errMissingArg reports a JS-side misuse (calling an export with too few
// arguments) — distinct from a content problem, so it's never confused with
// a parse/lint diagnostic.
func errMissingArg(name string) error {
	return fmt.Errorf("missing required argument %q", name)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// json.Marshal only fails on unmarshalable input (channels, funcs) —
		// never the plain structs this package passes it. Route the message
		// through json.Marshal on a bare string (which cannot itself fail)
		// rather than fmt.Sprintf-ing err.Error() straight into the JSON
		// literal, so a quote/backslash in the error text can't produce
		// invalid JSON that playground.js's JSON.parse then chokes on.
		msg, _ := json.Marshal(fmt.Sprintf("internal: failed to encode response: %s", err.Error()))
		return fmt.Sprintf(`{"error":%s}`, msg)
	}
	return string(b)
}
