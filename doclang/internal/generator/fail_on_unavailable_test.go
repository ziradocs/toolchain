// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
)

// unavailableDoc tiene, en líneas conocidas, una imagen que sí existe
// (ok.png), una de bloque que falta, una inline que falta dentro de una
// celda, una inline remota, un diagrama PlantUML y un mapa.
func unavailableDoc(t *testing.T) *ast.AST {
	t.Helper()
	src := "---\ntitle: U\nlang: es\nmode: flex\n---\n\n# U\n\n" + // 1-8
		"![bien](ok.png)\n\n" + // 9
		"![falta](falta.png)\n\n" + // 11
		"| A |\n|---|\n| ![celda](celda.png) |\n\n" + // 13-15
		"Texto ![remota](https://example.com/r.png).\n" // 17
	doc := parseInline(t, src)
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	m := ast.NewMapElement(diagnostics.NewPosition(30, 1), "default")
	m.Zoom = 3
	block.Elements = append(block.Elements,
		ast.NewPlantUMLElement(diagnostics.NewPosition(25, 1), "sequence", "@startuml\nA -> B\n@enduml"),
		m,
	)
	return doc
}

func lineOf(t *testing.T, doc *ast.AST, match func(ast.Element) bool) int {
	t.Helper()
	var line int
	WalkElements(doc, func(e ast.Element) bool {
		if line == 0 && match(e) {
			line = e.GetPosition().Line
		}
		return true
	})
	if line == 0 {
		t.Fatal("element not found")
	}
	return line
}

// TestDOCX_FailOnUnavailable: con FailOnUnavailable, un DOCX con contenido
// que quedaría como marcador falla sin escribir el archivo, y el error
// lista cada uno con su línea, tipo y fuente. Sin la opción (el default) el
// DOCX se escribe como siempre.
func TestDOCX_FailOnUnavailable(t *testing.T) {
	dir := t.TempDir()
	writePNGs(t, dir, "ok.png")
	doc := unavailableDoc(t)
	imgLine := lineOf(t, doc, func(e ast.Element) bool {
		img, ok := e.(*ast.ImageElement)
		return ok && img.Source == "falta.png"
	})
	tableLine := lineOf(t, doc, func(e ast.Element) bool { _, ok := e.(*ast.TableElement); return ok })
	textLine := lineOf(t, doc, func(e ast.Element) bool {
		txt, ok := e.(*ast.TextElement)
		return ok && strings.Contains(txt.Content, "remota")
	})

	// NoNetwork deja el PlantUML y el mapa como marcador sin pedir nada.
	base := GeneratorOptions{Format: "docx", AssetRoot: dir, SourceDir: dir, NoNetwork: true}

	out := filepath.Join(t.TempDir(), "default.docx")
	if err := New(newTestLogger()).Generate(doc, out, base); err != nil {
		t.Fatalf("default build failed: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("default build left no output: %v", err)
	}

	strict := base
	strict.FailOnUnavailable = true
	out = filepath.Join(t.TempDir(), "strict.docx")
	err := New(newTestLogger()).Generate(doc, out, strict)
	var ue *UnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("want *UnavailableError, got %v", err)
	}
	want := []UnavailableContent{
		{Kind: "image", Line: imgLine, Detail: "falta.png"},
		{Kind: "image", Line: tableLine, Detail: "celda.png"},
		{Kind: "image", Line: textLine, Detail: "https://example.com/r.png"},
		{Kind: "diagram", Line: 25, Detail: ""},
		{Kind: "map", Line: 30, Detail: ""},
	}
	if !reflect.DeepEqual(ue.Items, want) {
		t.Errorf("items = %+v\nwant %+v", ue.Items, want)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("failed build left an output file (stat err %v)", err)
	}
	msg := err.Error()
	for _, s := range []string{"5 items", "line 25: diagram", `image "falta.png"`, "no output was written"} {
		if !strings.Contains(msg, s) {
			t.Errorf("message lacks %q:\n%s", s, msg)
		}
	}
}

// TestDOCX_FailOnUnavailable_CleanDocumentPasses: sin nada faltante la
// opción no cambia nada.
func TestDOCX_FailOnUnavailable_CleanDocumentPasses(t *testing.T) {
	dir := t.TempDir()
	writePNGs(t, dir, "ok.png")
	doc := parseInline(t, "---\ntitle: C\nmode: flex\n---\n\n# C\n\n![bien](ok.png)\n")
	out := filepath.Join(t.TempDir(), "out.docx")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: dir, SourceDir: dir, FailOnUnavailable: true}); err != nil {
		t.Fatal(err)
	}
}

