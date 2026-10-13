// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/transform"
)

const chartSource = "---\nmode: strict\n---\nSLIDE content\n  title: \"First\"\n  TEXT\n    Intro\nSLIDE content\n  title: \"Chart\"\n  <!-- node-id: chart-a3 -->\n  <<chart: bar>>\n  {\"data\":[[\"North\",120],[\"South\",95],[\"West\",80]],\"series\":[\"Unidades\"],\"unit\":\"unidades\",\"showDataLabels\":true}\n  <</chart>>\nSLIDE content\n  title: \"Last\"\n  TEXT\n    Conclusion\n"

func fileDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func chartFrom(doc *ast.AST) *ast.ChartElement {
	var found *ast.ChartElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if c, ok := n.(*ast.ChartElement); ok {
			found = c
		}
		return nil
	})
	return found
}
func TestFmtRectangularChartsPortableSource(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "original.slidelang")
	output := filepath.Join(dir, "effective.slidelang")
	side := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(input, []byte(chartSource), 0600); err != nil {
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
	original, _ := os.ReadFile(input)
	if string(original) != chartSource {
		t.Fatal("original changed")
	}
	doc, diags := parser.New(nil).Parse(string(effective), output)
	for _, d := range diags {
		if d.IsError() {
			t.Fatal(d.String())
		}
	}
	chart := chartFrom(doc)
	if len(doc.ContentBlocks) != 3 || chart == nil || chart.IsJSONMode || chart.RawJSON != nil || chart.NodeID != "chart-a3" || chart.Unit == nil || *chart.Unit != "unidades" || chart.ShowDataLabels == nil || !*chart.ShowDataLabels || len(chart.Data) != 3 {
		t.Fatalf("effective chart lost content: %+v", chart)
	}
	b, err := os.ReadFile(side)
	if err != nil {
		t.Fatal(err)
	}
	var receipt transform.ChartNormalizationReceipt
	if err = json.Unmarshal(b, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Version != "rectangular-chart-normalization-v1" || !receipt.Accepted || !receipt.OutputProduced || receipt.SourceSHA256 != fileDigest(original) || receipt.EffectiveSourceSHA256 != fileDigest(effective) || receipt.OriginalSource != chartSource {
		t.Fatalf("receipt: %+v", receipt)
	}
}
func TestFmtRectangularChartsRejectsWithoutPartialSource(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "original.slidelang")
	output := filepath.Join(dir, "effective.slidelang")
	side := filepath.Join(dir, "receipt.json")
	source := strings.Replace(chartSource, `"series":["Unidades"]`, `"series":["Unidades"],"unknown":true`, 1)
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := NewFmtCommand()
	cmd.SetArgs([]string{input, "--normalize-rectangular-charts", "--chart-normalization-receipt", side, "--output", output})
	if err := cmd.Execute(); err == nil {
		t.Fatal("unsupported JSON accepted")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("partial source produced")
	}
	b, err := os.ReadFile(side)
	if err != nil {
		t.Fatal(err)
	}
	var r transform.ChartNormalizationReceipt
	if err = json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if r.Accepted || r.OutputProduced || len(r.Outcomes) != 1 || r.Outcomes[0].Status != "rejected" {
		t.Fatalf("%+v", r)
	}
	// An ordinary fmt call keeps opaque JSON, rather than implicitly converting it.
	normal := NewFmtCommand()
	normal.SetArgs([]string{input, "--output", output})
	if err := normal.Execute(); err != nil {
		t.Fatal(err)
	}
	native, _ := os.ReadFile(output)
	doc, _ := parser.New(nil).Parse(string(native), output)
	if !chartFrom(doc).IsJSONMode {
		t.Fatal("ordinary fmt converted opaque JSON")
	}
}
func TestFmtRectangularChartsRequiresExplicitFreshOutputs(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "original.slidelang")
	side := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(input, []byte(chartSource), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{input, "--normalize-rectangular-charts"}, {input, "--normalize-rectangular-charts", "--chart-normalization-receipt", side, "--output", input}} {
		cmd := NewFmtCommand()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatal("unsafe invocation accepted")
		}
	}
	original, _ := os.ReadFile(input)
	if string(original) != chartSource {
		t.Fatal("original overwritten")
	}
}

func TestFmtOutputRejectsWriteAndCheck(t *testing.T) {
	for _, flag := range []string{"--write", "--check"} {
		cmd := NewFmtCommand()
		cmd.SetArgs([]string{"unused.slidelang", "--output", "unused-output.slidelang", flag})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatalf("ambiguous output accepted: %v", err)
		}
	}
}
