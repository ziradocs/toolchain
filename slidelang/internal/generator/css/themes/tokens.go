// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
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
// the same way. Exceeding maxVarResolutionDepth aborts with ok=false — the
// token is then treated as ABSENT, never emitted half-resolved, since
// neither Chart.js's canvas fillStyle nor maps.js's marker-color allowlist
// accept a var() reference.
//
// owner is the canonical name of the custom property whose declared value
// IS raw — e.g. for the top-level call from resolveTokenGroup, raw is
// vars[owner]. Pass "" for an anonymous, non-property expression (a
// consumer like "background: var(--x, red)" that isn't itself a custom
// property) — this only matters for cycle handling, see cycleRoot below.
//
// A value that isn't a whole var(...) call is returned as-is UNLESS it
// still mentions "var(" somewhere (e.g. "1px solid var(--x)") — every
// token this resolver serves is a bare color, and a value that doesn't
// take that shape is refused rather than partially flattened.
//
// cycleRoot is non-empty only when ok is false AND the failure is a
// reference cycle (as opposed to a missing reference or a depth-cap hit),
// naming the canonical property where the cycle closes. This distinction
// exists because CSS's own var() cycle semantics are stricter than a plain
// "missing reference": per CSS Custom Properties §3, EVERY custom property
// that participates in a cycle computes to its guaranteed-invalid value,
// even one only reached through a would-be-rescuing fallback — so a frame
// whose own property IS part of the cycle must not apply its own
// fallback. Only once unwinding reaches the frame whose owner matches the
// cycleRoot does the cycle "close": that frame clears the signal, so
// whatever's above it (now outside the cycle, e.g. an anonymous consumer
// with owner="") is free to use ITS OWN fallback again — this is what
// makes "var(--a, #fff)" against "--a: var(--a)" still yield "#fff": the
// anonymous consumer (owner="") can never equal a cycleRoot (always a real
// property name), so it always gets to fall back once the cycle beneath it
// fails.
func resolveTokenValue(vars ThemeVariables, raw string, seen map[string]bool, owner string, depth int) (value, cycleRoot string, ok bool) {
	trimmed := strings.TrimSpace(raw)
	if depth > maxVarResolutionDepth {
		return "", "", false
	}
	name, fallback, hasFallback, isWhole := parseWholeVarCall(trimmed)
	if !isWhole {
		if strings.Contains(strings.ToLower(trimmed), "var(") {
			return "", "", false
		}
		return trimmed, "", true
	}
	canonical := CanonicalVarName(name)

	if seen[canonical] {
		cycleRoot = canonical
	} else {
		seen[canonical] = true
		if refValue, refOK := lookupCanonical(vars, canonical); refOK {
			var resolved string
			var resolvedOK bool
			resolved, cycleRoot, resolvedOK = resolveTokenValue(vars, refValue, seen, canonical, depth+1)
			if resolvedOK {
				delete(seen, canonical)
				return resolved, "", true
			}
		}
		// seen is a path stack, not an accumulated set: pop before trying
		// this var() call's own fallback below, so a sibling branch (the
		// fallback) can freely reference canonical again without a false
		// "cycle" against a path we've already left.
		delete(seen, canonical)
	}

	if cycleRoot != "" {
		if cycleRoot == owner {
			// The cycle closes exactly at this frame's own property.
			// hasFallback is deliberately not consulted here — this
			// frame's fallback is exactly as guaranteed-invalid as its
			// main value.
			return "", "", false
		}
		// Still inside a cycle that closes elsewhere — this frame's
		// fallback is equally invalid; propagate without trying it.
		return "", cycleRoot, false
	}

	if hasFallback {
		return resolveTokenValue(vars, fallback, seen, owner, depth+1)
	}
	return "", "", false
}

// varReferenceNames returns the canonical names of every var() reference a
// value structurally contains — the primary reference AND, recursively,
// any reference nested inside its fallback (a fallback can itself be
// "var(--y, var(--z))", chaining further). A value outside the shape
// parseWholeVarCall recognizes (a literal, or a var() embedded in a larger
// expression like "1px solid var(--x)") contributes no references —
// consistent with resolveTokenValue's own refusal to interpret those.
func varReferenceNames(value string) []string {
	name, fallback, hasFallback, isWhole := parseWholeVarCall(strings.TrimSpace(value))
	if !isWhole {
		return nil
	}
	names := []string{CanonicalVarName(name)}
	if hasFallback {
		names = append(names, varReferenceNames(fallback)...)
	}
	return names
}

