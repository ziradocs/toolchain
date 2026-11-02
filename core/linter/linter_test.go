// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// hasDiagnostic busca id tanto en RuleID (diagnostics.NewError(...).
// WithRuleID(...), el camino de FrontMatterValidRule/CORE00x) como en Code
// (el struct literal directo que usan las reglas de layout, LAYOUT_*) — las
// dos convenciones conviven en este paquete y un test que solo mire una se
// equivoca en silencio con diagnósticos de la otra.
func hasDiagnostic(diags []diagnostics.Diagnostic, id string) bool {
	for _, d := range diags {
		if d.RuleID == id || d.Code == id {
			return true
		}
	}
	return false
}

// TestLinter_WithDialect_IntegratesEndToEnd confirma que Linter.Lint() con
// WithDialect(DialectDocuments) realmente apaga FRONT003/LAYOUT002/
// LAYOUT_FORBIDDEN_ELEMENT para un documento — no solo que las reglas
// individuales lo hacen aisladas (ver rules_test.go/layout_validation_test.go).
// Repro end-to-end del audit 2026-09-11 (F11/F12): un .doclang sin
// frontmatter, con prosa normal en su primera sección (BlockType "title"),
// disparaba los tres.
func TestLinter_WithDialect_IntegratesEndToEnd(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	astNode := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "title")
	block.Heading = "Introducción"
	block.Elements = append(block.Elements, ast.NewTextElement(pos, "prosa normal de un documento"))
	astNode.ContentBlocks = append(astNode.ContentBlocks, *block)
	// FrontMatter deliberadamente nil — DocumentFlexParser lo tolera.

	baseline := New().Lint(astNode)
	if !hasDiagnostic(baseline, "FRONT003") {
		t.Fatalf("baseline (sin WithDialect) debe reportar FRONT003, obtenidos: %+v", baseline)
	}
	if !hasDiagnostic(baseline, "LAYOUT_FORBIDDEN_ELEMENT") {
		t.Fatalf("baseline (sin WithDialect) debe reportar LAYOUT_FORBIDDEN_ELEMENT, obtenidos: %+v", baseline)
	}

	filtered := New().WithDialect(DialectDocuments).Lint(astNode)
	if hasDiagnostic(filtered, "FRONT003") {
		t.Errorf("WithDialect(DialectDocuments) debe apagar FRONT003, obtenidos: %+v", filtered)
	}
	if hasDiagnostic(filtered, "LAYOUT_FORBIDDEN_ELEMENT") {
		t.Errorf("WithDialect(DialectDocuments) debe apagar LAYOUT_FORBIDDEN_ELEMENT, obtenidos: %+v", filtered)
	}
}

// TestLinter_WithDialect_AppliesToRulesAddedAfter confirma que AddRule (no
// solo WithDialect) también cablea el dialecto ya fijado — mismo contrato
// que layoutPolicyAware/ThemeAware ya garantizan para policy/tema.
func TestLinter_WithDialect_AppliesToRulesAddedAfter(t *testing.T) {
	l := NewWithRules().WithDialect(DialectDocuments)
	rule := &FrontMatterValidRule{}
	l.AddRule(rule)

	if rule.dialect != DialectDocuments {
		t.Errorf("AddRule no propagó el dialecto ya fijado, rule.dialect = %v, want %v", rule.dialect, DialectDocuments)
	}
}

// TestLinter_WithDialect_EmptyDocumentSaysDocument: CORE001 sigue siendo
// error para un documento vacío, pero en DialectDocuments no habla de
// slides; sin dialecto el mensaje de slidelang queda igual.
func TestLinter_WithDialect_EmptyDocumentSaysDocument(t *testing.T) {
	astNode := ast.NewAST(diagnostics.NewPosition(1, 1))

	find := func(diags []diagnostics.Diagnostic) *diagnostics.Diagnostic {
		for i := range diags {
			if diags[i].RuleID == "CORE001" || diags[i].Code == "CORE001" {
				return &diags[i]
			}
		}
		return nil
	}

	base := find(New().Lint(astNode))
	if base == nil || base.Message != "Presentation must contain at least one slide" || !base.IsError() {
		t.Fatalf("sin dialecto CORE001 no debe cambiar, got %+v", base)
	}
	doc := find(New().WithDialect(DialectDocuments).Lint(astNode))
	if doc == nil || !doc.IsError() {
		t.Fatalf("un documento vacío sigue siendo error CORE001, got %+v", doc)
	}
	if strings.Contains(strings.ToLower(doc.Message), "slide") || strings.Contains(strings.ToLower(doc.Message), "presentation") {
		t.Errorf("el mensaje de documentos no debe hablar de slides, got %q", doc.Message)
	}
}

// TestLinter_WithDialect_NoClosingRelabelForDocuments: la sección inicial
// sin encabezado de un Markdown con solo `##` es también la última y no
// tiene título; en documentos no se reetiqueta "closing".
func TestLinter_WithDialect_NoClosingRelabelForDocuments(t *testing.T) {
	build := func() *ast.AST {
		pos := diagnostics.NewPosition(1, 1)
		astNode := ast.NewAST(pos)
		block := ast.NewContentBlock(pos, "content")
		block.Elements = append(block.Elements, ast.NewTextElement(pos, "texto"))
		astNode.ContentBlocks = append(astNode.ContentBlocks, *block)
		return astNode
	}

	slides := build()
	if diags := New().Lint(slides); !hasDiagnostic(diags, "LAYOUT_AUTO_CLOSING") || slides.ContentBlocks[0].BlockType != "closing" {
		t.Fatalf("sin dialecto el último slide sin título sigue siendo closing, got %q %+v", slides.ContentBlocks[0].BlockType, diags)
	}
	doc := build()
	diags := New().WithDialect(DialectDocuments).Lint(doc)
	if hasDiagnostic(diags, "LAYOUT_AUTO_CLOSING") || doc.ContentBlocks[0].BlockType != "content" {
		t.Errorf("en documentos no hay closing, got %q %+v", doc.ContentBlocks[0].BlockType, diags)
	}
}
