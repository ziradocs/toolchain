// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestChartCategoricalPalette_EmptyOverrideUsesFallback(t *testing.T) {
	got := chartCategoricalPalette(nil, defaultChartColors6)
	if len(got) != len(defaultChartColors6) || got[0] != defaultChartColors6[0] {
		t.Errorf("expected fallback palette unchanged, got %v", got)
	}
}

func TestChartCategoricalPalette_NonEmptyOverrideWins(t *testing.T) {
	override := []string{"#111111", "#222222"}
	got := chartCategoricalPalette(override, defaultChartColors6)
	if len(got) != 2 || got[0] != "#111111" || got[1] != "#222222" {
		t.Errorf("expected override palette, got %v", got)
	}
}

// TestGenerateChartConfig_NilOverridePreservesDefaults is the
// non-regression half of motor-temas-v2.md §2.2: the two exported
// convenience functions (GenerateChartConfig/GenerateChartConfigForExport)
// always pass nil to GenerateChartConfigWithMode, so their output must be
// byte-for-byte unchanged from before RenderContext.ChartCategoricalColors
// existed — every doclang/slidelang caller that never threads a
// RenderContext (e.g. doclang's docx.go) must see no behavior change.
func TestGenerateChartConfig_NilOverridePreservesDefaults(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	chart := ast.NewChartElement(pos, "bar")
	chart.Data = [][]interface{}{{"Q1", 10.0, 20.0}}
	chart.Series = []string{"A", "B"}

	config := GenerateChartConfig(chart)
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(config), &decoded); err != nil {
		t.Fatalf("chart config is not valid JSON: %v\n%s", err, config)
	}
	datasets := decoded["data"].(map[string]interface{})["datasets"].([]interface{})
	first := datasets[0].(map[string]interface{})
	if first["backgroundColor"] != defaultChartColors6[0] {
		t.Errorf("expected default palette's first color %q, got %v", defaultChartColors6[0], first["backgroundColor"])
	}
}

// TestGenerateChartConfigWithMode_OverrideAppliesToAllThreeBranches covers
// the three chart-shape branches that each carried their own hardcoded
// palette literal (combo, pie/doughnut, bar/line — core/renderer/html.go)
// — motor-temas-v2.md §2.2 lists all three as in scope. A non-empty
// categoricalColors must be honored by every one of them, not just
// whichever branch happened to be tested first.
func TestGenerateChartConfigWithMode_OverrideAppliesToAllThreeBranches(t *testing.T) {
	override := []string{"#abcdef", "#123456"}
	pos := diagnostics.NewPosition(1, 1)

	t.Run("bar_line", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "bar")
		chart.Data = [][]interface{}{{"Q1", 10.0}}
		chart.Series = []string{"A"}

		config := GenerateChartConfigWithMode(chart, false, override)
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(config), &decoded); err != nil {
			t.Fatalf("chart config is not valid JSON: %v\n%s", err, config)
		}
		dataset := decoded["data"].(map[string]interface{})["datasets"].([]interface{})[0].(map[string]interface{})
		if dataset["backgroundColor"] != override[0] {
			t.Errorf("bar/line: expected override color %q, got %v", override[0], dataset["backgroundColor"])
		}
	})

	t.Run("combo", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "combo")
		chart.Data = [][]interface{}{{"Q1", 10.0}}
		chart.SeriesTypes = []string{"bar"}

		config := GenerateChartConfigWithMode(chart, false, override)
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(config), &decoded); err != nil {
			t.Fatalf("chart config is not valid JSON: %v\n%s", err, config)
		}
		dataset := decoded["data"].(map[string]interface{})["datasets"].([]interface{})[0].(map[string]interface{})
		if dataset["backgroundColor"] != override[0] {
			t.Errorf("combo: expected override color %q, got %v", override[0], dataset["backgroundColor"])
		}
	})

	t.Run("pie_doughnut", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "pie")
		chart.Data = [][]interface{}{{"A", 1.0}, {"B", 2.0}}

		config := GenerateChartConfigWithMode(chart, false, override)
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(config), &decoded); err != nil {
			t.Fatalf("chart config is not valid JSON: %v\n%s", err, config)
		}
		dataset := decoded["data"].(map[string]interface{})["datasets"].([]interface{})[0].(map[string]interface{})
		backgroundColors := dataset["backgroundColor"].([]interface{})
		if backgroundColors[0] != override[0] || backgroundColors[1] != override[1] {
			t.Errorf("pie/doughnut: expected override palette, got %v", backgroundColors)
		}
	})
}

// TestChartAreaFillColor is the code-review finding on PR #224:
// `color + "33"` assumed color was always this file's own "#rrggbb"
// literal — true before ChartCategoricalColors existed, false now that a
// theme's chart-cat-* token can resolve to any valid CSS color syntax.
func TestChartAreaFillColor(t *testing.T) {
	cases := []struct {
		name  string
		color string
		want  string
	}{
		{"hex6 expands with alpha suffix", "#3498db", "#3498db33"},
		{"hex3 shorthand expands to hex6 then alpha", "#abc", "#aabbcc33"},
		{"rgb() degrades to opaque, no corruption", "rgb(255, 0, 0)", "rgb(255, 0, 0)"},
		{"named color degrades to opaque, no corruption", "red", "red"},
		{"already-alpha hex8 degrades to opaque, no double-alpha", "#3498db80", "#3498db80"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := chartAreaFillColor(c.color); got != c.want {
				t.Errorf("chartAreaFillColor(%q) = %q, want %q", c.color, got, c.want)
			}
		})
	}
}

// TestGenerateChartConfigWithMode_NonHexOverrideProducesValidBackgroundColor
// is the end-to-end version of the same finding: a line chart with a
// non-hex-6 categorical override must not embed corrupted CSS (e.g.
// "red33") in the generated Chart.js config.
func TestGenerateChartConfigWithMode_NonHexOverrideProducesValidBackgroundColor(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	chart := ast.NewChartElement(pos, "line")
	chart.Data = [][]interface{}{{"Q1", 10.0}}
	chart.Series = []string{"A"}

	config := GenerateChartConfigWithMode(chart, false, []string{"rgb(255, 0, 0)"})
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(config), &decoded); err != nil {
		t.Fatalf("chart config is not valid JSON: %v\n%s", err, config)
	}
	dataset := decoded["data"].(map[string]interface{})["datasets"].([]interface{})[0].(map[string]interface{})
	if bg := dataset["backgroundColor"]; bg != "rgb(255, 0, 0)" {
		t.Errorf("backgroundColor = %v, want the unmodified rgb() color (not a corrupted \"rgb(255, 0, 0)33\")", bg)
	}
}
