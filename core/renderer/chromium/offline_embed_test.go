// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"crypto/sha512"
	"encoding/base64"
	"io/fs"
	"path"
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
		"mathjax ui/safe":       embeddedMathJaxSafeJS,
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

// TestNeedsFullPreload fija la regla de reintento: solo se repite por un
// archivo que está embebido.
func TestNeedsFullPreload(t *testing.T) {
	for msg, want := range map[string]bool{
		"MathJax: Error: dynamic file 'double-struck' failed to load": true,
		"MathJax: Error: dynamic file 'cyrillic' failed to load":      true,
		"MathJax: Error: dynamic file 'no-such-file' failed to load":  false,
		`MathJax: Extension "color" failed to load`:                   true,
		`MathJax: Extension "html" failed to load`:                    false, // excluido a propósito
		`MathJax: Extension "texhtml" failed to load`:                 false, // excluido a propósito
		"MathJax: Missing close brace":                                false, // error del autor
	} {
		if got := needsFullPreload(msg); got != want {
			t.Errorf("needsFullPreload(%q) = %v, want %v", msg, got, want)
		}
	}
}

// TestMathPreload_Scripts: la precarga completa inyecta cada archivo
// embebido y lo marca como cargado con el nombre que pediría el loader de
// MathJax; la vacía no agrega nada.
func TestMathPreload_Scripts(t *testing.T) {
	got := mathPreload{all: true}.scripts()
	for _, dir := range []struct{ dir, prefix string }{
		{mathJaxExtensionDir, "[tex]/"},
		{mathJaxFontDir, "[mathjax-newcm]/svg/dynamic/"},
	} {
		entries, err := mathJaxAssets.ReadDir(dir.dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := dir.prefix + strings.TrimSuffix(e.Name(), ".js")
			if !strings.Contains(got, `MathJax.loader.preLoaded("`+name+`")`) {
				t.Errorf("full preload misses %s", name)
			}
		}
	}
	if (mathPreload{}).scripts() != "" {
		t.Error("an empty preload must add nothing to the page")
	}
}

// TestEmbeddedMathFonts_AreTheWholeDynamicSet: los 40 archivos dinámicos de
// la fuente, para que ningún alfabeto que el modo browser sí carga falle sin
// red (antes eran 11 y \text{Привет} fallaba).
func TestEmbeddedMathFonts_AreTheWholeDynamicSet(t *testing.T) {
	entries, err := mathJaxAssets.ReadDir(mathJaxFontDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 40 {
		t.Errorf("embedded %d dynamic font files, want the 40 of @mathjax/mathjax-newcm-font 4.1.3", len(entries))
	}
	for _, e := range entries {
		data, _ := mathJaxAssets.ReadFile(path.Join(mathJaxFontDir, e.Name()))
		if !strings.Contains(string(data), `,"4.1.3","dynamic-font")`) {
			t.Errorf("%s is not from the 4.1.3 font package", e.Name())
		}
	}
}

// TestMathPages_LoadSafe: toda página de math carga ui/safe.
func TestMathPages_LoadSafe(t *testing.T) {
	for _, html := range []string{buildMathSVGHTML(`x`), buildMathPNGHTML(`x`, 100, 100)} {
		if !strings.Contains(html, embeddedMathJaxSafeScriptTag) {
			t.Error("math page does not load ui/safe")
		}
	}
	if !strings.Contains(embeddedMathJaxSafeJS, `checkVersion("ui/safe","4.1.3"`) {
		t.Error("ui/safe.js is not the 4.1.3 component")
	}
}
