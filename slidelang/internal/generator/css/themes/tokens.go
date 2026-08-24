// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"fmt"
	"regexp"
	"strings"
)

// CanonicalVarName strips the leading "--" and, if present, the
// "slidelang-" namespace, returning the bare name a theme author would
// recognize regardless of which form declared it — e.g. both
// "--primary-color" (embedded Go themes, variables.go) and
// "--slidelang-primary-color" (external theme.json) canonicalize to
// "primary-color". ThemeVariables keys are NOT normalized at storage time;
// GenerateThemeCSS applies the prefix only when emitting CSS. Any code that
// needs to look up a token regardless of which theme authored it must go
// through this, not a raw map index — a lookup against the wrong spelling
// silently returns nothing, and every fallback path then looks like it
// "works" when it's actually just never firing.
func CanonicalVarName(name string) string {
	name = strings.TrimPrefix(name, "--")
	name = strings.TrimPrefix(name, slidelangVarPrefix)
	return name
}

// lookupCanonical finds vars' value for canonicalName, tolerating both the
// "--slidelang-x" and "--x" spellings of the same token.
func lookupCanonical(vars ThemeVariables, canonicalName string) (string, bool) {
	if v, ok := vars["--"+slidelangVarPrefix+canonicalName]; ok {
		return v, true
	}
	if v, ok := vars["--"+canonicalName]; ok {
		return v, true
	}
	return "", false
}

const maxVarResolutionDepth = 12

// parseWholeVarCall reports whether value, once trimmed, is EXACTLY one
// var(--name[, fallback]) call — not a var() reference embedded inside a
// larger expression (e.g. "1px solid var(--x)"). Reuses the same
// balanced-paren/comment-and-string-aware machinery namespace.go's
// NamespaceValue relies on (varTokenRe, findMatchingParen, varInnerRe,
// protectedSpanRe) rather than a second, weaker parser — that machinery
// paid seven adversarial review rounds on PR #223 to get exactly this
// class of CSS parsing right.
func parseWholeVarCall(value string) (name, fallback string, hasFallback, ok bool) {
	loc := varTokenRe.FindStringIndex(value)
	if loc == nil || loc[0] != 0 {
		return "", "", false, false
	}
	openParen := loc[1] - 1
	spans := protectedSpanRe.FindAllStringIndex(value, -1)
	closeParen := findMatchingParen(value, openParen, spans)
	if closeParen == -1 || closeParen != len(value)-1 {
		return "", "", false, false
	}
	inner := value[openParen+1 : closeParen]
	m := varInnerRe.FindStringSubmatchIndex(inner)
	if m == nil {
		return "", "", false, false
	}
	name = inner[m[4]:m[5]]
	hasFallback = m[8] != -1
	if hasFallback {
		fallback = inner[m[10]:m[11]]
	}
	return name, fallback, hasFallback, true
}

// resolveTokenValue expands a token's raw value by following var(--x[,
// fallback]) references against vars until it reaches a literal (no var()
// left) or gives up — needed because a theme's tokens are not guaranteed
// to already be literals: variables.go:120 declares
// "--bg-title-slide": "var(--title-gradient)" even for the built-in
// embedded themes, and a theme.json author can chain --slidelang- tokens
// the same way. A reference cycle (a resolves to b resolves to a) or
// exceeding maxVarResolutionDepth aborts and returns ok=false — the token
// is then treated as ABSENT, never emitted half-resolved, since neither
// Chart.js's canvas fillStyle nor maps.js's marker-color allowlist accept
// a var() reference.
//
// A value that isn't a whole var(...) call is returned as-is UNLESS it
// still mentions "var(" somewhere (e.g. "1px solid var(--x)") — every
// token this resolver serves is a bare color, and a value that doesn't
// take that shape is refused rather than partially flattened.
func resolveTokenValue(vars ThemeVariables, raw string, seen map[string]bool, depth int) (string, bool) {
	value := strings.TrimSpace(raw)
	if depth > maxVarResolutionDepth {
		return "", false
	}
	name, fallback, hasFallback, isWhole := parseWholeVarCall(value)
	if !isWhole {
		if strings.Contains(strings.ToLower(value), "var(") {
			return "", false
		}
		return value, true
	}
	canonical := CanonicalVarName(name)
	if seen[canonical] {
		return "", false // reference cycle
	}
	seen[canonical] = true
	if refValue, ok := lookupCanonical(vars, canonical); ok {
		return resolveTokenValue(vars, refValue, seen, depth+1)
	}
	if hasFallback {
		return resolveTokenValue(vars, fallback, seen, depth+1)
	}
	return "", false
}

