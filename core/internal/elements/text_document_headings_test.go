// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/util"
)

// TestTextParser_DocumentHeadingsInterruptParagraph: con DocumentHeadings
// (lo fija solo DocumentFlexParser) un `#`/`##` o un `---` pegado a un
// párrafo lo corta. Sin él (slidelang) el comportamiento es el de siempre:
// `# B` y `---` quedan en el texto, `## B` ya cortaba.
func TestTextParser_DocumentHeadingsInterruptParagraph(t *testing.T) {
	slides := map[string]int{"# B": 3, "## B": 1, "---": 3}
	for next, slideWant := range slides {
		lines := []string{"line1", next, "after"}
		for _, doc := range []bool{false, true} {
			ctx := &ParseContext{Mode: "flex", Lines: lines, Logger: util.NewNoop(), DocumentHeadings: doc}
			res := (&TextParser{}).Parse(ctx, 0)
			want := slideWant
			if doc {
				want = 1
			}
			if res.ConsumedLines != want {
				t.Errorf("next=%q DocumentHeadings=%v: consumed %d, want %d", next, doc, res.ConsumedLines, want)
			}
		}
	}
}
