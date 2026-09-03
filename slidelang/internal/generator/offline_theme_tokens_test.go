// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/generator/css/themes"
)

func TestResolveChartCategoricalColors_NilResolvedTheme(t *testing.T) {
	got := resolveChartCategoricalColors(GeneratorOptions{})
	if got != nil {
		t.Errorf("expected nil when ResolvedTheme is nil (reproduces today's behavior byte for byte), got %#v", got)
	}
}

func TestResolveChartCategoricalColors_ThemeWithoutTokens(t *testing.T) {
	theme := &themes.Theme{
		Name:      "no-tokens",
		Variables: themes.ThemeVariables{"--slidelang-primary-color": "#2563eb"},
	}
	got := resolveChartCategoricalColors(GeneratorOptions{ResolvedTheme: theme})
	if got != nil {
		t.Errorf("expected nil for a theme with no chart-cat-* tokens, got %#v", got)
	}
}

func TestResolveChartCategoricalColors_ThemeWithTokens(t *testing.T) {
	theme := &themes.Theme{
		Name: "with-tokens",
		Variables: themes.ThemeVariables{
			"--slidelang-chart-cat-1": "#111111",
			"--slidelang-chart-cat-2": "#222222",
		},
	}
	got := resolveChartCategoricalColors(GeneratorOptions{ResolvedTheme: theme})
	want := []string{"#111111", "#222222"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

// TestTryBuildNativeContext_ChartCategoricalColorsReachContext is the
// offline/PDF/pptx half of motor-temas-v2.md §2.2: chart-cat-* is the only
// token group with core plumbing today (PR #224), and it must reach
// RenderContext.ChartCategoricalColors when a theme declares it, so both
// RenderChartNativePNGWithColors (called inline, above) and any later
// consumer of ctx see the same palette.
func TestTryBuildNativeContext_ChartCategoricalColorsReachContext(t *testing.T) {
	g := New(util.NewNoop())
	theme := &themes.Theme{
		Name: "with-tokens",
		Variables: themes.ThemeVariables{
			"--slidelang-chart-cat-1": "#aabbcc",
			"--slidelang-chart-cat-2": "#ddeeff",
		},
	}
	doc := astWithElements(nativeBarChart())

	ctx, ok := g.tryBuildNativeContext(doc, t.TempDir(), GeneratorOptions{ResolvedTheme: theme})
	if !ok {
		t.Fatal("expected tryBuildNativeContext to succeed for a native-capable chart")
	}
	want := []string{"#aabbcc", "#ddeeff"}
	if !reflect.DeepEqual(ctx.ChartCategoricalColors, want) {
		t.Errorf("ctx.ChartCategoricalColors = %#v, want %#v", ctx.ChartCategoricalColors, want)
	}
}

// TestTryBuildNativeContext_NoResolvedTheme_ChartCategoricalColorsNil is the
// byte-for-byte non-regression: every deck built without a theme declaring
// §2.2 tokens (every deck today) must leave ChartCategoricalColors nil,
// reproducing the hardcoded palette exactly as before this PR.
func TestTryBuildNativeContext_NoResolvedTheme_ChartCategoricalColorsNil(t *testing.T) {
	g := New(util.NewNoop())
	doc := astWithElements(nativeBarChart())

	ctx, ok := g.tryBuildNativeContext(doc, t.TempDir(), GeneratorOptions{})
	if !ok {
		t.Fatal("expected tryBuildNativeContext to succeed for a native-capable chart")
	}
	if ctx.ChartCategoricalColors != nil {
		t.Errorf("expected nil ChartCategoricalColors with no ResolvedTheme, got %#v", ctx.ChartCategoricalColors)
	}
}
