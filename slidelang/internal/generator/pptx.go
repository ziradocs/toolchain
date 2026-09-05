// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package generator

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // registra el decoder GIF para image.DecodeConfig
	_ "image/jpeg" // registra el decoder JPEG para image.DecodeConfig
	_ "image/png"  // registra el decoder PNG para image.DecodeConfig
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mmonterroca/pptxgo/drawingml"
	"github.com/mmonterroca/pptxgo/pptx"
	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/generator/config"
)

// pptx.go implementa --format pptx, AST → .pptx vía pptxgo
// (github.com/mmonterroca/pptxgo, MIT, sin dependencias) — pinneada por
// pseudo-versión de commit en go.mod, NO `replace ../../pptxgo`: un replace
// a un directorio fuera de este checkout rompería CI, que solo clona este
// repo (no el repo hermano de pptxgo).
//
// Elementos cubiertos: TextElement (párrafos, bold/italic/code inline por
// segmentos), PointsElement (viñetas), TableElement, ImageElement,
// QuoteElement, ChecklistElement, CodeElement y ChartElement — mapeados vía
// Slide.AddTextBox/AddTable/AddImageFromBytes en freeform, apilados
// verticalmente por altura estimada (pptxgo no mide texto; el resultado es
// editable en PowerPoint si el estimado no calza exacto, no es un layout
// final de precisión). Título/subtítulo sí usan placeholders de layout
// (Slide.Title, PlaceholderSubTitle) — evita inventar coordenadas para esos
// dos, a diferencia del cuerpo.
//
// La propiedad que define este formato es que NO usa Chromium: Generator ni
// siquiera tiene el campo, y es lo que hace de --format pptx la salida que
// funciona sin navegador instalado. Eso decide qué queda fuera:
//
//   - ChartElement se cubre SOLO por el camino nativo en Go
//     (renderer.RenderChartNativePNGWithColors, go-analyze/charts): bar/line/pie/
//     doughnut, no modo JSON. Un bloque options: NO descalifica el chart —
//     se dibuja aproximado y se avisa qué claves se ignoraron; ver
//     pptxChartIgnoringOptions para por qué la política acá difiere de la
//     que aplican HTML/PDF/DOCX. Un ChartType sin mapeo nativo (radar,
//     combo) sí se omite con warning, en vez de instanciar un navegador.
//     El título NO se emite como caption: el renderer nativo ya lo dibuja
//     dentro del PNG.
//
//     Es una solución puente: en cuanto pptxgo emita charts OOXML nativos
//     (mmonterroca/pptxgo#25), PowerPoint redibujaría el chart con los datos
//     editables y la mayoría de estas claves tendrían equivalente directo en
//     su vocabulario, en vez de tener que replicarlas en un rasterizador.
//   - Mermaid/PlantUML se cubren CONDICIONALMENTE (issue #144): solo con
//     --diagram-backend kroki (+ opcionalmente --kroki-server), pidiéndole
//     al servidor Kroki el PNG directo en vez de svg (chromium.KrokiFetcher
//     ya soporta el parámetro format). Sin ese flag se omiten con un
//     warning que dice qué hacer — nunca en silencio, y nunca salen a la
//     red sin que el operador lo haya pedido: --format pptx se define por
//     no necesitar navegador NI red en la máquina.
//   - Map/Math NO tienen camino bajo ningún --diagram-backend: Leaflet y
//     MathJax necesitan un navegador de verdad, y Kroki no los resuelve.
//   - SpecialBlockElement/CodeGroupElement/GridElement tampoco: son
//     contenedores con layout propio, no un shape suelto.
//
// Un elemento no cubierto se omite con un warning explícito (ver
// pptxAddElement), nunca en silencio.
//
// Cada ast.ContentBlock → un slide, mismo mapeo que el generador HTML
// propio de slidelang (CLAUDE.md: "cada ContentBlock es un slide").

// Geometría del canvas: pptx.New() sin WithSlideSize usa el default 16:9
// (13.333x7.5in, mismas proporciones que slidesPDFOptions en pdf.go) — estos
// valores son relativos a ese mismo canvas, en EMU (1 pulgada = 914400 EMU).
const (
	pptxMarginEMU         = 457200   // 0.5in
	pptxContentTopEMU     = 1706880  // ~1.87in: debajo del título en LayoutTitleAndContent
	pptxTitleSlideBodyTop = 3200400  // ~3.5in: debajo de ctrTitle+subTitle centrados
	pptxContentWidthEMU   = 11430000 // 12.5in (13.333in canvas - 2*0.5in margen)
	pptxSlideHeightEMU    = 6858000  // 7.5in
	pptxLineHeightEMU     = 274320   // ~0.3in por línea de texto ~18pt, estimado (pptxgo no mide texto)
	pptxParaGapEMU        = 91440    // 0.1in de separación entre elementos consecutivos
	pptxCharsPerLine      = 90       // estimado de wrap a ancho completo, para el cálculo de altura
	pptxDefaultImageEMU   = 3200400  // ~3.5in: alto por defecto si no se puede leer la imagen (URL remota o lectura fallida)
	pptxQuoteIndentEMU    = 457200   // 0.5in extra de sangría para el bloque de cita
	pptxChartWidthEMU     = 6858000  // 7.5in: ancho fijo del chart, el alto sale de su aspect ratio
	pptxSlideWidthEMU     = 12192000 // 13.333in: ancho completo del canvas (pptxgo.SlideSizeWidescreen16x9Width) — pptx.New() nunca se llama con WithSlideSize acá
)

// Fuentes y glifos de los elementos añadidos después del MVP v0.
const (
	pptxMonoFont          = "Consolas"        // monoespaciada para CodeElement; PowerPoint cae a una equivalente si no está
	pptxSymbolFont        = "Segoe UI Symbol" // cubre U+2610/U+2611, que Arial no siempre trae
	pptxCheckboxChecked   = "☑"               // ☑
	pptxCheckboxUnchecked = "☐"               // ☐
	pptxCodeFontSizePt    = 14.0              // el código a cuerpo completo (18pt) se sale del slide muy rápido
)

// Sangría de listas (viñetas y checklist), en puntos. El margen crece por
// nivel; el colgante se queda fijo para no desalinear la viñeta de su texto.
// Ver pptxApplyListIndent.
const (
	pptxListIndentBasePt = 18.0
	pptxListIndentStepPt = 18.0
	pptxListHangingPt    = 18.0
)

func (g *Generator) generatePPTX(astNode *ast.AST, outputDir string, opts GeneratorOptions) error {
	g.logger.Info("PPTX", "Building PPTX presentation...")

	p := pptx.New()

	// kroki es el directorio temporal (creado LAZY, solo si de verdad
	// aparece un mermaid/plantuml con --diagram-backend kroki) donde
	// KrokiFetcher.FetchAndSave escribe el PNG que luego se lee y se
	// embebe -- --format pptx nunca persiste archivos de diagrama junto a
	// su salida (a diferencia de offline-assets), así que este directorio
	// se limpia al terminar el build, un solo temp dir para todo el deck,
	// no uno por diagrama.
	kroki := &pptxKrokiContext{}
	defer kroki.cleanup()

	// Resuelto una sola vez (issue #179): a diferencia de header/footer,
	// watermark es global sin cascada por slide/layout, así que no hay
	// nada que recalcular por ContentBlock. astNode.FrontMatter nil es
	// defensivo, no esperado — slidelang exige frontmatter en el parser —
	// pero un método a través de un puntero nil (BuildVariables) es seguro;
	// un acceso directo a campo (Watermark) no lo sería.
	var watermark renderer.ResolvedWatermark
	var hasWatermark bool
	if astNode.FrontMatter != nil {
		watermark, hasWatermark = renderer.ResolveWatermark(astNode.FrontMatter.Watermark, astNode.FrontMatter.BuildVariables())
	}
	if hasWatermark {
		g.logger.Warn("PPTX: watermark se dibuja con opacidad aproximada por pre-mezcla contra el fondo del slide (pptxgo no expone alpha en el color de texto) y siempre como una sola marca centrada — 'repeat: true' se ignora en --format pptx (ver llm-kit/reference/frontmatter.md)")
	}

	for i := range astNode.ContentBlocks {
		g.pptxAddSlide(p, &astNode.ContentBlocks[i], opts, kroki, watermark, hasWatermark)
	}

	outputPath := filepath.Join(outputDir, resolveOutputFilename(astNode, "pptx"))
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create pptx file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := p.Save(f); err != nil {
		return fmt.Errorf("failed to save pptx: %w", err)
	}

	g.logger.Info("PPTX", "✅ PPTX presentation generated successfully: %s", outputPath)
	return nil
}

