// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// Defaults applied by ResolveWatermark when `watermark:` is present but a
// given key isn't — kept in sync by hand with core/parser/frontmatter.go's
// own defaultWatermark* constants (those exist purely for diagnostic
// message text; these are what a renderer actually falls back to when the
// parsed pointer is nil, since the AST is a public JSON contract that a
// tool other than this parser could also produce directly).
const (
	DefaultWatermarkOpacity  = 0.08
	DefaultWatermarkRotation = -45.0
	DefaultWatermarkColor    = "#000000"
	DefaultWatermarkFontSize = "72pt"
)

// watermarkRepeatCount is how many copies of the text WatermarkHTML draws
// when Repeat is true — an oversized rotated flex-wrap grid tiles them
// into a diagonal repeating pattern via pure CSS, no per-viewport-size
// computation needed. Approximate by nature (the exact tile count a given
// viewport/page needs varies with text length and font size), same
// "close enough, not pixel-exact" acceptance the pptx text-height
// estimation elsewhere in this codebase already relies on.
const watermarkRepeatCount = 48

// ResolvedWatermark is ast.WatermarkConfig after defaults are applied and
// Text has gone through ProcessVariables — the one shape every backend
// (slidelang html/pdf/pptx, doclang html/pdf/docx) consumes, so the
// defaulting policy lives in exactly one place instead of five.
type ResolvedWatermark struct {
	Text     string
	Color    string  // CSS color string, author-provided or DefaultWatermarkColor
	Opacity  float64 // 0.0-1.0, already clamped
	Rotation float64 // degrees, already normalized to (-360, 360)
	FontSize string  // verbatim length string (e.g. "72pt"), author-provided or DefaultWatermarkFontSize
	Repeat   bool
}

// ResolveWatermark applies defaults to cfg and substitutes {{variables}}
// into Text. ok is false when there's nothing to draw — cfg is nil,
// Enabled is false, or Text resolves to empty/blank — so every caller can
// write `if rw, ok := ResolveWatermark(...); ok { ... }` without its own
// nil/empty-string checks.
func ResolveWatermark(cfg *ast.WatermarkConfig, variables map[string]interface{}) (ResolvedWatermark, bool) {
	if cfg == nil || !cfg.Enabled {
		return ResolvedWatermark{}, false
	}
	text := ProcessVariables(cfg.Text, variables)
	if strings.TrimSpace(text) == "" {
		return ResolvedWatermark{}, false
	}

	rw := ResolvedWatermark{
		Text:     text,
		Color:    DefaultWatermarkColor,
		Opacity:  DefaultWatermarkOpacity,
		Rotation: DefaultWatermarkRotation,
		FontSize: DefaultWatermarkFontSize,
		Repeat:   true,
	}
	if cfg.Color != "" {
		rw.Color = cfg.Color
	}
	if cfg.Opacity != nil {
		rw.Opacity = *cfg.Opacity
	}
	if cfg.Rotation != nil {
		rw.Rotation = *cfg.Rotation
	}
	if cfg.FontSize != "" {
		rw.FontSize = cfg.FontSize
	}
	if cfg.Repeat != nil {
		rw.Repeat = *cfg.Repeat
	}

	// Defensive clamp/normalize, independent of core/parser/frontmatter.go's
	// own clamp — ast.WatermarkConfig is a public JSON contract another
	// tool could populate directly (e.g. via --format json roundtrip),
	// bypassing the parser entirely.
	rw.Opacity = math.Min(1, math.Max(0, rw.Opacity))
	rw.Rotation = math.Mod(rw.Rotation, 360)

	return rw, true
}

// formatWatermarkFloat renders a resolver-produced float (never the
// author's raw string) for interpolation into inline CSS.
func formatWatermarkFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// WatermarkHTML renders rw as a decorative overlay: a full-size wrapper
// (the caller sizes/positions it via class, matching the slide/page
// container it decorates) holding a rotated, centered inner box that
// either holds one instance of the text (Repeat false) or
// watermarkRepeatCount instances laid out by CSS flex-wrap (Repeat true),
// which tiles them into a diagonal repeating pattern purely via CSS.
//
// class is used LITERALLY, not namespaced here — this package builds HTML
// with fmt.Sprintf, not html/template, and has no notion of slidelang's
// "slidelang-" class-prefixing pass (which only rewrites literal class="..."
// strings in STATIC Go-template source before it's parsed — a string
// returned by a Go function at execute time never passes through it, the
// same reason the header logo div is hand-namespaced as
// "slidelang-header-logo" rather than "header-logo"). Callers must pass an
// already-final class name (e.g. "slidelang-watermark", "doclang-watermark").
//
// aria-hidden is set because this is decorative, not content: the text
// carries no information beyond what's already in the document, and a
// screen reader announcing it once per slide/page would be pure noise.
//
// The rotate/translate transform, color and opacity are all built from
// already-parsed values (ResolveWatermark's output), never concatenated
// from the author's raw string — sanitizeStyleValue (below) rejects
// ";"/quotes, and assembling the transform from a raw string would be
// exactly how that guard gets bypassed.
func WatermarkHTML(rw ResolvedWatermark, class string) string {
	textEscaped := EscapeHTML(rw.Text)

	color := sanitizeStyleValue(rw.Color)
	if color == "" {
		color = DefaultWatermarkColor
	}
	fontSize := sanitizeStyleValue(rw.FontSize)
	if fontSize == "" {
		fontSize = DefaultWatermarkFontSize
	}
	opacity := math.Min(1, math.Max(0, rw.Opacity))
	rotation := math.Mod(rw.Rotation, 360)

	spanStyle := fmt.Sprintf("color: %s; opacity: %s; font-size: %s;",
		color, formatWatermarkFloat(opacity), fontSize)

	count := 1
	if rw.Repeat {
		count = watermarkRepeatCount
	}
	var spans strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&spans, `<span style="%s">%s</span>`, spanStyle, textEscaped)
	}

	return fmt.Sprintf(`<div class="%s" aria-hidden="true"><div class="%s-rotator" style="transform: translate(-50%%, -50%%) rotate(%sdeg);">%s</div></div>
`, class, class, formatWatermarkFloat(rotation), spans.String())
}

// BlendOverOpaque computes the flat, fully-opaque color that results from
// painting fg at the given opacity over an opaque bg — the pre-blend
// slidelang's --format pptx needs (issue #179): pptxgo's
// Paragraph.Color/drawingml.Color carry no alpha channel, so a true
// translucent watermark isn't expressible there. Drawing the pre-blended
// result BEHIND the slide's content (first shape in the spTree) is
// visually exact as long as the content sitting on top of the watermark is
// opaque — see the pptx fidelity notes in llm-kit/reference/frontmatter.md.
func BlendOverOpaque(fg, bg color.RGBA, opacity float64) color.RGBA {
	opacity = math.Min(1, math.Max(0, opacity))
	blend := func(f, b uint8) uint8 {
		return uint8(math.Round(float64(f)*opacity + float64(b)*(1-opacity)))
	}
	return color.RGBA{
		R: blend(fg.R, bg.R),
		G: blend(fg.G, bg.G),
		B: blend(fg.B, bg.B),
		A: 255,
	}
}
