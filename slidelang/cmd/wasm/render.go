// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build js

package main

import (
	"syscall/js"

	"go.ziradocs.com/slidelang/internal/generator"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/renderer"
)

type renderOutput struct {
	HTML        string                   `json:"html,omitempty"`
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
	Valid       bool                     `json:"valid"`
	Error       string                   `json:"error,omitempty"`
}

// slidelangRenderSlides(source: string, theme: string) -> JSON string.
// Renders a self-contained slide-deck HTML (CSS/JS inlined, same
// EmbedAssets=true path Generator.RenderHTMLPreview uses for the MCP
// `preview` tool) — safe to drop into an <iframe srcdoc>. theme may be "" to
// use the document's own frontmatter theme or the built-in default; the
// marquee named themes (modern-blue, ...) are layered on afterward via
// applyNamedThemeOverride since they don't resolve through disk in a
// browser — see theme.go.
func slidelangRenderSlides(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return errJSON(errMissingArg("source"))
	}
	source := args[0].String()
	theme := ""
	if len(args) > 1 {
		theme = args[1].String()
	}

	astNode, parseDiags, err := parseSlidelang(source)
	if err != nil || astNode == nil || hasErrorDiagnostic(parseDiags) {
		out := renderOutput{Diagnostics: parseDiags, Valid: false}
		if err != nil {
			out.Error = err.Error()
		}
		return mustJSON(out)
	}

	gen := generator.New(logger)
	opts := generator.GeneratorOptions{Theme: theme}
	html, err := gen.RenderHTMLPreview(astNode, opts, renderer.NewDefaultRenderContext())
	if err != nil {
		return mustJSON(renderOutput{Diagnostics: parseDiags, Error: err.Error()})
	}

	effectiveTheme := theme
	if effectiveTheme == "" && astNode.FrontMatter != nil {
		effectiveTheme = astNode.FrontMatter.Theme
	}
	html = applyNamedThemeOverride(html, effectiveTheme)

	return mustJSON(renderOutput{HTML: html, Diagnostics: parseDiags, Valid: true})
}

// doclangRenderHTML(source: string, theme: string) -> JSON string. Renders
// a self-contained DocLang document HTML directly via
// renderer.GenerateDocumentHTML (the same pure-core function doclang
// uses) with EmbedAssets forced true, since there's no disk to write
// separate CSS/JS files to. theme resolution is intentionally minimal here
// (see theme.go's doc comment): doclang's named presets
// (professional/academic/technical/page-view) live in a separate Go module
// this package doesn't import, so an unset/unknown theme renders with plain,
// functional default styling rather than reimplementing those presets here.
func doclangRenderHTML(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return errJSON(errMissingArg("source"))
	}
	source := args[0].String()
	theme := ""
	if len(args) > 1 {
		theme = args[1].String()
	}

	astNode, parseDiags, err := parseDoclang(source)
	if err != nil || astNode == nil || hasErrorDiagnostic(parseDiags) {
		out := renderOutput{Diagnostics: parseDiags, Valid: false}
		if err != nil {
			out.Error = err.Error()
		}
		return mustJSON(out)
	}

	title := ""
	if astNode.FrontMatter != nil {
		title = astNode.FrontMatter.Title
	}

	opts := renderer.DocumentHTMLOptions{
		Title:       title,
		TOC:         true,
		Numbering:   true,
		Theme:       theme,
		EmbedAssets: true,
	}
	html := renderer.GenerateDocumentHTML(astNode, opts, renderer.NewDefaultRenderContext())

	return mustJSON(renderOutput{HTML: html, Diagnostics: parseDiags, Valid: true})
}
