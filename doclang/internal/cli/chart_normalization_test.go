// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFmtRectangularChartsProducesStrictDoclang(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "original.doclang")
	output := filepath.Join(dir, "effective.doclang")
	side := filepath.Join(dir, "receipt.json")
	source := "# Chart\n\n<<chart: bar>>\n{\"data\":[[\"North\",120],[\"South\",95]],\"series\":[\"Unidades\"],\"unit\":\"\",\"showDataLabels\":false}\n<</chart>>\n"
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := NewFmtCommand()
	cmd.SetArgs([]string{input, "--normalize-rectangular-charts", "--chart-normalization-receipt", side, "--output", output})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	effective, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(effective), "mode: strict") || !strings.Contains(string(effective), "SECTION") {
		t.Fatal(string(effective))
	}
	doc, diags := parser.New(nil).ParseDocument(string(effective), output)
	for _, d := range diags {
		if d.IsError() {
			t.Fatal(d.String())
		}
	}
	var chart *ast.ChartElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if c, ok := n.(*ast.ChartElement); ok {
			chart = c
		}
		return nil
	})
	if chart == nil || chart.RawJSON != nil || chart.IsJSONMode || chart.Unit == nil || *chart.Unit != "" || chart.ShowDataLabels == nil || *chart.ShowDataLabels {
		t.Fatalf("lost empty/false: %+v", chart)
	}
}
