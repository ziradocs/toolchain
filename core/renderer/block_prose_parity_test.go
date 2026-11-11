// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer_test

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// blockProseCases son avisos con un párrafo, con varios, con listas numeradas
// y anidadas, mezclados y con elementos anidados. want es el HTML de la prosa
// (el contentHTML de un aviso sin Elements); vacío si el aviso tiene Elements.
var blockProseCases = []struct {
	name, body, want string
}{
	{"un párrafo", "Solo un párrafo con **negrita**.", "Solo un párrafo con <strong>negrita</strong>."},
	{"un párrafo en dos líneas", "Primera línea\nsegunda línea.", "Primera línea<br>segunda línea."},
	{"varios párrafos", "Primer párrafo.\n\nSegundo párrafo.\n\n\nTercero.",
		"<p>Primer párrafo.</p><p>Segundo párrafo.</p><p>Tercero.</p>"},
	{"lista numerada", "1. a\n2. b", "<ol><li>a</li><li>b</li></ol>"},
	{"lista numerada desde 3", "3. c\n4. d", `<ol start="3"><li>c</li><li>d</li></ol>`},
	{"lista anidada", "- a\n  - b\n    - c", "<ul><li>a<ul><li>b<ul><li>c</li></ul></li></ul></li></ul>"},
	{"numerada con viñetas dentro", "1. a\n   - b\n2. c", "<ol><li>a<ul><li>b</li></ul></li><li>c</li></ol>"},
	{"prosa y lista", "Antes:\n- x\n- y", "Antes:<ul><li>x</li><li>y</li></ul>"},
	{"mixto", "Intro.\n\n1. uno\n2. dos\n\nCierre en\ndos líneas.\n\nOtro párrafo.",
		"<p>Intro.</p><ol><li>uno</li><li>dos</li></ol><p>Cierre en<br>dos líneas.</p><p>Otro párrafo.</p>"},
	{"variables y código literal", "Hola {{name}} y `{{name}}`.\n\nAdiós {{name}}.\n\n- {{name}}",
		"<p>Hola ACME y <code>{{name}}</code>.</p><p>Adiós ACME.</p><ul><li>ACME</li></ul>"},
	{"un párrafo y una lista", "Uno.\n\n- x\n\nDos.", "Uno.<ul><li>x</li></ul>Dos."},
	{"con elementos", "### Título\n1. a\n2. b\n\nPrimer párrafo.\n\nSegundo párrafo.\n- x\n  - y", ""},
	{"con tabla", "Antes.\n\nDespués.\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\nFinal.", ""},
}

func blockProseSource() string {
	var src strings.Builder
	src.WriteString("---\ntitle: Avisos\nvariables:\n  name: ACME\n---\n\n# Avisos\n\n")
	for _, c := range blockProseCases {
		src.WriteString(":::note\n" + c.body + "\n:::\n\n")
	}
	return src.String()
}

func parseBlockProseDoc(t *testing.T) *ast.AST {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(blockProseSource(), "avisos.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %s", d.Message)
		}
	}
	return doc
}

func topSpecialBlocks(t *testing.T, doc *ast.AST) []*ast.SpecialBlockElement {
	t.Helper()
	var blocks []*ast.SpecialBlockElement
	for _, block := range doc.ContentBlocks {
		for _, elem := range block.Elements {
			if sb, ok := elem.(*ast.SpecialBlockElement); ok {
				blocks = append(blocks, sb)
			}
		}
	}
	if len(blocks) != len(blockProseCases) {
		t.Fatalf("%d avisos en el AST, se esperaban %d", len(blocks), len(blockProseCases))
	}
	return blocks
}

// alertContents devuelve el interior de cada <div class="alert-content"> de
// html, en orden, con sus <div> anidados balanceados.
func alertContents(html string) []string {
	const open = `<div class="alert-content">`
	var out []string
	for {
		start := strings.Index(html, open)
		if start < 0 {
			return out
		}
		html = html[start+len(open):]
		depth, i := 1, 0
		for depth > 0 && i < len(html) {
			switch {
			case strings.HasPrefix(html[i:], "<div"):
				depth++
				i += 4
			case strings.HasPrefix(html[i:], "</div>"):
				depth--
				if depth == 0 {
					out = append(out, html[:i])
				}
				i += 6
			default:
				i++
			}
		}
		html = html[i:]
	}
}

// wrapLikeRenderer es lo que hace el renderer con la prosa entre los elementos
// de un aviso: la envuelve en <p> salvo que ya traiga HTML de bloque.
func wrapLikeRenderer(s string) string {
	if strings.Contains(s, "<ul") || strings.Contains(s, "<ol") || strings.Contains(s, "<p>") {
		return s
	}
	return "<p>" + s + "</p>"
}

