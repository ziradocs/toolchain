// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"
	"testing"
)

func TestCodeGroupFormatterRule_Apply(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Convierte code-group con un archivo",
			input: `::::code-group
:::code-item{title="config.yaml"}
` + "```yaml" + `
server:
  port: 8080
` + "```" + `
:::
::::`,
			expected: `:::code-group
` + "```yaml [config.yaml]" + `
server:
  port: 8080
` + "```" + `
:::`,
		},
		{
			name: "Convierte code-group con múltiples archivos",
			input: `::::code-group
:::code-item{title="main.go"}
` + "```go" + `
package main
func main() {}
` + "```" + `
:::

:::code-item{title="test.go"}
` + "```go" + `
package main
func TestMain() {}
` + "```" + `
:::
::::`,
			expected: `:::code-group
` + "```go [main.go]" + `
package main
func main() {}
` + "```" + `

` + "```go [test.go]" + `
package main
func TestMain() {}
` + "```" + `
:::`,
		},
		{
			name: "Preserva contenido sin code-groups",
			input: `# Título

Contenido normal

` + "```javascript" + `
console.log("Hello");
` + "```",
			expected: `# Título

Contenido normal

` + "```javascript" + `
console.log("Hello");
` + "```",
		},
		{
			name: "Maneja code-groups con espacios",
			input: `::::code-group
:::code-item{title="example.js"}
` + "```javascript" + `
console.log("test");
` + "```" + `
:::
::::`,
			expected: `:::code-group
` + "```javascript [example.js]" + `
console.log("test");
` + "```" + `
:::`,
		},
	}

	rule := NewCodeGroupFormatterRule()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			// Normalizar espacios en blanco para comparación
			resultNorm := strings.TrimSpace(result)
			expectedNorm := strings.TrimSpace(tt.expected)

			if resultNorm != expectedNorm {
				t.Errorf("Apply() mismatch\nGot:\n%s\n\nWant:\n%s", result, tt.expected)
			}
		})
	}
}

