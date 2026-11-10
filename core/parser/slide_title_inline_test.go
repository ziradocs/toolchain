// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

func parseSlidesWithHTML(t *testing.T, content string) []ast.ContentBlock {
	t.Helper()
	doc, diags := New(util.NewNoop()).Parse(content, "titles.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("error de parseo: %v", d.Message)
		}
	}
	renderer.PopulateInlineHTML(doc, doc.FrontMatter.BuildVariables())
	return doc.ContentBlocks
}

// El encabezado de un slide con formato inline llega procesado a
// titleHTML/headingHTML, en flex (`#`/`##`) y en strict (`heading:`/`title:`),
// y title/heading conservan la fuente con sus marcas.
func TestSlideTitlesCarryInlineMarkdown(t *testing.T) {
	const flex = "---\nmode: flex\n---\n\n" +
		"# Portada con **negrita**\n\n" +
		"---\nlayout: stats\n---\n" +
		"## Las ventas crecieron **18%** frente al anterior\n\n" +
		"- **$48.2M** ventas netas\n\n" +
		"---\nlayout: content\n---\n" +
		"## Cursiva *uno* y código `x`\n\nTexto.\n\n" +
		"---\nlayout: content\n---\n" +
		"## Resultados del trimestre\n\nTexto.\n"
	const strict = "---\nmode: strict\n---\n\n" +
		"SLIDE title\n  heading: \"Portada con **negrita**\"\n  subtitle: \"Sub con *cursiva*\"\n\n" +
		"SLIDE stats\n  title: \"Las ventas crecieron **18%** frente al anterior\"\n  TEXT\n    Cuerpo.\n\n" +
		"SLIDE content\n  title: \"Cursiva *uno* y código `x`\"\n  TEXT\n    Cuerpo.\n\n" +
		"SLIDE content\n  title: \"Resultados del trimestre\"\n  TEXT\n    Cuerpo.\n"

	for name, src := range map[string]string{"flex": flex, "strict": strict} {
		t.Run(name, func(t *testing.T) {
			blocks := parseSlidesWithHTML(t, src)
			if len(blocks) != 4 {
				t.Fatalf("blocks = %d, want 4", len(blocks))
			}
			cover := blocks[0]
			if cover.Heading != "Portada con **negrita**" || cover.HeadingHTML != "Portada con <strong>negrita</strong>" {
				t.Errorf("heading %q headingHTML %q", cover.Heading, cover.HeadingHTML)
			}
			want := []struct{ title, html string }{
				{"Las ventas crecieron **18%** frente al anterior", "Las ventas crecieron <strong>18%</strong> frente al anterior"},
				{"Cursiva *uno* y código `x`", "Cursiva <em>uno</em> y código <code>x</code>"},
				{"Resultados del trimestre", "Resultados del trimestre"},
			}
			for i, w := range want {
				b := blocks[i+1]
				if b.Title != w.title || b.TitleHTML != w.html {
					t.Errorf("slide %d: title %q titleHTML %q, want %q / %q", i+2, b.Title, b.TitleHTML, w.title, w.html)
				}
			}
			if name == "strict" && cover.SubtitleHTML != "Sub con <em>cursiva</em>" {
				t.Errorf("subtitleHTML %q", cover.SubtitleHTML)
			}
		})
	}
}
