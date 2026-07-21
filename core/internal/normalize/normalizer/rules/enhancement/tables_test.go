// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import "testing"

// TestTablesRule_Apply_DoesNotRewriteJSONInsideCodeFence cubre issue #204:
// isInSpecialBlock no reconocía fences de triple backtick (```chart,
// ```map, etc.), así que líneas "key": value dentro de un fence se
// reescribían como tabla Markdown.
func TestTablesRule_Apply_DoesNotRewriteJSONInsideCodeFence(t *testing.T) {
	rule := NewTablesRule()

	tests := []struct {
		name  string
		input string
	}{
		{
			name: "chart fence with 3+ consecutive key:value JSON lines",
			input: "# Chart\n\n```chart\n{\n" +
				"  \"type\": \"bar\",\n" +
				"  \"zoom\": 12,\n" +
				"  \"title\": \"Sales\"\n" +
				"}\n```\n",
		},
		{
			// Regression test para el hallazgo de code-review sobre el fix
			// original (PR de issue #204): una implementación ingenua de
			// isInCodeFence (toggle por prefix match en vez de exact match
			// para el cierre) trataba CUALQUIER línea que empezara con
			// "```" -- incluida una línea de CONTENIDO dentro del fence que
			// documenta su propia sintaxis -- como el cierre, cerrando el
			// fence prematuramente y dejando las líneas key:value
			// siguientes vulnerables a la reescritura de tabla otra vez.
			// El parser real (internal/elements/code.go, parseFlexCode)
			// solo cierra con un "```" EXACTO tras trim -- esta regla debe
			// espejar esa asimetría exactamente.
			name: "fence content line starting with triple backticks does not close the fence early",
			input: "# Chart\n\n```chart\n{\n" +
				"  \"note\": \"use ```json for inline examples\",\n" +
				"  \"type\": \"bar\",\n" +
				"  \"zoom\": 12,\n" +
				"  \"title\": \"Sales\"\n" +
				"}\n```\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rule.Apply(tc.input)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got != tc.input {
				t.Fatalf("Apply() rewrote content inside a code fence, want no-op\n--- input ---\n%s\n--- got ---\n%s", tc.input, got)
			}
		})
	}
}

// TestTablesRule_Apply_StillRewritesTableOutsideFence confirma que el fix
// de isInCodeFence no desactivó la funcionalidad real de la regla: 3+
// líneas key:value FUERA de cualquier fence siguen convirtiéndose en tabla.
func TestTablesRule_Apply_StillRewritesTableOutsideFence(t *testing.T) {
	rule := NewTablesRule()
	input := "# Config\n\nname: value1\ntype: value2\nowner: value3\n"

	got, err := rule.Apply(input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got == input {
		t.Fatalf("Apply() left 3+ key:value lines outside any fence unchanged, want a Markdown table rewrite:\n%s", got)
	}
}
