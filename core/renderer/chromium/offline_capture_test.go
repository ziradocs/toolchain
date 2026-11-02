// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"bytes"
	"context"
	"image/png"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"go.ziradocs.com/core/v2/renderer"
)

// Tests de integración con Chromium real (se saltan sin Chromium o con
// -short, ver requireTestChromiumPath). Las páginas de captura no tienen
// ningún origen de red en su CSP, así que estos renders son los mismos que
// en una máquina sin red: si pasan acá, no dependen del CDN.

// TestCapture_PageThatNeverSignalsFailsWithinTheBound: una página que nunca
// publica listo ni error termina en un error de timeout dentro del plazo,
// en vez de colgar el build (lo que pasaba con un chart sin Chart.js).
func TestCapture_PageThatNeverSignalsFailsWithinTheBound(t *testing.T) {
	r := newTestChromiumRenderer(t)
	if err := r.ensureBrowser(context.Background()); err != nil {
		t.Fatal(err)
	}
	const bound = 2 * time.Second
	runCtx, cancel := r.newCaptureContext(context.Background(), bound)
	defer cancel()

	start := time.Now()
	err := chromedp.Run(runCtx,
		navigateAndSetContent(`<!DOCTYPE html><html><body>never ready</body></html>`),
		waitRenderState(renderStateJS),
	)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("a page that never signals must fail")
	}
	if elapsed > bound+3*time.Second {
		t.Errorf("capture took %s, want about %s", elapsed, bound)
	}
	if got := captureError("chart", bound, context.Background(), err).Error(); !strings.Contains(got, "timed out after 2s") {
		t.Errorf("error does not say it timed out: %s", got)
	}

	// El browser sigue sirviendo después de un plazo vencido.
	if _, err := r.RenderChartToPNG(context.Background(), `{"type":"bar","data":{"labels":["a"],"datasets":[{"data":[1]}]}}`, 200, 150); err != nil {
		t.Fatalf("renderer unusable after a timed-out capture: %v", err)
	}
}

// TestRenderChartToPNG_EmbeddedChartJS: un chart que solo Chart.js dibuja
// (JSON, treemap) sale sin red, y un script que lanza (como lanzaba
// `new Chart` cuando Chart.js no cargaba) falla de inmediato con el
// mensaje, no por timeout.
func TestRenderChartToPNG_EmbeddedChartJS(t *testing.T) {
	r := newTestChromiumRenderer(t)
	for name, cfg := range map[string]string{
		"bar":     `{"type":"bar","data":{"labels":["Ene","Feb"],"datasets":[{"label":"Ventas","data":[10,20]}]}}`,
		"treemap": `{"type":"treemap","data":{"datasets":[{"tree":[6,6,4,3,2,2,1],"label":"Áreas"}]}}`,
	} {
		data, err := r.RenderChartToPNG(context.Background(), cfg, 400, 300)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
			t.Fatalf("%s: not a PNG: %v", name, err)
		}
	}

	start := time.Now()
	_, err := r.RenderChartToPNG(context.Background(), `{"type":"bar","data":notDefined}`, 400, 300)
	if err == nil || !strings.Contains(err.Error(), "Chart.js: notDefined is not defined") {
		t.Fatalf("throwing chart page: want the script error, got %v", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("a throwing chart page took %s to fail; it must not wait for the timeout", time.Since(start))
	}

	if _, err := r.RenderChartToWebP(context.Background(), `{"type":"bar","data":{"labels":["a"],"datasets":[{"data":[1]}]}}`, 200, 150, 80); err != nil {
		t.Fatalf("WebP chart: %v", err)
	}
}