func coreReportsUnavailable() bool {
	_, ok := any(renderer.NewDefaultRenderContext()).(unavailableReporterSetter)
	return ok
}

// TestHTML_FailOnUnavailable: el HTML offline-inline (la misma ruta que el
// PDF, sin Chromium) falla igual, con la línea que reporta core, y no
// escribe el archivo. Necesita un core con SetUnavailableReporter; contra
// uno que no lo tiene, la opción es un error explícito en vez de pasar en
// silencio.
func TestHTML_FailOnUnavailable(t *testing.T) {
	dir := t.TempDir()
	writePNGs(t, dir, "ok.png")
	doc := parseInline(t, "---\ntitle: H\nmode: flex\n---\n\n# H\n\n![bien](ok.png)\n\n![falta](falta.png)\n\nTexto ![p](otra.png).\n")
	imgLine := lineOf(t, doc, func(e ast.Element) bool {
		img, ok := e.(*ast.ImageElement)
		return ok && img.Source == "falta.png"
	})
	textLine := lineOf(t, doc, func(e ast.Element) bool { _, ok := e.(*ast.TextElement); return ok })
	out := filepath.Join(t.TempDir(), "out.html")
	opts := GeneratorOptions{Format: "html", ImageMode: "offline-inline", AssetRoot: dir, SourceDir: dir, FailOnUnavailable: true}
	err := New(newTestLogger()).Generate(doc, out, opts)
	if !coreReportsUnavailable() {
		if !errors.Is(err, errUnavailableNeedsCore) {
			t.Fatalf("old core: want errUnavailableNeedsCore, got %v", err)
		}
		t.Skip("this core has no RenderContext.SetUnavailableReporter")
	}
	var ue *UnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("want *UnavailableError, got %v", err)
	}
	want := []UnavailableContent{{Kind: "image", Line: imgLine, Detail: "falta.png"}, {Kind: "image", Line: textLine, Detail: "otra.png"}}
	if !reflect.DeepEqual(ue.Items, want) {
		t.Errorf("items = %+v, want %+v", ue.Items, want)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("failed build left an output file (stat err %v)", err)
	}

	// Sin la opción el HTML se escribe con sus marcadores.
	opts.FailOnUnavailable = false
	if err := New(newTestLogger()).Generate(doc, out, opts); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), "image-unavailable") {
		t.Fatalf("default HTML lacks its marker (err %v)", err)
	}
}

// TestPDF_FailOnUnavailable (Chromium real): el PDF falla antes de imprimir
// y no deja el archivo.
func TestPDF_FailOnUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Chromium")
	}
	if !coreReportsUnavailable() {
		t.Skip("this core has no RenderContext.SetUnavailableReporter")
	}
	dir := t.TempDir()
	doc := parseInline(t, "---\ntitle: P\nmode: flex\n---\n\n# P\n\n![falta](falta.png)\n")
	out := filepath.Join(t.TempDir(), "out.pdf")
	err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "pdf", AssetRoot: dir, SourceDir: dir, FailOnUnavailable: true})
	if err != nil && strings.Contains(err.Error(), "failed to initialize chromium") {
		t.Skipf("no Chromium: %v", err)
	}
	var ue *UnavailableError
	if !errors.As(err, &ue) || len(ue.Items) != 1 || ue.Items[0].Detail != "falta.png" {
		t.Fatalf("want one missing image, got %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("failed build left an output file (stat err %v)", err)
	}
}

