// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package frontmatter

import (
	"strings"
	"testing"
)

// TestInjectionRule_Apply_EmitsFlexFull confirma que InjectionRule (que
// agrega frontmatter/mode faltante a contenido sin normalizar) emite
// "mode: flex-full" — no "mode: flex-ai" — como parte de #210 (rename del
// modo user-facing; "flex-ai" pasa a ser un alias deprecado, ya no el
// nombre que el normalizador escribe).
func TestInjectionRule_Apply_EmitsFlexFull(t *testing.T) {
	r := NewInjectionRule()

	t.Run("missing frontmatter entirely", func(t *testing.T) {
		out, err := r.Apply("# Just a heading\n\nSome content.\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "mode: flex-full") {
			t.Errorf("expected injected frontmatter to contain 'mode: flex-full', got:\n%s", out)
		}
		if strings.Contains(out, "mode: flex-ai") {
			t.Errorf("did not expect injected frontmatter to contain 'mode: flex-ai', got:\n%s", out)
		}
	})

	t.Run("frontmatter present but mode missing", func(t *testing.T) {
		input := "---\ntitle: \"Some Title\"\n---\n\n# Heading\n\nContent.\n"
		out, err := r.Apply(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "mode: flex-full") {
			t.Errorf("expected mode-injected frontmatter to contain 'mode: flex-full', got:\n%s", out)
		}
		if strings.Contains(out, "mode: flex-ai") {
			t.Errorf("did not expect mode-injected frontmatter to contain 'mode: flex-ai', got:\n%s", out)
		}
	})
}
