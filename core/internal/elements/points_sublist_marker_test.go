// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

func parsePoints(t *testing.T, mode string, lines ...string) *ast.PointsElement {
	t.Helper()
	ctx := &ParseContext{Mode: mode, Lines: lines, Logger: util.NewNoop()}
	res := (&PointsParser{}).Parse(ctx, 0)
	pts, ok := res.Element.(*ast.PointsElement)
	if !ok {
		t.Fatalf("element is %T", res.Element)
	}
	return pts
}

// The marker of the first sub-item is kept in memory for the render, in both
// dialects and without nested-list-types-v1, and it never reaches the JSON.
func TestPointItemKeepsTheSubListMarkerInMemoryOnly(t *testing.T) {
	cases := []struct {
		name  string
		pts   *ast.PointsElement
		first string
		sec   string
	}{
		{"flex", parsePoints(t, "flex", "1. uno", "   - a", "   - b", "2. dos", "   1. x"), "unordered", "ordered"},
		{"strict", parsePoints(t, "strict", "POINTS", "  - uno", "    1. a", "  - dos", "    - b"), "ordered", "unordered"},
	}
	for _, c := range cases {
		if len(c.pts.Items) != 2 {
			t.Fatalf("%s: %d items", c.name, len(c.pts.Items))
		}
		if got := c.pts.Items[0].SubListMarker; got != c.first {
			t.Errorf("%s: first item marker %q, want %q", c.name, got, c.first)
		}
		if got := c.pts.Items[1].SubListMarker; got != c.sec {
			t.Errorf("%s: second item marker %q, want %q", c.name, got, c.sec)
		}
		raw, err := json.Marshal(c.pts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(raw)), "sublistmarker") {
			t.Errorf("%s: the marker leaked into the JSON: %s", c.name, raw)
		}
		if c.pts.Items[0].SubListType != "" {
			t.Errorf("%s: SubListType is the capability's field and must stay empty, got %q", c.name, c.pts.Items[0].SubListType)
		}
	}
}