// pptxAddSlide añade un slide para block. Usa LayoutTitleSlide (título
// centrado + subtítulo) para el BlockType "title" y LayoutTitleAndContent
// para el resto — el mismo mapeo semántico que ya usa el HTML propio de
// slidelang (template/base.go: Heading para bloques "title", Title para
// los demás).
func (g *Generator) pptxAddSlide(p *pptx.Presentation, block *ast.ContentBlock, opts GeneratorOptions, kroki *pptxKrokiContext, watermark renderer.ResolvedWatermark, hasWatermark bool) {
	// config.IsSlideTitle y no `== "title"`: desde el issue #239 un bloque
	// flex puede declarar `layout: title_slide` (y strict siempre pudo con
	// SLIDE title_slide), y esos también van con LayoutTitleSlide. El
	// converter de HTML ya resolvía el tipo con esta misma función; PPTX
	// comparaba contra el literal y por eso un title_slide caía en el layout
	// de contenido.
	isTitleBlock := config.IsSlideTitle(block.BlockType)

	layout := pptx.LayoutTitleAndContent
	if isTitleBlock {
		layout = pptx.LayoutTitleSlide
	}
	s := p.AddSlide(pptx.WithLayout(layout))

	// Primer shape del spTree ⇒ detrás de todo lo demás (issue #179): la
	// pre-mezcla de opacidad solo es visualmente exacta contra el fondo del
	// slide, así que tiene que quedar debajo de cualquier contenido opaco
	// que se agregue después. AddSlide/AddPlaceholder/AddTextBox de acá en
	// adelante siempre agregan encima en el spTree.
	if hasWatermark {
		g.pptxAddWatermark(s, watermark)
	}

	heading := block.Title
	if isTitleBlock {
		heading = block.Heading
		if heading == "" {
			heading = block.Title
		}
	}
	if heading != "" {
		s.Title(heading)
	}

	cursorY := pptxContentTopEMU
	if isTitleBlock {
		if block.Subtitle != "" {
			s.AddPlaceholder(pptx.PlaceholderSubTitle, 1).AddParagraph().Text(block.Subtitle)
		}
		// LayoutTitleSlide solo declara ctrTitle+subTitle (sin body) — Elements
		// extra en un bloque "title" (el linter los desaconseja, no los prohíbe)
		// se apilan en freeform debajo del título+subtítulo centrados, igual
		// que en un bloque de contenido normal.
		cursorY = pptxTitleSlideBodyTop
	}

	for i := range block.Elements {
		cursorY = g.pptxAddElement(s, block.Elements[i], cursorY, opts, kroki)
	}
}

// pptxAddWatermark dibuja rw como una única marca centrada, rotada,
// cubriendo el slide completo (issue #179). Dos divergencias deliberadas
// respecto a HTML/PDF, ambas documentadas en
// llm-kit/reference/frontmatter.md y advertidas una sola vez por build (ver
// generatePPTX):
//
//   - Opacidad aproximada: pptxgo no expone un setter público para el
//     alpha de drawingml.SrgbClr (el campo existe en el struct, el método
//     no), así que en vez de un color translúcido se dibuja el color plano
//     resultante de pre-mezclar rw.Color contra el fondo del slide
//     (blanco — pptx.go nunca llama Slide.Background) a la opacidad
//     pedida. Solo es exacto si lo que quede encima es opaco, que es
//     exactamente por qué este shape se agrega PRIMERO en pptxAddSlide
//     (antes de título/subtítulo/elementos): cualquier otra cosa queda
//     por encima en el spTree.
//   - rw.Repeat se ignora siempre: un mosaico serían N shapes rotados por
//     slide (PowerPoint los listaría como N objetos individuales en el
//     panel de selección), y detrás de contenido opaco un mosaico se lee
//     como ruido en cuanto una tabla o imagen lo cruza. Una sola marca
//     centrada es la forma convencional del watermark de Office y la
//     única que sobrevive bien a la colocación "detrás del contenido".
func (g *Generator) pptxAddWatermark(s *pptx.Slide, rw renderer.ResolvedWatermark) {
	r, gr, b, ok := a11y.ParseColor(rw.Color)
	if !ok {
		r, gr, b = 0, 0, 0
	}
	blended := renderer.BlendOverOpaque(
		color.RGBA{R: r, G: gr, B: b, A: 255},
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
		rw.Opacity,
	)

	points := 44.0
	if inches, err := util.ParseLengthInches(rw.FontSize); err == nil {
		points = inches * 72
	}

	tb := s.AddTextBox(0, 0, pptxSlideWidthEMU, pptxSlideHeightEMU)
	tb.Anchor(pptx.AnchorMiddle)
	tb.WordWrap(false)
	tb.Rotation(rw.Rotation)
	tb.AddParagraph().
		Text(rw.Text).
		FontSize(points).
		Color(drawingml.Color{R: blended.R, G: blended.G, B: blended.B}).
		Alignment(pptx.AlignCenter)
}

// pptxHeadingRe reconoce el `<hN id="…">…</hN>` que
// parser.buildHeadingElement produce para un encabezado de subsección
// (issue #194). Es la misma forma que formatter.subsectionHeadingRe
// reconoce del otro lado.
var pptxHeadingRe = regexp.MustCompile(`(?s)^<h([1-6])(?: id="[^"]*")?>(.*)</h[1-6]>$`)

// pptxInlineHTMLToMarkdown son las inversas de los formatos inline que
// renderer.ProcessInlineMarkdownFormatsSecure puede emitir dentro de un
// encabezado. Se aplican en orden: `<code>` primero, porque su contenido no
// debe reinterpretarse, y el resto después.
//
// La inversión es determinista, no una heurística: el procesador ESCAPA el
// HTML antes de aplicar Markdown, así que un `<strong>` en la salida solo
// puede venir de `**`. Un `<strong>` que el autor haya tecleado nunca llegó
// a ser una tag — quedó como `&lt;strong&gt;` y sigue ahí, sin tocar.
var pptxInlineHTMLToMarkdown = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?s)<code>(.*?)</code>`), "`$1`"},
	{regexp.MustCompile(`(?s)<strong>(.*?)</strong>`), "**$1**"},
	{regexp.MustCompile(`(?s)<em>(.*?)</em>`), "*$1*"},
	{regexp.MustCompile(`(?s)<del>(.*?)</del>`), "~~$1~~"},
	{regexp.MustCompile(`(?s)<mark>(.*?)</mark>`), "==$1=="},
	{regexp.MustCompile(`(?s)<a href="([^"]*)">(.*?)</a>`), "[$2]($1)"},
}

// pptxTagRe borra cualquier tag que sobreviva a las inversas de arriba —
// red de seguridad, no la ruta normal.
var pptxTagRe = regexp.MustCompile(`<[^>]*>`)