// TestDOCX_FailOnUnavailable_WithoutChromium: sin Chromium (una
// --chromium-path que no existe), un diagrama mermaid, una ecuación y una
// gráfica que no se dibuja de forma nativa quedan como marcador, y
// FailOnUnavailable los lista con su línea y su tipo.
func TestDOCX_FailOnUnavailable_WithoutChromium(t *testing.T) {
	doc := parseInline(t, "---\ntitle: S\nlang: es\nmode: flex\n---\n\n# S\n\nTexto.\n")
	radar := ast.NewChartElement(diagnostics.NewPosition(30, 1), "radar") // radar no tiene render nativo
	radar.Data = [][]interface{}{{"A", 1}, {"B", 2}, {"C", 3}}
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	block.Elements = append(block.Elements,
		ast.NewMermaidElement(diagnostics.NewPosition(20, 1), "flowchart", "flowchart LR\n A --> B"),
		ast.NewMathElement(diagnostics.NewPosition(25, 1), "E = mc^2"),
		radar,
	)
	out := filepath.Join(t.TempDir(), "out.docx")
	opts := GeneratorOptions{Format: "docx", ChromiumPath: filepath.Join(t.TempDir(), "no-chromium"), FailOnUnavailable: true}
	err := New(newTestLogger()).Generate(doc, out, opts)
	var ue *UnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("want *UnavailableError, got %v", err)
	}
	want := []UnavailableContent{{Kind: "diagram", Line: 20, Detail: ""}, {Kind: "equation", Line: 25, Detail: ""}, {Kind: "chart", Line: 30, Detail: ""}}
	if !reflect.DeepEqual(ue.Items, want) {
		t.Errorf("items = %+v, want %+v", ue.Items, want)
	}
}

// TestFailOnUnavailable_HeaderLogoOnlyWhereDrawn: un logo de encabezado que
// falta solo cuenta donde la salida lo dibuja, el HTML page-view con el
// encabezado activo, en un modo que resuelve imágenes al compilar (lo
// reporta core). DOCX, PDF y el HTML normal no dibujan
// el logo, un encabezado con enabled: false tampoco, ni un layout_defaults
// que ningún bloque usa: ahí la bandera no falla por el logo.
func TestFailOnUnavailable_HeaderLogoOnlyWhereDrawn(t *testing.T) {
	dir := t.TempDir()
	logo := &ast.LogoConfig{Source: "logo-falta.png"}
	enabled := &ast.HeaderFooterConfig{Header: &ast.HeaderConfig{Enabled: true, Logo: logo}}
	disabled := &ast.HeaderFooterConfig{Header: &ast.HeaderConfig{Enabled: false, Logo: logo}}
	unusedLayout := &ast.HeaderFooterConfig{LayoutDefaults: map[string]*ast.LayoutHeaderFooterConfig{
		"title_slide": {Header: &ast.HeaderConfig{Enabled: true, Logo: logo}},
	}}
	build := func(format string, hf *ast.HeaderFooterConfig, pageView bool) error {
		doc := parseInline(t, "---\ntitle: L\nmode: flex\n---\n\n# L\n\nTexto.\n")
		opts := GeneratorOptions{Format: format, AssetRoot: dir, SourceDir: dir, HeaderFooter: hf, FailOnUnavailable: true}
		if pageView {
			// offline-inline: en modo browser el logo lo resuelve el
			// navegador del lector y no se comprueba al compilar.
			opts.Theme, opts.ShowHeaders, opts.ShowFooters, opts.ImageMode = "page-view", true, true, "offline-inline"
		}
		return New(newTestLogger()).Generate(doc, filepath.Join(t.TempDir(), "out."+format), opts)
	}
	if err := build("docx", enabled, false); err != nil {
		t.Errorf("docx failed for a logo it never draws: %v", err)
	}
	if !coreReportsUnavailable() {
		return
	}
	if err := build("html", enabled, false); err != nil {
		t.Errorf("html without page-view failed for a logo it never draws: %v", err)
	}
	for name, hf := range map[string]*ast.HeaderFooterConfig{"enabled: false": disabled, "unused layout": unusedLayout} {
		if err := build("html", hf, true); err != nil {
			t.Errorf("page-view with %s failed: %v", name, err)
		}
	}
	err := build("html", enabled, true)
	var ue *UnavailableError
	if !errors.As(err, &ue) || !reflect.DeepEqual(ue.Items, []UnavailableContent{{Kind: "image", Line: 0, Detail: "logo-falta.png"}}) {
		t.Errorf("page-view with the header enabled: want the missing logo, got %v", err)
	}
}

