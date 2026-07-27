// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
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
	if !hasInteractiveElements(doc) {
		t.Error("expected hasInteractiveElements(doc-with-math) = true")
	}
}

// TestHasInteractiveElements_PlantUMLOnlyDoesNotRequireChromium: PlantUML es
// HTTP puro (chromium.PlantUMLFetcher no usa chromedp), así que NO debe
// entrar al gate que decide si hace falta arrancar Chromium — eso lo cubre
// hasPlantUMLElements, un chequeo aparte.
func TestHasInteractiveElements_PlantUMLOnlyDoesNotRequireChromium(t *testing.T) {
	doc := astWithElements(ast.NewPlantUMLElement(nativePos(), "sequence", "A->B: hola"))
	if hasInteractiveElements(doc) {
		t.Error("expected hasInteractiveElements(doc-with-only-plantuml) = false — PlantUML doesn't need Chromium")
	}
}

// TestHasChromiumOnlyElements_ExcludesMath cubre el split que separó
// hasInteractiveElements en hasChromiumOnlyElements (mermaid/chart/map, sin
// degradación razonable) y hasMathElements (con degradación a LaTeX crudo) —
// hallazgo de code-review sobre PR #56.
func TestHasChromiumOnlyElements_ExcludesMath(t *testing.T) {
	mathOnly := astWithElements(ast.NewMathElement(nativePos(), "E = mc^2"))
	if hasChromiumOnlyElements(mathOnly) {
		t.Error("expected hasChromiumOnlyElements(doc-with-only-math) = false — math has a text fallback")
	}

	mermaidDoc := astWithElements(ast.NewMermaidElement(nativePos(), "flowchart", "graph TD; A-->B"))
	if !hasChromiumOnlyElements(mermaidDoc) {
		t.Error("expected hasChromiumOnlyElements(doc-with-mermaid) = true")
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