// pptxTextContent devuelve el texto que va a la diapositiva por un
// TextElement.
//
// Un encabezado de subsección llega como HTML crudo en Content, así que hay
// que des-renderizarlo: si se mandara tal cual, la diapositiva mostraría
// `<h3 id="foo">Foo</h3>` literal. Y no alcanza con borrar las tags:
// `pptxAddText` resuelve el formato inline con `pptxSplitInline`, que lee
// MARKDOWN. Borrar `<strong>` dejaría "Important" como un run plano, cuando
// antes de #194 —con la línea todavía como texto literal `### **Important**`—
// sí salía en negrita. Por eso se reconstruye el Markdown de origen y no el
// texto pelado: el PPTX de cualquier deck sale igual que antes.
//
// Cubre los seis formatos que un encabezado puede traer: negrita, cursiva,
// código, tachado, resaltado y enlaces, más el `<span lang>` de
// accesibilidad, que tiene su propia inversa en core
// (renderer.LangSpanHTMLToSource) y se aplica primero por la misma razón
// que en el formatter. Los dos primeros y el lang son los que
// `pptxSplitInline` convierte en runs; el resto vuelve a su marcador
// Markdown, que es exactamente lo que la diapositiva mostraba antes.
//
// El des-escapado va AL FINAL, y ese orden importa: un encabezado que
// contenga `&lt;strong&gt;` —una tag que el autor tecleó y que el
// procesador neutralizó— se convertiría en una tag real si se des-escapara
// primero, y las inversas de arriba la tomarían por Markdown.
//
// core/formatter tiene su propia versión de esto (formatSubsectionHeading),
// que hoy borra el énfasis en vez de invertirlo. Las dos convergen en el
// issue #260.
func pptxTextContent(e *ast.TextElement) string {
	if !e.IsRawHTML {
		return e.Content
	}
	m := pptxHeadingRe.FindStringSubmatch(e.Content)
	if m == nil {
		// TextElement RawHTML que no es un encabezado. Hoy no existe
		// ninguno (buildHeadingElement es el único productor), pero si
		// apareciera, mostrar el texto sin tags es mejor que el markup.
		return pptxInlineHTMLToText(e.Content)
	}
	level := int(m[1][0] - '0')
	return strings.Repeat("#", level) + " " + pptxInlineHTMLToText(m[2])
}

// pptxInlineHTMLToText convierte el HTML inline de un encabezado de vuelta a
// su fuente Markdown. Ver pptxTextContent para el porqué del orden.
func pptxInlineHTMLToText(html string) string {
	out := renderer.LangSpanHTMLToSource(html)
	for _, r := range pptxInlineHTMLToMarkdown {
		out = r.re.ReplaceAllString(out, r.repl)
	}
	out = pptxTagRe.ReplaceAllString(out, "")
	return renderer.UnescapeHTML(out)
}

// pptxAddElement despacha por tipo de ast.Element y devuelve el cursorY
// actualizado para el próximo elemento. Un tipo no cubierto (ver el
// comentario del paquete para qué queda fuera y por qué) se omite con un
// warning explícito — nunca en silencio, para que "faltan diagramas en este
// deck" sea visible en el log del build, no un misterio.
func (g *Generator) pptxAddElement(s *pptx.Slide, elem ast.Element, cursorY int, opts GeneratorOptions, kroki *pptxKrokiContext) int {
	switch e := elem.(type) {
	case *ast.TextElement:
		return g.pptxAddText(s, pptxTextContent(e), cursorY)
	case *ast.PointsElement:
		return g.pptxAddPoints(s, e, cursorY)
	case *ast.TableElement:
		return g.pptxAddTable(s, e, cursorY)
	case *ast.ImageElement:
		return g.pptxAddImage(s, e, cursorY, opts)
	case *ast.QuoteElement:
		return g.pptxAddQuote(s, e, cursorY)
	case *ast.ChecklistElement:
		return g.pptxAddChecklist(s, e, cursorY)
	case *ast.CodeElement:
		return g.pptxAddCode(s, e, cursorY)
	case *ast.ChartElement:
		return g.pptxAddChart(s, e, cursorY, opts)
	case *ast.MermaidElement:
		return g.pptxAddDiagram(s, "mermaid", e.Content, e.Title, cursorY, opts, kroki)
	case *ast.PlantUMLElement:
		return g.pptxAddDiagram(s, "plantuml", e.Content, e.Title, cursorY, opts, kroki)
	case *ast.MapElement:
		g.logger.Warn("PPTX: map element skipped, no hay camino sin Chromium (Leaflet necesita navegador) — --format pptx no usa Chromium")
		return g.pptxAddText(s, "[Map not rendered]", cursorY)
	case *ast.MathElement:
		g.logger.Warn("PPTX: math element skipped, no hay camino sin Chromium (MathJax necesita navegador) — --format pptx no usa Chromium")
		return g.pptxAddText(s, "[Math not rendered]", cursorY)
	default:
		g.logger.Warn("PPTX: element type %T not supported yet, skipped (issue #144 tracks the remaining coverage: SpecialBlock/CodeGroup/Grid)", elem)
		return cursorY
	}
}

// pptxEstimateLines estima cuántas líneas ocupará text envuelto a
// pptxCharsPerLine caracteres — pptxgo no mide texto (no hay motor de
// layout de fuentes embebido), así que el apilado vertical de v0 usa este
// estimado en vez de una medición real. Subestima con contenido muy denso
// de caracteres anchos; el resultado sigue siendo editable en PowerPoint.
func pptxEstimateLines(text string) int {
	lines := strings.Split(text, "\n")
	total := 0
	for _, line := range lines {
		n := len(line)/pptxCharsPerLine + 1
		total += n
	}
	if total < 1 {
		total = 1
	}
	return total
}

// pptxInlineSegment es un fragmento de texto con el formato inline que le
// aplica (negrita/cursiva/código) — la unidad que pptxAddText emite como
// una llamada Paragraph.Text(seg).<formato>() encadenada, replicando el
// patrón de multi-run de doclang/internal/generator/docx.go
// (renderInlineMarkdown) pero sobre la API fluida de pptxgo en vez de
// domain.Run/domain.Paragraph.
type pptxInlineSegment struct {
	text   string
	bold   bool
	italic bool
	code   bool
	lang   string
}

var (
	pptxCodeRe   = regexp.MustCompile("`([^`]+)`")
	pptxBoldRe   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	pptxItalicRe = regexp.MustCompile(`\*([^*]+)\*`)
)

// pptxBasicPatterns es el subconjunto code/bold/italic usado tanto por el
// scan de nivel superior de pptxSplitInline como, recursivamente, por el
// texto INTERNO de un [texto]{lang=xx} ya validado (issue #63 code review,
// finding #2) — nunca links ni otro span de idioma anidado: el
// content-class de renderer.InlineLangSpanPattern excluye "[", así que
// ninguno de los dos puede aparecer ahí adentro para empezar (mismo
// razonamiento que core/renderer/sanitizer.go's inlineSpanPattern doc
// comment sobre por qué excluir "[" evita el straddle).
var pptxBasicPatterns = []struct {
	re                 *regexp.Regexp
	bold, italic, code bool
}{
	{pptxCodeRe, false, false, true},
	{pptxBoldRe, true, false, false},
	{pptxItalicRe, false, true, false},
}

