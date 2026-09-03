// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"regexp"
	"strings"
)

// quotedStringRe matches a single- or double-quoted CSS string — the only
// construct inside a font-family value where a "," is not a stack
// separator (font-family: "Foo, Bar", serif is legal CSS). A dedicated
// pattern, rather than reusing namespace.go's protectedSpanRe (which also
// matches comments and url(), neither valid inside a font-family value),
// keeps this to exactly what a font stack can contain.
var quotedStringRe = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`)

// genericFontFamilyKeywords lists the CSS generic family keywords and
// wide-value keywords that never need a matching Assets.Fonts entry — they
// resolve to whatever the reader's browser/OS considers, e.g., "serif",
// not to a specific typeface a theme could ship.
var genericFontFamilyKeywords = map[string]bool{
	"serif": true, "sans-serif": true, "monospace": true, "cursive": true,
	"fantasy": true, "system-ui": true, "ui-serif": true, "ui-sans-serif": true,
	"ui-monospace": true, "ui-rounded": true, "math": true, "emoji": true,
	"fangsong": true, "inherit": true, "initial": true, "unset": true, "revert": true,
}

// SplitFontStack splits a CSS font-family value into its comma-separated
// entries, respecting commas inside quoted family names — a naive
// strings.Split(stack, ",") breaks on font-family: "Foo, Bar", serif,
// treating `"Foo` and ` Bar"` as two separate entries. #223's namespacing
// engine paid seven adversarial review rounds finding exactly this class of
// bug in CSS string handling; this reuses the same "skip past quoted spans"
// technique rather than writing a new one. Each returned entry is trimmed
// of surrounding whitespace and quotes are preserved (callers need them to
// tell a quoted family name apart from a bare keyword like serif). Empty
// entries (e.g. a trailing comma, or an empty stack) are dropped.
func SplitFontStack(stack string) []string {
	spans := quotedStringRe.FindAllStringIndex(stack, -1)
	var entries []string
	start := 0
	spanIdx := 0
	for i := 0; i < len(stack); i++ {
		if spanIdx < len(spans) && i == spans[spanIdx][0] {
			i = spans[spanIdx][1] - 1
			spanIdx++
			continue
		}
		if stack[i] == ',' {
			entries = append(entries, strings.TrimSpace(stack[start:i]))
			start = i + 1
		}
	}
	entries = append(entries, strings.TrimSpace(stack[start:]))

	out := entries[:0]
	for _, e := range entries {
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

// unquoteFontFamily strips the surrounding quotes from a single font-family
// stack entry (as returned by SplitFontStack) and unescapes \" and \\, so
// it can be compared against a plain ThemeFont.Name. A bare keyword like
// serif is returned unchanged.
func unquoteFontFamily(entry string) string {
	entry = strings.TrimSpace(entry)
	if len(entry) < 2 {
		return entry
	}
	quote := entry[0]
	if (quote != '"' && quote != '\'') || entry[len(entry)-1] != quote {
		return entry
	}
	inner := entry[1 : len(entry)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] == '\\' && i+1 < len(inner) {
			i++
		}
		b.WriteByte(inner[i])
	}
	return b.String()
}
