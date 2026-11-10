// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

var updateFenceGolden = flag.Bool("update-fence-golden", false, "rewrite testdata/fences/*.golden")

// TestFenceGolden fija cómo se leen las cercas de testdata/fences: cercas de
// cuatro o más backticks, cierres con más backticks, cercas que se leen con la
// regla de las versiones anteriores y backticks en el info string, con sus
// avisos CODE002/CODE003/CODE004. Para regenerar:
// go test ./parser -run TestFenceGolden -update-fence-golden
func TestFenceGolden(t *testing.T) {
	sources, _ := filepath.Glob("testdata/fences/*.doclang")
	if len(sources) == 0 {
		t.Fatal("no sources in testdata/fences")
	}
	for _, src := range sources {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		got := dumpFences(string(data))
		golden := strings.TrimSuffix(src, ".doclang") + ".golden"
		if *updateFenceGolden {
			if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Errorf("%s changed:\n--- got\n%s\n--- want\n%s", src, got, want)
		}
	}
}

func dumpFences(src string) string {
	p := New(util.NewNoop())
	p.SetNormalization(false)
	doc, diags := p.ParseDocument(src, "")
	var b strings.Builder
	for _, block := range doc.ContentBlocks {
		title, _ := block.SectionTitle()
		fmt.Fprintf(&b, "section %q\n", title)
		for _, el := range block.Elements {
			switch e := el.(type) {
			case *ast.CodeElement:
				fmt.Fprintf(&b, "  code %q line %d: %q\n", e.Language, e.Position.Line, e.Content)
			case *ast.CodeGroupElement:
				for _, cb := range e.CodeBlocks {
					fmt.Fprintf(&b, "  code-group tab %q line %d: %q\n", cb.Language, e.Position.Line, cb.Content)
				}
			case *ast.MermaidElement:
				fmt.Fprintf(&b, "  mermaid line %d: %q\n", e.Position.Line, e.Content)
			case *ast.TextElement:
				fmt.Fprintf(&b, "  text line %d: %q\n", e.Position.Line, e.Content)
			default:
				fmt.Fprintf(&b, "  %T line %d\n", el, el.GetPosition().Line)
			}
		}
	}
	var lines []string
	for _, d := range diags {
		if strings.HasPrefix(d.RuleID, "CODE") {
			lines = append(lines, fmt.Sprintf("%s line %d: %s", d.RuleID, d.Position.Line, d.Message))
		}
	}
	sort.Strings(lines)
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	return b.String()
}
