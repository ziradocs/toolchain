// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// TestHasInteractiveElements_MathRequiresChromium cubre el hallazgo Alta de
// code-review sobre PR #56: antes del fix, hasInteractiveElements ignoraba
// MathElement por completo, así que un deck cuyo único elemento rico era una
// ecuación nunca instanciaba Chromium en modo offline/PDF y la ecuación
// quedaba como LaTeX crudo sin tipografiar.
func TestHasInteractiveElements_MathRequiresChromium(t *testing.T) {
	doc := astWithElements(ast.NewMathElement(nativePos(), "E = mc^2"))
	if !hasInteractiveElements(doc, "chromium") {
		t.Error("expected hasInteractiveElements(doc-with-math) = true")
	}
}

// TestHasInteractiveElements_PlantUMLOnlyDoesNotRequireChromium: PlantUML es
// HTTP puro (chromium.PlantUMLFetcher no usa chromedp), así que NO debe
// entrar al gate que decide si hace falta arrancar Chromium — eso lo cubre
// hasPlantUMLElements, un chequeo aparte.
func TestHasInteractiveElements_PlantUMLOnlyDoesNotRequireChromium(t *testing.T) {
	doc := astWithElements(ast.NewPlantUMLElement(nativePos(), "sequence", "A->B: hola"))
	if hasInteractiveElements(doc, "chromium") {
		t.Error("expected hasInteractiveElements(doc-with-only-plantuml) = false — PlantUML doesn't need Chromium")
	}
}

// TestHasChromiumOnlyElements_ExcludesMath cubre el split que separó
// hasInteractiveElements en hasChromiumOnlyElements (mermaid/chart/map, sin
// degradación razonable) y hasMathElements (con degradación a LaTeX crudo) —
// hallazgo de code-review sobre PR #56.
func TestHasChromiumOnlyElements_ExcludesMath(t *testing.T) {
	mathOnly := astWithElements(ast.NewMathElement(nativePos(), "E = mc^2"))
	if hasChromiumOnlyElements(mathOnly, "chromium") {
		t.Error("expected hasChromiumOnlyElements(doc-with-only-math) = false — math has a text fallback")
	}

	mermaidDoc := astWithElements(ast.NewMermaidElement(nativePos(), "flowchart", "graph TD; A-->B"))
	if !hasChromiumOnlyElements(mermaidDoc, "chromium") {
		t.Error("expected hasChromiumOnlyElements(doc-with-mermaid) = true")
	}
}

// TestHasChromiumOnlyElements_KrokiExcludesMermaid cubre el cambio de esta
// sesión (issue "quitar Chrome del pipeline"): con DiagramBackend=="kroki",
// mermaid deja de forzar Chromium — KrokiFetcher lo cubre por HTTP puro.
// Chart/Map siguen forzándolo, Kroki no los cubre.
func TestHasChromiumOnlyElements_KrokiExcludesMermaid(t *testing.T) {
	mermaidDoc := astWithElements(ast.NewMermaidElement(nativePos(), "flowchart", "graph TD; A-->B"))
	if hasChromiumOnlyElements(mermaidDoc, "kroki") {
		t.Error("expected hasChromiumOnlyElements(doc-with-mermaid, kroki) = false — Kroki covers mermaid")
	}

	chartDoc := astWithElements(&ast.ChartElement{ChartType: "scatter"})
	if !hasChromiumOnlyElements(chartDoc, "kroki") {
		t.Error("expected hasChromiumOnlyElements(doc-with-chart, kroki) = true — Kroki doesn't cover charts")
	}
}

func TestHasMathElements(t *testing.T) {
	withMath := astWithElements(ast.NewMathElement(nativePos(), "E = mc^2"))
	withoutMath := astWithElements(ast.NewTextElement(nativePos(), "just text"))

	if !hasMathElements(withMath) {
		t.Error("expected hasMathElements(doc-with-math) = true")
	}
	if hasMathElements(withoutMath) {
		t.Error("expected hasMathElements(doc-without-math) = false")
	}
	if hasMathElements(nil) {
		t.Error("expected hasMathElements(nil) = false")
	}
}