// pptxSplitBasic segmenta content en texto plano + code/bold/italic
// solamente (sin lang) — mismo orden de prioridad que docx.go (code antes
// que bold, para no interpretar "**" dentro de un `código`). Reusada por
// pptxSplitInline (nivel superior) y por pptxSplitLangSpanInner (texto
// interno de un span de idioma, finding #2).
func pptxSplitBasic(content string) []pptxInlineSegment {
	var segments []pptxInlineSegment
	remaining := content
	pos := 0

	for pos < len(remaining) {
		var best *struct {
			start, end         int
			inner              string
			bold, italic, code bool
		}
		bestRelPos := len(remaining) - pos

		for _, re := range pptxBasicPatterns {
			loc := re.re.FindStringSubmatchIndex(remaining[pos:])
			if loc == nil || loc[0] >= bestRelPos {
				continue
			}
			bestRelPos = loc[0]
			best = &struct {
				start, end         int
				inner              string
				bold, italic, code bool
			}{
				start: pos + loc[0], end: pos + loc[1],
				inner: remaining[pos+loc[2] : pos+loc[3]],
				bold:  re.bold, italic: re.italic, code: re.code,
			}
		}

		if best == nil {
			segments = append(segments, pptxInlineSegment{text: remaining[pos:]})
			break
		}
		if best.start > pos {
			segments = append(segments, pptxInlineSegment{text: remaining[pos:best.start]})
		}
		segments = append(segments, pptxInlineSegment{text: best.inner, bold: best.bold, italic: best.italic, code: best.code})
		pos = best.end
	}

	return segments
}

// pptxSplitInline segmenta content en texto plano + code/bold/italic/lang.
// Los links [text](url) quedan fuera del alcance v0 (issue de seguimiento):
// un link se muestra como texto plano con su sintaxis markdown literal en
// vez de resolverse.
func pptxSplitInline(content string) []pptxInlineSegment {
	var segments []pptxInlineSegment
	remaining := content
	pos := 0

	for pos < len(remaining) {
		type basicMatch struct {
			start, end         int
			inner              string
			bold, italic, code bool
		}
		var best *basicMatch
		// bestRelPos se mantiene relativo a remaining[pos:] durante toda
		// esta iteración (pos es constante aquí), así que se compara
		// directo contra loc[0] — sin re-relativizar una posición
		// absoluta a mitad de la comparación, que es lo que hacía frágil
		// agregar un patrón más.
		bestRelPos := len(remaining) - pos

		for _, re := range pptxBasicPatterns {
			loc := re.re.FindStringSubmatchIndex(remaining[pos:])
			if loc == nil || loc[0] >= bestRelPos {
				continue
			}
			bestRelPos = loc[0]
			best = &basicMatch{
				start: pos + loc[0], end: pos + loc[1],
				inner: remaining[pos+loc[2] : pos+loc[3]],
				bold:  re.bold, italic: re.italic, code: re.code,
			}
		}

		var langLoc []int
		if loc := renderer.InlineLangSpanPattern.FindStringSubmatchIndex(remaining[pos:]); loc != nil && loc[0] < bestRelPos {
			langLoc = loc
			best = nil // el span de idioma gana sobre code/bold/italic en esta posición
		}

		if langLoc != nil {
			matchStart, matchEnd := pos+langLoc[0], pos+langLoc[1]
			inner := remaining[pos+langLoc[2] : pos+langLoc[3]]
			lang := remaining[pos+langLoc[4] : pos+langLoc[5]]

			if matchStart > pos {
				segments = append(segments, pptxInlineSegment{text: remaining[pos:matchStart]})
			}
			if a11y.IsValidLangTag(lang) {
				// Procesar el texto interno recursivamente por
				// code/bold/italic (finding #2) en vez de emitirlo
				// verbatim con los asteriscos/backticks literales, y
				// estampar el idioma en cada segmento resultante.
				for _, inner := range pptxSplitBasic(inner) {
					inner.lang = lang
					segments = append(segments, inner)
				}
			} else {
				// Tag inválido: degradar al texto LITERAL completo
				// (con corchetes/llaves — finding #5) en vez de
				// quedarse silenciosamente solo con el texto interno,
				// que escondería el error de tipeo del autor.
				segments = append(segments, pptxInlineSegment{text: remaining[matchStart:matchEnd]})
			}
			pos = matchEnd
			continue
		}

		if best == nil {
			segments = append(segments, pptxInlineSegment{text: remaining[pos:]})
			break
		}
		if best.start > pos {
			segments = append(segments, pptxInlineSegment{text: remaining[pos:best.start]})
		}
		segments = append(segments, pptxInlineSegment{text: best.inner, bold: best.bold, italic: best.italic, code: best.code})
		pos = best.end
	}

	return segments
}

// pptxApplyInline escribe content en para como una o más runs, resolviendo
// bold/italic/code por segmento vía pptxSplitInline — única función que
// emite Paragraph.Text(...), reusada por pptxAddText y
// pptxAddPointParagraph para que ninguna de las dos rutas de texto (párrafo
// suelto, item de viñeta) deje escapar sintaxis markdown literal sin
// resolver (regresión real encontrada vía smoke-test E2E: un primer borrador
// de pptxAddPointParagraph llamaba Text(item.Content) directo, sin pasar por
// pptxSplitInline, y un `*italic*` dentro de un bullet se veía literal en el
// .pptx generado en vez de en cursiva).
func pptxApplyInline(para *pptx.Paragraph, content string) {
	pptxApplyInlineBase(para, content, false)
}

// pptxApplyInlineBase es pptxApplyInline con un estilo base que se aplica a
// TODOS los segmentos, no solo a los que el markdown marcó. baseItalic existe
// para QuoteElement: una cita debe verse en cursiva completa, y no basta con
// llamar para.Italic() después de pptxApplyInline porque Paragraph.Text()
// reinicia curRuns en cada llamada — el Italic() final solo alcanzaría al
// último segmento. El estilo tiene que entrar por segmento, acá.
func pptxApplyInlineBase(para *pptx.Paragraph, content string, baseItalic bool) {
	for _, seg := range pptxSplitInline(content) {
		para.Text(seg.text)
		if seg.bold {
			para.Bold()
		}
		if seg.italic || baseItalic {
			para.Italic()
		}
		if seg.code {
			para.Font("Courier New")
		}
		// seg.lang ya viene validado por a11y.IsValidLangTag en
		// pptxSplitInline (un tag inválido nunca llega a set near text con
		// lang != "") — la comprobación acá es defensa en profundidad,
		// barata de mantener.
		if seg.lang != "" && a11y.IsValidLangTag(seg.lang) {
			para.Lang(seg.lang)
		}
	}
}

// pptxAddText agrega content como un textbox de ancho completo en cursorY,
// con el formato inline resuelto por pptxApplyInline, y devuelve el cursorY
// actualizado.
func (g *Generator) pptxAddText(s *pptx.Slide, content string, cursorY int) int {
	if strings.TrimSpace(content) == "" {
		return cursorY
	}

	height := pptxEstimateLines(content) * pptxLineHeightEMU
	tb := s.AddTextBox(pptxMarginEMU, cursorY, pptxContentWidthEMU, height)
	pptxApplyInline(tb.AddParagraph(), content)

	return cursorY + height + pptxParaGapEMU
}

// pptxAddQuote agrega e como un bloque citado: la prosa en cursiva completa
// y, si hay autor o fuente, una línea de atribución debajo. La cursiva va por
// segmento vía pptxApplyInlineBase, no con un Italic() final — ver el
// comentario de esa función.
func (g *Generator) pptxAddQuote(s *pptx.Slide, e *ast.QuoteElement, cursorY int) int {
	if strings.TrimSpace(e.Content) == "" && e.Author == "" && e.Source == "" {
		return cursorY
	}

	attribution := pptxQuoteAttribution(e)

	lines := pptxEstimateLines(e.Content)
	if attribution != "" {
		lines += pptxEstimateLines(attribution)
	}
	height := lines * pptxLineHeightEMU

	// Indentado respecto al margen: es el único recurso visual de "cita" que
	// queda sin inventar una forma decorativa aparte (pptxgo dibuja shapes,
	// pero una barra vertical exigiría coordenadas propias y un segundo shape
	// que el usuario tendría que mover a mano al editar en PowerPoint).
	tb := s.AddTextBox(pptxMarginEMU+pptxQuoteIndentEMU, cursorY, pptxContentWidthEMU-pptxQuoteIndentEMU, height)

	if strings.TrimSpace(e.Content) != "" {
		pptxApplyInlineBase(tb.AddParagraph(), e.Content, true)
	}
	if attribution != "" {
		tb.AddParagraph().Text(attribution)
	}

	return cursorY + height + pptxParaGapEMU
}

