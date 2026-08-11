// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// TestRenderHTMLPreview_UndeclaredTOCKeepsDefaultOff covers the "only when
// declared" contract from RenderHTMLPreview's doc comment: a document that
// says nothing about `toc:` must keep the preview's own pre-existing
// default (no TOC), untouched by `doclang build`'s separate "front matter
// present ⇒ TOC on" defaulting policy.
func TestRenderHTMLPreview_UndeclaredTOCKeepsDefaultOff(t *testing.T) {
	gen := New(newTestLogger())
	doc := newTestAST()

	html := gen.RenderHTMLPreview(doc, "")

	if strings.Contains(html, "Tabla de Contenidos") {
		t.Errorf("expected no TOC section when toc: isn't declared, got:\n%s", html)
	}
}

// TestRenderHTMLPreview_HonorsExplicitTOCTrue covers the opt-in half of the
// same contract.
func TestRenderHTMLPreview_HonorsExplicitTOCTrue(t *testing.T) {
	gen := New(newTestLogger())
	doc := newTestAST()
	enabled := true
	doc.FrontMatter.TOC = &ast.TOCConfig{Enabled: &enabled}

	html := gen.RenderHTMLPreview(doc, "")

	if !strings.Contains(html, "Tabla de Contenidos") {
		t.Errorf("expected a TOC section when toc.enabled is explicitly true, got:\n%s", html)
	}
}

// TestRenderHTMLPreview_HonorsExplicitTOCFalse covers the explicit opt-out
// (distinct from "undeclared" above — both currently render no TOC, but for
// different reasons, and this locks in the explicit-false path
// specifically).
func TestRenderHTMLPreview_HonorsExplicitTOCFalse(t *testing.T) {
	gen := New(newTestLogger())
	doc := newTestAST()
	disabled := false
	doc.FrontMatter.TOC = &ast.TOCConfig{Enabled: &disabled}

	html := gen.RenderHTMLPreview(doc, "")

	if strings.Contains(html, "Tabla de Contenidos") {
		t.Errorf("expected no TOC section when toc.enabled is explicitly false, got:\n%s", html)
	}
}
