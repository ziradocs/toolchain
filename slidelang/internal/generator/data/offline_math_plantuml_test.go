// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"context"
	"errors"
	"fmt"
	htmltemplate "html/template"
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
// assets vs fallo), no el motor MathJax en sí (eso ya lo cubren los tests de
// Mermaid/Chart existentes, que comparten el mismo mecanismo de fetcher).
// calls cuenta invocaciones — usado para probar deduplicación por cache.
type fakeMathFetcher struct {
	svg       string
	assetPath string
	err       error
	calls     int
}

func (f *fakeMathFetcher) FetchInline(_ context.Context, _ string) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.svg, nil
}

func (f *fakeMathFetcher) FetchAndSave(_ context.Context, _ string, _ string) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.assetPath, nil
}

// spyLogger implementa util.Logger grabando los mensajes de Warn (ya
// formateados vía fmt.Sprintf, mismo contrato que ConsoleLogger.Warn) —
// usado para verificar que un fallo de fetch offline se loguea (en vez de
// quedar silencioso) sin necesitar un ConsoleLogger real escribiendo a
// stderr.
type spyLogger struct {
	warns []string
}

func (s *spyLogger) Error(string, ...interface{})           {}
func (s *spyLogger) Info(string, string, ...interface{})    {}
func (s *spyLogger) Debug(string, string, ...interface{})   {}
func (s *spyLogger) Progress(string, string, int)           {}
func (s *spyLogger) Summary(string, map[string]interface{}) {}
func (s *spyLogger) SetLevel(util.LogLevel)                 {}
func (s *spyLogger) Warn(message string, args ...interface{}) {
	s.warns = append(s.warns, fmt.Sprintf(message, args...))
}

