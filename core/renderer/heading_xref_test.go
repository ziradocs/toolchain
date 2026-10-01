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
	"go.ziradocs.com/core/v2/xref"
)

const headingXrefSource = "---\ntitle: \"Probe\"\nvariables:\n" +
	"  u: \"https://ok.example/p\"\n" +
	"  n: \"*Ana*\"\n" +
	"  v: \"javascript:alert(1)\"\n" +
	"---\n\n# Doc\n\n" +
	"## Ver [doc]({{u}}) y \\ref{eq:e} {{n}}\n\n" +
	"Ver [doc]({{u}}) y \\ref{eq:e} {{n}}\n\n" +
	"## Mal [x]({{v}}) y \\ref{eq:e}\n\n" +
	"Mal [x]({{v}}) y \\ref{eq:e}\n\n" +
	"<<math>>\ne = mc^2\nlabel: \"eq:e\"\n<<end>>\n"

// xref reescribe el Content de un encabezado legado después de parsear, así
// que Content ya no es lo que HeadingHTML produce con HeadingSource. Antes
// de mantener HeadingSource y HeadingContent al día, el renderer caía al
// camino sin fuente: el enlace con variable quedaba como href="{{u}}" (roto),
// el Markdown del valor salía literal y el \ref resuelto quedaba como
// "[Ecuación 1](#eq-e)" literal dentro del <h2>. Ahora el encabezado sale
// igual que el párrafo con la misma fuente, en el cuerpo y en el TOC.
func TestHeadingVariables_AfterXrefMatchParagraphText(t *testing.T) {
	doc, diags := parser.New(util.NewNoop()).ParseDocument(headingXrefSource, "probe.doclang")
	if doc == nil {
		t.Fatalf("ParseDocument: %v", diags)
	}
	doc, err := xref.Transform(doc)
	if err != nil {
		t.Fatalf("xref.Transform: %v", err)
	}
	vars := doc.FrontMatter.BuildVariables()

	var headings, paragraphs []*ast.TextElement
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if text, ok := el.(*ast.TextElement); ok {
				if text.IsRawHTML && text.Level == 2 {
					headings = append(headings, text)
				} else if !text.IsRawHTML {
					paragraphs = append(paragraphs, text)
				}
			}
		}
	}
	if len(headings) != 2 || len(paragraphs) != 2 {
		t.Fatalf("se esperaban 2 encabezados y 2 párrafos, hay %d y %d", len(headings), len(paragraphs))
	}

	for i, heading := range headings {
		paragraph := renderer.RenderElementToHTML(paragraphs[i], vars, nil)
		body := strings.TrimSuffix(strings.TrimPrefix(paragraph, "<p>"), "</p>")
		got := renderer.RenderElementToHTML(heading, vars, nil)
		want := `<h2 id="` + heading.HeadingAnchor + `">` + body + `</h2>`
		if got != want {
			t.Errorf("encabezado %d\n got %q\nwant %q", i, got, want)
		}
		if !strings.Contains(got, `<a href="#eq-e">`) {
			t.Errorf("encabezado %d sin el enlace del \\ref: %q", i, got)
		}
	}

	html := renderer.GenerateDocumentHTML(doc, renderer.DocumentHTMLOptions{TOC: true, TOCDepth: 3}, nil)
	for _, bad := range []string{`href="{{u}}"`, `href="javascript:`, "[Ecuación 1](#eq-e)", "*Ana*"} {
		if strings.Contains(html, bad) {
			t.Errorf("el documento contiene %q", bad)
		}
	}
	if n := strings.Count(html, `<a href="https://ok.example/p">doc</a>`); n < 3 {
		t.Errorf("el enlace con variable aparece %d veces, quería al menos 3 (párrafo, <h2> y TOC)", n)
	}
}
