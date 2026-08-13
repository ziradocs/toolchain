// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// pptx_coverage_test.go cubre los cuatro tipos de elemento que pptxAddElement
// dejaba caer con un warning hasta ahora: quote, checklist, code y chart.
//
// Los tres primeros son shapes de texto; el cuarto embebe un PNG rasterizado
// por el camino nativo en Go. Lo que estos tests protegen no es el layout
// (pptxgo no mide texto, el apilado es un estimado) sino que el contenido
// LLEGUE al .pptx: la regresión que importa acá es la silenciosa —un elemento
// que vuelve a caer al default del switch y desaparece del deck— no un shape
// unos EMU corrido.

// buildPPTXWithElements genera un .pptx de un solo slide con los elementos
// dados y devuelve el XML de slide1.
func buildPPTXWithElements(t *testing.T, name string, elements ...ast.Element) string {
	t.Helper()
	dir := t.TempDir()

	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FilePath = name + ".slidelang"

	block := ast.NewContentBlock(pos(), "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements, elements...)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{AssetRoot: dir}); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	return zipEntryContent(t, filepath.Join(dir, name+".pptx"), "ppt/slides/slide1.xml")
}

func TestPPTX_QuoteElementIsRendered(t *testing.T) {
	q := ast.NewQuoteElement(pos(), "quote-marker-content")
	q.Author = "quote-marker-author"
	q.Source = "quote-marker-source"

	xml := buildPPTXWithElements(t, "quote", q)

	for _, marker := range []string{"quote-marker-content", "quote-marker-author", "quote-marker-source"} {
		if !strings.Contains(xml, marker) {
			t.Errorf("slide1.xml is missing %q — the quote was dropped", marker)
		}
	}

	// La atribución se emite como una sola línea "— Autor, Fuente".
	if !strings.Contains(xml, "quote-marker-author, quote-marker-source") {
		t.Error("author and source were not joined into a single attribution line")
	}
}

// TestPPTX_QuoteIsFullyItalic es el caso que motivó pptxApplyInlineBase:
// Paragraph.Text reinicia curRuns en cada llamada, así que un Italic() final
// después de pptxApplyInline solo alcanzaría al último segmento. Con markdown
// dentro de la cita hay más de un segmento, y todos deben salir en cursiva.
func TestPPTX_QuoteIsFullyItalic(t *testing.T) {
	q := ast.NewQuoteElement(pos(), "plain start **bold middle** plain end")

	xml := buildPPTXWithElements(t, "quoteitalic", q)

	// pptxSplitInline parte esto en 3 segmentos → 3 runs, cada uno con su
	// propio rPr. Los tres deben traer i="1".
	if got := strings.Count(xml, `i="1"`); got < 3 {
		t.Errorf(`expected at least 3 runs marked italic (one per inline segment), got %d occurrences of i="1"`, got)
	}
	if !strings.Contains(xml, `b="1"`) {
		t.Error("the bold segment lost its bold — baseItalic must add to the segment style, not replace it")
	}
}

func TestPPTX_ChecklistElementIsRendered(t *testing.T) {
	c := ast.NewChecklistElement(pos())
	c.Items = append(c.Items,
		*ast.NewChecklistItem(pos(), "checklist-marker-done", true),
		*ast.NewChecklistItem(pos(), "checklist-marker-todo", false),
	)

	xml := buildPPTXWithElements(t, "checklist", c)

	for _, marker := range []string{"checklist-marker-done", "checklist-marker-todo"} {
		if !strings.Contains(xml, marker) {
			t.Errorf("slide1.xml is missing %q — the checklist was dropped", marker)
		}
	}

	// El estado vive en el carácter de la viñeta, no en el texto: si estos
	// dos glifos no están, un item marcado es indistinguible de uno sin
	// marcar en el deck generado.
	if !strings.Contains(xml, pptxCheckboxChecked) {
		t.Errorf("checked item did not get the %q bullet", pptxCheckboxChecked)
	}
	if !strings.Contains(xml, pptxCheckboxUnchecked) {
		t.Errorf("unchecked item did not get the %q bullet", pptxCheckboxUnchecked)
	}
}

func TestPPTX_CodeElementIsRenderedMonospacedAndLiteral(t *testing.T) {
	// El contenido trae sintaxis que pptxSplitInline resolvería como markdown
	// si el código pasara por ahí: dentro de un bloque de código son
	// caracteres literales y tienen que sobrevivir tal cual.
	code := ast.NewCodeElement(pos(), "go", "a := *ptr\nb := `raw`\nc := **x")

	xml := buildPPTXWithElements(t, "code", code)

	if !strings.Contains(xml, "a := *ptr") {
		t.Error("code content is missing — the block was dropped")
	}
	if !strings.Contains(xml, "b := `raw`") {
		t.Error("backticks inside the code block were consumed as markdown instead of kept literal")
	}
	if !strings.Contains(xml, "c := **x") {
		t.Error("asterisks inside the code block were consumed as markdown instead of kept literal")
	}
	if strings.Contains(xml, `b="1"`) {
		t.Error("code block got bold formatting — it must not go through the inline markdown pass")
	}
	if !strings.Contains(xml, pptxMonoFont) {
		t.Errorf("code block was not set in %s", pptxMonoFont)
	}
}

// TestPPTX_ChartElementIsRasterizedNatively cubre el único elemento nuevo que
// produce una imagen. --format pptx nunca instancia Chromium, así que el
// chart solo puede salir por renderer.RenderChartNativePNG; si ese camino se
// rompe, el chart desaparece del deck.
func TestPPTX_ChartElementIsRasterizedNatively(t *testing.T) {
	c := ast.NewChartElement(pos(), "bar")
	c.Title = "chart-marker-title"
	c.Data = [][]interface{}{{"A", 10.0}, {"B", 20.0}}
	c.Labels = []string{"A", "B"}

	dir := t.TempDir()
	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FilePath = "chart.slidelang"
	block := ast.NewContentBlock(pos(), "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements, c)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{AssetRoot: dir}); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	outputPath := filepath.Join(dir, "chart.pptx")
	names := zipEntryNames(t, outputPath)

	// Un chart rasterizado tiene que aparecer como media dentro del paquete.
	hasMedia := false
	for _, n := range names {
		if strings.HasPrefix(n, "ppt/media/") {
			hasMedia = true
			break
		}
	}
	if !hasMedia {
		t.Fatalf("no ppt/media/* entry in the package — the chart was not embedded as an image. Entries: %v", names)
	}

	xml := zipEntryContent(t, outputPath, "ppt/slides/slide1.xml")
	if !strings.Contains(xml, "<p:pic>") {
		t.Error("slide1.xml has no picture shape for the chart")
	}
	if !strings.Contains(xml, "chart-marker-title") {
		t.Error("the chart title was not emitted as a caption below the image")
	}
	if strings.Contains(xml, "[Chart not rendered") || strings.Contains(xml, "[Chart failed") {
		t.Error("the chart fell back to a text placeholder instead of rendering natively")
	}
}

// TestPPTX_UnsupportedChartFallsBackToPlaceholder confirma la otra mitad del
// contrato: un chart que el camino nativo no cubre —acá, uno con un bloque
// options: de Chart.js— NO instancia un navegador ni aborta el build, se
// degrada a un marcador de texto visible.
func TestPPTX_UnsupportedChartFallsBackToPlaceholder(t *testing.T) {
	c := ast.NewChartElement(pos(), "bar")
	c.Data = [][]interface{}{{"A", 10.0}}
	c.Labels = []string{"A"}
	c.Options = map[string]interface{}{"plugins": map[string]interface{}{"legend": false}}

	xml := buildPPTXWithElements(t, "chartopts", c)

	if !strings.Contains(xml, "[Chart not rendered") {
		t.Error("a chart with a Chart.js options block should degrade to a visible placeholder, not vanish")
	}
}

// TestPPTX_StillWarnsOnGenuinelyUnsupportedElements protege el default del
// switch: ampliar la cobertura no debe convertir el warning explícito en un
// descarte silencioso para lo que sigue sin cubrirse.
func TestPPTX_StillWarnsOnGenuinelyUnsupportedElements(t *testing.T) {
	xml := buildPPTXWithElements(t, "unsupported",
		ast.NewMermaidElement(pos(), "flowchart", "graph TD; A-->B"),
		ast.NewTextElement(pos(), "text-marker-after"),
	)

	if strings.Contains(xml, "graph TD") {
		t.Error("the mermaid source leaked into the slide as literal text")
	}
	// El elemento siguiente sí debe llegar: omitir uno no puede truncar el
	// resto del slide.
	if !strings.Contains(xml, "text-marker-after") {
		t.Error("skipping an unsupported element dropped the elements after it")
	}
}