// TestRenderMath_OfflineFontsAndExtensions: fórmulas que piden glifos o
// extensiones que tex-svg.js no trae (y que sin red no salían en absoluto)
// salen con la precarga embebida.
func TestRenderMath_OfflineFontsAndExtensions(t *testing.T) {
	r := newTestChromiumRenderer(t)
	for _, latex := range []string{
		`\int_0^1 x^2\,dx = \frac{1}{3}`,
		`\mathbb{R}^n \to \mathfrak{g}`,      // fuentes dinámicas
		`\text{acción}`,                      // latín acentuado
		`\boldsymbol{\alpha} + \color{red}x`, // extensiones
		`\ce{H2O}`,
		// Los de la revisión de #455: negrita y cursiva acentuadas, cirílico
		// y una fórmula que pide muchos archivos a la vez (antes se agregaba
		// uno por intento y se agotaban los intentos).
		`\textbf{información} \mathbf{é}`,
		`\textbf{\textit{café}}`,
		`\text{Привет}`,
		`\mathbb{R} \mathfrak{g} \mathscr{F} \mathsf{A} \mathtt{B} \text{ñ} \textbf{é} \textit{ç} \text{Ж} \boldsymbol{\alpha} \color{red}{x} \cancel{y} \ce{H2O} \braket{a|b} \SI{3}{m} \xrightarrow{f} \mathcal{O} \bbox[yellow]{z} \upalpha`,
	} {
		svg, err := r.RenderMathToSVGWithTheme(context.Background(), latex, renderer.DiagramThemeColors{})
		if err != nil {
			t.Errorf("%s: %v", latex, err)
			continue
		}
		if !strings.HasPrefix(svg, "<svg") || strings.Contains(svg, "data-mjx-error") {
			t.Errorf("%s: unexpected SVG: %.200s", latex, svg)
		}
	}
}

// TestRenderMath_ErrorsAreReportedFast: un error de la fórmula (o una
// extensión que no se embebe) vuelve como error con el motivo, no como un
// SVG con el error en rojo ni como un timeout.
func TestRenderMath_ErrorsAreReportedFast(t *testing.T) {
	r := newTestChromiumRenderer(t)
	for latex, want := range map[string]string{
		`\frac{`:                        "no equation",
		`x^{2}}`:                        "MathJax:",
		`\href{https://example.com}{x}`: `Extension "html"`,
	} {
		start := time.Now()
		_, err := r.RenderMathToSVGWithTheme(context.Background(), latex, renderer.DiagramThemeColors{})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want an error containing %q, got %v", latex, want, err)
		}
		if time.Since(start) > 10*time.Second {
			t.Errorf("%s took %s to fail; it must not wait for the timeout", latex, time.Since(start))
		}
	}
}

// TestRenderMathToFittedPNG: el PNG es la caja de la ecuación a 2x, no un
// contenedor fijo mayormente blanco.
func TestRenderMathToFittedPNG(t *testing.T) {
	r := newTestChromiumRenderer(t)
	data, err := r.RenderMathToFittedPNGWithTheme(context.Background(), `E = mc^2`, renderer.DiagramThemeColors{})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	// E = mc^2 a 16 px CSS mide unos 90x25 px CSS; a 2x y con el margen,
	// bastante menos que la página y bastante más que nada.
	if cfg.Width < 100 || cfg.Width > 600 || cfg.Height < 30 || cfg.Height > 200 {
		t.Errorf("fitted PNG is %dx%d, want roughly the equation's box at 2x", cfg.Width, cfg.Height)
	}
}

// TestRenderMath_SafeFiltersExternalURLs: ui/safe va cargado, así que un
// estilo del autor con una URL externa (\bbox[background:url(...)]) no
// llega al SVG que termina en el HTML offline o en el PDF.
func TestRenderMath_SafeFiltersExternalURLs(t *testing.T) {
	r := newTestChromiumRenderer(t)
	for _, latex := range []string{
		`\bbox[background:url(http://example.com/x.png)]{x}`,
		`\bbox[border:1px solid red; background-image:url(https://example.com/y)]{y}`,
	} {
		svg, err := r.RenderMathToSVGWithTheme(context.Background(), latex, renderer.DiagramThemeColors{})
		if err != nil {
			t.Fatalf("%s: %v", latex, err)
		}
		// El LaTeX fuente queda como texto en data-latex y en los
		// data-semantic-*; eso no se pide a ningún lado. Lo que no puede
		// quedar es la URL en un atributo que el navegador sí resuelve.
		rendered := regexp.MustCompile(`data-[a-z-]+="[^"]*"`).ReplaceAllString(svg, "")
		if strings.Contains(rendered, "example.com") {
			t.Errorf("%s: external URL reached a rendered attribute: %s", latex, rendered)
		}
	}
}
