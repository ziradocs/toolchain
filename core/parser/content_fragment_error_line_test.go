// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

// En un fragmento de contenido, el error de una línea interna de un elemento
// (un marcador de <<map>>) sale en esa línea, como en un documento; el
// fragmento sigue rechazándose por ese error.
func TestContentFragmentElementErrorLine(t *testing.T) {
	nodes, diags := ParseMarkdownContent("texto\n\n<<map>>\ncenter: 10, 20\nmarker: 10, abc, X\n<</map>>\n", ContentOptions{AnchorNamespace: "doc/slot"})
	if nodes != nil {
		t.Errorf("the fragment should still be rejected, got %+v", nodes)
	}
	wantDiagAt(t, diags, mapMarkerMsg, diagnostics.Error, 5, 0)
}
