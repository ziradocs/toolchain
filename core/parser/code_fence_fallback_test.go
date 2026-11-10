// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// rulesAt son los RuleID de diags en la línea line.
func rulesAt(diags []diagnostics.Diagnostic, line int) []string {
	var out []string
	for _, d := range diags {
		if d.Position.Line == line && strings.HasPrefix(d.RuleID, "CODE") {
			out = append(out, d.RuleID)
		}
	}
	return out
}

func hasHeading(doc *ast.AST, text string) bool {
	found := false
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch h := n.(type) {
		case *ast.TextElement:
			found = found || h.IsRawHTML && strings.Contains(h.Content, ">"+text+"<")
		case *ast.HeadingElement:
			found = found || h.Text == text
		}
		return nil
	})
	for _, b := range doc.ContentBlocks {
		found = found || b.Title == text || b.Heading == text
	}
	return found
}

// P1-1 del review de #498: una cerca que con la regla de CommonMark quedaría
// sin cerrar se lee como en las versiones anteriores, con un aviso CODE003 en
// su apertura; el resto del documento no cambia de significado.
func TestCodeFence_FallsBackToTheEarlierRule(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		check     func(t *testing.T, doc *ast.AST)
		line      int
		rules     string
	}{
		{"four-backtick code closed by three", "# A\n\n````js\nq\n```\n\n## T\n", func(t *testing.T, doc *ast.AST) {
			if c := codesOf(doc); len(c) != 1 || c[0].Content != "q" {
				t.Errorf("codes %#v", c)
			}
		}, 3, "CODE003"},
		{"four-backtick mermaid closed by three", "# A\n\n````mermaid\ngraph TD\n```\n\n## T\n", func(t *testing.T, doc *ast.AST) {
			var m []*ast.MermaidElement
			_ = ast.Walk(doc, func(n ast.Node) error {
				if e, ok := n.(*ast.MermaidElement); ok {
					m = append(m, e)
				}
				return nil
			})
			if len(m) != 1 || strings.TrimSpace(m[0].Content) != "graph TD" {
				t.Errorf("mermaid %#v", m)
			}
		}, 3, "CODE003"},
		{"four-backtick plantuml closed by three", "# A\n\n````plantuml\nA -> B\n```\n\n## T\n", nil, 3, "CODE003"},
		{"four-backtick chart closed by three", "# A\n\n````chart\n{\"type\":\"bar\"}\n```\n\n## T\n", nil, 3, "CODE003"},
		{"four-backtick map closed by three", "# A\n\n````map\n{\"type\":\"world\"}\n```\n\n## T\n", nil, 3, "CODE003"},
		{"code-group tab closed by three", "# A\n\n:::code-group\n````js [a]\nx\n```\n:::\n\n## T\n", func(t *testing.T, doc *ast.AST) {
			var g []*ast.CodeGroupElement
			_ = ast.Walk(doc, func(n ast.Node) error {
				if e, ok := n.(*ast.CodeGroupElement); ok {
					g = append(g, e)
				}
				return nil
			})
			if len(g) != 1 || len(g[0].CodeBlocks) != 1 || g[0].CodeBlocks[0].Content != "x" {
				t.Errorf("code-group %#v", g)
			}
		}, 4, "CODE003"},
		{"backtick in the info string", "# A\n\nIntro.\n```ts title=`a.ts`\ncode\n```\n\n## T\n", func(t *testing.T, doc *ast.AST) {
			if c := codesOf(doc); len(c) != 1 || c[0].Content != "code" {
				t.Errorf("codes %#v", c)
			}
		}, 4, "CODE003"},
		{"unclosed in the earlier rule too", "# A\n\n````js\nq\n", nil, 3, "CODE003"},
		// ```md cierra en ```` (CommonMark, CODE002); el ```` siguiente no
		// tiene cierre de cuatro y cierra en ``` (CODE003).
		{"three-backtick fence around four", "# A\n\n```md\n````\ninner\n````\n```\n\n## T\n", nil, 6, "CODE003"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, diags := flexDoc(t, tc.src)
			if tc.check != nil {
				tc.check(t, doc)
			}
			if got := strings.Join(rulesAt(diags, tc.line), ","); got != tc.rules {
				t.Errorf("rules at line %d: %q, want %q (all: %v)", tc.line, got, tc.rules, diags)
			}
			if strings.Contains(tc.src, "## T") && !hasHeading(doc, "T") {
				t.Errorf("the ## after the fence must be a heading")
			}
		})
	}
}

// Una cerca de CommonMark bien formada que termina en otra línea que en las
// versiones anteriores lleva un aviso CODE004, y una línea que antes era cerca
// y ahora es texto también: ningún documento cambia de significado en
// silencio.
func TestCodeFence_ExtentChangeIsReported(t *testing.T) {
	doc, diags := flexDoc(t, "# A\n\n````js\na\n```\n## T\n````md\nb\n````\n")
	if got := rulesAt(diags, 3); strings.Join(got, ",") != "CODE004" {
		t.Errorf("want CODE004 at the opener, got %v (all %v)", got, diags)
	}
	if c := codesOf(doc); len(c) != 1 || !strings.Contains(c[0].Content, "## T") {
		t.Errorf("CommonMark reading: one block through ````: %#v", c)
	}

	doc, diags = flexDoc(t, "# A\n\n```ts title=`a.ts`\ncode\n```\n## Siguiente\n```js\nb\n```\n")
	if got := rulesAt(diags, 3); strings.Join(got, ",") != "CODE004" {
		t.Errorf("want CODE004 at the backtick-info line, got %v (all %v)", got, diags)
	}
	_ = doc
}

// El fragmento de contenido usa la misma lectura (P1-1: content_fragment solo
// protegía CodeElement).
func TestParseMarkdownContent_FenceFallback(t *testing.T) {
	els, diags := ParseMarkdownContent("````mermaid\ngraph TD\n```\n\nTexto final.\n", ContentOptions{AnchorNamespace: "t"})
	text := false
	for _, e := range els {
		if tx, ok := e.(*ast.TextElement); ok && strings.Contains(tx.Content, "Texto final.") {
			text = true
		}
	}
	if !text {
		t.Errorf("the text after the fence must survive: %#v %v", els, diags)
	}
}

// El pre-pase de node-id lee las cercas igual que el parser: después de una
// cerca de cuatro cerrada por ``` la anotación se aplica.
func TestNodeIDPrePass_FenceFallback(t *testing.T) {
	doc, _ := flexDoc(t, "# A\n\n````js\nq\n```\n\n<!-- node-id: n1 -->\nTexto.\n")
	applied := false
	_ = ast.Walk(doc, func(n ast.Node) error {
		if node, ok := n.(ast.IdentityNode); ok && node.GetNodeID() == "n1" {
			applied = true
		}
		return nil
	})
	if !applied {
		t.Error("the node-id after the fence must apply")
	}
}

// El fragmento de contenido acepta un bloque de código que cierra con la regla
// de las versiones anteriores (fence.ClosedByMain): está cerrado.
func TestParseMarkdownContent_CodeClosedByTheEarlierRule(t *testing.T) {
	els, diags := ParseMarkdownContent("````js\nq\n```\n\nfin\n", ContentOptions{AnchorNamespace: "t"})
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error: %v", d)
		}
	}
	if len(els) != 2 {
		t.Fatalf("want the code block and the text: %#v", els)
	}
}
