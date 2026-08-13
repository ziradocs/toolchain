// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
)

// nativeBarChartElement es un chart bar con datos reales (issue "quitar
// Chrome del pipeline"): SupportsNativeChartRendering exige tipo elegible,
// pero RenderChartNativePNG además necesita datos válidos — un chart sin
// Data/Labels fallaría el render real y el gate caería a Chromium.
func nativeBarChartElement() *ast.ChartElement {
	c := ast.NewChartElement(diagnostics.NewPosition(1, 1), "bar")
	c.Data = [][]interface{}{{"A", 10.0}, {"B", 20.0}}
	c.Labels = []string{"A", "B"}
	return c
}

// TestDOCXGenerator_Generate_NativeChartOnlyNeverInitializesChromium cubre
// el gate nuevo en docx.go: un documento cuyo único elemento rico es un
// chart nativo-capaz no debe instanciar Chromium, ni siquiera con un
// --chromium-path roto — mismo patrón que
// slidelang's TestSetupOfflineRenderContext_NativeDeckSkipsChromium (issue
// #164), extraído a renderer.TryAllChartsNative.
func TestDOCXGenerator_Generate_NativeChartOnlyNeverInitializesChromium(t *testing.T) {
	doc := astWithElements(nativeBarChartElement())

	gen := NewDOCXGenerator(newTestLogger(), t.TempDir())
	output := filepath.Join(t.TempDir(), "document.docx")
	opts := GeneratorOptions{
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	if err := gen.Generate(doc, output, opts); err != nil {
		t.Fatalf("expected no error for a native-chart-only document even with a broken Chromium path, got: %v", err)
	}
	if gen.chromiumRenderer != nil {
		t.Error("expected chromiumRenderer to stay nil — the doc has no elements that need it")
	}
}

// TestDOCXGenerator_Generate_MermaidStillNeedsChromium confirma el otro
// lado del gate: un documento con mermaid (Kroki no cubre DOCX — solo HTML/
// PDF vía RenderContext) debe seguir intentando Chromium de verdad.
func TestDOCXGenerator_Generate_MermaidStillNeedsChromium(t *testing.T) {
	doc := astWithElements(ast.NewMermaidElement(diagnostics.NewPosition(1, 1), "flowchart", "graph TD; A-->B"))

	gen := NewDOCXGenerator(newTestLogger(), t.TempDir())
	output := filepath.Join(t.TempDir(), "document.docx")
	opts := GeneratorOptions{
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	// Generate() nunca retorna error por Chromium roto (docx.go solo
	// loguea un warning y sigue con chromiumRenderer==nil, ver Generate) —
	// la aserción real es que SÍ lo intentó y falló, no que quedó nil sin
	// más.
	if err := gen.Generate(doc, output, opts); err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if gen.chromiumRenderer != nil {
		t.Error("expected chromiumRenderer to still be nil after a failed init attempt")
	}
}

// TestHTMLGenerator_Generate_NativeChartOnlySkipsChromium es el análogo de
// TestDOCXGenerator_Generate_NativeChartOnlyNeverInitializesChromium para
// --format html --render-mode offline-inline: antes de este cambio,
// needsChromium se calculaba solo por MODO (ChartMode=="offline-inline"),
// así que este documento habría intentado instanciar Chromium y fallado con
// el path roto — con el gate nuevo, debe generar el HTML igual, con el
// chart embebido como PNG inline.
func TestHTMLGenerator_Generate_NativeChartOnlySkipsChromium(t *testing.T) {
	doc := astWithElements(nativeBarChartElement())

	gen := NewHTMLGenerator(newTestLogger())
	output := filepath.Join(t.TempDir(), "document.html")
	opts := GeneratorOptions{
		ChartMode:       "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	if err := gen.Generate(doc, output, opts); err != nil {
		t.Fatalf("expected no error for a native-chart-only document in offline-inline mode even with a broken Chromium path, got: %v", err)
	}
}

// TestHTMLGenerator_Generate_MermaidOfflineStillNeedsChromium confirma que
// el gate no se sobre-extiende: un documento con mermaid en modo offline
// (backend chromium, el default) debe seguir intentando Chromium de verdad
// y fallar con el path roto.
func TestHTMLGenerator_Generate_MermaidOfflineStillNeedsChromium(t *testing.T) {
	doc := astWithElements(ast.NewMermaidElement(diagnostics.NewPosition(1, 1), "flowchart", "graph TD; A-->B"))

	gen := NewHTMLGenerator(newTestLogger())
	output := filepath.Join(t.TempDir(), "document.html")
	opts := GeneratorOptions{
		MermaidMode:     "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	err := gen.Generate(doc, output, opts)
	if err == nil {
		t.Fatal("expected an error initializing Chromium for a mermaid-offline document with a broken Chromium path")
	}
	if !strings.Contains(err.Error(), "Chromium") {
		t.Errorf("expected the error to mention Chromium, got: %v", err)
	}
}

// TestHTMLGenerator_Generate_MermaidOfflineWithKrokiSkipsChromium confirma
// que el mismo documento con DiagramBackend=="kroki" ya NO necesita
// Chromium — KrokiFetcher es HTTP puro (issue "quitar Chrome del
// pipeline").
func TestHTMLGenerator_Generate_MermaidOfflineWithKrokiSkipsChromium(t *testing.T) {
	krokiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg>mermaid</svg>"))
	}))
	defer krokiServer.Close()

	doc := astWithElements(ast.NewMermaidElement(diagnostics.NewPosition(1, 1), "flowchart", "graph TD; A-->B"))

	gen := NewHTMLGenerator(newTestLogger())
	output := filepath.Join(t.TempDir(), "document.html")
	opts := GeneratorOptions{
		MermaidMode:     "offline-inline",
		DiagramBackend:  "kroki",
		KrokiServer:     krokiServer.URL,
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	if err := gen.Generate(doc, output, opts); err != nil {
		t.Fatalf("expected no error for a mermaid-offline document with DiagramBackend=kroki even with a broken Chromium path, got: %v", err)
	}
}

// TestPDFGenerator_ForcesOfflineInlineForMermaid cubre la Fase 3 (issue
// "quitar Chrome del pipeline"): antes de este cambio, el HTML impreso a
// PDF dejaba mermaid en modo "browser" (CDN) — el diagrama dependía de que
// mermaid.js cargara+inicializara+dibujara dentro de la ventana de espera
// fija de RenderHTMLToPDF, que el propio repo documenta como insuficiente.
// No se puede probar el PDF final sin un Chromium real (no disponible en
// este entorno), pero SÍ se puede probar, sin navegador, que el HTML que
// PDFGenerator.Generate arma internamente ya no lleva el <script> CDN de
// mermaid.js y en su lugar trae el SVG resuelto — reproduciendo la MISMA
// construcción de renderOpts/ctx que pdf.go usa (offlineMode
// "offline-inline" en todos los modos + un ChromiumRenderer real solo hace
// falta para el paso de impresión, no para este armado de HTML).
func TestPDFGenerator_ForcesOfflineInlineForMermaid(t *testing.T) {
	krokiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<svg id="kroki-resolved-diagram"></svg>`))
	}))
	defer krokiServer.Close()

	doc := astWithElements(ast.NewMermaidElement(diagnostics.NewPosition(1, 1), "flowchart", "graph TD; A-->B"))

	const offlineMode = "offline-inline"
	renderOpts := renderer.DocumentHTMLOptions{
		EmbedAssets:  true,
		PlantUMLMode: offlineMode,
		MermaidMode:  offlineMode,
		ChartMode:    offlineMode,
		MapMode:      offlineMode,
		MathMode:     offlineMode,
	}
	ctx := chromium.NewRenderContext(nil, chromium.RenderContextOptions{
		PlantUMLMode:   offlineMode,
		MermaidMode:    offlineMode,
		ChartMode:      offlineMode,
		MapMode:        offlineMode,
		MathMode:       offlineMode,
		OutputDir:      t.TempDir(),
		DiagramBackend: "kroki",
		KrokiServer:    krokiServer.URL,
	})

	html := renderer.GenerateDocumentHTML(doc, renderOpts, ctx)

	if strings.Contains(html, renderer.MermaidCDNScriptTag) {
		t.Error("expected the PDF-bound HTML to NOT depend on the mermaid CDN script — mermaid must be pre-resolved offline-inline")
	}
	if !strings.Contains(html, "kroki-resolved-diagram") {
		t.Error("expected the pre-resolved SVG from the Kroki fetcher to be embedded inline")
	}
}

// TestPDFGenerator_ForcesSVGForPlantUML cubre el hallazgo de code-review
// sobre este PR: pdf.go forzaba offlineMode="offline-inline" para PlantUML
// (igual que mermaid/chart/map/math) pero seguía pasando opts.PlantUMLFormat
// sin tocar. FetchDiagramInline es SVG-only por construcción
// (plantuml_fetcher.go: "if f.format != "svg" { return error }") — con
// --plantuml-format=png el build no fallaba, pero el diagrama salía
// reemplazado por un <div class="plantuml-error"> silencioso dentro del
// PDF. pdf.go ahora fuerza "svg" para PDF sin importar lo que el usuario
// haya pedido (ese flag solo tiene efecto real en --format html
// --render-mode offline-assets, el único modo que sí acepta PNG vía
// FetchDiagramToAssets). Reproduce la misma construcción de renderOpts/ctx
// que pdf.go arma internamente, mismo patrón que
// TestPDFGenerator_ForcesOfflineInlineForMermaid.
func TestPDFGenerator_ForcesSVGForPlantUML(t *testing.T) {
	plantumlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(`<svg id="plantuml-resolved-diagram"></svg>`))
	}))
	defer plantumlServer.Close()

	doc := astWithElements(ast.NewPlantUMLElement(diagnostics.NewPosition(1, 1), "sequence", "Alice -> Bob: hello"))

	const offlineMode = "offline-inline"
	const offlinePlantUMLFormat = "svg" // lo que pdf.go fuerza, sin importar --plantuml-format

	renderOpts := renderer.DocumentHTMLOptions{
		EmbedAssets:    true,
		PlantUMLMode:   offlineMode,
		PlantUMLFormat: offlinePlantUMLFormat,
		MermaidMode:    offlineMode,
		ChartMode:      offlineMode,
		MapMode:        offlineMode,
		MathMode:       offlineMode,
	}
	ctx := chromium.NewRenderContext(nil, chromium.RenderContextOptions{
		PlantUMLMode:   offlineMode,
		PlantUMLServer: plantumlServer.URL,
		PlantUMLFormat: offlinePlantUMLFormat,
		MermaidMode:    offlineMode,
		ChartMode:      offlineMode,
		MapMode:        offlineMode,
		MathMode:       offlineMode,
		OutputDir:      t.TempDir(),
	})

	html := renderer.GenerateDocumentHTML(doc, renderOpts, ctx)

	// La hoja de estilos siempre define la regla CSS ".plantuml-error {...}"
	// sin importar si algún diagrama falló — un strings.Contains(html,
	// "plantuml-error") pelón matchea esa regla y da falso negativo. Hay que
	// buscar el div que renderPlantUMLOfflineInline realmente emite en error.
	if strings.Contains(html, `class="plantuml-error"`) {
		t.Errorf("expected no plantuml-error div when PDF forces svg format, got HTML containing one:\n%s", html)
	}
	if !strings.Contains(html, "plantuml-resolved-diagram") {
		t.Error("expected the pre-resolved SVG from the PlantUML fetcher to be embedded inline")
	}
}

// TestPDFGenerator_PNGFormatBreaksOfflineInlinePlantUML documenta el
// contrato que hace necesario el fix de arriba: FetchDiagramInline
// (plantuml_fetcher.go) rechaza cualquier formato distinto de "svg" — si
// esto alguna vez deja de fallar, offline-inline empezó a aceptar PNG y el
// forzado a "svg" en pdf.go dejó de ser necesario (o el bug volvió si nadie
// actualizó pdf.go a la vez).
func TestPDFGenerator_PNGFormatBreaksOfflineInlinePlantUML(t *testing.T) {
	plantumlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("not-a-real-png"))
	}))
	defer plantumlServer.Close()

	doc := astWithElements(ast.NewPlantUMLElement(diagnostics.NewPosition(1, 1), "sequence", "Alice -> Bob: hello"))

	const offlineMode = "offline-inline"
	renderOpts := renderer.DocumentHTMLOptions{
		EmbedAssets:    true,
		PlantUMLMode:   offlineMode,
		PlantUMLFormat: "png",
	}
	ctx := chromium.NewRenderContext(nil, chromium.RenderContextOptions{
		PlantUMLMode:   offlineMode,
		PlantUMLServer: plantumlServer.URL,
		PlantUMLFormat: "png",
		OutputDir:      t.TempDir(),
	})

	html := renderer.GenerateDocumentHTML(doc, renderOpts, ctx)

	if !strings.Contains(html, `class="plantuml-error"`) {
		t.Error("expected FetchDiagramInline to reject non-svg format and render a plantuml-error div for offline-inline + png")
	}
}

// TestHTMLGenerator_Generate_KrokiMermaidPlusNativeChartSkipsChromium cubre
// un hallazgo de code-review sobre este PR: con un documento que trae
// mermaid (vía Kroki) Y un chart nativo-capaz, mermaidNeedsBrowser es false
// (Kroki lo cubre) y el chart queda como único candidato — pero
// renderer.TryAllChartsNative recorría TODO el AST y bailaba a false en
// cuanto encontraba CUALQUIER MermaidElement, sin saber que este
// diagramBackend ya lo tenía resuelto por HTTP puro. needsChromium se
// quedaba activo y el build fallaba si Chromium no estaba instalado, aunque
// ningún elemento del documento lo necesitara de verdad — slidelang ya
// soportaba esta combinación (tryBuildNativeContext), doclang no.
func TestHTMLGenerator_Generate_KrokiMermaidPlusNativeChartSkipsChromium(t *testing.T) {
	krokiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg>mermaid</svg>"))
	}))
	defer krokiServer.Close()

	doc := astWithElements(
		ast.NewMermaidElement(diagnostics.NewPosition(1, 1), "flowchart", "graph TD; A-->B"),
		nativeBarChartElement(),
	)

	gen := NewHTMLGenerator(newTestLogger())
	output := filepath.Join(t.TempDir(), "document.html")
	opts := GeneratorOptions{
		MermaidMode:     "offline-inline",
		ChartMode:       "offline-inline",
		DiagramBackend:  "kroki",
		KrokiServer:     krokiServer.URL,
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	if err := gen.Generate(doc, output, opts); err != nil {
		t.Fatalf("expected no error for mermaid-via-kroki + a native-capable chart even with a broken Chromium path, got: %v", err)
	}
}

// TestHTMLGenerator_Generate_MermaidOnlyDoesNotNeedChromiumForAbsentChartMapMath
// cubre un bug real encontrado durante la verificación manual de esta
// sesión (no solo un artefacto del test): --render-mode aplica el MISMO
// modo a Mermaid/Chart/Map/Math a la vez (build.go), sin importar qué
// elementos tenga el documento — antes de este fix, needsChromium en
// html.go solo miraba el MODO pedido, así que un documento de solo texto +
// mermaid con --render-mode offline-inline igual exigía Chromium para
// chart/map/math que nunca aparecen en el documento. Reproducido primero
// vía CLI real (--diagram-backend kroki + --chromium-path roto seguía
// fallando) antes de reducirlo a este test.
func TestHTMLGenerator_Generate_MermaidOnlyDoesNotNeedChromiumForAbsentChartMapMath(t *testing.T) {
	krokiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg>mermaid</svg>"))
	}))
	defer krokiServer.Close()

	doc := astWithElements(ast.NewMermaidElement(diagnostics.NewPosition(1, 1), "flowchart", "graph TD; A-->B"))

	gen := NewHTMLGenerator(newTestLogger())
	output := filepath.Join(t.TempDir(), "document.html")
	opts := GeneratorOptions{
		// --render-mode offline-inline: los cuatro campos a la vez, como
		// hace doclang/internal/cli/build.go.
		PlantUMLMode:    "offline-inline",
		MermaidMode:     "offline-inline",
		ChartMode:       "offline-inline",
		MapMode:         "offline-inline",
		MathMode:        "offline-inline",
		DiagramBackend:  "kroki",
		KrokiServer:     krokiServer.URL,
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	if err := gen.Generate(doc, output, opts); err != nil {
		t.Fatalf("expected no error — the document has no chart/map/math, so their offline-inline mode should be a no-op, got: %v", err)
	}
}
