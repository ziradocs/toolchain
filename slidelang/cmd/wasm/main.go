// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build js

// Command wasm builds the SlideLang/DocLang playground's WebAssembly entry
// point (issue #134). It exposes parse/lint/render as plain JS-callable
// functions on the global object — each takes/returns strings (JSON for
// structured results), so the JS side (playground/playground.js) needs no
// bindings generator, just window.<name>(...).
//
// Build with:
//
//	GOOS=js GOARCH=wasm go build -o playground/slidelang.wasm ./cmd/wasm
//
// and pair it with $(go env GOROOT)/lib/wasm/wasm_exec.js, which this
// program depends on for its JS runtime shims (see playground/README.md).
package main

import (
	"fmt"
	"syscall/js"
)

// guard wraps a js.FuncOf callback with panic recovery. Unlike the MCP
// server (a long-lived process serving many callers, where util.RunGuarded's
// RecoverGuard turns a panic into an error for just that one request), a
// panic that escapes a js.FuncOf callback here propagates out of the JS
// callback and kills the single Go runtime instance backing the whole
// playground tab — every export becomes permanently unresponsive until the
// user manually reloads the page. guard converts that into an ordinary error
// JSON instead, matching every other export's failure shape.
func guard(fn func(js.Value, []js.Value) any) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) (result any) {
		defer func() {
			if r := recover(); r != nil {
				result = errJSON(fmt.Errorf("internal: panic during call: %v", r))
			}
		}()
		return fn(this, args)
	})
}

func main() {
	js.Global().Set("slidelangLint", guard(slidelangLint))
	js.Global().Set("slidelangGetAST", guard(slidelangGetAST))
	js.Global().Set("slidelangRenderSlides", guard(slidelangRenderSlides))
	js.Global().Set("slidelangListThemes", guard(slidelangListThemes))
	js.Global().Set("doclangRenderHTML", guard(doclangRenderHTML))

	// A wasm program that returns from main() exits — its exported functions
	// stop working the instant JS calls back into a dead Go runtime. Every
	// other Go-wasm program with JS-callable exports blocks the same way.
	select {}
}
