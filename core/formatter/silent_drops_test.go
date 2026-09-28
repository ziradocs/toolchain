// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/linter"
)

// Una etiqueta de code-group con espacios ya no se pierde y sobrevive a fmt.
func TestCodeGroupLabelWithSpaces(t *testing.T) {
	src := "---\nmode: flex\n---\n# Deck\n\n## Code\n\n:::code-group\n```python [With Visualization]\nprint(1)\n```\n```go [Go]\nfmt.Println(1)\n```\n:::\n"
	doc := mustParse(t, src, false)
	group := firstOfType[*ast.CodeGroupElement](t, doc)
	if len(group.CodeBlocks) != 2 || group.CodeBlocks[0].Label != "With Visualization" || group.CodeBlocks[1].Label != "Go" {
		t.Fatalf("code blocks = %+v", group.CodeBlocks)
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	again := firstOfType[*ast.CodeGroupElement](t, mustParse(t, out, false))
	if again.CodeBlocks[0].Label != "With Visualization" {
		t.Fatalf("label lost in fmt:\n%s", out)
	}
}

// Una línea suelta dentro de un code-group o una info de fence mal formada
// ya no desaparecen sin aviso.
func TestCodeGroupRejectsStrayContent(t *testing.T) {
	for _, body := range []string{
		":::code-group\nThis line is outside any fence.\n```python [Py]\nprint(1)\n```\n:::\n",
		":::code-group\n```python [unclosed\nprint(1)\n```\n:::\n",
		":::code-group\n```python label\nprint(1)\n```\n:::\n",
	} {
		src := "---\nmode: flex\n---\n# Deck\n\n## Code\n\n" + body
		if _, diags := parseAny(src); !anyError(diags) {
			t.Errorf("accepted:\n%s", body)
		}
	}
}

// Un SLIDE con layout declarado nunca se reclasifica; uno sin declarar, sí.
func TestLastSlideClosingRespectsDeclaredLayout(t *testing.T) {
	declared := mustParse(t, "---\nmode: strict\n---\nSLIDE content\n  title: \"First\"\n  TEXT\n    Hello.\n\nSLIDE content\n  TEXT\n    Questions?\n", false)
	linter.New().Lint(declared)
	if got := declared.ContentBlocks[len(declared.ContentBlocks)-1].BlockType; got != "content" {
		t.Fatalf("declared SLIDE content became %q", got)
	}
	inferred := mustParse(t, "---\nmode: flex\n---\n# Deck\n\n## First\n\nHello.\n\n---\n\nQuestions?\n", false)
	last := &inferred.ContentBlocks[len(inferred.ContentBlocks)-1]
	if last.LayoutDeclared {
		t.Fatalf("flex slide without layout marked as declared: %+v", last)
	}
}

// Un SECTION inmediatamente después de un QUOTE (o un CHECKLIST) ya no se
// vuelve texto de la cita, así que su node-id se liga al encabezado.
func TestSectionAfterQuoteKeepsNodeID(t *testing.T) {
	for _, first := range []string{"  QUOTE\n    A quote.\n", "  CHECKLIST\n    - [x] Done\n"} {
		src := "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n" + first + "  <!-- node-id: HeadingA -->\n  SECTION \"After\"\n    level: 3\n"
		doc := mustParse(t, src, false)
		found := false
		_ = ast.Walk(doc, func(n ast.Node) error {
			if te, ok := n.(*ast.TextElement); ok && te.NodeID == "HeadingA" && strings.Contains(te.Content, "After") {
				found = true
			}
			if q, ok := n.(*ast.QuoteElement); ok && strings.Contains(q.Content, "SECTION") {
				t.Fatalf("quote swallowed the heading: %q", q.Content)
			}
			return nil
		})
		if !found {
			t.Fatalf("heading with node-id not found after:\n%s", first)
		}
	}
}
