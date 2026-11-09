// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"bytes"
	"html"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

// docPrPattern es el <wp:docPr> de cada imagen del DOCX, con su descr (el
// texto alternativo) si lo lleva.
var docPrPattern = regexp.MustCompile(`<wp:docPr\b[^>]*>`)

// docPrDescr es el atributo descr de un <wp:docPr>.
var docPrDescr = regexp.MustCompile(`\sdescr="([^"]*)"`)

// docxDescrs devuelve el descr de cada imagen de las partes del DOCX cuyo
// nombre empieza con prefix ("word/document.xml", "word/header"), en orden.
func docxDescrs(t *testing.T, docxPath, prefix string) []string {
	t.Helper()
	r, err := zip.OpenReader(docxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	var out []string
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, tag := range docPrPattern.FindAllString(string(data), -1) {
			descr := ""
			if m := docPrDescr.FindStringSubmatch(tag); m != nil {
				descr = html.UnescapeString(m[1])
			}
			out = append(out, descr)
		}
	}
	return out
}

// pngServer sirve un PNG de w×h a cualquier pedido y cuenta los pedidos.
func pngServer(t *testing.T, w, h int) (*httptest.Server, *atomic.Int32, *atomic.Value) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{0x80, 0x80, 0x80, 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	var lastPath atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		lastPath.Store(r.URL.Path)
		rw.Header().Set("Content-Type", "image/png")
		_, _ = rw.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv, &hits, &lastPath
}

func barChart(line int, title string) *ast.ChartElement {
	c := ast.NewChartElement(diagnostics.NewPosition(line, 1), "bar")
	c.Title = title
	c.Data = [][]interface{}{{"A", 80}, {"B", 90}}
	return c
}

// TestDOCX_ImagesCarryAltText: cada imagen del DOCX lleva su texto
// alternativo en el descr de wp:docPr: una imagen de bloque o inline su alt
// (o, sin alt, su título), una gráfica o un diagrama su título (o, sin él,
// el tipo de figura en el idioma del documento) y la marca de agua su texto.
// Antes ninguna lo llevaba.
func TestDOCX_ImagesCarryAltText(t *testing.T) {
	dir := t.TempDir()
	writePNGs(t, dir, "a.png", "b.png", "c.png", "d.png", "e.png")
	srv, hits, _ := pngServer(t, 120, 60)

	src := "---\ntitle: Alt\nlang: es\nmode: flex\n---\n\n# Alt\n\n" +
		"## Encabezado ![](e.png \"Ícono del encabezado\")\n\n" +
		"![Logo de la empresa](a.png)\n\n" +
		"![](b.png \"Solo título\")\n\n" +
		"Texto ![icono en línea](c.png) y ![](d.png \"Título en línea\").\n"
	doc := parseInline(t, src)
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	block.Elements = append(block.Elements,
		barChart(40, "Ventas 2026"),
		barChart(41, ""),
		ast.NewPlantUMLElement(diagnostics.NewPosition(42, 1), "sequence", "@startuml\nA -> B\n@enduml"),
	)

	out := filepath.Join(t.TempDir(), "out.docx")
	opts := GeneratorOptions{
		Format: "docx", AssetRoot: dir, SourceDir: dir,
		PlantUMLServer: srv.URL,
		Watermark:      &ast.WatermarkConfig{Enabled: true, Text: "BORRADOR"},
	}
	if err := New(newTestLogger()).Generate(doc, out, opts); err != nil {
		t.Fatal(err)
	}
	if hits.Load() == 0 {
		t.Fatal("PlantUML server never contacted")
	}

	want := []string{
		"Ícono del encabezado",
		"Logo de la empresa",
		"Solo título",
		"icono en línea",
		"Título en línea",
		"Ventas 2026",
		"Gráfica: bar",
		"Diagrama: PlantUML sequence",
	}
	if got := docxDescrs(t, out, "word/document.xml"); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("document descr = %q\nwant %q", got, want)
	}
	if got := docxDescrs(t, out, "word/header"); len(got) != 1 || got[0] != "BORRADOR" {
		t.Errorf("watermark descr = %q, want [BORRADOR]", got)
	}
}