// DiagramTokenNames are the diagram-* extension tokens motor-temas-v2.md
// §2.2 defines, propagated to Mermaid's themeVariables (see mermaid.js's
// buildMermaidThemeVariables for the exact mapping table).
var DiagramTokenNames = []string{
	"diagram-node-bg", "diagram-node-fg", "diagram-node-line", "diagram-edge",
	"diagram-edge-label-bg", "diagram-cluster-bg", "diagram-note-bg", "diagram-accent-bg",
}

// chartScalarTokenNames are the chart-* tokens that must reach Chart.js as
// literal JS config values because they color CANVAS-drawn elements
// (gridlines, axis ticks, legend text, tooltip background) — unlike
// chart-surface, which is a plain background behind the canvas and is
// therefore propagated through ordinary CSS instead (see charts.css;
// GenerateThemeCSS already emits ANY variable a theme declares into :root,
// so a dedicated Go/JS path for that one token would just duplicate a
// mechanism that already works).
var chartScalarTokenNames = []string{"chart-grid", "chart-axis", "chart-label", "chart-tooltip-bg"}

// MapTokenNames are the map-* tokens that must reach maps.js as literal
// values because they're written into a marker's inline style string in JS
// (see maps.js) — map-surface is, like chart-surface, plain CSS (see
// maps.css's .slidelang-map-container background) and isn't one of these.
var MapTokenNames = []string{"map-line", "map-label"}

const maxChartCategorical = 8
const maxChartSequential = 5

// ThemeTokens holds every motor-temas-v2.md §2.2 extension token that must
// reach client-side JS as a literal CSS value — never a var() reference,
// since neither Chart.js's canvas fillStyle nor maps.js's marker-color
// allowlist accept one. A token the theme doesn't declare, or one whose
// var() chain doesn't fully resolve (a cycle, a missing reference with no
// fallback, a depth-cap hit, an invalid map color), is simply ABSENT here
// — callers (mermaid.js/charts.js/maps.js) fall back to their existing
// hardcoded defaults for it, never synthesize a value.
//
// chart-surface and map-surface are deliberately NOT here: both are plain
// backgrounds behind a canvas/map container, which ordinary CSS custom
// properties already handle without any Go/JS plumbing (see charts.css and
// maps.css).
type ThemeTokens struct {
	// Diagram maps a bare token name (e.g. "diagram-node-bg") to its
	// resolved literal value.
	Diagram map[string]string `json:"diagram,omitempty"`
	// Chart maps a bare token name (chart-grid/-axis/-label/-tooltip-bg) to
	// its resolved literal value.
	Chart map[string]string `json:"chart,omitempty"`
	// ChartCategorical is chart-cat-1..8, in order, as the contiguous
	// prefix present — an ORDERED set consumed by array index modulo
	// length (colorblind-legible series identity); a gap would silently
	// misalign every index past it, so resolution stops at the first
	// missing or unresolvable entry rather than skipping over it.
	ChartCategorical []string `json:"chartCategorical,omitempty"`
	// ChartSequential is chart-seq-1..5, resolved the same way as
	// ChartCategorical. Exposed for forward-compatibility with the
	// documented §2.2 contract — no chart type in this codebase's Chart.js
	// integration consumes a sequential palette yet, so this PR resolves
	// and carries it but wires no consumer, the same "documented gap, not
	// silent" treatment PR #224 gave the offline/PDF token groups it left
	// unconsumed.
	ChartSequential []string `json:"chartSequential,omitempty"`
	// Map maps a bare token name (map-line/-label) to its resolved
	// literal value, already filtered through IsValidMapColor.
	Map map[string]string `json:"map,omitempty"`
}

// IsEmpty reports whether no §2.2 token resolved to anything — the case
// for every theme in the repo today, since none declares diagram-*/chart-*/
// map-* yet.
func (t ThemeTokens) IsEmpty() bool {
	return len(t.Diagram) == 0 && len(t.Chart) == 0 &&
		len(t.ChartCategorical) == 0 && len(t.ChartSequential) == 0 && len(t.Map) == 0
}

// ResolveThemeTokens resolves every §2.2 extension token in vars to a
// literal value, grouped for its respective JS consumer.
func ResolveThemeTokens(vars ThemeVariables) ThemeTokens {
	return ThemeTokens{
		Diagram:          resolveTokenGroup(vars, DiagramTokenNames),
		Chart:            resolveTokenGroup(vars, chartScalarTokenNames),
		ChartCategorical: resolveOrderedTokens(vars, "chart-cat-", maxChartCategorical),
		ChartSequential:  resolveOrderedTokens(vars, "chart-seq-", maxChartSequential),
		Map:              resolveMapTokenGroup(vars, MapTokenNames),
	}
}

