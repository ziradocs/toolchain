// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"
)

// La prosa con listas escapa el HTML del autor igual que el camino de siempre.
func TestProseWithListsEscapes(t *testing.T) {
	got := renderProseWithLists("<b>x</b>\n- <script>a</script>\n  1. **b**", true)
	if strings.Contains(got, "<script>") || strings.Contains(got, "<b>x") {
		t.Fatalf("unescaped author HTML: %s", got)
	}
	if !strings.Contains(got, "<ol><li><strong>b</strong></li></ol>") {
		t.Fatalf("inline markdown lost: %s", got)
	}
}
