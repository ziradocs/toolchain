// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// validPNGBytes produce un PNG mínimo pero real (1x1 del color dado) — un
// servidor Kroki de mentira necesita devolver bytes decodificables, no
// solo cualquier []byte, porque pptxAddDiagram los pasa por
// image.DecodeConfig para calcular el aspect ratio antes de embeberlos.
func validPNGBytes(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, c)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return buf.Bytes()
}

// TestPPTX_MermaidPlantUMLViaKroki_RasterizeWhenBackendConfigured es el
// caso positivo (issue #144): con --diagram-backend kroki, mermaid y
// plantuml deben salir como imagen embebida (ppt/media/*), no como el
// placeholder de texto — mismo molde que
// TestPPTX_ChartElementIsRasterizedNatively.
func TestPPTX_MermaidPlantUMLViaKroki_RasterizeWhenBackendConfigured(t *testing.T) {
	// Dos imágenes DISTINTAS: pptxgo dedupea ppt/media/* por hash de
	// contenido (mediaBasename), así que dos diagramas con bytes idénticos
	// colapsarían a una sola entrada de media -- eso probaría el dedup de
	// pptxgo, no que los dos diagramas se rasterizaron.
	mermaidPNG := validPNGBytes(t, color.RGBA{0, 255, 0, 255})
	plantumlPNG := validPNGBytes(t, color.RGBA{0, 0, 255, 255})

	var gotPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		w.Header().Set("Content-Type", "image/png")
		if strings.HasPrefix(r.URL.Path, "/mermaid/") {
			_, _ = w.Write(mermaidPNG)
		} else {
			_, _ = w.Write(plantumlPNG)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FilePath = "diagrams.slidelang"
	block := ast.NewContentBlock(pos(), "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements,
		ast.NewMermaidElement(pos(), "flowchart", "graph TD; A-->B"),
		ast.NewPlantUMLElement(pos(), "sequence", "A->B: hola"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	opts := GeneratorOptions{AssetRoot: dir, DiagramBackend: "kroki", KrokiServer: server.URL}
	if err := g.generatePPTX(doc, dir, opts); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	outputPath := filepath.Join(dir, "diagrams.pptx")
	names := zipEntryNames(t, outputPath)

	mediaCount := 0
	for _, n := range names {
		if strings.HasPrefix(n, "ppt/media/") {
			mediaCount++
		}
	}
	if mediaCount != 2 {
		t.Fatalf("want 2 ppt/media/* entries (mermaid + plantuml, distinct bytes so pptxgo's hash-dedup can't collapse them), got %d. Entries: %v", mediaCount, names)
	}

	xml := zipEntryContent(t, outputPath, "ppt/slides/slide1.xml")
	if strings.Contains(xml, "graph TD") || strings.Contains(xml, "A->B: hola") {
		t.Error("diagram source leaked into the slide as literal text instead of being rasterized")
	}
	if strings.Contains(xml, "Diagram not rendered") || strings.Contains(xml, "Diagram failed to render") {
		t.Errorf("placeholder text present even though the kroki backend was configured: %s", xml)
	}
	if picCount := strings.Count(xml, "<p:pic>"); picCount != 2 {
		t.Errorf("slide1.xml has %d <p:pic> shapes, want 2", picCount)
	}

	if len(gotPaths) != 2 {
		t.Fatalf("expected 2 requests to the kroki server, got %d: %v", len(gotPaths), gotPaths)
	}
	wantPaths := map[string]bool{"/mermaid/png": false, "/plantuml/png": false}
	for _, p := range gotPaths {
		if _, ok := wantPaths[p]; ok {
			wantPaths[p] = true
		}
	}
	for p, got := range wantPaths {
		if !got {
			t.Errorf("expected a request to %s, got paths %v", p, gotPaths)
		}
	}
}

// TestPPTX_MermaidWithoutKrokiBackend_WarnsAndSkips cubre la decisión que
// el issue #144 dejaba abierta: sin --diagram-backend kroki, el diagrama se
// omite con un placeholder — nunca sale a la red en silencio (--format
// pptx se define por no necesitar red, no solo por no necesitar Chromium).
func TestPPTX_MermaidWithoutKrokiBackend_WarnsAndSkips(t *testing.T) {
	spy := &warnSpyLogger{}
	dir := t.TempDir()
	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FilePath = "no-backend.slidelang"
	block := ast.NewContentBlock(pos(), "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements, ast.NewMermaidElement(pos(), "flowchart", "graph TD; A-->B"))
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(spy)
	if err := g.generatePPTX(doc, dir, GeneratorOptions{AssetRoot: dir}); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	xml := zipEntryContent(t, filepath.Join(dir, "no-backend.pptx"), "ppt/slides/slide1.xml")
	if strings.Contains(xml, "graph TD") {
		t.Error("mermaid source leaked into the slide as literal text")
	}
	if !strings.Contains(xml, "Diagram not rendered: mermaid") {
		t.Errorf("expected the placeholder text, slide1.xml: %s", xml)
	}
	if !spy.sawSubstring("--diagram-backend kroki") {
		t.Errorf("warning does not tell the operator what to do, got: %v", spy.warnings)
	}
}

// TestPPTX_KrokiServerError_DegradesToPlaceholder confirma que un servidor
// Kroki caído/erróneo degrada al placeholder de texto en vez de tumbar el
// build completo — mismo criterio que ya aplica pptxAddChart cuando el
// render nativo falla.
func TestPPTX_KrokiServerError_DegradesToPlaceholder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dir := t.TempDir()
	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FilePath = "kroki-down.slidelang"
	block := ast.NewContentBlock(pos(), "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements,
		ast.NewMermaidElement(pos(), "flowchart", "graph TD; A-->B"),
		ast.NewTextElement(pos(), "text-marker-after"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	opts := GeneratorOptions{AssetRoot: dir, DiagramBackend: "kroki", KrokiServer: server.URL}
	if err := g.generatePPTX(doc, dir, opts); err != nil {
		t.Fatalf("generatePPTX() error = %v, want a graceful degrade instead of a build failure", err)
	}

	xml := zipEntryContent(t, filepath.Join(dir, "kroki-down.pptx"), "ppt/slides/slide1.xml")
	if !strings.Contains(xml, "Diagram failed to render: mermaid") {
		t.Errorf("expected the failure placeholder, slide1.xml: %s", xml)
	}
	// Omitir un diagrama no puede truncar el resto del slide.
	if !strings.Contains(xml, "text-marker-after") {
		t.Error("a failed diagram fetch dropped the elements after it")
	}
}

// tallPNGBytes produce un PNG real de w x h de un solo color -- a
// diferencia de validPNGBytes (1x1), necesitamos una imagen con un aspect
// ratio genuinamente alto para forzar el caso que pptxFitInSlide clampea
// hasta el margen inferior.
func tallPNGBytes(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return buf.Bytes()
}

// TestPPTX_KrokiNonImageResponse_DegradesToPlaceholder cubre un hallazgo de
// code-review distinto del servidor caído (ese ya lo cubre
// TestPPTX_KrokiServerError_DegradesToPlaceholder): un Kroki que responde
// 200 OK con Content-Type image/png pero un cuerpo que NO es una imagen
// decodificable (una página de error HTML, un SVG cuando se pidió png).
// Antes, image.DecodeConfig fallaba pero el error se ignoraba y esos bytes
// no-imagen se le pasaban tal cual a AddImageFromBytesWithSize -- pptxgo no
// valida ahí, así que el fallo solo aparecía hasta Save() al final del
// build, abortando el deck ENTERO por un solo diagrama en vez de degradar
// nada más ese elemento.
func TestPPTX_KrokiNonImageResponse_DegradesToPlaceholder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>not a png, an error page kroki still answered 200 for</body></html>"))
	}))
	defer server.Close()

	dir := t.TempDir()
	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FilePath = "kroki-non-image.slidelang"
	block := ast.NewContentBlock(pos(), "content")
	block.Title = "Coverage"
	block.Elements = append(block.Elements,
		ast.NewMermaidElement(pos(), "flowchart", "graph TD; A-->B"),
		ast.NewTextElement(pos(), "text-marker-after"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	opts := GeneratorOptions{AssetRoot: dir, DiagramBackend: "kroki", KrokiServer: server.URL}
	if err := g.generatePPTX(doc, dir, opts); err != nil {
		t.Fatalf("generatePPTX() error = %v, want a graceful degrade instead of a build failure", err)
	}

	outputPath := filepath.Join(dir, "kroki-non-image.pptx")
	xml := zipEntryContent(t, outputPath, "ppt/slides/slide1.xml")
	if !strings.Contains(xml, "Diagram failed to render: mermaid") {
		t.Errorf("expected the failure placeholder, slide1.xml: %s", xml)
	}
	if !strings.Contains(xml, "text-marker-after") {
		t.Error("a non-image kroki response dropped the elements after it")
	}
	for _, n := range zipEntryNames(t, outputPath) {
		if strings.HasPrefix(n, "ppt/media/") {
			t.Errorf("non-decodable bytes were embedded as media: %s", n)
		}
	}
}

// TestPPTX_DiagramCaption_StaysWithinSlideBounds cubre un hallazgo de
// code-review: pptxFitInSlide clampeaba la imagen hasta el margen inferior
// sin reservar espacio para el caption que pptxAddDiagram agrega DESPUÉS
// (a diferencia de pptxAddChart, que nunca agrega caption -- el título del
// chart ya sale dentro del PNG) -- el textbox del título quedaba fuera del
// slide. Un diagrama con aspect ratio muy alto (100x1000) es el peor caso:
// sin la reserva, la imagen sola ya llega hasta el margen.
func TestPPTX_DiagramCaption_StaysWithinSlideBounds(t *testing.T) {
	tallPNG := tallPNGBytes(t, 100, 1000, color.RGBA{0, 255, 0, 255})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tallPNG)
	}))
	defer server.Close()

	dir := t.TempDir()
	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FilePath = "diagram-caption.slidelang"
	block := ast.NewContentBlock(pos(), "content")
	block.Title = "Coverage"
	mermaid := ast.NewMermaidElement(pos(), "flowchart", "graph TD; A-->B")
	mermaid.Title = "This caption must stay on-slide"
	block.Elements = append(block.Elements, mermaid)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	opts := GeneratorOptions{AssetRoot: dir, DiagramBackend: "kroki", KrokiServer: server.URL}
	if err := g.generatePPTX(doc, dir, opts); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	xml := zipEntryContent(t, filepath.Join(dir, "diagram-caption.pptx"), "ppt/slides/slide1.xml")
	if !strings.Contains(xml, "This caption must stay on-slide") {
		t.Fatalf("expected the caption text in the slide, got: %s", xml)
	}

	// Cada shape con <a:xfrm> (imagen y textbox del caption) debe tener su
	// borde inferior (y + cy) dentro del área de contenido -- no solo su
	// altura (cy) por separado, que es lo que ya cubre
	// TestPPTX_ChartIsClampedToSlide y no atrapa este caso.
	maxBottom := pptxSlideHeightEMU - pptxMarginEMU
	re := regexp.MustCompile(`<a:off x="\d+" y="(\d+)"></a:off>\s*<a:ext cx="\d+" cy="(\d+)"`)
	matches := re.FindAllStringSubmatch(xml, -1)
	if len(matches) == 0 {
		t.Fatal("no <a:xfrm> shapes found to check")
	}
	for _, m := range matches {
		y, _ := strconv.Atoi(m[1])
		cy, _ := strconv.Atoi(m[2])
		if bottom := y + cy; bottom > maxBottom {
			t.Errorf("shape bottom edge = %d EMU, want <= %d EMU (off-slide, y=%d cy=%d)", bottom, maxBottom, y, cy)
		}
	}
}

// warnSpyLogger es un util.Logger mínimo (vía util.NoopLogger embebido)
// que solo registra Warn(), para assertar el CONTENIDO del mensaje, no
// solo que algo se logueó — mismo patrón que core/renderer's spyLogger.
type warnSpyLogger struct {
	util.NoopLogger
	warnings []string
}

func (s *warnSpyLogger) Warn(format string, args ...interface{}) {
	s.warnings = append(s.warnings, fmt.Sprintf(format, args...))
}

func (s *warnSpyLogger) sawSubstring(substr string) bool {
	for _, w := range s.warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}