func resolveTokenGroup(vars ThemeVariables, names []string) map[string]string {
	var out map[string]string
	for _, name := range names {
		raw, ok := lookupCanonical(vars, name)
		if !ok {
			continue
		}
		resolved, ok := resolveTokenValue(vars, raw, map[string]bool{}, 0)
		if !ok || resolved == "" {
			continue
		}
		if out == nil {
			out = make(map[string]string, len(names))
		}
		out[name] = resolved
	}
	return out
}

func resolveMapTokenGroup(vars ThemeVariables, names []string) map[string]string {
	group := resolveTokenGroup(vars, names)
	for name, value := range group {
		if !IsValidMapColor(value) {
			delete(group, name)
		}
	}
	if len(group) == 0 {
		return nil
	}
	return group
}

// resolveOrderedTokens resolves a numbered token family (chart-cat-1..N,
// chart-seq-1..N) as a CONTIGUOUS prefix starting at 1, stopping at the
// first missing or unresolvable entry — see ThemeTokens.ChartCategorical's
// doc comment for why a gap can't just be skipped over.
func resolveOrderedTokens(vars ThemeVariables, prefix string, max int) []string {
	var out []string
	for i := 1; i <= max; i++ {
		raw, ok := lookupCanonical(vars, fmt.Sprintf("%s%d", prefix, i))
		if !ok {
			break
		}
		resolved, ok := resolveTokenValue(vars, raw, map[string]bool{}, 0)
		if !ok || resolved == "" {
			break
		}
		out = append(out, resolved)
	}
	return out
}

// ResolveFontMain resolves the theme's font-main variable (--font-main or
// --slidelang-font-main) through any var() chain to a literal — used for
// motor-temas-v2.md §2.2's "font-main" -> Mermaid "fontFamily" mapping.
// Returns "" if absent or the chain doesn't resolve; the caller must fall
// back to Mermaid's own default in that case, never guess a font name. In
// the common case the value is already a literal font stack (e.g. "'Inter',
// sans-serif", not a var() reference at all) — resolveTokenValue returns
// that unchanged, which is exactly what Mermaid's themeVariables.fontFamily
// expects.
func ResolveFontMain(vars ThemeVariables) string {
	raw, ok := lookupCanonical(vars, "font-main")
	if !ok {
		return ""
	}
	resolved, ok := resolveTokenValue(vars, raw, map[string]bool{}, 0)
	if !ok {
		return ""
	}
	return resolved
}

// mapColorNamePattern mirrors maps.js's hexColorPattern exactly, which
// itself mirrors core/renderer/sanitizer.go's hexColorPattern (that
// comment names slidelang-core/renderer/sanitizer.go as the canonical
// source). Duplicated rather than imported: sanitizer.go's version is
// unexported, and exporting one would add a new core symbol this PR's
// scope explicitly avoids (motor-temas-v2.md plan: PR4 pays no
// bump-core.sh). Keep the three copies in sync by hand.
var mapColorNamePattern = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)

// mapNamedColors mirrors maps.js's cssNamedColors / core/renderer/
// sanitizer.go's cssNamedColors exactly — see mapColorNamePattern's doc
// comment for why this is a third hand-kept copy instead of a shared one.
var mapNamedColors = map[string]bool{
	"black": true, "silver": true, "gray": true, "white": true, "maroon": true,
	"red": true, "purple": true, "fuchsia": true, "green": true, "lime": true,
	"olive": true, "yellow": true, "navy": true, "blue": true, "teal": true,
	"aqua": true, "orange": true, "pink": true, "brown": true, "cyan": true,
	"magenta": true, "gold": true, "indigo": true, "violet": true, "coral": true,
	"salmon": true, "khaki": true, "crimson": true, "turquoise": true, "orchid": true,
	"tomato": true, "chocolate": true, "darkgreen": true, "darkblue": true,
	"darkred": true, "lightblue": true, "lightgreen": true, "lightgray": true,
	"lightgrey": true, "darkgray": true, "darkgrey": true, "transparent": true,
}

// IsValidMapColor reports whether value is safe to hand to maps.js as a
// map-line/map-label token: a §2.2 token that fails this check is dropped
// server-side (see resolveMapTokenGroup) instead of shipping a value
// maps.js's own client-side allowlist would just silently reject anyway
// (falling back to its hardcoded '#3388ff'/white) — filtering here keeps
// the metadata payload honest about what actually applies.
func IsValidMapColor(value string) bool {
	return mapColorNamePattern.MatchString(value) || mapNamedColors[strings.ToLower(value)]
}