// propertyIsCyclic reports whether canonical property name structurally
// participates in a var() reference cycle — CSS Custom Properties §3's
// "guaranteed-invalid" condition, computed as a PRE-PASS before any value
// substitution and independent of it.
//
// This exists because resolveTokenValue's own cycle detection (the `seen`
// map, checked during substitution) only walks the branch actually
// substituted at runtime — it recurses into a var() call's fallback ONLY
// when the primary reference fails. That misses a cycle hidden entirely
// inside a fallback that never gets visited because the primary reference
// resolves fine: "--a: var(--defined, var(--a))" with "--defined: #123456"
// resolves --defined successfully and, before this fix, never looked at
// the fallback "var(--a)" at all — but per spec the dependency graph
// includes EVERY var() reference a property's value contains, fallback or
// not, regardless of whether it's ever actually evaluated. --a
// self-references through its own fallback, so --a is cyclic —
// unconditionally, not only when the primary reference happens to fail
// too.
//
// A property merely REFERENCING a cyclic property (without itself being
// IN the cycle) must report false, not true: that case is a normal "the
// reference didn't resolve", already handled correctly by
// resolveTokenValue's existing substitution-time logic when it walks into
// that property and fails (and, critically, still gets to try ITS OWN
// fallback afterward). propertyIsCyclic answers, for name specifically:
// is name reachable from itself through this graph — NOT "can name reach
// some node that happens to be revisited during the walk".
//
// A code-review-flagged correctness bug lived in an earlier version of
// this function: it treated ANY revisited node during the DFS as proof of
// a cycle, not just a revisit of name itself. That conflates two
// different things — "this graph contains a cycle somewhere reachable
// from name" (true whenever name transitively reaches ANY cyclic pair)
// with "name itself is part of a cycle" (only true when the walk loops
// back to name specifically). Repro: chart-cat-1: var(--b, red), --b:
// var(--c), --c: var(--b) — b and c form a real cycle, but chart-cat-1
// merely references b once and is not itself part of that loop, so per
// CSS it must resolve via its own fallback to "red". The buggy version
// walked chart-cat-1 -> b -> c -> b (revisiting b, not chart-cat-1) and
// reported chart-cat-1 cyclic anyway, discarding "red" for nothing.
func propertyIsCyclic(vars ThemeVariables, start string) bool {
	return propertyReaches(vars, start, start, map[string]bool{})
}

