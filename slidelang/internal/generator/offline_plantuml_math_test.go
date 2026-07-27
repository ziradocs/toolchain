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

	wirePlantUMLFetcher(ctx, doc, "offline-inline", t.TempDir())

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

	wirePlantUMLFetcher(ctx, doc, "offline-assets", t.TempDir())

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

// TestSetupOfflineRenderContext_MathOnlyDeckStillNeedsChromium confirma el
// otro lado del fix a hasInteractiveElements: un deck con SOLO una ecuación
// debe seguir intentando instanciar Chromium de verdad (MathJax corre vía
// chromedp) — antes del fix, este caso ni siquiera llegaba a intentarlo.
func TestSetupOfflineRenderContext_MathOnlyDeckStillNeedsChromium(t *testing.T) {
	g := New(util.NewNoop())
	doc := astWithElements(ast.NewMathElement(nativePos(), "E = mc^2"))
	opts := GeneratorOptions{
		RenderMode:      "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	_, cleanup, err := g.SetupOfflineRenderContext(doc, t.TempDir(), opts)
	defer cleanup()
	if err == nil {
		t.Fatal("expected an error initializing Chromium for a math-only deck with a broken ChromiumPath — math must force the Chromium path (issue de code-review sobre PR #56)")
	}
}