func TestHasPlantUMLElements(t *testing.T) {
	withPlantUML := astWithElements(ast.NewPlantUMLElement(nativePos(), "sequence", "A->B"))
	withoutPlantUML := astWithElements(ast.NewTextElement(nativePos(), "just text"))

	if !hasPlantUMLElements(withPlantUML) {
		t.Error("expected hasPlantUMLElements(doc-with-plantuml) = true")
	}
	if hasPlantUMLElements(withoutPlantUML) {
		t.Error("expected hasPlantUMLElements(doc-without-plantuml) = false")
	}
	if hasPlantUMLElements(nil) {
		t.Error("expected hasPlantUMLElements(nil) = false")
	}
}

// TestWirePlantUMLFetcher_OnlyWiresWhenDocHasPlantUML: llamarla sobre un
// deck sin PlantUML no debe tocar el ctx — evita construir un fetcher HTTP
// (y su cliente restringido anti-SSRF) que nunca se va a usar.
func TestWirePlantUMLFetcher_OnlyWiresWhenDocHasPlantUML(t *testing.T) {
	doc := astWithElements(ast.NewTextElement(nativePos(), "just text"))
	ctx := renderer.NewDefaultRenderContext()

	wirePlantUMLFetcher(ctx, doc, GeneratorOptions{RenderMode: "offline-inline"}, t.TempDir())

	if ctx.Fetcher != nil {
		t.Error("expected ctx.Fetcher to stay nil for a doc without PlantUML")
	}
	if ctx.PlantUMLMode != "browser" {
		t.Errorf("PlantUMLMode = %q, want unchanged default %q", ctx.PlantUMLMode, "browser")
	}
}

func TestWirePlantUMLFetcher_WiresWhenDocHasPlantUML(t *testing.T) {
	doc := astWithElements(ast.NewPlantUMLElement(nativePos(), "sequence", "A->B"))
	ctx := renderer.NewDefaultRenderContext()

	wirePlantUMLFetcher(ctx, doc, GeneratorOptions{RenderMode: "offline-assets"}, t.TempDir())

	if ctx.Fetcher == nil {
		t.Fatal("expected ctx.Fetcher to be wired for a doc with PlantUML")
	}
	if ctx.PlantUMLMode != "offline-assets" {
		t.Errorf("PlantUMLMode = %q, want %q", ctx.PlantUMLMode, "offline-assets")
	}
	if ctx.PlantUMLFormat != "svg" {
		t.Errorf("PlantUMLFormat = %q, want %q", ctx.PlantUMLFormat, "svg")
	}
}

// TestWirePlantUMLFetcher_UsesCustomServerAndFormat cubre el hallazgo de
// code-review sobre PR #56 (--plantuml-server/--plantuml-format): antes de
// este fix, slidelang siempre apuntaba al servidor público plantuml.com sin
// ninguna forma de redirigirlo — necesario en una red air-gapped o con un
// servidor PlantUML propio.
func TestWirePlantUMLFetcher_UsesCustomServerAndFormat(t *testing.T) {
	doc := astWithElements(ast.NewPlantUMLElement(nativePos(), "sequence", "A->B"))
	ctx := renderer.NewDefaultRenderContext()

	wirePlantUMLFetcher(ctx, doc, GeneratorOptions{
		RenderMode:     "offline-assets",
		PlantUMLServer: "http://plantuml.internal.example.com",
		PlantUMLFormat: "png",
	}, t.TempDir())

	if ctx.PlantUMLFormat != "png" {
		t.Errorf("PlantUMLFormat = %q, want %q", ctx.PlantUMLFormat, "png")
	}
	if ctx.Fetcher == nil {
		t.Fatal("expected ctx.Fetcher to be wired")
	}
}

