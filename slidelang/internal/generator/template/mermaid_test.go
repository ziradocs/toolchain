// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"strings"
	"testing"
)

// TestMermaidAsset_SafeAndConsistent es el lock del lado Go sobre el
// comportamiento del asset JS embebido (issues #73 SINK B + #70): el
// fallback de renderDiagramFromConfig ya no debe construir HTML vía
// innerHTML con el source crudo del diagrama, y la config de
// mermaid.initialize debe ser consistente (securityLevel:'strict',
// htmlLabels:false) con los demás sitios de init del repo.
func TestMermaidAsset_SafeAndConsistent(t *testing.T) {
	js := GetMermaidJS()

	if strings.Contains(js, "innerHTML = `<div class=\"mermaid\">${") {
		t.Error("expected the mermaid.run fallback to no longer build HTML via innerHTML string interpolation")
	}
	if !strings.Contains(js, "textContent = graphDefinition") {
		t.Error("expected the mermaid.run fallback to inject diagram content via textContent, not innerHTML")
	}
	if !strings.Contains(js, "securityLevel: 'strict'") {
		t.Error("expected mermaid.initialize to set securityLevel:'strict'")
	}
	if !strings.Contains(js, "htmlLabels: false") {
		t.Error("expected mermaid.initialize to set htmlLabels:false")
	}
	if strings.Contains(js, "htmlLabels: true") {
		t.Error("expected no remaining htmlLabels:true (the #70 divergence)")
	}
}

func TestMermaidJSFallback_HtmlLabelsFalse(t *testing.T) {
	js := getMermaidJSFallback()

	if !strings.Contains(js, "htmlLabels: false") {
		t.Error("expected the minimal fallback's mermaid.initialize to also set htmlLabels:false")
	}
}
