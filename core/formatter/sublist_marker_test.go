// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func firstPoints(doc *ast.AST) *ast.PointsElement {
	for _, b := range doc.ContentBlocks {
		for _, el := range b.Elements {
			if p, ok := el.(*ast.PointsElement); ok {
				return p
			}
		}
	}
	return nil
}

// Without nested-list-types-v1 the parser still records the marker kind of each
// sublist in memory, and fmt writes it back: a numbered sublist stayed numbered and
// a bulleted one stayed bulleted, where every sublist used to come out as bullets.
func TestFormatStrict_KeepsTheMarkerOfASublist(t *testing.T) {
	for name, src := range map[string]string{
		"numbers under a bullet": "---\nmode: flex\n---\n\n## One\n\n- uno\n  1. a\n  2. b\n- dos\n",
		"bullets under a number": "---\nmode: flex\n---\n\n## One\n\n1. uno\n   - a\n   - b\n2. dos\n",
		"numbers under numbers":  "---\nmode: flex\n---\n\n## One\n\n1. uno\n   1. a\n   2. b\n2. dos\n",
	} {
		t.Run(name, func(t *testing.T) {
			want := firstPoints(parseSlides(t, src))
			out, reparsed := transpile(t, src)
			got := firstPoints(reparsed)
			for i := range want.Items {
				if want.Items[i].SubListMarker != got.Items[i].SubListMarker {
					t.Errorf("item %d: marker %q before, %q after\n%s", i, want.Items[i].SubListMarker, got.Items[i].SubListMarker, out)
				}
			}
			again, err := FormatStrict(reparsed)
			if err != nil || again != out {
				t.Errorf("not idempotent (%v):\n%s\n---\n%s", err, out, again)
			}
		})
	}

	out, _ := transpile(t, "---\nmode: flex\n---\n\n## One\n\n- uno\n  1. a\n  2. b\n")
	if !strings.Contains(out, "1. a") || !strings.Contains(out, "2. b") {
		t.Errorf("a numbered sublist must be written with numbers:\n%s", out)
	}
}
