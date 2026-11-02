// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
)

// svgImageRasterizer es RenderSVGImageToPNG de core (PNG a 2 px por px CSS).
// Se pide por interfaz y no por nombre por lo mismo que fittedMathRenderer:
// este módulo compila también contra el core publicado, que puede no
// tenerlo; con ese core la imagen SVG queda como marcador y aviso.
type svgImageRasterizer interface {
	RenderSVGImageToPNG(ctx context.Context, svg []byte, widthPx, heightPx float64) ([]byte, error)
}

// svgDefaultWidthPx/svgDefaultHeightPx son el tamaño que un navegador le da
// a un <svg> sin width, height ni viewBox (el de un elemento reemplazado sin
// tamaño intrínseco, CSS 2 §10.3.2).
const (
	svgDefaultWidthPx  = 300.0
	svgDefaultHeightPx = 150.0
)

// errSVGNeedsChromium dice por qué una imagen SVG no se insertó cuando no hay
// con qué rasterizarla.
var errSVGNeedsChromium = errors.New("SVG images are rasterized with Chromium, which is not available. Install Chrome/Chromium/Edge, pass --chromium-path, or use --install-chromium")

// isSVGImageSource dice si la fuente de una imagen local es un SVG, por la
// extensión (sin query ni fragmento). Es lo que usan needsChromiumRendering y
// pipeline.ChromiumRequirements para decidir si arrancar Chromium antes de
// leer ningún archivo.
func isSVGImageSource(source string) bool {
	u, err := url.Parse(source)
	if err != nil || u.Scheme != "" {
		return false
	}
	return strings.EqualFold(path.Ext(u.Path), ".svg")
}

// HasSVGImages dice si doc tiene alguna imagen local SVG en cualquier lugar:
// de bloque en una sección, anidada en un grid, un callout o una columna, o
// inline en un párrafo, un ítem, una celda, una cita o un quiz. Una versión
// anterior miraba solo las imágenes de bloque del nivel de sección; otra
// buscaba ".svg" con regex en el AST serializado, y contaba también un
// ejemplo en un bloque de código, un code span, una imagen escapada, el
// \text{} de una ecuación o un <<video src="a.svg">>: el export DOCX del MCP
// fallaba pidiendo Chromium para un tutorial sin ningún SVG de verdad.
//
// Ahora recorre el AST con tipos, como la regla LINK001 del linter: la
// fuente de cada ImageElement, y las imágenes inline de los campos de texto
// según renderer.FindInlineLinks, que corre las mismas pasadas del renderer
// (salta code spans y escapes). Código, math, mermaid, plantuml, media,
// charts y mapas no se miran. Una imagen remota no cuenta.
func HasSVGImages(doc *ast.AST) bool {
	if doc == nil {
		return false
	}
	var variables map[string]interface{}
	if doc.FrontMatter != nil {
		variables = doc.FrontMatter.BuildVariables()
	}
	// FindInlineLinks trabaja por línea; solo se le pasan las líneas que
	// pueden tener una imagen SVG ("![" y ".svg" en cualquier caja), así un
	// documento grande sin SVG se revisa casi sin costo.
	inline := func(texts ...string) bool {
		for _, text := range texts {
			if !strings.Contains(text, "![") {
				continue
			}
			// Una {{variable}} puede ser la que trae el ".svg".
			if strings.Contains(text, "{{") {
				text = renderer.ProcessVariables(text, variables)
			}
			if !containsFoldSVG(text) {
				continue
			}
			for _, line := range strings.Split(text, "\n") {
				if !strings.Contains(line, "![") || !containsFoldSVG(line) {
					continue
				}
				for _, link := range renderer.FindInlineLinks(line) {
					if !link.Image || link.Dropped {
						continue
					}
					if src, ok := docxLocalImageSource(link.Destination); ok && isSVGImageSource(src) {
						return true
					}
				}
			}
		}
		return false
	}
	var points func([]ast.PointItem) bool
	points = func(items []ast.PointItem) bool {
		for _, it := range items {
			if inline(it.Content) || points(it.SubPoints) {
				return true
			}
		}
		return false
	}
	var checklist func([]ast.ChecklistItem) bool
	checklist = func(items []ast.ChecklistItem) bool {
		for _, it := range items {
			if inline(it.Content) || checklist(it.SubItems) {
				return true
			}
		}
		return false
	}
	var visit func(ast.Element) bool
	visit = func(el ast.Element) bool {
		switch e := el.(type) {
		case *ast.ImageElement:
			src := e.Source
			if strings.Contains(src, "{{") {
				src = renderer.ProcessVariables(src, variables)
			}
			return isSVGImageSource(src) || inline(e.Caption)
		case *ast.TextElement:
			if e.IsRawHTML {
				return inline(e.HeadingSource)
			}
			return inline(e.Content)
		case *ast.HeadingElement:
			return inline(e.Text)
		case *ast.PointsElement:
			return points(e.Items)
		case *ast.ChecklistElement:
			return checklist(e.Items)
		case *ast.QuoteElement:
			return inline(e.Content)
		case *ast.TableElement:
			if inline(e.Headers...) {
				return true
			}
			for _, row := range e.Rows {
				if inline(row...) {
					return true
				}
			}
			for _, row := range e.TableRows {
				for _, cell := range row.Cells {
					if inline(cell.Content) {
						return true
					}
				}
			}
			return false
		case *ast.SpecialBlockElement:
			if inline(e.Title, e.Content) {
				return true
			}
			for _, nested := range e.Elements {
				if visit(nested) {
					return true
				}
			}
			return false
		case *ast.GridElement:
			if inline(e.Content) {
				return true
			}
			for _, col := range e.Columns {
				if inline(col.Content) {
					return true
				}
				for _, nested := range col.Elements {
					if visit(nested) {
						return true
					}
				}
			}
			return false
		case *ast.MetricElement:
			return inline(e.Label, e.Value, e.Delta, e.Caption)
		case *ast.QuizElement:
			return inline(e.Question, e.Explanation) || inline(e.Options...)
		case *ast.PollElement:
			return inline(e.Question) || inline(e.Options...)
		}
		// Code, CodeGroup, Math, Mermaid, PlantUML, Media, Chart, Map: su
		// contenido no tiene imágenes del documento.
		return false
	}
	for _, block := range doc.ContentBlocks {
		if inline(block.Title, block.Heading, block.Subtitle) {
			return true
		}
		for _, el := range block.Elements {
			if visit(el) {
				return true
			}
		}
	}
	return false
}