// propertyReaches walks current's declared value looking for a path back
// to start — see propertyIsCyclic's doc comment for why this must check
// specifically for start, not for "any name already in path". Revisiting
// some OTHER ancestor (a cycle elsewhere in the graph, not looping back
// to start) stops that branch without flagging start: continuing would
// only ever oscillate among those other nodes, never reach start, since a
// second, disjoint cycle can't also pass through start without start
// already being on path.
func propertyReaches(vars ThemeVariables, start, current string, path map[string]bool) bool {
	if path[current] {
		return current == start
	}
	raw, ok := lookupCanonical(vars, current)
	if !ok {
		return false // an undeclared reference can't be part of a cycle
	}
	path[current] = true
	defer delete(path, current)
	for _, ref := range varReferenceNames(raw) {
		if propertyReaches(vars, start, ref, path) {
			return true
		}
	}
	return false
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
		Diagram:          resolveDiagramTokenGroup(vars, DiagramTokenNames),
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
		// propertyIsCyclic is a pre-pass, not redundant with the `seen`-based
		// tracking resolveTokenValue does during substitution below — see
		// its doc comment for exactly which case it catches that
		// substitution-time tracking structurally can't (a cycle hidden
		// inside a fallback branch that never gets visited because the
		// primary reference resolves fine).
		if propertyIsCyclic(vars, name) {
			continue
		}
		resolved, _, ok := resolveTokenValue(vars, raw, map[string]bool{name: true}, name, 0)
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

// resolveDiagramTokenGroup filters diagram-* tokens through
// IsValidMermaidColor before they're allowed to reach mermaid.js's
// themeVariables — unlike Chart.js's canvas fillStyle (which silently
// ignores a string it can't parse as a color), Mermaid's own theming layer
// throws "Unsupported color format" for a value it rejects (e.g. a
// gradient or a bare var() that slipped through resolution), which aborts
// mermaid.initialize() for the whole page. Same "drop rather than ship a
// value the consumer would reject anyway" treatment as resolveMapTokenGroup.
// Verified against the exact Mermaid build this toolchain embeds —
// MermaidCDNScriptTag (core/renderer/cdn_tags.go) pins mermaid@10.9.6,
// which parses theme colors with khroma (same parser through at least
// Mermaid 11.x): hex accepts only 3/4/6/8 digits, and hsl()/hsla() require
// '%' on saturation/lightness.
func resolveDiagramTokenGroup(vars ThemeVariables, names []string) map[string]string {
	group := resolveTokenGroup(vars, names)
	for name, value := range group {
		normalized, ok := normalizeMermaidColor(value)
		if !ok {
			delete(group, name)
			continue
		}
		group[name] = normalized
	}
	if len(group) == 0 {
		return nil
	}
	return group
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
		name := fmt.Sprintf("%s%d", prefix, i)
		raw, ok := lookupCanonical(vars, name)
		if !ok {
			break
		}
		if propertyIsCyclic(vars, name) {
			break
		}
		resolved, _, ok := resolveTokenValue(vars, raw, map[string]bool{name: true}, name, 0)
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
	if propertyIsCyclic(vars, "font-main") {
		return ""
	}
	resolved, _, ok := resolveTokenValue(vars, raw, map[string]bool{"font-main": true}, "font-main", 0)
	if !ok {
		return ""
	}
	return resolved
}

// mapColorNamePattern is DELIBERATELY stricter than maps.js's
// hexColorPattern (`{3,8}`, accepting the invalid lengths 5 and 7) and
// core/renderer/sanitizer.go's own copy — `{3,4}|{6}|{8}` is the actual
// set of valid CSS hex-color lengths (#rgb, #rgba, #rrggbb, #rrggbbaa).
// This asymmetry is one-way and safe: an invalid 5/7-digit hex reaching
// maps.js's client-side allowlist just gets ignored (falls back to
// '#3388ff'), but the same value reaching Mermaid's themeVariables (see
// IsValidMermaidColor below, which reuses this pattern) throws
// "Unsupported color format" inside mermaid.initialize() and aborts the
// whole module — so the server-side gate has to be the strict one. Keep
// the other two copies in sync with each other; this one is intentionally
// not identical to them.
var mapColorNamePattern = regexp.MustCompile(`^#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

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

// functionalColorCallRe splits a functional color notation into its
// function name (rgb/rgba/hsl/hsla — CSS Color 4 makes each pair true
// aliases, so the name doesn't fix the arity) and its raw argument list.
// Everything about WHICH numbers are valid and how many there are is
// handled by normalizeFunctionalColor itself, not by this regex — see its
// doc comment for why. Deliberately does NOT match gradients or any other
// CSS <image> syntax — Mermaid throws "Unsupported color format" for
// those (reproduced with diagram-node-bg: linear-gradient(red, blue)), so
// falling through to "invalid" for anything that isn't `name(...)` shaped
// remains the safe default.
var functionalColorCallRe = regexp.MustCompile(`(?i)^(rgba?|hsla?)\(\s*(.*?)\s*\)$`)

// cssUnitSuffixes lists the unit suffixes parseCSSNumber recognizes, in an
// order where no earlier entry is a suffix of a later one's remaining
// text — "grad" must be checked before "rad", since "60grad" itself ends
// in the three characters "rad".
var cssUnitSuffixes = []string{"grad", "turn", "deg", "rad", "%"}

// cssNumberLexemeRe matches the CSS <number-token> grammar (CSS Syntax
// Level 3 §4.3.3) and nothing else: an optional sign, digits with an
// optional fractional part (a '.' must be followed by at least one
// digit — "10." is not a CSS number), and an optional decimal exponent.
//
// It exists because strconv.ParseFloat implements GO's float literal
// grammar, not CSS's, and the two are different languages: ParseFloat
// accepts "NaN", "Inf", "Infinity", hex-float literals ("0x1p2") and
// digit separators ("1_0"), returning NO error for any of them. A
// code-review finding showed this was not merely an over-permissive
// filter but a correctness bug: "rgb(NaN 0 0)" produced a NaN component,
// converting NaN to an int is undefined in Go, and the formatter then
// emitted a platform-dependent garbage string ("#-80000000000000000000"
// on linux/amd64) as if it were a hex color — handing mermaid.js exactly
// the kind of unparseable value this whole normalization step exists to
// prevent, with an init abort as the possible outcome. Validating the
// lexeme BEFORE parsing is what closes that; checking ParseFloat's error
// alone cannot.
var cssNumberLexemeRe = regexp.MustCompile(`^[+-]?(?:[0-9]+|[0-9]*\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

// parseCSSNumber parses a single CSS numeric token: a CSS number (see
// cssNumberLexemeRe) with an optional trailing '%' or CSS angle unit
// (deg/grad/rad/turn). A code-review finding confirmed mermaid@10.9.6
// accepts negative and scientific-notation numbers in rgb()/hsl()
// components ("rgb(-10 0 0)", "rgb(1e2 0 0)") that an earlier
// regex-based grammar (`[\d.]+`-shaped, no sign, no exponent) rejected
// outright — parsing the number for real, the way any CSS engine does,
// is what closes that gap instead of chasing one more hand-written
// pattern.
func parseCSSNumber(raw string) (value float64, percent bool, unit string, ok bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false, "", false
	}
	lower := strings.ToLower(trimmed)
	for _, suffix := range cssUnitSuffixes {
		if !strings.HasSuffix(lower, suffix) {
			continue
		}
		v, numOK := parseCSSNumberLexeme(trimmed[:len(trimmed)-len(suffix)])
		if !numOK {
			return 0, false, "", false
		}
		if suffix == "%" {
			return v, true, "", true
		}
		return v, false, suffix, true
	}
	v, numOK := parseCSSNumberLexeme(trimmed)
	if !numOK {
		return 0, false, "", false
	}
	return v, false, "", true
}

// parseCSSNumberLexeme validates raw against the CSS number grammar and
// only then converts it. The order is load-bearing — see
// cssNumberLexemeRe's doc comment for the bug that comes from trusting
// strconv.ParseFloat's error as the only gate.
//
// It deliberately does NOT trim raw. Its callers hand it either a
// whole already-trimmed component or that component minus a unit suffix,
// so any whitespace still present at this point sits BETWEEN the number
// and its unit — which CSS does not allow, because a dimension like
// "10%" or "30deg" is a single token, not two. A code-review finding
// caught an earlier version trimming here anyway, which quietly undid
// the lexeme check for every dimension: "rgb(10 %,0,0)" was accepted as
// 10%, "hsl(30 deg,50%,50%)" as 30deg. Trimming here would also
// contradict this same grammar's rejection of "10." — there is no
// reading under which "10 %" is valid CSS but "10." is not.
func parseCSSNumberLexeme(raw string) (float64, bool) {
	if !cssNumberLexemeRe.MatchString(raw) {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false
	}
	// ErrRange is the one error a VALID CSS lexeme can still produce
	// ("1e400"), and ParseFloat already returns the saturated value for
	// it — which is exactly the clamp CSS asks for on an out-of-range
	// component, so it is accepted rather than dropped.
	if math.IsNaN(v) {
		// Unreachable while the lexeme check above stands: no CSS
		// number lexeme parses to NaN. Kept so that loosening that
		// grammar later cannot silently reintroduce the garbage-hex
		// output described in cssNumberLexemeRe's doc comment.
		return 0, false
	}
	return v, true
}

// splitFunctionalArgs parses a functional color's raw argument text into
// its (up to 3) color components plus an optional alpha, accepting both
// legacy comma syntax (3 components, or 4 with alpha as the last comma
// item — regardless of whether the function was spelled with or without
// the trailing 'a', since CSS Color 4 makes rgb/rgba and hsl/hsla true
// aliases: a code-review finding confirmed mermaid@10.9.6 accepts both
// "rgba(10,20,30)" with 3 comma components and "rgb(10,20,30,0.5)" with
// 4, neither of which an earlier strict-per-name-arity grammar allowed)
// and modern space syntax (3 components, optionally followed by "/
// alpha").
func splitFunctionalArgs(args string) (components []string, alpha string, ok bool) {
	if idx := strings.LastIndex(args, "/"); idx != -1 {
		main := splitComponents(strings.TrimSpace(args[:idx]))
		alphaPart := strings.TrimSpace(args[idx+1:])
		if len(main) != 3 || alphaPart == "" {
			return nil, "", false
		}
		return main, alphaPart, true
	}
	if strings.Contains(args, ",") {
		parts := strings.Split(args, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		switch len(parts) {
		case 3:
			return parts, "", true
		case 4:
			return parts[:3], parts[3], true
		default:
			return nil, "", false
		}
	}
	parts := strings.Fields(args)
	if len(parts) != 3 {
		return nil, "", false
	}
	return parts, "", true
}

func splitComponents(s string) []string {
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	return strings.Fields(s)
}

// rgbComponent parses one rgb()/rgba() red/green/blue component: a plain
// number (0-255) or a percentage (0%-100%), clamped to the valid byte
// range per CSS's out-of-range handling — a component can't itself carry
// an angle unit.
func rgbComponent(raw string) (float64, bool) {
	v, percent, unit, ok := parseCSSNumber(raw)
	if !ok || unit != "" {
		return 0, false
	}
	if percent {
		v = v / 100 * 255
	}
	return math.Max(0, math.Min(255, v)), true
}

// alphaComponent parses an alpha value: a plain number (0-1) or a
// percentage (0%-100%), clamped — same out-of-range handling as any other
// CSS Color 4 component.
func alphaComponent(raw string) (float64, bool) {
	v, percent, unit, ok := parseCSSNumber(raw)
	if !ok || unit != "" {
		return 0, false
	}
	if percent {
		v /= 100
	}
	return math.Max(0, math.Min(1, v)), true
}

// hueDegrees parses an hsl()/hsla() hue component into degrees in
// [0,360): a signed number, optionally carrying a CSS angle unit
// (deg/grad/rad/turn) — confirmed accepted by the exact Mermaid build
// this toolchain embeds (mermaid@10.9.6, via khroma; a code-review
// finding): "hsl(-30,50%,50%)" and "hsl(0.5turn,50%,50%)" both parse.
// Never a percentage — a percentage hue is invalid CSS regardless of sign
// or unit.
// A non-finite hue is rejected outright, before AND after the unit
// conversion, unlike every other component — those live on a bounded
// scale where saturating IS the CSS-defined clamp (an overflowing
// "rgb(1e400 0 0)" is simply the reddest red), but a hue is an angle on a
// circle and ±Inf has no residue mod 360: math.Mod(±Inf, 360) is NaN.
// A code-review finding traced where that NaN ended up: hslToRGB's
// comparison chain is all-false for NaN, so it fell through to the
// default branch and returned a perfectly finite 0 for every channel —
// "hsl(1e400 100% 50%)" silently rendered BLACK instead of being
// dropped, and normalizeFunctionalColor's allFinite guard could not see
// it because by then the NaN was gone. Both checks are needed: the first
// catches an overflowing literal (1e400), the second catches a finite
// literal that overflows only once scaled into degrees (1e308turn,
// 1e308rad).
func hueDegrees(raw string) (float64, bool) {
	v, percent, unit, ok := parseCSSNumber(raw)
	if !ok || percent || !allFinite(v) {
		return 0, false
	}
	switch unit {
	case "", "deg":
	case "grad":
		v *= 0.9
	case "rad":
		v *= 180 / math.Pi
	case "turn":
		v *= 360
	}
	if !allFinite(v) {
		return 0, false
	}
	v = math.Mod(v, 360)
	if v < 0 {
		v += 360
	}
	return v, true
}

// percentComponent parses an hsl()/hsla() saturation or lightness
// component: MUST be a percentage — `hsl(120,50,50)` (no '%') is invalid
// CSS and throws "Unsupported color format" in Mermaid, exactly like a
// gradient does — clamped to 0-100.
func percentComponent(raw string) (float64, bool) {
	v, percent, unit, ok := parseCSSNumber(raw)
	if !ok || !percent || unit != "" {
		return 0, false
	}
	return math.Max(0, math.Min(100, v)), true
}

// hslToRGB converts an hsl() triple (h in degrees, s and l as 0-1
// fractions) to 0-255 RGB bytes, via the standard CSS Color reference
// algorithm.
func hslToRGB(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		v := l * 255
		return v, v, v
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	hk := h / 360
	return hueToRGBComponent(p, q, hk+1.0/3) * 255,
		hueToRGBComponent(p, q, hk) * 255,
		hueToRGBComponent(p, q, hk-1.0/3) * 255
}

func hueToRGBComponent(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 1.0/2:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	default:
		return p
	}
}

// normalizeFunctionalColor parses an rgb()/rgba()/hsl()/hsla() value —
// legacy comma syntax or modern CSS Color 4 space syntax, either function
// name for either arity, any component grammar a real CSS number allows
// (sign, scientific notation, out-of-range values that clamp rather than
// fail) — and converts it to a hex string. This mirrors
// normalizeMermaidColor's own strategy for named colors: rather than
// keep hand-extending a regex to match Mermaid's actual accepted
// grammar (a gap three separate review rounds found new instances of —
// see cssNamedColorHex's doc comment for the analogous history on the
// named-color side), parse the value as real CSS and emit hex, which
// every real color parser (khroma included) accepts unconditionally
// regardless of which functional syntax the theme author used.
func normalizeFunctionalColor(value string) (string, bool) {
	match := functionalColorCallRe.FindStringSubmatch(value)
	if match == nil {
		return "", false
	}
	fn := strings.ToLower(match[1])
	components, alphaRaw, ok := splitFunctionalArgs(match[2])
	if !ok {
		return "", false
	}

	var r, g, b float64
	switch fn {
	case "rgb", "rgba":
		var ok1, ok2, ok3 bool
		r, ok1 = rgbComponent(components[0])
		g, ok2 = rgbComponent(components[1])
		b, ok3 = rgbComponent(components[2])
		if !ok1 || !ok2 || !ok3 {
			return "", false
		}
	case "hsl", "hsla":
		h, okH := hueDegrees(components[0])
		s, okS := percentComponent(components[1])
		l, okL := percentComponent(components[2])
		if !okH || !okS || !okL {
			return "", false
		}
		r, g, b = hslToRGB(h, s/100, l/100)
	default:
		return "", false
	}

	// Fail closed on anything non-finite before formatting. round255
	// converts to int, and converting a NaN to an int is UNDEFINED in Go:
	// it yields a platform-dependent garbage integer, which %02X then
	// renders into a "#..." string that is not a hex color at all
	// ("#-80000000000000000000" on linux/amd64) — precisely the
	// unparseable value handed to mermaid.js that a code-review finding
	// reproduced. parseCSSNumber's lexeme validation is what actually
	// prevents a NaN from ever getting here; this is the output-boundary
	// invariant (never emit a non-hex string) that keeps any future path
	// from reintroducing it silently.
	if !allFinite(r, g, b) {
		return "", false
	}
	if alphaRaw == "" {
		return fmt.Sprintf("#%02X%02X%02X", round255(r), round255(g), round255(b)), true
	}
	a, ok := alphaComponent(alphaRaw)
	if !ok || !allFinite(a) {
		return "", false
	}
	return fmt.Sprintf("#%02X%02X%02X%02X", round255(r), round255(g), round255(b), round255(a*255)), true
}

func allFinite(values ...float64) bool {
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func round255(v float64) int {
	return int(math.Round(math.Max(0, math.Min(255, v))))
}

// cssNamedColorHex maps every CSS Color Module named color (Level 3
// extended keywords + the Level 4 addition "rebeccapurple" + the special
// keyword "transparent") to its hex equivalent — 149 entries (147 CSS3 extended keywords + the CSS4 addition "rebeccapurple" + the special keyword "transparent").
//
// A code-review finding drove this design, across two rounds: first,
// mapNamedColors (41 names, deliberately mirroring maps.js's OWN small,
// hand-picked allowlist) was reused here, silently dropping a valid CSS
// color like "aliceblue" or "rebeccapurple". Replacing it with a
// standalone bool-valued list fixed that,
// but a SECOND finding showed the deeper problem: mermaid@10.9.6's own
// color parser (khroma) does NOT necessarily recognize every standard CSS
// named color either — confirmed empirically that "cyan", "steelblue",
// and "tomato" (all valid CSS, all in this list) still throw
// "Unsupported color format" against the exact pinned bundle. Chasing
// khroma's own keyword-completeness by hand is exactly the kind of gap
// this keeps rediscovering. The robust fix (the reviewer's own
// suggestion): STOP shipping named colors to Mermaid at all. Every named
// color a theme declares is normalized to its hex equivalent here,
// before it ever reaches mermaid.js — hex parsing is universal and
// unambiguous in every real CSS color parser (khroma included), so
// nothing downstream needs to match a name against ANY keyword table
// ever again. See normalizeMermaidColor, the function that actually
// performs this substitution.
var cssNamedColorHex = map[string]string{
	"aliceblue": "#F0F8FF", "antiquewhite": "#FAEBD7", "aqua": "#00FFFF", "aquamarine": "#7FFFD4", "azure": "#F0FFFF",
	"beige": "#F5F5DC", "bisque": "#FFE4C4", "black": "#000000", "blanchedalmond": "#FFEBCD", "blue": "#0000FF",
	"blueviolet": "#8A2BE2", "brown": "#A52A2A", "burlywood": "#DEB887", "cadetblue": "#5F9EA0", "chartreuse": "#7FFF00",
	"chocolate": "#D2691E", "coral": "#FF7F50", "cornflowerblue": "#6495ED", "cornsilk": "#FFF8DC", "crimson": "#DC143C",
	"cyan": "#00FFFF", "darkblue": "#00008B", "darkcyan": "#008B8B", "darkgoldenrod": "#B8860B", "darkgray": "#A9A9A9",
	"darkgreen": "#006400", "darkgrey": "#A9A9A9", "darkkhaki": "#BDB76B", "darkmagenta": "#8B008B", "darkolivegreen": "#556B2F",
	"darkorange": "#FF8C00", "darkorchid": "#9932CC", "darkred": "#8B0000", "darksalmon": "#E9967A", "darkseagreen": "#8FBC8F",
	"darkslateblue": "#483D8B", "darkslategray": "#2F4F4F", "darkslategrey": "#2F4F4F", "darkturquoise": "#00CED1", "darkviolet": "#9400D3",
	"deeppink": "#FF1493", "deepskyblue": "#00BFFF", "dimgray": "#696969", "dimgrey": "#696969", "dodgerblue": "#1E90FF",
	"firebrick": "#B22222", "floralwhite": "#FFFAF0", "forestgreen": "#228B22", "fuchsia": "#FF00FF", "gainsboro": "#DCDCDC",
	"ghostwhite": "#F8F8FF", "gold": "#FFD700", "goldenrod": "#DAA520", "gray": "#808080", "green": "#008000",
	"greenyellow": "#ADFF2F", "grey": "#808080", "honeydew": "#F0FFF0", "hotpink": "#FF69B4", "indianred": "#CD5C5C",
	"indigo": "#4B0082", "ivory": "#FFFFF0", "khaki": "#F0E68C", "lavender": "#E6E6FA", "lavenderblush": "#FFF0F5",
	"lawngreen": "#7CFC00", "lemonchiffon": "#FFFACD", "lightblue": "#ADD8E6", "lightcoral": "#F08080", "lightcyan": "#E0FFFF",
	"lightgoldenrodyellow": "#FAFAD2", "lightgray": "#D3D3D3", "lightgreen": "#90EE90", "lightgrey": "#D3D3D3", "lightpink": "#FFB6C1",
	"lightsalmon": "#FFA07A", "lightseagreen": "#20B2AA", "lightskyblue": "#87CEFA", "lightslategray": "#778899", "lightslategrey": "#778899",
	"lightsteelblue": "#B0C4DE", "lightyellow": "#FFFFE0", "lime": "#00FF00", "limegreen": "#32CD32", "linen": "#FAF0E6",
	"magenta": "#FF00FF", "maroon": "#800000", "mediumaquamarine": "#66CDAA", "mediumblue": "#0000CD", "mediumorchid": "#BA55D3",
	"mediumpurple": "#9370DB", "mediumseagreen": "#3CB371", "mediumslateblue": "#7B68EE", "mediumspringgreen": "#00FA9A", "mediumturquoise": "#48D1CC",
	"mediumvioletred": "#C71585", "midnightblue": "#191970", "mintcream": "#F5FFFA", "mistyrose": "#FFE4E1", "moccasin": "#FFE4B5",
	"navajowhite": "#FFDEAD", "navy": "#000080", "oldlace": "#FDF5E6", "olive": "#808000", "olivedrab": "#6B8E23",
	"orange": "#FFA500", "orangered": "#FF4500", "orchid": "#DA70D6", "palegoldenrod": "#EEE8AA", "palegreen": "#98FB98",
	"paleturquoise": "#AFEEEE", "palevioletred": "#DB7093", "papayawhip": "#FFEFD5", "peachpuff": "#FFDAB9", "peru": "#CD853F",
	"pink": "#FFC0CB", "plum": "#DDA0DD", "powderblue": "#B0E0E6", "purple": "#800080", "rebeccapurple": "#663399",
	"red": "#FF0000", "rosybrown": "#BC8F8F", "royalblue": "#4169E1", "saddlebrown": "#8B4513", "salmon": "#FA8072",
	"sandybrown": "#F4A460", "seagreen": "#2E8B57", "seashell": "#FFF5EE", "sienna": "#A0522D", "silver": "#C0C0C0",
	"skyblue": "#87CEEB", "slateblue": "#6A5ACD", "slategray": "#708090", "slategrey": "#708090", "snow": "#FFFAFA",
	"springgreen": "#00FF7F", "steelblue": "#4682B4", "tan": "#D2B48C", "teal": "#008080", "thistle": "#D8BFD8",
	"tomato": "#FF6347", "turquoise": "#40E0D0", "violet": "#EE82EE", "wheat": "#F5DEB3", "white": "#FFFFFF",
	"whitesmoke": "#F5F5F5", "yellow": "#FFFF00", "yellowgreen": "#9ACD32",
	// "transparent" has no fully-opaque hex equivalent — an 8-digit hex
	// with a zero alpha byte is the correct encoding, universally parsed
	// (unlike the bare keyword, whose khroma acceptance is exactly what
	// this whole normalization step exists to stop depending on).
	"transparent": "#00000000",
}

// IsValidMermaidColor reports whether value is safe to hand to Mermaid's
// themeVariables as a diagram-* token: hex (3/4/6/8 digit), any CSS named
// color, or rgb()/rgba()/hsl()/hsla(). A §2.2 token that fails this check is
// dropped server-side (see resolveDiagramTokenGroup) — see its doc comment
// for why a bad value can't just be shipped and left to fail client-side.
// Named colors pass this check based on the full standard CSS keyword set,
// NOT on whatever subset khroma happens to implement — see
// normalizeMermaidColor, which is what actually makes that safe to do.
func IsValidMermaidColor(value string) bool {
	_, ok := normalizeMermaidColor(value)
	return ok
}

// normalizeMermaidColor validates value the same way IsValidMermaidColor
// does, but returns the value Mermaid should actually receive: both a
// named color (cssNamedColorHex) and a functional rgb()/rgba()/hsl()/
// hsla() value (normalizeFunctionalColor) are rewritten to their hex
// equivalent rather than passed through, so mermaid@10.9.6's own
// khroma-based parser never has to recognize the original syntax at all
// — only hex, which every real CSS color parser (khroma included)
// supports unconditionally. Only hex values already in that form are
// returned unchanged.
func normalizeMermaidColor(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if mapColorNamePattern.MatchString(trimmed) {
		return trimmed, true
	}
	if hex, ok := cssNamedColorHex[strings.ToLower(trimmed)]; ok {
		return hex, true
	}
	if hex, ok := normalizeFunctionalColor(trimmed); ok {
		return hex, true
	}
	return "", false
}
