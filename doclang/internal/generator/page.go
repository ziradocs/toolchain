// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
)

// resolvePDFOptions resolves the parsed `page:` front matter namespace
// (ast.PageConfig — author-facing strings verbatim, "A4", "2cm", see
// core/util/length.go's doc comment) into chromium.PDFOptions, which
// Chromium's own Page.printToPDF wants in inches. All the coupling to
// PageConfig's shape lives in this one file: if core's contract for `page:`
// ever changes shape, only this file moves.
//
// Starts from chromium.DefaultPDFOptions() and overrides only what page
// actually declares — not stylistic: cdproto's PDFOptions margin fields are
// float64 WITHOUT `omitempty`, so a bare `0` serializes as a real
// zero-margin instruction to Chromium. An "unset = 0" internal
// representation here would turn an undeclared margin side into an actual
// zero margin instead of falling back to DefaultPDFOptions' 0.4in.
//
// An unrecognized page.size or page.margins.* value logs a warning and
// falls back to the default rather than failing the build — same criterion
// core/parser/frontmatter.go uses for a malformed `lang:` — the parser
// already warned once at parse time (FRONT006) and conserved the value
// verbatim; this is the point where a still-unresolvable value finally
// falls back to something renderable.
func resolvePDFOptions(page *ast.PageConfig, logger util.Logger) chromium.PDFOptions {
	opts := chromium.DefaultPDFOptions()
	if page == nil {
		return opts
	}

	if page.Size != "" {
		if w, h, ok := util.PaperSizeInches(page.Size); ok {
			opts.PaperWidth = w
			opts.PaperHeight = h
		} else {
			logger.Warn("page.size %q is not a recognized paper size (e.g. A4, Letter, Legal) — keeping the default", page.Size)
		}
	}

	if page.Margins != nil {
		resolveMargin(logger, "top", page.Margins.Top, &opts.MarginTop)
		resolveMargin(logger, "right", page.Margins.Right, &opts.MarginRight)
		resolveMargin(logger, "bottom", page.Margins.Bottom, &opts.MarginBottom)
		resolveMargin(logger, "left", page.Margins.Left, &opts.MarginLeft)
	}

	return opts
}

// resolveMargin overrides *target in place only when value is declared and
// resolves to a recognized length — an empty side (the per-side map form
// leaves undeclared sides as "") or an unrecognized unit both leave *target
// at whatever chromium.DefaultPDFOptions() already set it to.
func resolveMargin(logger util.Logger, side, value string, target *float64) {
	if value == "" {
		return
	}
	inches, err := util.ParseLengthInches(value)
	if err != nil {
		logger.Warn("page.margins.%s %q is not a recognized length (e.g. 2cm, 0.5in, 40px) — keeping the default", side, value)
		return
	}
	*target = inches
}
