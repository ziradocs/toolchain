// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// bareInlineScriptPattern matchea un <script> SIN nonce y sin src= (externo,
// no lo necesita) ni type="application/json" (no ejecutable) — usado para
// confirmar que ningún <script> inline se coló sin el nonce del build.
// <style> NO se chequea: style-src usa 'unsafe-inline' a propósito (ver
// renderer.BuildDefaultOutputCSP — Mermaid inyecta su CSS de tema en
// runtime sin nonce y sin forma de asignarle uno, verificado en vivo).
var bareInlineScriptPattern = regexp.MustCompile(`<script(\s[^>]*)?>`)

func assertNoUnnoncedInlineScripts(t *testing.T, html string) {
	t.Helper()
	for _, m := range bareInlineScriptPattern.FindAllString(html, -1) {
		if strings.Contains(m, "src=") || strings.Contains(m, `type="application/json"`) {
			continue
		}
		if !strings.Contains(m, "nonce=") {
			t.Errorf("found an inline <script> with no nonce and no src=: %s", m)
		}
	}
}

// TestBuild_EmitsCSPWithMatchingNonces cubre BA-10: la salida HTML por
// defecto de slidelang (modo EmbedAssets, que embebe <style>/<script>
// inline) debe llevar una CSP con nonce, y ESE MISMO nonce debe estar en el
// <script> inline (no en <style>: ver bareInlineScriptPattern).
func TestBuild_EmitsCSPWithMatchingNonces(t *testing.T) {
	tb := NewTemplateBuilder().
		WithTheme("default").
		WithModules([]string{"core", "navigation", "mermaid", "charts", "maps"}).
		WithRequiredElements([]string{"text", "code", "images"}).
		WithEmbedAssets(true)

	html := tb.Build()

	cspMatch := regexp.MustCompile(`Content-Security-Policy" content="([^"]*)"`).FindStringSubmatch(html)
	if cspMatch == nil {
		t.Fatal("expected output to contain a Content-Security-Policy meta tag")
	}
	csp := cspMatch[1]

	nonceMatch := regexp.MustCompile(`'nonce-([A-Za-z0-9+/=]+)'`).FindStringSubmatch(csp)
	if nonceMatch == nil {
		t.Fatalf("expected CSP to contain a 'nonce-...' source, got: %s", csp)
	}
	nonce := nonceMatch[1]

	nonceAttrCount := strings.Count(html, fmt.Sprintf("nonce=%q", nonce))
	// EmbedAssets(true) siempre produce exactamente un <script> embebido con
	// nonce (ver Build()) — el <style> embebido ya no lleva nonce.
	if nonceAttrCount != 1 {
		t.Errorf("expected exactly 1 nonce attribute (embedded <script>), got %d", nonceAttrCount)
	}

	assertNoUnnoncedInlineScripts(t, html)
}

// TestBuild_SeparateFilesMode_OmitsInlineTagsButStillEmitsCSP cubre el modo
// no-EmbedAssets (CSS/JS como archivos separados, sin bloques inline en el
// HTML): la CSP sigue emitiéndose (protege igual los <script src>/<link>
// externos vía la allowlist de hosts) pero no debería haber ningún <script>
// inline que necesite nonce.
func TestBuild_SeparateFilesMode_OmitsInlineTagsButStillEmitsCSP(t *testing.T) {
	tb := NewTemplateBuilder().
		WithTheme("default").
		WithEmbedAssets(false)

	html := tb.Build()

	if !strings.Contains(html, "Content-Security-Policy") {
		t.Error("expected a CSP meta tag even in separate-files mode")
	}
	assertNoUnnoncedInlineScripts(t, html)
}
