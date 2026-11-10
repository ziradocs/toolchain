// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import "testing"

// MermaidRule convierte solo una cerca ```mermaid que cierra con la regla de
// internal/fence: un ````mermaid con un ``` adentro cierra en su ````, y un
// ```mermaid sin cierre queda como está.
func TestMermaidRule_FenceRule(t *testing.T) {
	r := NewMermaidRule()
	for in, want := range map[string]string{
		"```mermaid\ngraph TD\n```":           "<<mermaid>>\n  graph TD",
		"````mermaid\ngraph TD\n```\nB\n````": "<<mermaid>>\n  graph TD\n  ```\n  B",
		"```mermaid\ngraph TD\nsin cerrar":    "```mermaid\ngraph TD\nsin cerrar",
		"texto ```mermaid\nno es cerca\n```":  "texto ```mermaid\nno es cerca\n```",
	} {
		got, err := r.Apply(in)
		if err != nil || got != want {
			t.Errorf("Apply(%q) = %q, want %q (%v)", in, got, want, err)
		}
	}
}

// isInCodeFence sigue la regla del parser: ``` cierra con ````, y una línea
// ```js` no abre.
func TestTablesRule_IsInCodeFenceUsesTheFenceRule(t *testing.T) {
	r := &TablesRule{}
	lines := []string{"```", "a", "````", "b", "```js` x", "c"}
	for i, want := range []bool{false, true, true, false, false, false} {
		if got := r.isInCodeFence(lines, i); got != want {
			t.Errorf("line %d (%q): inside=%v, want %v", i, lines[i], got, want)
		}
	}
}
