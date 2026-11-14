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
// aviso (fence.AtWithin, fenceDiagnostics). Con EarlierFences se lee siempre
// como en esas versiones, sin aviso.
func TestCodeParser_FenceCloseRule(t *testing.T) {
	for _, tc := range []struct {
		src, content string
		consumed     int
		rule         string
		earlier      bool
	}{
		// Una línea de cuatro backticks cierra una cerca de tres (CODE002).
		{"```\na\n````\nb\n```\nfin", "a", 3, LongerClosingFenceRuleID, false},
		// Una cerca de cuatro sin cierre de cuatro cierra en ``` (CODE003).
		{"````js\nq\n```\nfin", "q", 3, FenceFallbackRuleID, false},
		// Sin ningún cierre llega al final (CODE003).
		{"````js\nq\nfin", "q\nfin", 3, FenceFallbackRuleID, false},
		// Una cerca de cuatro guarda un ``` y cierra en su ````. Antes
		// cerraba en el ```, pero esa lectura dejaba abierta la línea ````
		// hasta el final: no hay CODE004.
		{"````md\n```\nx\n````\nfin", "```\nx", 4, "", false},
		// Antes la leían bien formada (la ```` abría una cerca que cerraba
		// la ``` siguiente): CODE004.
		{"````md\n```\nx\n````\nb\n```", "```\nx", 4, FenceExtentChangedRuleID, false},
		// ```x no cierra.
		{"```\na\n```x\n```", "a\n```x", 4, "", false},
		// La regla anterior: solo una ``` exacta cierra.
		{"```\na\n````\nb\n```\nfin", "a\n````\nb", 5, "", true},
		{"````md\n```\nx\n````\nfin", "", 2, "", true},
	} {
		ctx := &ParseContext{Mode: "flex", Lines: strings.Split(tc.src, "\n"), EarlierFences: tc.earlier}
		res := (&CodeParser{}).Parse(ctx, 0)
		code, ok := res.Element.(*ast.CodeElement)
		if !ok || code.Content != tc.content || res.ConsumedLines != tc.consumed {
			t.Errorf("%q (earlier %v): content %q consumed %d, want %q %d", tc.src, tc.earlier, code.Content, res.ConsumedLines, tc.content, tc.consumed)
		}
		var rules []string
		for _, d := range res.Diagnostics {
			rules = append(rules, d.RuleID)
		}
		if got := strings.Join(rules, ","); got != tc.rule {
			t.Errorf("%q (earlier %v): diagnostics %v, want %q", tc.src, tc.earlier, rules, tc.rule)
		}
	}
}

// El lenguaje de una cerca sigue a todos sus backticks: antes se cortaban
// tres y el de una cerca ````md quedaba como "`md". Con la regla anterior,
// como antes.
func TestCodeParser_LanguageAfterAllBackticks(t *testing.T) {
	for _, earlier := range []bool{false, true} {
		ctx := &ParseContext{Mode: "flex", Lines: strings.Split("````md\nx\n````", "\n"), EarlierFences: earlier}
		code := (&CodeParser{}).Parse(ctx, 0).Element.(*ast.CodeElement)
		want := "md"
		if earlier {
			want = "`md"
		}
		if code.Language != want {
			t.Errorf("earlier %v: language %q, want %q", earlier, code.Language, want)
		}
	}
}

// Las cercas de diagrama y de datos se leen igual que un bloque de código
// (fence.AtWithin): ``` cierra con ````` (CODE002; antes cerraba en ``` o en
// ````) y ```` sin cierre de cuatro cierra en ``` (CODE003).
func TestDiagramFences_CloseLikeCode(t *testing.T) {
	for _, tc := range []struct {
		parser   ElementParser
		src      string
		consumed int
		rule     string
	}{
		{&MermaidParser{}, "```mermaid\ngraph TD\n`````\nfin", 3, LongerClosingFenceRuleID},
		{&MermaidParser{}, "````mermaid\ngraph TD\n```\nfin", 3, FenceFallbackRuleID},
		{&MermaidParser{}, "```mermaid\ngraph TD\n````\nfin", 3, ""},
		{&PlantUMLParser{}, "```plantuml\nA -> B\n`````\nfin", 3, LongerClosingFenceRuleID},
		{&PlantUMLParser{}, "````plantuml\nA -> B\n```\nfin", 3, FenceFallbackRuleID},
		{&ChartParser{}, "```chart\n{}\n`````\nfin", 3, LongerClosingFenceRuleID},
		{&ChartParser{}, "````chart\n{}\n```\nfin", 3, FenceFallbackRuleID},
		{&MapParser{}, "```map\n{}\n`````\nfin", 3, LongerClosingFenceRuleID},
		{&MapParser{}, "````map\n{}\n```\nfin", 3, FenceFallbackRuleID},
		// Una cerca de cuatro que guarda un ```: cierra en su ````.
		{&MermaidParser{}, "````mermaid\ngraph TD\n```\nA-->B\n````\nfin", 5, ""},
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

// Una línea con un backtick en el info string que fence.AtWithin lee como
// cerca (OpenedByMain) corta el párrafo anterior y la lee CodeParser por el
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

// Dentro de un :::code-group una línea que empieza con ``` siempre abre una
// pestaña, también con un backtick en el info string (como antes): una línea
// de texto no tiene lugar en el grupo.
func TestCodeGroup_BacktickInfoLineOpensATab(t *testing.T) {
	src := ":::code-group\n```js` [a]\nx\n```\n```py [b]\ny\n```\n:::"
	ctx := &ParseContext{Mode: "flex", Lines: strings.Split(src, "\n")}
	res := (&CodeGroupParser{}).Parse(ctx, 0)
	group := res.Element.(*ast.CodeGroupElement)
	if res.Error != nil || len(group.CodeBlocks) != 2 || group.CodeBlocks[0].Content != "x" || group.CodeBlocks[1].Content != "y" {
		t.Fatalf("want two tabs: %#v err %v", group.CodeBlocks, res.Error)
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].RuleID != FenceFallbackRuleID {
		t.Errorf("want one CODE003 at the tab: %v", res.Diagnostics)
	}
}

// Una línea que empieza con ``` pero no abre una cerca (internal/fence) no
// abre otro bloque: continúa un item y no corta la prosa.
func TestFenceRule_BacktickInfoLineIsNotABlock(t *testing.T) {
	if StartsOtherBlock("```js` sigue") {
		t.Error("StartsOtherBlock must not take ```js` as a fence")
	}
	if !StartsOtherBlock("```go") || !StartsOtherBlock("````md") {
		t.Error("StartsOtherBlock must take a real fence")
	}
	if IsNewElement("```js` sigue", "flex") {
		t.Error("IsNewElement must not take ```js` as a code block")
	}
	if !IsNewElement("````md", "flex") {
		t.Error("IsNewElement must take ````md as a code block")
	}
}
