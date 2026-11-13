// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Sin HeadingSource (un AST que llegó por un filtro), el formatter rearma el
// encabezado desde su HTML. Un `*` que el HTML muestra como texto vuelve
// escapado: la fuente -> HTML -> fuente -> HTML da el mismo texto, sin
// cursiva, y formatear otra vez da la misma fuente.
func TestFormatDocument_HeadingFallbackEscapesLiteralDelimiters(t *testing.T) {
	const want = `Sub \*h\* b a\=\=b y 2 \* 3`
	for name, format := range map[string]func(*ast.AST) (string, error){"flex": FormatDocument, "strict": FormatDocumentStrict} {
		t.Run(name, func(t *testing.T) {
			pos := diagnostics.NewPosition(1, 1)
			heading := ast.NewRawHTMLTextElement(pos, `<h2 id="sub">Sub *h* <strong>b</strong> a==b y 2 * 3</h2>`)
			heading.Level = 2
			block := ast.NewContentBlock(pos, "content")
			block.Title = "Sección"
			block.Elements = []ast.Element{heading}
			doc := ast.NewAST(pos)
			doc.ContentBlocks = []ast.ContentBlock{*block}

			out, err := format(doc)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, want) {
				t.Fatalf("formatted heading lacks %q:\n%s", want, out)
			}

			again, diags := parser.New(util.NewNoop()).ParseDocument(out, "d.doclang")
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("reparse: %v\n%s", d.Message, out)
				}
			}
			var html string
			_ = ast.Walk(again, func(n ast.Node) error {
				switch e := n.(type) {
				case *ast.TextElement:
					if e.Level == 2 {
						html = e.Content
					}
				case *ast.HeadingElement:
					html = e.Text
				}
				return nil
			})
			if !strings.Contains(html, "Sub *h* b a==b y 2 * 3") || strings.Contains(html, "<em>") || strings.Contains(html, "<mark>") {
				t.Errorf("reparsed heading %q", html)
			}
			out2, err := format(again)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out2, want) {
				t.Errorf("second format lacks %q:\n%s", want, out2)
			}
		})
	}
}