// containsFoldSVG dice si s contiene ".svg" sin distinguir mayúsculas.
func containsFoldSVG(s string) bool {
	for i := strings.IndexByte(s, '.'); i >= 0 && i+4 <= len(s); {
		if strings.EqualFold(s[i:i+4], ".svg") {
			return true
		}
		j := strings.IndexByte(s[i+1:], '.')
		if j < 0 {
			return false
		}
		i += 1 + j
	}
	return false
}

// looksLikeSVG reconoce un SVG por su contenido: el primer elemento XML es
// <svg>. Cubre un .svg con otra extensión y deja fuera un .svg que en
// realidad es otra cosa.
func looksLikeSVG(data []byte) bool {
	_, ok := svgRootElement(data)
	return ok
}

// svgRootElement devuelve el elemento raíz si es <svg>.
func svgRootElement(data []byte) (xml.StartElement, bool) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	for {
		tok, err := d.Token()
		if err != nil {
			return xml.StartElement{}, false
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se, strings.EqualFold(se.Name.Local, "svg")
		}
	}
}

// svgNaturalSizePx es el tamaño, en px CSS, al que un navegador muestra el
// SVG sin otro tamaño impuesto: width y height del <svg> (en px, pt, pc, mm,
// cm, in o sin unidad); si falta uno, sale de la proporción del viewBox; si
// faltan los dos, el viewBox; sin nada de eso, 300x150. Un porcentaje o
// "auto" cuenta como ausente.
func svgNaturalSizePx(data []byte) (float64, float64, error) {
	root, ok := svgRootElement(data)
	if !ok {
		return 0, 0, fmt.Errorf("not an SVG document")
	}
	var w, h, vbW, vbH float64
	for _, a := range root.Attr {
		switch a.Name.Local {
		case "width":
			w = svgLengthPx(a.Value)
		case "height":
			h = svgLengthPx(a.Value)
		case "viewBox":
			vbW, vbH = svgViewBoxSize(a.Value)
		}
	}
	hasVB := vbW > 0 && vbH > 0
	switch {
	case w > 0 && h > 0:
	case w > 0 && hasVB:
		h = w * vbH / vbW
	case h > 0 && hasVB:
		w = h * vbW / vbH
	case hasVB:
		w, h = vbW, vbH
	case w > 0:
		h = svgDefaultHeightPx
	case h > 0:
		w = svgDefaultWidthPx
	default:
		w, h = svgDefaultWidthPx, svgDefaultHeightPx
	}
	if !svgFinitePositive(w) || !svgFinitePositive(h) {
		w, h = svgDefaultWidthPx, svgDefaultHeightPx
	}
	return w, h, nil
}

