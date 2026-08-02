// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import "testing"

// TestFrontMatterParser_Lang cubre el prerrequisito de los issues #62/#63:
// `lang:` en el frontmatter debe llegar a FrontMatterNode.Lang como campo de
// primera clase, y NO debe requerir que el autor lo repita dentro de
// `variables:` para que otras rutas (BuildVariables → {{lang}}) lo vean.
func TestFrontMatterParser_Lang(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nlang: fr\n---\n\nContenido.")
	for _, d := range diags {
		t.Logf("diagnostic: %v", d)
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Lang != "fr" {
		t.Errorf("Lang = %q, want %q", node.Lang, "fr")
	}
}

func TestFrontMatterParser_LangAbsent(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Lang != "" {
		t.Errorf("Lang = %q, want empty when not declared", node.Lang)
	}
}

func TestFrontMatterNode_BuildVariables_IncludesLang(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\nlang: pt-BR\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	vars := node.BuildVariables()
	if got, _ := vars["lang"].(string); got != "pt-BR" {
		t.Errorf("BuildVariables()[\"lang\"] = %v, want %q", vars["lang"], "pt-BR")
	}
}
