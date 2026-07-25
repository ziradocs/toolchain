// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// MediaParser handles parsing of embedded audio/video elements (issue #21),
// using the same single-line `<<type ...>>` marker ChartParser uses for
// `<<chart: ...>>` — syntax: `<<video src="..." controls>>`,
// `<<audio src="..." controls autoplay loop muted>>`.
type MediaParser struct{}

// CanParse determina si puede parsear una línea como Media
func (p *MediaParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)
	return matchesMediaTag(trimmed, "video") || matchesMediaTag(trimmed, "audio")
}

// matchesMediaTag reports whether trimmed is a complete, well-formed
// `<<mediaType...>>` marker: it must both open with `<<mediaType` as a
// whole token (not just as a string prefix — `<<video` alone would also
// match `<<videofoo ...>>`, a different, unrelated tag/typo) AND close with
// an actual ">>". A previous version accepted end-of-string, a bare space,
// or a single ">" as if any of those meant the marker was complete, which
// let a truncated/malformed line like `<<video`, `<<video>garbage`, or
// `<<audio controls` (no closing ">>" at all, in every case) be consumed as
// a real MediaElement — typically with an empty Source and, in the
// `<<audio controls` case, Controls silently set to true from a fragment
// that was never a valid attribute list. Requiring a genuine ">>" suffix
// closes both holes: what follows the prefix must be exactly ">>" (the
// bare `<<video>>` form) or start with a space (attributes follow, and the
// line already ends in ">>").
func matchesMediaTag(trimmed, mediaType string) bool {
	prefix := "<<" + mediaType
	if !strings.HasPrefix(trimmed, prefix) || !strings.HasSuffix(trimmed, ">>") {
		return false
	}
	rest := trimmed[len(prefix):]
	return rest == ">>" || strings.HasPrefix(rest, " ")
}

// Parse parsea un elemento de una sola línea `<<video ...>>` / `<<audio ...>>`.
func (p *MediaParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{Error: nil}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	mediaType := "video"
	if strings.HasPrefix(line, "<<audio") {
		mediaType = "audio"
	}

	attrStr := strings.TrimPrefix(line, "<<"+mediaType)
	attrStr = strings.TrimSuffix(strings.TrimSpace(attrStr), ">>")

	media := ast.NewMediaElement(pos, mediaType, extractAttribute(attrStr, "src"))
	media.Controls = hasBooleanAttribute(attrStr, "controls")
	media.Autoplay = hasBooleanAttribute(attrStr, "autoplay")
	media.Loop = hasBooleanAttribute(attrStr, "loop")
	media.Muted = hasBooleanAttribute(attrStr, "muted")

	return &ParseResult{
		Element:       media,
		ConsumedLines: 1,
		Error:         nil,
	}
}

// hasBooleanAttribute reports whether attrName appears as a standalone
// token in str (e.g. "controls" in `src="x.mp4" controls autoplay`) — unlike
// extractAttribute (for `name="value"` attributes), an HTML-style boolean
// attribute carries no value, only its presence matters. Quoted attribute
// values are stripped before tokenizing: without this, a quoted value that
// happens to contain another attribute's name as a word (e.g.
// `src="my controls video.mp4"`) would tokenize to a standalone "controls"
// and be mistaken for that attribute actually being declared.
func hasBooleanAttribute(str, attrName string) bool {
	for _, token := range strings.Fields(stripQuotedAttributeValues(str)) {
		if token == attrName {
			return true
		}
	}
	return false
}

// stripQuotedAttributeValues replaces the contents of every "..."/'...' span
// in str with a single space, so tokenizing what remains can't mistake a
// word inside a quoted attribute value for a separate token.
func stripQuotedAttributeValues(str string) string {
	var b strings.Builder
	var inQuote byte
	for i := 0; i < len(str); i++ {
		c := str[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