// TestCodeGroupFormatterRule_DoesNotSwallowBlankLines es una prueba de
// regresión: los patrones de esta regla usaban `[\s]*` alrededor de los
// marcadores (::::code-group, :::code-item{}, :::), y `\s` en Go incluye
// `\n`. Combinado con (?m), eso permitía que el match "engullera" la línea
// en blanco inmediatamente anterior o posterior al marcador, corrompiendo el
// conteo de líneas del documento (y por tanto las posiciones/diagnósticos
// reportados para todo el contenido posterior) — incluso en documentos que
// YA usan la sintaxis canónica ```lang [label] y no tienen ningún
// :::code-item{} que reescribir. Se detectó al correr un diff de AST antes/
// después sobre el corpus de examples/ para el fix del issue #174.
func TestCodeGroupFormatterRule_DoesNotSwallowBlankLines(t *testing.T) {
	input := "## Heading\n\n::::code-group\n" +
		"```go [fibonacci.go]\n" +
		"func fibonacci(n int) int { return n }\n" +
		"```\n" +
		":::\n\n## Next Heading\n"

	rule := NewCodeGroupFormatterRule()
	result, err := rule.Apply(input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	inLines := strings.Count(input, "\n")
	outLines := strings.Count(result, "\n")
	if inLines != outLines {
		t.Errorf("Apply() changed line count: input had %d newlines, output had %d\ninput:\n%s\noutput:\n%s",
			inLines, outLines, input, result)
	}

	if !strings.Contains(result, "\n\n:::code-group\n") {
		t.Errorf("expected the blank line preceding the code-group marker to be preserved, got:\n%s", result)
	}
	if !strings.Contains(result, ":::\n\n## Next Heading") {
		t.Errorf("expected the blank line following the closing ':::' to be preserved, got:\n%s", result)
	}
}

// TestCodeGroupFormatterRule_CanonicalCloseDoesNotLeakState is a regression
// test: a code-group that already uses canonical fence syntax (no
// :::code-item{} wrapper) is opened with "::::code-group" but, per the real
// parser (elements/code_group.go), always closes with a single ":::" — the
// rule only cleared its internal `inCodeGroup` flag on a literal "::::"
// closer (the legacy wrapper-style end marker). That left `inCodeGroup` true
// for the rest of the document after a canonical code-group, so an
// unrelated, later bare "::::" line (e.g. an unrelated divider elsewhere in
// the doc) got silently rewritten to ":::", corrupting content that has
// nothing to do with any code-group.
func TestCodeGroupFormatterRule_CanonicalCloseDoesNotLeakState(t *testing.T) {
	input := "::::code-group\n" +
		"```go [a.go]\n" +
		"code\n" +
		"```\n" +
		":::\n\n" +
		"Some prose.\n\n" +
		"::::\n\n" +
		"More text after.\n"

	rule := NewCodeGroupFormatterRule()
	result, err := rule.Apply(input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if !strings.Contains(result, "\n\n::::\n\n") {
		t.Errorf("expected the unrelated, later '::::' line to be preserved as-is (not rewritten to ':::'), got:\n%s", result)
	}
}

// TestCodeGroupFormatterRule_LiteralColonsInsideCanonicalFenceDoNotCloseGroup
// is a regression test found in code-review: a canonical code-group (bare
// ```lang [label] fences, no :::code-item{} wrapper) only tracked
// inCodeBlock while inside a :::code-item{} wrapper, so a fence's CONTENT
// containing a literal line that is exactly ":::" (plausible in, say, a
// documentation example demonstrating this very code-group syntax) tripped
// the canonical-close detection early. Any later, genuinely-wrapped
// :::code-item{} block in the SAME code-group then never got recognized
// (code-item detection requires inCodeGroup == true) and was left
// unrewritten — reproducing the exact "only the first tab survives" bug
// this whole rule exists to fix, just via a different trigger.
func TestCodeGroupFormatterRule_LiteralColonsInsideCanonicalFenceDoNotCloseGroup(t *testing.T) {
	input := "::::code-group\n" +
		"```text [example.txt]\n" +
		"A canonical code-group closes with:\n" +
		":::\n" +
		"```\n" +
		"\n" +
		":::code-item{title=\"real.go\"}\n" +
		"```go\n" +
		"package main\n" +
		"```\n" +
		":::\n" +
		"::::\n"

	rule := NewCodeGroupFormatterRule()
	result, err := rule.Apply(input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if strings.Contains(result, ":::code-item") {
		t.Errorf("expected the :::code-item{} wrapper after the bare fence to still be rewritten, got:\n%q", result)
	}
	if !strings.Contains(result, "```go [real.go]") {
		t.Errorf("expected the code-item after the bare fence to be converted to canonical fenced syntax with label, got:\n%q", result)
	}
	// The literal ":::" inside the first fence's content must survive untouched.
	if !strings.Contains(result, "A canonical code-group closes with:\n:::\n```") {
		t.Errorf("expected the literal ':::' inside the bare fence's content to be preserved as-is, got:\n%q", result)
	}
}

// TestCodeGroupFormatterRule_CRLF is a regression test: the markers'
// patterns were tightened from `[\s]*` to `[ \t]*` (to stop swallowing
// blank lines, see TestCodeGroupFormatterRule_DoesNotSwallowBlankLines), but
// without an explicit `\r?` before `$`, that same tightening broke matching
// on CRLF-terminated files: `$` in multiline mode asserts the position right
// before `\n`, and on a CRLF line that position has a `\r` in front of it,
// which `[ \t]*` alone does not consume.
func TestCodeGroupFormatterRule_CRLF(t *testing.T) {
	input := "::::code-group\r\n" +
		":::code-item{title=\"a.go\"}\r\n" +
		"```go\r\n" +
		"code\r\n" +
		"```\r\n" +
		":::\r\n" +
		"::::\r\n"

	rule := NewCodeGroupFormatterRule()
	result, err := rule.Apply(input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if strings.Contains(result, ":::code-item") {
		t.Errorf("expected the :::code-item{} wrapper to be rewritten even on CRLF input, got:\n%q", result)
	}
	if !strings.Contains(result, "```go [a.go]") {
		t.Errorf("expected canonical fenced syntax with label on CRLF input, got:\n%q", result)
	}
}

func TestCodeGroupFormatterRule_Metadata(t *testing.T) {
	rule := NewCodeGroupFormatterRule()

	if rule.Description() == "" {
		t.Error("Description() should not be empty")
	}

	if rule.Priority() < 0 {
		t.Error("Priority() should be non-negative")
	}
}
