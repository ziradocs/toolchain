// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import "testing"

// TestFrontMatterParser_Lang cubre el prerrequisito de los issues #62/#63:
// `lang:` en el frontmatter debe llegar a FrontMatterNode.Lang como campo de
// primera clase.
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

// TestFrontMatterParser_LangInvalid cubre el prerrequisito de code review:
// a11y.IsValidLangTag existía sin ningún caller de producción, así que un
// `lang:` malformado llegaba sin aviso hasta <html lang>/w:lang. El parser
// ahora emite un diagnóstico FRONT004.
func TestFrontMatterParser_LangInvalid(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\nlang: es_MX\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	// El valor crudo se preserva en el AST aunque sea inválido — el
	// diagnóstico avisa, no reescribe silenciosamente lo que el autor
	// declaró.
	if node.Lang != "es_MX" {
		t.Errorf("Lang = %q, want %q (raw value preserved)", node.Lang, "es_MX")
	}

	var found bool
	for _, d := range diags {
		if d.RuleID == "FRONT004" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a FRONT004 diagnostic for an invalid lang tag, got: %+v", diags)
	}
}

func TestFrontMatterParser_LangValid_NoDiagnostic(t *testing.T) {
	p := &FrontMatterParser{}

	_, _, diags := p.Parse("---\nmode: flex\nlang: es-MX\n---\n\nContenido.")
	for _, d := range diags {
		if d.RuleID == "FRONT004" {
			t.Errorf("did not expect a FRONT004 diagnostic for a well-formed lang tag, got: %+v", d)
		}
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

// TestFrontMatterNode_BuildVariables_DoesNotIncludeLang es una regresión de
// code review: BuildVariables NO debe promover "lang" a placeholder de
// sustitución — un documento que declara `lang:` y también menciona
// "{{lang}}" como texto literal (p.ej. documentación sobre el propio
// mecanismo de idioma) no debe ver ese texto reescrito.
func TestFrontMatterNode_BuildVariables_DoesNotIncludeLang(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\nlang: pt-BR\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	vars := node.BuildVariables()
	if _, ok := vars["lang"]; ok {
		t.Errorf("BuildVariables()[\"lang\"] = %v, want absent (lang is not a substitution built-in)", vars["lang"])
	}
}
