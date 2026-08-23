// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"
)

func TestGenerateCSPNonce_ProducesDistinctValues(t *testing.T) {
	n1, err := GenerateCSPNonce()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n2, err := GenerateCSPNonce()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n1 == "" || n2 == "" {
		t.Fatal("expected non-empty nonces")
	}
	if n1 == n2 {
		t.Error("expected two calls to produce distinct nonces")
	}
}

func TestBuildDefaultOutputCSP_IncludesNonceAndRequiredHosts(t *testing.T) {
	nonce := "test-nonce-123"
	csp := BuildDefaultOutputCSP(nonce)

	for _, want := range []string{
		"'nonce-test-nonce-123'",
		"https://cdn.jsdelivr.net",
		"https://unpkg.com",
		"script-src",
		"style-src",
		"img-src",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("expected CSP to contain %q, got: %s", want, csp)
		}
	}

	scriptSrc := strings.Split(strings.Split(csp, "script-src ")[1], ";")[0]
	if strings.Contains(scriptSrc, "'unsafe-inline'") {
		t.Error("expected script-src to not use 'unsafe-inline' (defeats the nonce) — real code-execution risk lives here")
	}

	// style-src SÍ usa 'unsafe-inline' a propósito: verificado en vivo que
	// Mermaid inyecta su CSS de tema en runtime vía un <style> sin nonce y
	// sin forma de asignarle uno — un style-src con nonce lo bloquea
	// silenciosamente y rompe el render (ver comentario en
	// BuildDefaultOutputCSP). No reabre BA-11: esa vulnerabilidad se cierra
	// en el string vía SanitizeCSSCustomProperty, no acá.
	styleSrc := strings.Split(strings.Split(csp, "style-src ")[1], ";")[0]
	if !strings.Contains(styleSrc, "'unsafe-inline'") {
		t.Error("expected style-src to use 'unsafe-inline' (Mermaid injects unnonced runtime styles)")
	}
}

// TestBuildDefaultOutputCSP_FontSrcSelfHostedOnly is the §2.3 regression:
// before this, there was no font-src directive at all, so with
// default-src 'self' any @font-face a theme declared was blocked SILENTLY
// (no console violation without opening DevTools). font-src must now be
// present and scoped to 'self'/data: only — no external host — matching
// the §2.3 decision to always self-host a theme's fonts rather than link
// to a provider.
func TestBuildDefaultOutputCSP_FontSrcSelfHostedOnly(t *testing.T) {
	csp := BuildDefaultOutputCSP("test-nonce-123")

	if !strings.Contains(csp, "font-src") {
		t.Fatal("expected CSP to declare a font-src directive")
	}

	fontSrc := strings.Split(strings.Split(csp, "font-src ")[1], ";")[0]
	if !strings.Contains(fontSrc, "'self'") {
		t.Error("expected font-src to include 'self' (same-origin bundled fonts)")
	}
	if !strings.Contains(fontSrc, "data:") {
		t.Error("expected font-src to include data: (offline-inline/--embed-assets embeds fonts as data: URIs)")
	}
	if strings.Contains(fontSrc, "https://") || strings.Contains(fontSrc, "http://") {
		t.Errorf("font-src must not reference an external font provider — the motor always self-hosts, got: %q", fontSrc)
	}
}
