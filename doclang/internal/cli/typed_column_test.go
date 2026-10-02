// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"strings"
	"testing"
)

// Issue #373: una columna tipada en un .doclang compila (con node-id) y su
// contenido llega al Markdown junto con el de la columna cruda.
func TestBuild_TypedColumnWithNodeID(t *testing.T) {
	fixture := "---\ntitle: \"Doc\"\nmode: flex\n---\n\n# Report\n\n::: grid\n::: column typed\n<!-- node-id: ColTextA -->\nLeft side.\n:::\n::: column\nRight side.\n:::\n:::\n"
	content := buildMarkdown(t, fixture)
	for _, want := range []string{"Left side.", "Right side."} {
		if !strings.Contains(content, want) {
			t.Fatalf("markdown missing %q:\n%s", want, content)
		}
	}
}