// pptxQuoteAttribution arma la línea "— Autor, Fuente" a partir de los campos
// que estén presentes, o "" si no hay ninguno.
func pptxQuoteAttribution(e *ast.QuoteElement) string {
	parts := make([]string, 0, 2)
	if e.Author != "" {
		parts = append(parts, e.Author)
	}
	if e.Source != "" {
		parts = append(parts, e.Source)
	}
	if len(parts) == 0 {
		return ""
	}
	return "— " + strings.Join(parts, ", ")
}

// pptxAddChecklist agrega e como una lista con viñeta de casilla —☑ marcada,
// ☐ sin marcar— en vez del punto de PointsElement. El estado se codifica en
// el carácter de la viñeta y no en el texto, así que sigue siendo legible al
// editar en PowerPoint y no se pierde si el usuario reescribe el contenido.
func (g *Generator) pptxAddChecklist(s *pptx.Slide, e *ast.ChecklistElement, cursorY int) int {
	totalLines := 0
	for _, item := range e.Items {
		totalLines += pptxEstimateLines(item.Content)
		for _, sub := range item.SubItems {
			totalLines += pptxEstimateLines(sub.Content)
		}
	}
	if totalLines < 1 {
		totalLines = 1
	}
	height := totalLines * pptxLineHeightEMU

	tb := s.AddTextBox(pptxMarginEMU, cursorY, pptxContentWidthEMU, height)
	for _, item := range e.Items {
		pptxAddChecklistParagraph(tb, item, 0)
		for _, sub := range item.SubItems {
			pptxAddChecklistParagraph(tb, sub, 1)
		}
	}

	return cursorY + height + pptxParaGapEMU
}

// pptxApplyListIndent aplica la sangría de una lista EN FUNCIÓN del nivel.
//
// Hallazgo de code-review: tanto las viñetas como el checklist llamaban
// Indent(18, -18) fijo para todos los niveles. Level(lvl) sí cambia el
// atributo lvl del párrafo, pero un marL explícito en el mismo pPr GANA sobre
// la sangría que el layout asigna por nivel, así que el OOXML salía con
// marL="228600" idéntico en lvl="0" y lvl="1" — los subitems quedaban
// visualmente al mismo margen que sus padres, sin anidar. Verificado en el
// slide 4 de examples/checklist_demo.slidelang, que sí tiene dos niveles.
//
// El primer argumento es el margen izquierdo del párrafo y el segundo la
// sangría de primera línea (negativa = colgante, para que la viñeta quede
// fuera del texto). Escalando solo el margen y dejando el colgante fijo, cada
// nivel se corre pptxListIndentStepPt sin desalinear la viñeta de su texto.
func pptxApplyListIndent(para *pptx.Paragraph, level int) {
	para.Indent(pptxListIndentBasePt+float64(level)*pptxListIndentStepPt, -pptxListHangingPt)
}

func pptxAddChecklistParagraph(tb *pptx.TextBox, item ast.ChecklistItem, level int) {
	bullet := pptxCheckboxUnchecked
	if item.Checked {
		bullet = pptxCheckboxChecked
	}
	para := tb.AddParagraph().Level(level)
	pptxApplyListIndent(para, level)
	// "Segoe UI Symbol" cubre U+2610/U+2611 en Windows y PowerPoint cae a una
	// fuente equivalente en macOS; Arial (la que usa pptxAddPointParagraph
	// para "•") no siempre trae esos dos glifos.
	para.Bullet(bullet, pptxSymbolFont)
	pptxApplyInline(para, item.Content)
}

// pptxAddCode agrega e como un textbox monoespaciado sin viñeta. NO pasa por
// pptxApplyInline a propósito: dentro de un bloque de código un `*` o un
// backtick son contenido literal, no marcado, y resolverlos como markdown
// corrompería el código mostrado. Paragraph.Text ya convierte cada \n en un
// <a:br/>, así que el bloque entero cabe en un solo párrafo conservando sus
// saltos de línea.
func (g *Generator) pptxAddCode(s *pptx.Slide, e *ast.CodeElement, cursorY int) int {
	if strings.TrimSpace(e.Content) == "" {
		return cursorY
	}

	// Una línea de código no se envuelve como prosa: se estima por número de
	// líneas reales, no por pptxCharsPerLine, porque un bloque de código
	// típico es angosto y contarlo como prosa sobreestimaría la altura.
	lineCount := strings.Count(e.Content, "\n") + 1
	height := lineCount * pptxLineHeightEMU

	tb := s.AddTextBox(pptxMarginEMU, cursorY, pptxContentWidthEMU, height)
	para := tb.AddParagraph().NoBullet()
	para.Text(e.Content)
	para.Font(pptxMonoFont)
	para.FontSize(pptxCodeFontSizePt)

	return cursorY + height + pptxParaGapEMU
}

// pptxAddChart rasteriza e a PNG con el renderer nativo en Go
// (renderer.RenderChartNativePNGWithColors, go-analyze/charts) y lo embebe como imagen.
//
// Nativo-only a propósito: --format pptx no instancia un ChromiumRenderer en
// ningún punto (Generator ni siquiera tiene el campo), y ésa es justo la
// propiedad que hace de PPTX la salida que funciona sin navegador. Caer a
// Chromium acá para los charts que el camino nativo no cubre —los de tipo no
// mapeado, en modo JSON, o con un bloque options: de Chart.js— cambiaría esa
// garantía por un elemento; se omiten con warning, igual que antes, en vez de
// arrastrar una dependencia de navegador a este formato.
func (g *Generator) pptxAddChart(s *pptx.Slide, e *ast.ChartElement, cursorY int, opts GeneratorOptions) int {
	width, height := renderer.ChartDimensions(e)

	// Un bloque options: NO descalifica el chart acá, a diferencia de lo que
	// decide renderer.SupportsNativeChartRendering — ver pptxChartIgnoringOptions.
	target, droppedOptions := pptxChartIgnoringOptions(e)

	// motor-temas-v2.md §2.2: mismo chart-cat-* que el camino HTML/PDF
	// offline resuelve en offline.go — sin esto, pptx era el único formato
	// que se quedaba con la paleta fija mientras HTML y PDF ya respetaban
	// el tema.
	data, ok, err := renderer.RenderChartNativePNGWithColors(target, width, height, resolveChartCategoricalColors(opts))
	if !ok {
		g.logger.Warn("PPTX: chart type %q no tiene render nativo (tipo no mapeado o modo JSON), omitido — --format pptx no usa Chromium", e.ChartType)
		return g.pptxAddText(s, fmt.Sprintf("[Chart not rendered: %s]", e.ChartType), cursorY)
	}
	if len(droppedOptions) > 0 {
		g.logger.Warn("PPTX: el chart %q se dibujó de forma aproximada — de options: %s el renderer nativo solo aplica el título (--format pptx no usa Chromium, que es quien las honra completas en HTML/PDF)", e.ChartType, strings.Join(droppedOptions, ", "))
	}
	if err != nil {
		g.logger.Warn("PPTX: falló el render nativo del chart %q: %v", e.ChartType, err)
		return g.pptxAddText(s, fmt.Sprintf("[Chart failed to render: %s]", e.ChartType), cursorY)
	}

	// Ancho fijo con alto derivado del aspect ratio que pidió el chart, para
	// no deformarlo: es el mismo criterio que pptxAddImage aplica a las
	// imágenes, pero sin releer las dimensiones del PNG (acá ya se conocen).
	drawWidth := pptxChartWidthEMU
	drawHeight := pptxDefaultImageEMU
	if width > 0 && height > 0 {
		drawHeight = drawWidth * height / width
	}
	drawWidth, drawHeight = pptxFitInSlide(drawWidth, drawHeight, cursorY)

	s.AddImageFromBytesWithSize(data, pptxMarginEMU, cursorY, drawWidth, drawHeight)

	// e.Title NO se emite como caption: el renderer nativo ya lo dibuja DENTRO
	// del PNG (renderer.RenderChartNativePNGWithColors hace opt.Title.Text = elem.Title).
	// Añadirlo debajo lo duplicaba, y con las dimensiones por defecto (800x600)
	// la segunda copia además caía fuera del slide — el PNG terminaba en 7.492in
	// sobre un canvas de 7.5in, así que el textbox arrancaba en 7.592in.
	// Hallazgo de code-review.
	return cursorY + drawHeight + pptxParaGapEMU
}

