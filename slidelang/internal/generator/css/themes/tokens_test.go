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
	got, ok := resolveTokenValue(vars, "#2563eb", map[string]bool{}, 0)
	if !ok || got != "#2563eb" {
		t.Errorf("got (%q, %v), want (\"#2563eb\", true)", got, ok)
	}
}

func TestResolveTokenValue_SingleVarReference(t *testing.T) {
	vars := ThemeVariables{"--slidelang-primary-color": "#2563eb"}
	got, ok := resolveTokenValue(vars, "var(--slidelang-primary-color)", map[string]bool{}, 0)
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
	got, ok := resolveTokenValue(vars, "var(--title-gradient)", map[string]bool{}, 0)
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
	got, ok := resolveTokenValue(vars, "var(--a)", map[string]bool{}, 0)
	if !ok || got != "#111111" {
		t.Errorf("got (%q, %v), want (\"#111111\", true)", got, ok)
	}
}

func TestResolveTokenValue_Cycle(t *testing.T) {
	vars := ThemeVariables{
		"--a": "var(--b)",
		"--b": "var(--a)",
	}
	_, ok := resolveTokenValue(vars, "var(--a)", map[string]bool{}, 0)
	if ok {
		t.Error("expected a reference cycle to fail resolution, not hang or succeed")
	}
}

func TestResolveTokenValue_MissingNoFallback(t *testing.T) {
	vars := ThemeVariables{}
	_, ok := resolveTokenValue(vars, "var(--missing)", map[string]bool{}, 0)
	if ok {
		t.Error("expected a missing reference with no fallback to fail resolution")
	}
}

func TestResolveTokenValue_MissingWithLiteralFallback(t *testing.T) {
	vars := ThemeVariables{}
	got, ok := resolveTokenValue(vars, "var(--missing, #abcdef)", map[string]bool{}, 0)
	if !ok || got != "#abcdef" {
		t.Errorf("got (%q, %v), want (\"#abcdef\", true)", got, ok)
	}
}

func TestResolveTokenValue_MissingWithVarFallback(t *testing.T) {
	vars := ThemeVariables{"--slidelang-backup": "#123456"}
	got, ok := resolveTokenValue(vars, "var(--missing, var(--slidelang-backup))", map[string]bool{}, 0)
	if !ok || got != "#123456" {
		t.Errorf("got (%q, %v), want (\"#123456\", true)", got, ok)
	}
}

func TestResolveTokenValue_EmbeddedVarNotWholeValueRefused(t *testing.T) {
	vars := ThemeVariables{"--slidelang-x": "#000"}
	_, ok := resolveTokenValue(vars, "1px solid var(--slidelang-x)", map[string]bool{}, 0)
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
		"#ffffff":              true,
		"#ffffffff":            true,
		"CRIMSON":              true,
		"crimson":              true,
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
