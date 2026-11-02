// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"crypto/sha512"
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// TestEmbeddedMermaid_MatchesCDNIntegrity: el build embebido es byte a byte
// el que el HTML en modo browser carga del CDN. Si alguien bumpea la versión
// del tag (y su hash SRI) sin reemplazar el archivo embebido, o al revés,
// este test falla.
func TestEmbeddedMermaid_MatchesCDNIntegrity(t *testing.T) {
	m := regexp.MustCompile(`integrity="sha384-([^"]+)"`).FindStringSubmatch(renderer.MermaidCDNScriptTag)
	if m == nil {
		t.Fatalf("no sha384 integrity in MermaidCDNScriptTag: %s", renderer.MermaidCDNScriptTag)
	}
	sum := sha512.Sum384([]byte(embeddedMermaidJS))
	if got := base64.StdEncoding.EncodeToString(sum[:]); got != m[1] {
		t.Errorf("embedded mermaid sha384 = %s, CDN tag pins %s", got, m[1])
	}
}

// TestEmbeddedMermaid_HasNoScriptClose: el bundle va inline dentro de
// <script>; un "</script" literal cerraría el elemento antes de tiempo.
func TestEmbeddedMermaid_HasNoScriptClose(t *testing.T) {
	if strings.Contains(strings.ToLower(embeddedMermaidJS), "</script") {
		t.Error("embedded mermaid bundle contains </script; the inline tag relies on the escape in embeddedMermaidScriptTag")
	}
	if strings.Count(embeddedMermaidScriptTag, "</script>") != 1 {
		t.Error("embeddedMermaidScriptTag must close exactly once")
	}
}

// TestMermaidPages_NeedNoNetwork: las páginas que rasteriza Chromium llevan
// el build inline y una CSP sin ningún origen de red, así que DOCX y PDF con
// mermaid funcionan sin conexión.
func TestMermaidPages_NeedNoNetwork(t *testing.T) {
	pages := map[string]string{
		"svg": buildMermaidSVGHTML("graph TD; A-->B"),
		"png": buildMermaidPNGHTML("graph TD; A-->B", 800),
	}
	for name, html := range pages {
		if strings.Contains(html, "cdn.jsdelivr.net") || strings.Contains(html, "<script src=") {
			t.Errorf("%s page still loads mermaid from the network", name)
		}
		if !strings.Contains(html, embeddedMermaidScriptTag) {
			t.Errorf("%s page does not inline the embedded mermaid build", name)
		}
		if !strings.Contains(html, `content="`+mermaidRenderCSP+`"`) {
			t.Errorf("%s page does not use mermaidRenderCSP", name)
		}
	}
	if strings.Contains(mermaidRenderCSP, "https:") || strings.Contains(mermaidRenderCSP, "http:") {
		t.Errorf("mermaidRenderCSP allows a network origin: %s", mermaidRenderCSP)
	}
}

// TestMermaidPNGDeviceScale fija el factor que doclang usa para recuperar el
// tamaño natural del diagrama (ver mermaidPNGDeviceScale).
func TestMermaidPNGDeviceScale(t *testing.T) {
	if mermaidPNGDeviceScale != 2 {
		t.Errorf("mermaidPNGDeviceScale = %v; doclang's docx.go assumes 2", mermaidPNGDeviceScale)
	}
}
