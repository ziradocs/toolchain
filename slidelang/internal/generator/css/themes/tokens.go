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
// IN the cycle) is NOT what this reports false for by design: that case
// is a normal "the reference didn't resolve", already handled correctly
// by resolveTokenValue's existing substitution-time logic when it walks
// into that property and fails. propertyIsCyclic only needs to answer, for
// name itself: is name reachable from itself through this graph?
func propertyIsCyclic(vars ThemeVariables, name string, path map[string]bool) bool {
	if path[name] {
		return true
	}
	raw, ok := lookupCanonical(vars, name)
	if !ok {
		return false // an undeclared reference can't be part of a cycle
	}
	path[name] = true
	defer delete(path, name)
	for _, ref := range varReferenceNames(raw) {
		if propertyIsCyclic(vars, ref, path) {
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
		if propertyIsCyclic(vars, name, map[string]bool{}) {
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
		if !IsValidMermaidColor(value) {
			delete(group, name)
		}
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
		if propertyIsCyclic(vars, name, map[string]bool{}) {
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
	if propertyIsCyclic(vars, "font-main", map[string]bool{}) {
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

// cssNumber matches a plain CSS number (no unit), optionally with a
// trailing '%' — used for rgb()/rgba() components (CSS allows either
// 0-255 integers or percentages there) and for rgba()/hsla()'s alpha
// component (CSS Color 4 allows alpha as a fraction or a percentage).
// `[\d.]+` (the pattern this replaced) is not a real number grammar: it
// accepts "1.2.3" and a bare ".", neither of which is valid CSS.
const cssNumber = `(?:\d+(?:\.\d+)?|\.\d+)%?`

// cssPlainNumber is cssNumber with '%' forbidden — hsl()/hsla()'s FIRST
// component (the hue) is a bare angle/number, never a percentage. A
// code-review-flagged gap: cssNumber was reused here too, so
// "hsl(120%,50%,50%)" passed IsValidMermaidColor even though a
// percentage hue is invalid CSS and throws "Unsupported color format" in
// Mermaid exactly like an out-of-grammar value does.
const cssPlainNumber = `(?:\d+(?:\.\d+)?|\.\d+)`

// cssPercentage is cssPlainNumber with '%' mandatory — hsl()/hsla()'s
// saturation and lightness components MUST be percentages; `hsl(120,50,50)`
// (no '%') is invalid CSS and throws "Unsupported color format" in Mermaid,
// exactly like a gradient does.
const cssPercentage = `(?:\d+(?:\.\d+)?|\.\d+)%`

// mermaidFunctionalColorRe matches rgb()/rgba()/hsl()/hsla() — the
// functional color notations Mermaid's theming layer accepts alongside hex
// and named colors, per mermaid.js.org/config/theming.html. One alternative
// per function name, each with that function's exact component count and
// shape (rgb: 3 components; rgba: 4; hsl: hue [plain number, no '%'] + 2
// mandatory percentages; hsla: + alpha) — a shared `{2,3}`-repetition
// pattern (the original version) can't tell rgb from rgba apart, so it
// silently accepted component-count mismatches and let hsl's percentage
// requirement slide both on S/L (no '%') and on the hue (stray '%').
// Deliberately does NOT match gradients, any other CSS <image> syntax, or
// modern space-separated syntax (e.g. "rgb(255 0 0 / 50%)"): Mermaid
// throws "Unsupported color format" for anything outside what this
// pattern accepts (reproduced with diagram-node-bg: linear-gradient(red,
// blue)), so dropping an unrecognized-but-maybe-valid token is the safe
// default — shipping one Mermaid rejects is not.
var mermaidFunctionalColorRe = regexp.MustCompile(`(?i)^(?:` +
	`rgb\(\s*` + cssNumber + `\s*,\s*` + cssNumber + `\s*,\s*` + cssNumber + `\s*\)` +
	`|rgba\(\s*` + cssNumber + `\s*,\s*` + cssNumber + `\s*,\s*` + cssNumber + `\s*,\s*` + cssNumber + `\s*\)` +
	`|hsl\(\s*` + cssPlainNumber + `\s*,\s*` + cssPercentage + `\s*,\s*` + cssPercentage + `\s*\)` +
	`|hsla\(\s*` + cssPlainNumber + `\s*,\s*` + cssPercentage + `\s*,\s*` + cssPercentage + `\s*,\s*` + cssNumber + `\s*\)` +
	`)$`)

// mermaidNamedColors is the full CSS Color Module (Level 3 extended
// keywords + the Level 4 addition "rebeccapurple") named-color set — 148
// names. A code-review-flagged gap: mapNamedColors (41 names, deliberately
// mirroring maps.js's OWN small, hand-picked allowlist) was reused here
// too, so a theme declaring a perfectly valid, common CSS color like
// "aliceblue", "darkslategray", or "rebeccapurple" for a diagram-* token
// got silently dropped even though mermaid@10.9.6 (via khroma, its color
// parser) accepts all of them. Mermaid's acceptance surface is the full
// CSS named-color grammar, not maps.js's narrow allowlist — the two
// consumers have different needs and must not share one list.
var mermaidNamedColors = map[string]bool{
	"aliceblue": true, "antiquewhite": true, "aqua": true, "aquamarine": true, "azure": true,
	"beige": true, "bisque": true, "black": true, "blanchedalmond": true, "blue": true,
	"blueviolet": true, "brown": true, "burlywood": true, "cadetblue": true, "chartreuse": true,
	"chocolate": true, "coral": true, "cornflowerblue": true, "cornsilk": true, "crimson": true,
	"cyan": true, "darkblue": true, "darkcyan": true, "darkgoldenrod": true, "darkgray": true,
	"darkgreen": true, "darkgrey": true, "darkkhaki": true, "darkmagenta": true, "darkolivegreen": true,
	"darkorange": true, "darkorchid": true, "darkred": true, "darksalmon": true, "darkseagreen": true,
	"darkslateblue": true, "darkslategray": true, "darkslategrey": true, "darkturquoise": true, "darkviolet": true,
	"deeppink": true, "deepskyblue": true, "dimgray": true, "dimgrey": true, "dodgerblue": true,
	"firebrick": true, "floralwhite": true, "forestgreen": true, "fuchsia": true, "gainsboro": true,
	"ghostwhite": true, "gold": true, "goldenrod": true, "gray": true, "green": true,
	"greenyellow": true, "grey": true, "honeydew": true, "hotpink": true, "indianred": true,
	"indigo": true, "ivory": true, "khaki": true, "lavender": true, "lavenderblush": true,
	"lawngreen": true, "lemonchiffon": true, "lightblue": true, "lightcoral": true, "lightcyan": true,
	"lightgoldenrodyellow": true, "lightgray": true, "lightgreen": true, "lightgrey": true, "lightpink": true,
	"lightsalmon": true, "lightseagreen": true, "lightskyblue": true, "lightslategray": true, "lightslategrey": true,
	"lightsteelblue": true, "lightyellow": true, "lime": true, "limegreen": true, "linen": true,
	"magenta": true, "maroon": true, "mediumaquamarine": true, "mediumblue": true, "mediumorchid": true,
	"mediumpurple": true, "mediumseagreen": true, "mediumslateblue": true, "mediumspringgreen": true, "mediumturquoise": true,
	"mediumvioletred": true, "midnightblue": true, "mintcream": true, "mistyrose": true, "moccasin": true,
	"navajowhite": true, "navy": true, "oldlace": true, "olive": true, "olivedrab": true,
	"orange": true, "orangered": true, "orchid": true, "palegoldenrod": true, "palegreen": true,
	"paleturquoise": true, "palevioletred": true, "papayawhip": true, "peachpuff": true, "peru": true,
	"pink": true, "plum": true, "powderblue": true, "purple": true, "rebeccapurple": true,
	"red": true, "rosybrown": true, "royalblue": true, "saddlebrown": true, "salmon": true,
	"sandybrown": true, "seagreen": true, "seashell": true, "sienna": true, "silver": true,
	"skyblue": true, "slateblue": true, "slategray": true, "slategrey": true, "snow": true,
	"springgreen": true, "steelblue": true, "tan": true, "teal": true, "thistle": true,
	"tomato": true, "turquoise": true, "violet": true, "wheat": true, "white": true,
	"whitesmoke": true, "yellow": true, "yellowgreen": true, "transparent": true,
}

// IsValidMermaidColor reports whether value is safe to hand to Mermaid's
// themeVariables as a diagram-* token: hex (3/4/6/8 digit), any CSS named
// color, or rgb()/rgba()/hsl()/hsla(). A §2.2 token that fails this check is
// dropped server-side (see resolveDiagramTokenGroup) — see its doc comment
// for why a bad value can't just be shipped and left to fail client-side.
func IsValidMermaidColor(value string) bool {
	value = strings.TrimSpace(value)
	if mapColorNamePattern.MatchString(value) || mermaidNamedColors[strings.ToLower(value)] {
		return true
	}
	return mermaidFunctionalColorRe.MatchString(value)
}
