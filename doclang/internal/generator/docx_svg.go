// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
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
// de bloque en una sección, anidada en un grid o un callout, o inline en un
// párrafo, un ítem o una celda. Una versión anterior miraba solo las imágenes
// de bloque del nivel de sección, así que un documento cuyo único SVG estaba
// anidado o inline no arrancaba Chromium y el SVG quedaba como marcador. El
// AST se recorre serializado: así no depende de conocer cada tipo que anida
// elementos ni cada campo con markdown inline.
func HasSVGImages(doc *ast.AST) bool {
	if doc == nil {
		return false
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // "<a b.svg>" tiene que llegar tal cual
	if err := enc.Encode(doc); err != nil {
		return false
	}
	text := buf.String()
	for _, m := range svgSourceFieldPattern.FindAllStringSubmatch(text, -1) {
		var src string
		if json.Unmarshal([]byte(`"`+m[1]+`"`), &src) == nil && isSVGImageSource(src) {
			return true
		}
	}
	for _, m := range svgInlineImagePattern.FindAllStringSubmatch(text, -1) {
		if src, ok := docxLocalImageSource(m[1]); ok && isSVGImageSource(src) {
			return true
		}
	}
	return false
}

var (
	// svgSourceFieldPattern es el campo "source" de un ImageElement en el
	// AST serializado.
	svgSourceFieldPattern = regexp.MustCompile(`"source":"((?:[^"\\]|\\.)*)"`)
	// svgInlineImagePattern es el destino de un ![alt](src) inline dentro de
	// cualquier texto del AST serializado (hasta el primer ")" o comilla).
	svgInlineImagePattern = regexp.MustCompile(`!\[[^\]]*\]\(\s*([^)"]+)\)`)
)

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
