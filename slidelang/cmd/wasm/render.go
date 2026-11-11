// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build js

package main

import (
	"syscall/js"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/slidelang/v2/internal/docpreview"
	"go.ziradocs.com/slidelang/v2/internal/generator"
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

	// docpreview.Render reads with doclang build's rules (docpreview.Parse)
	// and renders with them (docpreview.HTML: code is literal, so a
	// {{variable}} inside a code block or a code span stays as written). An
	// xref failure (an unresolved \ref, a duplicate label) still returns the
	// HTML that could be drawn, with the error and valid=false: a live
	// preview would otherwise go blank each time the author types a \ref to
	// a label that doesn't exist yet.
	res := docpreview.Render(logger, source, func(astNode *ast.AST) renderer.DocumentHTMLOptions {
		return doclangOptions(astNode, theme)
	})
	out := renderOutput{HTML: res.HTML, Diagnostics: res.Diagnostics, Valid: res.Valid}
	if res.Err != nil {
		out.Error = res.Err.Error()
	}
	return mustJSON(out)
}

// doclangOptions are the document HTML options doclangRenderHTML renders
// astNode with; theme is the caller's selector ("" for the document's own).
func doclangOptions(astNode *ast.AST, theme string) renderer.DocumentHTMLOptions {
	title := ""
	var headerFooterConfig *ast.HeaderFooterConfig
	var watermarkConfig *ast.WatermarkConfig
	if astNode.FrontMatter != nil {
		title = astNode.FrontMatter.Title
		headerFooterConfig = astNode.FrontMatter.HeaderFooter
		watermarkConfig = astNode.FrontMatter.Watermark
		// theme == "" es el selector en "document default" (el caller no
		// pasó un segundo argumento, o lo pasó vacío a propósito) — cae al
		// `theme:` del propio front matter, mismo orden de prioridad que
		// generator.go's RenderHTMLPreview (el preview MCP de doclang: CLI
		// > frontmatter > default). Sin esto, un documento con
		// `theme: page-view` sin selector explícito nunca activaba
		// ShowHeaders/ShowFooters ni el chrome legado (hallazgo de code
		// review sobre issue #117).
		if theme == "" {
			theme = astNode.FrontMatter.Theme
		}
	}

	return renderer.DocumentHTMLOptions{
		Title:     title,
		TOC:       true,
		Numbering: true,
		Theme:     theme,
		// ShowHeaders/ShowFooters: mismo criterio que doclang's CLI/preview
		// MCP (theme == "page-view"), para que este playground coincida con
		// lo que `doclang build` produciría. HeaderFooter: header:/footer:/
		// layout_defaults: del front matter (issue #117) — antes esta ruta
		// los omitía por completo, así que el preview nunca coincidía con
		// el CLI para un documento con esa config.
		ShowHeaders:  theme == "page-view",
		ShowFooters:  theme == "page-view",
		HeaderFooter: headerFooterConfig,
		Watermark:    watermarkConfig, // watermark: front matter (issue #179)
		EmbedAssets:  true,
	}
}
