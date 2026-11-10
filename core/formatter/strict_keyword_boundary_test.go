// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Issue #489: una cita cuyo texto empieza con "TEXTO" se escribe y se relee
// igual, porque el parser ya no la corta ahí. Antes fmt la rechazaba.
func TestFormatStrict_QuoteStartingWithGluedKeywordRoundTrips(t *testing.T) {
	for _, content := range []string{"TEXTO citado", "TEXT-heavy slides", "MAPA norte"} {
		q := ast.NewQuoteElement(diagnostics.NewPosition(3, 1), content)
		q.Author = "Ana"
		out, err := FormatStrict(chartDoc(q))
		if err != nil {
			t.Fatalf("%q: FormatStrict: %v", content, err)
		}
		got := parseSlides(t, out)
		if want := quoteContents(chartDoc(q)); len(quoteContents(got)) != 1 || quoteContents(got)[0] != want[0] {
			t.Errorf("%q: quote changed: %q\n%s", content, quoteContents(got), out)
		}
	}
}
