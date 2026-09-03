// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCanonicalVarName(t *testing.T) {
	cases := map[string]string{
		"--primary-color":             "primary-color",
		"--slidelang-primary-color":   "primary-color",
		"primary-color":               "primary-color",
		"slidelang-primary-color":     "primary-color",
		"--diagram-node-bg":           "diagram-node-bg",
		"--slidelang-diagram-node-bg": "diagram-node-bg",
	}
	for in, want := range cases {
		if got := CanonicalVarName(in); got != want {
			t.Errorf("CanonicalVarName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveTokenValue_Literal(t *testing.T) {
	vars := ThemeVariables{}
	got, _, ok := resolveTokenValue(vars, "#2563eb", map[string]bool{}, "", 0)
	if !ok || got != "#2563eb" {
		t.Errorf("got (%q, %v), want (\"#2563eb\", true)", got, ok)
	}
}

func TestResolveTokenValue_SingleVarReference(t *testing.T) {
	vars := ThemeVariables{"--slidelang-primary-color": "#2563eb"}
	got, _, ok := resolveTokenValue(vars, "var(--slidelang-primary-color)", map[string]bool{}, "", 0)
	if !ok || got != "#2563eb" {
		t.Errorf("got (%q, %v), want (\"#2563eb\", true)", got, ok)
	}
}

// TestResolveTokenValue_CrossPrefixReference is the exact case
// variables.go:120 exhibits today: an embedded theme's unprefixed var
// referencing another unprefixed var, and an external theme's prefixed
// var referencing an unprefixed lookup name.
func TestResolveTokenValue_CrossPrefixReference(t *testing.T) {
	vars := ThemeVariables{"--title-gradient": "linear-gradient(135deg, #667eea 0%, #764ba2 100%)"}
	got, _, ok := resolveTokenValue(vars, "var(--title-gradient)", map[string]bool{}, "", 0)
	if !ok || got != "linear-gradient(135deg, #667eea 0%, #764ba2 100%)" {
		t.Errorf("got (%q, %v)", got, ok)
	}
}

func TestResolveTokenValue_MultiHopChain(t *testing.T) {
	vars := ThemeVariables{
		"--a": "var(--b)",
		"--b": "var(--c)",
		"--c": "#111111",
	}
	got, _, ok := resolveTokenValue(vars, "var(--a)", map[string]bool{}, "", 0)
	if !ok || got != "#111111" {
		t.Errorf("got (%q, %v), want (\"#111111\", true)", got, ok)
	}
}

func TestResolveTokenValue_Cycle(t *testing.T) {
	vars := ThemeVariables{
		"--a": "var(--b)",
		"--b": "var(--a)",
	}
	_, _, ok := resolveTokenValue(vars, "var(--a)", map[string]bool{}, "", 0)
	if ok {
		t.Error("expected a reference cycle to fail resolution, not hang or succeed")
	}
}

// TestResolveTokenValue_CycleFallsBackToLiteral is the exact repro a code
// review flagged: --a resolves to var(--a) (a self-cycle), and the
// ANONYMOUS caller's own var() reference (owner="", not itself a named
// custom property) declares a fallback. CSS's own var() semantics say a
// consumer outside the cycle still gets its own fallback when the
// reference it depends on is guaranteed-invalid — the resolver must do
// the same instead of discarding the fallback the moment the cycle is
// detected one hop down.
func TestResolveTokenValue_CycleFallsBackToLiteral(t *testing.T) {
	vars := ThemeVariables{"--a": "var(--a)"}
	got, _, ok := resolveTokenValue(vars, "var(--a, #fff)", map[string]bool{}, "", 0)
	if !ok || got != "#fff" {
		t.Errorf("got (%q, %v), want (\"#fff\", true)", got, ok)
	}
}

// TestResolveTokenValue_SelfCycleWithOwnFallbackStaysInvalid is
// CycleFallsBackToLiteral's counterpart: when the property that OWNS the
// cyclic var() call is itself part of the cycle — "--a: var(--a, #fff)",
// resolved as token "a" itself (owner="a", seeded exactly like the three
// real production entry points seed it) — CSS does NOT let a property
// rescue itself with its own fallback; every property in the cycle
// computes to guaranteed-invalid regardless of any fallback along the way.
// Real browsers implement it this way too: "--foo: var(--foo, red)" does
// not resolve to red.
func TestResolveTokenValue_SelfCycleWithOwnFallbackStaysInvalid(t *testing.T) {
	vars := ThemeVariables{"--a": "var(--a, #fff)"}
	_, _, ok := resolveTokenValue(vars, "var(--a, #fff)", map[string]bool{"a": true}, "a", 0)
	if ok {
		t.Error("expected a property that references itself, even via its own fallback, to stay guaranteed-invalid")
	}
}

// TestResolveTokenValue_FallbackInsideCycleDoesNotRescue is the exact H6
// repro: --a: var(--b, red), --b: var(--a, blue) — a two-property cycle
// where EACH property's own declaration has a fallback. Per CSS Custom
// Properties §3 the dependency cycle includes references inside fallbacks
// too, so both --a and --b are guaranteed-invalid; neither "red" nor
// "blue" may rescue a property that is itself part of the cycle. Before
// this fix, resolveTokenValue returned "blue" for this case.
func TestResolveTokenValue_FallbackInsideCycleDoesNotRescue(t *testing.T) {
	vars := ThemeVariables{
		"--a": "var(--b, red)",
		"--b": "var(--a, blue)",
	}
	_, _, ok := resolveTokenValue(vars, "var(--a)", map[string]bool{}, "", 0)
	if ok {
		t.Error("expected a cycle formed through fallbacks to fail resolution, not rescue itself with either fallback")
	}
}

// TestPropertyIsCyclic_HiddenInUnevaluatedFallback is the exact repro a
// code review flagged: --a: var(--defined, var(--a)), --defined:
// #123456. Value substitution alone (resolveTokenValue) never even looks
// at the fallback here because the primary reference (--defined) resolves
// fine — but per CSS Custom Properties §3 the dependency graph includes
// every var() reference a property's value contains, fallback or not,
// regardless of whether it's ever actually evaluated at runtime. --a
// self-references through its own fallback, so --a is cyclic and must
// resolve to guaranteed-invalid (absent), not "#123456".
func TestPropertyIsCyclic_HiddenInUnevaluatedFallback(t *testing.T) {
	vars := ThemeVariables{
		"--a":       "var(--defined, var(--a))",
		"--defined": "#123456",
	}
	if !propertyIsCyclic(vars, "a") {
		t.Error("expected a self-reference hidden inside an unevaluated fallback to be detected as cyclic")
	}
}

func TestPropertyIsCyclic_NoFalsePositiveOnOrdinaryChain(t *testing.T) {
	vars := ThemeVariables{
		"--a": "var(--b)",
		"--b": "#2563eb",
	}
	if propertyIsCyclic(vars, "a") {
		t.Error("expected an ordinary, non-cyclic reference chain not to be flagged as cyclic")
	}
}

func TestPropertyIsCyclic_NoFalsePositiveThroughAGenuinelyUnrelatedFallback(t *testing.T) {
	vars := ThemeVariables{
		"--a": "var(--missing, var(--b))",
		"--b": "#111111",
	}
	if propertyIsCyclic(vars, "a") {
		t.Error("expected a fallback that references an unrelated, non-cyclic property not to be flagged")
	}
}

// TestPropertyIsCyclic_NoFalsePositiveWhenOnlyReferencingACycle is the
// exact repro a code review flagged: chart-cat-1 references --b (via its
// own fallback slot, though the same bug applied to a primary reference
// too), and --b/--c form a genuine cycle between THEMSELVES — but
// chart-cat-1 is not part of that cycle, it merely points at one edge of
// it. Per CSS Custom Properties §3, --b and --c are guaranteed-invalid,
// but chart-cat-1's own var(--b, red) reference simply fails to resolve
// (because --b is invalid) and falls back to "red", exactly like any
// other unresolvable reference. An earlier version of propertyIsCyclic
// treated "the DFS revisited ANY node" as proof that the STARTING node
// (chart-cat-1) was cyclic, when only a revisit of chart-cat-1 itself
// would prove that — it incorrectly reported chart-cat-1 as cyclic and
// discarded "red" for nothing.
func TestPropertyIsCyclic_NoFalsePositiveWhenOnlyReferencingACycle(t *testing.T) {
	vars := ThemeVariables{
		"--chart-cat-1": "var(--b, red)",
		"--b":           "var(--c)",
		"--c":           "var(--b)",
	}
	if propertyIsCyclic(vars, "chart-cat-1") {
		t.Error("expected a property that only references a cycle (without being part of it) not to be flagged as cyclic itself")
	}
	// --b and --c themselves genuinely are cyclic — confirms the fix
	// didn't just make propertyIsCyclic permissive across the board.
	if !propertyIsCyclic(vars, "b") {
		t.Error("expected --b, which IS part of the b<->c cycle, to still be detected as cyclic")
	}
	if !propertyIsCyclic(vars, "c") {
		t.Error("expected --c, which IS part of the b<->c cycle, to still be detected as cyclic")
	}
}

// TestResolveOrderedTokens_ResolvesViaFallbackWhenOnlyReferencingACycle is
// TestPropertyIsCyclic_NoFalsePositiveWhenOnlyReferencingACycle's
// end-to-end version, through the real chart-cat-* entry point
// (resolveOrderedTokens, via ResolveThemeTokens) — confirms chart-cat-1
// actually resolves to "red", not just that propertyIsCyclic itself
// returns the right bool in isolation.
func TestResolveOrderedTokens_ResolvesViaFallbackWhenOnlyReferencingACycle(t *testing.T) {
	vars := ThemeVariables{
		"--slidelang-chart-cat-1": "var(--slidelang-b, red)",
		"--slidelang-b":           "var(--slidelang-c)",
		"--slidelang-c":           "var(--slidelang-b)",
	}
	tokens := ResolveThemeTokens(vars)
	want := []string{"red"}
	if !reflect.DeepEqual(tokens.ChartCategorical, want) {
		t.Errorf("ChartCategorical = %#v, want %#v", tokens.ChartCategorical, want)
	}
}

// TestResolveThemeTokens_DiagramTokenCycleHiddenInFallbackDropped is the
// end-to-end version of TestPropertyIsCyclic_HiddenInUnevaluatedFallback,
// exercised through the real production entry point (resolveTokenGroup,
// via ResolveThemeTokens) instead of calling propertyIsCyclic directly.
func TestResolveThemeTokens_DiagramTokenCycleHiddenInFallbackDropped(t *testing.T) {
	vars := ThemeVariables{
		"--slidelang-diagram-node-bg":        "var(--slidelang-diagram-node-bg-source, var(--slidelang-diagram-node-bg))",
		"--slidelang-diagram-node-bg-source": "#1e293b",
		"--slidelang-diagram-edge":           "#2563eb",
	}
	tokens := ResolveThemeTokens(vars)
	if _, ok := tokens.Diagram["diagram-node-bg"]; ok {
		t.Errorf("expected diagram-node-bg (cyclic through its own hidden fallback) to be dropped, got %#v", tokens.Diagram)
	}
	if tokens.Diagram["diagram-edge"] != "#2563eb" {
		t.Errorf("expected the unrelated diagram-edge token to still resolve normally, got %#v", tokens.Diagram)
	}
}

// TestResolveTokenValue_UnresolvableChainFallsBackToLiteral covers the
// non-cyclic sibling case: --a exists but resolves to a reference that
// itself doesn't resolve (missing, no fallback) — the outer var()'s own
// fallback must still apply.
func TestResolveTokenValue_UnresolvableChainFallsBackToLiteral(t *testing.T) {
	vars := ThemeVariables{"--a": "var(--missing)"}
	got, _, ok := resolveTokenValue(vars, "var(--a, #fff)", map[string]bool{}, "", 0)
	if !ok || got != "#fff" {
		t.Errorf("got (%q, %v), want (\"#fff\", true)", got, ok)
	}
}

func TestResolveTokenValue_MissingNoFallback(t *testing.T) {
	vars := ThemeVariables{}
	_, _, ok := resolveTokenValue(vars, "var(--missing)", map[string]bool{}, "", 0)
	if ok {
		t.Error("expected a missing reference with no fallback to fail resolution")
	}
}

func TestResolveTokenValue_MissingWithLiteralFallback(t *testing.T) {
	vars := ThemeVariables{}
	got, _, ok := resolveTokenValue(vars, "var(--missing, #abcdef)", map[string]bool{}, "", 0)
	if !ok || got != "#abcdef" {
		t.Errorf("got (%q, %v), want (\"#abcdef\", true)", got, ok)
	}
}

func TestResolveTokenValue_MissingWithVarFallback(t *testing.T) {
	vars := ThemeVariables{"--slidelang-backup": "#123456"}
	got, _, ok := resolveTokenValue(vars, "var(--missing, var(--slidelang-backup))", map[string]bool{}, "", 0)
	if !ok || got != "#123456" {
		t.Errorf("got (%q, %v), want (\"#123456\", true)", got, ok)
	}
}

func TestResolveTokenValue_EmbeddedVarNotWholeValueRefused(t *testing.T) {
	vars := ThemeVariables{"--slidelang-x": "#000"}
	_, _, ok := resolveTokenValue(vars, "1px solid var(--slidelang-x)", map[string]bool{}, "", 0)
	if ok {
		t.Error("expected a var() embedded inside a larger expression to be refused, not partially flattened")
	}
}

func TestResolveThemeTokens_DiagramGroup(t *testing.T) {
	vars := ThemeVariables{
		"--slidelang-diagram-node-bg": "#1e293b",
		"--slidelang-diagram-edge":    "var(--slidelang-primary-color)",
		"--slidelang-primary-color":   "#2563eb",
	}
	tokens := ResolveThemeTokens(vars)
	want := map[string]string{
		"diagram-node-bg": "#1e293b",
		"diagram-edge":    "#2563eb",
	}
	if !reflect.DeepEqual(tokens.Diagram, want) {
		t.Errorf("Diagram = %#v, want %#v", tokens.Diagram, want)
	}
}

func TestResolveThemeTokens_EmptyForThemeWithNoExtensionTokens(t *testing.T) {
	vars := ThemeVariables{
		"--slidelang-primary-color": "#2563eb",
		"--slidelang-font-main":     "'Inter', sans-serif",
	}
	tokens := ResolveThemeTokens(vars)
	if !tokens.IsEmpty() {
		t.Errorf("expected IsEmpty() for a theme declaring no §2.2 tokens, got %#v", tokens)
	}
}

func TestResolveThemeTokens_ChartCategoricalStopsAtGap(t *testing.T) {
	vars := ThemeVariables{
		"--slidelang-chart-cat-1": "#111111",
		"--slidelang-chart-cat-2": "#222222",
		// chart-cat-3 intentionally missing
		"--slidelang-chart-cat-4": "#444444",
	}
	tokens := ResolveThemeTokens(vars)
	want := []string{"#111111", "#222222"}
	if !reflect.DeepEqual(tokens.ChartCategorical, want) {
		t.Errorf("ChartCategorical = %#v, want %#v (must stop at the gap, not skip over it)", tokens.ChartCategorical, want)
	}
}

func TestResolveThemeTokens_ChartCategoricalFullSet(t *testing.T) {
	vars := ThemeVariables{}
	want := []string{"#1", "#2", "#3", "#4", "#5", "#6", "#7", "#8"}
	for i, v := range want {
		vars[fmt.Sprintf("--slidelang-chart-cat-%d", i+1)] = v
	}
	tokens := ResolveThemeTokens(vars)
	if !reflect.DeepEqual(tokens.ChartCategorical, want) {
		t.Errorf("ChartCategorical = %#v, want %#v", tokens.ChartCategorical, want)
	}
}

func TestResolveThemeTokens_MapTokensFilteredByColorAllowlist(t *testing.T) {
	vars := ThemeVariables{
		"--slidelang-map-line":  "rgba(0, 0, 0, 0.5)", // rejected: not in the allowlist
		"--slidelang-map-label": "#ffffff",            // accepted
	}
	tokens := ResolveThemeTokens(vars)
	want := map[string]string{"map-label": "#ffffff"}
	if !reflect.DeepEqual(tokens.Map, want) {
		t.Errorf("Map = %#v, want %#v (rgba() must be dropped, not passed through)", tokens.Map, want)
	}
}

func TestResolveThemeTokens_MapTokenNamedColorAccepted(t *testing.T) {
	vars := ThemeVariables{"--slidelang-map-line": "crimson"}
	tokens := ResolveThemeTokens(vars)
	if tokens.Map["map-line"] != "crimson" {
		t.Errorf("expected a valid named color to pass through, got %#v", tokens.Map)
	}
}

func TestIsValidMapColor(t *testing.T) {
	cases := map[string]bool{
		"#fff":                 true,
		"#ffff":                true,
		"#ffffff":              true,
		"#ffffffff":            true,
		"CRIMSON":              true,
		"crimson":              true,
		"#12345":               false, // 5-digit hex: not a valid CSS hex length (mapColorNamePattern tightened for IsValidMermaidColor's sake — see its doc comment)
		"#1234567":             false, // 7-digit hex: same
		"rgba(0,0,0,0.5)":      false,
		"linear-gradient(red)": false,
		"var(--x)":             false,
		"":                     false,
	}
	for in, want := range cases {
		if got := IsValidMapColor(in); got != want {
			t.Errorf("IsValidMapColor(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsValidMermaidColor(t *testing.T) {
	cases := map[string]bool{
		"#fff":                     true,
		"#ffff":                    true, // 4-digit hex (#rgba) is valid CSS
		"#ffffff":                  true,
		"#ffffffff":                true,
		"crimson":                  true,
		"CRIMSON":                  true,
		"rgba(0,0,0,0.5)":          true,
		"rgb(10, 20, 30)":          true,
		"hsl(120, 50%, 50%)":       true,
		"hsla(120, 50%, 50%, 0.5)": true,
		// A code-review-flagged gap: these three throw "Unsupported color
		// format" against the real Mermaid build this toolchain embeds
		// (mermaid@10.9.6, core/renderer/cdn_tags.go) and must be rejected,
		// not just accepted-by-accident of a loose regex.
		"#12345":         false, // 5-digit hex: not a valid CSS hex length
		"#1234567":       false, // 7-digit hex: not a valid CSS hex length
		"hsl(120,50,50)": false, // hsl() requires '%' on S/L
		// A fourth code-review-flagged gap: rgb/rgba (and hsl/hsla) are
		// true CSS Color 4 aliases of each other — the function name
		// doesn't fix the arity. mermaid@10.9.6 accepts both of these;
		// an earlier grammar rejected them because it hard-coded arity
		// per function name instead of per argument count.
		"rgb(10,20,30,40)":     true,
		"rgba(10,20,30)":       true,
		"rgb(1.2.3, 0, 0)":     false, // not a real number
		"linear-gradient(red)": false,
		"var(--x)":             false,
		"":                     false,
		// A code-review-flagged gap: the hue component reused the same
		// pattern as rgb()/rgba()'s components, which allows a trailing
		// '%' — but hsl()'s hue is a bare angle/number, never a
		// percentage. "120%" as a hue is invalid CSS and throws
		// "Unsupported color format" in Mermaid exactly like any other
		// out-of-grammar value.
		"hsl(120%, 50%, 50%)":       false,
		"hsla(120%, 50%, 50%, 0.5)": false,
		// A second code-review-flagged gap: mermaidNamedColors must cover
		// the full CSS named-color set, not maps.js's narrow 41-name
		// allowlist (mapNamedColors) — these are all valid CSS colors
		// mermaid@10.9.6 accepts that the narrower list didn't have.
		"aliceblue":     true,
		"darkslategray": true,
		"rebeccapurple": true,
		// A third code-review-flagged gap: mermaid@10.9.6 accepts a hue
		// with a sign and/or an angle unit, and the modern CSS Color 4
		// space-separated syntax with an optional "/ alpha" — none of
		// which the pattern used to cover, silently producing incomplete
		// theming (dropped tokens, not a crash) for anyone using these
		// otherwise-valid forms.
		"hsl(-30,50%,50%)":       true,
		"hsl(0.5turn,50%,50%)":   true,
		"rgb(255 0 0 / 50%)":     true,
		"hsl(120 50% 50% / 25%)": true,
		// A fifth code-review-flagged gap, closed by parsing components
		// as real CSS numbers (parseCSSNumber) instead of matching a
		// no-sign, no-exponent digit pattern: mermaid@10.9.6 accepts a
		// negative rgb() component, scientific notation, and an
		// out-of-range (clamped, not rejected) hsl() saturation/
		// lightness — none of which the previous regex-only grammar
		// could ever accept short of one more hand-added alternative.
		"rgb(-10 0 0)":       true,
		"rgb(1e2 0 0)":       true,
		"hsl(-30 -10% 120%)": true,
	}
	for in, want := range cases {
		if got := IsValidMermaidColor(in); got != want {
			t.Errorf("IsValidMermaidColor(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestNormalizeMermaidColor_NamedColorBecomesHex is the exact repro a code
// review flagged: mermaid@10.9.6's own khroma-based color parser does NOT
// necessarily recognize every standard CSS named color — "cyan",
// "steelblue", and "tomato" (all valid CSS, all in cssNamedColorHex) still
// throw "Unsupported color format" against the real pinned bundle. The
// fix is to never ship a named color to Mermaid at all: normalize it to
// hex first, which every real color parser (khroma included) accepts
// unconditionally.
func TestNormalizeMermaidColor_NamedColorBecomesHex(t *testing.T) {
	cases := map[string]string{
		"cyan":          "#00FFFF",
		"CYAN":          "#00FFFF", // case-insensitive lookup
		"steelblue":     "#4682B4",
		"tomato":        "#FF6347",
		"aliceblue":     "#F0F8FF",
		"rebeccapurple": "#663399",
		"transparent":   "#00000000",
	}
	for in, want := range cases {
		got, ok := normalizeMermaidColor(in)
		if !ok || got != want {
			t.Errorf("normalizeMermaidColor(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}
}

// TestNormalizeMermaidColor_HexPassesThroughUnchanged confirms hex values
// — already in the one form every real CSS color parser (khroma
// included) accepts unconditionally — are never needlessly reformatted.
func TestNormalizeMermaidColor_HexPassesThroughUnchanged(t *testing.T) {
	cases := []string{"#ff0000", "#F00", "#12ab34cd"}
	for _, in := range cases {
		got, ok := normalizeMermaidColor(in)
		if !ok || got != in {
			t.Errorf("normalizeMermaidColor(%q) = (%q, %v), want (%q, true) unchanged", in, got, ok, in)
		}
	}
}

// TestNormalizeMermaidColor_FunctionalBecomesHex is the analogous repro
// to TestNormalizeMermaidColor_NamedColorBecomesHex for functional
// syntax: rgb()/hsl() are ALSO rewritten to hex now, not just passed
// through — the same fix applied to the same underlying problem
// (chasing a third-party parser's exact accepted grammar via regex is
// how three separate review rounds each found a new gap; parsing the
// value as real CSS numbers and emitting hex is what stops that from
// having a fourth round).
func TestNormalizeMermaidColor_FunctionalBecomesHex(t *testing.T) {
	cases := map[string]string{
		"rgb(255, 0, 0)":    "#FF0000",
		"rgba(0,0,0,0.5)":   "#00000080",
		"rgb(-10 0 0)":      "#000000",   // negative clamps to 0
		"rgb(1e2 0 0)":      "#640000",   // scientific notation: 100 = 0x64
		"rgba(10,20,30)":    "#0A141E",   // rgba() alias accepts 3 components
		"rgb(10,20,30,0.5)": "#0A141E80", // rgb() alias accepts a 4th (alpha)
	}
	for in, want := range cases {
		got, ok := normalizeMermaidColor(in)
		if !ok || got != want {
			t.Errorf("normalizeMermaidColor(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}
}

func TestNormalizeMermaidColor_InvalidRejected(t *testing.T) {
	for _, in := range []string{"linear-gradient(red)", "var(--x)", "notacolor", ""} {
		if _, ok := normalizeMermaidColor(in); ok {
			t.Errorf("normalizeMermaidColor(%q) unexpectedly succeeded", in)
		}
	}
}

// goOnlyFloatSyntax is the corpus a code-review finding produced: values
// strconv.ParseFloat accepts (Go's float literal grammar) that are NOT
// valid CSS numbers. The NaN cases were the dangerous ones — they reached
// round255, and converting NaN to an int is UNDEFINED in Go, so the
// emitted "hex" was platform-dependent garbage: "#-80000000000000000000"
// on linux/amd64 (the reviewer's Ubuntu repro), silently "#000000" on
// darwin/arm64. Either way mermaid.js gets something it never should:
// on amd64 a string that is not a color at all, which can abort its init.
var goOnlyFloatSyntax = []string{
	"rgb(NaN 0 0)",      // the reviewer's exact repro
	"rgba(0 0 0 / NaN)", // same, via the alpha channel
	"rgb(Inf 0 0)",
	"rgb(0x1p2 0 0)", // Go hex-float literal
	"rgb(-Inf 0 0)",
	"rgb(Infinity 0 0)",
	"rgb(nan 0 0)",  // ParseFloat is case-insensitive about these
	"rgb(1_0 0 0)",  // Go digit separator
	"rgb(0x10 0 0)", // Go hex integer literal
	"hsl(NaN 50% 50%)",
	"hsl(NaN, 50%, 50%)",
	"rgb(10. 0 0)", // a '.' with no digit after it is not a CSS number
}

// TestNormalizeMermaidColor_RejectsGoOnlyFloatSyntax pins the fix for
// that finding: validating the CSS <number-token> lexeme BEFORE calling
// ParseFloat. Checking only ParseFloat's error cannot catch any of these
// — it returns no error for a single one of them.
func TestNormalizeMermaidColor_RejectsGoOnlyFloatSyntax(t *testing.T) {
	for _, in := range goOnlyFloatSyntax {
		if got, ok := normalizeMermaidColor(in); ok {
			t.Errorf("normalizeMermaidColor(%q) = (%q, true), want rejected: Go float syntax is not CSS number syntax", in, got)
		}
	}
}

// whitespaceBetweenNumberAndUnit is a code-review finding's corpus: CSS
// dimensions are single tokens ("10%", "30deg"), so whitespace between
// the number and its unit is not valid CSS. An earlier version of
// parseCSSNumberLexeme trimmed its input, which quietly undid the lexeme
// check for exactly this case — "rgb(10 %,0,0)" was accepted as 10% and
// emitted "#1A0000". Note that neither this corpus nor
// nonFiniteHueSyntax below would be caught by
// TestNormalizeMermaidColor_NeverEmitsNonHex: both produced perfectly
// well-formed hex, just from input that is not a color. Wrong-but-hex
// output needs its own cases.
var whitespaceBetweenNumberAndUnit = []string{
	"rgb(10 %,0,0)",
	"rgba(0,0,0,50 %)",
	"hsl(30 deg,50%,50%)",
	"hsl(30,50 %,50%)",
	"hsl(30\tdeg,50%,50%)",
	"hsl(30 turn,50%,50%)",
}

func TestNormalizeMermaidColor_RejectsWhitespaceBetweenNumberAndUnit(t *testing.T) {
	for _, in := range whitespaceBetweenNumberAndUnit {
		if got, ok := normalizeMermaidColor(in); ok {
			t.Errorf("normalizeMermaidColor(%q) = (%q, true), want rejected: a CSS dimension is one token, so no whitespace may sit between the number and its unit", in, got)
		}
	}
}

// nonFiniteHueSyntax is a code-review finding's corpus for the one
// component where saturating an overflow is NOT the right answer. Every
// other component lives on a bounded scale, so ±Inf clamps to the end of
// it and stays finite; a hue is an angle on a circle, and math.Mod(±Inf,
// 360) is NaN. That NaN then vanished inside hslToRGB — its comparison
// chain is all-false for NaN, so it fell through to the default branch
// and returned a finite 0 per channel — making "hsl(1e400 100% 50%)"
// render as BLACK rather than being dropped, with allFinite at the
// output boundary unable to see it. The last two entries are the reason
// the check has to run again AFTER the unit conversion: the literal
// itself is finite and only overflows once scaled into degrees.
var nonFiniteHueSyntax = []string{
	"hsl(1e400 100% 50%)",
	"hsl(-1e400 100% 50%)",
	"hsl(1e400deg 100% 50%)",
	"hsla(1e400, 100%, 50%, 0.5)",
	"hsl(1e308turn 100% 50%)",
	"hsl(1e308rad 100% 50%)",
}

func TestNormalizeMermaidColor_RejectsNonFiniteHue(t *testing.T) {
	for _, in := range nonFiniteHueSyntax {
		if got, ok := normalizeMermaidColor(in); ok {
			t.Errorf("normalizeMermaidColor(%q) = (%q, true), want rejected: a non-finite hue has no residue mod 360", in, got)
		}
	}
}

// TestNormalizeMermaidColor_HugeButFiniteHueStillResolves is the
// counterpart that keeps the fix above from becoming an
// over-rejection: only a genuinely non-finite hue is dropped, not merely
// a large one. 1e305 turns is absurd but finite even after scaling, so
// it still wraps into [0,360) and produces a color.
func TestNormalizeMermaidColor_HugeButFiniteHueStillResolves(t *testing.T) {
	if got, ok := normalizeMermaidColor("hsl(1e305turn 100% 50%)"); !ok || !mapColorNamePattern.MatchString(got) {
		t.Errorf("normalizeMermaidColor(\"hsl(1e305turn 100%% 50%%)\") = (%q, %v), want a hex color", got, ok)
	}
}

// TestNormalizeMermaidColor_LegitimateWhitespaceStillAccepted guards the
// other direction of the whitespace fix: space AROUND a component (and
// the modern slash-alpha syntax's own spacing) is perfectly valid CSS and
// must keep working — only space INSIDE a dimension is illegal.
func TestNormalizeMermaidColor_LegitimateWhitespaceStillAccepted(t *testing.T) {
	cases := map[string]string{
		"rgb( 10 , 20 , 30 )":    "#0A141E",
		"rgb(255 0 0 / 50%)":     "#FF000080",
		"hsl(120 50% 50% / 25%)": "#40BF4040",
		"hsl( 120 , 50% , 50% )": "#40BF40",
		"  rgb(10,20,30)  ":      "#0A141E",
		"hsl(-30 -10% 120%)":     "#FFFFFF",
	}
	for in, want := range cases {
		if got, ok := normalizeMermaidColor(in); !ok || got != want {
			t.Errorf("normalizeMermaidColor(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}
}

// TestNormalizeMermaidColor_NewlineIsOrdinaryWhitespace is a code-review
// finding's repro: a newline is ordinary CSS whitespace, Mermaid accepts
// it, and a theme's manifest.json can carry one inside a variable's
// string value — but functionalColorCallRe used `.*?` without the dot-all
// flag, and Go's `.` does not match a newline, so the pattern failed
// outright and the color was dropped before splitFunctionalArgs (which
// handles newlines fine, via strings.Fields and per-part TrimSpace) ever
// saw it.
func TestNormalizeMermaidColor_NewlineIsOrdinaryWhitespace(t *testing.T) {
	cases := map[string]string{
		"rgb(255\n0\n0)":        "#FF0000",
		"rgb(255,\n0,\n0)":      "#FF0000",
		"hsl(120 100%\n50%)":    "#00FF00",
		"hsl(120,\n100%,\n50%)": "#00FF00",
		"rgba(255 0\n0 / 50%)":  "#FF000080",
		"rgb(255,\r\n0,\r\n0)":  "#FF0000", // CRLF too
		"rgb(\n255, 0, 0\n)":    "#FF0000", // newlines just inside the parens
	}
	for in, want := range cases {
		if got, ok := normalizeMermaidColor(in); !ok || got != want {
			t.Errorf("normalizeMermaidColor(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}
}

// TestNormalizeMermaidColor_NewlineInsideDimensionStillRejected keeps the
// dot-all fix from widening more than intended: a newline is whitespace
// like any other, so it is legal BETWEEN components and illegal INSIDE a
// dimension, exactly like a space (see whitespaceBetweenNumberAndUnit).
func TestNormalizeMermaidColor_NewlineInsideDimensionStillRejected(t *testing.T) {
	for _, in := range []string{"rgb(10\n%,0,0)", "hsl(30\ndeg,50%,50%)"} {
		if got, ok := normalizeMermaidColor(in); ok {
			t.Errorf("normalizeMermaidColor(%q) = (%q, true), want rejected: a newline inside a dimension is no more legal than a space", in, got)
		}
	}
}

// TestNormalizeMermaidColor_NeverEmitsNonHex is the generic invariant the
// NaN bug violated, stated directly instead of case by case: whatever
// normalizeMermaidColor accepts, what it RETURNS must always be a literal
// hex color — the one notation every real CSS color parser (khroma
// included) handles unconditionally, which is the whole point of
// normalizing. It is deliberately asserted with mapColorNamePattern, the
// same pattern the validator accepts hex by, so the two can never drift
// into a state where this function emits something its own validator
// would reject.
func TestNormalizeMermaidColor_NeverEmitsNonHex(t *testing.T) {
	corpus := append([]string{
		"red", "cyan", "transparent", "#fff", "#ffff", "#ffffff", "#ffffffff",
		"rgb(255, 0, 0)", "rgba(0,0,0,0.5)", "rgb(-10 0 0)", "rgb(1e2 0 0)",
		"hsl(-30 -10% 120%)", "hsl(0.5turn,50%,50%)", "rgb(255 0 0 / 50%)",
		"rgb(1e400 0 0)", "rgb(.5 0 0)", "rgb(+10 0 0)", "hsl(1e305turn 100% 50%)",
		"linear-gradient(red, blue)", "var(--x)", "notacolor", "",
	}, goOnlyFloatSyntax...)
	corpus = append(corpus, whitespaceBetweenNumberAndUnit...)
	corpus = append(corpus, nonFiniteHueSyntax...)

	for _, in := range corpus {
		got, ok := normalizeMermaidColor(in)
		if !ok {
			continue
		}
		if !mapColorNamePattern.MatchString(got) {
			t.Errorf("normalizeMermaidColor(%q) = %q — accepted a value but emitted a non-hex string", in, got)
		}
	}
}

// TestNormalizeFunctionalColor_OutOfRangeExponentClamps documents the one
// error a VALID CSS lexeme can still produce: ParseFloat returns ErrRange
// for "1e400" along with the saturated value (+Inf), and saturating is
// exactly the clamp CSS asks for on an out-of-range component — so it is
// accepted and clamped, not dropped. (Before the lexeme fix this was
// rejected outright, since the old code treated any ParseFloat error as
// fatal.)
func TestNormalizeFunctionalColor_OutOfRangeExponentClamps(t *testing.T) {
	got, ok := normalizeMermaidColor("rgb(1e400 0 0)")
	if !ok || got != "#FF0000" {
		t.Errorf("normalizeMermaidColor(\"rgb(1e400 0 0)\") = (%q, %v), want (\"#FF0000\", true)", got, ok)
	}
}

func TestParseCSSNumber_CSSLexemeGrammar(t *testing.T) {
	accepted := map[string]float64{
		"0": 0, "10": 10, "+10": 10, "-10": -10,
		".5": 0.5, "-.5": -0.5, "10.5": 10.5,
		"1e2": 100, "1E2": 100, "1e+2": 100, "1e-2": 0.01, ".5e1": 5,
	}
	for in, want := range accepted {
		v, percent, unit, ok := parseCSSNumber(in)
		if !ok || v != want || percent || unit != "" {
			t.Errorf("parseCSSNumber(%q) = (%v, %v, %q, %v), want (%v, false, \"\", true)", in, v, percent, unit, ok, want)
		}
	}

	rejected := []string{
		"NaN", "nan", "Inf", "-Inf", "Infinity", "0x1p2", "0x10", "1_0",
		"10.", ".", "1e", "1.2.3", "e5", "+", "-", "", "  ", "abc",
		// Whitespace between the number and its unit: a CSS dimension is
		// a single token, so these are not valid CSS (a code-review
		// finding — parseCSSNumberLexeme used to trim them into validity).
		"10 %", "30 deg", "0.5 turn",
	}
	for _, in := range rejected {
		if v, _, _, ok := parseCSSNumber(in); ok {
			t.Errorf("parseCSSNumber(%q) = (%v, true), want rejected: not a CSS number lexeme", in, v)
		}
	}

	// Units and percentages ride on the same lexeme check.
	if v, percent, unit, ok := parseCSSNumber("-30deg"); !ok || v != -30 || percent || unit != "deg" {
		t.Errorf("parseCSSNumber(\"-30deg\") = (%v, %v, %q, %v), want (-30, false, \"deg\", true)", v, percent, unit, ok)
	}
	if v, percent, unit, ok := parseCSSNumber("50%"); !ok || v != 50 || !percent || unit != "" {
		t.Errorf("parseCSSNumber(\"50%%\") = (%v, %v, %q, %v), want (50, true, \"\", true)", v, percent, unit, ok)
	}
	if _, _, _, ok := parseCSSNumber("NaNdeg"); ok {
		t.Error("parseCSSNumber(\"NaNdeg\") accepted a non-CSS number carrying a valid unit")
	}
}

// TestCSSNamedColorHex_CoversEveryMermaidNamedColor is a structural
// safety net: it doesn't re-verify each hex value against an external
// source, but it does guarantee cssNamedColorHex's key set is exactly
// the full CSS Color Module set IsValidMermaidColor is documented to
// accept — every name TestIsValidMermaidColor exercises as true (and a
// spot-check of the full set's expected size) must have a hex entry, or
// normalizeMermaidColor would silently start rejecting a name
// IsValidMermaidColor claims to accept.
func TestCSSNamedColorHex_CoversEveryMermaidNamedColor(t *testing.T) {
	if len(cssNamedColorHex) != 149 {
		t.Errorf("cssNamedColorHex has %d entries, want 149", len(cssNamedColorHex))
	}
	for _, name := range []string{"aliceblue", "cyan", "rebeccapurple", "transparent", "tomato", "steelblue"} {
		if _, ok := cssNamedColorHex[name]; !ok {
			t.Errorf("cssNamedColorHex missing %q", name)
		}
	}
}

// TestResolveThemeTokens_DiagramNamedColorReachesPayloadAsHex is the
// end-to-end version of TestNormalizeMermaidColor_NamedColorBecomesHex,
// through the real production entry point (ResolveThemeTokens) — confirms
// a theme declaring a diagram-* token as a bare named color reaches the
// metadata payload already normalized to hex, not as the name.
func TestResolveThemeTokens_DiagramNamedColorReachesPayloadAsHex(t *testing.T) {
	vars := ThemeVariables{"--slidelang-diagram-node-bg": "cyan"}
	tokens := ResolveThemeTokens(vars)
	if tokens.Diagram["diagram-node-bg"] != "#00FFFF" {
		t.Errorf("diagram-node-bg = %q, want \"#00FFFF\" (normalized from \"cyan\")", tokens.Diagram["diagram-node-bg"])
	}
}

// TestResolveThemeTokens_DiagramTokenGradientDropped is the exact repro a
// code review flagged: an unvalidated diagram-* token reaching Mermaid's
// themeVariables as a gradient throws "Unsupported color format" inside
// mermaid.initialize(), which mermaid.js's own error handling swallows —
// leaving the module silently half-initialized. The token must never reach
// that far; it has to be dropped server-side instead.
func TestResolveThemeTokens_DiagramTokenGradientDropped(t *testing.T) {
	vars := ThemeVariables{
		"--slidelang-diagram-node-bg": "linear-gradient(red, blue)",
		"--slidelang-diagram-edge":    "#2563eb",
	}
	tokens := ResolveThemeTokens(vars)
	want := map[string]string{"diagram-edge": "#2563eb"}
	if !reflect.DeepEqual(tokens.Diagram, want) {
		t.Errorf("Diagram = %#v, want %#v (gradient must be dropped, not passed through)", tokens.Diagram, want)
	}
}

func TestResolveFontMain_LiteralStack(t *testing.T) {
	vars := ThemeVariables{"--slidelang-font-main": "'Inter', sans-serif"}
	if got := ResolveFontMain(vars); got != "'Inter', sans-serif" {
		t.Errorf("got %q", got)
	}
}

func TestResolveFontMain_UnprefixedEmbeddedTheme(t *testing.T) {
	vars := ThemeVariables{"--font-main": "'Inter', sans-serif"}
	if got := ResolveFontMain(vars); got != "'Inter', sans-serif" {
		t.Errorf("got %q", got)
	}
}

func TestResolveFontMain_Absent(t *testing.T) {
	vars := ThemeVariables{}
	if got := ResolveFontMain(vars); got != "" {
		t.Errorf("expected empty string when font-main is absent, got %q", got)
	}
}
