// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Un `*` literal de un encabezado (`\*` en la fuente) sale escapado en el
// Markdown: si no, cualquier visor lo muestra en cursiva. El encabezado se
// arma con el HTML que produce core para `### Sub \*h\* **b** a\=\=b y 2 * 3`,
// sin parsear, porque doclang compila contra el core publicado.
func TestMarkdown_HeadingKeepsLiteralDelimitersEscaped(t *testing.T) {
	heading := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1), `<h3 id="sub">Sub *h* <strong>b</strong> a==b y 2 * 3 <code>c*d</code></h3>`)
	heading.Level = 3
	out := filepath.Join(t.TempDir(), "d.md")
	if err := New(newTestLogger()).Generate(astWithElements(heading), out, GeneratorOptions{Format: "markdown"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	line := ""
	for _, l := range strings.Split(string(data), "\n") {
		if strings.Contains(l, "Sub ") {
			line = l
		}
	}
	text := strings.TrimSpace(strings.TrimLeft(strings.SplitN(line, " {#", 2)[0], "#"))
	// El código del encabezado sale como texto plano (el Markdown de un
	// encabezado nunca conservó los backticks), así que también se escapa.
	if want := `Sub \*h\* b a\=\=b y 2 \* 3 c\*d`; text != want {
		t.Fatalf("heading %q, want %q", text, want)
	}
	// Leído otra vez con la regla de escapes del dialecto, es el mismo texto
	// que mostraba el HTML: la ida y vuelta es estable.
	if got := docxUnescapeDelimiters(text); got != "Sub *h* b a==b y 2 * 3 c*d" {
		t.Errorf("round trip %q", got)
	}
}