// pptxKrokiContext es el directorio temporal compartido por todos los
// fetches Kroki de un mismo build de PPTX (issue #144) — creado LAZY (nil
// hasta el primer diagrama que de verdad lo necesite) para no pagar el
// costo de un MkdirTemp/RemoveAll en decks sin mermaid/plantuml, y
// compartido entre diagramas para no crear un directorio por cada uno.
type pptxKrokiContext struct {
	tempDir string
}

// dir devuelve el directorio temporal, creándolo en la primera llamada.
func (k *pptxKrokiContext) dir() (string, error) {
	if k.tempDir != "" {
		return k.tempDir, nil
	}
	dir, err := os.MkdirTemp("", "slidelang-pptx-kroki-*")
	if err != nil {
		return "", err
	}
	k.tempDir = dir
	return dir, nil
}

// cleanup borra el directorio temporal si se llegó a crear. No-op si nunca
// se usó (el caso común: un deck sin mermaid/plantuml, o con
// --diagram-backend=chromium por default).
func (k *pptxKrokiContext) cleanup() {
	if k.tempDir != "" {
		_ = os.RemoveAll(k.tempDir)
	}
}

// pptxAddDiagram renderiza un diagrama mermaid/plantuml vía Kroki (formato
// png) y lo embebe como imagen (issue #144). --format pptx nunca instancia
// Chromium — Generator ni siquiera tiene el campo — así que la única vía
// sin navegador es pedirle a un servidor Kroki el PNG directo:
// chromium.KrokiFetcher ya soporta el parámetro format explícito (ver su
// doc comment sobre por qué no bastaba con pedir siempre svg), pedirle
// "png" ya funciona sin cambios en core.
//
// Sin --diagram-backend kroki configurado, se omite con un warning que
// dice QUÉ hacer, no solo que se omitió: convertir un build offline (sin
// red) en uno que hace peticiones HTTP salientes sin que el operador lo
// haya pedido explícitamente sería peor que omitir el diagrama —
// warning-y-skip es la política, decidida en el propio issue #144.
func (g *Generator) pptxAddDiagram(s *pptx.Slide, diagramType, content, title string, cursorY int, opts GeneratorOptions, kroki *pptxKrokiContext) int {
	if opts.DiagramBackend != "kroki" {
		g.logger.Warn("PPTX: %s diagram skipped — --format pptx no usa Chromium, pasa --diagram-backend kroki (+ opcionalmente --kroki-server) para incluirlo", diagramType)
		return g.pptxAddText(s, fmt.Sprintf("[Diagram not rendered: %s]", diagramType), cursorY)
	}

	dir, err := kroki.dir()
	if err != nil {
		g.logger.Warn("PPTX: failed to create temp dir for kroki fetch: %v", err)
		return g.pptxAddText(s, fmt.Sprintf("[Diagram not rendered: %s]", diagramType), cursorY)
	}

	fetcher := chromium.NewKrokiFetcher(opts.KrokiServer, diagramType, "png", dir)
	relPath, err := fetcher.FetchAndSave(context.Background(), content, dir)
	if err != nil {
		g.logger.Warn("PPTX: kroki fetch failed for %s diagram: %v", diagramType, err)
		return g.pptxAddText(s, fmt.Sprintf("[Diagram failed to render: %s]", diagramType), cursorY)
	}

	data, err := os.ReadFile(filepath.Join(dir, relPath))
	if err != nil {
		g.logger.Warn("PPTX: failed to read rendered %s diagram: %v", diagramType, err)
		return g.pptxAddText(s, fmt.Sprintf("[Diagram not rendered: %s]", diagramType), cursorY)
	}

	// Mismo criterio de escalado que pptxAddChart: ancho fijo, alto
	// derivado del aspect ratio real del PNG para no deformarlo.
	//
	// cfgErr != nil es tratado como fallo del fetch, no ignorado (hallazgo
	// de code-review): un Kroki devolviendo 200 con un cuerpo que no es una
	// imagen (HTML de error, SVG cuando se pidió png) llegaba hasta acá con
	// drawWidth/drawHeight por defecto y esos bytes-no-imagen se le pasaban
	// tal cual a AddImageFromBytesWithSize -- pptxgo no valida ahí, solo
	// registra el error y lo propaga hasta que falle Save() al final del
	// build, abortando el deck completo por un solo diagrama.
	cfg, _, cfgErr := image.DecodeConfig(bytes.NewReader(data))
	if cfgErr != nil {
		g.logger.Warn("PPTX: kroki devolvió una respuesta que no es una imagen decodificable para el diagrama %s: %v", diagramType, cfgErr)
		return g.pptxAddText(s, fmt.Sprintf("[Diagram failed to render: %s]", diagramType), cursorY)
	}
	drawWidth := pptxChartWidthEMU
	drawHeight := pptxDefaultImageEMU
	if cfg.Width > 0 && cfg.Height > 0 {
		drawHeight = drawWidth * cfg.Height / cfg.Width
	}

	// Reservar la altura del caption ANTES de ajustar la imagen (hallazgo
	// de code-review): a diferencia de pptxAddChart (que nunca agrega un
	// caption -- el título del chart ya sale DENTRO del PNG), acá title se
	// agrega en un textbox aparte después de la imagen. Sin esta reserva,
	// pptxFitInSlide dejaba que una imagen alta consumiera todo el espacio
	// hasta el margen inferior, y el textbox del título quedaba fuera del
	// slide.
	captionReserve := 0
	if title != "" {
		captionReserve = pptxEstimateLines(title)*pptxLineHeightEMU + pptxParaGapEMU
	}
	drawWidth, drawHeight = pptxFitInSlide(drawWidth, drawHeight, cursorY+captionReserve)

	s.AddImageFromBytesWithSize(data, pptxMarginEMU, cursorY, drawWidth, drawHeight)

	newCursorY := cursorY + drawHeight + pptxParaGapEMU
	if title != "" {
		newCursorY = g.pptxAddText(s, title, newCursorY)
	}
	return newCursorY
}

