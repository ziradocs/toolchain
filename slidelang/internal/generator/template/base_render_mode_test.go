// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"strings"
	"testing"
)

// TestBuildCDNIncludes_BrowserEmitsAllCDN: en modo browser se emiten las 4
// librerías CDN (mermaid/chart.js/leaflet históricas + MathJax, issue #38).
func TestBuildCDNIncludes_BrowserEmitsAllCDN(t *testing.T) {
	tb := NewTemplateBuilder().WithRenderMode("browser")
	got := tb.buildCDNIncludes()
	for _, want := range []string{"cdn.jsdelivr.net/npm/mermaid", "chart.js", "unpkg.com/leaflet", "cdn.jsdelivr.net/npm/mathjax"} {
		if !strings.Contains(got, want) {
			t.Errorf("browser CDN includes missing %q\ngot: %s", want, got)
		}
	}
}

// TestBuildCDNIncludes_OfflineEmitsNothing: en modos offline no se emite ninguna
// librería CDN (el contenido va pre-renderizado) — issue #92.
func TestBuildCDNIncludes_OfflineEmitsNothing(t *testing.T) {
	for _, mode := range []string{"offline-assets", "offline-inline"} {
		tb := NewTemplateBuilder().WithRenderMode(mode)
		if got := tb.buildCDNIncludes(); got != "" {
			t.Errorf("mode %q should emit no CDN includes, got %q", mode, got)
		}
	}
}

// TestBuildCDNIncludes_EmptyRenderModeEmitsCDN: RenderMode vacío == browser.
func TestBuildCDNIncludes_EmptyRenderModeEmitsCDN(t *testing.T) {
	tb := NewTemplateBuilder()
	if !strings.Contains(tb.buildCDNIncludes(), "cdn.jsdelivr.net") {
		t.Error("empty render mode should emit CDN includes like browser")
	}
}
