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

// A sublist that mixes markers, and a third level (which the parser flattens
// into the sublist of the base item), come back with the markers the author
// wrote, build to the same AST, and the output is stable.
func TestFormatStrict_KeepsTheMarkersOfAMixedOrDeepSublist(t *testing.T) {
	for name, src := range map[string]string{
		"bullet then number": "---\nmode: flex\n---\n\n## One\n\n- uno\n  - a\n  1. b\n- dos\n",
		"number then bullet": "---\nmode: flex\n---\n\n## One\n\n1. uno\n   1. a\n   - b\n   2. c\n2. dos\n",
		"three levels":       "---\nmode: flex\n---\n\n## One\n\n- uno\n  - a\n    1. b\n      - c\n- dos\n",
	} {
		t.Run(name, func(t *testing.T) {
			diffs, out := flexCorpusASTDiffs(t, src)
			if len(diffs) != 0 {
				t.Fatalf("the AST changed:\n  %s\n%s", strings.Join(diffs, "\n  "), out)
			}
			want := firstPoints(parseSlides(t, src))
			got := firstPoints(parseSlides(t, out))
			for i := range want.Items {
				for j := range want.Items[i].SubPoints {
					if w, g := want.Items[i].SubPoints[j].Marker, got.Items[i].SubPoints[j].Marker; w != g {
						t.Errorf("item %d sub-point %d: marker %q before, %q after\n%s", i, j, w, g, out)
					}
				}
			}
			again, err := FormatStrict(parseSlides(t, out))
			if err != nil || again != out {
				t.Errorf("not idempotent (%v):\n%s\n---\n%s", err, out, again)
			}
		})
	}

	out, _ := transpile(t, "---\nmode: flex\n---\n\n## One\n\n- uno\n  - a\n  1. b\n")
	if !strings.Contains(out, "- a") || !strings.Contains(out, "1. b") {
		t.Errorf("a mixed sublist must keep each marker:\n%s", out)
	}
}

// An AST that came back from JSON has no per-item markers: the sublist is
// written with one marker, as before.
func TestFormatStrict_WithoutItemMarkersASublistUsesOneMarker(t *testing.T) {
	doc := parseSlides(t, "---\nmode: flex\n---\n\n## One\n\n- uno\n  - a\n  1. b\n")
	pts := firstPoints(doc)
	for i := range pts.Items[0].SubPoints {
		pts.Items[0].SubPoints[i].Marker = ""
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "- a") || !strings.Contains(out, "- b") || strings.Contains(out, "1. b") {
		t.Errorf("want every sub-point under the first marker:\n%s", out)
	}
}
