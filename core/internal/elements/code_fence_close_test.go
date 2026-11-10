// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// Una cerca flex cierra con la regla de CommonMark cuando da un resultado
// bien formado, y si no, donde cerraba en las versiones anteriores, con un
// aviso (fence.At, fenceDiagnostics).
func TestCodeParser_FenceCloseRule(t *testing.T) {
	for _, tc := range []struct {
		src, content string
		consumed     int
		rule         string
	}{
		// Una línea de cuatro backticks cierra una cerca de tres (CODE002).
		{"```\na\n````\nb\n```\nfin", "a", 3, LongerClosingFenceRuleID},
		// Una cerca de cuatro sin cierre de cuatro cierra en ``` (CODE003).
		{"````js\nq\n```\nfin", "q", 3, FenceFallbackRuleID},
		// Sin ningún cierre llega al final (CODE003).
		{"````js\nq\nfin", "q\nfin", 3, FenceFallbackRuleID},
		// Una cerca de cuatro guarda un ``` y cierra en su ```` (CODE004:
		// antes cerraba en el ```).
		{"````md\n```\nx\n````\nfin", "```\nx", 4, FenceExtentChangedRuleID},
		// ```x no cierra.
		{"```\na\n```x\n```", "a\n```x", 4, ""},
	} {
		ctx := &ParseContext{Mode: "flex", Lines: strings.Split(tc.src, "\n")}
		var recorded [][2]int
		ctx.RecordCode = func(line, n int) { recorded = append(recorded, [2]int{line, n}) }
		res := (&CodeParser{}).Parse(ctx, 0)
		code, ok := res.Element.(*ast.CodeElement)
		if !ok || code.Content != tc.content || res.ConsumedLines != tc.consumed {
			t.Errorf("%q: content %q consumed %d, want %q %d", tc.src, code.Content, res.ConsumedLines, tc.content, tc.consumed)
		}
		var rules []string
		for _, d := range res.Diagnostics {
			rules = append(rules, d.RuleID)
		}
		if got := strings.Join(rules, ","); got != tc.rule {
			t.Errorf("%q: diagnostics %v, want %q", tc.src, rules, tc.rule)
		}
		if len(recorded) != 1 || recorded[0] != [2]int{1, tc.consumed} {
			t.Errorf("%q: RecordCode got %v, want the %d consumed lines from line 1", tc.src, recorded, tc.consumed)
		}
	}
}

// Un code-group avisa a RecordCode su cabecera y cada pestaña completa, no las
// líneas sueltas entre pestañas.
func TestCodeGroupParser_RecordsHeaderAndTabs(t *testing.T) {
	src := ":::code-group\nsuelta\n```js [a]\nx\n```\n:::"
	ctx := &ParseContext{Mode: "flex", Lines: strings.Split(src, "\n"), LineOffset: 10}
	var recorded [][2]int
	ctx.RecordCode = func(line, n int) { recorded = append(recorded, [2]int{line, n}) }
	(&CodeGroupParser{}).Parse(ctx, 0)
	if want := [][2]int{{11, 1}, {13, 3}}; len(recorded) != 2 || recorded[0] != want[0] || recorded[1] != want[1] {
		t.Errorf("RecordCode got %v, want %v", recorded, want)
	}
}

// Las cercas de diagrama y de datos se leen igual que un bloque de código
// (fence.At): ``` cierra con ```` (CODE002) y ```` sin cierre de cuatro
// cierra en ``` (CODE003).
func TestDiagramFences_CloseLikeCode(t *testing.T) {
	for _, tc := range []struct {
		parser   ElementParser
		src      string
		consumed int
		rule     string
	}{
		{&MermaidParser{}, "```mermaid\ngraph TD\n````\nfin", 3, LongerClosingFenceRuleID},
		{&MermaidParser{}, "````mermaid\ngraph TD\n```\nfin", 3, FenceFallbackRuleID},
		{&PlantUMLParser{}, "```plantuml\nA -> B\n````\nfin", 3, LongerClosingFenceRuleID},
		{&PlantUMLParser{}, "````plantuml\nA -> B\n```\nfin", 3, FenceFallbackRuleID},
		{&ChartParser{}, "```chart\n{}\n````\nfin", 3, LongerClosingFenceRuleID},
		{&ChartParser{}, "````chart\n{}\n```\nfin", 3, FenceFallbackRuleID},
		{&MapParser{}, "```map\n{}\n````\nfin", 3, LongerClosingFenceRuleID},
		{&MapParser{}, "````map\n{}\n```\nfin", 3, FenceFallbackRuleID},
	} {
		ctx := &ParseContext{Mode: "flex", Lines: strings.Split(tc.src, "\n")}
		res := tc.parser.Parse(ctx, 0)
		var rules []string
		for _, d := range res.Diagnostics {
			if strings.HasPrefix(d.RuleID, "CODE") {
				rules = append(rules, d.RuleID)
			}
		}
		if res.ConsumedLines != tc.consumed || strings.Join(rules, ",") != tc.rule {
			t.Errorf("%T %q: consumed %d rules %v, want %d %q", tc.parser, tc.src, res.ConsumedLines, rules, tc.consumed, tc.rule)
		}
	}
}

// Una línea con un backtick en el info string que fence.At lee como cerca
// (OpenedByMain) corta el párrafo anterior y la lee CodeParser por el
// registry (CanParseAt).
func TestBacktickInfoFence_CutsProseAndDispatches(t *testing.T) {
	ctx := &ParseContext{Mode: "flex", Lines: strings.Split("Intro.\n```ts title=`a.ts`\ncode\n```\nfin", "\n")}
	res := (&TextParser{}).Parse(ctx, 0)
	if res.ConsumedLines != 1 {
		t.Fatalf("the paragraph must stop before the fence: consumed %d", res.ConsumedLines)
	}
	res = GetDefaultRegistry().Parse(ctx, 1)
	if code, ok := res.Element.(*ast.CodeElement); !ok || code.Content != "code" || res.ConsumedLines != 3 {
		t.Fatalf("want the code block: %#v consumed %d", res.Element, res.ConsumedLines)
	}
}
