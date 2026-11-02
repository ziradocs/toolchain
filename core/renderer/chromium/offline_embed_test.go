// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"crypto/sha512"
	"encoding/base64"
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// assertMatchesSRI: el bundle embebido es byte a byte el que el HTML en modo
// browser carga del CDN con ese tag. Un bump de un lado sin el otro rompe
// el test en vez de divergir en silencio.
func assertMatchesSRI(t *testing.T, name, tag, embedded string) {
	t.Helper()
	m := regexp.MustCompile(`integrity="sha384-([^"]+)"`).FindStringSubmatch(tag)
	if m == nil {
		t.Fatalf("%s: no sha384 integrity in tag: %s", name, tag)
	}
	sum := sha512.Sum384([]byte(embedded))
	if got := base64.StdEncoding.EncodeToString(sum[:]); got != m[1] {
		t.Errorf("%s: embedded sha384 = %s, CDN tag pins %s", name, got, m[1])
	}
}

func TestEmbeddedChartJS_MatchesCDNIntegrity(t *testing.T) {
	assertMatchesSRI(t, "chart.js", renderer.ChartJSCDNScriptTag, embeddedChartJS)
	assertMatchesSRI(t, "chartjs-chart-treemap", renderer.ChartJSTreemapCDNScriptTag, embeddedChartJSTreemap)
}

func TestEmbeddedMathJax_MatchesCDNIntegrity(t *testing.T) {
	assertMatchesSRI(t, "mathjax", renderer.MathCDNScriptTag, embeddedMathJaxJS)
}