// pptxChartIgnoringOptions devuelve una copia de e sin el bloque options:,
// más la lista ordenada de claves que se ignoraron. Si e no traía options,
// devuelve e tal cual y una lista vacía.
//
// Existe porque la política correcta para PPTX es DISTINTA de la que aplica
// renderer.SupportsNativeChartRendering, y la diferencia está en qué significa
// "rechazar" en cada formato:
//
//   - En HTML/PDF/DOCX, rechazar el camino nativo significa "usa Chromium",
//     que sí honra la config de Chart.js al pie de la letra. Ahí ser estricto
//     no cuesta nada más que arrancar un navegador, y evita dibujar algo
//     distinto de lo que el autor pidió.
//   - En PPTX no hay tal fallback: el formato entero se define por no usar
//     Chromium. Rechazar significa que el chart NO SALE. Y como en la práctica
//     casi todo chart real trae options:, ser igual de estricto acá dejaba la
//     cobertura de charts sin activarse nunca.
//
// El criterio original —"nunca dibujes algo distinto de lo que el autor
// pidió"— protege contra el error SILENCIOSO. Con un warning explícito que
// nombra las claves ignoradas, el error deja de ser silencioso, y un chart
// aproximado le gana a ningún chart. Lo que sigue descalificando es lo que el
// renderer nativo no puede dibujar en absoluto: un ChartType sin mapeo
// (radar, combo...) o el modo JSON.
//
// Solución puente mientras pptxgo no emite charts OOXML nativos
// (mmonterroca/pptxgo#25), que resolvería esto de raíz: PowerPoint redibujaría
// el chart y la mayoría de estas claves tendrían equivalente directo en su
// vocabulario, en vez de tener que replicarlas en un rasterizador.
func pptxChartIgnoringOptions(e *ast.ChartElement) (*ast.ChartElement, []string) {
	if len(e.Options) == 0 {
		return e, nil
	}

	dropped := make([]string, 0, len(e.Options))
	for k := range e.Options {
		dropped = append(dropped, k)
	}
	sort.Strings(dropped) // el orden de un map es aleatorio; el warning no debe serlo

	// Copia superficial: solo se limpia Options, y el resto de los campos
	// (Data/Labels/Series) se leen, nunca se mutan.
	clone := *e
	clone.Options = nil

	// Rescatar el título antes de tirar el bloque. En Chart.js el título de un
	// chart vive en options.plugins.title.text, no en una propiedad de primer
	// nivel, y así es como lo escriben los ejemplos de este repo — sin este
	// rescate, limpiar Options dejaría todos esos charts sin título en el
	// PNG. Es la única clave que se recupera: su semántica es inequívoca y el
	// renderer nativo ya tiene dónde ponerla (opt.Title.Text).
	if clone.Title == "" {
		if title := chartTitleFromOptions(e.Options); title != "" {
			clone.Title = title
		}
	}

	return &clone, dropped
}

// chartTitleFromOptions extrae options.plugins.title.text, respetando
// plugins.title.display: false (Chart.js no dibuja el título en ese caso, y
// replicarlo sería dibujar algo que el autor apagó).
func chartTitleFromOptions(options map[string]interface{}) string {
	plugins, ok := options["plugins"].(map[string]interface{})
	if !ok {
		return ""
	}
	title, ok := plugins["title"].(map[string]interface{})
	if !ok {
		return ""
	}
	if display, ok := title["display"].(bool); ok && !display {
		return ""
	}
	text, _ := title["text"].(string)
	return text
}

// pptxFitInSlide reduce (w, h) preservando el aspect ratio hasta que quepa en
// el espacio vertical que queda debajo de cursorY, dejando el margen inferior.
//
// El overflow no era exclusivo del caption duplicado: ChartDimensions respeta
// elem.Width/elem.Height, así que un chart declarado 800x800 daba drawHeight
// = 7.5in arrancando en 1.87in y la imagen misma quedaba casi entera fuera
// del canvas, sin caption de por medio. Clampear cubre los dos casos a la vez.
func pptxFitInSlide(w, h, cursorY int) (int, int) {
	available := pptxSlideHeightEMU - cursorY - pptxMarginEMU
	if available <= 0 || h <= available {
		return w, h
	}
	// Escalar por el mismo factor en ambos ejes: deformar el chart para que
	// quepa sería peor que mostrarlo más chico.
	return w * available / h, available
}

// pptxAddPoints agrega e como una lista de viñetas (subItems anidados vía
// Level, hasta el primer nivel de anidación — mismo alcance que el resto
// del MVP v0) y devuelve el cursorY actualizado.
func (g *Generator) pptxAddPoints(s *pptx.Slide, e *ast.PointsElement, cursorY int) int {
	totalLines := 0
	for _, item := range e.Items {
		totalLines += pptxEstimateLines(item.Content)
		for _, sub := range item.SubPoints {
			totalLines += pptxEstimateLines(sub.Content)
		}
	}
	if totalLines < 1 {
		totalLines = 1
	}
	height := totalLines * pptxLineHeightEMU

	tb := s.AddTextBox(pptxMarginEMU, cursorY, pptxContentWidthEMU, height)
	for _, item := range e.Items {
		g.pptxAddPointParagraph(tb, item, e.ListType, 0)
		for _, sub := range item.SubPoints {
			g.pptxAddPointParagraph(tb, sub, e.ListType, 1)
		}
	}

	return cursorY + height + pptxParaGapEMU
}

func (g *Generator) pptxAddPointParagraph(tb *pptx.TextBox, item ast.PointItem, listType string, level int) {
	para := tb.AddParagraph().Level(level)
	pptxApplyListIndent(para, level)
	if listType == "ordered" {
		para.NumberedBullet(pptx.NumArabicPeriod)
	} else {
		para.Bullet("•", "Arial")
	}
	// Level/Indent/Bullet son propiedades de párrafo (pPr), independientes de
	// qué run esté activo — seguro aplicarlas antes de escribir el texto vía
	// pptxApplyInline (que sí necesita ser el último paso: cada .Text() que
	// llama fija cuáles runs afecta el .Bold()/.Italic() siguiente).
	pptxApplyInline(para, item.Content)
}

// pptxTableUsesCellStructure reporta si e.Cells dice algo que la vista plana
// Headers/Rows no dice ya — duplicado de la misma lógica en
// data/converter.go (a su vez espejo de core/renderer/html.go's
// tableUsesCellStructure, tampoco exportado): ninguna vive en un paquete
// importable desde acá, y las tres deben coincidir exactamente en criterio
// para que HTML/PPTX no diverjan en qué tablas tratan como "con estructura
// real" (issue #20).
func pptxTableUsesCellStructure(elem *ast.TableElement) bool {
	for i, row := range elem.Cells {
		for _, cell := range row {
			if cell.ColSpan > 1 || cell.RowSpan > 1 {
				return true
			}
			if i == 0 {
				if !cell.IsHeader || cell.Scope != "col" {
					return true
				}
			} else if cell.IsHeader || cell.Scope != "" {
				return true
			}
		}
	}
	return false
}

// pptxClampSpan normaliza un ColSpan/RowSpan declarado al rango
// [1, ast.MaxCellSpan] — el mismo criterio que ast's clampSpan (no
// exportado, así que no se puede reusar directamente), necesario porque
// TableCell no tiene UnmarshalJSON y ast/decode.go nunca toca ColSpan: un
// AST externo (--filter, slidelang/internal/cli/build.go:132/141) puede
// traer un span disparatado que este generador lee directo de e.Cells sin
// pasar por FlattenCellsToRows.
func pptxClampSpan(span int) int {
	if span < 1 {
		return 1
	}
	if span > ast.MaxCellSpan {
		return ast.MaxCellSpan
	}
	return span
}

// pptxTableGridWidth calcula el ancho REAL de la grilla de cells —
// necesario porque AddTable(rows, cols, ...) exige una grilla rectangular,
// y con celdas combinadas cols != len(row): una fila con una celda
// colspan=2 seguida de una celda normal tiene len(row)==2 pero ocupa 3
// columnas de grilla.
//
// Delega en ast.FlattenCellsToRows en vez de sumar el ColSpan de la
// primera fila: (a) esa es la fuente de verdad de "qué tan ancha es la
// grilla" — ya usada por el propio HTML (vía la vista plana) y clampeada
// con ast.MaxCellSpan, así que un ColSpan disparatado en un AST externo no
// puede inflar cols sin límite; y (b) coincide con el ancho de la fila MÁS
// ANCHA, no solo la fila 0 — una tabla cuya fila 0 tiene colspan=2 pero
// cuya fila 1 declara 3 celdas propias necesita cols=3, no 2.
func pptxTableGridWidth(cells [][]ast.TableCell) int {
	headers, rows := ast.FlattenCellsToRows(cells)
	if len(headers) > 0 {
		return len(headers)
	}
	if len(rows) > 0 {
		return len(rows[0])
	}
	return 0
}

