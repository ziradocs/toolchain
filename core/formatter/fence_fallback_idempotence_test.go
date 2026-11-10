// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"testing"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// P3 del review de #498: fmt sobre una cerca que se lee con la regla de las
// versiones anteriores (sin cierre de CommonMark, sin cierre en absoluto, o con
// un backtick en el info string) es idempotente: la segunda pasada no cambia
// nada.
func TestFormatDocument_FenceFallbackIsIdempotent(t *testing.T) {
	for _, src := range []string{
		"# A\n\n````js\nq\n```\n\n## T\n\nfin\n",
		"# A\n\n````js\nq\n",
		"# A\n\n````math\nx^2\n",
		"# A\n\n```math\nx^2\n",
		"# A\n\n````mermaid\ngraph TD\n```\n\nfin\n",
		"# A\n\n<<mermaid>>\ngraph TD\n  A-->B\n\n## T\n",
		"# A\n\n```ts title=`a.ts`\ncode\n```\n\nfin\n",
		"# A\n\n:::code-group\n````js [a]\nx\n```\n:::\n",
		"# A\n\n```md\n````\ninner\n````\n```\n",
	} {
		first := format(t, src)
		if second := format(t, first); second != first {
			t.Errorf("fmt is not idempotent for %q:\nfirst:\n%s\nsecond:\n%s", src, first, second)
		}
	}
}

func format(t *testing.T, src string) string {
	t.Helper()
	doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "d.doclang")
	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument(%q): %v", src, err)
	}
	return out
}
