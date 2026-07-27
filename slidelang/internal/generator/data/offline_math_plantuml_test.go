// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
)

// fakeMathFetcher implementa renderer.MathFetcher sin tocar Chromium/red —
// solo se necesita para probar el DESPACHO de renderOfflineMath (inline vs
// assets vs error), no el motor MathJax en sí (eso ya lo cubren los tests de
// Mermaid/Chart existentes, que comparten el mismo mecanismo de fetcher).
type fakeMathFetcher struct {
	svg       string
	assetPath string
	err       error
}

func (f *fakeMathFetcher) FetchInline(_ context.Context, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.svg, nil
}

func (f *fakeMathFetcher) FetchAndSave(_ context.Context, _ string, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.assetPath, nil
}

// TestRenderOfflineMath_NilFetcherProducesErrorDiv cubre el caso que
// hallazgo de code-review sobre PR #56 dejaba roto: sin este guard, un ctx
// mal cableado (MathFetcher nil) habría hecho panic contra un puntero nil en
// vez de degradar con un mensaje accionable — mismo criterio que
// core/renderer/html.go's renderMathOfflineInline.
func TestRenderOfflineMath_NilFetcherProducesErrorDiv(t *testing.T) {
	ctx := renderer.NewDefaultRenderContext()
	got := renderOfflineMath("x^2", "offline-inline", ctx)
	if !strings.Contains(string(got), "slidelang-math-error") {
		t.Errorf("expected an error div, got %q", got)
	}
}

// TestRenderOfflineMath_InlineWrapsFetchedSVG confirma el hallazgo Alta del
// review: en offline-inline, el SVG pre-renderizado (no LaTeX crudo sin
// tipografiar) queda embebido dentro de un div con clase propia de
// slidelang, para que math.css lo estilice sin depender de las clases
// "math-*" de core.
func TestRenderOfflineMath_InlineWrapsFetchedSVG(t *testing.T) {
	ctx := renderer.NewDefaultRenderContext()
	ctx.MathFetcher = &fakeMathFetcher{svg: `<svg><path d="M0 0"/></svg>`}

	got := renderOfflineMath("x^2 + y^2 = z^2", "offline-inline", ctx)

	if !strings.Contains(string(got), `class="slidelang-math-diagram slidelang-math-inline"`) {
		t.Errorf("expected the slidelang math wrapper class, got %q", got)
	}
	if !strings.Contains(string(got), `<path d="M0 0"/>`) {
		t.Errorf("expected the fetched SVG body inline, got %q", got)
	}
	if strings.Contains(string(got), `\[`) {
		t.Errorf("offline-inline output must not contain raw LaTeX delimiters, got %q", got)
	}
}

// TestRenderOfflineMath_AssetsModeReferencesSavedFile cubre --render-mode
// offline-assets: el resultado debe ser un <img> apuntando al path relativo
// que devuelve FetchAndSave, con el prefijo "assets/" que
// core/renderer/html.go's renderMathOfflineAssets también antepone (el
// fetcher retorna la ruta relativa a ese subdirectorio, no al output root).
func TestRenderOfflineMath_AssetsModeReferencesSavedFile(t *testing.T) {
	ctx := renderer.NewDefaultRenderContext()
	ctx.OutputDir = "out"
	ctx.MathFetcher = &fakeMathFetcher{assetPath: "equations/deadbeef.svg"}

	got := renderOfflineMath("E = mc^2", "offline-assets", ctx)

	if !strings.Contains(string(got), `src="assets/equations/deadbeef.svg"`) {
		t.Errorf("expected an <img> pointing at the saved asset, got %q", got)
	}
}

// TestRenderOfflineMath_FetcherErrorProducesErrorDiv: un fallo del fetcher
// (Chromium caído, timeout) no debe hacer que el elemento desaparezca del
// output ni que el build entero falle — degrada a un mensaje visible, mismo
// contrato que Mermaid/Chart/Map ya tienen.
func TestRenderOfflineMath_FetcherErrorProducesErrorDiv(t *testing.T) {
	ctx := renderer.NewDefaultRenderContext()
	ctx.MathFetcher = &fakeMathFetcher{err: errors.New("chromium timeout")}

	got := renderOfflineMath("x^2", "offline-inline", ctx)

	if !strings.Contains(string(got), "slidelang-math-error") || !strings.Contains(string(got), "chromium timeout") {
		t.Errorf("expected an error div naming the failure, got %q", got)
	}
}

// TestRenderOfflinePlantUML_NilFetcherProducesErrorDiv: mismo contrato que
// TestRenderOfflineMath_NilFetcherProducesErrorDiv, del lado de PlantUML.
func TestRenderOfflinePlantUML_NilFetcherProducesErrorDiv(t *testing.T) {
	ctx := renderer.NewDefaultRenderContext()
	got := renderOfflinePlantUML("A->B", "offline-inline", ctx)
	if !strings.Contains(string(got), "slidelang-plantuml-error") {
		t.Errorf("expected an error div, got %q", got)
	}
}