// svgLengthPx convierte una longitud SVG a px CSS (96 por pulgada). 0 si no
// es una longitud absoluta.
func svgLengthPx(v string) float64 {
	v = strings.TrimSpace(strings.ToLower(v))
	units := []struct {
		suffix string
		px     float64
	}{
		{"px", 1}, {"pt", 96.0 / 72}, {"pc", 16}, {"mm", 96 / 25.4}, {"cm", 96 / 2.54}, {"in", 96},
	}
	factor := 1.0
	for _, u := range units {
		if strings.HasSuffix(v, u.suffix) {
			v = strings.TrimSpace(strings.TrimSuffix(v, u.suffix))
			factor = u.px
			break
		}
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || !svgFinitePositive(n) || !svgFinitePositive(n*factor) {
		return 0
	}
	return n * factor
}

// svgViewBoxSize lee el ancho y alto de un viewBox ("min-x min-y w h",
// separados por espacios o comas).
func svgViewBoxSize(v string) (float64, float64) {
	f := strings.FieldsFunc(v, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' || r == '\n' || r == '\r' })
	if len(f) != 4 {
		return 0, 0
	}
	w, err1 := strconv.ParseFloat(f[2], 64)
	h, err2 := strconv.ParseFloat(f[3], 64)
	if err1 != nil || err2 != nil || !svgFinitePositive(w) || !svgFinitePositive(h) {
		return 0, 0
	}
	return w, h
}

// svgFinitePositive descarta 0, negativos, NaN e infinitos: ParseFloat
// acepta "NaN", "Inf" y "1e999", y un tamaño así llegaba a la captura de
// Chromium y a los EMU del DOCX.
func svgFinitePositive(f float64) bool {
	return f > 0 && !math.IsInf(f, 0) && !math.IsNaN(f)
}

// rasterizeSVGImage convierte una imagen SVG local en un PNG del tamaño al
// que va en la página (el natural, reducido si no cabe) y devuelve el PNG
// escrito en el directorio temporal y ese tamaño. Antes un SVG llegaba a
// image.DecodeConfig, que no lo conoce, y la imagen se perdía del DOCX con
// "unknown format" aunque el PDF la mostrara.
func (g *DOCXGenerator) rasterizeSVGImage(svgPath string, hasCaption bool) (string, float64, float64, error) {
	data, err := os.ReadFile(svgPath)
	if err != nil {
		return "", 0, 0, err
	}
	wPx, hPx, err := svgNaturalSizePx(data)
	if err != nil {
		return "", 0, 0, err
	}
	w, h := g.fitFigure(wPx/cssPxPerInch, hPx/cssPxPerInch, hasCaption)
	w, h = g.enforceMinSide(w, h, hasCaption)

	if g.chromiumRenderer == nil {
		return "", 0, 0, errSVGNeedsChromium
	}
	rasterizer, ok := any(g.chromiumRenderer).(svgImageRasterizer)
	if !ok {
		return "", 0, 0, fmt.Errorf("this build has no SVG rasterizer (it needs a newer go.ziradocs.com/core/v2)")
	}
	pngBytes, err := rasterizer.RenderSVGImageToPNG(g.runCtx(), data, w*cssPxPerInch, h*cssPxPerInch)
	if err != nil {
		return "", 0, 0, err
	}
	sum := sha256.Sum256(append(data, fmt.Sprintf("|%.4fx%.4f", w, h)...))
	pngPath := filepath.Join(g.tempDir, "svg_"+hex.EncodeToString(sum[:8])+".png")
	if err := os.WriteFile(pngPath, pngBytes, 0o644); err != nil {
		return "", 0, 0, err
	}
	return pngPath, w, h, nil
}

// fileLooksLikeSVG lee el principio del archivo y aplica looksLikeSVG: un SVG
// guardado sin la extensión .svg. Un error de lectura cuenta como "no".
func fileLooksLikeSVG(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 4096)
	n, _ := io.ReadFull(f, head)
	return looksLikeSVG(head[:n])
}
