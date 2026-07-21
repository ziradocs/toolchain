// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package generator

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"  // registra el decoder GIF para image.DecodeConfig
	_ "image/jpeg" // registra el decoder JPEG para image.DecodeConfig
	_ "image/png"  // registra el decoder PNG para image.DecodeConfig
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mmonterroca/pptxgo/pptx"
	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/util"
)

// pptx.go implementa --format pptx (issue #129), AST → .pptx vía pptxgo
// (github.com/mmonterroca/pptxgo, MIT, sin dependencias) — pinneada por
// pseudo-versión de commit en go.mod, NO `replace ../../pptxgo`: un replace
// a un directorio fuera de este checkout rompería CI, que solo clona este
// repo (no el repo hermano de pptxgo).
//
// Alcance MVP v0 (deliberado): TextElement (párrafos, bold/italic/code
// inline por segmentos), PointsElement (viñetas), TableElement,
// ImageElement — mapeados vía Slide.AddTextBox/AddTable/AddImageFromBytes
// en freeform, apilados verticalmente por altura estimada (pptxgo no mide
// texto; el resultado es editable en PowerPoint si el estimado no calza
// exacto, no es un layout final de precisión). Título/subtítulo sí usan
// placeholders de layout (Slide.Title, PlaceholderSubTitle) — evita
// inventar coordenadas para esos dos, a diferencia del cuerpo.
//
// Diferido a v1: CodeElement/QuoteElement/ChecklistElement/
// SpecialBlockElement/CodeGroupElement/GridElement (aproximables como
// textboxes estilizados) y Mermaid/Chart/Map/Math/PlantUML (requieren
// rasterizar a PNG con el pipeline Chromium existente — la infraestructura
// ya existe, igual que en doclang/internal/generator/docx.go, pero
// suma trabajo y una dependencia de Chromium que el MVP evita). Un elemento
// no soportado se omite con un warning explícito (ver pptxAddElement), no
// en silencio.
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
)

func (g *Generator) generatePPTX(astNode *ast.AST, outputDir string, opts GeneratorOptions) error {
	g.logger.Info("PPTX", "Building PPTX presentation...")

	p := pptx.New()

	for i := range astNode.ContentBlocks {
		g.pptxAddSlide(p, &astNode.ContentBlocks[i], opts)
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
func (g *Generator) pptxAddSlide(p *pptx.Presentation, block *ast.ContentBlock, opts GeneratorOptions) {
	isTitleBlock := block.BlockType == "title"

	layout := pptx.LayoutTitleAndContent
	if isTitleBlock {
		layout = pptx.LayoutTitleSlide
	}
	s := p.AddSlide(pptx.WithLayout(layout))

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
		cursorY = g.pptxAddElement(s, block.Elements[i], cursorY, opts)
	}
}

// pptxAddElement despacha por tipo de ast.Element y devuelve el cursorY
// actualizado para el próximo elemento. Un tipo no cubierto por el alcance
// MVP v0 (ver el comentario del paquete) se omite con un warning explícito
// — nunca en silencio, para que "faltan diagramas en este deck" sea visible
// en el log del build, no un misterio.
func (g *Generator) pptxAddElement(s *pptx.Slide, elem ast.Element, cursorY int, opts GeneratorOptions) int {
	switch e := elem.(type) {
	case *ast.TextElement:
		return g.pptxAddText(s, e.Content, cursorY)
	case *ast.PointsElement:
		return g.pptxAddPoints(s, e, cursorY)
	case *ast.TableElement:
		return g.pptxAddTable(s, e, cursorY)
	case *ast.ImageElement:
		return g.pptxAddImage(s, e, cursorY, opts)
	default:
		g.logger.Warn("PPTX: element type %T not supported in v0, skipped (issue #129 tracks v1 coverage)", elem)
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
}

var (
	pptxCodeRe   = regexp.MustCompile("`([^`]+)`")
	pptxBoldRe   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	pptxItalicRe = regexp.MustCompile(`\*([^*]+)\*`)
)

// pptxSplitInline segmenta content en texto plano + code/bold/italic —
// mismo orden de prioridad que docx.go (code antes que bold, para no
// interpretar "**" dentro de un `código`). Los links [text](url) quedan
// fuera del alcance v0 (issue de seguimiento): un link se muestra como
// texto plano con su sintaxis markdown literal en vez de resolverse.
func pptxSplitInline(content string) []pptxInlineSegment {
	type match struct {
		start, end int
		inner      string
		bold       bool
		italic     bool
		code       bool
	}

	var segments []pptxInlineSegment
	remaining := content
	pos := 0

	for pos < len(remaining) {
		var best *match
		for _, re := range []struct {
			re                 *regexp.Regexp
			bold, italic, code bool
		}{
			{pptxCodeRe, false, false, true},
			{pptxBoldRe, true, false, false},
			{pptxItalicRe, false, true, false},
		} {
			loc := re.re.FindStringSubmatchIndex(remaining[pos:])
			if loc == nil {
				continue
			}
			if best == nil || loc[0] < best.start-pos {
				best = &match{
					start: pos + loc[0], end: pos + loc[1],
					inner: remaining[pos+loc[2] : pos+loc[3]],
					bold:  re.bold, italic: re.italic, code: re.code,
				}
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
	for _, seg := range pptxSplitInline(content) {
		para.Text(seg.text)
		if seg.bold {
			para.Bold()
		}
		if seg.italic {
			para.Italic()
		}
		if seg.code {
			para.Font("Courier New")
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
	para := tb.AddParagraph().Level(level).Indent(18, -18)
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

// pptxAddTable agrega e como una tabla nativa OOXML y devuelve el cursorY
// actualizado. Sin Caption/Label en v0 (issue #257: doclang/slidelang no
// pueden etiquetar tablas vía markdown/flex hoy — solo strict YAML — así
// que Caption suele venir vacío de todos modos para el caso común).
func (g *Generator) pptxAddTable(s *pptx.Slide, e *ast.TableElement, cursorY int) int {
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