// TestSetupOfflineRenderContext_PlantUMLOnlyDeckSkipsChromium es la prueba
// decisiva del hallazgo Alta de code-review sobre PR #56, análoga a
// TestSetupOfflineRenderContext_NativeDeckSkipsChromium (offline_native_test.go):
// un ChromiumPath deliberadamente roto (e InstallChromium=false) no debe
// impedir un build offline para un deck cuyo único elemento rico es
// PlantUML — si SetupOfflineRenderContext intentara instanciar Chromium
// igual, fallaría con ese path roto; que retorne sin error ES la prueba de
// que Chromium nunca se tocó.
func TestSetupOfflineRenderContext_PlantUMLOnlyDeckSkipsChromium(t *testing.T) {
	g := New(util.NewNoop())
	doc := astWithElements(ast.NewPlantUMLElement(nativePos(), "sequence", "A->B: hola"))
	opts := GeneratorOptions{
		RenderMode:      "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	ctx, cleanup, err := g.SetupOfflineRenderContext(doc, t.TempDir(), opts)
	defer cleanup()
	if err != nil {
		t.Fatalf("expected no error for a PlantUML-only deck even with a broken Chromium path, got: %v", err)
	}
	if ctx.Fetcher == nil {
		t.Fatal("expected ctx.Fetcher (PlantUML) to be wired")
	}
	if ctx.PlantUMLMode != "offline-inline" {
		t.Errorf("PlantUMLMode = %q, want %q", ctx.PlantUMLMode, "offline-inline")
	}
}

// TestSetupOfflineRenderContext_MermaidPlusPlantUMLStillNeedsChromium
// confirma que la presencia de PlantUML no hace que un deck que TAMBIÉN
// trae mermaid se salte Chromium por error — el gate de PlantUML es un OR,
// no reemplaza al de mermaid/chart/map/math.
func TestSetupOfflineRenderContext_MermaidPlusPlantUMLStillNeedsChromium(t *testing.T) {
	g := New(util.NewNoop())
	doc := astWithElements(
		ast.NewMermaidElement(nativePos(), "flowchart", "graph TD; A-->B"),
		ast.NewPlantUMLElement(nativePos(), "sequence", "A->B: hola"),
	)
	opts := GeneratorOptions{
		RenderMode:      "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	_, cleanup, err := g.SetupOfflineRenderContext(doc, t.TempDir(), opts)
	defer cleanup()
	if err == nil {
		t.Fatal("expected an error initializing Chromium for a mermaid+plantuml deck with a broken ChromiumPath — mermaid must still force the Chromium path")
	}
}

// TestSetupOfflineRenderContext_MathOnlyDeckTriesChromiumThenDegrades
// confirma que un deck con SOLO una ecuación SÍ intenta instanciar Chromium
// (MathJax corre vía chromedp) pero, a diferencia de mermaid/chart/map, no
// tumba el build entero si Chromium no está disponible (hallazgo de
// code-review sobre PR #56: agregar MathElement a hasInteractiveElements
// quitó la garantía de "sin Chromium interactivo, nunca se toca Chromium"
// para un deck de solo texto+ecuaciones — antes de #56 ese mismo deck
// construía igual, solo con LaTeX sin tipografiar). El resultado debe ser un
// build exitoso SIN MathFetcher, para que renderOfflineMath caiga a esa
// misma degradación.
func TestSetupOfflineRenderContext_MathOnlyDeckTriesChromiumThenDegrades(t *testing.T) {
	g := New(util.NewNoop())
	doc := astWithElements(ast.NewMathElement(nativePos(), "E = mc^2"))
	opts := GeneratorOptions{
		RenderMode:      "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	ctx, cleanup, err := g.SetupOfflineRenderContext(doc, t.TempDir(), opts)
	defer cleanup()
	if err != nil {
		t.Fatalf("expected a math-only deck to degrade gracefully instead of failing the build when Chromium is unavailable, got: %v", err)
	}
	if ctx.MathFetcher != nil {
		t.Error("expected no MathFetcher wired when Chromium initialization failed — the caller must fall back to raw LaTeX")
	}
}

// TestGeneratePDF_ForcesSVGForPlantUML cubre un hallazgo de code review
// (mismo patrón que el fix equivalente en
// doclang/internal/generator/pdf.go): generatePDF fuerza
// pdfOpts.RenderMode = "offline-inline" para PDF, pero antes de este fix
// nunca tocaba pdfOpts.PlantUMLFormat — el flag --plantuml-format ya
// documentaba "offline-inline and --format pdf always use svg"
// (slidelang/internal/cli/build.go) sin que el código lo cumpliera.
// wirePlantUMLFetcher enruta offline-inline a FetchDiagramInline, que es
// SVG-only por construcción (plantuml_fetcher.go: "if f.format != "svg" {
// return error }") — con --plantuml-format=png el build no fallaba, pero el
// diagrama salía reemplazado por un <div class="plantuml-error"> silencioso
// dentro del PDF. No se puede invocar generatePDF directamente sin un
// Chromium real (mismo límite que los tests de doclang), así que este test
// reproduce la construcción de ctx que generatePDF hace via
// wirePlantUMLFetcher, con pdfOpts.PlantUMLFormat forzado a "svg" como hace
// el código real.
func TestGeneratePDF_ForcesSVGForPlantUML(t *testing.T) {
	plantumlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(`<svg id="plantuml-resolved-diagram"></svg>`))
	}))
	defer plantumlServer.Close()

	doc := astWithElements(ast.NewPlantUMLElement(nativePos(), "sequence", "A->B: hola"))
	ctx := renderer.NewDefaultRenderContext()

	// Mismo pdfOpts que generatePDF arma: RenderMode forzado a
	// offline-inline, PlantUMLFormat forzado a "svg" sin importar lo que el
	// usuario haya pasado con --plantuml-format.
	pdfOpts := GeneratorOptions{
		RenderMode:     "offline-inline",
		PlantUMLServer: plantumlServer.URL,
		PlantUMLFormat: "svg",
	}
	wirePlantUMLFetcher(ctx, doc, pdfOpts, t.TempDir())

	html := renderer.RenderElementToHTML(doc.ContentBlocks[0].Elements[0], nil, ctx)
	if strings.Contains(html, `class="plantuml-error"`) {
		t.Errorf("expected no plantuml-error div when PDF forces svg format, got:\n%s", html)
	}
	if !strings.Contains(html, "plantuml-resolved-diagram") {
		t.Error("expected the pre-resolved SVG from the PlantUML fetcher to be embedded inline")
	}
}

