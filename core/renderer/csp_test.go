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
