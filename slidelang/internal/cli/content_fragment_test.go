// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/fragment"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFmtContentFragmentRealDispatchAndAssembly(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "original.md")
	out := filepath.Join(dir, "fragment.strict")
	side := filepath.Join(dir, "receipt.json")
	original := "## Supplied\n\n```bash\n  command\n  \targument\n\n```\n\nAfter"
	if err := os.WriteFile(input, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := NewFmtCommand()
	cmd.SetArgs([]string{input, "--content-fragment", "--fragment-namespace", "deck/slot/block", "--fragment-receipt", side, "--output", out})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(input)
	if string(raw) != original {
		t.Fatal("original changed")
	}
	bytes, _ := os.ReadFile(out)
	receiptBytes, _ := os.ReadFile(side)
	var r fragment.ContentFragmentReceipt
	if err := json.Unmarshal(receiptBytes, &r); err != nil {
		t.Fatal(err)
	}
	if !r.Accepted || !r.OutputProduced || r.OriginalMarkdown != original || r.SourceSHA256 == r.EffectiveSourceSHA256 {
		t.Fatal("receipt inaccurate")
	}
	if strings.Contains(string(bytes), "SLIDE") {
		t.Fatal("fragment gained outline")
	}
	wrapper := "---\nmode: strict\nast_capabilities: [typed-headings-v1, content-markdown-v1]\n---\nSLIDE content\n" + string(bytes)
	doc, ds := parser.New(util.NewNoop()).Parse(wrapper, "")
	for _, d := range ds {
		if d.IsError() {
			t.Fatal(d)
		}
	}
	if len(doc.ContentBlocks) != 1 || doc.ContentBlocks[0].Elements[0].(*ast.HeadingElement).Level != 2 || doc.ContentBlocks[0].Elements[1].(*ast.CodeElement).Content != "  command\n  \targument\n" {
		t.Fatal("payload changed")
	}
}
func TestFmtContentFragmentRejectedAndFlags(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "original.md")
	_ = os.WriteFile(input, []byte("```bash\nUnclosed"), 0600)
	for _, extra := range [][]string{{"--write"}, {"--check"}, {"--strict=false"}, {"--normalize-rectangular-charts"}} {
		cmd := NewFmtCommand()
		args := []string{input, "--content-fragment", "--fragment-namespace", "n", "--fragment-receipt", filepath.Join(dir, "flags.json"), "--output", filepath.Join(dir, "flags.strict")}
		cmd.SetArgs(append(args, extra...))
		if err := cmd.Execute(); err == nil {
			t.Fatal("incompatible flags accepted")
		}
	}
	out := filepath.Join(dir, "rejected.strict")
	side := filepath.Join(dir, "rejected.json")
	cmd := NewFmtCommand()
	cmd.SetArgs([]string{input, "--content-fragment", "--fragment-namespace", "n", "--fragment-receipt", side, "--output", out})
	if err := cmd.Execute(); err == nil {
		t.Fatal("rejected fragment success")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("rejected output produced")
	}
	if _, err := os.Stat(side); err != nil {
		t.Fatal("receipt missing")
	}
}
