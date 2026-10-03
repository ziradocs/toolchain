// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package css

import (
	"os"
	"strings"
	"testing"
)

// The marker of a list item belongs to the list that is its direct parent. With
// the descendant combinator (`ol li`), a bullet list nested inside an ordered one
// matched the ordered rules and was drawn as numbers that continued the outer
// count (1. one / 2. a / 3. b / 4. two), in the HTML and in the PDF alike.
func TestPointsListMarkersAreScopedToTheirOwnList(t *testing.T) {
	raw, err := os.ReadFile("assets/css/elements/text.css")
	if err != nil {
		t.Fatalf("read text.css: %v", err)
	}
	css := string(raw)

	for _, want := range []string{
		".slidelang-element.slidelang-points ol > li:before",
		".slidelang-element.slidelang-points ol > li {",
		".slidelang-element.slidelang-points ul > li:before",
		".slidelang-element.slidelang-points li > ul > li:before",
		".slidelang-element.slidelang-points ol ol > li:before",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("text.css lacks the rule %q", want)
		}
	}
	// The descendant forms are the bug: they reach the items of any list nested
	// below, whatever its own type is.
	for _, bad := range []string{
		".slidelang-points ol li {",
		".slidelang-points ol li:before",
		".slidelang-points ul li:before",
	} {
		if strings.Contains(css, bad) {
			t.Errorf("text.css still has the descendant rule %q", bad)
		}
	}
}