// pptxAddTableCells agrega e (con Cells reales, issue #20) como una tabla
// nativa OOXML con celdas combinadas vía Table.MergeCells, y devuelve el
// cursorY actualizado.
//
// Dos restricciones de MergeCells (ver su doc comment en pptxgo) que este
// código respeta a propósito:
//  1. Orden: MergeCells ANEXA los párrafos no-ancla al ancla en vez de
//     descartarlos, así que escribir texto en una celda ANTES de mergearla
//     apilaría texto viejo y nuevo. Por eso acá se escribe el texto DESPUÉS
//     de mergear, solo en la celda ancla (fila/columna de inicio del span).
//  2. Regiones inválidas (solapadas/invertidas/fuera de rango) quedan
//     registradas como error en la presentación y salen recién en Save() —
//     así que un ColSpan/RowSpan disparatado no aborta esta función, pero sí
//     puede hacer fallar el build más adelante. Aceptado: es la misma
//     superficie de validación que MergeCells ya le da a cualquier caller.
func (g *Generator) pptxAddTableCells(s *pptx.Slide, e *ast.TableElement, cursorY int) int {
	cols := pptxTableGridWidth(e.Cells)
	rows := len(e.Cells)
	if cols == 0 || rows == 0 {
		return cursorY
	}

	rowHeight := pptxLineHeightEMU * 2
	height := rows * rowHeight

	tbl := s.AddTable(rows, cols, pptxMarginEMU, cursorY, pptxContentWidthEMU, height)

	// occupied[r][c] marca las celdas ya cubiertas por un span anterior (de
	// esta fila o de una fila de arriba vía RowSpan), para saber en qué
	// columna de grilla real empieza la próxima celda declarada de cada
	// fila — Cells es [][]TableCell "denso" (una entrada por celda propia,
	// no por columna de grilla), así que el índice de grilla y el índice
	// del slice divergen apenas hay un span.
	occupied := make([][]bool, rows)
	for r := range occupied {
		occupied[r] = make([]bool, cols)
	}

	for r, row := range e.Cells {
		gridCol := 0
		for _, cell := range row {
			for gridCol < cols && occupied[r][gridCol] {
				gridCol++
			}
			if gridCol >= cols {
				break // fila más ancha que la grilla calculada: se descartan las celdas de más, criterio defensivo consistente con el resto del pipeline
			}

			colSpan := pptxClampSpan(cell.ColSpan)
			rowSpan := pptxClampSpan(cell.RowSpan)
			toRow := r + rowSpan - 1
			toCol := gridCol + colSpan - 1
			if toRow >= rows {
				toRow = rows - 1
			}
			if toCol >= cols {
				toCol = cols - 1
			}

			// Achicar la región para que nunca reclame una celda ya
			// cubierta por un merge anterior (un rowspan de una fila de
			// arriba, o un colspan de más atrás en esta misma fila): un
			// Cells solapado/malformado no debe llegar a MergeCells con
			// una región inválida, porque eso queda registrado como error
			// y hace fallar Save() para TODA la presentación — un
			// resultado mucho peor que un merge recortado.
			for cc := gridCol + 1; cc <= toCol; cc++ {
				if occupied[r][cc] {
					toCol = cc - 1
					break
				}
			}
			for rr := r + 1; rr <= toRow; rr++ {
				overlap := false
				for cc := gridCol; cc <= toCol; cc++ {
					if occupied[rr][cc] {
						overlap = true
						break
					}
				}
				if overlap {
					toRow = rr - 1
					break
				}
			}

			if toRow > r || toCol > gridCol {
				tbl.MergeCells(r, gridCol, toRow, toCol)
			}
			tblCell := tbl.Cell(r, gridCol).Text(cell.Content)
			if cell.IsHeader {
				tblCell.Bold()
			}

			for rr := r; rr <= toRow; rr++ {
				for cc := gridCol; cc <= toCol; cc++ {
					occupied[rr][cc] = true
				}
			}
			gridCol = toCol + 1
		}
	}

	return cursorY + height + pptxParaGapEMU
}

// pptxAddTable agrega e como una tabla nativa OOXML y devuelve el cursorY
// actualizado. Sin Caption/Label en v0 (issue #257: doclang/slidelang no
// pueden etiquetar tablas vía markdown/flex hoy — solo strict YAML — así
// que Caption suele venir vacío de todos modos para el caso común).
//
// Delega a pptxAddTableCells cuando e.Cells dice algo real (issue #20) que
// Headers/Rows no dice — la MISMA condición que decide el branch de
// template/base.go, para que HTML y PPTX rendericen la misma tabla como
// "con estructura" o no.
func (g *Generator) pptxAddTable(s *pptx.Slide, e *ast.TableElement, cursorY int) int {
	if pptxTableUsesCellStructure(e) {
		return g.pptxAddTableCells(s, e, cursorY)
	}

	rows := len(e.Rows) + 1 // +1 por la fila de headers
	cols := len(e.Headers)
	if cols == 0 || rows == 0 {
		return cursorY
	}

	rowHeight := pptxLineHeightEMU * 2
	height := rows * rowHeight

	tbl := s.AddTable(rows, cols, pptxMarginEMU, cursorY, pptxContentWidthEMU, height)
	for c, header := range e.Headers {
		tbl.Cell(0, c).Text(header).Bold()
	}
	for r, row := range e.Rows {
		for c, cell := range row {
			if c >= cols {
				break // fila con más columnas que headers: se descartan las de más, mismo criterio defensivo que el resto del pipeline con datos malformados
			}
			tbl.Cell(r+1, c).Text(cell)
		}
	}

	return cursorY + height + pptxParaGapEMU
}

// pptxAddImage agrega e como imagen embebida (PNG/JPEG/GIF) y devuelve el
// cursorY actualizado. e.Source es contenido del documento (no confiable):
// se confina a opts.AssetRoot con util.ResolveConfinedPath antes de leerlo
// — mismo mecanismo AL-4 que doclang/internal/generator/docx.go
// (docs/SECURITY_AUDIT_2026-07.md), sin el cual una ruta absoluta o con
// ".." podría embeber un archivo local arbitrario del disco del operador
// en el .pptx generado. Una URL remota (http/https) o una lectura fallida
// degrada a un placeholder de texto en vez de abortar el build completo.
func (g *Generator) pptxAddImage(s *pptx.Slide, e *ast.ImageElement, cursorY int, opts GeneratorOptions) int {
	if strings.HasPrefix(e.Source, "http://") || strings.HasPrefix(e.Source, "https://") {
		g.logger.Warn("PPTX: remote image source not supported in v0, skipped: %s", e.Source)
		return g.pptxAddText(s, fmt.Sprintf("[Image not embedded: %s]", e.Source), cursorY)
	}

	imagePath := e.Source
	if opts.AssetRoot != "" {
		confined, err := util.ResolveConfinedPath(opts.AssetRoot, imagePath)
		if err != nil {
			g.logger.Warn("PPTX: image source blocked (outside asset root): %s: %v", imagePath, err)
			return g.pptxAddText(s, fmt.Sprintf("[Image blocked: %s]", imagePath), cursorY)
		}
		imagePath = confined
	}

	data, err := os.ReadFile(imagePath)
	if err != nil {
		g.logger.Warn("PPTX: failed to read image %s: %v", imagePath, err)
		return g.pptxAddText(s, fmt.Sprintf("[Image not found: %s]", e.Source), cursorY)
	}

	width := pptxContentWidthEMU / 2
	height := pptxDefaultImageEMU
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && cfg.Width > 0 && cfg.Height > 0 {
		height = width * cfg.Height / cfg.Width
	}

	s.AddImageFromBytesWithSize(data, pptxMarginEMU, cursorY, width, height)

	newCursorY := cursorY + height + pptxParaGapEMU
	if e.Caption != "" {
		newCursorY = g.pptxAddText(s, e.Caption, newCursorY)
	}

	return newCursorY
}
