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
	if !propertyIsCyclic(vars, "a", map[string]bool{}) {
		t.Error("expected a self-reference hidden inside an unevaluated fallback to be detected as cyclic")
	}
}

func TestPropertyIsCyclic_NoFalsePositiveOnOrdinaryChain(t *testing.T) {
	vars := ThemeVariables{
		"--a": "var(--b)",
		"--b": "#2563eb",
	}
	if propertyIsCyclic(vars, "a", map[string]bool{}) {
		t.Error("expected an ordinary, non-cyclic reference chain not to be flagged as cyclic")
	}
}

func TestPropertyIsCyclic_NoFalsePositiveThroughAGenuinelyUnrelatedFallback(t *testing.T) {
	vars := ThemeVariables{
		"--a": "var(--missing, var(--b))",
		"--b": "#111111",
	}
	if propertyIsCyclic(vars, "a", map[string]bool{}) {
		t.Error("expected a fallback that references an unrelated, non-cyclic property not to be flagged")
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
		"#12345":               false, // 5-digit hex: not a valid CSS hex length
		"#1234567":             false, // 7-digit hex: not a valid CSS hex length
		"hsl(120,50,50)":       false, // hsl() requires '%' on S/L
		"rgb(10,20,30,40)":     false, // rgb() takes exactly 3 components, not 4
		"rgba(10,20,30)":       false, // rgba() takes exactly 4 components, not 3
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
	}
	for in, want := range cases {
		if got := IsValidMermaidColor(in); got != want {
			t.Errorf("IsValidMermaidColor(%q) = %v, want %v", in, got, want)
		}
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