// TestGeneratePDF_PNGFormatBreaksOfflineInlinePlantUML documenta el
// contrato que hace necesario el fix de arriba: FetchDiagramInline
// (plantuml_fetcher.go) rechaza cualquier formato distinto de "svg" — si
// esto alguna vez deja de fallar, offline-inline empezó a aceptar PNG y el
// forzado a "svg" en generatePDF dejó de ser necesario (o el bug volvió si
// nadie actualizó pdf.go a la vez).
func TestGeneratePDF_PNGFormatBreaksOfflineInlinePlantUML(t *testing.T) {
	plantumlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("not-a-real-png"))
	}))
	defer plantumlServer.Close()

	doc := astWithElements(ast.NewPlantUMLElement(nativePos(), "sequence", "A->B: hola"))
	ctx := renderer.NewDefaultRenderContext()

	pdfOpts := GeneratorOptions{
		RenderMode:     "offline-inline",
		PlantUMLServer: plantumlServer.URL,
		PlantUMLFormat: "png",
	}
	wirePlantUMLFetcher(ctx, doc, pdfOpts, t.TempDir())

	html := renderer.RenderElementToHTML(doc.ContentBlocks[0].Elements[0], nil, ctx)
	if !strings.Contains(html, `class="plantuml-error"`) {
		t.Error("expected FetchDiagramInline to reject non-svg format and render a plantuml-error div for offline-inline + png")
	}
}

// TestSetupOfflineRenderContext_MermaidPlusMathStillFailsWithoutChromium
// confirma que la degradación graceful de math-only NO se extiende a un
// deck que TAMBIÉN trae mermaid/chart/map: esos no tienen alternativa
// razonable sin Chromium, así que el build debe seguir fallando duro —
// mismo comportamiento histórico preservado.
func TestSetupOfflineRenderContext_MermaidPlusMathStillFailsWithoutChromium(t *testing.T) {
	g := New(util.NewNoop())
	doc := astWithElements(
		ast.NewMermaidElement(nativePos(), "flowchart", "graph TD; A-->B"),
		ast.NewMathElement(nativePos(), "E = mc^2"),
	)
	opts := GeneratorOptions{
		RenderMode:      "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	_, cleanup, err := g.SetupOfflineRenderContext(doc, t.TempDir(), opts)
	defer cleanup()
	if err == nil {
		t.Fatal("expected an error initializing Chromium for a mermaid+math deck with a broken ChromiumPath — mermaid has no fallback and must still force a hard failure")
	}
}
