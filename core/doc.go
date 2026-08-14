// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package core is the shared engine for parsing, AST, linting, and rendering
// used by the slidelang and doclang CLIs. Starting from v2.0.0, this module
// provides a stable public API for third-party integrations.
//
// # Consumption Model
//
// SlideLang/DocLang can be used as standalone binaries, or they can be
// embedded in other Go programs to inject custom validation rules.
//
//	import (
//		"go.ziradocs.com/slidelang/v2/cli"
//		"go.ziradocs.com/core/v2/linter"
//	)
//
//	func main() {
//		cli.Execute(cli.Options{
//			CustomRules: []linter.Rule{MyRule{}},
//		})
//	}
//
// # Stable Public Contracts (v2.x)
//
// What this project promises to maintain and version according to SemVer:
//
//  1. The entry point API for the CLIs (slidelang/cli and doclang/cli packages),
//     specifically the cli.Options struct which allows injecting policies,
//     custom rules, and PostLint hooks.
//
//  2. The serialized AST (json) schema is versioned independently
//     under ast.SchemaVersion (see @ziradocs/ast-types and the JSON/AST
//     contract at https://ziradocs.com/docs/architecture/json-ast-contract/).
//
// The rest of the Go API (core/ast, core/linter, etc.) has NO SemVer guarantees
// and may change in minor versions. The generated HTML structure and its
// CSS classes are also not part of this stable contract.
//
// # Theme-aware rules (issue #30)
//
// A CustomRule can implement linter.ThemeAware (SetThemeVariables(map[string]string))
// to receive the active theme's resolved CSS variables — supplied by the CLI via
// linter.Linter.WithThemeVariables — before Check runs. core/a11y provides the
// underlying WCAG contrast math (color parsing, luminance, ratio); it has no
// notion of "theme" and does not map element types to variable names, since
// slidelang and doclang each own a different, non-shared variable convention.
// linter.ThemeContrastRule is a reference implementation, not part of DefaultRules().
package core