// TestRenderOfflineMath_NilFetcherFallsBackSilently cubre el hallazgo de
// code-review sobre PR #56: un fetcher no configurado ya NO debe producir un
// <div class="math-error"> que el template trataría como un pre-render
// EXITOSO (PreRenderedHTML no está vacío → suprime el fallback de LaTeX
// crudo) — debe devolver "" para que template/base.go's
// `{{if .PreRenderedHTML}}` caiga al \[...\] de modo browser, y loguear el
// motivo por separado.
func TestRenderOfflineMath_NilFetcherFallsBackSilently(t *testing.T) {
	ctx := renderer.NewDefaultRenderContext()
	log := &spyLogger{}
	got := renderOfflineMath("x^2", "offline-inline", ctx, log, map[string]htmltemplate.HTML{})
	if got != "" {
		t.Errorf("expected empty PreRenderedHTML so the template falls back to raw LaTeX, got %q", got)
	}
	if len(log.warns) == 0 {
		t.Error("expected the nil-fetcher condition to be logged")
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

	got := renderOfflineMath("x^2 + y^2 = z^2", "offline-inline", ctx, util.NewNoop(), map[string]htmltemplate.HTML{})

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

	got := renderOfflineMath("E = mc^2", "offline-assets", ctx, util.NewNoop(), map[string]htmltemplate.HTML{})

	if !strings.Contains(string(got), `src="assets/equations/deadbeef.svg"`) {
		t.Errorf("expected an <img> pointing at the saved asset, got %q", got)
	}
}

// TestRenderOfflineMath_FetcherErrorFallsBackSilently: un fallo del fetcher
// (Chromium caído, timeout) no debe borrar la ecuación del output ni marcar
// el elemento como "ya renderizado" — debe caer al LaTeX crudo (empty
// PreRenderedHTML) y loguear el error, no reemplazar la ecuación por un
// mensaje permanente sin la fuente.
func TestRenderOfflineMath_FetcherErrorFallsBackSilently(t *testing.T) {
	ctx := renderer.NewDefaultRenderContext()
	ctx.MathFetcher = &fakeMathFetcher{err: errors.New("chromium timeout")}
	log := &spyLogger{}

	got := renderOfflineMath("x^2", "offline-inline", ctx, log, map[string]htmltemplate.HTML{})

	if got != "" {
		t.Errorf("expected empty PreRenderedHTML on fetch error, got %q", got)
	}
	found := false
	for _, w := range log.warns {
		if strings.Contains(w, "chromium timeout") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the fetch error to be logged, got %v", log.warns)
	}
}

// TestRenderOfflineMath_DedupesRepeatedContent cubre el hallazgo de
// code-review sobre PR #56: una ecuación repetida no debe disparar un fetch
// nuevo cada vez — el cache por contenido debe devolver el resultado ya
// calculado sin volver a llamar al fetcher.
func TestRenderOfflineMath_DedupesRepeatedContent(t *testing.T) {
	ctx := renderer.NewDefaultRenderContext()
	fetcher := &fakeMathFetcher{svg: `<svg><path d="M0 0"/></svg>`}
	ctx.MathFetcher = fetcher
	cache := map[string]htmltemplate.HTML{}

	first := renderOfflineMath("E = mc^2", "offline-inline", ctx, util.NewNoop(), cache)
	second := renderOfflineMath("E = mc^2", "offline-inline", ctx, util.NewNoop(), cache)

	if first != second {
		t.Errorf("expected identical cached results, got %q vs %q", first, second)
	}
	if fetcher.calls != 1 {
		t.Errorf("expected exactly 1 fetch call for repeated identical content, got %d", fetcher.calls)
	}
}

// TestRenderOfflinePlantUML_NilFetcherFallsBackSilently: mismo contrato que
// TestRenderOfflineMath_NilFetcherFallsBackSilently, del lado de PlantUML.
func TestRenderOfflinePlantUML_NilFetcherFallsBackSilently(t *testing.T) {
	ctx := renderer.NewDefaultRenderContext()
	log := &spyLogger{}
	got := renderOfflinePlantUML("A->B", "offline-inline", ctx, log, map[string]htmltemplate.HTML{})
	if got != "" {
		t.Errorf("expected empty PreRenderedHTML so the template falls back to the remote-URL diagram, got %q", got)
	}
	if len(log.warns) == 0 {
		t.Error("expected the nil-fetcher condition to be logged")
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

	got := renderOfflinePlantUML("A->B: hola", "offline-inline", ctx, util.NewNoop(), map[string]htmltemplate.HTML{})

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

// TestRenderOfflinePlantUML_InlineStripsHardcodedSize cubre el hallazgo de
// code-review sobre PR #56: el SVG que retorna el servidor PlantUML trae su
// propio style="width:Npx;height:Npx;..." — un atributo style inline gana
// por especificidad sobre la regla CSS .slidelang-plantuml-diagram{max-width
// :100%;height:auto}, así que sin limpiarlo el diagrama queda deformado
// (ancho encogido por la clase, alto fijo al original). preserveAspectRatio
// ="none" también se corrige a un valor que preserva proporción.
func TestRenderOfflinePlantUML_InlineStripsHardcodedSize(t *testing.T) {
	const fakeSVG = `<svg viewBox="0 0 113 159" style="width:113px;height:159px;background:#FFFFFF;" preserveAspectRatio="none"><rect width="10" height="10"/></svg>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fakeSVG))
	}))
	defer server.Close()

	fetcher := chromium.NewPlantUMLFetcher(server.URL, "svg", t.TempDir())
	ctx := renderer.NewDefaultRenderContext()
	ctx.Fetcher = fetcher

	got := renderOfflinePlantUML("A->B", "offline-inline", ctx, util.NewNoop(), map[string]htmltemplate.HTML{})

	if strings.Contains(string(got), "width:113px") || strings.Contains(string(got), "height:159px") {
		t.Errorf("expected the hardcoded pixel size to be stripped so CSS can scale the SVG, got %q", got)
	}
	if !strings.Contains(string(got), "background:#FFFFFF;") {
		t.Errorf("expected the unrelated background style to survive, got %q", got)
	}
	if strings.Contains(string(got), `preserveAspectRatio="none"`) {
		t.Errorf("expected preserveAspectRatio=\"none\" to be replaced with an aspect-ratio-preserving value, got %q", got)
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

	got := renderOfflinePlantUML("A->B: hola", "offline-assets", ctx, util.NewNoop(), map[string]htmltemplate.HTML{})

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

// TestPrepareTemplateData_OfflineMode_FailedFetchStillLeavesContentUsable es
// el test de integración del hallazgo de code-review sobre PR #56: un
// fetcher que siempre falla no debe dejar PreRenderedHTML poblado con un
// mensaje de error — el template debe poder caer a su fallback de modo
// browser (LaTeX crudo / URL remota) en vez de perder la ecuación/diagrama.
func TestPrepareTemplateData_OfflineMode_FailedFetchStillLeavesContentUsable(t *testing.T) {
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
	ctx.MathFetcher = &fakeMathFetcher{err: errors.New("chromium unavailable")}
	// ctx.Fetcher se deja nil a propósito: mismo contrato de fallback.

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "offline-inline", util.NewNoop(), ctx)

	mathEl := got.ContentBlocks[0].Elements[0]
	plantumlEl := got.ContentBlocks[0].Elements[1]

	if mathEl.PreRenderedHTML != "" {
		t.Errorf("expected empty PreRenderedHTML on fetch failure so the template falls back to raw LaTeX, got %q", mathEl.PreRenderedHTML)
	}
	if mathEl.Content != "E = mc^2" {
		t.Errorf("expected Content to still carry the raw LaTeX for the fallback, got %q", mathEl.Content)
	}
	if plantumlEl.PreRenderedHTML != "" {
		t.Errorf("expected empty PreRenderedHTML with no fetcher so the template falls back to the remote-URL diagram, got %q", plantumlEl.PreRenderedHTML)
	}
	if plantumlEl.PlantUMLSVGURL == "" {
		t.Error("expected PlantUMLSVGURL to still be populated for the browser-mode fallback")
	}
}
