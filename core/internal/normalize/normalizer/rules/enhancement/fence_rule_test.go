// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"
	"testing"
)

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

// P1-2 del review de #498: un ```mermaid dentro de otra cerca o de un
// :::code-group es código y no se reescribe como <<mermaid>>.
func TestMermaidRule_RespectsSurroundingCode(t *testing.T) {
	r := NewMermaidRule()
	for _, in := range []string{
		"````md\n```mermaid\ngraph TD\n```\n````",
		":::code-group\n```mermaid [diagrama]\ngraph TD\n```\n:::",
		"::::code-group\n```md [a]\nx\n```\n```mermaid [b]\ngraph TD\n```\n:::",
		"```js\n```mermaid\n",
	} {
		if got, err := r.Apply(in); err != nil || got != in {
			t.Errorf("Apply(%q) = %q, want it unchanged (%v)", in, got, err)
		}
	}
	// Después de la otra cerca, una ```mermaid suelta sí se convierte.
	got, _ := r.Apply("````md\n```mermaid\nA\n```\n````\n```mermaid\ngraph TD\n```")
	if want := "````md\n```mermaid\nA\n```\n````\n<<mermaid>>\n  graph TD"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// code_group_formatter escribe la pestaña de un :::code-item con una cerca
// más larga que cualquier línea de backticks de su contenido (fence.For).
func TestCodeGroupFormatter_LongerFenceForContentWithBackticks(t *testing.T) {
	in := ":::code-group\n:::code-item{title=\"a\"}\n````md\n```\nx\n```\n````\n:::\n:::"
	got, err := NewCodeGroupFormatterRule().Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "````md [a]\n```\nx\n```\n````") {
		t.Errorf("got %q", got)
	}
}

// Los formateadores de mermaid no tocan un <<mermaid>> escrito dentro de una
// cerca ````md que guarda un ``` (fence.End): es código.
func TestMermaidFormatters_SkipLongerFences(t *testing.T) {
	for name, tc := range map[string]struct {
		apply func(string) (string, error)
		in    string
	}{
		"formatter":    {NewMermaidFormatterRule().Apply, "````md\n```\n<<mermaid>>\ngraph TD\nA-->B\n<<end>>\n```\n````"},
		"syntax fixer": {NewMermaidSyntaxFixerRule().Apply, "````md\n```\n<<mermaid>>\n  gantt\n  dateFormat  YYYY-MM\n  Task 1 :2024-01, 2024-02\n```\n````"},
	} {
		apply, in := tc.apply, tc.in
		if got, err := apply(in); err != nil || got != in {
			t.Errorf("%s: Apply(%q) = %q (%v), want it unchanged", name, in, got, err)
		}
	}
}
