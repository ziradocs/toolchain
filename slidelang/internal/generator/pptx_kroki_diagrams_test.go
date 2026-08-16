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