// TestEmbeddedBundles_HaveNoScriptClose: los bundles van inline dentro de
// <script>; un "</script" literal cerraría el elemento antes de tiempo.
func TestEmbeddedBundles_HaveNoScriptClose(t *testing.T) {
	bundles := map[string]string{
		"chart.js":              embeddedChartJS,
		"chartjs-chart-treemap": embeddedChartJSTreemap,
		"mathjax":               embeddedMathJaxJS,
	}
	err := fs.WalkDir(mathJaxAssets, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := mathJaxAssets.ReadFile(p)
		if err != nil {
			return err
		}
		bundles[p] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, js := range bundles {
		if strings.Contains(strings.ToLower(js), "</script") {
			t.Errorf("%s contains </script; the inline tag relies on the escape in inlineScriptTag", name)
		}
	}
}

// TestCaptureCSPs_AllowNoNetwork: ninguna página de captura de chart o math
// puede pedir nada a la red.
func TestCaptureCSPs_AllowNoNetwork(t *testing.T) {
	for name, csp := range map[string]string{"chart": chartRenderCSP, "math": mathRenderCSP} {
		if strings.Contains(csp, "https:") || strings.Contains(csp, "http:") {
			t.Errorf("%s CSP allows a network origin: %s", name, csp)
		}
	}
}

// TestChartAndMathPages_NeedNoNetwork: las páginas que rasteriza Chromium
// llevan los bundles inline, así que DOCX y PDF con charts y ecuaciones
// funcionan sin conexión. Antes cargaban Chart.js y MathJax de jsDelivr y,
// sin red, la captura se quedaba esperando para siempre.
func TestChartAndMathPages_NeedNoNetwork(t *testing.T) {
	pages := map[string]struct {
		html, csp string
		inline    []string
	}{
		"chart":    {buildChartHTML(`{"type":"bar","data":{}}`, 400, 300, ""), chartRenderCSP, []string{embeddedChartScriptTags}},
		"math-svg": {buildMathSVGHTML(`x^2`), mathRenderCSP, []string{embeddedMathJaxScriptTag}},
		"math-png": {buildMathPNGHTML(`x^2`, 800, 200), mathRenderCSP, []string{embeddedMathJaxScriptTag}},
	}
	for name, p := range pages {
		if strings.Contains(p.html, "cdn.jsdelivr.net/npm/chart.js") || strings.Contains(p.html, `<script src=`) {
			t.Errorf("%s page still loads a script from the network", name)
		}
		for _, tag := range p.inline {
			if !strings.Contains(p.html, tag) {
				t.Errorf("%s page does not inline the embedded bundle", name)
			}
		}
		if !strings.Contains(p.html, `content="`+p.csp+`"`) {
			t.Errorf("%s page does not use its no-network CSP", name)
		}
	}
}

// TestChartPage_ReportsScriptErrors: la página de chart publica el error de
// Chart.js en vez de quedarse sin señal (lo que colgaba la captura).
func TestChartPage_ReportsScriptErrors(t *testing.T) {
	html := buildChartHTML(`{"type":"bar","data":{}}`, 400, 300, "")
	for _, want := range []string{"window.__renderError = 'Chart.js: '", "window.__renderReady = true", renderErrorHookJS} {
		if !strings.Contains(html, want) {
			t.Errorf("chart page is missing %q", want)
		}
	}
}

// TestLeafletPage_ReportsScriptErrors: sin red Leaflet no carga y `L.map`
// lanza; el hook lo publica y la captura del mapa corta de inmediato.
func TestLeafletPage_ReportsScriptErrors(t *testing.T) {
	r := &ChromiumRenderer{}
	if html := r.generateLeafletHTML(renderer.MapConfig{Zoom: 5}, 800, 600); !strings.Contains(html, renderErrorHookJS) {
		t.Error("map page does not install the render error hook")
	}
}

// TestMathPreload_WithMissing fija la regla de reintento: solo se reintenta
// por un archivo embebido que no estaba ya precargado.
func TestMathPreload_WithMissing(t *testing.T) {
	var p mathPreload
	next, ok := p.withMissing("MathJax: Error: dynamic file 'double-struck' failed to load")
	if !ok || len(next.fonts) != 1 || next.fonts[0] != "double-struck" {
		t.Fatalf("missing font: got %+v, %v", next, ok)
	}
	if _, ok := next.withMissing("MathJax: Error: dynamic file 'double-struck' failed to load"); ok {
		t.Error("a font already preloaded must not trigger another attempt")
	}
	next, ok = next.withMissing(`MathJax: Extension "color" failed to load`)
	if !ok || len(next.extensions) != 1 || next.extensions[0] != "color" || len(next.fonts) != 1 {
		t.Fatalf("missing extension: got %+v, %v", next, ok)
	}
	for _, msg := range []string{
		"MathJax: Error: dynamic file 'cyrillic' failed to load", // no embebido
		`MathJax: Extension "html" failed to load`,               // excluido a propósito
		`MathJax: Extension "texhtml" failed to load`,            // excluido a propósito
		"MathJax: Missing close brace",                           // error del autor
	} {
		if _, ok := p.withMissing(msg); ok {
			t.Errorf("%q must not trigger a retry", msg)
		}
	}
	if len(p.fonts) != 0 || len(p.extensions) != 0 {
		t.Error("withMissing must not mutate the receiver")
	}
}

// TestMathPreload_Scripts: la precarga inyecta cada archivo y lo marca como
// cargado con el nombre que pediría el loader de MathJax.
func TestMathPreload_Scripts(t *testing.T) {
	got := mathPreload{extensions: []string{"color"}, fonts: []string{"double-struck"}}.scripts()
	for _, want := range []string{
		`MathJax.loader.preLoaded("[tex]/color")`,
		`MathJax.loader.preLoaded("[mathjax-newcm]/svg/dynamic/double-struck")`,
		`MathJax.loader.checkVersion("[tex]/color"`,
		`MathJax.loader.checkVersion("[mathjax-newcm]/svg/dynamic/double-struck"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("preload scripts missing %s", want)
		}
	}
	if (mathPreload{}).scripts() != "" {
		t.Error("an empty preload must add nothing to the page")
	}
}