// TestBlockProse_ContentHTMLMatchesDocumentHTML es la prueba diferencial entre
// el contentHTML que doclang pone en get_ast (PopulateInlineHTMLWithOptions con
// LiteralCode y BlockProse, como RenderASTJSON) y el HTML del build
// (GenerateDocumentHTML con LiteralCode, que es también el del PDF). El
// oráculo: sin Elements, el contentHTML del aviso es el interior de su
// .alert-content; con Elements, cada TextElement de prosa entre ellos aparece
// en ese interior como wrapLikeRenderer(contentHTML), en orden.
func TestBlockProse_ContentHTMLMatchesDocumentHTML(t *testing.T) {
	built := parseBlockProseDoc(t)
	ctx := renderer.NewDefaultRenderContext()
	ctx.LiteralCode = true
	fragments := alertContents(renderer.GenerateDocumentHTML(built, renderer.DocumentHTMLOptions{}, ctx))
	if len(fragments) != len(blockProseCases) {
		t.Fatalf("%d .alert-content en el HTML, se esperaban %d", len(fragments), len(blockProseCases))
	}

	doc := parseBlockProseDoc(t)
	opts := renderer.InlineHTMLOptions{LiteralCode: true}
	opts.SetBlockProse(true)
	renderer.PopulateInlineHTMLWithOptions(doc, doc.FrontMatter.BuildVariables(), opts)

	for i, sb := range topSpecialBlocks(t, doc) {
		c := blockProseCases[i]
		t.Run(c.name, func(t *testing.T) {
			fragment := fragments[i]
			if len(sb.Elements) == 0 {
				if c.want == "" {
					t.Fatalf("el caso esperaba Elements y el aviso no tiene")
				}
				if sb.ContentHTML != fragment {
					t.Errorf("contentHTML y HTML del build difieren:\ncontentHTML: %s\nbuild:       %s", sb.ContentHTML, fragment)
				}
				if fragment != c.want {
					t.Errorf("HTML del aviso:\n got: %s\nwant: %s", fragment, c.want)
				}
				return
			}
			if c.want != "" {
				t.Fatalf("el caso no esperaba Elements y el aviso tiene %d", len(sb.Elements))
			}
			rest := fragment
			texts := 0
			for _, nested := range sb.Elements {
				text, ok := nested.(*ast.TextElement)
				if !ok || text.IsRawHTML {
					continue
				}
				texts++
				want := wrapLikeRenderer(text.ContentHTML)
				at := strings.Index(rest, want)
				if at < 0 {
					t.Fatalf("la prosa %q (contentHTML %s) no está en el HTML del build:\n%s", text.Content, text.ContentHTML, fragment)
				}
				rest = rest[at+len(want):]
			}
			if texts == 0 {
				t.Fatalf("el aviso no tiene prosa entre sus elementos")
			}
		})
	}
}

// Los párrafos y las listas de la prosa entre los elementos de un aviso, en
// el HTML: cada párrafo en su <p>, las listas fuera de ellos y sin un <p>
// dentro de otro.
func TestBlockProse_NestedProseParagraphs(t *testing.T) {
	doc := parseBlockProseDoc(t)
	ctx := renderer.NewDefaultRenderContext()
	ctx.LiteralCode = true
	fragments := alertContents(renderer.GenerateDocumentHTML(doc, renderer.DocumentHTMLOptions{}, ctx))
	want := map[string]string{
		"con elementos": `<h3 id="titulo">Título</h3><ol><li>a</li><li>b</li></ol><p>Primer párrafo.</p><p>Segundo párrafo.</p><ul><li>x<ul><li>y</li></ul></li></ul>`,
		"con tabla":     "<p>Antes.</p><p>Después.</p>",
	}
	for i, c := range blockProseCases {
		w, ok := want[c.name]
		if !ok {
			continue
		}
		if !strings.HasPrefix(fragments[i], w) {
			t.Errorf("%s:\n got: %s\nwant prefix: %s", c.name, fragments[i], w)
		}
		if strings.Contains(fragments[i], "<p><p>") {
			t.Errorf("%s: un <p> dentro de otro:\n%s", c.name, fragments[i])
		}
	}
}

// Sin la opción (slidelang) el contentHTML de un aviso es el de antes:
// ProcessInlineMarkdownSecure, lo mismo que pinta la plantilla de slidelang
// ({{.Content | markdown}}), para que su get_ast siga describiendo su HTML.
func TestBlockProse_OffKeepsSlideLangContentHTML(t *testing.T) {
	doc := parseBlockProseDoc(t)
	vars := doc.FrontMatter.BuildVariables()
	renderer.PopulateInlineHTML(doc, vars)
	for i, sb := range topSpecialBlocks(t, doc) {
		want := renderer.ProcessInlineMarkdownSecure(renderer.ProcessVariables(sb.Content, vars))
		if sb.ContentHTML != want {
			t.Errorf("%s: contentHTML %q, want %q", blockProseCases[i].name, sb.ContentHTML, want)
		}
	}
}