// TestDOCX_FigureAltTextFollowsLanguage: el texto alternativo de una figura
// sin título sigue el lang: del documento.
func TestDOCX_FigureAltTextFollowsLanguage(t *testing.T) {
	doc := parseInline(t, "---\ntitle: Alt\nlang: en\nmode: flex\n---\n\n# Alt\n\nText.\n")
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	block.Elements = append(block.Elements, barChart(9, ""))
	out := filepath.Join(t.TempDir(), "out.docx")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatal(err)
	}
	if got := docxDescrs(t, out, "word/document.xml"); len(got) != 1 || got[0] != "Chart: bar" {
		t.Errorf("descr = %q, want [Chart: bar]", got)
	}
}

// TestDOCX_MapAltText: un mapa lleva su título o, sin él, "Mapa" y las
// etiquetas de sus marcadores. Los tiles salen de un servidor local
// (--tile-server), así que necesita un core con MapConfig.WithTileServer.
func TestDOCX_MapAltText(t *testing.T) {
	if _, ok := any(renderer.MapConfig{}).(mapTileServerConfig); !ok {
		t.Skip("this core has no MapConfig.WithTileServer")
	}
	srv, hits, _ := pngServer(t, 256, 256)
	doc := parseInline(t, "---\ntitle: M\nlang: es\nmode: flex\n---\n\n# M\n\nTexto.\n")
	newMap := func(line int, title string) *ast.MapElement {
		m := ast.NewMapElement(diagnostics.NewPosition(line, 1), "default")
		m.Title = title
		m.Zoom = 5
		m.Center = &ast.MapCoordinate{Lat: 19.43, Lng: -99.13}
		m.Markers = []ast.MapMarker{{Lat: 19.43, Lng: -99.13, Label: "CDMX"}, {Lat: 20.67, Lng: -103.35, Label: "Guadalajara"}}
		return m
	}
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	block.Elements = append(block.Elements, newMap(10, "Sucursales"), newMap(11, ""))
	out := filepath.Join(t.TempDir(), "out.docx")
	opts := GeneratorOptions{Format: "docx", TileServer: srv.URL + "/{z}/{x}/{y}.png", TileUserAgent: "doclang-test"}
	if err := New(newTestLogger()).Generate(doc, out, opts); err != nil {
		t.Fatal(err)
	}
	if hits.Load() == 0 {
		t.Fatal("tile server never contacted")
	}
	want := []string{"Sucursales", "Mapa: CDMX, Guadalajara"}
	if got := docxDescrs(t, out, "word/document.xml"); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("descr = %q, want %q", got, want)
	}
}

// TestDOCX_ChromiumFiguresCarryAltText (Chromium real): un diagrama mermaid
// y una ecuación, que se rasterizan con Chromium, también llevan su texto
// alternativo: el título del diagrama y, para la ecuación, su leyenda o la
// fuente LaTeX.
func TestDOCX_ChromiumFiguresCarryAltText(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Chromium")
	}
	doc := parseInline(t, "---\ntitle: C\nlang: es\nmode: flex\n---\n\n# C\n\nTexto.\n")
	mer := ast.NewMermaidElement(diagnostics.NewPosition(10, 1), "flowchart", "flowchart LR\n  A --> B")
	mer.Title = "Flujo de aprobación"
	withCaption := ast.NewMathElement(diagnostics.NewPosition(12, 1), "E = mc^2")
	withCaption.Caption = "Equivalencia masa-energía"
	bare := ast.NewMathElement(diagnostics.NewPosition(14, 1), `a^2 + b^2 = c^2`)
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	block.Elements = append(block.Elements, mer, withCaption, bare)
	out := filepath.Join(t.TempDir(), "out.docx")
	logger := newTestLogger()
	if err := New(logger).Generate(doc, out, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatal(err)
	}
	got := docxDescrs(t, out, "word/document.xml")
	if len(got) == 0 {
		for _, w := range logger.warns {
			if strings.Contains(w, "Chromium") {
				t.Skipf("no Chromium: %s", w)
			}
		}
	}
	want := []string{"Flujo de aprobación", "Equivalencia masa-energía", "a^2 + b^2 = c^2"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("descr = %q, want %q (warnings %q)", got, want, logger.warns)
	}
}
