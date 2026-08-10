// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// requireTestChromiumPath localiza un binario de Chromium/Chrome real para
// los tests de este archivo, que ejercitan el ciclo de vida del browser (no
// se pueden simular sin un proceso de Chrome de verdad). Se salta el test en
// vez de fallarlo si no hay ninguno disponible — mismo criterio que el resto
// de este paquete usa para condiciones de entorno que no siempre se dan
// (chromium_manager_test.go, chromium_installer_test.go).
//
// ZIRADOCS_TEST_CHROMIUM_PATH permite apuntar a un binario específico (p.
// ej. un chrome-headless-shell cacheado por Playwright, que Detect() no
// encuentra por default porque no vive en ninguna de las rutas comunes del
// sistema operativo).
func requireTestChromiumPath(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping chromium integration test in -short mode")
	}
	if p := os.Getenv("ZIRADOCS_TEST_CHROMIUM_PATH"); p != "" {
		return p
	}
	mgr := newChromiumManager("", false, noopChromiumLogger{}, DefaultChromiumBrand)
	path, err := mgr.Detect()
	if err != nil {
		t.Skipf("no chromium available for this integration test (set ZIRADOCS_TEST_CHROMIUM_PATH to force one): %v", err)
	}
	return path
}

func newTestChromiumRenderer(t *testing.T) *ChromiumRenderer {
	t.Helper()
	path := requireTestChromiumPath(t)
	r, err := NewChromiumRenderer(context.Background(), path, false, noopChromiumLogger{})
	if err != nil {
		t.Fatalf("NewChromiumRenderer failed: %v", err)
	}
	t.Cleanup(r.Close)
	return r
}

// TestChromiumRenderer_MermaidThenPDF_Issue114 es el repro literal del
// issue #114: un mermaid pre-renderizado a SVG, seguido de un PDF, sobre el
// mismo *ChromiumRenderer (el patrón real de slidelang's pdf.go, que
// construye un único renderer y lo pasa tanto al MermaidFetcher como a
// RenderHTMLToPDF). Antes del fix, la primera llamada (RenderMermaidToSVG)
// mataba el proceso de Chrome al retornar — su `defer cancel()` corría sobre
// el mismo *chromedp.Context que r.ctx — y el watchdog LostConnection de
// chromedp cancelaba r.ctx mismo, así que la segunda llamada
// (RenderHTMLToPDF) fallaba con "context canceled" el 100% de las veces.
func TestChromiumRenderer_MermaidThenPDF_Issue114(t *testing.T) {
	r := newTestChromiumRenderer(t)

	svg, err := r.RenderMermaidToSVG(context.Background(), "graph TD\nA[Inicio] --> B[Fin]")
	if err != nil {
		t.Fatalf("first RenderMermaidToSVG failed: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatalf("expected svg content from RenderMermaidToSVG, got: %s", svg)
	}

	outPath := filepath.Join(t.TempDir(), "out.pdf")
	err = r.RenderHTMLToPDF(context.Background(), "<html><body><h1>hola</h1></body></html>", outPath, DefaultPDFOptions())
	if err != nil {
		t.Fatalf("RenderHTMLToPDF after a prior RenderMermaidToSVG on the same renderer must not fail (issue #114), got: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("expected a non-empty PDF at %s, stat err: %v", outPath, err)
	}

	if err := r.ctx.Err(); err != nil {
		t.Fatalf("renderer's long-lived context should still be alive after two sequential renders, got: %v", err)
	}
}

// TestChromiumRenderer_TwoSequentialMermaidRenders cubre la corrupción
// silenciosa observada al reproducir el issue #114 contra HEAD: dos
// diagramas mermaid en un mismo build offline-inline (sin PDF de por medio)
// — el segundo fallaba con "context canceled" por el mismo mecanismo, pero
// el build seguía terminando con exit 0 y "0 warnings", horneando el div de
// error directamente en el HTML entregado.
func TestChromiumRenderer_TwoSequentialMermaidRenders(t *testing.T) {
	r := newTestChromiumRenderer(t)

	svg1, err := r.RenderMermaidToSVG(context.Background(), "graph TD\nA-->B")
	if err != nil {
		t.Fatalf("first RenderMermaidToSVG failed: %v", err)
	}
	svg2, err := r.RenderMermaidToSVG(context.Background(), "graph TD\nX-->Y-->Z")
	if err != nil {
		t.Fatalf("second RenderMermaidToSVG on the same renderer must succeed too, got: %v", err)
	}

	if !strings.Contains(svg1, "<svg") || !strings.Contains(svg2, "<svg") {
		t.Fatalf("expected both renders to produce svg content, got svg1=%q svg2=%q", svg1, svg2)
	}
}

// TestChromiumRenderer_CallerCancelDoesNotPoisonBrowser cubre el efecto
// colateral que el fix de ensureBrowser también corrige: antes, cancelar el
// ctx puntual de una llamada individual (p. ej. RenderMapToPNG's timeout
// interno, o un caller que aborta un request) podía matar el browser
// entero si esa llamada resultaba ser la PRIMERA operación de Chromium del
// proceso — porque chromedp ata la alocación del browser al ctx de su
// primer Run. Con ensureBrowser() alocando siempre contra r.ctx antes de
// que exista cualquier ctx puntual, cancelar una llamada individual solo
// debe cerrar su propio tab.
func TestChromiumRenderer_CallerCancelDoesNotPoisonBrowser(t *testing.T) {
	r := newTestChromiumRenderer(t)

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // ya cancelado antes de invocar

	if _, err := r.RenderMermaidToSVG(canceledCtx, "graph TD\nA-->B"); err == nil {
		t.Fatal("expected an error rendering with an already-canceled caller ctx")
	}

	if err := r.ctx.Err(); err != nil {
		t.Fatalf("a caller-canceled per-call context must not cancel the renderer's long-lived browser context, got: %v", err)
	}

	svg, err := r.RenderMermaidToSVG(context.Background(), "graph TD\nA-->B")
	if err != nil {
		t.Fatalf("renderer should still work after a caller-canceled call, got: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatalf("expected svg content, got: %s", svg)
	}
}
