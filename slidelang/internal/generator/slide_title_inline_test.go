// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

func slideTitleDoc() *ast.AST {
	p := diagnostics.NewPosition(1, 1)
	cover := ast.NewContentBlock(p, "title")
	cover.Heading = "Portada con **negrita**"
	cover.Subtitle = "Sub con *cursiva*"
	cover.Kicker = "Antetítulo `v2`"
	stats := ast.NewContentBlock(p, "stats")
	stats.Title = "Las ventas crecieron **18%** frente al {{periodo}}"
	stats.Elements = []ast.Element{ast.NewTextElement(p, "Cuerpo.")}
	hostile := ast.NewContentBlock(p, "content")
	hostile.Title = "A & B <img src=x onerror=alert(1)> {{evil}}"
	hostile.Elements = []ast.Element{ast.NewTextElement(p, "Cuerpo.")}
	plain := ast.NewContentBlock(p, "content")
	plain.Title = "Resultados del trimestre"
	plain.Subtitle = "Sub ~~viejo~~"
	plain.Elements = []ast.Element{ast.NewTextElement(p, "Cuerpo.")}
	fm := ast.NewFrontMatterNode(p)
	fm.Variables = map[string]interface{}{"periodo": "año anterior", "evil": "<script>x</script>"}
	return &ast.AST{FrontMatter: fm, ContentBlocks: []ast.ContentBlock{*cover, *stats, *hostile, *plain}}
}

// El encabezado del slide en el HTML lleva el formato inline (no los
// asteriscos), escapado una sola vez, y los metadatos del slide llevan su
// texto plano.
func TestRenderHTMLPreview_SlideTitlesRenderInlineMarkdown(t *testing.T) {
	html, err := New(util.NewNoop()).RenderHTMLPreview(slideTitleDoc(), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	for _, want := range []string{
		`<h1 class="slidelang-main-title">Portada con <strong>negrita</strong></h1>`,
		`<h2 class="slidelang-subtitle">Sub con <em>cursiva</em></h2>`,
		`<p class="slidelang-kicker">Antetítulo <code>v2</code></p>`,
		`<h1>Las ventas crecieron <strong>18%</strong> frente al año anterior</h1>`,
		`<h1>A &amp; B &lt;img src=x onerror=alert(1)&gt; &lt;script&gt;x&lt;/script&gt;</h1>`,
		`<h1>Resultados del trimestre</h1>`,
		`<p class="slidelang-subtitle">Sub <del>viejo</del></p>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, bad := range []string{"**", "&amp;amp;", "<img src=x", "<script>x"} {
		if strings.Contains(html, bad) {
			t.Errorf("HTML contains %q", bad)
		}
	}

	for idx, want := range map[int]string{
		0: "Portada con negrita",
		1: "Las ventas crecieron 18% frente al año anterior",
		// El atributo lleva el texto visible, escapado por html/template.
		2: "A &amp; B &lt;img src=x onerror=alert(1)&gt; &lt;script&gt;x&lt;/script&gt;",
		3: "Resultados del trimestre",
	} {
		if got := slideDivAttr(t, html, "data-slide-title", idx); got != want {
			t.Errorf("slide %d data-slide-title = %q, want %q", idx, got, want)
		}
	}
}

// En PPTX el título y el subtítulo llevan la negrita/cursiva como runs, no
// los asteriscos.
func TestGeneratePPTX_SlideTitleInlineFormatting(t *testing.T) {
	dir := t.TempDir()
	doc := slideTitleDoc()
	doc.FilePath = "test.slidelang"
	if err := New(util.NewNoop()).generatePPTX(doc, dir, GeneratorOptions{AssetRoot: dir}); err != nil {
		t.Fatalf("generatePPTX: %v", err)
	}
	path := filepath.Join(dir, "test.pptx")
	cover := zipEntryContent(t, path, "ppt/slides/slide1.xml")
	stats := zipEntryContent(t, path, "ppt/slides/slide2.xml")
	plain := zipEntryContent(t, path, "ppt/slides/slide4.xml")

	for _, tc := range []struct{ name, xml, attr, text string }{
		{"cover title", cover, "b", "negrita"},
		{"cover subtitle", cover, "i", "cursiva"},
		{"stats title", stats, "b", "18%"},
	} {
		if !pptxStyledRun(tc.xml, tc.attr, tc.text) {
			t.Errorf("%s: no %s run for %q:\n%s", tc.name, tc.attr, tc.text, tc.xml)
		}
	}
	for name, xml := range map[string]string{"cover": cover, "stats": stats, "plain": plain} {
		// ~~tachado~~ queda fuera: pptxgo no tiene tachado y el cuerpo
		// tampoco lo traduce hoy.
		if strings.Contains(xml, "**") {
			t.Errorf("%s slide keeps Markdown marks:\n%s", name, xml)
		}
	}
	if !strings.Contains(plain, `<a:t xml:space="preserve">Resultados del trimestre</a:t>`) {
		t.Errorf("plain title changed:\n%s", plain)
	}
}

// pptxStyledRun dice si xml tiene un run con attr="1" (b, i) cuyo texto es
// exactamente text.
func pptxStyledRun(xml, attr, text string) bool {
	re := regexp.MustCompile(`<a:rPr[^>]*\b` + attr + `="1"[^>]*>(?:</a:rPr>)?\s*<a:t[^>]*>` + regexp.QuoteMeta(text) + `</a:t>`)
	return re.MatchString(xml)
}

// data-slide-title lleva lo que el <h1> muestra: una entidad escrita como
// texto sigue siendo texto, no se decodifica dos veces.
func TestRenderHTMLPreview_SlideTitlePlainTextKeepsWrittenEntities(t *testing.T) {
	p := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(p, "content")
	block.Title = "Usa &lt;br&gt; aquí"
	block.Elements = []ast.Element{ast.NewTextElement(p, "Cuerpo.")}
	html, err := New(util.NewNoop()).RenderHTMLPreview(&ast.AST{ContentBlocks: []ast.ContentBlock{*block}}, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	if !strings.Contains(html, "<h1>Usa &amp;lt;br&amp;gt; aquí</h1>") {
		t.Errorf("h1 changed")
	}
	if got := slideDivAttr(t, html, "data-slide-title", 0); got != "Usa &amp;lt;br&amp;gt; aquí" {
		t.Errorf("data-slide-title = %q", got)
	}
}

// Un título que es solo una imagen identifica al slide con el alt de la
// imagen en data-slide-title y en el índice del JSON embebido.
func TestRenderHTMLPreview_ImageOnlyTitleUsesAlt(t *testing.T) {
	p := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(p, "content")
	block.Title = "![solo](ok.png)"
	block.Elements = []ast.Element{ast.NewTextElement(p, "Cuerpo.")}
	html, err := New(util.NewNoop()).RenderHTMLPreview(&ast.AST{ContentBlocks: []ast.ContentBlock{*block}}, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	if got := slideDivAttr(t, html, "data-slide-title", 0); got != "solo" {
		t.Errorf("data-slide-title = %q", got)
	}
	if !strings.Contains(html, `"title": "solo"`) {
		t.Errorf("the embedded JSON does not carry the alt as the slide title")
	}
}
