// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package generator

import (
	"path/filepath"
	"regexp"
	"strconv"
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

// TestPPTX_NestedListItemsIndentByLevel cubre un hallazgo de code-review:
// tanto pptxAddPointParagraph como pptxAddChecklistParagraph llamaban
// Indent(18, -18) fijo. Level(lvl) sí cambiaba el atributo lvl, pero un marL
// explícito en el mismo pPr gana sobre la sangría por nivel del layout, así
// que el OOXML salía con marL idéntico en lvl="0" y lvl="1" y los subitems
// quedaban visualmente al mismo margen que sus padres.
func TestPPTX_NestedListItemsIndentByLevel(t *testing.T) {
	c := ast.NewChecklistElement(pos())
	parent := ast.NewChecklistItem(pos(), "nested-parent", false)
	parent.SubItems = append(parent.SubItems, *ast.NewChecklistItem(pos(), "nested-child", true))
	c.Items = append(c.Items, *parent)

	points := ast.NewPointsElement(pos())
	pi := ast.NewPointItem(pos(), "points-parent")
	pi.SubPoints = append(pi.SubPoints, *ast.NewPointItem(pos(), "points-child"))
	points.Items = append(points.Items, *pi)

	xml := buildPPTXWithElements(t, "nested", c, points)

	// Recolectar (lvl, marL) de cada párrafo con sangría explícita.
	re := regexp.MustCompile(`<a:pPr[^>]*marL="(\d+)"[^>]*lvl="(\d+)"|<a:pPr[^>]*lvl="(\d+)"[^>]*marL="(\d+)"`)
	byLevel := map[string]map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(xml, -1) {
		marL, lvl := m[1], m[2]
		if marL == "" {
			lvl, marL = m[3], m[4]
		}
		if byLevel[lvl] == nil {
			byLevel[lvl] = map[string]bool{}
		}
		byLevel[lvl][marL] = true
	}

	if len(byLevel["0"]) == 0 || len(byLevel["1"]) == 0 {
		t.Fatalf("expected paragraphs at both lvl=0 and lvl=1, got %v", byLevel)
	}
	for m0 := range byLevel["0"] {
		if byLevel["1"][m0] {
			t.Errorf("lvl=0 and lvl=1 share marL=%s — subitems are not visually nested", m0)
		}
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
	// El título va DENTRO del PNG (RenderChartNativePNG hace
	// opt.Title.Text = elem.Title), así que no debe aparecer también como
	// textbox: la primera versión de pptxAddChart lo emitía dos veces y la
	// segunda copia caía fuera del slide. Hallazgo de code-review; el test de
	// entonces afirmaba justamente el comportamiento con el bug.
	if strings.Contains(xml, "chart-marker-title") {
		t.Error("the chart title was emitted as a caption as well as inside the PNG — it is drawn twice")
	}
	if strings.Contains(xml, "[Chart not rendered") || strings.Contains(xml, "[Chart failed") {
		t.Error("the chart fell back to a text placeholder instead of rendering natively")
	}
}

// TestPPTX_ChartIsClampedToSlide cubre el overflow que no dependía del
// caption: ChartDimensions respeta elem.Width/elem.Height, así que un chart
// cuadrado daba drawHeight = 7.5in arrancando en 1.87in y la imagen quedaba
// casi entera fuera de un canvas de 7.5in.
func TestPPTX_ChartIsClampedToSlide(t *testing.T) {
	c := ast.NewChartElement(pos(), "bar")
	c.Data = [][]interface{}{{"A", 10.0}, {"B", 20.0}}
	c.Labels = []string{"A", "B"}
	c.Width, c.Height = 800, 800 // cuadrado: el peor caso

	xml := buildPPTXWithElements(t, "chartclamp", c)

	// La extensión del shape va como <a:ext cx=".." cy=".."/>. El alto tiene
	// que caber en lo que queda debajo del top de contenido.
	maxH := pptxSlideHeightEMU - pptxContentTopEMU - pptxMarginEMU
	re := regexp.MustCompile(`<a:ext cx="(\d+)" cy="(\d+)"`)
	found := false
	for _, m := range re.FindAllStringSubmatch(xml, -1) {
		cy, _ := strconv.Atoi(m[2])
		cx, _ := strconv.Atoi(m[1])
		if cy > maxH {
			t.Errorf("a shape is %d EMU tall but only %d EMU fit below the content top — it lands off-slide", cy, maxH)
		}
		if cy == maxH {
			found = true
			// El clamp debe preservar el aspect ratio 1:1, no deformar.
			if cx != cy {
				t.Errorf("clamping deformed the chart: %dx%d, want a square", cx, cy)
			}
		}
	}
	if !found {
		t.Error("the oversized chart was not clamped to the available height")
	}
}

// TestPPTX_UnclampedChartKeepsItsSize confirma que el clamp no encoge lo que
// ya cabía.
func TestPPTX_UnclampedChartKeepsItsSize(t *testing.T) {
	w, h := pptxFitInSlide(pptxChartWidthEMU, 1000000, pptxContentTopEMU)
	if w != pptxChartWidthEMU || h != 1000000 {
		t.Errorf("pptxFitInSlide shrank a chart that already fit: got %dx%d", w, h)
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
