// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/generator/css/themes"
)

// TestGeneratePPTX_ThemeCategoricalColorsDoNotBreakBuild is motor-temas-v2.md
// §2.2's third offline call site (pptx.go:pptxAddChart, the one the plan
// initially missed — offline.go's tryBuildNativeContext and its
// chromium.ChartFetcher were the other two): a themed chart-cat-* must
// reach RenderChartNativePNGWithColors here too, or --format pptx would be
// the only output left with the fixed palette while HTML/PDF already
// respect the theme. This doesn't assert on decoded pixel colors (core's
// own PR #224 tests already cover RenderChartNativePNGWithColors's color
// application) — it proves the plumbing compiles, threads through
// pptxAddElement -> pptxAddChart, and produces a valid, non-empty PPTX
// package for a themed deck with a native-capable chart.
func TestGeneratePPTX_ThemeCategoricalColorsDoNotBreakBuild(t *testing.T) {
	dir := t.TempDir()

	p := nativePos()
	block := ast.NewContentBlock(p, "content")
	chart := ast.NewChartElement(p, "bar")
	chart.Data = [][]interface{}{{"A", 10.0}, {"B", 20.0}}
	chart.Labels = []string{"A", "B"}
	block.Elements = append(block.Elements, chart)

	doc := ast.NewAST(p)
	doc.FilePath = "themed.slidelang"
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	theme := &themes.Theme{
		Name: "with-tokens",
		Variables: themes.ThemeVariables{
			"--slidelang-chart-cat-1": "#aabbcc",
			"--slidelang-chart-cat-2": "#ddeeff",
		},
	}

	g := New(util.NewNoop())
	opts := GeneratorOptions{AssetRoot: dir, ResolvedTheme: theme}
	if err := g.generatePPTX(doc, dir, opts); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	outputPath := filepath.Join(dir, "themed.pptx")
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("expected output file %s to exist: %v", outputPath, err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}
}