// TestRenderOfflinePlantUML_InlineEmbedsSVGWithoutRemoteURL es el test que,
// según el hallazgo Alta de code-review sobre PR #56, habría atrapado la
// regresión original: en offline-inline el diagrama debe llegar como SVG
// embebido, nunca como referencia a un servidor remoto (plantuml.com), que
// es justo lo que rompía el modo "autocontenido" en PDF/offline-inline. Usa
// un *chromium.PlantUMLFetcher REAL apuntando a un httptest.Server local (no
// un fake casero) para probar de verdad el mismo tipo concreto que produce
// el binario, sin red real ni Chromium (el fetcher de PlantUML es HTTP puro).
func TestRenderOfflinePlantUML_InlineEmbedsSVGWithoutRemoteURL(t *testing.T) {
	const fakeSVG = `<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(fakeSVG))
	}))
	defer server.Close()

	fetcher := chromium.NewPlantUMLFetcher(server.URL, "svg", t.TempDir())
	ctx := renderer.NewDefaultRenderContext()
	ctx.Fetcher = fetcher

	got := renderOfflinePlantUML("A->B: hola", "offline-inline", ctx)

	if !strings.Contains(string(got), `class="slidelang-plantuml-diagram slidelang-plantuml-inline"`) {
		t.Errorf("expected the slidelang plantuml wrapper class, got %q", got)
	}
	if !strings.Contains(string(got), `<rect width="10" height="10"/>`) {
		t.Errorf("expected the fetched SVG body inline, got %q", got)
	}
	if strings.Contains(string(got), "plantuml.com") {
		t.Errorf("offline-inline output must never reference a remote PlantUML server, got %q", got)
	}
}

// TestRenderOfflinePlantUML_AssetsModeSavesFileAndReferencesIt cubre
// --render-mode offline-assets con el mismo fetcher real: el archivo debe
// existir de verdad en outputDir/assets/diagrams, y el <img> debe apuntar a
// esa ruta relativa (a diferencia de Math, PlantUMLFetcher.FetchDiagramToAssets
// ya retorna el path con el prefijo "assets/diagrams/" incluido).
func TestRenderOfflinePlantUML_AssetsModeSavesFileAndReferencesIt(t *testing.T) {
	const fakeSVG = `<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(fakeSVG))
	}))
	defer server.Close()

	outputDir := t.TempDir()
	fetcher := chromium.NewPlantUMLFetcher(server.URL, "svg", outputDir)
	ctx := renderer.NewDefaultRenderContext()
	ctx.Fetcher = fetcher

	got := renderOfflinePlantUML("A->B: hola", "offline-assets", ctx)

	if !strings.Contains(string(got), `src="assets/diagrams/plantuml_`) {
		t.Errorf("expected an <img> pointing at assets/diagrams/, got %q", got)
	}

	entries, err := os.ReadDir(filepath.Join(outputDir, "assets", "diagrams"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one saved diagram file, err=%v entries=%v", err, entries)
	}
}

// TestPrepareTemplateData_OfflineMode_PlantUMLAndMathGetPreRendered es el
// test de integración a nivel de PrepareTemplateDataWithRenderMode: confirma
// que, con un ctx cuyos fetchers funcionan, un deck offline con Math Y
// PlantUML termina con PreRenderedHTML poblado para ambos — el campo que
// template/base.go's ramas "math"/"plantuml" prefieren sobre el markup de
// modo browser (issue de code-review sobre PR #56).
func TestPrepareTemplateData_OfflineMode_PlantUMLAndMathGetPreRendered(t *testing.T) {
	const fakeSVG = `<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(fakeSVG))
	}))
	defer server.Close()

	astDoc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{{
			BlockType: "content",
			Elements: []ast.Element{
				&ast.MathElement{Content: "E = mc^2"},
				&ast.PlantUMLElement{DiagramType: "sequence", Content: "A->B: hola"},
			},
		}},
	}

	ctx := renderer.NewDefaultRenderContext()
	ctx.MathFetcher = &fakeMathFetcher{svg: `<svg><path d="M1 1"/></svg>`}
	ctx.Fetcher = chromium.NewPlantUMLFetcher(server.URL, "svg", t.TempDir())

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "offline-inline", util.NewNoop(), ctx)

	mathEl := got.ContentBlocks[0].Elements[0]
	plantumlEl := got.ContentBlocks[0].Elements[1]

	if mathEl.PreRenderedHTML == "" {
		t.Error("expected Math element to have PreRenderedHTML set in offline mode")
	}
	if plantumlEl.PreRenderedHTML == "" {
		t.Error("expected PlantUML element to have PreRenderedHTML set in offline mode")
	}
	if strings.Contains(string(plantumlEl.PreRenderedHTML), "plantuml.com") {
		t.Errorf("PlantUML PreRenderedHTML must not reference a remote server, got %q", plantumlEl.PreRenderedHTML)
	}
}
