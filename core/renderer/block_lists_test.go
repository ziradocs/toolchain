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

// En la prosa de una cita o un bloque, una línea que abre otro bloque no se
// une al item de arriba aunque esté sangrada; un número que no es 1 debajo de
// un item es texto de ese item; y un tabulador sangra a la parada de 4.
func TestProseListsInQuotesAndCallouts(t *testing.T) {
	for _, line := range []string{"  ```go", "  $$x^2$$", "  ---", "  <<chart: bar>>", "  # h"} {
		got := renderProseWithLists("- a\n"+line, true)
		if !strings.Contains(got, "<ul><li>a</li></ul>") {
			t.Errorf("%q joined the item: %s", line, got)
		}
	}
	if got := renderProseWithLists("- a\n  1998. el año", true); got != "<ul><li>a 1998. el año</li></ul>" {
		t.Errorf("ordinal: %s", got)
	}
	if got := renderProseWithLists("- a\n\t- b", true); got != "<ul><li>a<ul><li>b</li></ul></li></ul>" {
		t.Errorf("tab: %s", got)
	}
}
