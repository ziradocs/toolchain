// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter_test

import (
	"strings"
	"testing"
)

// Issue #489: una cita cuya línea empieza con las letras de una palabra clave
// ("TEXTO citado") se formatea y se vuelve a leer igual. Antes el parser la
// leía como un TEXT, y la guarda del formatter rechazaba la línea por el mismo
// prefijo.
func TestFormatDocumentStrict_QuoteLineThatStartsWithAKeywordRoundTrips(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  QUOTE\n    TEXTO citado\n    AUTHOR: Ana\n"
	once := parseDoc(t, src)
	if !strings.Contains(once, "    TEXTO citado\n") || !strings.Contains(once, "AUTHOR: Ana") {
		t.Fatalf("the quote lost its text or its author:\n%s", once)
	}
	if twice := parseDoc(t, once); twice != once {
		t.Errorf("formatting is not stable:\n--- first ---\n%s\n--- second ---\n%s", once, twice)
	}
}