// TestDOCX_ChartAltFromOptionsTitle: sin title, el texto alternativo de una
// gráfica es options.plugins.title.text, el título que dibuja Chart.js.
func TestDOCX_ChartAltFromOptionsTitle(t *testing.T) {
	doc := parseInline(t, "---\ntitle: C\nlang: es\nmode: flex\n---\n\n# C\n\nTexto.\n")
	c := barChart(9, "")
	c.Options = map[string]interface{}{"plugins": map[string]interface{}{"title": map[string]interface{}{"text": "Ventas desde options"}}}
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	block.Elements = append(block.Elements, c)
	out := filepath.Join(t.TempDir(), "out.docx")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatal(err)
	}
	if got := docxDescrs(t, out, "word/document.xml"); len(got) != 1 || got[0] != "Ventas desde options" {
		t.Errorf("descr = %q", got)
	}
}

// TestNoNetwork_ChromiumIsRestricted (Chromium real): con NoNetwork el DOCX
// sigue dibujando un mermaid (los scripts van embebidos) con el navegador
// arrancado sin red; contra un core sin RestrictNetwork, NoNetwork con
// Chromium es un error explícito.
func TestNoNetwork_ChromiumIsRestricted(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Chromium")
	}
	doc := parseInline(t, "---\ntitle: N\nlang: es\nmode: flex\n---\n\n# N\n\nTexto.\n")
	m := ast.NewMermaidElement(diagnostics.NewPosition(9, 1), "flowchart", "flowchart LR\n A --> B")
	m.Title = "Flujo"
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	block.Elements = append(block.Elements, m)
	out := filepath.Join(t.TempDir(), "out.docx")
	logger := newTestLogger()
	err := New(logger).Generate(doc, out, GeneratorOptions{Format: "docx", NoNetwork: true, FailOnUnavailable: true})
	for _, w := range logger.warns {
		if strings.Contains(w, "Failed to initialize Chromium") {
			t.Skipf("no Chromium: %s", w)
		}
	}
	if _, ok := any(&chromium.ChromiumRenderer{}).(networkRestricter); !ok {
		if !errors.Is(err, errNoNetworkNeedsCore) {
			t.Fatalf("old core: want errNoNetworkNeedsCore, got %v", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("NoNetwork build with mermaid failed: %v", err)
	}
	if got := docxDescrs(t, out, "word/document.xml"); len(got) != 1 || got[0] != "Flujo" {
		t.Errorf("descr = %q, want the mermaid image", got)
	}
}

// TestNoNetwork_EveryChromiumPathRestricts (Chromium real): cada generador
// que arranca Chromium con NoNetwork llama a RestrictNetwork antes de usarlo
// (DOCX con un mermaid, HTML offline-inline con un mermaid y PDF). Si un
// camino deja de llamarlo, esta prueba falla.
func TestNoNetwork_EveryChromiumPathRestricts(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Chromium")
	}
	if _, ok := any(&chromium.ChromiumRenderer{}).(networkRestricter); !ok {
		t.Skip("this core has no ChromiumRenderer.RestrictNetwork")
	}
	var calls []string
	prev := restrictNetworkFn
	t.Cleanup(func() { restrictNetworkFn = prev })
	current := ""
	restrictNetworkFn = func(r any, opts GeneratorOptions) error {
		if opts.NoNetwork {
			calls = append(calls, current)
		}
		return prev(r, opts)
	}
	for _, c := range []struct {
		format string
		opts   GeneratorOptions
	}{
		{"docx", GeneratorOptions{Format: "docx"}},
		{"html", GeneratorOptions{Format: "html", MermaidMode: "offline-inline"}},
		{"pdf", GeneratorOptions{Format: "pdf"}},
	} {
		current = c.format
		doc := parseInline(t, "---\ntitle: N\nlang: es\nmode: flex\n---\n\n# N\n\nTexto.\n")
		block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
		block.Elements = append(block.Elements, ast.NewMermaidElement(diagnostics.NewPosition(9, 1), "flowchart", "flowchart LR\n A --> B"))
		opts := c.opts
		opts.NoNetwork = true
		logger := newTestLogger()
		err := New(logger).Generate(doc, filepath.Join(t.TempDir(), "out."+c.format), opts)
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "chromium") {
			t.Skipf("no Chromium: %v", err)
		}
		for _, w := range logger.warns {
			if strings.Contains(w, "Failed to initialize Chromium") {
				t.Skipf("no Chromium: %s", w)
			}
		}
		if err != nil {
			t.Fatalf("%s: %v", c.format, err)
		}
	}
	if want := []string{"docx", "html", "pdf"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("RestrictNetwork called for %v, want %v", calls, want)
	}
}
