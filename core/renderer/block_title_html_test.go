// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// blockTitleDoc arma un AST con un slide de título (heading, subtitle,
// kicker) y uno de contenido (title), todos con el mismo texto.
func blockTitleDoc(text string) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	cover := ast.NewContentBlock(pos, "title")
	cover.Heading, cover.Subtitle, cover.Kicker = text, text, text
	slide := ast.NewContentBlock(pos, "stats")
	slide.Title = text
	doc.ContentBlocks = []ast.ContentBlock{*cover, *slide}
	return doc
}

// El título de un slide lleva el Markdown inline de una línea en titleHTML
// (y lo mismo heading, subtitle y kicker): negrita, cursiva, código,
// tachado, resaltado y enlaces, con el HTML crudo escapado una sola vez.
func TestPopulateInlineHTML_BlockTitlesApplyInlineMarkdown(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"bold", "Las ventas crecieron **18%** frente al anterior", "Las ventas crecieron <strong>18%</strong> frente al anterior"},
		// El dialecto inline no tiene cursiva con guion bajo (igual que en el
		// cuerpo), así que un snake_case de un título no cambia.
		{"italic", "Cursiva *uno* y _dos_ en mi_variable", "Cursiva <em>uno</em> y _dos_ en mi_variable"},
		{"code", "Código `x = 1`", "Código <code>x = 1</code>"},
		{"strike", "Precio ~~viejo~~ nuevo", "Precio <del>viejo</del> nuevo"},
		{"highlight", "Un ==dato== clave", "Un <mark>dato</mark> clave"},
		{"link", "Ver [informe](https://example.com)", `Ver <a href="https://example.com">informe</a>`},
		{"plain", "Resultados del trimestre", "Resultados del trimestre"},
		{"ampersand", "Ventas & margen", "Ventas &amp; margen"},
		{"raw html", "Hola <b>x</b>", "Hola &lt;b&gt;x&lt;/b&gt;"},
		{"xss", `T <img src=x onerror=alert(1)>`, "T &lt;img src=x onerror=alert(1)&gt;"},
		{"javascript link", "[clic](javascript:alert(1))", "clic"},
		{"leading dash is not a list", "- 5% menos que 2025", "- 5% menos que 2025"},
		{"leading number is not a list", "1. Resultados", "1. Resultados"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := blockTitleDoc(tc.src)
			PopulateInlineHTML(doc, nil)
			cover, slide := doc.ContentBlocks[0], doc.ContentBlocks[1]
			for field, got := range map[string]string{
				"titleHTML":    slide.TitleHTML,
				"headingHTML":  cover.HeadingHTML,
				"subtitleHTML": cover.SubtitleHTML,
				"kickerHTML":   cover.KickerHTML,
			} {
				if got != tc.want {
					t.Errorf("%s = %q, want %q", field, got, tc.want)
				}
			}
			// Aditivo: el campo crudo conserva la fuente con sus marcas.
			if slide.Title != tc.src || cover.Heading != tc.src {
				t.Errorf("raw fields changed: title %q heading %q", slide.Title, cover.Heading)
			}
		})
	}
}

// Las {{variables}} se sustituyen antes del Markdown, como en un encabezado
// `##`: el valor se escapa, su Markdown se interpreta igual que en el cuerpo
// y un destino javascript: que llega por variable se descarta.
func TestPopulateInlineHTML_BlockTitleVariables(t *testing.T) {
	vars := map[string]interface{}{
		"pct":  "18%",
		"evil": xssPayload,
		"url":  "javascript:alert(1)",
	}
	cases := []struct{ src, want string }{
		{"Crecimos **{{pct}}**", "Crecimos <strong>18%</strong>"},
		{"T {{evil}}", "T &lt;img src=x onerror=alert(1)&gt;"},
		{"[clic]({{url}})", "clic"},
	}
	for _, tc := range cases {
		doc := blockTitleDoc(tc.src)
		PopulateInlineHTML(doc, vars)
		if got := doc.ContentBlocks[1].TitleHTML; got != tc.want {
			t.Errorf("%q: titleHTML = %q, want %q", tc.src, got, tc.want)
		}
	}
}

// slidelang pinta el encabezado del slide con funciones que ya existen en el
// core que tiene fijado (ProcessVariables + ProcessInlineMarkdownSecureLine).
// Esta prueba fija que, sin código literal, BlockTitleHTML es exactamente esa
// composición: si una de las dos cambia, el titleHTML del AST y el <h1> del
// HTML no pueden separarse sin que esto falle.
func TestBlockTitleHTML_MatchesSlidelangComposition(t *testing.T) {
	vars := map[string]interface{}{"name": xssPayload, "pct": "**18%**"}
	for _, src := range []string{
		"Las ventas crecieron **18%** frente al anterior",
		"{{name}} y `code` y ~~x~~ y [a](https://e.com)",
		"Crecimos {{pct}} & más <i>",
		"- no es lista",
		"[hola]{lang=fr} y [Ctrl]{.kbd}",
		"",
	} {
		want := ProcessInlineMarkdownSecureLine(ProcessVariables(src, vars))
		if got := BlockTitleHTML(src, vars); got != want {
			t.Errorf("%q: BlockTitleHTML %q, composition %q", src, got, want)
		}
	}
}

// Con PlainBlockTitles (doclang) los campos quedan como antes: variables
// sustituidas y todo escapado, sin Markdown.
func TestPopulateInlineHTML_PlainBlockTitles(t *testing.T) {
	src := "Las ventas crecieron **18%** & {{evil}}"
	vars := map[string]interface{}{"evil": xssPayload}
	want := ProcessVariablesSecure(src, vars)

	var opts InlineHTMLOptions
	var setter interface{ SetPlainBlockTitles(bool) } = &opts
	setter.SetPlainBlockTitles(true)

	doc := blockTitleDoc(src)
	PopulateInlineHTMLWithOptions(doc, vars, opts)
	cover, slide := doc.ContentBlocks[0], doc.ContentBlocks[1]
	for field, got := range map[string]string{
		"titleHTML": slide.TitleHTML, "headingHTML": cover.HeadingHTML,
		"subtitleHTML": cover.SubtitleHTML, "kickerHTML": cover.KickerHTML,
	} {
		if got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
}

// Repoblar no vuelve a escapar: PopulateInlineHTML siempre parte del campo
// crudo.
func TestPopulateInlineHTML_BlockTitleIdempotent(t *testing.T) {
	doc := blockTitleDoc("A & **B**")
	PopulateInlineHTML(doc, nil)
	PopulateInlineHTML(doc, nil)
	if got := doc.ContentBlocks[1].TitleHTML; got != "A &amp; <strong>B</strong>" {
		t.Errorf("titleHTML after two passes = %q", got)
	}
}
